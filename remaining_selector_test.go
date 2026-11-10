// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package prudence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Remaining selector chapter. selectorToNode's first FindingOpName search
// extends an undefined leading name with each later NameSel until the symbol
// table resolves a compound name, collecting the skipped segments' arguments
// in tempArgs (Generator.java:732-813); the selection then continues from
// the resolving segment exactly as for a defined first name
// (Generator.java:870-1000, 1897-1916). Operator-argument selectors that
// name a LET definition after an operand or colon selector continue with
// that definition's labels and operands (Generator.java:1121-1137,
// 1557-1566, 1960-2030).

// remainingCorpusCases are recorded sany-conformance cases. Their files are
// verbatim under testdata/remaining-selectors (provenance in PROVENANCE);
// want is the recorded stock-SANY output as compare normalises it. All of
// them stopped at unresolved-selector or label-operator-argument before.
var remainingCorpusCases = []struct {
	id, root string
	files    map[string]string // file name to SHA-256
	want     []letSANYError
}{
	{id: "B-specifyingsystems-fifo-apinnerfifo-d2848b", root: "APInnerFIFO.tla", files: map[string]string{
		"APInnerFIFO.tla": "8706d74891957f6d7ed84715fea05bd869212b47802dbbd96269fb4be15ed93c",
		"Channel.tla":     "8b2cbd4533a157ae0760e3117e3c008fb9a31dc3673005fc8b537d3879de1bfc",
		"InnerFIFO.tla":   "6971e25b26afd099fbeba9dd2f48e85dbfbd3461f75a01491613351211a6f907",
	}},
	{id: "C-specifyingsystems-fifo-apinnerfifo-d2848b-misalign-junction", root: "APInnerFIFO.tla", files: map[string]string{
		"APInnerFIFO.tla": "6101c999d49c9fe4f36478cfb363c8e2703bdb140cc66a21130e780a7cfb7322",
		"Channel.tla":     "8b2cbd4533a157ae0760e3117e3c008fb9a31dc3673005fc8b537d3879de1bfc",
		"InnerFIFO.tla":   "6971e25b26afd099fbeba9dd2f48e85dbfbd3461f75a01491613351211a6f907",
	}},
	{id: "B-test-model-suite-test216-dde257", root: "test216.tla", files: map[string]string{
		"test216.tla":  "3fa36e9038bcb6377fdc63c2db093aecf662001eff6db891589090d0a10bffc0",
		"test216a.tla": "c447061e3b8c9bfeccba01e4c6a7b9a9bdae9a255cd43b7e823275855a2590b1",
		"test216b.tla": "3c8a9ca7b2e3a4b0e25d6ddfb239ac060fb29c4e833effaa7259bf146a28186a",
		"test216c.tla": "09e98415f4b672f20cd0018a18f44de4ac034d83f1b63006f99565d09d6d8e58",
	}},
	{id: "B-test-model-suite-test219-0f575e", root: "test219.tla", files: map[string]string{
		"test206a.tla": "bf5ff7f32e07c37f2d7bee29d268f315f801fc0941192bcf5fc3a9a5378e89e0",
		"test219.tla":  "a5ad52bb53654a1bfbad6de843e49fa32c2f0ca5c91efac43013f14bdec61b98",
		"test219a.tla": "8695d02c5f2c6abddd2c89ce2e0d2a89bf9f0b42df3350c28276236c3c9811db",
	}},
	{id: "B-test-model-suite-test206-8e93d4", root: "test206.tla", files: map[string]string{
		"test206.tla":  "6c787f9ef900016de4be1068ec0f91721677f230bcf0510694c1053f02ba41e5",
		"test206a.tla": "bf5ff7f32e07c37f2d7bee29d268f315f801fc0941192bcf5fc3a9a5378e89e0",
	}},
	{id: "C-test-model-suite-test206-8e93d4-misalign-junction", root: "test206.tla", files: map[string]string{
		"test206.tla":  "b0b7f8e6ac4fc9e3ddbec625f701996f69a88bbe8b8e7866358254f7f60a35ac",
		"test206a.tla": "bf5ff7f32e07c37f2d7bee29d268f315f801fc0941192bcf5fc3a9a5378e89e0",
	}},
	{id: "C-test-model-suite-test219-0f575e-bad-arity", root: "test219.tla", files: map[string]string{
		"test206a.tla": "bf5ff7f32e07c37f2d7bee29d268f315f801fc0941192bcf5fc3a9a5378e89e0",
		"test219.tla":  "adcd175f1dca17464a44058308ee6b1bedab6f5079ecd3760d5729f8d34f0a36",
		"test219a.tla": "8695d02c5f2c6abddd2c89ce2e0d2a89bf9f0b42df3350c28276236c3c9811db",
	}, want: []letSANYError{
		{"test219", 153, 9, "The operator I requires 1 arguments."},
	}},
	{id: "C-general-tests-test206-ab9bff-unknown-name", root: "test206.tla", files: map[string]string{
		"test206.tla":  "05a6c82d7239b64e67730c66d8475a56dcca3756dc219b166595eb4727866407",
		"test206a.tla": "dbd5586ec9d61b5b69dd898d9ae33640f8bd3e00aa3897b47cd8871991734efe",
	}, want: []letSANYError{
		{"test206", 109, 8, "Argument number 1 to operator 'OOp2' should be a 2-parameter operator."},
		{"test206", 109, 13, "Unknown operator: `InstUndefd!Bar4!lab'."},
		{"test206", 144, 13, "Nonexistent operand specified by `>>'."},
	}},
}

