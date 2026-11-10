// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package prudence

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// requireCompleteCheck asserts that Check ran the requested analysis to its
// valid stopping point: no error, CompletionComplete, the given Level and no
// analysis/incomplete diagnostic.
func requireCompleteCheck(t *testing.T, r Result, err error, level AnalysisLevel) {
	t.Helper()
	if err != nil || r.Completion != CompletionComplete || r.Level != level {
		t.Fatalf("want complete check at %s: %+v %v", level, r, err)
	}
	for _, d := range r.Diagnostics {
		if d.Code == CodeIncomplete {
			t.Fatalf("unexpected incomplete diagnostic: %+v", r.Diagnostics)
		}
	}
}

func checkText(t *testing.T, stage AnalysisLevel, path, text string, resolve Resolver) Result {
	t.Helper()
	r, err := Check(context.Background(), Request{Stage: stage, Path: path, Text: []byte(text), Resolve: resolve})
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return r
}

func noModules(context.Context, string) (Source, error) { return Source{}, ErrModuleNotFound }

// sanyAt asserts one diagnostic against SANY's printed "line L, col C to line
// L2, col C2" (one-based, inclusive end) and message.
func sanyAt(t *testing.T, d Diagnostic, code string, phase AnalysisLevel, path string, l1, c1, l2, c2 int, message string) {
	t.Helper()
	if d.Code != code || d.Phase != phase || d.Severity != SeverityError || d.Path != path || d.Message != message {
		t.Fatalf("diagnostic %+v", d)
	}
	if d.Span.Start.Line+1 != l1 || d.Span.Start.Column+1 != c1 || d.Span.End.Line+1 != l2 || d.Span.End.Column != c2 {
		t.Fatalf("span %+v, want SANY line %d, col %d to line %d, col %d", d.Span, l1, c1, l2, c2)
	}
}

const ordinarySpec = `---- MODULE Counter ----
EXTENDS Naturals, Sequences, FiniteSets, TLC
CONSTANT N
VARIABLE x, y
vars == <<x, y>>
TypeOK == x \in 0..N /\ y \in Seq(Nat)
Init == x = 0 /\ y = <<>>
Next == x < N /\ x' = x + 1 /\ y' = Append(y, x)
Spec == Init /\ [][Next]_vars /\ WF_vars(Next)
Live == <>(x = N)
Size == Cardinality({x, N})
THEOREM Spec => []TypeOK
====
`

func TestPublicCheckOrdinarySpecIsComplete(t *testing.T) {
	for _, stage := range []AnalysisLevel{"", LevelComplete} {
		r := checkText(t, stage, "Counter.tla", ordinarySpec, nil)
		requireCompleteCheck(t, r, nil, LevelComplete)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("stage %q: %+v", stage, r.Diagnostics)
		}
	}
	r := checkText(t, LevelSemantic, "Counter.tla", ordinarySpec, nil)
	requireCompleteCheck(t, r, nil, LevelSemantic)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("semantic stage: %+v", r.Diagnostics)
	}
}

const e1Spec = `---- MODULE E1 ----
\* SANY level error: an ASSUME must be constant-level.
EXTENDS Naturals
VARIABLE x
ASSUME x = 0
Init == x = 0
Next == x' = (x + 1) % 3
====
`

