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

// TestNullLabelBodyNamesSelectingSelector pins the position SANY reports when
// a selector reaches a label whose body is null. The LabelKind branch
// (Generator.java:1530-1533) steps idx back, so isNullSelection (1544) names
// the selector that selected the label, not the one after it. Recorded stock
// SANY output (tlaplus dc6a944) for fuzz-jazzer case D-71fc60f55bf7: the
// 144:26 entry is "argument selector 2" of Def23!(2,3,4)!2!1, where !2 selects
// the label (label2 :: p ++ C) whose body failed to resolve.
func TestNullLabelBodyNamesSelectingSelector(t *testing.T) {
	library, err := os.ReadFile(filepath.Join("testdata", "sentinel", "TLAPS.tla"))
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join("testdata", "sentinel", "D-71fc60f55bf7", "SubrefLL_test.tla"))
	if err != nil {
		t.Fatal(err)
	}
	resolve := func(_ context.Context, name string) (Source, error) {
		if name == "TLAPS" {
			return Source{Path: "TLAPS.tla", Text: library, Origin: OriginWorkspace}, nil
		}
		return Source{}, ErrModuleNotFound
	}
	r := checkText(t, "", "SubrefLL_test.tla", string(text), resolve)
	class, got := letErrors(t, r)
	if class != "semantic-error" {
		t.Fatalf("class %s: %+v", class, got)
	}
	m := "" // letErrors leaves the module empty for a single-file root
	want := []letSANYError{
		{m, 18, 42, "Couldn't resolve infix operator symbol `:>'."},
		{m, 19, 34, "Could not find declaration or definition of symbol ':>'."},
		{m, 25, 17, "Unknown operator: `u'."},
		{m, 37, 31, "Cannot find label `label2'."},
		{m, 43, 12, "Unknown operator: `Def13'."},
		{m, 44, 12, "Unknown operator: `Def13'."},
		{m, 45, 12, "Unknown operator: `Def13'."},
		{m, 46, 12, "Unknown operator: `Def13'."},
		{m, 51, 14, "Label label2 must contain formal parameter `x'."},
		{m, 51, 14, "Label label2 must contain formal parameter `y'."},
		{m, 51, 14, "Label label2 must contain formal parameter `z'."},
		{m, 51, 24, "Couldn't resolve infix operator symbol `++'."},
		{m, 51, 26, "Could not find declaration or definition of symbol '++'."},
		{m, 53, 1, "Operator Foo already defined or declared."},
		{m, 53, 17, "Unknown operator: `u'."},
		{m, 64, 9, "Unknown operator: `Def2'."},
		{m, 67, 9, "Unknown operator: `Def2'."},
		{m, 140, 1, "Operator Def23 already defined or declared."},
		{m, 144, 26, "An unexpected null node specified by 'argument selector 2'. This is probably due to a previous error."},
	}
	remainingSortErrors(got)
	remainingSortErrors(want)
	if len(got) != len(want) {
		t.Fatalf("%d errors, want %d: %+v", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("error %d: %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestNullLabelBodyFirstSelectorIsIncomplete covers a null label body met at
// the first suffix selector. SANY's sel.ops[0] is the root name, so
// Generator.java's stepped-back idx would name that name, not a suffix
// selector; prudence's selector slices are suffix-only and some callers start
// mid-chain, so the predecessor is not provable from source. The boundary is
// reported incomplete, not guessed.
func TestNullLabelBodyFirstSelectorIsIncomplete(t *testing.T) {
	text := "---- MODULE M ----\nQ == lab :: 1 ** 2\nASSUME Q!1\n====\n"
	r := checkText(t, "", "M.tla", text, noModules)
	if r.Completion != CompletionIncomplete {
		t.Fatalf("completion %v: %+v", r.Completion, r.Diagnostics)
	}
	last := r.Diagnostics[len(r.Diagnostics)-1]
	if last.Code != CodeIncomplete || !strings.Contains(last.Message, "generator.construct.label-null-body-first-selector") {
		t.Fatalf("last diagnostic %+v", last)
	}
	for _, d := range r.Diagnostics {
		if d.Severity == SeverityError && d.Span.Start.Line == 2 {
			t.Fatalf("guessed a verdict at the selector: %+v", d)
		}
	}
}
