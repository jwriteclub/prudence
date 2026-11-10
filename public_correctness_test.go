// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package prudence

import (
	"context"
	"strings"
	"testing"
)

// Each expectation below is stock SANY's recorded output (tlaplus dc6a944)
// for the named sany-conformance case.

// unlocated asserts a diagnostic that SANY reports without a source position
// ("Unknown location" or "In module X"): an empty span at the start of path.
func unlocated(t *testing.T, d Diagnostic, code, path, message string) {
	t.Helper()
	if d.Code != code || d.Phase != LevelSemantic || d.Severity != SeverityError || d.Path != path || d.Message != message || d.Span != (Span{}) {
		t.Fatalf("diagnostic %+v", d)
	}
}

func TestPublicCheckFileNameMismatch(t *testing.T) {
	// tlaplus-semantic-error-corpus/A-err-e4221.
	text := "If the top-level module in a .tla source file does not match the filename, it\nshould be an error.\n---- MODULE Not_Named_E4221_Test ----\n====\n\n"
	r := checkText(t, "", "E4221_Test.tla", text, noModules)
	requireCompleteCheck(t, r, nil, LevelSemantic)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	unlocated(t, r.Diagnostics[0], "semantic/4221", "E4221_Test.tla", "File name 'E4221_Test' does not match the name 'Not_Named_E4221_Test' of the top level module it contains.")

	// tlapm/B-unit-e-arith-intlit0-test-2c620e: SANY checks the root's file
	// name before it looks for TLAPS, which this workspace lacks.
	calls := 0
	resolve := func(context.Context, string) (Source, error) { calls++; return Source{}, ErrModuleNotFound }
	r = checkText(t, "", "intlit0_test.tla", "---- MODULE intlit_test ----\n\nEXTENDS TLAPS, Integers\n\nTHEOREM 0 \\in Int\n    OBVIOUS\n\n====\n", resolve)
	requireCompleteCheck(t, r, nil, LevelSemantic)
	if len(r.Diagnostics) != 1 || calls != 0 {
		t.Fatalf("calls %d: %+v", calls, r.Diagnostics)
	}
	unlocated(t, r.Diagnostics[0], "semantic/4221", "intlit0_test.tla", "File name 'intlit0_test' does not match the name 'intlit_test' of the top level module it contains.")

	// The file identity is the last element of a .tla label, with the
	// extension in any letter case (FileUtil.createNamedInputStream).
	for _, path := range []string{"dir/sub/E4221_Test.tla", `C:\specs\E4221_Test.tla`, "E4221_Test.TLA"} {
		r = checkText(t, "", path, text, noModules)
		if len(r.Diagnostics) != 1 {
			t.Fatalf("%s: %+v", path, r.Diagnostics)
		}
		unlocated(t, r.Diagnostics[0], "semantic/4221", path, "File name 'E4221_Test' does not match the name 'Not_Named_E4221_Test' of the top level module it contains.")
		r = checkText(t, "", path, "---- MODULE E4221_Test ----\n====\n", noModules)
		requireCompleteCheck(t, r, nil, LevelComplete)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("%s: %+v", path, r.Diagnostics)
		}
	}

	// Empty and opaque labels carry no file identity and no requirement.
	for _, path := range []string{"", "memory", "untitled-1", "inmemory:Spec", "label/", ".tla", "spec.txt"} {
		r = checkText(t, "", path, text, noModules)
		requireCompleteCheck(t, r, nil, LevelComplete)
		if len(r.Diagnostics) != 0 {
			t.Fatalf("%q: %+v", path, r.Diagnostics)
		}
	}

	// A syntax-only request does not load modules, so it does not check.
	r = checkText(t, LevelSyntax, "E4221_Test.tla", text, noModules)
	if r.Completion != CompletionComplete || r.Level != LevelSyntax || len(r.Diagnostics) != 0 {
		t.Fatalf("syntax stage: %+v", r)
	}
}

