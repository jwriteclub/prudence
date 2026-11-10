// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package prudence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Clean-rejection chapter. Recorded SANY semantic rejections that stopped
// at an unported recovery path now follow SANY's own recovery:
//
//   - instance markers followed by a selector that is not a NameSel report
//     the markers' compound name as unknown (Generator.java:732-813, 846);
//   - an ASSUME/PROVE clause or a label body that is null after an error is
//     reported by isNullSelection (Generator.java:1446-1450, 1544-1556);
//   - an operand selector on a null definition body reports a failed
//     selection, and on nullOAN a missing operand (Generator.java:1138-1145,
//     1394-1400), also before a LET name;
//   - an @ step whose infix operator does not resolve ends generateProof
//     (Generator.java:6150-6159), admitted where nothing generated later can
//     observe the contexts Java leaves pushed.

// cleanRejectionCases are recorded sany-conformance cases, verbatim under
// testdata/clean-rejections (provenance in PROVENANCE). want is stock SANY's
// error list in its printed order (sany-conformance manifest stdout), with
// whitespace runs collapsed; line 0 is SANY's location-less entry ("In
// module --TLA+ BUILTINS--"). site is the incomplete origin before this
// chapter.
var cleanRejectionCases = []struct {
	set, id, site, root string
	files               map[string]string // file name to SHA-256
	want                []letSANYError
}{
	{set: "deep-cuts/inspection", id: "U-4103c37b50b1", site: "qualified-instance", root: "AtPrefix.tla", files: map[string]string{
		"AtPrefix.tla": "eaeda83f92f94d78264d71f43384594dbb78019a938c7983bb3bbe88067a5c2f",
	}, want: []letSANYError{
		{"AtPrefix", 6, 6, "Unknown operator: `I'."},
		{"AtPrefix", 7, 30, "Unknown operator: `I'."},
	}},
	{set: "mutants", id: "C-tests-specs-ovrentrylet-a281d5-delete-token", site: "qualified-instance", root: "OvrEntryLet.tla", files: map[string]string{
		"OvrEBase.tla":    "a221daefc7644a88159c5f9dd4c78502e0d5781092c82a06ed7d94ed5182cf92",
		"OvrEntryLet.tla": "820708de5aeb1d3d7c7087da59377b4be5e72308306d8499fd98275626d77b49",
	}, want: []letSANYError{
		{"OvrEntryLet", 8, 45, "Unknown operator: `CC'."},
	}},
	{set: "fuzz-jazzer", id: "D-2ebde0f7e165", site: "definition-level-selector", root: "test210.tla", files: map[string]string{
		"test210.tla": "f19e7c296eca873dc525f46b084cf5f69e0bc6f9dc93dd4e08aff74bcad7d7cf",
	}, want: []letSANYError{
		{"test210", 33, 41, "Label not allowed within scope of declaration in nested ASSUME/PROVE."},
		{"test210", 34, 30, "Label not allowed within scope of declaration in nested ASSUME/PROVE."},
		{"test210", 38, 6, "Could not find declaration or definition of symbol '*'."},
		{"test210", 35, 23, "Couldn't resolve infix operator symbol `*'."},
		{"test210", 44, 18, "An unexpected null node specified by 'argument selector 5'. This is probably due to a previous error."},
		{"test210", 45, 18, "An unexpected null node specified by 'argument selector 5'. This is probably due to a previous error."},
		{"test210", 46, 16, "Accessing non-existent subexpression of a SUFFICES"},
		{"test210", 53, 15, "Accessing subexpression labeled `laby' of ASSUME/PROVE clause within the scope of a declaration from outside that declaration's scope."},
		{"test210", 54, 15, "Accessing subexpression labeled `labi' of ASSUME/PROVE clause within the scope of a declaration from outside that declaration's scope."},
		{"test210", 55, 15, "Accessing subexpression labeled `labu' of ASSUME/PROVE clause within the scope of a declaration from outside that declaration's scope."},
		{"test210", 56, 15, "Accessing ASSUME/PROVE clause within the scope of a declaration from outside that declaration's scope."},
	}},
	{set: "fuzz-jazzer", id: "D-3bb7d96df8a9", site: "definition-level-selector", root: "Target.tla", files: map[string]string{
		"Target.tla": "41e82fc80cb1dce2910a9cacd6feceb933b1f5466c1d13a3995b13754f7a3974",
	}, want: []letSANYError{
		{"Target", 3, 17, "Could not find declaration or definition of symbol '/'."},
		{"Target", 3, 10, "Couldn't resolve infix operator symbol `/'."},
		{"Target", 5, 8, "An unexpected null node specified by 'argument selector 1'. This is probably due to a previous error."},
		{"--TLA+ BUILTINS--", 0, 0, "The only expression allowed as a fact in a HIDE is the name of a theorem, assumption, or step."},
	}},
	{set: "fuzz-jazzer", id: "D-ee4205c16ded", site: "definition-level-selector", root: "test210.tla", files: map[string]string{
		"test210.tla": "207df24d811fe7e1d42d3cac4e859db99eea8ee0f1d759721f2296b6fbb27ef7",
	}, want: []letSANYError{
		{"test210", 5, 21, "Unknown operator: `lab'."},
		{"test210", 5, 32, "Unknown operator: `assump'."},
		{"test210", 15, 24, "Selected a NEW declaration as an expression or operator."},
		{"test210", 20, 11, "Cannot find label `labx'."},
		{"test210", 23, 16, "Cannot find label `labx'."},
		{"test210", 24, 16, "Accessing subexpression labeled `laby' of ASSUME/PROVE clause within the scope of a declaration from outside that declaration's scope."},
		{"test210", 26, 16, "Accessing ASSUME/PROVE clause within the scope of a declaration from outside that declaration's scope."},
		{"test210", 33, 41, "Label not allowed within scope of declaration in nested ASSUME/PROVE."},
		{"test210", 34, 30, "Label not allowed within scope of declaration in nested ASSUME/PROVE."},
		{"test210", 38, 6, "Could not find declaration or definition of symbol '*'."},
		{"test210", 35, 23, "Couldn't resolve infix operator symbol `*'."},
		{"test210", 44, 18, "An unexpected null node specified by 'argument selector 5'. This is probably due to a previous error."},
		{"test210", 45, 11, "Unknown operator: `<0>2'."},
		{"test210", 46, 16, "Accessing non-existent subexpression of a SUFFICES"},
		{"test210", 53, 15, "Accessing subexpression labeled `laby' of ASSUME/PROVE clause within the scope of a declaration from outside that declaration's scope."},
		{"test210", 54, 15, "Accessing subexpression labeled `labi' of ASSUME/PROVE clause within the scope of a declaration from outside that declaration's scope."},
		{"test210", 55, 15, "Accessing subexpression labeled `labu' of ASSUME/PROVE clause within the scope of a declaration from outside that declaration's scope."},
	}},
	{set: "fuzz-jazzer", id: "D-63187a1f9c31", site: "simple-label-selector-mode", root: "SubrefLL_test.tla", files: map[string]string{
		"SubrefLL_test.tla": "1cd5812b425de9d3277b92193d0cd538328a27f55f6706e655418fc9bbe46490",
		"TLAPS.tla":         "9afe54984062748a0568966434cc0945d682f8cd89fdbc38f73b5579751b0c55",
	}, want: []letSANYError{
		{"SubrefLL_test", 7, 1, "Empty BY, USE, or HIDE"},
		{"SubrefLL_test", 7, 1, "Empty USE or HIDE statement."},
		{"SubrefLL_test", 7, 5, "Operator c4 already defined or declared."},
		{"SubrefLL_test", 9, 12, "Unknown operator: `s'."},
		{"SubrefLL_test", 9, 16, "Unknown operator: `s'."},
		{"SubrefLL_test", 7, 5, "Multiply-defined symbol 'c4': this definition or declaration conflicts with the one at line 6, col 1 to line 6, col 18 of module SubrefLL_test."},
		{"SubrefLL_test", 10, 9, "Unknown operator: `Salt'."},
		{"SubrefLL_test", 10, 9, "DEF clause entry should describe a defined operator."},
		{"SubrefLL_test", 10, 1, "Empty USE or HIDE statement."},
		{"SubrefLL_test", 14, 29, "Illegal parameter i of label `lab'."},
		{"SubrefLL_test", 14, 23, "Label lab must contain formal parameter `y'."},
		{"SubrefLL_test", 14, 23, "Label lab declares extra parameter(s) i"},
		{"SubrefLL_test", 24, 16, "Could not find declaration or definition of symbol '&'."},
		{"SubrefLL_test", 24, 4, "Couldn't resolve infix operator symbol `&'."},
		{"SubrefLL_test", 27, 19, "Subexpression selection failed, probably due to an error in the selected expression."},
		{"SubrefLL_test", 27, 52, "Unknown operator: `Salt'."},
		{"SubrefLL_test", 30, 19, "Subexpression selection failed, probably due to an error in the selected expression."},
		{"SubrefLL_test", 30, 50, "Unknown operator: `Salt'."},
		{"SubrefLL_test", 31, 19, "Subexpression selection failed, probably due to an error in the selected expression."},
		{"SubrefLL_test", 31, 47, "Label not in definition or proof step."},
		{"SubrefLL_test", 34, 19, "Subexpression selection failed, probably due to an error in the selected expression."},
		{"SubrefLL_test", 34, 52, "Unknown operator: `Salt'."},
		{"SubrefLL_test", 37, 19, "Subexpression selection failed, probably due to an error in the selected expression."},
		{"SubrefLL_test", 37, 50, "Unknown operator: `Salt'."},
	}},
	{set: "fuzz-jazzer", id: "D-d970fe3c26fb", site: "simple-label-selector-mode", root: "SubrefLL_test.tla", files: map[string]string{
		"SubrefLL_test.tla": "9ce66e2b90f111cb1c581cc57f56b392a6427e2622ead807324ba5b289b14298",
		"TLAPS.tla":         "9afe54984062748a0568966434cc0945d682f8cd89fdbc38f73b5579751b0c55",
	}, want: []letSANYError{
		{"SubrefLL_test", 14, 29, "Unknown operator: `A'."},
		{"SubrefLL_test", 18, 9, "Unknown operator: `Fonnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnno'."},
		{"SubrefLL_test", 19, 12, "Unknown operator: `Foo'."},
		{"SubrefLL_test", 19, 12, "DEF clause entry should describe a defined operator."},
		{"SubrefLL_test", 21, 9, "Unknown operator: `Op!lab!label2'."},
		{"SubrefLL_test", 21, 40, "Unknown operator: `Op!lab!++'."},
		{"SubrefLL_test", 28, 4, "Unknown operator: `CHOO'."},
		{"SubrefLL_test", 29, 9, "Unknown operator: `Op!lab!label2'."},
		{"SubrefLL_test", 29, 40, "Unknown operator: `Op!lab!++'."},
		{"SubrefLL_test", 33, 1, "Operator Op3 already defined or declared."},
		{"SubrefLL_test", 35, 1, "Operator COp already defined or declared."},
		{"SubrefLL_test", 36, 54, "Unknown operator: `x'."},
		{"SubrefLL_test", 36, 19, "Illegal parameter x of label `CHOOSEFUNCTION'."},
		{"SubrefLL_test", 36, 4, "Label CHOOSEFUNCTION declares extra parameter(s) x"},
		{"SubrefLL_test", 39, 19, "Choosing operand `!(...)' of subexpression with no operands."},
		{"SubrefLL_test", 42, 9, "Unknown operator: `CO0'."},
		{"SubrefLL_test", 45, 19, "Choosing operand `!(...)' of subexpression with no operands."},
		{"SubrefLL_test", 48, 19, "Choosing operand `!(...)' of subexpression with no operands."},
		{"SubrefLL_test", 65, 9, "Unknown operator: `cViolated'."},
	}},
	{set: "model-one-shot", id: "E-257dd8bdb327", site: "proof-at-expression", root: "Target.tla", files: map[string]string{
		"Target.tla": "0cd051955be2a836264b237e341bc8d55cb0140de0cfbf91ea4258cedfa32546",
	}, want: []letSANYError{
		{"Target", 4, 8, "Could not find declaration or definition of symbol '\\o'."},
		{"Target", 4, 6, "Couldn't resolve infix operator symbol `\\o'."},
	}},
}