// The expected messages and positions are stock SANY's output for the same
// sources (sany-conformance tlx and model-one-shot cases).
func TestPublicCheckLevelErrorsMatchSANY(t *testing.T) {
	for _, tc := range []struct {
		name, text     string
		module, code   string
		l1, c1, l2, c2 int
		message        string
	}{
		{"E1", e1Spec, "E1", "semantic/4206", 5, 1, 5, 12, "Level error: assumptions must be level 0 (Constant), \nbut this one has level 1."},
		{"inner", "---- MODULE Target ----\n---- MODULE Inner ----\nASSUME []TRUE\n====\n====\n", "Target", "semantic/4206", 3, 1, 3, 13, "Level error: assumptions must be level 0 (Constant), \nbut this one has level 3."},
		{"bounded", "---- MODULE Target ----\nCONSTANT F(_)\nTHEOREM T ==\n  ASSUME NEW VARIABLE v,\n         \\E z \\in []TRUE : TRUE\n  PROVE TRUE\n====\n", "Target", "semantic/4315", 5, 10, 5, 31, "Level error in applying operator $BoundedExists:\nThe level of the range for the bounded variable line 5, col 13 to line 5, col 13 of module Target \nexceeds the maximum level allowed by the operator."},
		{"unchanged", "---- MODULE Target ----\nVARIABLE a, x, y, z\nr == [a EXCEPT ![x] = y', ![z] = UNCHANGED @]\n====\n", "Target", "semantic/4205", 3, 34, 3, 44, "Level error in applying operator UNCHANGED:\nThe level of argument 1 exceeds the maximum level allowed by the operator."},
		{"instance", "---- MODULE Del07 ----\nVARIABLE x\n---- MODULE N ----\nTHEOREM TN == []TRUE\n====\n---- MODULE M ----\nEXTENDS N\nCONSTANT c\n====\nJ == INSTANCE M WITH c <- x\n====\n", "Del07", "semantic/4245", 10, 1, 10, 27, "Level error in instantiating module 'M':\nThe level of the expression or operator substituted for 'c' \nmust be at most 0."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.module + ".tla" // SANY requires the file name to match the module.
			r := checkText(t, "", path, tc.text, noModules)
			requireCompleteCheck(t, r, nil, LevelComplete)
			if len(r.Diagnostics) != 1 {
				t.Fatalf("%+v", r.Diagnostics)
			}
			sanyAt(t, r.Diagnostics[0], tc.code, LevelComplete, path, tc.l1, tc.c1, tc.l2, tc.c2, tc.message)
			// A semantic-only request never runs the level checker.
			s := checkText(t, LevelSemantic, path, tc.text, noModules)
			requireCompleteCheck(t, s, nil, LevelSemantic)
			if len(s.Diagnostics) != 0 {
				t.Fatalf("semantic stage: %+v", s.Diagnostics)
			}
		})
	}
}

func TestPublicCheckSemanticErrorSkipsLevels(t *testing.T) {
	// Both lines carry a level error, but SANY's shared Errors gate skips the
	// level check once generation has reported an error.
	text := "---- MODULE S ----\nVARIABLE x\nASSUME x = y\nASSUME x' = 1\n====\n"
	r := checkText(t, "", "S.tla", text, nil)
	requireCompleteCheck(t, r, nil, LevelSemantic)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	sanyAt(t, r.Diagnostics[0], "semantic/4200", LevelSemantic, "S.tla", 3, 12, 3, 12, "Unknown operator: `y'.")
	// The model-one-shot E-10fbdf421283 source: three generation errors.
	r = checkText(t, "", "Target.tla", "---- MODULE Target ----\nVARIABLE f\nf[x \\in Nat] == x\n====\n", nil)
	requireCompleteCheck(t, r, nil, LevelSemantic)
	if len(r.Diagnostics) != 3 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	sanyAt(t, r.Diagnostics[0], "semantic/4200", LevelSemantic, "Target.tla", 3, 9, 3, 11, "Unknown operator: `Nat'.")
	sanyAt(t, r.Diagnostics[1], "semantic/4201", LevelSemantic, "Target.tla", 3, 1, 3, 17, "Function name `f' already defined or declared.")
	sanyAt(t, r.Diagnostics[2], "semantic/4200", LevelSemantic, "Target.tla", 3, 17, 3, 17, "Unknown operator: `x'.")
}

