// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package prudence

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	sany "github.com/jwriteclub/prudence/internal/generated/sany"
	parser "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/lexer"
	"github.com/jwriteclub/prudence/internal/modulegraph"
)

// The raw-byte corpus lives in testdata/raw-utf8; its README documents the
// format, templates, expectation keys and which rows still need the oracle.
const rawUTF8Manifest = "testdata/raw-utf8/manifest.tsv"

// rawUTF8Rows pins the finite corpus size so a lost row fails loudly.
const rawUTF8Rows = 99

const rawUTF8Resolver = "---- MODULE M ----\nEXTENDS N\n====\n"

type rawRow struct {
	id, form, basis, status string
	bytes                   []byte
	groups                  []rawGroupWant
	expect                  []string
}

type rawGroupWant struct {
	bad    bool
	scalar rune
	length int
}

// rawGroup is one decoder group as the product lexer sees it: a scalar (or
// U+FFFD for a malformed group), its byte range and its Java position.
type rawGroup struct {
	scalar       rune
	start, end   int
	line, column int
}

func unescapeRaw(s string) ([]byte, error) {
	var out []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c > 0x7e {
			return nil, fmt.Errorf("unescaped byte %#x at %d", c, i)
		}
		if c != '\\' {
			out = append(out, c)
			continue
		}
		if i+1 >= len(s) {
			return nil, errors.New("dangling escape")
		}
		i++
		switch s[i] {
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case '\\':
			out = append(out, '\\')
		case 'x':
			if i+2 >= len(s) {
				return nil, errors.New("short hex escape")
			}
			v, err := strconv.ParseUint(s[i+1:i+3], 16, 8)
			if err != nil {
				return nil, err
			}
			out = append(out, byte(v))
			i += 2
		default:
			return nil, fmt.Errorf("unknown escape %q", s[i])
		}
	}
	return out, nil
}

func parseGroupWants(s string) ([]rawGroupWant, error) {
	var out []rawGroupWant
	for _, field := range strings.Fields(s) {
		name, size, ok := strings.Cut(field, "/")
		if !ok {
			return nil, fmt.Errorf("group %q", field)
		}
		n, err := strconv.Atoi(size)
		if err != nil || n < 1 || n > 4 {
			return nil, fmt.Errorf("group length %q", field)
		}
		if name == "bad" {
			out = append(out, rawGroupWant{bad: true, scalar: 0xfffd, length: n})
			continue
		}
		hex, ok := strings.CutPrefix(name, "U+")
		if !ok {
			return nil, fmt.Errorf("group scalar %q", field)
		}
		v, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return nil, err
		}
		out = append(out, rawGroupWant{scalar: rune(v), length: n})
	}
	return out, nil
}

func loadRawUTF8(t *testing.T) ([]rawRow, map[string]rawRow) {
	t.Helper()
	data, err := os.ReadFile(rawUTF8Manifest)
	if err != nil {
		t.Fatal(err)
	}
	// The manifest itself carries no invisible or non-ASCII bytes.
	for i, c := range data {
		if (c < 0x20 && c != '\t' && c != '\n') || c > 0x7e {
			t.Fatalf("manifest byte %#x at %d", c, i)
		}
	}
	var rows []rawRow
	byID := map[string]rawRow{}
	for n, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 7 {
			t.Fatalf("line %d: %d fields", n+1, len(f))
		}
		b, err := unescapeRaw(f[2])
		if err != nil {
			t.Fatalf("%s bytes: %v", f[0], err)
		}
		g, err := parseGroupWants(f[3])
		if err != nil {
			t.Fatalf("%s groups: %v", f[0], err)
		}
		r := rawRow{id: f[0], form: f[1], bytes: b, groups: g, basis: f[5], status: f[6]}
		if f[4] != "-" {
			r.expect = strings.Fields(f[4])
		}
		if _, dup := byID[r.id]; dup || r.id == "" || r.basis == "" || r.status == "" {
			t.Fatalf("row %q duplicate or incomplete", r.id)
		}
		rows = append(rows, r)
		byID[r.id] = r
	}
	if len(rows) != rawUTF8Rows {
		t.Fatalf("corpus has %d rows, want %d", len(rows), rawUTF8Rows)
	}
	return rows, byID
}

func rawTemplate(form string, x []byte) (src []byte, start int, ok bool) {
	pad := 0
	if at, found := strings.CutPrefix(form, "C@"); found {
		n, err := strconv.Atoi(at)
		if err != nil || n < 22 {
			return nil, 0, false
		}
		pad, form = n-22, "C"
	}
	var head, tail string
	switch form {
	case "C":
		head, tail = "---- MODULE M ----\n(* "+strings.Repeat("a", pad), " *) @\n====\n"
	case "K":
		head, tail = "---- MODULE M ----\nx == ", "\n====\n"
	case "S":
		head, tail = "---- MODULE M ----\nx == \"a", "b\"\n====\n"
	case "T":
		head, tail = "---- MODULE M ----\nx == \"a", ""
	default:
		return nil, 0, false
	}
	src = append(append([]byte(head), x...), tail...)
	return src, len(head), true
}