// cleanOrderedErrors returns a result's errors in emission order, with the
// module named by the diagnostic path and 1-based positions. A diagnostic
// with an empty span is rendered at line 0, column 0.
func cleanOrderedErrors(t *testing.T, r Result) []letSANYError {
	t.Helper()
	var got []letSANYError
	for _, d := range r.Diagnostics {
		if d.Code == CodeIncomplete {
			t.Fatalf("incomplete: %+v", r.Diagnostics)
		}
		if d.Severity != SeverityError {
			continue
		}
		e := letSANYError{module: strings.TrimSuffix(d.Path, ".tla"), line: d.Span.Start.Line + 1, col: d.Span.Start.Column + 1, message: letCollapse(d.Message)}
		if d.Span == (Span{}) {
			e.line, e.col = 0, 0
		}
		got = append(got, e)
	}
	return got
}

// cleanCompareOrdered compares errors in order. A location-less SANY entry
// matches by text alone, whatever path prudence attaches to it.
func cleanCompareOrdered(t *testing.T, got, want []letSANYError) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d errors, want %d:\n%+v", len(got), len(want), got)
	}
	for i := range want {
		w := want[i]
		w.message = letCollapse(w.message)
		g := got[i]
		if w.line == 0 {
			if g.line != 0 || g.message != w.message {
				t.Fatalf("error %d: %+v, want location-less %q", i, g, w.message)
			}
			continue
		}
		if g != w {
			t.Fatalf("error %d: %+v, want %+v", i, g, w)
		}
	}
}