func remainingSortErrors(errs []letSANYError) {
	sort.SliceStable(errs, func(i, j int) bool {
		a, b := errs[i], errs[j]
		if a.line != b.line {
			return a.line < b.line
		}
		if a.col != b.col {
			return a.col < b.col
		}
		return a.message < b.message
	})
}

func TestRemainingSelectorCorpusCases(t *testing.T) {
	for _, c := range remainingCorpusCases {
		t.Run(c.id, func(t *testing.T) {
			files := map[string]string{}
			for name, sum := range c.files {
				data, err := os.ReadFile(filepath.Join("testdata", "remaining-selectors", c.id, name))
				if err != nil {
					t.Fatal(err)
				}
				digest := sha256.Sum256(data)
				if hex.EncodeToString(digest[:]) != sum {
					t.Fatalf("%s is not the recorded upstream bytes", name)
				}
				files[name] = string(data)
			}
			r, err := Check(context.Background(), Request{Path: c.root, Origin: OriginWorkspace, Text: []byte(files[c.root]), Resolve: letCaseResolver(files)})
			if err != nil || r.Completion != CompletionComplete || r.Level != LevelComplete {
				t.Fatalf("%v %+v", err, r)
			}
			class, got := letErrors(t, r)
			wantClass := "ok"
			if len(c.want) != 0 {
				wantClass = "semantic-error"
			}
			want := make([]letSANYError, len(c.want))
			for i, w := range c.want {
				want[i] = letSANYError{line: w.line, col: w.col, message: letCollapse(w.message)}
			}
			remainingSortErrors(got)
			remainingSortErrors(want)
			if class != wantClass || len(got) != len(want) {
				t.Fatalf("class %s, want %s: %+v", class, wantClass, got)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("error %d: %+v, want %+v", i, got[i], want[i])
				}
			}
		})
	}
}

// remainingAt is the 1-based line and column of the first occurrence of
// marker in text.
func remainingAt(t *testing.T, text, marker string) (int, int) {
	t.Helper()
	i := strings.Index(text, marker)
	if i < 0 || strings.Count(text, marker) != 1 {
		t.Fatalf("marker %q must occur once", marker)
	}
	line := strings.Count(text[:i], "\n") + 1
	return line, i - strings.LastIndex(text[:i], "\n")
}

// remainingCheck runs Root over helper modules N (instantiated by M as I and
// as the parameterized P) and M, which Root instantiates without a name: the
// definitions I!... and P!... exist in Root, I and P do not.
func remainingCheck(t *testing.T, body string) (string, Result) {
	t.Helper()
	files := letModules(map[string]string{
		"N": "Op == 1\nF(a) == a\nG(a) == lab :: <<a, 2>>\nTHEOREM Thm == 1 = 1",
		"M": "I == INSTANCE N\nP(x) == INSTANCE N",
	})
	text := "---- MODULE Root ----\nINSTANCE M\nApply(Op(_), x) == Op(x)\n" +
		"K(q) == \\E x \\in {q} : lab(x) :: LET a (+) b == {inner(y) :: <<y, a, b>> : y \\in {x}}\n" +
		"                                 IN 1 (+) 2\n" +
		"Use4(Op(_, _, _, _)) == Op(1, 2, 3, 4)\nUse5(Op(_, _, _, _, _)) == Op(1, 2, 3, 4, 5)\n" +
		body + "\n====\n"
	files["Root.tla"] = text
	r, err := Check(context.Background(), Request{Path: "Root.tla", Origin: OriginWorkspace, Text: []byte(text), Resolve: letCaseResolver(files)})
	if err != nil {
		t.Fatal(err)
	}
	return text, r
}

// remainingExpect checks a complete result's errors against markers: each
// want entry is a marker in the Root text and the message reported there.
func remainingExpect(t *testing.T, text string, r Result, want [][2]string) {
	t.Helper()
	if r.Completion != CompletionComplete {
		t.Fatalf("%+v", r.Diagnostics)
	}
	_, got := letErrors(t, r)
	var expected []letSANYError
	for _, w := range want {
		line, col := remainingAt(t, text, w[0])
		expected = append(expected, letSANYError{line: line, col: col, message: letCollapse(w[1])})
	}
	remainingSortErrors(got)
	remainingSortErrors(expected)
	if len(got) != len(expected) {
		t.Fatalf("%+v, want %+v", got, expected)
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Fatalf("error %d: %+v, want %+v", i, got[i], expected[i])
		}
	}
}