func productGroups(src []byte) []rawGroup {
	l := sany.New(string(src))
	units, positions := l.InputUnits()
	ranges := l.InputByteRanges()
	var out []rawGroup
	for i := 0; i < len(units); i++ {
		r := rune(units[i])
		next := i
		if i+1 < len(units) && ranges[i+1] == ranges[i] && utf16.IsSurrogate(r) {
			r = utf16.DecodeRune(r, rune(units[i+1]))
			next = i + 1
		}
		out = append(out, rawGroup{r, ranges[i][0], ranges[i][1], positions[i].Line, positions[i].Column})
		i = next
	}
	return out
}

func groupMatches(src []byte, g rawGroup, w rawGroupWant) bool {
	if g.end-g.start != w.length || g.scalar != w.scalar {
		return false
	}
	encodedFFFD := bytes.Equal(src[g.start:g.end], []byte{0xef, 0xbf, 0xbd})
	return w.scalar != 0xfffd || w.bad != encodedFFFD
}

func checkGroups(t *testing.T, id string, src []byte, got []rawGroup, want []rawGroupWant) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: groups %+v, want %+v", id, got, want)
	}
	for i := range want {
		if !groupMatches(src, got[i], want[i]) {
			t.Fatalf("%s: group %d %+v (% x), want %+v", id, i, got[i], src[got[i].start:got[i].end], want[i])
		}
	}
}

// groupsIn returns the groups tiling src[a:b], failing if a group crosses it.
func groupsIn(t *testing.T, id string, all []rawGroup, a, b int) []rawGroup {
	t.Helper()
	var out []rawGroup
	for _, g := range all {
		if g.end <= a || g.start >= b {
			continue
		}
		if g.start < a || g.end > b {
			t.Fatalf("%s: group %+v crosses [%d,%d)", id, g, a, b)
		}
		out = append(out, g)
	}
	return out
}

func nonASCIIGroups(src []byte, all []rawGroup) []rawGroup {
	var out []rawGroup
	for _, g := range all {
		if src[g.start] >= 0x80 {
			out = append(out, g)
		}
	}
	return out
}

func groupBoundaries(src []byte) map[int]bool {
	out := map[int]bool{len(src): true}
	for _, g := range productGroups(src) {
		out[g.start] = true
	}
	return out
}

func checkSpanBoundaries(t *testing.T, what string, b map[int]bool, sp Span) {
	t.Helper()
	if !b[sp.Start.ByteOffset] || !b[sp.End.ByteOffset] {
		t.Fatalf("%s: span [%d,%d) splits a decoder group", what, sp.Start.ByteOffset, sp.End.ByteOffset)
	}
}

// javaColumn applies SimpleCharStream's column rule (tab size 8).
func javaColumn(units []uint16) int {
	col := 0
	for _, u := range units {
		col++
		if u == '\t' {
			col--
			col += 8 - col%8
		}
	}
	return col
}

func javaEscape(u uint16) string {
	if u < 0x20 || u > 0x7e {
		return fmt.Sprintf(`\u%04x`, u)
	}
	return string(rune(u))
}

func groupUnits(gs []rawGroup) []uint16 {
	var out []uint16
	for _, g := range gs {
		out = append(out, utf16.Encode([]rune{g.scalar})...)
	}
	return out
}

func rawResolver(n []byte) func(context.Context, string) (Source, error) {
	return func(_ context.Context, name string) (Source, error) {
		if name == "N" {
			return Source{Path: "N.tla", Text: n}, nil
		}
		return Source{}, ErrModuleNotFound
	}
}

func intPair(t *testing.T, id, s string) (int, int) {
	t.Helper()
	a, b, ok := strings.Cut(s, "-")
	x, err1 := strconv.Atoi(a)
	y, err2 := strconv.Atoi(b)
	if !ok || err1 != nil || err2 != nil {
		t.Fatalf("%s: range %q", id, s)
	}
	return x, y
}

func intList(t *testing.T, id, s string, sep string) []int {
	t.Helper()
	var out []int
	for _, f := range strings.Split(s, sep) {
		v, err := strconv.Atoi(f)
		if err != nil {
			t.Fatalf("%s: number %q", id, s)
		}
		out = append(out, v)
	}
	return out
}

// lineColumnAt splits an expectation of the form L:C@REST.
func lineColumnAt(t *testing.T, id, s string) (int, int, string) {
	t.Helper()
	lc, rest, ok := strings.Cut(s, "@")
	v := intList(t, id, lc, ":")
	if !ok || len(v) != 2 {
		t.Fatalf("%s: position %q", id, s)
	}
	return v[0], v[1], rest
}

