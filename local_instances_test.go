// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package prudence

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStandardInstances(t *testing.T) {
	for n, expr := range map[string]string{"FiniteSets": "Cardinality({1})", "Sequences": "Len(<<1>>)", "TLC": "TLCGet(1)"} {
		t.Run(n, func(t *testing.T) {
			r, e := Check(context.Background(), Request{Text: []byte("---- MODULE Root ----\nEXTENDS " + n + "\nR == " + expr + "\n====\n")})
			if e != nil || r.Level != LevelComplete || r.Completion != CompletionComplete || len(r.Diagnostics) != 0 {
				t.Fatalf("%+v %v", r, e)
			}
		})
	}
}

func TestLocalInstanceBoundariesAndLocations(t *testing.T) {
	files := map[string]string{
		"A":             "---- MODULE A ----\nBad == MissingA\nVisible == 1\nLOCAL Hidden == 2\n====\n",
		"Parameterized": "---- MODULE Parameterized ----\nCONSTANT X\nV == X\n====\n",
	}
	resolve := func(_ context.Context, name string) (Source, error) {
		if text, ok := files[name]; ok {
			return Source{Path: name + ".tla", Text: []byte(text)}, nil
		}
		return Source{}, ErrModuleNotFound
	}
	for _, tc := range []struct {
		body string
		want string
	}{
		{"LOCAL INSTANCE A\nR == Visible", ""},
		{"LOCAL INSTANCE Parameterized", ""},
		{"LOCAL INSTANCE A WITH X <- 1", ""},
		{"INSTANCE A\nR == Visible", ""},
		{"I == INSTANCE A", ""},
		{"R == LET I == INSTANCE A IN 1", ""},
	} {
		t.Run(tc.body, func(t *testing.T) {
			r, e := Check(context.Background(), Request{Path: "Root.tla", Text: []byte("---- MODULE Root ----\n" + tc.body + "\n====\n"), Resolve: resolve})
			if tc.want == "" {
				// A's generation error is a definitive verdict; the shared
				// Errors gate then skips level checking.
				// Parameterized itself level-checks before the root's error.
				level := LevelSemantic
				if strings.Contains(tc.body, "Parameterized") {
					level = LevelComplete
				}
				requireCompleteCheck(t, r, e, level)
			} else if e != nil || len(r.Diagnostics) == 0 || r.Diagnostics[len(r.Diagnostics)-1].Message != tc.want {
				t.Fatalf("%+v %v", r, e)
			}
			if tc.body == "LOCAL INSTANCE A\nR == Visible" {
				if len(r.Diagnostics) != 1 || r.Diagnostics[0].Path != "A.tla" || r.Diagnostics[0].Code != "semantic/4200" {
					t.Fatal(r.Diagnostics)
				}
			}
		})
	}
}

func TestExternalInstancesAndSubstitutionDiagnostics(t *testing.T) {
	files := map[string]string{"A": "CONSTANT X, Op(_)\nF(y) == Op(y)\nValue == X", "B": "Id(x) == x\nJ(t) == INSTANCE A WITH X <- t, Op <- Id"}
	resolve := func(_ context.Context, n string) (Source, error) {
		body, ok := files[n]
		if !ok {
			return Source{}, ErrModuleNotFound
		}
		return Source{Path: n + ".tla", Text: []byte("---- MODULE " + n + " ----\n" + body + "\n====\n")}, nil
	}
	for _, tc := range []struct {
		body string
		want []string
	}{
		{"CONSTANT X, Op(_)\nINSTANCE A\nR == F(X)", nil},
		{"Id(x) == x\nINSTANCE A WITH X <- 1, Op <- Id\nR == Value", nil},
		{"Id(x) == x\nI(t) == INSTANCE A WITH X <- t, Op <- Id\nR == I(1)!F(2)", nil},
		{"I == INSTANCE B\nR == I!J(1)!F(2)", nil},
		{"I(F(_)) == INSTANCE A WITH X <- 1, Op <- F\nId(x) == x\nR == I(Id)!F(2)", nil},
		{"I(x) == INSTANCE A WITH X <- x, Op <- LAMBDA z: z\nR == I(1)!F(2)", nil},
		{"I(x) == INSTANCE A WITH X <- x, Op <- LAMBDA z: z\nR == I", []string{"semantic/4204"}},
		{"Id(x) == x\nI == INSTANCE A WITH X <- 1, X <- 2, Op <- Id", []string{"semantic/4241"}},
		{"INSTANCE A WITH Nope <- 1", []string{"semantic/4242", "semantic/4240", "semantic/4240"}},
	} {
		t.Run(tc.body, func(t *testing.T) {
			r, e := Check(context.Background(), Request{Path: "Root.tla", Text: []byte("---- MODULE Root ----\n" + tc.body + "\n====\n"), Resolve: resolve})
			if e != nil {
				t.Fatal(e)
			}
			var got []string
			for _, d := range r.Diagnostics {
				if d.Severity == SeverityError {
					got = append(got, d.Code)
				}
			}
			if len(got) != len(tc.want) {
				t.Fatalf("diagnostics: %+v", r.Diagnostics)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("diagnostics: %+v", r.Diagnostics)
				}
			}
			// Every case level-checks A or B before the root.
			requireCompleteCheck(t, r, e, LevelComplete)
		})
	}
}