func TestPublicCheckDependencyFileIdentity(t *testing.T) {
	// SANY reads a dependency from <name>.tla, so its identity is the
	// requested name whatever label the resolver returns; the check precedes
	// that dependency's own dependencies.
	var calls []string
	resolve := func(_ context.Context, name string) (Source, error) {
		calls = append(calls, name)
		if name == "Dep" {
			return Source{Path: "lib/dep-file.tla", Text: []byte("---- MODULE Other ----\nEXTENDS Missing\n====\n"), Origin: OriginWorkspace}, nil
		}
		return Source{}, ErrModuleNotFound
	}
	r := checkText(t, "", "Root.tla", "---- MODULE Root ----\nEXTENDS Dep\n====\n", resolve)
	requireCompleteCheck(t, r, nil, LevelSemantic)
	if len(r.Diagnostics) != 1 || len(calls) != 1 {
		t.Fatalf("calls %v: %+v", calls, r.Diagnostics)
	}
	unlocated(t, r.Diagnostics[0], "semantic/4221", "lib/dep-file.tla", "File name 'Dep' does not match the name 'Other' of the top level module it contains.")

	// A monolithic module's identity is its own name, never the owner's label.
	root := "---- MODULE Root ----\nEXTENDS Wanted\n====\n---- MODULE Wanted ----\nW == 1\n====\n"
	r = checkText(t, "", "Root.tla", root, noModules)
	requireCompleteCheck(t, r, nil, LevelComplete)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("monolith: %+v", r.Diagnostics)
	}
}

func TestPublicCheckBuiltinLocatedDiagnostics(t *testing.T) {
	// deep-cuts/model-one-shot/E-e3ddc9b6dec0: two entries, the second "In
	// module --TLA+ BUILTINS--" (code 4357), which has no source position.
	text := "---- MODULE Target ----\nEXTENDS Naturals\n\nRECURSIVE Fact(_)\nFact(n) ==\n  IF n = 0 THEN 1 ELSE n * Fact(n - 1)\nTHEOREM Fact(0) = 1\nOBVIOUS\n\nHIDE Fact\n\na == Fact(3)\n\nb(n) == Fact(n)\n\nf[n \\in Nat] == n\n\n==== \n"
	r := checkText(t, "", "Target.tla", text, nil)
	requireCompleteCheck(t, r, nil, LevelComplete)
	if len(r.Diagnostics) != 2 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	sanyAt(t, r.Diagnostics[0], "semantic/4204", LevelSemantic, "Target.tla", 10, 6, 10, 9, "The operator Fact requires 1 arguments.")
	unlocated(t, r.Diagnostics[1], "semantic/4357", "Target.tla", "The only expression allowed as a fact in a HIDE is \nthe name of a theorem, assumption, or step.")

	// deep-cuts/model-one-shot/E-f7439160513c: 4271, then 4243 at a built-in.
	files := map[string]string{"M": "---- MODULE M ----\nCONSTANT F(_)\nA == F(1)\n====\n"}
	resolve := func(_ context.Context, name string) (Source, error) {
		if text, ok := files[name]; ok {
			return Source{Path: name + ".tla", Text: []byte(text), Origin: OriginWorkspace}, nil
		}
		return Source{}, ErrModuleNotFound
	}
	r = checkText(t, "", "Target.tla", "---- MODULE Target ----\nCONSTANT G\nINSTANCE M WITH F <- G\n====\n", resolve)
	requireCompleteCheck(t, r, nil, LevelComplete)
	if len(r.Diagnostics) != 2 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	sanyAt(t, r.Diagnostics[0], "semantic/4271", LevelSemantic, "Target.tla", 3, 22, 3, 22, "Expected arity 1 but found operator of arity 0.")
	unlocated(t, r.Diagnostics[1], "semantic/4243", "Target.tla", "An operator must be substituted for symbol 'F', and it must have arity 1.")
}

