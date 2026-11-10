// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"testing"
)

func TestLevelProofSubnodesAndCache(t *testing.T) {
	p := &formalParamNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 11}}}
	marker := &userOpDefNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 6}}}
	// A module and instance marker are explicitly ignored, as are DEF entries.
	by := &leafProofNode{facts: []levelNode{&moduleNode{}, marker, p}, defs: []leafNode{&userOpDefNode{}}}
	use := &useOrHideNode{facts: []levelNode{by}, defs: []leafNode{&userOpDefNode{}}}
	step := &defStepNode{defs: []definitionNode{marker}}
	proof := &nonLeafProofNode{steps: []levelNode{use, step}}
	c := testLevelChecker(t, p)
	if ok, err := c.check(proof, 1); err != nil || !ok {
		t.Fatalf("proof: %v %v", ok, err)
	}
	state, err := c.data(proof)
	if err != nil {
		t.Fatal(err)
	}
	sym, err := c.sym(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := state.params.Get(sym, sym.hash); err != nil || !found {
		t.Fatalf("fact parameter: %v %v", found, err)
	}
	if c.states[marker] != nil {
		t.Fatal("module marker was checked")
	}
	if ok, err := c.check(proof, 1); err != nil || !ok || state.checked != 1 {
		t.Fatalf("cached proof: %v %v", ok, err)
	}
}

func TestLevelTemporalProofTraversal(t *testing.T) {
	pool := newNamePool()
	operator := func(s string) *builtinOpDefNode {
		return &builtinOpDefNode{nodeHeader: nodeHeader{name: pool.intern(s)}}
	}
	app := func(s string) *opApplNode { return &opApplNode{operator: operator(s)} }
	have, cas, witness, ignored := app("$Have"), app("$Pfcase"), app("$Witness"), app("$Have")
	nested := &nonLeafProofNode{steps: []levelNode{&theoremNode{body: witness}}}
	proof := &nonLeafProofNode{steps: []levelNode{
		&theoremNode{body: have, proof: &nonLeafProofNode{steps: []levelNode{&theoremNode{body: ignored}}}},
		&theoremNode{body: cas, proof: nested},
	}}
	c := testLevelChecker(t)
	for _, n := range []*opApplNode{have, cas, witness, ignored} {
		s, err := c.state(n)
		if err != nil {
			t.Fatal(err)
		}
		s.level = 1
		s.checked = 1
	}
	if err := c.temporalProof(proof); err != nil {
		t.Fatal(err)
	}
	msgs := c.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4352 || msgs[1].Code.Value != 4353 {
		t.Fatalf("diagnostics and dedup: %#v", msgs)
	}
}

func TestLevelProofGeneratedCombinedModel(t *testing.T) {
	g, err := newGenerator(stdcontext.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m, err := g.module(parsedModule(t, "---- MODULE M ----\nCONSTANT C\nF == C\nTHEOREM T == ASSUME NEW x PROVE x = x\n<1>1. USE F DEF F\n<1>2. TRUE BY F\n<1> QED OBVIOUS\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	ids := make(map[leafNode]uint64)
	c, err := newLevelChecker(stdcontext.Background(), func(n leafNode) (uint64, uint32, error) {
		id := ids[n]
		if id == 0 {
			id = uint64(len(ids) + 1)
			ids[n] = id
		}
		return id, uint32(id), nil
	}, 1000, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.theorems) != 1 {
		t.Fatalf("theorems: %d", len(m.theorems))
	}
	if ok, err := c.check(m.theorems[0], 1); err != nil || !ok {
		t.Fatalf("generated theorem: %v %v", ok, err)
	}
	p, ok := m.theorems[0].proof.(*nonLeafProofNode)
	if !ok || len(p.steps) != 3 {
		t.Fatalf("proof: %#v", m.theorems[0].proof)
	}
	for _, step := range p.steps {
		if c.states[step] == nil {
			t.Fatalf("step not checked: %T", step)
		}
	}
}

// Numbered non-theorem aliases check their step, but the early OpDef cache
// makes Java's subsequent aggregation helper return without copying fields.
func TestLevelNumberedAliasCache(t *testing.T) {
	p := &formalParamNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 11}}}
	step := &useOrHideNode{facts: []levelNode{p}}
	alias := &userOpDefNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 37}}, stepNode: step}
	c := testLevelChecker(t, p)
	if ok, err := c.check(alias, 1); err != nil || !ok {
		t.Fatalf("alias: %v %v", ok, err)
	}
	a, _ := c.data(alias)
	b, _ := c.data(step)
	sym, _ := c.sym(p)
	if _, present, _ := b.params.Get(sym, sym.hash); !present {
		t.Fatal("step lost parameter")
	}
	if _, present, _ := a.params.Get(sym, sym.hash); present {
		t.Fatal("alias copied step parameters")
	}
	if ok, err := c.check(alias, 2); err != nil || !ok || a.checked != 1 {
		t.Fatalf("alias cache: %v %v", ok, err)
	}
}
