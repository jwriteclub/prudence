// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package prudence

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	parser "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

// Java SANY never tokenizes text after the root module's EndModule
// (TLAplusParser.java:83, :1458-1472). These tests pin that such text yields
// only informational CodeTrailingLexical diagnostics and never blocks Check,
// while genuine errors before the real root end still block.

type trailingCase struct {
	name string
	src  string
	// trail lists the bad byte sequences after the root end, in order; each
	// must produce exactly one trailing diagnostic over its first occurrence
	// at or after the proved end. Nil with anyTrail unset means none.
	trail    []string
	anyTrail bool
	minTrail int // With anyTrail, the least number of trailing diagnostics.
	// end is the expected proved boundary, or -1 when it must be unproved.
	end int
	// check is the expected Check outcome: "semantic", "semantic-error" or
	// the code of the first blocking error.
	check string
}

func trailingCases() []trailingCase {
	return []trailingCase{
		{name: "ascii-tail", src: "---- MODULE M ----\nx == 1\n====\nfree text, x == @ and ---- more\n", end: 30, check: "semantic"},
		// The post-module lexer state still recognizes comments: an open one
		// reaches EOF, and malformed bytes inside closed ones are not errors.
		{name: "open-comment-tail", src: "---- MODULE M ----\nx == 1\n====\nfree text (* never closed\n", anyTrail: true, minTrail: 1, end: 30, check: "semantic"},
		{name: "same-line", src: "---- MODULE M ----\nx == 1\n====\xff\n", trail: []string{"\xff"}, end: 30, check: "semantic"},
		{name: "same-line-space", src: "---- MODULE M ----\nx == 1\n====  \xe2\x82 x\n", trail: []string{"\xe2\x82"}, end: 30, check: "semantic"},
		{name: "long-delimiter", src: "---- MODULE M ----\nx == 1\n=======\xed\xa0\x80\n", trail: []string{"\xed\xa0\x80"}, end: 33, check: "semantic"},
		{name: "comment-shaped-tail", src: "---- MODULE M ----\nx == 1\n====\n(* \xff *)\n\\* \xc2\x80\n", end: 30, check: "semantic"},
		{name: "valid-non-ascii-tail", src: "---- MODULE M ----\nx == 1\n====\n\xe2\x89\xba \xf0\x9f\x98\x80\n", anyTrail: true, end: 30, check: "semantic"},
		{name: "bom-tail", src: "---- MODULE M ----\nx == 1\n====\n\xef\xbb\xbf\n", anyTrail: true, end: 30, check: "semantic"},
		{name: "eof-group", src: "---- MODULE M ----\nx == 1\n====\xf0\x9f\x98", trail: []string{"\xf0\x9f\x98"}, end: 30, check: "semantic"},
		{name: "crlf", src: "---- MODULE M ----\r\nx == 1\r\n====\r\n\xff\r\n", trail: []string{"\xff"}, end: 32, check: "semantic"},
		{name: "cr", src: "---- MODULE M ----\rx == 1\r====\r\xff\r", trail: []string{"\xff"}, end: 30, check: "semantic"},
		// An inner module's terminator is not the root boundary.
		{name: "inner-then-tail", src: "---- MODULE M ----\n---- MODULE Inner ----\nA == 1\n====\nB == 2\n====\n\xff\n", trail: []string{"\xff"}, end: 65, check: "semantic"},
		// A textual ==== in a string or comment is not the boundary.
		{name: "textual-delimiters", src: "---- MODULE M ----\nx == \"====\xff\"\n(* ==== \xff *)\n====\n\xfe\n", trail: []string{"\xfe"}, end: 49, check: "semantic"},
		// Later monolith modules and markers do not move the boundary.
		{name: "later-markers", src: "---- MODULE M ----\n====\n---- MODULE N ----\nz == \xff\n====\n\xfe\n---- MODULE P ----\n====\n", trail: []string{"\xff", "\xfe"}, end: 23, check: "semantic"},
		{name: "semantic-error-kept", src: "---- MODULE M ----\nx == y\n====\n\xff\n", trail: []string{"\xff"}, end: 30, check: "semantic-error"},
		// Genuine errors before the real root end keep blocking.
		{name: "inner-before-end-error", src: "---- MODULE M ----\n---- MODULE Inner ----\nA == 1\n====\nB == \xff\n====\n\xfe\n", end: -1, check: CodeLexical},
		{name: "lexical-before-end", src: "---- MODULE M ----\nx == \xff\n====\n\xfe\n", end: -1, check: CodeLexical},
		{name: "syntax-before-end", src: "---- MODULE M ----\nx == (1\n====\n\xff\n", end: -1, check: CodeSyntax},
		{name: "incomplete-root", src: "---- MODULE M ----\nx == 1\n\xff\n", end: -1, check: CodeLexical},
		{name: "incomplete-root-ascii", src: "---- MODULE M ----\nx == 1\n", end: -1, check: CodeSyntax},
		{name: "open-comment-before-end", src: "---- MODULE M ----\nx == 1\n(* open\n====\n\xff", end: -1, check: CodeLexical},
	}
}