func firstDiagnostic(ds []Diagnostic, code string) *Diagnostic {
	for i := range ds {
		if ds[i].Code == code {
			return &ds[i]
		}
	}
	return nil
}

func hasErrors(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

func treeHasImage(n *parser.SyntaxTreeNode, want []uint16) bool {
	if n == nil {
		return false
	}
	if img := n.Image(); img != nil {
		u := img.Units()
		if slices.Equal(u, want) || (len(u) == len(want)+2 && u[0] == '"' && u[len(u)-1] == '"' && slices.Equal(u[1:len(u)-1], want)) {
			return true
		}
	}
	for _, h := range n.Heirs() {
		if treeHasImage(h, want) {
			return true
		}
	}
	return false
}

// TestRawUTF8Corpus runs every manifest row through the product lexer, the
// strict parser, the recovering scan, monolith extraction and public Check.
func TestRawUTF8Corpus(t *testing.T) {
	rows, byID := loadRawUTF8(t)
	extras := map[string]func(*testing.T, rawRow){
		"B4":    rawCheckResolverLikeRoot,
		"I2-c0": func(t *testing.T, _ rawRow) { rawCheckIdentityClass(t, byID) },
		"I3":    rawCheckSupplementaryFallback,
	}
	for _, row := range rows {
		t.Run(row.id, func(t *testing.T) {
			forms := strings.Fields(row.form)
			if len(forms) == 0 {
				t.Fatal("no form")
			}
			for _, form := range forms {
				switch form {
				case "raw":
					rawCheckInput(t, row, row.bytes, nil)
				case "resolver":
					rawCheckInput(t, row, []byte(rawUTF8Resolver), row.bytes)
				default:
					src, start, ok := rawTemplate(form, row.bytes)
					if !ok {
						t.Fatalf("form %q", form)
					}
					all := productGroups(src)
					x := groupsIn(t, row.id, all, start, start+len(row.bytes))
					if len(x) == 0 || x[0].start != start || x[len(x)-1].end != start+len(row.bytes) {
						t.Fatalf("groups do not tile X: %+v", x)
					}
					checkGroups(t, row.id+"/"+form, src, x, row.groups)
					rawCheckInvariants(t, row.id+"/"+form, src, nil)
					rawCheckTemplate(t, row, form, src, start, x)
				}
			}
			if f := extras[row.id]; f != nil {
				f(t, row)
			} else if (row.form == "raw" || row.form == "resolver") && len(row.expect) == 0 {
				t.Fatal("raw row without expectations")
			}
		})
	}
}

func rawCheckTemplate(t *testing.T, row rawRow, form string, src []byte, start int, x []rawGroup) {
	t.Helper()
	ctx := context.Background()
	id := row.id + "/" + form
	r, err := Parse(ctx, "M.tla", src, Options{})
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	switch {
	case form == "C" || strings.HasPrefix(form, "C@"):
		line := append(utf16.Encode([]rune("(* "+strings.Repeat("a", start-22))), groupUnits(x)...)
		line = append(line, utf16.Encode([]rune(" *) @"))...)
		at := start + len(row.bytes) + 4
		d := firstDiagnostic(r.Diagnostics, CodeSyntax)
		want := fmt.Sprintf("Encountered \"@\" at line 2, column %d ", javaColumn(line))
		if d == nil || len(r.Diagnostics) != 1 || !strings.Contains(d.Message, want) || d.Span.Start.ByteOffset != at || d.Span.End.ByteOffset != at+1 || d.Span.Start.Line != 1 {
			t.Fatalf("%s: want %q at byte %d, got %+v", id, want, at, r.Diagnostics)
		}
	case form == "K":
		u := utf16.Encode([]rune{x[0].scalar})[0]
		want := fmt.Sprintf("Lexical error at line 2, column 6.  Encountered: \"%s\" (%d)", javaEscape(u), u)
		if len(r.Diagnostics) == 0 {
			t.Fatalf("%s: no diagnostic", id)
		}
		d := r.Diagnostics[0]
		if d.Code != CodeLexical || !strings.HasPrefix(d.Message, want) || d.Span.Start.ByteOffset != 24 || d.Span.End.ByteOffset != x[0].end || d.Span.Start.Line != 1 {
			t.Fatalf("%s: want %q over [24,%d), got %+v", id, want, x[0].end, d)
		}
		_, ds, err := Tokens(ctx, "M.tla", src)
		if err != nil || len(ds) == 0 || ds[0].Span != d.Span {
			t.Fatalf("%s: scan %+v %v", id, ds, err)
		}
	case form == "S":
		c, err := Check(ctx, Request{Path: "M.tla", Text: src})
		if err != nil || c.Level != LevelComplete || c.Completion != CompletionComplete || hasErrors(c.Diagnostics) {
			t.Fatalf("%s: check %+v %v", id, c, err)
		}
		raw, err := parser.Parse(ctx, src, parser.ParseOptions{})
		want := append(append([]uint16{'a'}, groupUnits(x)...), 'b')
		if err != nil || len(raw.Errors) != 0 || !treeHasImage(raw.Tree, want) {
			t.Fatalf("%s: string image %v not in tree (%v)", id, want, err)
		}
	case form == "T":
		if len(r.Diagnostics) == 0 || r.Diagnostics[0].Code != CodeLexical || !strings.Contains(r.Diagnostics[0].Message, "Encountered: <EOF>") {
			t.Fatalf("%s: want an EOF lexical error, got %+v", id, r.Diagnostics)
		}
	}
}

// rawCheckInvariants confirms raw-byte identity and that no public token or
// diagnostic span boundary falls inside a decoder group.
func rawCheckInvariants(t *testing.T, id string, src, resolved []byte) {
	t.Helper()
	ctx := context.Background()
	keep := append([]byte(nil), src...)
	keepN := append([]byte(nil), resolved...)
	b := groupBoundaries(src)
	r, err := Parse(ctx, "M.tla", src, Options{IncludeTokens: true})
	if err != nil {
		t.Fatalf("%s: parse %v", id, err)
	}
	for _, tok := range r.Tokens {
		checkSpanBoundaries(t, id+" token", b, tok.Span)
	}
	for _, d := range r.Diagnostics {
		checkSpanBoundaries(t, id+" parse diagnostic", b, d.Span)
	}
	toks, ds, err := Tokens(ctx, "M.tla", src)
	if err != nil {
		t.Fatalf("%s: tokens %v", id, err)
	}
	for _, tok := range toks {
		checkSpanBoundaries(t, id+" scan token", b, tok.Span)
	}
	for _, d := range ds {
		checkSpanBoundaries(t, id+" scan diagnostic", b, d.Span)
	}
	req := Request{Path: "M.tla", Text: src}
	if resolved != nil {
		req.Resolve = rawResolver(resolved)
	}
	c, err := Check(ctx, req)
	if err != nil {
		t.Fatalf("%s: check %v", id, err)
	}
	bn := groupBoundaries(resolved)
	for _, d := range c.Diagnostics {
		switch d.Path {
		case "M.tla":
			checkSpanBoundaries(t, id+" check diagnostic", b, d.Span)
		case "N.tla":
			checkSpanBoundaries(t, id+" resolver diagnostic", bn, d.Span)
		default:
			t.Fatalf("%s: unexpected path %+v", id, d)
		}
	}
	if !bytes.Equal(src, keep) || !bytes.Equal(resolved, keepN) {
		t.Fatalf("%s: caller bytes changed", id)
	}
}

func rawCheckInput(t *testing.T, row rawRow, src, resolved []byte) {
	t.Helper()
	ctx := context.Background()
	decoded := src
	if resolved != nil {
		decoded = resolved
	}
	all := productGroups(decoded)
	checkGroups(t, row.id, decoded, nonASCIIGroups(decoded, all), row.groups)
	rawCheckInvariants(t, row.id, src, resolved)
	req := Request{Path: "M.tla", Text: src}
	if resolved != nil {
		req.Resolve = rawResolver(resolved)
	}
	check := func() Result {
		c, err := Check(ctx, req)
		if err != nil {
			t.Fatalf("%s: check %v", row.id, err)
		}
		return c
	}
	for _, e := range row.expect {
		key, val, _ := strings.Cut(e, "=")
		switch key {
		case "parse":
			l, c, at := lineColumnAt(t, row.id, val)
			b := intList(t, row.id, at, ":")[0]
			r, err := Parse(ctx, "M.tla", src, Options{})
			want := fmt.Sprintf("Encountered \"@\" at line %d, column %d ", l, c)
			d := firstDiagnostic(r.Diagnostics, CodeSyntax)
			if err != nil || d == nil || !strings.Contains(d.Message, want) || d.Span.Start.ByteOffset != b || d.Span.End.ByteOffset != b+1 || d.Span.Start.Line != l-1 {
				t.Fatalf("%s: want %q at %d, got %+v %v", row.id, want, b, r.Diagnostics, err)
			}
		case "lex":
			l, c, rng := lineColumnAt(t, row.id, val)
			a, b := intPair(t, row.id, rng)
			r, err := Parse(ctx, "M.tla", src, Options{})
			d := firstDiagnostic(r.Diagnostics, CodeLexical)
			want := fmt.Sprintf("Lexical error at line %d, column %d.", l, c)
			if err != nil || d == nil || !strings.HasPrefix(d.Message, want) || d.Span.Start.ByteOffset != a || d.Span.End.ByteOffset != b {
				t.Fatalf("%s: want %q over [%d,%d), got %+v %v", row.id, want, a, b, r.Diagnostics, err)
			}
		case "jpos":
			v := intList(t, row.id, val, ":")
			found := false
			for _, g := range all {
				if g.start == v[0] {
					found = true
					if g.line != v[1] || g.column != v[2] {
						t.Fatalf("%s: byte %d at Java %d:%d, want %d:%d", row.id, v[0], g.line, g.column, v[1], v[2])
					}
				}
			}
			if !found {
				t.Fatalf("%s: no unit starts at byte %d", row.id, v[0])
			}
		case "check":
			if c := check(); c.Level != LevelComplete || c.Completion != CompletionComplete || hasErrors(c.Diagnostics) {
				t.Fatalf("%s: want semantic without errors, got %+v", row.id, c)
			}
		case "checksem":
			a, b := intPair(t, row.id, val)
			c := check()
			ok := c.Level == LevelSemantic
			n := 0
			for _, d := range c.Diagnostics {
				if d.Severity == SeverityError {
					n++
					ok = ok && d.Phase == LevelSemantic && d.Path == "M.tla" && d.Span.Start.ByteOffset == a && d.Span.End.ByteOffset == b
				}
			}
			if !ok || n != 1 {
				t.Fatalf("%s: want one semantic error over [%d,%d), got %+v", row.id, a, b, c)
			}
		case "checklex":
			path, pos, _ := strings.Cut(val, ":")
			l, col, rng := lineColumnAt(t, row.id, pos)
			a, b := intPair(t, row.id, rng)
			d := firstDiagnostic(check().Diagnostics, CodeLexical)
			want := fmt.Sprintf("Lexical error at line %d, column %d.", l, col)
			if d == nil || d.Path != path || !strings.HasPrefix(d.Message, want) || d.Span.Start.ByteOffset != a || d.Span.End.ByteOffset != b {
				t.Fatalf("%s: want %s %q over [%d,%d), got %+v", row.id, path, want, a, b, d)
			}
		case "strict":
			raw, err := parser.Parse(ctx, src, parser.ParseOptions{})
			if err != nil || raw.Tree == nil || len(raw.Errors) != 0 {
				t.Fatalf("%s: strict parse %v %v", row.id, raw.Errors, err)
			}
		case "trail":
			rawCheckTrailing(t, row.id, src, val)
		case "checktail":
			if val != "root" {
				t.Fatalf("%s: checktail=%s", row.id, val)
			}
			checkTailIndependent(t, row.id, src)
		case "extract":
			rawCheckExtract(t, row.id, src, val)
		case "extractlex":
			rawCheckExtractLexical(t, row.id, src, val)
		case "token":
			a, b := intPair(t, row.id, val)
			toks, _, err := Tokens(ctx, "M.tla", src)
			found := false
			for _, tok := range toks {
				found = found || (tok.Span.Start.ByteOffset == a && tok.Span.End.ByteOffset == b)
			}
			if err != nil || !found {
				t.Fatalf("%s: no token over [%d,%d): %+v %v", row.id, a, b, toks, err)
			}
		case "fallback":
			l, c, rng := lineColumnAt(t, row.id, val)
			a, b := intPair(t, row.id, rng)
			_, _, coords, err := scan(ctx, "M.tla", src)
			if err != nil {
				t.Fatal(err)
			}
			sp := diagSpan(positions(src), src, coords, &lexer.LexicalError{Position: lexer.Position{Line: l, Column: c}})
			if sp.Start.ByteOffset != a || sp.End.ByteOffset != b {
				t.Fatalf("%s: fallback span [%d,%d), want [%d,%d)", row.id, sp.Start.ByteOffset, sp.End.ByteOffset, a, b)
			}
		default:
			t.Fatalf("%s: unknown expectation %q", row.id, e)
		}
	}
}

// rawCheckTrailing checks a root whose strict parse is clean and whose only
// scan diagnostics lie after the root's closing delimiter: Parse completes
// without errors and reports exactly the wanted informational trailing
// diagnostics. The raw scan keeps them as lexical errors with the same spans
// and messages, and Check carries the same trailing diagnostics.
func rawCheckTrailing(t *testing.T, id string, src []byte, val string) {
	t.Helper()
	ctx := context.Background()
	end := provedRootEnd(t, id, src)
	r, err := Parse(ctx, "M.tla", src, Options{})
	if err != nil || r.Completion != CompletionComplete || hasErrors(r.Diagnostics) {
		t.Fatalf("%s: parse %+v %v", id, r, err)
	}
	spans := strings.Split(val, ",")
	if len(r.Diagnostics) != len(spans) {
		t.Fatalf("%s: want %d trailing diagnostics, got %+v", id, len(spans), r.Diagnostics)
	}
	_, scanned, err := Tokens(ctx, "M.tla", src)
	if err != nil || len(scanned) != len(spans) {
		t.Fatalf("%s: scan %+v %v", id, scanned, err)
	}
	for i, s := range spans {
		a, b := intPair(t, id, s)
		d, raw := r.Diagnostics[i], scanned[i]
		if d.Code != CodeTrailingLexical || d.Severity != SeverityInfo || d.Phase != LevelLexical || d.Path != "M.tla" || d.Span.Start.ByteOffset != a || d.Span.End.ByteOffset != b || a < end {
			t.Fatalf("%s: want trailing [%d,%d) after %d, got %+v", id, a, b, end, d)
		}
		if raw.Code != CodeLexical || raw.Severity != SeverityError || raw.Span != d.Span || raw.Message != d.Message || !strings.HasPrefix(d.Message, "Lexical error at line ") {
			t.Fatalf("%s: scan %+v, parse %+v", id, raw, d)
		}
	}
	c, err := Check(ctx, Request{Path: "M.tla", Text: src})
	if err != nil {
		t.Fatalf("%s: check %v", id, err)
	}
	if got := trailingOnly(c.Diagnostics); !slices.Equal(got, r.Diagnostics) {
		t.Fatalf("%s: check trailing %+v, parse %+v", id, got, r.Diagnostics)
	}
}

// provedRootEnd returns the proved byte offset just past the root module's
// closing delimiter.
func provedRootEnd(t *testing.T, id string, src []byte) int {
	t.Helper()
	ctx := context.Background()
	toks, _, coords, err := scan(ctx, "M.tla", src)
	if err != nil {
		t.Fatalf("%s: scan %v", id, err)
	}
	raw, err := parser.Parse(ctx, src, parser.ParseOptions{})
	if err != nil || !raw.Accepted {
		t.Fatalf("%s: strict parse %v %v", id, raw.Errors, err)
	}
	end, ok := rootEndBoundary(raw.Tree, src, coords, toks)
	if !ok {
		t.Fatalf("%s: root end not proved", id)
	}
	return end
}

func trailingOnly(ds []Diagnostic) []Diagnostic {
	var out []Diagnostic
	for _, d := range ds {
		if d.Code == CodeTrailingLexical {
			out = append(out, d)
		}
	}
	return out
}

func withoutTrailing(ds []Diagnostic) []Diagnostic {
	var out []Diagnostic
	for _, d := range ds {
		if d.Code != CodeTrailingLexical {
			out = append(out, d)
		}
	}
	return out
}

// checkTailIndependent: Check's verdict and level depend only on the region
// Java tokenizes. Check of the whole root equals Check of the root cut at its
// proved end, apart from the informational trailing diagnostics, which must
// be present.
func checkTailIndependent(t *testing.T, id string, src []byte) {
	t.Helper()
	ctx := context.Background()
	end := provedRootEnd(t, id, src)
	full, err := Check(ctx, Request{Path: "M.tla", Text: src})
	if err != nil {
		t.Fatalf("%s: check %v", id, err)
	}
	cut, err := Check(ctx, Request{Path: "M.tla", Text: src[:end]})
	if err != nil {
		t.Fatalf("%s: cut check %v", id, err)
	}
	if len(trailingOnly(full.Diagnostics)) == 0 || len(trailingOnly(cut.Diagnostics)) != 0 {
		t.Fatalf("%s: trailing full %+v, cut %+v", id, full.Diagnostics, cut.Diagnostics)
	}
	if full.Level != cut.Level || full.Completion != cut.Completion || !slices.Equal(withoutTrailing(full.Diagnostics), cut.Diagnostics) {
		t.Fatalf("%s: full %+v, cut at %d %+v", id, full, end, cut)
	}
}

// rawCheckExtract checks one monolith extraction: raw line bytes are copied
// unchanged plus LF, every segment maps back to the owner (terminator
// included), and the product decoder sees the same groups in the extracted
// text as in the owner's lines.
func rawCheckExtract(t *testing.T, id string, src []byte, val string) {
	t.Helper()
	if name, none := strings.CutSuffix(val, ":none"); none {
		budget := 1 << 20
		_, found, err := modulegraph.ExtractMonolith(context.Background(), modulegraph.Input{Path: "M.tla", Text: src}, name, &budget)
		if err != nil || found {
			t.Fatalf("%s: %s should not extract: %v %v", id, name, found, err)
		}
		return
	}
	name, rng, _ := strings.Cut(val, "@")
	a, b := intPair(t, id, rng)
	out := rawExtract(t, id, src, name)
	segs := out.View.Segments
	if len(segs) == 0 || segs[0].SourceStart != a || segs[len(segs)-1].SourceNext != b {
		t.Fatalf("%s: segments %+v, want owner [%d,%d)", id, segs, a, b)
	}
	var want []byte
	for _, s := range segs {
		want = append(append(want, src[s.SourceStart:s.SourceEnd]...), '\n')
		x, y, ok := out.View.MapSpan(s.ParsedEnd-1, s.ParsedEnd)
		if !ok || x != s.SourceEnd || y != s.SourceNext {
			t.Fatalf("%s: terminator of %+v maps to [%d,%d) %v", id, s, x, y, ok)
		}
	}
	if !bytes.Equal(out.Text, want) {
		t.Fatalf("%s: extracted %q, want %q", id, out.Text, want)
	}
	owner := productGroups(src)
	for _, g := range productGroups(out.Text) {
		if out.Text[g.start] == '\n' {
			continue
		}
		x, y, ok := out.View.MapSpan(g.start, g.end)
		if !ok {
			t.Fatalf("%s: group %+v does not map", id, g)
		}
		match := false
		for _, o := range owner {
			match = match || (o.start == x && o.end == y && o.scalar == g.scalar)
		}
		if !match {
			t.Fatalf("%s: extracted group %+v maps to [%d,%d), not an owner group", id, g, x, y)
		}
	}
}

func rawExtract(t *testing.T, id string, src []byte, name string) modulegraph.Input {
	t.Helper()
	budget := 1 << 20
	out, found, err := modulegraph.ExtractMonolith(context.Background(), modulegraph.Input{Path: "M.tla", Text: src}, name, &budget)
	if err != nil || !found || out.View == nil {
		t.Fatalf("%s: extract %s: %v %v", id, name, found, err)
	}
	return out
}

// rawCheckExtractLexical parses an extracted module and maps its lexical
// diagnostic back to the owner, as acquisition does for a failed unit.
func rawCheckExtractLexical(t *testing.T, id string, src []byte, val string) {
	t.Helper()
	name, pos, _ := strings.Cut(val, ":")
	l, c, rng := lineColumnAt(t, id, pos)
	a, b := intPair(t, id, rng)
	out := rawExtract(t, id, src, name)
	ctx := context.Background()
	raw, err := parser.Parse(ctx, out.Text, parser.ParseOptions{})
	if err != nil || len(raw.Errors) == 0 {
		t.Fatalf("%s: extracted parse %v %v", id, raw.Errors, err)
	}
	ds, err := failedParseDiagnostics(ctx, &modulegraph.Document{Name: name, Input: out, Parse: raw})
	if err != nil || len(ds) == 0 {
		t.Fatalf("%s: diagnostics %+v %v", id, ds, err)
	}
	d := ds[0]
	want := fmt.Sprintf("Lexical error at line %d, column %d.", l, c)
	if d.Code != CodeLexical || d.Path != "M.tla" || !strings.HasPrefix(d.Message, want) || d.Span.Start.ByteOffset != a || d.Span.End.ByteOffset != b {
		t.Fatalf("%s: want %q over owner [%d,%d), got %+v", id, want, a, b, d)
	}
}

// rawCheckResolverLikeRoot: a resolver source is decoded exactly like a root.
func rawCheckResolverLikeRoot(t *testing.T, row rawRow) {
	ctx := context.Background()
	direct, err := Parse(ctx, "N.tla", row.bytes, Options{})
	if err != nil {
		t.Fatal(err)
	}
	c, err := Check(ctx, Request{Path: "M.tla", Text: []byte(rawUTF8Resolver), Resolve: rawResolver(row.bytes)})
	if err != nil {
		t.Fatal(err)
	}
	var viaResolver []Diagnostic
	for _, d := range c.Diagnostics {
		if d.Path == "N.tla" {
			viaResolver = append(viaResolver, d)
		}
	}
	if len(direct.Diagnostics) == 0 || len(viaResolver) != len(direct.Diagnostics) {
		t.Fatalf("direct %+v, resolver %+v", direct.Diagnostics, viaResolver)
	}
	for i, d := range direct.Diagnostics {
		v := viaResolver[i]
		if d.Code != v.Code || d.Message != v.Message || d.Span != v.Span {
			t.Fatalf("direct %+v, resolver %+v", d, v)
		}
	}
}

// rawCheckIdentityClass: FF, EF BF BD and C0 decode to the same text, so the
// analysis output is identical apart from byte offsets.
func rawCheckIdentityClass(t *testing.T, byID map[string]rawRow) {
	ctx := context.Background()
	var first []string
	var level AnalysisLevel
	for i, id := range []string{"I2-ff", "I2-efbfbd", "I2-c0"} {
		row, ok := byID[id]
		if !ok {
			t.Fatalf("missing %s", id)
		}
		c, err := Check(ctx, Request{Path: "M.tla", Text: row.bytes})
		if err != nil {
			t.Fatal(err)
		}
		var msgs []string
		for _, d := range c.Diagnostics {
			msgs = append(msgs, string(d.Phase)+"|"+d.Code+"|"+d.Message)
		}
		if i == 0 {
			first, level = msgs, c.Level
		} else if !slices.Equal(first, msgs) || level != c.Level {
			t.Fatalf("%s: %v/%s, want %v/%s", id, msgs, c.Level, first, level)
		}
	}
}

// rawCheckSupplementaryFallback forces the lexical-error span fallback on both
// UTF-16 units of a supplementary scalar.
func rawCheckSupplementaryFallback(t *testing.T, _ rawRow) {
	src, _, _ := rawTemplate("K", []byte{0xf0, 0x9f, 0x98, 0x80})
	_, _, coords, err := scan(context.Background(), "M.tla", src)
	if err != nil {
		t.Fatal(err)
	}
	for _, col := range []int{6, 7} {
		sp := diagSpan(positions(src), src, coords, &lexer.LexicalError{Position: lexer.Position{Line: 2, Column: col}})
		if sp.Start.ByteOffset != 24 || sp.End.ByteOffset != 28 {
			t.Fatalf("column %d: [%d,%d)", col, sp.Start.ByteOffset, sp.End.ByteOffset)
		}
	}
}

// cancelAfter reports cancellation from its n-th Err call onwards.
type cancelAfter struct {
	context.Context
	n int
}

func (c *cancelAfter) Err() error {
	c.n--
	if c.n <= 0 {
		return context.Canceled
	}
	return nil
}

func TestRawUTF8MonolithCancellation(t *testing.T) {
	root := []byte("---- MODULE M ----\nEXTENDS N\n\\* \xff\nx == y\n====\n---- MODULE N ----\ny == 1\n====\n")
	budget := 1 << 20
	// Cancellation between lines, after the malformed line has been read.
	ctx := &cancelAfter{Context: context.Background(), n: 6}
	if _, _, err := modulegraph.ExtractMonolith(ctx, modulegraph.Input{Text: root}, "N", &budget); !errors.Is(err, context.Canceled) {
		t.Fatalf("extract: %v", err)
	}
	if spent := 1<<20 - budget; spent < bytes.Index(root, []byte("x ==")) || spent >= bytes.Index(root, []byte("---- MODULE N")) {
		t.Fatalf("cancelled after %d bytes", spent)
	}
	// Check stops with the caller's error when cancelled inside acquisition.
	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := Check(cctx, Request{Path: "M.tla", Text: root, Resolve: func(context.Context, string) (Source, error) {
		cancel()
		return Source{}, ErrModuleNotFound
	}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("check: %v", err)
	}
}

func TestRawUTF8MonolithDepth(t *testing.T) {
	nest := strings.Repeat("(", 80) + "1" + strings.Repeat(")", 80)
	root := []byte("---- MODULE M ----\nEXTENDS D\n\\* \xff\n====\n---- MODULE D ----\nX == " + nest + "\n====\n")
	direct := []byte("---- MODULE D ----\nX == " + nest + "\n====\n")
	opts := Options{MaxDepth: 30}
	r, err := Check(context.Background(), Request{Path: "M.tla", Text: root, Options: opts})
	if err != nil {
		t.Fatal(err)
	}
	d, err := Parse(context.Background(), "D.tla", direct, opts)
	if err != nil {
		t.Fatal(err)
	}
	got, want := firstDiagnostic(r.Diagnostics, CodeDepthLimit), firstDiagnostic(d.Diagnostics, CodeDepthLimit)
	shift := bytes.Index(root, []byte("---- MODULE D"))
	if got == nil || want == nil || got.Path != "M.tla" || got.Message != want.Message || got.Span.Start.ByteOffset != want.Span.Start.ByteOffset+shift || got.Span.End.ByteOffset != want.Span.End.ByteOffset+shift || r.Completion != CompletionIncomplete {
		t.Fatalf("monolith %+v, direct %+v", r.Diagnostics, d.Diagnostics)
	}
}

func TestRawUTF8MonolithBudgetIsSticky(t *testing.T) {
	root := []byte("---- MODULE M ----\n\\* \xff\xe2\x82\n====\n---- MODULE N ----\ny == 1\n====\n")
	header := bytes.Index(root, []byte("---- MODULE N"))
	budget := header + 3
	for i := 0; i < 2; i++ {
		before := budget
		_, _, err := modulegraph.ExtractMonolith(context.Background(), modulegraph.Input{Text: root}, "N", &budget)
		if !errors.Is(err, modulegraph.ErrIncomplete) || budget > before {
			t.Fatalf("attempt %d: %v, budget %d -> %d", i, err, before, budget)
		}
	}
	if budget >= header {
		t.Fatalf("budget %d was not spent on the skipped lines", budget)
	}
}