func TestPublicCheckPlusCalValidation(t *testing.T) {
	// inspection-pit-survivors/E-c7398d980c58: "--algorithm" inside a string
	// and a translation block; ParseAlgorithm.uncomment finds an unterminated
	// string, so SpecObj.validateParseUnit reports 4003 and SANY stops before
	// semantic analysis.
	other17 := "\\* Copyright 2026 Christopher O'Connell\n\\* All rights reserved\n\\* Aim: SANY.java:343 RemoveConditional_EQUAL_ELSE: validation adds a non-aborting error (algorithm marker not in a comment); baseline stops as a parse failure, mutant goes on to semantic analysis\n---- MODULE Other17 ----\nx == \"--algorithm foo\"\n\\* BEGIN TRANSLATION\ny == x\n\\* END TRANSLATION\n====\n"
	r := checkText(t, "", "Other17.tla", other17, nil)
	requireCompleteCheck(t, r, nil, LevelSemantic)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	unlocated(t, r.Diagnostics[0], "semantic/4003", "Other17.tla", "An unexpected problem was encountered attempting to validate the specification for Other17")

	// The same module with tab and mixed tab/space translation markers: Java
	// detabs before findTokenPair (Validator.java:126, 201, 207), so 4003 still
	// fires.
	for _, markers := range [][2]string{
		{"BEGIN\tTRANSLATION", "END\tTRANSLATION"},
		{"BEGIN \t TRANSLATION", "END\t\t  TRANSLATION"},
	} {
		tabbed := strings.Replace(strings.Replace(other17, "BEGIN TRANSLATION", markers[0], 1), "END TRANSLATION", markers[1], 1)
		r = checkText(t, "", "Other17.tla", tabbed, nil)
		requireCompleteCheck(t, r, nil, LevelSemantic)
		if len(r.Diagnostics) != 1 {
			t.Fatalf("%q: %+v", markers, r.Diagnostics)
		}
		unlocated(t, r.Diagnostics[0], "semantic/4003", "Other17.tla", "An unexpected problem was encountered attempting to validate the specification for Other17")
	}

	// deep-cuts/fuzz-jazzer/D-30f3dbe0534d: the error does not abort loading;
	// the missing-module abort follows it.
	text := "--------------------------- MODULE bug_05_12_10a ---------------------------\nEXTENDS Naturals, S(* \n--algorithm Pc \"Failure of assertion aock on termination. *)\n\\* END TRANSLATION - the hash of the generated TLA code (remove to silence divergence warnings): TLA-1e7f8dbaa6644662e3fa38b76045e01d\n===================================TAKE======================================\n"
	r = checkText(t, "", "bug_05_12_10a.tla", text, noModules)
	requireCompleteCheck(t, r, nil, LevelSemantic)
	if len(r.Diagnostics) != 2 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	unlocated(t, r.Diagnostics[0], "semantic/4003", "bug_05_12_10a.tla", "An unexpected problem was encountered attempting to validate the specification for bug_05_12_10a")
	if d := r.Diagnostics[1]; d.Code != "semantic/4220" || d.Message != "Cannot find source file for module S imported in module bug_05_12_10a." {
		t.Fatalf("%+v", d)
	}

	// fuzz-jazzer/D-a45c49c19e0e: a self-EXTENDS validates the root again;
	// the repeat is dropped, then the cycle aborts.
	text = "--------------------------- MODULE bug_05_12_10a ---------------------------\nEXTENDS bug_05_12_10a, Sequences, FiniteSets , TLC\n(* \n--algorithm Pcal\n             THEN /\\ Assert(exp[i] \\in STRING, \n                            \"Failure of assertion aock on termination. *)\nTerminating == pc = \"Done\" /\\ UNCHANGED vars\n\n\\* END TRANSLATION - the hash of the generated TLA code (remove to silence divergence warnings): TLA-1e7f8dbaa6644662e3fW38b76045e01d\n=============================================================================\n"
	r = checkText(t, "", "bug_05_12_10a.tla", text, noModules)
	requireCompleteCheck(t, r, nil, LevelSemantic)
	if len(r.Diagnostics) != 2 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	unlocated(t, r.Diagnostics[0], "semantic/4003", "bug_05_12_10a.tla", "An unexpected problem was encountered attempting to validate the specification for bug_05_12_10a")
	if d := r.Diagnostics[1]; d.Code != "semantic/4222" || d.Message != "Circular dependency among .tla files; dependency cycle is:\n\n  bug_05_12_10a.tla --> bug_05_12_10a.tla" {
		t.Fatalf("%+v", d)
	}

	// An ordinary PlusCal algorithm in a comment, with its translation,
	// validates (warnings, if any, are not modeled) and analysis continues.
	pcal := "---- MODULE Algo ----\nEXTENDS Naturals\n(* --algorithm Algo\nvariables x = 0;\nbegin\n  A: x := x + 1; \\* \"quoted\"\nend algorithm; *)\n\\* BEGIN TRANSLATION\nVARIABLES x, pc\nvars == << x, pc >>\nInit == x = 0 /\\ pc = \"A\"\nNext == pc = \"A\" /\\ x' = x + 1 /\\ pc' = \"Done\"\n\\* END TRANSLATION\n====\n"
	r = checkText(t, "", "Algo.tla", pcal, nil)
	requireCompleteCheck(t, r, nil, LevelComplete)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("ordinary PlusCal: %+v", r.Diagnostics)
	}
}

