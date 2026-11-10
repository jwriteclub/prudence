// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package semantic

import (
	stdcontext "context"
	"testing"

	"github.com/jwriteclub/prudence/internal/modulegraph"
)

// The pinned Generator.generateInstance (dc6a944) gates three instancee-copy
// branches on localness && topLevel. A proof-step INSTANCE has local() true
// (its zero array is non-null) and topLevel false, so it must reuse the
// instancee's node at Generator.java:5464 (an operator originally defined in
// a parameter-free module, reached through a parameterised instancee),
// Generator.java:5554 (the same for a theorem) and Generator.java:5597 (a
// theorem of a parameter-free instancee). Only Generator.java:5498, the
// operator branch of a parameter-free instancee, tests localness alone.
const (
	proofInstanceBase  = "---- MODULE Base ----\na ++ b == a\nTHEOREM Tp == TRUE\n====\n"
	proofInstanceParam = "---- MODULE Param ----\nEXTENDS Base\nCONSTANT c\n====\n"
)

func acquireProofInstanceGraph(t *testing.T, root string) *modulegraph.Acquisition {
	t.Helper()
	support := map[string]string{"Base": proofInstanceBase, "Param": proofInstanceParam}
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), modulegraph.Input{Path: "Root", Text: []byte(root)}, func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		return modulegraph.Input{Path: n, Text: []byte(support[n])}, nil
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func moduleByText(roots []*moduleNode, text string) *moduleNode {
	for _, m := range roots {
		if m.name.text == text {
			return m
		}
	}
	return nil
}

func symbolByText(c *context, text string) leafNode {
	for k, v := range c.table {
		if !k.module && k.name.text == text {
			return v.value
		}
	}
	return nil
}

func TestProofInstanceReusesTopLevelOnlyCopies(t *testing.T) {
	root := "---- MODULE Root ----\nCONSTANT c\n" +
		"THEOREM TRUE\n<1>1. INSTANCE Param\n<1>2. TRUE ++ TRUE\n  OBVIOUS\n<1>3. QED\n  OBVIOUS\n" +
		"THEOREM TRUE\n<1>1. INSTANCE Base\n<1>2. TRUE\n  BY Tp\n<1>3. QED\n  OBVIOUS\n====\n"
	details, _, top, roots, err := analyzeFlatExternalRoots(stdcontext.Background(), acquireProofInstanceGraph(t, root), 1000)
	if err != nil || len(details) != 0 || top == nil {
		t.Fatalf("error %v details %#v", err, details)
	}
	base := moduleByText(roots, "Base")
	plus, _ := symbolByText(base.context, "++").(*userOpDefNode)
	tp, _ := symbolByText(base.context, "Tp").(*thmOrAssumpDefNode)
	if plus == nil || tp == nil || len(top.theorems) != 2 {
		t.Fatal("fixture shape")
	}

	// Parameterised instancee, parameter-free origin (5464, 5554).
	first := top.theorems[0].proof.(*nonLeafProofNode)
	inst := first.steps[0].(*instanceNode)
	step := first.steps[1].(*theoremNode)
	if !inst.local || inst.module != moduleByText(roots, "Param") {
		t.Fatal("proof INSTANCE of Param")
	}
	if got := step.body.(*opApplNode).operator; got != plus {
		t.Fatalf("TRUE ++ TRUE resolves to %p, want Base's ++ %p (no 5464 copy)", got, plus)
	}
	if symbolByText(first.context, "++") != plus {
		t.Fatal("5464 reuse must add the original to the proof context")
	}
	if symbolByText(first.context, "Tp") != nil {
		t.Fatal("5554 reuse adds no theorem to the proof context")
	}

	// Parameter-free instancee (5498 copies, 5597 reuses).
	second := top.theorems[1].proof.(*nonLeafProofNode)
	if inst = second.steps[0].(*instanceNode); !inst.local || inst.module != base {
		t.Fatal("proof INSTANCE of Base")
	}
	facts := second.steps[1].(*theoremNode).proof.(*leafProofNode).facts
	if len(facts) != 1 || facts[0].(*opApplNode).operator != tp {
		t.Fatal("BY Tp must resolve to Base's Tp (no 5597 copy)")
	}
	if symbolByText(second.context, "Tp") != tp {
		t.Fatal("5597 reuse must add the original to the proof context")
	}
	clone, _ := symbolByText(second.context, "++").(*userOpDefNode)
	if clone == nil || clone == plus || !clone.local || clone.source != plus || clone.module != base || clone.body != plus.body {
		t.Fatalf("5498 must copy ++ for the LOCAL proof instance: %#v", clone)
	}

	// cm.appendDef runs for every operator, top-level or not, and for
	// theorems only at top level: Base's ++ reused, then the 5498 copy.
	if len(top.definitions) != 2 || top.definitions[0] != plus || top.definitions[1] != clone {
		t.Fatalf("Root definitions %v", top.definitions)
	}
}

// A top-level LOCAL INSTANCE keeps the copies at 5464 and 5554.
func TestTopLevelLocalInstanceKeepsCopies(t *testing.T) {
	root := "---- MODULE Root ----\nCONSTANT c\nLOCAL INSTANCE Param\nTHEOREM TRUE ++ TRUE BY Tp\n====\n"
	details, _, top, roots, err := analyzeFlatExternalRoots(stdcontext.Background(), acquireProofInstanceGraph(t, root), 1000)
	if err != nil || len(details) != 0 || top == nil {
		t.Fatalf("error %v details %#v", err, details)
	}
	base := moduleByText(roots, "Base")
	plus, _ := symbolByText(base.context, "++").(*userOpDefNode)
	tp, _ := symbolByText(base.context, "Tp").(*thmOrAssumpDefNode)
	opCopy, _ := symbolByText(top.context, "++").(*userOpDefNode)
	thmCopy, _ := symbolByText(top.context, "Tp").(*thmOrAssumpDefNode)
	if opCopy == nil || opCopy == plus || !opCopy.local || opCopy.source != plus || opCopy.module != base {
		t.Fatalf("5464 copy %#v", opCopy)
	}
	if thmCopy == nil || thmCopy == tp || !thmCopy.local || thmCopy.module != top || thmCopy.instantiatedFrom != moduleByText(roots, "Param") {
		t.Fatalf("5554 copy %#v", thmCopy)
	}
	thm := top.theorems[len(top.theorems)-1]
	if thm.body.(*opApplNode).operator != opCopy || thm.proof.(*leafProofNode).facts[0].(*opApplNode).operator != thmCopy {
		t.Fatal("top-level references must use the LOCAL copies")
	}
}