func TestPublicCheckWorkspaceImportsAndInstances(t *testing.T) {
	files := map[string]string{
		"M":     "---- MODULE M ----\nCONSTANT C0, C2(_, _)\nFoo == C0\nBar(a, b) == C2(a, b)\n====\n",
		"Inner": "---- MODULE Inner ----\nEXTENDS Naturals\nVARIABLE v\nInc == v' = v + 1\n====\n",
		"Bad":   "---- MODULE Bad ----\nVARIABLE w\nASSUME w = 0\n====\n",
	}
	var calls []string
	resolve := func(_ context.Context, name string) (Source, error) {
		calls = append(calls, name)
		if text, ok := files[name]; ok {
			return Source{Path: name + ".tla", Text: []byte(text), Origin: OriginWorkspace}, nil
		}
		return Source{}, ErrModuleNotFound
	}
	// model-one-shot E-0eb8a2c2b923: a parameterized INSTANCE with operator substitution.
	r := checkText(t, "", "Target.tla", "---- MODULE Target ----\nH(q(_), m) == q(m)\nA(p(_), c) ==\n  INSTANCE M WITH C0 <- p(c), C2 <- H\n====\n", resolve)
	requireCompleteCheck(t, r, nil, LevelComplete)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	// A mixed EXTENDS/INSTANCE graph with standard modules behind the resolver.
	r = checkText(t, "", "Root.tla", "---- MODULE Root ----\nEXTENDS Naturals, Sequences\nVARIABLE z\nI == INSTANCE Inner WITH v <- z\nInit == z = 0\nNext == I!Inc /\\ Len(<<z>>) = 1\nSpec == Init /\\ [][Next]_z\n====\n", resolve)
	requireCompleteCheck(t, r, nil, LevelComplete)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	// A level error inside a workspace dependency is reported at its own path.
	r = checkText(t, "", "Root.tla", "---- MODULE Root ----\nEXTENDS Bad\nR == w\n====\n", resolve)
	requireCompleteCheck(t, r, nil, LevelComplete)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	sanyAt(t, r.Diagnostics[0], "semantic/4206", LevelComplete, "Bad.tla", 3, 1, 3, 12, "Level error: assumptions must be level 0 (Constant), \nbut this one has level 1.")
}

func TestPublicCheckRecursionAndProof(t *testing.T) {
	for name, text := range map[string]string{
		"B7":                 "---- MODULE B7 ----\nEXTENDS Naturals\nVARIABLE x\nRECURSIVE F(_), H(_)\nF(n) == H(n) + x\nH(n) == IF n = 0 THEN 0 ELSE F(n - 1)\nK == H(1)\nInit == x = 0\nNext == x < 3 /\\ x' = x + 1\nInv == K = x\n====\n",
		"OpenStartProofCase": "---- MODULE OpenStartProofCase ----\nTHEOREM TRUE\nPROOF\n<1>1. HAVE CASE TRUE -> TRUE\n<1>2. HAVE TRUE => CASE TRUE -> TRUE\n<1> QED OBVIOUS\n====\n",
		"P":                  "---- MODULE P ----\nEXTENDS Naturals\nTHEOREM T == \\A n \\in Nat : n + 0 = n\n<1>1. TAKE n \\in Nat\n<1>2. QED BY <1>1\n====\n",
	} {
		t.Run(name, func(t *testing.T) {
			r := checkText(t, "", name+".tla", text, nil)
			requireCompleteCheck(t, r, nil, LevelComplete)
			if len(r.Diagnostics) != 0 {
				t.Fatalf("%+v", r.Diagnostics)
			}
		})
	}
}

func TestPublicCheckModuleResolutionDiagnostics(t *testing.T) {
	text := "---- MODULE E4220_Test ----\nEXTENDS DoesNotExist12345\nX == 1\n====\n"
	r := checkText(t, "", "E4220_Test.tla", text, noModules)
	requireCompleteCheck(t, r, nil, LevelSemantic)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	// SANY gives only "In module E4220_Test"; the span is the reference name.
	sanyAt(t, r.Diagnostics[0], "semantic/4220", LevelSemantic, "E4220_Test.tla", 2, 9, 2, 25, "Cannot find source file for module DoesNotExist12345 imported in module E4220_Test.")

	files := map[string]string{"A": "---- MODULE A ----\nEXTENDS B\n====\n", "B": "---- MODULE B ----\nINSTANCE A\n====\n"}
	resolve := func(_ context.Context, name string) (Source, error) {
		if text, ok := files[name]; ok {
			return Source{Path: name + ".tla", Text: []byte(text)}, nil
		}
		return Source{}, ErrModuleNotFound
	}
	r = checkText(t, "", "Root.tla", "---- MODULE Root ----\nEXTENDS A\n====\n", resolve)
	requireCompleteCheck(t, r, nil, LevelSemantic)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	d := r.Diagnostics[0]
	if d.Code != "semantic/4222" || d.Severity != SeverityError || d.Message != "Circular dependency among .tla files; dependency cycle is:\n\n  A.tla --> B.tla --> A.tla" || d.Path != "B.tla" || d.Span.Start.Line != 1 || d.Span.Start.Column != 9 {
		t.Fatalf("%+v", d)
	}
}