func TestInstanceDiagnosticsAndUnsupportedSelectors(t *testing.T) {
	source := Source{Path: "same", Text: []byte("---- MODULE A ----\nCONSTANT X, Op(_)\nF(y) == X\nValue == X\n====\n")}
	resolve := func(_ context.Context, n string) (Source, error) {
		if n == "A" {
			return source, nil
		}
		return Source{}, ErrModuleNotFound
	}
	cases := []struct {
		body       string
		codes      []string
		incomplete bool
	}{
		// SANY's second error is located at a built-in node ("In module
		// --TLA+ BUILTINS--"), so it has no source span.
		{"H(x,y) == x\nINSTANCE A WITH X <- 1, Op <- H", []string{"semantic/4271", "semantic/4243"}, false},
		{"H(x) == x\nINSTANCE A WITH X <- 1, X <- 2, Op <- H", []string{"semantic/4241"}, false},
		{"INSTANCE A WITH Bad <- 1", []string{"semantic/4242", "semantic/4240", "semantic/4240"}, false},
		{"H(x) == x\nINSTANCE A WITH X <- 1, Op <- H\nINSTANCE A WITH X <- 1, Op <- H", nil, false},
		{"H(x) == x\nI == INSTANCE A WITH X <- 1, Op <- H\nR == I!Missing", []string{"semantic/4200"}, false},
		{"X(x) == x\nH(x) == x\nINSTANCE A WITH X <- 1, Op <- H", []string{"semantic/4004"}, false},
		{"H(x) == x\nINSTANCE A WITH X <- 1, Op <- H\nR == Value!1", []string{"semantic/4005"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.body, func(t *testing.T) {
			r, e := Check(context.Background(), Request{Path: "same", Text: []byte("---- MODULE Root ----\n" + tc.body + "\n====\n"), Resolve: resolve})
			if e != nil {
				t.Fatal(e)
			}
			if tc.incomplete {
				if len(r.Diagnostics) == 0 || r.Diagnostics[len(r.Diagnostics)-1].Message != "semantic analysis stopped at an unsupported construct; level analysis is not yet implemented" {
					t.Fatalf("%+v", r.Diagnostics)
				}
				return
			}
			if strings.Count(tc.body, "INSTANCE A") == 2 {
				if len(r.Diagnostics) < 2 || r.Diagnostics[0].Code != "semantic/4801" || r.Diagnostics[1].Code != "semantic/4801" {
					t.Fatalf("%+v", r.Diagnostics)
				}
			}
			var codes []string
			for _, d := range r.Diagnostics {
				if d.Severity == SeverityError {
					codes = append(codes, d.Code)
					builtin := d.Code == "semantic/4243" && d.Span == Span{}
					if d.Path != "same" || (d.Span.Start.Line == 0 && !builtin) {
						t.Fatalf("bad source span: %+v", d)
					}
				}
			}
			if len(codes) != len(tc.codes) || r.Completion != CompletionComplete {
				t.Fatalf("%+v", r)
			}
			for i := range codes {
				if codes[i] != tc.codes[i] {
					t.Fatalf("%+v", r.Diagnostics)
				}
			}
		})
	}
}

func TestInstanceDiagnosticSourceOffsets(t *testing.T) {
	dependency := Source{Path: "same", Text: []byte("---- MODULE A ----\nCONSTANT X\nV == X\n====\n")}
	resolve := func(_ context.Context, n string) (Source, error) {
		if n == "A" {
			return dependency, nil
		}
		return Source{}, ErrModuleNotFound
	}
	for _, tc := range []struct{ body, code, at string }{
		{"INSTANCE A WITH Bad <- 1", "semantic/4242", "Bad"},
		{"INSTANCE A WITH X <- 1, X <- 2", "semantic/4241", "2"},
		{"INSTANCE A", "semantic/4240", "INSTANCE"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			text := "---- MODULE Root ----\n" + tc.body + "\n====\n"
			r, e := Check(context.Background(), Request{Path: "same", Text: []byte(text), Resolve: resolve})
			if e != nil {
				t.Fatal(e)
			}
			for _, d := range r.Diagnostics {
				if d.Code == tc.code {
					want := strings.LastIndex(text, tc.at)
					if d.Path != "same" || d.Span.Start.ByteOffset != want {
						t.Fatalf("got %+v want byte %d", d, want)
					}
					return
				}
			}
			t.Fatalf("missing %s: %+v", tc.code, r.Diagnostics)
		})
	}
}

