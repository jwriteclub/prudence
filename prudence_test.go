// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package prudence

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jwriteclub/prudence/internal/recursion"
)

func TestParseAndCheck(t *testing.T) {
	src := []byte("---- MODULE Example ----\nEXTENDS Naturals\nVARIABLE x\nNext == x + 1\n====\n")
	called := false
	r, err := Check(context.Background(), Request{Stage: LevelSyntax, Path: "memory", Text: src, Options: Options{IncludeTokens: true}, Resolve: func(context.Context, string) (Source, error) { called = true; return Source{}, nil }})
	if err != nil || len(r.Diagnostics) > 0 || r.Tree == nil || r.Level != LevelSyntax || called {
		t.Fatalf("result=%+v error=%v resolver=%v", r, err, called)
	}
	if r.Outline.Module != "Example" || len(r.Outline.Extends) != 1 || r.Outline.Extends[0] != "Naturals" {
		t.Fatalf("outline=%+v", r.Outline)
	}
	if len(r.Tokens) == 0 {
		t.Fatal("missing tokens")
	}
	found := map[string]DeclKind{}
	for _, d := range r.Outline.Decls {
		found[d.Name] = d.Kind
	}
	if found["x"] != DeclVariable || found["Next"] != DeclDefinition {
		t.Fatalf("declarations %+v", r.Outline.Decls)
	}
}
func TestTokensPositionsAndRecovery(t *testing.T) {
	src := []byte("---- MODULE M ----\n\\* 😀 comment\n\tX == \"a\"\n`\nY == 2\n====")
	ts, ds, err := Tokens(context.Background(), "m", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) == 0 {
		t.Fatal("expected lexical diagnostic")
	}
	var x, y, comment bool
	for _, tok := range ts {
		s := string(src[tok.Span.Start.ByteOffset:tok.Span.End.ByteOffset])
		if strings.Contains(s, "comment") {
			comment = true
		}
		if s == "X" {
			x = true
			if tok.Span.Start.Column != 1 || tok.Span.Start.UTF16Column != 1 {
				t.Fatalf("tab position %+v", tok.Span)
			}
		}
		if s == "Y" {
			y = true
		}
	}
	if !x || !y || !comment {
		t.Fatalf("x=%v y=%v comment=%v", x, y, comment)
	}
}
func TestCancellationInvalidBytesAndDepth(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Tokens(ctx, "", nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, _, err := Tokens(context.Background(), "", []byte{'-', 0xff, 'x'}); err != nil {
		t.Fatal(err)
	}
	r, err := Parse(context.Background(), "m", []byte("---- MODULE M ----\nX == 1\n===="), Options{MaxDepth: 1})
	if err != nil || len(r.Diagnostics) == 0 || r.Diagnostics[0].Code != CodeDepthLimit {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestSyntaxDiagnosticSpan(t *testing.T) {
	src := []byte("---- MODULE M ----\nVARIABLE ,\n====")
	r, err := Parse(context.Background(), "label", src, Options{})
	if err != nil || len(r.Diagnostics) == 0 {
		t.Fatalf("%+v %v", r, err)
	}
	d := r.Diagnostics[0]
	if d.Path != "label" || d.Code != CodeSyntax || d.Span.Start.ByteOffset == 0 || d.Span.Start.ByteOffset > len(src) {
		t.Fatalf("%+v", d)
	}
}

func TestPartialOutlinePreservesCompletedBody(t *testing.T) {
	for _, tail := range []string{
		"Third ==\n",
		"THEOREM Later == TRUE\n<1>1. TRUE\n<1>2.\n",
	} {
		src := []byte("---- MODULE Partial ----\nEXTENDS Naturals\nVARIABLE x\nFirst == LET Fake == 2 IN Fake\nSecond == \"Hidden == 3\"\n\\* Commented == 4\n" + tail)
		r, err := Parse(context.Background(), "partial", src, Options{})
		if err != nil || len(r.Diagnostics) == 0 {
			t.Fatalf("%+v %v", r, err)
		}
		if r.Outline.Module != "Partial" || len(r.Outline.Extends) != 1 || r.Outline.Extends[0] != "Naturals" {
			t.Fatalf("outline %+v", r.Outline)
		}
		found := map[string]bool{}
		for _, d := range r.Outline.Decls {
			found[d.Name] = true
		}
		for _, name := range []string{"x", "First", "Second"} {
			if !found[name] {
				t.Errorf("missing %s in %+v", name, r.Outline)
			}
		}
		for _, name := range []string{"Fake", "Hidden", "Commented", "Third"} {
			if found[name] {
				t.Errorf("invented %s in %+v", name, r.Outline)
			}
		}
	}
}

func TestTokenCategories(t *testing.T) {
	src := append([]byte("---- MODULE M ----\nVARIABLE x\nF == [x |-> 123, y |-> \\hFF] /\\ TRUE + 1\nS == \"text\"\nTHEOREM T == TRUE\n\\* line\n(* block *)\n"), 0xff)
	src = append(src, []byte("\n====")...)
	ts, ds, err := Tokens(context.Background(), "m", src)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]TokenKind{"VARIABLE": TokenKeyword, "x": TokenIdentifier, "==": TokenOperator, "[": TokenDelimiter, "]": TokenDelimiter, "123": TokenNumber, `\hFF`: TokenNumber, `/\`: TokenOperator, "+": TokenOperator, `"text"`: TokenString, "THEOREM": TokenProofKeyword}
	seen := map[string]bool{}
	var line, block, invalid bool
	for _, tok := range ts {
		v := string(src[tok.Span.Start.ByteOffset:tok.Span.End.ByteOffset])
		if want, ok := expected[v]; ok {
			seen[v] = true
			if tok.Kind != want {
				t.Errorf("%q kind %v want %v", v, tok.Kind, want)
			}
		}
		if tok.Kind == TokenLineComment {
			line = true
		}
		if tok.Kind == TokenBlockComment {
			block = true
		}
		if tok.Kind == TokenInvalid {
			invalid = true
		}
	}
	for v := range expected {
		if !seen[v] {
			t.Errorf("missing %q", v)
		}
	}
	if !line || !block || !invalid || len(ds) == 0 {
		t.Fatalf("line=%v block=%v invalid=%v diagnostics=%+v", line, block, invalid, ds)
	}
}
func TestOperatorDefinitionsAndSubmoduleScope(t *testing.T) {
	src := []byte("---- MODULE Outer ----\na ++ b == a\n~ a == a\na ^+ == a\n---- MODULE Inner ----\nHidden == 1\n====\nVisible == 1\n====\n")
	r, err := Parse(context.Background(), "", src, Options{})
	if err != nil || len(r.Diagnostics) > 0 {
		t.Fatalf("%+v %v", r, err)
	}
	found := map[string]DeclKind{}
	for _, d := range r.Outline.Decls {
		found[d.Name] = d.Kind
	}
	for _, name := range []string{"++", "~", "^+"} {
		if found[name] != DeclOperator {
			t.Errorf("%s: %+v", name, r.Outline.Decls)
		}
	}
	if found["Inner"] != DeclSubmodule || found["Visible"] != DeclDefinition || found["Hidden"] != "" {
		t.Fatalf("scope %+v", r.Outline.Decls)
	}
}

func TestPinnedStandardModules(t *testing.T) {
	for _, tc := range []struct{ name, hash string }{{"Naturals", "73f2a68412039aa86f37ad08f0d0fef50fa8327d0ad8806f515548c75e9d9d84"}, {"Integers", "071da69e5eea383d361ef308ba12ecc931bf968fcea98175c3bca419bbd8cbf2"}, {"Sequences", "f613cffe14133769f0972b6f6e0139aaf0773004a51bac88ada382426e3eb7f4"}} {
		t.Run(tc.name, func(t *testing.T) {
			src, err := os.ReadFile("testdata/" + tc.name + ".tla")
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256(src)) != tc.hash {
				t.Fatal("fixture hash mismatch")
			}
			r, err := Parse(context.Background(), tc.name, src, Options{IncludeTokens: true})
			if err != nil || len(r.Diagnostics) != 0 || r.Tree == nil || r.Outline.Module != tc.name || len(r.Outline.Decls) < 2 || len(r.Tokens) == 0 {
				t.Fatalf("module=%s result=%+v error=%v", tc.name, r.Outline, err)
			}
			ts, ds, err := Tokens(context.Background(), tc.name, src)
			if err != nil || len(ds) != 0 || len(ts) != len(r.Tokens) {
				t.Fatalf("tokens=%d diagnostics=%+v error=%v", len(ts), ds, err)
			}
		})
	}
}
func TestJavaCoordinatesAndSyntaxOffset(t *testing.T) {
	src := []byte("---- MODULE M ----\r\n\tVARIABLE ,\r\n\\* 😀 ")
	src = append(src, 0xff)
	src = append(src, []byte("\r\n====")...)
	_, _, idx, err := scan(context.Background(), "", src)
	if err != nil {
		t.Fatal(err)
	}
	tab := strings.Index(string(src), "\t")
	if got := idx.offset(2, 8, false, len(src)); got != tab {
		t.Fatalf("tab offset %d want %d", got, tab)
	}
	emoji := strings.Index(string(src), "😀")
	for _, col := range []int{4, 5} {
		if got := idx.offset(3, col, false, len(src)); got != emoji {
			t.Errorf("surrogate column %d offset %d want %d", col, got, emoji)
		}
	}
	r, err := Parse(context.Background(), "m", src, Options{})
	if err != nil || len(r.Diagnostics) == 0 {
		t.Fatalf("%+v %v", r, err)
	}
	comma := strings.Index(string(src), ",")
	if r.Diagnostics[0].Span.Start.ByteOffset != comma {
		t.Fatalf("diagnostic=%+v comma=%d", r.Diagnostics[0], comma)
	}
}

func TestPartialOutlineDoesNotLeakSubmodule(t *testing.T) {
	src := []byte("---- MODULE Outer ----\nEXTENDS Naturals\nFirst == 1\n---- MODULE Inner ----\nLOCAL INSTANCE Naturals\nHidden == 2\n====\nThird ==\n")
	r, err := Parse(context.Background(), "partial", src, Options{})
	if err != nil || len(r.Diagnostics) == 0 {
		t.Fatalf("%+v %v", r, err)
	}
	if r.Outline.Module != "Outer" || len(r.Outline.Extends) != 1 || r.Outline.Extends[0] != "Naturals" {
		t.Fatalf("outline %+v", r.Outline)
	}
	found := map[string]DeclKind{}
	for _, d := range r.Outline.Decls {
		found[d.Name] = d.Kind
	}
	if found["First"] != DeclDefinition || found["Inner"] != DeclSubmodule || found["Hidden"] != "" || found["Third"] != "" || len(r.Outline.Instances) != 0 {
		t.Fatalf("outline %+v", r.Outline)
	}
}

func TestConstantOutline(t *testing.T) {
	src := []byte("---- MODULE M ----\nCONSTANT Procs\nCONSTANTS A, B(_, _), _ ++ _, ~ _\nVARIABLE x\n====\n")
	r, err := Parse(context.Background(), "", src, Options{})
	if err != nil || len(r.Diagnostics) != 0 {
		t.Fatalf("%+v %v", r.Diagnostics, err)
	}
	found := map[string]Decl{}
	for _, d := range r.Outline.Decls {
		found[d.Name] = d
	}
	for name, params := range map[string]int{"Procs": 0, "A": 0, "B": 2, "++": 2, "~": 1} {
		d, ok := found[name]
		if !ok || d.Kind != DeclConstant || d.Params != params || string(src[d.NameSpan.Start.ByteOffset:d.NameSpan.End.ByteOffset]) != name {
			t.Errorf("%s: %+v", name, d)
		}
	}
}
func TestDefinitionMarkerDiagnosticPosition(t *testing.T) {
	for _, src := range [][]byte{[]byte("---- MODULE M ----\nA == 1 +\nB == 2\n====\n"), []byte("---- MODULE M ----\r\n\\* 😀\r\n\tA == 1 +\r\n\tB == 2\r\n====\r\n")} {
		r, err := Parse(context.Background(), "m", src, Options{})
		if err != nil || len(r.Diagnostics) == 0 {
			t.Fatalf("%+v %v", r, err)
		}
		want := strings.Index(string(src), "B ==")
		got := r.Diagnostics[0].Span
		if got.Start.ByteOffset != want || got.End.ByteOffset != want+1 {
			t.Fatalf("span %+v want %d", got, want)
		}
	}
}
func TestLexicalRecoveryPreservesCompleteOutline(t *testing.T) {
	src := []byte("---- MODULE Outer ----\nEXTENDS Naturals\nSeen == 1\n---- MODULE Inner ----\nLOCAL INSTANCE Naturals\nHidden == 1\n====\nAlso == 2\n(* unterminated (* nesting *)\nLost == 1\n====\n")
	r, err := Parse(context.Background(), "m", src, Options{IncludeTokens: true})
	if err != nil || len(r.Diagnostics) == 0 || r.Diagnostics[0].Code != CodeLexical {
		t.Fatalf("%+v %v", r.Diagnostics, err)
	}
	found := map[string]DeclKind{}
	for _, d := range r.Outline.Decls {
		found[d.Name] = d.Kind
	}
	if r.Outline.Module != "Outer" || found["Seen"] != DeclDefinition || found["Also"] != DeclDefinition || found["Inner"] != DeclSubmodule || found["Hidden"] != "" || found["Lost"] != "" || len(r.Outline.Instances) != 0 {
		t.Fatalf("outline %+v", r.Outline)
	}
	if r.Diagnostics[0].Span.Start.ByteOffset == 0 {
		t.Fatalf("lexical span %+v", r.Diagnostics[0].Span)
	}
}
func TestModuleNotFoundContract(t *testing.T) {
	if !errors.Is(fmt.Errorf("lookup: %w", ErrModuleNotFound), ErrModuleNotFound) || CodeModuleNotFound != "module/not-found" {
		t.Fatal("sentinel contract")
	}
	called := false
	r, err := Check(context.Background(), Request{Stage: LevelSyntax, Text: []byte("---- MODULE M ----\nEXTENDS Missing\n===="), Resolve: func(context.Context, string) (Source, error) { called = true; return Source{}, ErrModuleNotFound }})
	if err != nil || called || len(r.Diagnostics) != 0 || r.Level != LevelSyntax {
		t.Fatalf("%+v %v called=%v", r, err, called)
	}
}

func TestConsumerUnterminatedComment(t *testing.T) {
	src := []byte("---- MODULE M ----\nSeen == 1\nAlso == 2\n(* unterminated (* nesting *)\nLost == 1\n====\n")
	r, err := Parse(context.Background(), "m", src, Options{})
	if err != nil || len(r.Diagnostics) == 0 {
		t.Fatalf("%+v %v", r.Diagnostics, err)
	}
	names := map[string]bool{}
	for _, d := range r.Outline.Decls {
		names[d.Name] = true
	}
	if !names["Seen"] || !names["Also"] || names["Lost"] {
		t.Fatalf("%+v", r.Outline)
	}
	_, ds, err := Tokens(context.Background(), "m", src)
	if err != nil || len(ds) == 0 || r.Diagnostics[0].Span != ds[0].Span {
		t.Fatalf("parse=%+v tokens=%+v err=%v", r.Diagnostics, ds, err)
	}
}

func TestLexicalPrefixFailureClassification(t *testing.T) {
	src := []byte("---- MODULE M ----\nX == 1\n(* bad")
	p := positions(src)
	ctx := context.Background()
	_, _, coords, err := scan(ctx, "m", src)
	if err != nil {
		t.Fatal(err)
	}
	lexical := Diagnostic{Severity: SeverityError, Code: CodeLexical, Path: "m"}
	result := Result{Diagnostics: []Diagnostic{lexical}}
	depth := &recursion.DepthError{Stage: "parser", Limit: 1, Line: 2, Column: 1}
	if err := recoveryFailure(ctx, &result, depth, "m", p, src, coords); err != nil || len(result.Diagnostics) != 2 || result.Diagnostics[0].Code != CodeLexical || result.Diagnostics[1].Code != CodeDepthLimit || result.Diagnostics[1].Span.Start.ByteOffset != strings.Index(string(src), "X ==") {
		t.Fatalf("%+v %v", result.Diagnostics, err)
	}
	operational := errors.New("recovery failure")
	result = Result{Diagnostics: []Diagnostic{lexical}}
	if err := recoveryFailure(ctx, &result, operational, "m", p, src, coords); !errors.Is(err, operational) || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != CodeLexical {
		t.Fatalf("%+v %v", result.Diagnostics, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := recoveryFailure(canceled, &result, operational, "m", p, src, coords); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestCheckPhaseZeroContract(t *testing.T) {
	src := []byte("---- MODULE M ----\nEXTENDS Missing\nX == 1\n====\n")
	called := false
	resolver := func(context.Context, string) (Source, error) { called = true; return Source{}, ErrModuleNotFound }
	for _, stage := range []AnalysisLevel{"", LevelSemantic, LevelComplete} {
		t.Run(string(stage), func(t *testing.T) {
			called = false
			r, err := Check(context.Background(), Request{Stage: stage, Path: "memory", Text: src, Resolve: resolver, Options: Options{IncludeTokens: true}})
			if err != nil || !called || r.Level != LevelSemantic || r.Completion != CompletionComplete || r.Tree == nil || len(r.Tokens) == 0 || r.Outline.Module != "M" {
				t.Fatalf("result=%+v err=%v called=%v", r, err, called)
			}
			// SpecObj's MODULE_FILE_CANNOT_BE_FOUND abort, spanning the reference.
			at := strings.Index(string(src), "Missing")
			if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "semantic/4220" || r.Diagnostics[0].Severity != SeverityError || r.Diagnostics[0].Phase != LevelSemantic || r.Diagnostics[0].Path != "memory" || r.Diagnostics[0].Message != "Cannot find source file for module Missing imported in module M." || r.Diagnostics[0].Span.Start.ByteOffset != at || r.Diagnostics[0].Span.End.ByteOffset != at+len("Missing") || r.Diagnostics[0].Span.Start.Line != 1 || r.Diagnostics[0].Span.Start.Column != 8 {
				t.Fatalf("diagnostics=%+v", r.Diagnostics)
			}
		})
	}
	parsed, err := Parse(context.Background(), "memory", src, Options{IncludeTokens: true})
	if err != nil || parsed.Completion != CompletionComplete {
		t.Fatalf("parse=%+v err=%v", parsed, err)
	}
	called = false
	checked, err := Check(context.Background(), Request{Stage: LevelSyntax, Path: "memory", Text: src, Options: Options{IncludeTokens: true}, Resolve: resolver})
	if err != nil || called || checked.Level != LevelSyntax || checked.Completion != CompletionComplete || !reflect.DeepEqual(checked.Diagnostics, parsed.Diagnostics) || !reflect.DeepEqual(checked.Tokens, parsed.Tokens) || !reflect.DeepEqual(checked.Outline, parsed.Outline) {
		t.Fatalf("check=%+v parse=%+v err=%v called=%v", checked, parsed, err, called)
	}
	bad, err := Check(context.Background(), Request{Stage: AnalysisLevel("unknown"), Text: src, Resolve: resolver})
	if err == nil || called || bad.Completion != CompletionIncomplete || bad.Tree != nil {
		t.Fatalf("bad=%+v err=%v called=%v", bad, err, called)
	}
	if (Source{}).Origin != OriginUnspecified || (Request{}).Origin != OriginUnspecified || OriginWorkspace == OriginStandard {
		t.Fatal("source origin defaults")
	}
}

func TestDefinitiveRejectionsAndInterruptions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		src   []byte
		code  string
		phase AnalysisLevel
	}{
		{"syntax", []byte("---- MODULE M ----\nVARIABLE ,\n===="), CodeSyntax, LevelSyntax},
		{"lexical", []byte("---- MODULE M ----\nX == `\n===="), CodeLexical, LevelLexical},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Check(context.Background(), Request{Path: "m", Text: tc.src})
			if err != nil || r.Level != LevelSyntax || r.Completion != CompletionComplete || len(r.Diagnostics) == 0 || r.Diagnostics[0].Code != tc.code || r.Diagnostics[0].Phase != tc.phase {
				t.Fatalf("result=%+v err=%v", r, err)
			}
			for _, d := range r.Diagnostics {
				if d.Code == CodeIncomplete {
					t.Fatalf("unexpected incomplete: %+v", r.Diagnostics)
				}
			}
		})
	}
	src := []byte("---- MODULE M ----\nX == 1\n====")
	for _, check := range []bool{false, true} {
		var r Result
		var err error
		if check {
			r, err = Check(context.Background(), Request{Text: src, Options: Options{MaxDepth: 1}})
		} else {
			r, err = Parse(context.Background(), "m", src, Options{MaxDepth: 1})
		}
		if err != nil || r.Completion != CompletionIncomplete || len(r.Diagnostics) == 0 || r.Diagnostics[0].Code != CodeDepthLimit || r.Diagnostics[0].Phase != LevelSyntax {
			t.Fatalf("check=%v result=%+v err=%v", check, r, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err := Check(ctx, Request{Text: src})
	if !errors.Is(err, context.Canceled) || r.Completion != CompletionIncomplete {
		t.Fatalf("result=%+v err=%v", r, err)
	}
}

func TestCheckPartialSemanticDiagnostics(t *testing.T) {
	src := []byte("---- MODULE M ----\nQ == <<\"😀\", Missing>>\nF(x) == x\nA == F\n====\n")
	r, err := Check(context.Background(), Request{Path: "memory/M.tla", Text: src})
	requireCompleteCheck(t, r, err, LevelSemantic)
	if len(r.Diagnostics) != 2 {
		t.Fatalf("diagnostics: %+v", r.Diagnostics)
	}
	a, b := r.Diagnostics[0], r.Diagnostics[1]
	if a.Code != "semantic/4200" || a.Message != "Unknown operator: `Missing'." || a.Phase != LevelSemantic || a.Path != "memory/M.tla" {
		t.Fatal(a)
	}
	if got := string(src[a.Span.Start.ByteOffset:a.Span.End.ByteOffset]); got != "Missing" {
		t.Fatalf("span %q: %+v", got, a.Span)
	}
	if b.Code != "semantic/4204" || b.Message != "The operator F requires 1 arguments." || b.Severity != SeverityError {
		t.Fatal(b)
	}
}
func TestCheckSemanticContinuesAcrossAssumption(t *testing.T) {
	src := []byte("---- MODULE M ----\nA == Missing\nASSUME TRUE\nC == Other\n====\n")
	r, e := Check(context.Background(), Request{Text: src})
	requireCompleteCheck(t, r, e, LevelSemantic)
	if len(r.Diagnostics) != 2 || r.Diagnostics[0].Code != "semantic/4200" || r.Diagnostics[1].Code != "semantic/4200" {
		t.Fatal(r.Diagnostics)
	}
}

func TestCheckDuplicateSemanticDiagnostics(t *testing.T) {
	src := []byte("---- MODULE M ----\nA == 1\nA == 2\n====\n")
	r, e := Check(context.Background(), Request{Stage: LevelSemantic, Path: "M.tla", Text: src})
	requireCompleteCheck(t, r, e, LevelSemantic)
	if len(r.Diagnostics) != 2 || r.Diagnostics[0].Code != "semantic/4201" || r.Diagnostics[0].Severity != SeverityError || r.Diagnostics[1].Code != "semantic/4801" || r.Diagnostics[1].Severity != SeverityWarning {
		t.Fatal(r.Diagnostics)
	}
}

func TestCheckConcurrentSemanticRequests(t *testing.T) {
	src := []byte("---- MODULE M ----\nF(x) == x\nA == Missing\nB == F\n====\n")
	const n = 8
	type outcome struct {
		result Result
		err    error
	}
	results := make(chan outcome, n)
	for i := 0; i < n; i++ {
		go func() {
			r, e := Check(context.Background(), Request{Text: src, Path: "memory"})
			results <- outcome{r, e}
		}()
	}
	var reference []Diagnostic
	for i := 0; i < n; i++ {
		got := <-results
		if got.err != nil || got.result.Completion != CompletionComplete || got.result.Level != LevelSemantic {
			t.Fatal(got)
		}
		if i == 0 {
			reference = got.result.Diagnostics
		} else if !reflect.DeepEqual(reference, got.result.Diagnostics) {
			t.Fatal("request state leaked", reference, got.result.Diagnostics)
		}
	}
}

func TestCheckUnresolvedNotationContinues(t *testing.T) {
	src := []byte("---- MODULE M ----\nA == 1 + 2\nB == Missing\n====\n")
	r, e := Check(context.Background(), Request{Text: src, Path: "M.tla"})
	requireCompleteCheck(t, r, e, LevelSemantic)
	if len(r.Diagnostics) != 3 || r.Diagnostics[0].Code != "semantic/4004" || r.Diagnostics[1].Code != "semantic/4004" || r.Diagnostics[2].Code != "semantic/4200" {
		t.Fatal(r.Diagnostics)
	}
}

func TestCheckSuppressesSpeculativeImportDiagnostics(t *testing.T) {
	for _, src := range []string{
		"---- MODULE M ----\nEXTENDS Missing\nA == Unknown\n====\n",
		"---- MODULE M ----\nINSTANCE Missing\nA == Unknown\n====\n",
		"---- MODULE M ----\nI == INSTANCE Missing\nA == Unknown\n====\n",
	} {
		called := false
		r, e := Check(context.Background(), Request{Text: []byte(src), Resolve: func(context.Context, string) (Source, error) { called = true; return Source{}, ErrModuleNotFound }})
		// SpecObj aborts before semantic analysis, so A's name is never resolved.
		if e != nil || !called || r.Completion != CompletionComplete || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "semantic/4220" || r.Diagnostics[0].Severity != SeverityError || r.Diagnostics[0].Message != "Cannot find source file for module Missing imported in module M." {
			t.Fatalf("%q: %+v %v called=%v", src, r, e, called)
		}
	}
}
func TestCheckSuppressesEarlierDiagnosticsWhenGenerationIncomplete(t *testing.T) {
	src := []byte("---- MODULE M ----\nX == Missing\nINSTANCE Missing\nC == AlsoMissing\n====\n")
	r, e := Check(context.Background(), Request{Text: src})
	if e != nil || r.Completion != CompletionIncomplete {
		t.Fatal(r, e)
	}
	for _, d := range r.Diagnostics {
		if d.Severity == SeverityError {
			t.Fatalf("speculative error: %+v", r.Diagnostics)
		}
	}
}

func TestCheckFlatExtendsDocumentDiagnosticsAndContinuation(t *testing.T) {
	root := []byte("---- MODULE Root ----\nEXTENDS A, B\nR == MissingRoot\n====\n")
	sources := map[string]Source{
		"A": {Path: "same", Text: []byte("---- MODULE A ----\nN == 1\n====\n"), Origin: OriginWorkspace},
		"B": {Path: "same", Text: []byte("---- MODULE B ----\n\\* distinct offset\nN == 2\nX == MissingB\n====\n"), Origin: OriginStandard},
	}
	resolve := func(_ context.Context, n string) (Source, error) { return sources[n], nil }
	r, e := Check(context.Background(), Request{Path: "root", Text: root, Resolve: resolve})
	// A level-checks before B's errors close the shared Errors gate.
	requireCompleteCheck(t, r, e, LevelComplete)
	var conflict, dependency, after bool
	for _, d := range r.Diagnostics {
		if d.Code == "semantic/4800" {
			conflict = d.Path == "same" && d.Span.Start.ByteOffset == strings.Index(string(sources["B"].Text), "N ==")
		}
		if d.Code == "semantic/4200" && d.Path == "same" {
			dependency = d.Span.Start.ByteOffset == strings.Index(string(sources["B"].Text), "MissingB")
		}
		if d.Code == "semantic/4200" && d.Path == "root" {
			after = true
		}
	}
	if !conflict || !dependency || !after {
		t.Fatalf("diagnostics: %+v", r.Diagnostics)
	}
}

func TestCheckFlatExtendsFailuresAndIsolation(t *testing.T) {
	root := []byte("---- MODULE Root ----\nEXTENDS A\nR == 1\n====\n")
	missing, e := Check(context.Background(), Request{Text: root})
	if e != nil || !strings.Contains(missing.Diagnostics[len(missing.Diagnostics)-1].Message, "no module resolver") {
		t.Fatal(missing, e)
	}
	configured, e := Check(context.Background(), Request{Text: root, Resolve: func(context.Context, string) (Source, error) {
		return Source{}, fmt.Errorf("wrapped: %w", ErrModuleNotFound)
	}})
	if e != nil || configured.Completion != CompletionComplete || len(configured.Diagnostics) != 1 || configured.Diagnostics[0].Message != "Cannot find source file for module A imported in module Root." {
		t.Fatal(configured, e)
	}
	operational := errors.New("resolver failed")
	if _, e = Check(context.Background(), Request{Text: root, Resolve: func(context.Context, string) (Source, error) { return Source{}, operational }}); !errors.Is(e, operational) {
		t.Fatal(e)
	}
	broken := Source{Path: "dependency", Text: []byte("---- MODULE A ----\nBad == (\n====\n")}
	r, e := Check(context.Background(), Request{Text: root, Resolve: func(context.Context, string) (Source, error) { return broken, nil }})
	// SANY stops after a dependency's parse errors: a completed verdict.
	if e != nil || r.Completion != CompletionComplete || len(r.Diagnostics) == 0 || r.Diagnostics[0].Path != "dependency" || r.Diagnostics[0].Severity != SeverityError {
		t.Fatal(r, e)
	}
	good := Source{Path: "dependency", Text: []byte("---- MODULE A ----\nN == 1\n====\n")}
	r, e = Check(context.Background(), Request{Text: root, Resolve: func(context.Context, string) (Source, error) { return good, nil }})
	requireCompleteCheck(t, r, e, LevelComplete)
	if len(r.Diagnostics) != 0 {
		t.Fatal(r.Diagnostics)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = Check(ctx, Request{Text: root}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestCheckExtendsConflictFollowsClauseOrder(t *testing.T) {
	files := map[string]Source{
		"A": {Path: "A", Text: []byte("---- MODULE A ----\nN == 1\n====\n")},
		"B": {Path: "B", Text: []byte("---- MODULE B ----\nVARIABLE N\n====\n")},
	}
	for _, tc := range []struct{ clause, conflictPath string }{{"A, B", "B"}, {"B, A", "A"}} {
		src := []byte("---- MODULE Root ----\nEXTENDS " + tc.clause + "\nR == Missing\n====\n")
		r, err := Check(context.Background(), Request{Text: src, Resolve: func(_ context.Context, n string) (Source, error) { return files[n], nil }})
		if err != nil {
			t.Fatal(err)
		}
		conflict, continuation := false, false
		for _, d := range r.Diagnostics {
			if d.Code == "semantic/4224" && d.Path == tc.conflictPath {
				conflict = true
			}
			if d.Code == "semantic/4200" {
				continuation = true
			}
		}
		if !conflict || !continuation {
			t.Fatalf("%s: %+v", tc.clause, r.Diagnostics)
		}
	}
}

func TestCheckAssumptionTheoremFixture(t *testing.T) {
	src, err := os.ReadFile("testdata/assumption-theorem/Statements.tla")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Check(context.Background(), Request{Path: "Statements.tla", Text: src})
	if err != nil {
		t.Fatal(err)
	}
	requireCompleteCheck(t, result, err, LevelComplete)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("%+v", result.Diagnostics)
	}
}