func TestTrailingTextAdmission(t *testing.T) {
	ctx := context.Background()
	for _, tc := range trailingCases() {
		t.Run(tc.name, func(t *testing.T) {
			src := []byte(tc.src)
			keep := append([]byte(nil), src...)
			toks, _, coords, err := scan(ctx, "M.tla", src)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := parser.Parse(ctx, src, parser.ParseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			end, proved := rootEndBoundary(raw.Tree, src, coords, toks)
			if raw.Accepted != (tc.end >= 0) || proved != (tc.end >= 0) || (proved && end != tc.end) {
				t.Fatalf("boundary %d %v (accepted %v), want %d", end, proved, raw.Accepted, tc.end)
			}
			r, err := Parse(ctx, "M.tla", src, Options{IncludeTokens: true})
			if err != nil {
				t.Fatal(err)
			}
			again, err := Parse(ctx, "M.tla", src, Options{IncludeTokens: true})
			if err != nil || !reflect.DeepEqual(trailingView(r), trailingView(again)) {
				t.Fatalf("repeat parse differs: %v", err)
			}
			// Token coverage is the raw scan's, unchanged by classification.
			if !slices.Equal(r.Tokens, toks) {
				t.Fatalf("tokens %+v, scan %+v", r.Tokens, toks)
			}
			c, err := Check(ctx, Request{Path: "M.tla", Text: src})
			if err != nil {
				t.Fatal(err)
			}
			c2, err := Check(ctx, Request{Path: "M.tla", Text: src})
			if err != nil || !reflect.DeepEqual(trailingView(c), trailingView(c2)) {
				t.Fatalf("repeat check differs: %v", err)
			}
			if !bytes.Equal(src, keep) {
				t.Fatal("caller bytes changed")
			}
			if tc.end < 0 {
				// Unproved boundary: the original behavior, with no trailing code.
				if len(trailingOnly(r.Diagnostics)) != 0 || len(trailingOnly(c.Diagnostics)) != 0 {
					t.Fatalf("trailing code without a proved boundary: %+v", r.Diagnostics)
				}
				d := firstError(c.Diagnostics)
				if d == nil || d.Code != tc.check || c.Level != LevelSyntax || !hasErrors(r.Diagnostics) {
					t.Fatalf("want blocking %s, got %+v", tc.check, c)
				}
				return
			}
			if r.Completion != CompletionComplete || hasErrors(r.Diagnostics) {
				t.Fatalf("parse %+v", r)
			}
			got := trailingOnly(r.Diagnostics)
			if len(got) != len(r.Diagnostics) {
				t.Fatalf("non-trailing parse diagnostics %+v", r.Diagnostics)
			}
			if tc.anyTrail {
				if len(got) < tc.minTrail {
					t.Fatalf("want at least %d trailing, got %+v", tc.minTrail, got)
				}
				for _, d := range got {
					if d.Span.Start.ByteOffset < end {
						t.Fatalf("trailing before end: %+v", d)
					}
				}
			} else {
				if len(got) != len(tc.trail) {
					t.Fatalf("want %d trailing, got %+v", len(tc.trail), got)
				}
				for i, bad := range tc.trail {
					a := end + bytes.Index(src[end:], []byte(bad))
					if got[i].Span.Start.ByteOffset != a || got[i].Span.End.ByteOffset != a+len(bad) || got[i].Severity != SeverityInfo || got[i].Phase != LevelLexical {
						t.Fatalf("trailing %d: want [%d,%d), got %+v", i, a, a+len(bad), got[i])
					}
				}
			}
			if !slices.Equal(trailingOnly(c.Diagnostics), got) {
				t.Fatalf("check trailing %+v, parse %+v", c.Diagnostics, got)
			}
			// The outline and every non-trailing result equal the cut root's.
			cutParse, err := Parse(ctx, "M.tla", src[:end], Options{})
			if err != nil || !reflect.DeepEqual(cutParse.Outline, r.Outline) {
				t.Fatalf("outline %+v, cut %+v %v", r.Outline, cutParse.Outline, err)
			}
			cut, err := Check(ctx, Request{Path: "M.tla", Text: src[:end]})
			if err != nil || cut.Level != c.Level || cut.Completion != c.Completion || !slices.Equal(withoutTrailing(c.Diagnostics), cut.Diagnostics) {
				t.Fatalf("check %+v, cut %+v %v", c, cut, err)
			}
			switch tc.check {
			case "semantic":
				if c.Level != LevelComplete || c.Completion != CompletionComplete || hasErrors(c.Diagnostics) {
					t.Fatalf("want semantic without errors, got %+v", c)
				}
			case "semantic-error":
				d := firstError(c.Diagnostics)
				if c.Level != LevelSemantic || d == nil || d.Phase != LevelSemantic {
					t.Fatalf("want a semantic error, got %+v", c)
				}
			default:
				t.Fatalf("bad case outcome %q", tc.check)
			}
			// A syntax-stage request completes with the same diagnostics.
			s, err := Check(ctx, Request{Path: "M.tla", Text: src, Stage: LevelSyntax})
			if err != nil || s.Completion != CompletionComplete || !slices.Equal(s.Diagnostics, r.Diagnostics) {
				t.Fatalf("syntax stage %+v %v", s, err)
			}
		})
	}
}

// trailingView drops the opaque tree pointer so repeated results compare.
func trailingView(r Result) Result {
	r.Tree = nil
	return r
}

func firstError(ds []Diagnostic) *Diagnostic {
	for i := range ds {
		if ds[i].Severity == SeverityError {
			return &ds[i]
		}
	}
	return nil
}

// TestTrailingBoundaryUnproved: without an exactly matching tree node and
// delimiter token, no boundary is claimed and Parse keeps blocking errors.
func TestTrailingBoundaryUnproved(t *testing.T) {
	ctx := context.Background()
	src := []byte("---- MODULE M ----\nx == 1\n====\n\xff\n")
	toks, _, coords, err := scan(ctx, "M.tla", src)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := parser.Parse(ctx, src, parser.ParseOptions{})
	if err != nil || raw.Tree == nil {
		t.Fatal(err)
	}
	if end, ok := rootEndBoundary(raw.Tree, src, coords, toks); !ok || end != 30 {
		t.Fatalf("baseline boundary %d %v", end, ok)
	}
	if _, ok := rootEndBoundary(nil, src, coords, toks); ok {
		t.Fatal("nil tree proved")
	}
	if _, ok := rootEndBoundary(raw.Tree, src, coords, nil); ok {
		t.Fatal("missing delimiter token proved")
	}
	if _, ok := rootEndBoundary(raw.Tree, src, coordIndex{}, toks); ok {
		t.Fatal("missing coordinates proved")
	}
	shifted := slices.Clone(toks)
	for i := range shifted {
		if shifted[i].Span.Start.ByteOffset == 26 {
			shifted[i].Span.End.ByteOffset--
		}
	}
	if _, ok := rootEndBoundary(raw.Tree, src, coords, shifted); ok {
		t.Fatal("mismatched delimiter token proved")
	}
	other := slices.Clone(src)
	other[27] = '-'
	if _, ok := rootEndBoundary(raw.Tree, other, coords, toks); ok {
		t.Fatal("non-delimiter bytes proved")
	}
	for _, n := range raw.Tree.Heirs() {
		if n != nil && n.Kind() != parser.N_EndModule {
			if _, ok := rootEndBoundary(n, src, coords, toks); ok {
				t.Fatalf("non-module node %d proved", n.Kind())
			}
		}
	}
	// classifyTrailing changes only lexical diagnostics at or after the end,
	// and never its input.
	in := []Diagnostic{
		{Phase: LevelLexical, Severity: SeverityError, Code: CodeLexical, Span: Span{Start: Position{ByteOffset: 29}}},
		{Phase: LevelLexical, Severity: SeverityError, Code: CodeLexical, Span: Span{Start: Position{ByteOffset: 30}}},
		{Phase: LevelSyntax, Severity: SeverityError, Code: CodeSyntax, Span: Span{Start: Position{ByteOffset: 31}}},
	}
	before := slices.Clone(in)
	out := classifyTrailing(in, 30)
	if !slices.Equal(in, before) || out[0] != in[0] || out[2] != in[2] || out[1].Code != CodeTrailingLexical || out[1].Severity != SeverityInfo || out[1].Message != in[1].Message || out[1].Span != in[1].Span {
		t.Fatalf("classify %+v from %+v", out, in)
	}
}

// TestTrailingTextLimits: cancellation, depth and size limits stay real limits.
func TestTrailingTextLimits(t *testing.T) {
	tail := "\n====\n\xff\n"
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Parse(cancelled, "M.tla", []byte("---- MODULE M ----\nx == 1"+tail), Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("parse: %v", err)
	}
	if _, err := Check(cancelled, Request{Path: "M.tla", Text: []byte("---- MODULE M ----\nx == 1" + tail)}); !errors.Is(err, context.Canceled) {
		t.Fatalf("check: %v", err)
	}
	// The trailing text no longer stops Check before acquisition, which then
	// honours cancellation from the resolver.
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	called := false
	_, err := Check(ctx, Request{Path: "M.tla", Text: []byte("---- MODULE M ----\nEXTENDS N" + tail), Resolve: func(context.Context, string) (Source, error) {
		called = true
		stop()
		return Source{}, ErrModuleNotFound
	}})
	if !called || !errors.Is(err, context.Canceled) {
		t.Fatalf("resolver called %v, err %v", called, err)
	}
	nest := strings.Repeat("(", 80) + "1" + strings.Repeat(")", 80)
	deep := []byte("---- MODULE M ----\nX == " + nest + tail)
	r, err := Parse(context.Background(), "M.tla", deep, Options{MaxDepth: 30})
	if err != nil || firstDiagnostic(r.Diagnostics, CodeDepthLimit) == nil || r.Completion != CompletionIncomplete || len(trailingOnly(r.Diagnostics)) != 0 {
		t.Fatalf("depth parse %+v %v", r, err)
	}
	c, err := Check(context.Background(), Request{Path: "M.tla", Text: deep, Options: Options{MaxDepth: 30}})
	if err != nil || firstDiagnostic(c.Diagnostics, CodeDepthLimit) == nil || c.Level != LevelSyntax || c.Completion != CompletionIncomplete {
		t.Fatalf("depth check %+v %v", c, err)
	}
	big := append([]byte("---- MODULE M ----\n===="), bytes.Repeat([]byte{0xff}, 32<<20)...)
	c, err = Check(context.Background(), Request{Path: "M.tla", Text: big})
	if err != nil || c.Completion != CompletionIncomplete || len(c.Diagnostics) != 1 || c.Diagnostics[0].Code != CodeIncomplete || c.Diagnostics[0].Message != "source size limit exceeded" {
		t.Fatalf("size limit %+v %v", c.Diagnostics, err)
	}
}

// TestColumnPolicyPerByte pins Chris's decision: public Column and
// UTF16Column keep Go per-byte replacement inside malformed groups, while
// ByteOffset and the Java line/column in messages follow the lexer's Java
// decoder groups. Both contracts are checked on the same diagnostic.
func TestColumnPolicyPerByte(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name                string
		x                   string
		javaColumn          int // `@` column in the SANY message (1-based, tab 8).
		byteOffset          int // `@` ByteOffset with LF line endings.
		column, utf16Column int // Public per-byte columns of `@`.
	}{
		{"lone-ff", "\xff", 9, 27, 8, 8},
		{"truncated-3", "\xe2\x82 ", 10, 29, 10, 10},
		{"surrogate-group", "\xed\xa0\x80", 9, 29, 10, 10},
		{"two-surrogate-groups", "\xed\xa0\xbd\xed\xb8\x80", 10, 32, 13, 13},
		{"truncated-4", "\xf0\x90\x80 ", 10, 30, 11, 11},
		{"supplementary", "\xf0\x9f\x98\x80", 10, 30, 8, 9},
		{"truncated-tab", "\xe2\x82\t", 13, 29, 10, 10},
	} {
		for _, nl := range []string{"\n", "\r\n", "\r"} {
			src := []byte("---- MODULE M ----" + nl + "(* " + tc.x + " *) @" + nl + "====" + nl)
			r, err := Parse(ctx, "M.tla", src, Options{})
			if err != nil {
				t.Fatal(err)
			}
			d := firstDiagnostic(r.Diagnostics, CodeSyntax)
			want := "Encountered \"@\" at line 2, column " + strconv.Itoa(tc.javaColumn) + " "
			at := tc.byteOffset + len(nl) - 1
			if d == nil || !strings.Contains(d.Message, want) {
				t.Fatalf("%s %q: want %q, got %+v", tc.name, nl, want, r.Diagnostics)
			}
			p := d.Span.Start
			if p.ByteOffset != at || p.Line != 1 || p.Column != tc.column || p.UTF16Column != tc.utf16Column || d.Span.End.ByteOffset != at+1 {
				t.Fatalf("%s %q: @ at %+v, want byte %d line 1 column %d/%d", tc.name, nl, p, at, tc.column, tc.utf16Column)
			}
		}
	}
	// A two-byte malformed group is one Java character but two public columns;
	// a supplementary scalar is one code point and two UTF-16 units.
	for _, tc := range []struct {
		x                    string
		end, endCol, endUnit int
	}{
		{"\xe2\x82", 26, 7, 7},
		{"\xf0\x9f\x98\x80", 28, 6, 7},
	} {
		src := []byte("---- MODULE M ----\nx == " + tc.x + "\n====\n")
		r, err := Parse(ctx, "M.tla", src, Options{})
		if err != nil || len(r.Diagnostics) == 0 {
			t.Fatalf("%q: %+v %v", tc.x, r, err)
		}
		d := r.Diagnostics[0]
		if d.Code != CodeLexical || !strings.HasPrefix(d.Message, "Lexical error at line 2, column 6.") || d.Span.Start != (Position{24, 1, 5, 5}) || d.Span.End != (Position{tc.end, 1, tc.endCol, tc.endUnit}) {
			t.Fatalf("%q: %+v", tc.x, d)
		}
	}
}