func TestRemainingSelectorBoundaries(t *testing.T) {
	t.Run("compound names resolve", func(t *testing.T) {
		// A1-A4 resolve at the final selector; A3 passes P's tempArgs and
		// F's own argument. A5-A7 continue after the resolving segment,
		// A7 into a theorem with tempArgs. A8 and A9 are operator
		// arguments.
		text, r := remainingCheck(t, "A1 == I!Op\nA2 == I!F(1)\nA3 == P(1)!F(2)\nA4 == I!Thm\n"+
			"A5 == I!G(1)!lab!2\nA6 == P(1)!G(2)!lab!1\nA7 == P(1)!Thm!1\n"+
			"A8 == Apply(I!F, 1)\nA9 == Apply(I!G!lab, 1)")
		remainingExpect(t, text, r, nil)
	})
	t.Run("arity", func(t *testing.T) {
		// "The operator %s requires %s arguments." names the compound and
		// is located at the resolving segment's arguments, or at the
		// selector when it has none (Generator.java:897-905).
		text, r := remainingCheck(t, "B1 == I!F\nB2 == I!F(1, 2)\nB3 == P!Op\nB4 == I!G!lab\nB5 == P!G(2)!lab\nB6 == P!Thm!1")
		remainingExpect(t, text, r, [][2]string{
			{"I!F\n", "The operator I!F requires 1 arguments."},
			{"(1, 2)", "The operator I!F requires 1 arguments."},
			{"P!Op", "The operator P!Op requires 1 arguments."},
			{"I!G!lab", "The operator I!G requires 1 arguments."},
			{"(2)!lab", "The operator P!G requires 2 arguments."},
			{"P!Thm", "The operator P!Thm requires 1 arguments."},
		})
	})
	t.Run("unknown compound", func(t *testing.T) {
		text, r := remainingCheck(t, "C1 == I!Nope\nC2 == I!Nope!lab")
		remainingExpect(t, text, r, [][2]string{
			{"I!Nope\n", "Unknown operator: `I!Nope'."},
			{"I!Nope!lab", "Unknown operator: `I!Nope!lab'."},
		})
	})
	t.Run("operator argument arity", func(t *testing.T) {
		text, r := remainingCheck(t, "D1 == Apply(I!Op, 1)")
		remainingExpect(t, text, r, [][2]string{
			{"Apply(I!Op", "Argument number 1 to operator 'Apply' should be a 1-parameter operator."},
			{"I!Op,", "Expected arity 1 but found operator of arity 0."},
		})
	})
	t.Run("operator mode drops later segment arguments", func(t *testing.T) {
		// The compound search collects G's argument in tempArgs, which an
		// operator argument never generates or checks; that source quirk
		// keeps the explicit boundary.
		_, r := remainingCheck(t, "E1 == Apply(I!G(1)!lab, 1)")
		if r.Completion != CompletionIncomplete || len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "(generator.construct.unresolved-selector)") {
			t.Fatalf("%+v", r.Diagnostics)
		}
	})
	t.Run("LET definitions in operator arguments", func(t *testing.T) {
		// After an operand or colon selector a name is sought in the LET
		// context, as an operator token after resolveSynonym; its labels
		// and operands follow. \oplus selects the (+) definition.
		text, r := remainingCheck(t, "F1 == Use5(K!lab!:!\\oplus!inner)\nF2 == Use5(K!@!\\oplus!inner)\n"+
			"F3 == Use5(K!lab!:!\\oplus!@)\nF4 == Use4(K!lab!:!\\oplus)")
		remainingExpect(t, text, r, nil)
	})
	t.Run("LET definition errors", func(t *testing.T) {
		// An undefined LET name extends with the later NameSel selectors
		// into the reported compound (Generator.java:787-813).
		text, r := remainingCheck(t, "G1 == Use5(K!lab!:!zz!inner)\nG2 == Use4(K!lab!:!\\oplus!inner)\nG3 == Use5(K!lab!:!\\oplus!none)")
		remainingExpect(t, text, r, [][2]string{
			{"Use5(K!lab!:!zz", "Argument number 1 to operator 'Use5' should be a 5-parameter operator."},
			{"zz!inner", "Unknown operator: `zz!inner'."},
			{"Use4(K!lab!:!\\oplus!inner", "Argument number 1 to operator 'Use4' should be a 4-parameter operator."},
			{"K!lab!:!\\oplus!inner)\nG3", "Expected arity 4 but found operator of arity 5."},
			{"Use5(K!lab!:!\\oplus!none", "Argument number 1 to operator 'Use5' should be a 5-parameter operator."},
			{"none", "Cannot find label `none'."},
		})
	})
	t.Run("undefined proof step", func(t *testing.T) {
		// Proof-step lexemes are NameSel too (Generator.java:409-418).
		text, r := remainingCheck(t, "THEOREM T == TRUE\n<1>1. <1>9!lab\n  OBVIOUS\n<1> QED\n  OBVIOUS")
		remainingExpect(t, text, r, [][2]string{
			{"<1>9!lab", "Unknown operator: `<1>9!lab'."},
		})
	})
}
