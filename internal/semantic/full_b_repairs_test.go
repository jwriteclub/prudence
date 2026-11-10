// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package semantic

import (
	stdcontext "context"
	"errors"
	"testing"

	"github.com/jwriteclub/prudence/internal/modulegraph"
)

func TestSelectorClassificationPrecedesResolution(t *testing.T) {
	tree := parsedModule(t, "---- MODULE Target ----\nUSE Expr!⟩\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, err := g.module(tree)
	var abort *Abort
	msgs := g.table.errors.Messages()
	if !errors.As(err, &abort) || len(msgs) != 1 || msgs[0].Code.Value != 4003 || msgs[0].Format != "Internal error: Unexpected selector `%s'." || msgs[0].Parameters[0].Text != "⟩" || msgs[0].Location.BeginColumn != 10 {
		t.Fatalf("error %v messages %#v", err, msgs)
	}
}

func TestDefArgumentsBeforeUnknownResolution(t *testing.T) {
	tree := parsedModule(t, "---- MODULE Target ----\nUSE DEF Op(F)!F\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, err := g.module(tree)
	msgs := g.table.errors.Messages()
	if err != nil || len(msgs) != 3 || msgs[0].Code.Value != 4005 || msgs[0].Parameters[0].Text != "Op" || msgs[1].Code.Value != 4200 || msgs[2].Code.Value != 4003 {
		t.Fatalf("error %v messages %#v", err, msgs)
	}
}

func TestInstanceNullTheoremBodyDiagnostic(t *testing.T) {
	target := "---- MODULE Target ----\nI == INSTANCE Aux WITH c <- 1\n====\n"
	aux := "---- MODULE Aux ----\nCONSTANT c\nTHEOREM Th == c > 0\n====\n"
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), modulegraph.Input{Path: "Target", Text: []byte(target)}, func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		return modulegraph.Input{Path: n, Text: []byte(aux)}, nil
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	msgs, _, _, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil || len(msgs) != 3 || msgs[2].Code.Value != 4004 || msgs[2].Format != "Substitution error, probably due to error in module being instantiated." {
		t.Fatalf("error %v messages %#v", err, msgs)
	}
}

func TestNewMembershipChecksUnicodeSetBeforeDeclaration(t *testing.T) {
	for _, in := range []string{"\\in", "∈"} {
		tree := parsedModule(t, "---- MODULE M ----\nVARIABLE x\nTHEOREM ASSUME NEW x "+in+" Missing PROVE TRUE\n====\n")
		g, _ := newGenerator(stdcontext.Background(), 0)
		_, err := g.module(tree)
		msgs := g.table.errors.Messages()
		if err != nil || len(msgs) != 2 || msgs[0].Code.Value != 4200 || msgs[0].Parameters[0].Text != "Missing" || msgs[1].Code.Value != 4201 {
			t.Fatalf("%s: error %v messages %#v", in, err, msgs)
		}
	}
}

func TestSequentialQuantifierSelectionsRetainFormalIdentity(t *testing.T) {
	source := "---- MODULE M ----\nF(a,b,c) == <<\\E q \\in {1}: q, \\E r \\in {1}: r>>\nTHEOREM TRUE BY F(1,2,3)!1!(1), F(1,2,3)!2!(1)\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	root := m.context.lookup(contextKey{name: g.pool.intern("F")}).(*userOpDefNode)
	tuple := root.body.(*opApplNode)
	firstQuant := tuple.operands[0].(*opApplNode)
	secondQuant := tuple.operands[1].(*opApplNode)
	facts := m.theorems[0].proof.(*leafProofNode).facts
	first := facts[0].(*opApplNode).operator.(*userOpDefNode)
	second := facts[1].(*opApplNode).operator.(*userOpDefNode)
	if len(first.params) != 4 || len(second.params) != 4 || first.params[3] != firstQuant.bounded[0][0] || second.params[3] != secondQuant.bounded[0][0] || first.params[3] == second.params[3] || first.body != firstQuant.operands[0] || second.body != secondQuant.operands[0] {
		t.Fatalf("selector parameters lost binding identity: first=%v second=%v", first.params, second.params)
	}
	for i, p := range root.params {
		if first.params[i] != p || second.params[i] != p {
			t.Fatalf("root formal %d identity lost", i)
		}
	}
}