func TestPublicCheckFairnessIsSyntaxStage(t *testing.T) {
	par17 := "\\* Copyright 2026 Christopher O'Connell\n\\* All rights reserved\n\\* Aim: TLAplusParser.java:5551 RemoveConditional_EQUAL_ELSE: [310] kind check skipped: WF_[x](B) subscript is N_FcnAppl; loses the could-not-link error\n\\* Aim: TLAplusParser.java:5551 RemoveConditional_EQUAL_ELSE: [316] kind check skipped: WF_[x](B) subscript is N_FcnAppl; loses the could-not-link error\n\\* Aim: TLAplusParser.java:5551 RemoveConditional_EQUAL_ELSE: [322] kind check skipped: WF_[x](B) subscript is N_FcnAppl; loses the could-not-link error\n\\* Aim: TLAplusParser.java:5551 RemoveConditional_EQUAL_ELSE: [328] kind check skipped: WF_[x](B) subscript is N_FcnAppl; loses the could-not-link error\n\\* Aim: TLAplusParser.java:5551 RemoveConditional_EQUAL_ELSE: [334] kind check skipped: WF_[x](B) subscript is N_FcnAppl; loses the could-not-link error\n\\* Aim: TLAplusParser.java:5551 RemoveConditional_EQUAL_ELSE: [340] kind check skipped: WF_[x](B) subscript is N_FcnAppl; loses the could-not-link error\n\\* Aim: TLAplusParser.java:5551 RemoveConditional_EQUAL_ELSE: [346] kind check skipped: WF_[x](B) subscript is N_FcnAppl; loses the could-not-link error\n\\* Aim: TLAplusParser.java:5551 RemoveConditional_EQUAL_ELSE: [352] kind check skipped: WF_[x](B) subscript is N_FcnAppl; loses the could-not-link error\n\\* Aim: TLAplusParser.java:5551 RemoveConditional_EQUAL_ELSE: [358] kind check skipped: WF_[x](B) subscript is N_FcnAppl; loses the could-not-link error\n\\* Aim: TLAplusParser.java:5551 RemoveConditional_EQUAL_ELSE: [364] kind check skipped: WF_[x](B) subscript is N_FcnAppl; loses the could-not-link error\n---- MODULE Par17 ----\nVARIABLE x\nB == TRUE\nA == WF_[x](B)\n====\n"
	// SANY pushes "Error in fairness expression" onto the parser's PErrors
	// (TLAplusParser.java:5536, :5555): a parse error like any other, after
	// which ParseUnit adds its usual "Could not parse module" abort.
	for _, tc := range []struct {
		id, path, text, message string
		line, col               int
	}{
		{"deep-cuts/inspection/U-a9afbc587ef2", "RevFairArgs.tla", "---- MODULE RevFairArgs ----\nCONSTANT x, y, S\nA == WF_x(y, S)\n====\n", "Error in fairness expression at 3: 6\n", 3, 6},
		{"inspection-pit-survivors/E-3ab0b2a3dfd8", "Par17.tla", par17, "Error in fairness expression at 16: 6: could not link arguments\n", 16, 6},
	} {
		r := checkText(t, "", tc.path, tc.text, nil)
		if r.Completion != CompletionComplete || r.Level != LevelSyntax || len(r.Diagnostics) != 1 {
			t.Fatalf("%s: %+v", tc.id, r)
		}
		d := r.Diagnostics[0]
		if d.Code != CodeSyntax || d.Phase != LevelSyntax || d.Message != tc.message || d.Span.Start.Line+1 != tc.line || d.Span.Start.Column+1 != tc.col {
			t.Fatalf("%s: %+v", tc.id, d)
		}
	}
}

func TestPublicCheckExtendeeConflictSpan(t *testing.T) {
	// tlaplus-semantic-error-corpus/A-err-e4224: SANY locates the conflict at
	// "line 4, col 10 to line 4, col 12 of module E4224_Extended", which is
	// the dependency's own document.
	files := map[string]string{"E4224_Extended": "Used by E4224_Test as an EXTENDS target; the + operator type is a CONSTANT,\nas opposed to the + defined in Naturals which is an operator.\n---- MODULE E4224_Extended ----\nCONSTANT _+_\n====\n\n"}
	resolve := func(_ context.Context, name string) (Source, error) {
		if text, ok := files[name]; ok {
			return Source{Path: name + ".tla", Text: []byte(text), Origin: OriginWorkspace}, nil
		}
		return Source{}, ErrModuleNotFound
	}
	text := "Same identifier used for different symbol types in extended modules. For\nexample, in one module the symbol is an operator; in another, a constant.\n---- MODULE E4224_Test ----\nEXTENDS Naturals, E4224_Extended\n====\n\n"
	r := checkText(t, "", "E4224_Test.tla", text, resolve)
	requireCompleteCheck(t, r, nil, LevelComplete)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("%+v", r.Diagnostics)
	}
	sanyAt(t, r.Diagnostics[0], "semantic/4224", LevelSemantic, "E4224_Extended.tla", 4, 10, 4, 12, "The declaration of '+' conflicts with \nits definition at line 15, col 1 to line 15, col 19 of module Naturals.")
}