func TestPublicCheckHonestIncomplete(t *testing.T) {
	// No resolver for a nonstandard module: acquisition is unavailable.
	r := checkText(t, "", "Root.tla", "---- MODULE Root ----\nEXTENDS A\n====\n", nil)
	if r.Completion != CompletionIncomplete || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != CodeIncomplete || r.Diagnostics[0].Severity != SeverityInfo {
		t.Fatalf("%+v", r)
	}
	// An unported construct (a selector continuing through a LET-local
	// INSTANCE marker) stays incomplete.
	r = checkText(t, "", "Root.tla", "---- MODULE Root ----\n---- MODULE Inner ----\nk == 1\n====\nV == LET I == INSTANCE Inner IN 1\nASSUME V!:!I!k = 1\n====\n", nil)
	if r.Completion != CompletionIncomplete || r.Diagnostics[len(r.Diagnostics)-1].Code != CodeIncomplete {
		t.Fatalf("%+v", r)
	}
	// A parser depth limit is a resource stop.
	r, err := Check(context.Background(), Request{Path: "e1", Text: []byte(e1Spec), Options: Options{MaxDepth: 3}})
	if err != nil || r.Completion != CompletionIncomplete {
		t.Fatalf("%+v %v", r, err)
	}
	// Cancellation before or during the check returns the context error.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := Check(ctx, Request{Path: "e1", Text: []byte(e1Spec)}); !errors.Is(err, context.Canceled) || r.Completion == CompletionComplete {
		t.Fatalf("%+v %v", r, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	resolve := func(context.Context, string) (Source, error) { cancel(); return Source{}, ErrModuleNotFound }
	if r, err := Check(ctx, Request{Path: "root", Text: []byte("---- MODULE Root ----\nEXTENDS A\n====\n"), Resolve: resolve}); !errors.Is(err, context.Canceled) || r.Completion == CompletionComplete {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestPublicCheckRepeatedAndParallelCalls(t *testing.T) {
	texts := []string{ordinarySpec, e1Spec, "---- MODULE S ----\nVARIABLE x\nASSUME x = y\n====\n"}
	reference := make([]Result, len(texts))
	for i, text := range texts {
		reference[i] = checkText(t, "", "spec", text, nil)
	}
	const workers = 6
	var wg sync.WaitGroup
	failures := make(chan string, workers*len(texts))
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i, text := range texts {
				r, err := Check(context.Background(), Request{Path: "spec", Text: []byte(text)})
				if err != nil || r.Completion != reference[i].Completion || r.Level != reference[i].Level || !reflect.DeepEqual(r.Diagnostics, reference[i].Diagnostics) {
					failures <- fmt.Sprintf("text %d: %+v %v", i, r, err)
				}
			}
		}()
	}
	wg.Wait()
	close(failures)
	for f := range failures {
		t.Fatal(f)
	}
}

// SANY's operator-stack ParseException has no position; its message cites the
// block, which the public span uses (pit-survivor E-4e1a8648e73e).
func TestPrecedenceConflictUsesCitedBlock(t *testing.T) {
	text := "---- MODULE Other01 ----\nVARIABLE x\ny == []x'\n===="
	r := checkText(t, "", "Other01.tla", text, nil)
	if r.Completion != CompletionComplete || len(r.Diagnostics) == 0 {
		t.Fatalf("%+v", r)
	}
	d := r.Diagnostics[0]
	if d.Code != CodeSyntax || !strings.Contains(d.Message, "Precedence conflict between ops [] in block line 3, col 9 to line 3, col 9 of module Other01 and '.") {
		t.Fatalf("%+v", d)
	}
	if d.Span.Start.Line != 2 || d.Span.Start.Column != 8 || d.Span.Start.ByteOffset != strings.Index(text, "'") || d.Span.End.ByteOffset != strings.Index(text, "'")+1 {
		t.Fatalf("span %+v", d.Span)
	}
}