// TestInstanceSelectorFixtures checks the public request path with reusable
// modules whose nested wrappers and selectors are also checked internally.
func TestInstanceSelectorFixtures(t *testing.T) {
	read := func(name string) []byte {
		t.Helper()
		b, err := os.ReadFile(filepath.Join("testdata", "instance-selectors", name+".tla"))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	resolve := func(_ context.Context, name string) (Source, error) {
		if name != "Base" && name != "Bridge" {
			return Source{}, ErrModuleNotFound
		}
		return Source{Path: name + ".tla", Text: read(name)}, nil
	}
	r, err := Check(context.Background(), Request{Path: "Root.tla", Text: read("Root"), Resolve: resolve})
	requireCompleteCheck(t, r, err, LevelComplete)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("%+v", r.Diagnostics)
	}
}

func TestInstanceMarkerStructuralBoundary(t *testing.T) {
	root := []byte("---- MODULE Root ----\nI == INSTANCE Base WITH X <- 1\nR == I!1\n====\n")
	resolve := func(_ context.Context, name string) (Source, error) {
		return Source{Path: "Base.tla", Text: []byte("---- MODULE Base ----\nCONSTANT X\nF == X\n====\n")}, nil
	}
	r, err := Check(context.Background(), Request{Path: "Root.tla", Text: root, Resolve: resolve})
	if err != nil || r.Completion != CompletionComplete {
		t.Fatalf("%v %+v", err, r.Diagnostics)
	}
	// FindingOpName after the marker: the operand selector is not a
	// NameSel, so the compound name stays I and nothing resolves
	// (Generator.java:732-813, 846); recorded SANY reports I!@ the same
	// way (sany-conformance U-4103c37b50b1).
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "semantic/4200" || r.Diagnostics[0].Message != "Unknown operator: `I'." ||
		r.Diagnostics[0].Span.Start.Line != 2 || r.Diagnostics[0].Span.Start.Column != 5 || r.Diagnostics[0].Span.End.Column != 6 {
		t.Fatalf("%+v", r.Diagnostics)
	}
}

func TestInstanceSelectorDiagnosticSourceOwnership(t *testing.T) {
	dep := []byte("---- MODULE Base ----\nCONSTANT X\nBroken == MissingBase\nF == <<X>>\n====\n")
	root := []byte("---- MODULE Root ----\nINSTANCE Base WITH X <- 1\nR == F!2\n====\n")
	resolve := func(_ context.Context, _ string) (Source, error) { return Source{Path: "same", Text: dep}, nil }
	r, err := Check(context.Background(), Request{Path: "same", Text: root, Resolve: resolve})
	if err != nil {
		t.Fatal(err)
	}
	var actual []Diagnostic
	for _, d := range r.Diagnostics {
		if d.Severity == SeverityError {
			actual = append(actual, d)
		}
	}
	if len(actual) != 2 || actual[0].Code != "semantic/4200" || actual[1].Code != "semantic/4005" {
		t.Fatalf("%+v", r.Diagnostics)
	}
	if actual[0].Span.Start.ByteOffset != strings.Index(string(dep), "MissingBase") || actual[1].Span.Start.ByteOffset != strings.Index(string(root), "2\n====") {
		t.Fatalf("wrong source ownership: %+v", actual)
	}
}

func TestQualifiedSelectorUnknownCompoundName(t *testing.T) {
	root := []byte("---- MODULE Root ----\nI == INSTANCE Base WITH X <- 1\nR == I!Missing!Next\n====\n")
	resolve := func(_ context.Context, _ string) (Source, error) {
		return Source{Path: "Base.tla", Text: []byte("---- MODULE Base ----\nCONSTANT X\nF == X\n====\n")}, nil
	}
	r, err := Check(context.Background(), Request{Path: "Root.tla", Text: root, Resolve: resolve})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Diagnostics) < 1 || r.Diagnostics[0].Code != "semantic/4200" || r.Diagnostics[0].Message != "Unknown operator: `I!Missing!Next'." || r.Diagnostics[0].Span.Start.ByteOffset != strings.Index(string(root), "I!Missing") {
		t.Fatalf("%+v", r.Diagnostics)
	}
}