func TestCleanRejectionCorpusCases(t *testing.T) {
	for _, c := range cleanRejectionCases {
		t.Run(c.id, func(t *testing.T) {
			files := map[string]string{}
			for name, sum := range c.files {
				data, err := os.ReadFile(filepath.Join("testdata", "clean-rejections", c.id, name))
				if err != nil {
					t.Fatal(err)
				}
				digest := sha256.Sum256(data)
				if hex.EncodeToString(digest[:]) != sum {
					t.Fatalf("%s is not the recorded case bytes", name)
				}
				files[name] = string(data)
			}
			r, err := Check(context.Background(), Request{Path: c.root, Origin: OriginWorkspace, Text: []byte(files[c.root]), Resolve: letCaseResolver(files)})
			if err != nil || r.Completion != CompletionComplete {
				t.Fatalf("%v %+v", err, r)
			}
			cleanCompareOrdered(t, cleanOrderedErrors(t, r), c.want)
		})
	}
}

// cleanWant is an expected error: the first occurrence of marker in the
// root text, moved offset bytes right, and the message reported there.
type cleanWant struct {
	marker  string
	offset  int
	message string
}

// cleanCheck runs Root.tla with helper modules N (an operator and a
// theorem) and M (which instantiates N as K).
func cleanCheck(t *testing.T, text string) Result {
	t.Helper()
	files := letModules(map[string]string{"N": "Op == 1\nTHEOREM Thm == TRUE", "M": "K == INSTANCE N"})
	files["Root.tla"] = text
	r, err := Check(context.Background(), Request{Path: "Root.tla", Origin: OriginWorkspace, Text: []byte(text), Resolve: letCaseResolver(files)})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// cleanExpect compares a complete result's errors, in order, with want.
func cleanExpect(t *testing.T, text string, r Result, want []cleanWant) {
	t.Helper()
	if r.Completion != CompletionComplete {
		t.Fatalf("%+v", r.Diagnostics)
	}
	expected := make([]letSANYError, len(want))
	for i, w := range want {
		line, col := remainingAt(t, text, w.marker)
		expected[i] = letSANYError{module: "Root", line: line, col: col + w.offset, message: w.message}
	}
	cleanCompareOrdered(t, cleanOrderedErrors(t, r), expected)
}

// cleanIncomplete requires an incomplete result stopped at site.
func cleanIncomplete(t *testing.T, r Result, site string) {
	t.Helper()
	if r.Completion != CompletionIncomplete {
		t.Fatalf("complete: %+v", r.Diagnostics)
	}
	for _, d := range r.Diagnostics {
		if d.Code == CodeIncomplete && strings.Contains(d.Message, "("+site+")") {
			return
		}
	}
	t.Fatalf("no incomplete stop at %s: %+v", site, r.Diagnostics)
}

func TestCleanRejectionBoundaries(t *testing.T) {
	t.Run("instance markers before a structural selector", func(t *testing.T) {
		// FindingOpName leaves the compound name unchanged at a selector
		// that is not a NameSel; nothing resolves, so the markers' compound
		// name is unknown, located at their name selectors
		// (Generator.java:732-813, 846). A NullSel's arguments only join
		// tempArgs and are never generated (790-800); a marker's own
		// arguments are generated after its arity check (878-922), before
		// the error; a missing marker argument is the arity error at the
		// whole selector (897-905).
		text := "---- MODULE Root ----\nI == INSTANCE N\nP(x) == INSTANCE N\nJ == INSTANCE M\n" +
			"Q1 == I!@\nQ2 == I!:\nQ3 == I!(Undef1)\nQ4 == J!K!<<\nQ5 == P(Undef2)!>>\nQ6 == P!@\n====\n"
		cleanExpect(t, text, cleanCheck(t, text), []cleanWant{
			{"I!@", 0, "Unknown operator: `I'."},
			{"I!:", 0, "Unknown operator: `I'."},
			{"I!(Undef1)", 0, "Unknown operator: `I'."},
			{"J!K!<<", 0, "Unknown operator: `J!K'."},
			{"Undef2", 0, "Unknown operator: `Undef2'."},
			{"P(Undef2)", 0, "Unknown operator: `P'."},
			{"P!@", 0, "The operator P requires 1 arguments."},
		})
	})
	t.Run("null ASSUME/PROVE clause or label body", func(t *testing.T) {
		// isNullSelection reports a null assumption (Generator.java:
		// 1446-1450), a null PROVE (1544-1547) and a null label body
		// unwrapped before a LET name (1550-1556) at the selector that
		// chose the clause.
		text := "---- MODULE Root ----\nTHEOREM T1 == ASSUME 1 \\o 2 PROVE TRUE\n" +
			"THEOREM T2 == ASSUME TRUE PROVE 3 \\o 4\nTHEOREM T3 == ASSUME TRUE PROVE lab :: 5 \\o 6\n" +
			"D1 == T1!1\nD2 == T2!2\nD3 == T3!2!x\n====\n"
		null := "An unexpected null node specified by 'argument selector %s'. This is probably due to a previous error."
		cleanExpect(t, text, cleanCheck(t, text), []cleanWant{
			{"\\o 2", 0, "Could not find declaration or definition of symbol '\\o'."},
			{"1 \\o 2", 0, "Couldn't resolve infix operator symbol `\\o'."},
			{"\\o 4", 0, "Could not find declaration or definition of symbol '\\o'."},
			{"3 \\o 4", 0, "Couldn't resolve infix operator symbol `\\o'."},
			{"\\o 6", 0, "Could not find declaration or definition of symbol '\\o'."},
			{"5 \\o 6", 0, "Couldn't resolve infix operator symbol `\\o'."},
			{"T1!1", 3, strings.Replace(null, "%s", "1", 1)},
			{"T2!2", 3, strings.Replace(null, "%s", "2", 1)},
			{"T3!2!x", 3, strings.Replace(null, "%s", "2", 1)},
		})
	})
	t.Run("operand selector on a failed body before a LET name", func(t *testing.T) {
		// FindingSubExpr on a null body reports a failed selection
		// (Generator.java:1138-1145); on nullOAN, whose operator is the
		// kind-0 null OpDefNode, a missing operand (1394-1400). The
		// definition's own arguments were generated first (878-922).
		text := "---- MODULE Root ----\nBad(p) == p \\o 1\nUnd(p) == Undef\n" +
			"S1 == Bad(1)!1!foo\nS2 == Und(1)!1!foo\nS3 == Und(1)!(2)!foo\n====\n"
		cleanExpect(t, text, cleanCheck(t, text), []cleanWant{
			{"\\o 1", 0, "Could not find declaration or definition of symbol '\\o'."},
			{"p \\o 1", 0, "Couldn't resolve infix operator symbol `\\o'."},
			{"Undef", 0, "Unknown operator: `Undef'."},
			{"Bad(1)!1!foo", 7, "Subexpression selection failed, probably due to an error in the selected expression."},
			{"Und(1)!1!foo", 7, "Choosing operand `argument selector 1' of subexpression with no operands."},
			{"Und(1)!(2)!foo", 7, "Choosing operand `!(...)' of subexpression with no operands."},
		})
	})
	t.Run("distinct-name label extras print the full list", func(t *testing.T) {
		// The extra-parameter list iterates a HashSet of FormalParamNodes
		// only to append names (Generator.java:7092-7097); outside the
		// bounded label cursor prudence prints every distinct name in
		// UTF-16 order (the approved fallback). The body's errors come first, then the
		// illegal dummy parameters, as in recorded E-d0e6bde23d7c.
		text := "---- MODULE Root ----\nA == L(a, b) :: a = b\n====\n"
		cleanExpect(t, text, cleanCheck(t, text), []cleanWant{
			{"a = b", 0, "Unknown operator: `a'."},
			{"a = b", 4, "Unknown operator: `b'."},
			{"a, b)", 0, "Illegal parameter a of label `L'."},
			{"b) ::", 0, "Illegal parameter b of label `L'."},
			{"L(a, b)", 0, "Label L declares extra parameter(s) a b"},
		})
	})
	at := "THEOREM TRUE\n<1>1 1 = 1\n<1>2 @ \\o 1\n<1>3 Undef\n<1> QED\n"
	t.Run("@ step with an unresolved operator ends the proof", func(t *testing.T) {
		// generateGenID reports the operator, the step reports the
		// expression, and generateProof returns null (Generator.java:
		// 6150-6159): <1>3 and QED are never generated. The theorem is the
		// last unit of the outermost module, so nothing observes the
		// contexts left pushed.
		text := "---- MODULE Root ----\n" + at + "====\n"
		cleanExpect(t, text, cleanCheck(t, text), []cleanWant{
			{"\\o 1", 0, "Could not find declaration or definition of symbol '\\o'."},
			{"@ \\o 1", 0, "Couldn't resolve infix operator symbol `\\o'."},
		})
	})
	t.Run("@ step abort observable later stays incomplete", func(t *testing.T) {
		// A later unit, a nested proof whose parent proof continues, and an
		// inner module all run on with the contexts and label frames Java
		// left pushed (Generator.java:6159, 6399, 2240-2246).
		for _, text := range []string{
			"---- MODULE Root ----\n" + at + "Foo == 1\n====\n",
			"---- MODULE Root ----\nTHEOREM TRUE\n<1>1 TRUE\n  <2>1 1 = 1\n  <2>2 @ \\o 1\n  <2> QED\n<1> QED\n====\n",
			"---- MODULE Root ----\n---- MODULE Inner ----\n" + at + "====\n====\n",
		} {
			cleanIncomplete(t, cleanCheck(t, text), "generator.construct.proof-at-expression")
		}
	})
}
