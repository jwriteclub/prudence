// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"errors"
	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/modulegraph"
	"os"
	"testing"
)

func TestProofTraversalRelations(t *testing.T) {
	p := newNamePool()
	first := &leafProofNode{}
	second := &leafProofNode{}
	binding := &formalParamNode{}
	c := newContext(p)
	c.add(contextKey{name: p.intern("x")}, binding)
	c.add(contextKey{name: p.intern("Inner"), module: true}, &moduleNode{})
	n := &nonLeafProofNode{steps: []levelNode{first, second}, insts: []*instanceNode{{}}, context: c}
	if got := n.children(); len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("children: %v", got)
	}
	if got := n.walkChildren(); len(got) != 3 || got[0] != first || got[1] != second || got[2] != binding {
		t.Fatalf("walk: %v", got)
	}
	leaf := &leafProofNode{facts: []levelNode{first}, defs: []leafNode{binding}}
	if got := leaf.walkChildren(); len(got) != 1 || got[0] != first {
		t.Fatalf("leaf walk: %v", got)
	}
	operand := &numeralNode{}
	instance := &instanceNode{substs: []*subst{{expr: operand}}}
	if got := instance.children(); len(got) != 1 || got[0] != operand {
		t.Fatalf("instance children: %v", got)
	}
	if got := instance.walkChildren(); len(got) != 0 {
		t.Fatalf("instance walk: %v", got)
	}
	use := &useOrHideNode{facts: []levelNode{second}, defs: []leafNode{binding}}
	if got := use.children(); len(got) != 1 || got[0] != second {
		t.Fatalf("use children: %v", got)
	}
}

func TestLeafProofCommonGeneration(t *testing.T) {
	g, err := newGenerator(stdcontext.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	module, err := g.module(parsedModule(t, "---- MODULE M ----\nF == TRUE\nTHEOREM T == ASSUME NEW x PROVE x = x\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE BY ONLY MODULE M, MODULE M, T, F DEF F, T, F\n====\n")
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_TerminalProof {
			return n
		}
		for _, c := range n.Heirs() {
			if v := find(c); v != nil {
				return v
			}
		}
		return nil
	}
	tree := find(syntax)
	if tree == nil {
		t.Fatal("missing terminal proof")
	}
	proof, err := g.leafProof(tree, module, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !proof.only || len(proof.facts) != 4 || proof.facts[0] != module || proof.facts[1] != module || len(proof.defs) != 3 || proof.defs[0] != proof.defs[2] || proof.defs[1] != module.theorems[0].definition {
		t.Fatalf("BY data: %#v", proof)
	}
	if fact, ok := proof.facts[2].(*opApplNode); !ok || fact.operator != module.theorems[0].definition {
		t.Fatal("theorem fact must refer to definition")
	}
	if g.pool.java.exact {
		t.Fatal("BY allocation must invalidate bounded UID profile")
	}
}

func TestStructuredProofPublicationAndScope(t *testing.T) {
	g, err := newGenerator(stdcontext.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	module, err := g.module(parsedModule(t, "---- MODULE M ----\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. TRUE\n<1>2. <1>1 BY <1>1\n<1> QED OBVIOUS\n====\n")
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_Proof {
			return n
		}
		for _, c := range n.Heirs() {
			if v := find(c); v != nil {
				return v
			}
		}
		return nil
	}
	tree := find(syntax)
	if tree == nil {
		t.Fatal("missing structured proof")
	}
	result, err := g.proof(tree, module, 1)
	if err != nil {
		t.Fatal(err)
	}
	proof := result.(*nonLeafProofNode)
	if len(proof.steps) != 3 {
		t.Fatalf("steps: %d", len(proof.steps))
	}
	first := proof.steps[0].(*theoremNode)
	second := proof.steps[1].(*theoremNode)
	if first.definition == nil || first.definition.statement != first || second.body.(*opApplNode).operator != first.definition {
		t.Fatal("step publication or backlink")
	}
	leaf := second.proof.(*leafProofNode)
	if len(leaf.facts) != 1 || leaf.facts[0].(*opApplNode).operator != first.definition {
		t.Fatal("proof fact binding")
	}
	if g.table.top.lookup(contextKey{name: first.definition.name}) != nil {
		t.Fatal("proof name leaked")
	}
	if proof.context.lookup(contextKey{name: first.definition.name}) != first.definition {
		t.Fatal("proof context missing binding")
	}
}

func TestProofNamePublishedAfterStatementBeforeSubproof(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(parsedModule(t, "---- MODULE M ----\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. <1>1 BY <1>1\n<1>2. <1>1\n<1> QED OBVIOUS\n====\n")
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_Proof {
			return n
		}
		for _, c := range n.Heirs() {
			if v := find(c); v != nil {
				return v
			}
		}
		return nil
	}
	result, err := g.proof(find(syntax), module, 1)
	if err != nil {
		t.Fatal(err)
	}
	steps := result.(*nonLeafProofNode).steps
	first := steps[0].(*theoremNode)
	later := steps[1].(*theoremNode)
	if first.body != g.nullOAN || len(g.table.errors.Messages()) != 1 {
		t.Fatal("own statement unexpectedly resolved")
	}
	if first.proof.(*leafProofNode).facts[0].(*opApplNode).operator != first.definition || later.body.(*opApplNode).operator != first.definition {
		t.Fatal("publication did not precede subproof and sibling")
	}
}

func TestProofAssumeProveContextLifetimes(t *testing.T) {
	for _, tc := range []struct {
		name, prefix string
		errorInProof bool
	}{{"ordinary", "", false}, {"suffices", "SUFFICES ", true}} {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := newGenerator(stdcontext.Background(), 0)
			module, err := g.module(parsedModule(t, "---- MODULE M ----\n====\n"))
			if err != nil {
				t.Fatal(err)
			}
			syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. "+tc.prefix+"ASSUME NEW x PROVE x = x BY x = x\n<1>2. x = x\n<1> QED OBVIOUS\n====\n")
			var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
			find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
				if n.Kind() == generated.N_Proof {
					return n
				}
				for _, c := range n.Heirs() {
					if v := find(c); v != nil {
						return v
					}
				}
				return nil
			}
			result, err := g.proof(find(syntax), module, 1)
			if err != nil {
				t.Fatal(err)
			}
			pf := result.(*nonLeafProofNode)
			first := pf.steps[0].(*theoremNode)
			ap := first.body.(*assumeProveNode)
			if ap.inProof || ap.suffices != tc.errorInProof || first.suffices != tc.errorInProof || first.definition.suffices != tc.errorInProof {
				t.Fatal("AP flags")
			}
			errors := g.table.errors.Messages()
			if len(errors) != 2 {
				t.Fatalf("expected two unresolved x occurrences, got %v", errors)
			}
			proofExpr := first.proof.(*leafProofNode).facts[0].(*opApplNode)
			laterExpr := pf.steps[1].(*theoremNode).body.(*opApplNode)
			proofUnresolved := proofExpr.operands[0] == g.nullOAN
			laterUnresolved := laterExpr.operands[0] == g.nullOAN
			if proofUnresolved != tc.errorInProof || laterUnresolved == tc.errorInProof {
				t.Fatal("AP visibility reversed")
			}
			if got := pf.context.lookup(contextKey{name: g.pool.intern("x")}); (got != nil) != tc.errorInProof {
				t.Fatal("folded context")
			}
			if g.table.top.lookup(contextKey{name: g.pool.intern("x")}) != nil {
				t.Fatal("AP leaked outside proof")
			}
		})
	}
}

func TestTakePickBindersAndVisibility(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(parsedModule(t, "---- MODULE M ----\nCONSTANT S\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. TAKE x \\in S\n<1>2. PICK y \\in S : y = x BY y = y\n<1>3. y = x\n<1> QED OBVIOUS\n====\n")
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_Proof {
			return n
		}
		for _, c := range n.Heirs() {
			if v := find(c); v != nil {
				return v
			}
		}
		return nil
	}
	result, err := g.proof(find(syntax), module, 1)
	if err != nil {
		t.Fatal(err)
	}
	pf := result.(*nonLeafProofNode)
	take := pf.steps[0].(*theoremNode).body.(*opApplNode)
	pick := pf.steps[1].(*theoremNode).body.(*opApplNode)
	if len(take.bounded) != 1 || len(pick.bounded) != 1 || len(take.operands) != 0 || len(pick.operands) != 1 {
		t.Fatal("binder shape")
	}
	x, y := take.bounded[0][0], pick.bounded[0][0]
	pred := pick.operands[0].(*opApplNode)
	if pred.operands[0].(*opApplNode).operator != y || pred.operands[1].(*opApplNode).operator != x {
		t.Fatal("predicate formal identities")
	}
	by := pf.steps[1].(*theoremNode).proof.(*leafProofNode).facts[0].(*opApplNode)
	if by.operands[0] != g.nullOAN {
		t.Fatal("PICK formal visible in subproof")
	}
	later := pf.steps[2].(*theoremNode).body.(*opApplNode)
	if later.operands[0].(*opApplNode).operator != y || later.operands[1].(*opApplNode).operator != x {
		t.Fatal("later formal identities")
	}
	if len(g.table.errors.Messages()) != 2 {
		t.Fatalf("PICK proof diagnostics: %v", g.table.errors.Messages())
	}
}

func TestProofAtSharesPriorRHSAndSufficesRetainsIt(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(parsedModule(t, "---- MODULE M ----\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. 1 = 2\n<1>2. SUFFICES TRUE\n<1>3. @ = 3\n<1>4. @ = 4\n<1> QED OBVIOUS\n====\n")
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_Proof {
			return n
		}
		for _, c := range n.Heirs() {
			if v := find(c); v != nil {
				return v
			}
		}
		return nil
	}
	result, err := g.proof(find(syntax), module, 1)
	if err != nil {
		t.Fatal(err)
	}
	steps := result.(*nonLeafProofNode).steps
	first := steps[0].(*theoremNode).body.(*opApplNode)
	third := steps[2].(*theoremNode).body.(*opApplNode)
	fourth := steps[3].(*theoremNode).body.(*opApplNode)
	if third.operands[0].(*opApplNode).operands[0] != first.operands[1] || fourth.operands[0].(*opApplNode).operands[0] != third.operands[1] {
		t.Fatal("@ did not share preceding RHS")
	}
	if third.operands[0].(*opApplNode).operator.header().name.text != "$Nop" {
		t.Fatal("missing $Nop")
	}
}

func TestDefinitionAndUseHideProofSteps(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(parsedModule(t, "---- MODULE M ----\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. DEFINE Foo == TRUE\n<1>2. USE Foo DEF <1>1\n<1>3. HIDE Foo\n<1> QED OBVIOUS\n====\n")
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_Proof {
			return n
		}
		for _, c := range n.Heirs() {
			if v := find(c); v != nil {
				return v
			}
		}
		return nil
	}
	result, err := g.proof(find(syntax), module, 1)
	if err != nil {
		t.Fatal(err)
	}
	pf := result.(*nonLeafProofNode)
	def := pf.steps[0].(*defStepNode)
	use := pf.steps[1].(*useOrHideNode)
	hide := pf.steps[2].(*useOrHideNode)
	if len(def.defs) != 1 || len(use.facts) != 1 || len(use.defs) != 1 || !hide.hide {
		t.Fatal("step contents")
	}
	alias := use.defs[0].(*userOpDefNode)
	if alias.kind != 37 || alias.stepNode != def || use.facts[0].(*opApplNode).operator != def.defs[0] {
		t.Fatal("definition alias or fact binding")
	}
	if len(alias.children()) != 1 || alias.children()[0] != nil || len(alias.walkChildren()) != 1 || alias.walkChildren()[0] != def {
		t.Fatal("alias traversal")
	}
	if len(module.definitions) != 1 || module.definitions[0] != def.defs[0] {
		t.Fatal("module definition history")
	}
	if msgs := g.table.errors.Messages(); len(msgs) != 1 || msgs[0].Code.Value != 4357 {
		t.Fatalf("HIDE diagnostics: %v", msgs)
	}
}

func TestProofLocalInstancePublication(t *testing.T) {
	ctx := stdcontext.Background()
	pool := newNamePool()
	external := make(map[*name]leafNode)
	errs := &Errors{}
	baseGen, err := newOwnedGenerator(ctx, 0, pool, external, errs)
	if err != nil {
		t.Fatal(err)
	}
	base, err := baseGen.module(parsedModule(t, "---- MODULE A ----\nF == TRUE\nTHEOREM T == TRUE\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	external[base.name] = base
	g, err := newOwnedGenerator(ctx, 0, pool, external, errs)
	if err != nil {
		t.Fatal(err)
	}
	module, err := g.module(parsedModule(t, "---- MODULE M ----\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. INSTANCE A\n<1>2. DEFINE I == INSTANCE A\n<1>3. I!F BY I!T DEF I!F\n<1> QED OBVIOUS\n====\n")
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_Proof {
			return n
		}
		for _, c := range n.Heirs() {
			if v := find(c); v != nil {
				return v
			}
		}
		return nil
	}
	result, err := g.proof(find(syntax), module, 1)
	if err != nil {
		t.Fatal(err)
	}
	proof := result.(*nonLeafProofNode)
	plain := proof.steps[0].(*instanceNode)
	def := proof.steps[1].(*defStepNode)
	if !plain.local || plain.module != base || plain.stepName == nil || len(proof.insts) != 1 || proof.insts[0].module != base || len(def.defs) != 1 || def.defs[0].header().kind != 6 {
		t.Fatal("instance step identities")
	}
	if len(module.instances) != 0 || len(module.topLevel) != 0 || len(module.definitions) != 1 || module.definitions[0] == base.definitions[0] {
		t.Fatalf("module history: instances %d top %d definitions %v", len(module.instances), len(module.topLevel), module.definitions)
	}
	clone, ok := module.definitions[0].(*userOpDefNode)
	if !ok || !clone.local || clone.source != base.definitions[0] {
		t.Fatal("proof-local definition must clone its source")
	}
	if got := proof.context.lookup(contextKey{name: pool.intern("I!F")}); got == nil {
		t.Fatal("qualified import missing from proof scope")
	}
	leaf := proof.steps[2].(*theoremNode).proof.(*leafProofNode)
	if len(leaf.facts) != 1 || len(leaf.defs) != 1 || leaf.facts[0].(*opApplNode).operator != proof.context.lookup(contextKey{name: pool.intern("I!T")}) || leaf.defs[0] != proof.context.lookup(contextKey{name: pool.intern("I!F")}) {
		t.Fatal("qualified fact and DEF identities")
	}
	if got := g.table.top.lookup(contextKey{name: pool.intern("I!F")}); got != nil {
		t.Fatal("qualified import leaked")
	}
}

func TestProofFactSelectorsReturnNewAndAssumeProve(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(parsedModule(t, "---- MODULE M ----\nTHEOREM T == ASSUME NEW x PROVE TRUE\nTHEOREM U == ASSUME ASSUME TRUE PROVE TRUE PROVE TRUE\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE BY T!1, U!1\n====\n")
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_TerminalProof {
			return n
		}
		for _, c := range n.Heirs() {
			if v := find(c); v != nil {
				return v
			}
		}
		return nil
	}
	proof, err := g.leafProof(find(syntax), module, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(proof.facts) != 2 {
		t.Fatal("missing facts")
	}
	if proof.facts[0] != module.theorems[0].body.(*assumeProveNode).assumptions[0] {
		t.Fatalf("NEW fact %T", proof.facts[0])
	}
	if proof.facts[1] != module.theorems[1].body.(*assumeProveNode).assumptions[0] {
		t.Fatalf("AP fact %T", proof.facts[1])
	}
}

func TestStructuredProofFixtureThroughModule(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/structured-proofs/Scopes.tla")
	if err != nil {
		t.Fatal(err)
	}
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(parsedModule(t, string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.table.errors.Messages()) != 0 || len(module.topLevel) != 3 {
		t.Fatalf("module generation: %v", g.table.errors.Messages())
	}
	if _, ok := module.topLevel[1].(*useOrHideNode); !ok {
		t.Fatal("module USE order")
	}
	proof, ok := module.theorems[1].proof.(*nonLeafProofNode)
	if !ok || len(proof.steps) != 4 {
		t.Fatal("structured proof")
	}
}

func TestProofStepFactVersusExpression(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. HAVE TRUE\n<1>2. <1>1\n<1>3. TRUE BY <1>1\n<1> QED OBVIOUS\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	pf := module.theorems[0].proof.(*nonLeafProofNode)
	first := pf.steps[0].(*theoremNode)
	second := pf.steps[1].(*theoremNode)
	third := pf.steps[2].(*theoremNode)
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4351 || second.body != g.nullOAN {
		t.Fatalf("expression diagnostics: %v", msgs)
	}
	if third.proof.(*leafProofNode).facts[0].(*opApplNode).operator != first.definition {
		t.Fatal("HAVE should be usable as fact")
	}
}

func TestProofInstanceAbortUnwindsOwnedState(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(parsedModule(t, "---- MODULE M ----\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. INSTANCE Missing\n<1> QED OBVIOUS\n====\n")
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_Proof {
			return n
		}
		for _, c := range n.Heirs() {
			if v := find(c); v != nil {
				return v
			}
		}
		return nil
	}
	before := len(g.table.stack)
	_, err = g.proof(find(syntax), module, 1)
	var abort *Abort
	if !errors.As(err, &abort) || len(g.table.errors.Messages()) != 1 || g.table.errors.Messages()[0].Code.Value != 4003 {
		t.Fatalf("abort: %v %v", err, g.table.errors.Messages())
	}
	if len(g.table.stack) != before || len(g.labelScopes) != 0 || g.table.top.lookup(contextKey{name: g.pool.intern("<1>1")}) != nil {
		t.Fatal("owned proof state leaked")
	}
}

func TestNonDefinitionAliasDefDiagnosticsAndContinuation(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. USE TRUE\n<1>2. TRUE BY DEF <1>1\n<1>3. TRUE\n<1> QED OBVIOUS\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 3 || msgs[0].Code.Value != 4004 || msgs[1].Code.Value != 4200 || msgs[2].Code.Value != 4003 {
		t.Fatalf("diagnostic order: %v", msgs)
	}
	if len(module.theorems[0].proof.(*nonLeafProofNode).steps) != 4 {
		t.Fatal("generation failed to continue")
	}
}

func TestEmptyByDiagnosticOrder(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(parsedModule(t, "---- MODULE M ----\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE BY TRUE\n====\n")
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_TerminalProof {
			return n
		}
		for _, c := range n.Heirs() {
			if v := find(c); v != nil {
				return v
			}
		}
		return nil
	}
	parsed := find(syntax)
	children := parsed.Heirs()
	tree, err := g.pool.syntax.NewNode(parsed.Filename(), generated.N_TerminalProof, children[:1])
	if err != nil {
		t.Fatal(err)
	}
	proof, err := g.leafProof(tree, module, 1)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if proof == nil || len(msgs) != 2 || msgs[0].Format != "Empty BY, USE, or HIDE" || msgs[1].Format != "Empty BY" {
		t.Fatalf("empty BY: %v", msgs)
	}
}

func TestProofInstanceAcquisitionAndGeneration(t *testing.T) {
	root, err := os.ReadFile("../../testdata/structured-proofs/instances/Root.tla")
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.ReadFile("../../testdata/structured-proofs/instances/A.tla")
	if err != nil {
		t.Fatal(err)
	}
	acquisition, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), modulegraph.Input{Path: "Root.tla", Text: root}, func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		if n != "A" {
			t.Fatalf("unexpected dependency %s", n)
		}
		return modulegraph.Input{Path: "A.tla", Text: base}, nil
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, examined, module, roots, err := analyzeFlatExternalRoots(stdcontext.Background(), acquisition, 1000)
	if err != nil || !examined || len(details) != 0 || len(roots) != 2 {
		t.Fatalf("analysis: %v %v %v", examined, err, details)
	}
	proof := module.theorems[0].proof.(*nonLeafProofNode)
	if len(proof.steps) != 4 || len(proof.insts) != 1 || len(module.instances) != 0 {
		t.Fatal("proof instance publication")
	}
}

func TestSufficesAPSelectorScope(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. SUFFICES ASSUME NEW x, x = x PROVE TRUE BY <1>1!1!2\n<1>2. TRUE BY <1>1!1!1, <1>1!1!2\n<1> QED OBVIOUS\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	proof := module.theorems[0].proof.(*nonLeafProofNode)
	first := proof.steps[0].(*theoremNode)
	ap := first.body.(*assumeProveNode)
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4005 {
		t.Fatalf("scope diagnostic: %v", msgs)
	}
	if first.proof.(*leafProofNode).facts[0] != g.nullOAN {
		t.Fatal("SUFFICES proof saw later declaration scope")
	}
	facts := proof.steps[1].(*theoremNode).proof.(*leafProofNode).facts
	if len(facts) != 2 || facts[0] != ap.assumptions[0] {
		t.Fatal("selected NEW identity")
	}
	if _, ok := facts[1].(*opApplNode); !ok || facts[1] == g.nullOAN {
		t.Fatal("SUFFICES later clause unavailable")
	}
}

func TestImplicitNamedProofStepUsesRewrittenName(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. TRUE\n<*>Named. TRUE\n<1>2. TRUE BY <1>Named\n<1> QED OBVIOUS\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	pf := module.theorems[0].proof.(*nonLeafProofNode)
	named := pf.steps[1].(*theoremNode).definition
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4350 {
		t.Fatalf("implicit step diagnostics: %v", msgs)
	}
	if named.name.text != "<1>Named" || pf.steps[2].(*theoremNode).proof.(*leafProofNode).facts[0].(*opApplNode).operator != named {
		t.Fatal("rewritten step binding")
	}
}

func TestTakeRangesBeforeBindersAndTupleAndUnboundedPick(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT S\nTHEOREM TRUE\n<1>1. TAKE x \\in S, y \\in {x}\n<1>2. TAKE <<a, b>> \\in S\n<1>3. PICK p, q : p = q OBVIOUS\n<1>4. p = q\n<1> QED OBVIOUS\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	steps := module.theorems[0].proof.(*nonLeafProofNode).steps
	first := steps[0].(*theoremNode).body.(*opApplNode)
	tuple := steps[1].(*theoremNode).body.(*opApplNode)
	pick := steps[2].(*theoremNode).body.(*opApplNode)
	if len(g.table.errors.Messages()) != 1 || len(first.bounded) != 2 || first.ranges[1].(*opApplNode).operands[0] != g.nullOAN {
		t.Fatal("ranges must precede all binders")
	}
	if len(tuple.bounded) != 1 || !tuple.tupleBounds[0] || len(tuple.bounded[0]) != 2 {
		t.Fatal("tuple binder shape")
	}
	if len(pick.unbounded) != 2 || pick.operands[0].(*opApplNode).operands[0].(*opApplNode).operator != pick.unbounded[0] {
		t.Fatal("unbounded PICK identities")
	}
	later := steps[3].(*theoremNode).body.(*opApplNode)
	if later.operands[0].(*opApplNode).operator != pick.unbounded[0] || later.operands[1].(*opApplNode).operator != pick.unbounded[1] {
		t.Fatal("PICK publication")
	}
}

func TestProofDepthAndCancellationUnwind(t *testing.T) {
	syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. TRUE\n<1> QED OBVIOUS\n====\n")
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_Proof {
			return n
		}
		for _, c := range n.Heirs() {
			if v := find(c); v != nil {
				return v
			}
		}
		return nil
	}
	for _, cancel := range []bool{false, true} {
		ctx, stop := stdcontext.WithCancel(stdcontext.Background())
		g, _ := newGenerator(ctx, 0)
		module, err := g.module(parsedModule(t, "---- MODULE M ----\n====\n"))
		if err != nil {
			t.Fatal(err)
		}
		before := len(g.table.stack)
		if cancel {
			stop()
		} else {
			g.maxDepth = 1
		}
		_, err = g.proof(find(syntax), module, 1)
		if cancel && !errors.Is(err, stdcontext.Canceled) || !cancel && !errors.Is(err, ErrPortIncomplete) {
			t.Fatalf("resource stop: %v", err)
		}
		if len(g.table.stack) != before || len(g.labelScopes) != 0 || len(g.table.errors.Messages()) != 0 {
			t.Fatal("resource stop leaked state or diagnostic")
		}
		stop()
	}
}

func TestNonTheoremAliasFactAndSubexpressionDiagnostics(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. USE TRUE\n<1>2. <1>1!1\n<1>3. TRUE BY <1>1, <1>1!1\n<1> QED OBVIOUS\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 3 || msgs[0].Code.Value != 4005 || msgs[1].Code.Value != 4004 || msgs[2].Code.Value != 4005 {
		t.Fatalf("alias diagnostics: %v", msgs)
	}
	pf := module.theorems[0].proof.(*nonLeafProofNode)
	if pf.steps[1].(*theoremNode).body != g.nullOAN || len(pf.steps[2].(*theoremNode).proof.(*leafProofNode).facts) != 2 {
		t.Fatal("alias error recovery")
	}
}

func TestExpressionLabelFactSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == L:: <<1, 2>>\nTHEOREM TRUE BY F!L\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	proof := module.theorems[0].proof.(*leafProofNode)
	if len(g.table.errors.Messages()) != 0 || len(proof.facts) != 1 || proof.facts[0] == g.nullOAN {
		t.Fatalf("label fact: %v", g.table.errors.Messages())
	}
}

func TestUnknownFactAndDefContinueWithOrderedDiagnostics(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE BY Missing DEF Missing\nF == TRUE\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 3 || msgs[0].Code.Value != 4200 || msgs[1].Code.Value != 4200 || msgs[2].Code.Value != 4200 {
		t.Fatalf("diagnostics: %v", msgs)
	}
	proof := module.theorems[0].proof.(*leafProofNode)
	if len(proof.facts) != 1 || proof.facts[0] != g.nullOAN || len(proof.defs) != 0 || len(module.definitions) != 1 {
		t.Fatal("sentinel or continuation")
	}
}

func TestModuleHideUnknownModuleDiagnostics(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nHIDE MODULE Missing\nF == TRUE\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4005 || msgs[1].Code.Value != 4003 {
		t.Fatalf("diagnostic order: %v", msgs)
	}
	if len(module.topLevel) != 1 || len(module.definitions) != 1 {
		t.Fatal("module HIDE continuation")
	}
}

func TestProofSelectorDiagnosticDedupUsesNameIdentity(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(parsedModule(t, "---- MODULE M ----\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE BY Missing DEF Missing\n====\n")
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_TerminalProof {
			return n
		}
		for _, c := range n.Heirs() {
			if v := find(c); v != nil {
				return v
			}
		}
		return nil
	}
	parsed := find(syntax)
	parts := parsed.Heirs()
	if len(parts) != 4 {
		t.Fatal("unexpected BY shape")
	}
	duplicate, err := g.pool.syntax.NewNode(parsed.Filename(), generated.N_TerminalProof, []*generated.SyntaxTreeNode{parts[0], parts[1], parts[2], parts[1]})
	if err != nil {
		t.Fatal(err)
	}
	_, err = g.leafProof(duplicate, module, 1)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Parameters[0].Kind != ObjectIdentityParameter || msgs[1].Format != "DEF clause entry should describe a defined operator." {
		t.Fatalf("dedup: %v", msgs)
	}
}

func TestDuplicateProofStepKeepsFirstBinding(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. TRUE\n<1>1. FALSE\n<1>2. <1>1\n<1> QED OBVIOUS\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	pf := module.theorems[0].proof.(*nonLeafProofNode)
	first := pf.steps[0].(*theoremNode).definition
	duplicate := pf.steps[1].(*theoremNode).definition
	if first == duplicate || pf.steps[2].(*theoremNode).body.(*opApplNode).operator != first {
		t.Fatal("duplicate replaced first binding")
	}
	if msgs := g.table.errors.Messages(); len(msgs) != 1 || msgs[0].Code.Value != 4801 {
		t.Fatalf("conflict: %v", msgs)
	}
}

func TestPickOuterConflictKeepsOuterBinding(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT x\nTHEOREM TRUE\n<1>1. PICK x : x = x BY x = x\n<1>2. x = x\n<1> QED OBVIOUS\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	pf := module.theorems[0].proof.(*nonLeafProofNode)
	pick := pf.steps[0].(*theoremNode).body.(*opApplNode)
	outer := module.context.lookup(contextKey{name: g.pool.intern("x")})
	formal := pick.unbounded[0]
	if formal == nil || outer == formal || pick.operands[0].(*opApplNode).operands[0].(*opApplNode).operator != outer {
		t.Fatal("PICK conflict binding")
	}
	later := pf.steps[1].(*theoremNode).body.(*opApplNode)
	if later.operands[0].(*opApplNode).operator != outer {
		t.Fatal("PICK replaced outer binding")
	}
	if msgs := g.table.errors.Messages(); len(msgs) != 1 || msgs[0].Code.Value != 4201 {
		t.Fatalf("conflict: %v", msgs)
	}
}

func TestProofLabelFrameAndPickFormalIdentity(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == TRUE\n<1> HAVE Outside:: TRUE\n<1>1. PICK y : L(y):: y = y OBVIOUS\n<1> QED OBVIOUS\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4333 {
		t.Fatalf("unnamed label diagnostics: %v", msgs)
	}
	pf := module.theorems[0].proof.(*nonLeafProofNode)
	pickStep := pf.steps[1].(*theoremNode)
	pick := pickStep.body.(*opApplNode)
	label := pickStep.definition.labels.get(g.pool.intern("L"))
	if label == nil || len(label.params) != 1 || label.params[0] != pick.unbounded[0] {
		t.Fatal("PICK label formal identity")
	}
}

func TestAssumeProveFixedDepthGuard(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(parsedModule(t, "---- MODULE M ----\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM ASSUME TRUE PROVE TRUE\n====\n")
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_AssumeProve {
			return n
		}
		for _, c := range n.Heirs() {
			if v := find(c); v != nil {
				return v
			}
		}
		return nil
	}
	ap := find(syntax)
	parts := ap.Heirs()
	nested := ap
	for i := 1; i < 100; i++ {
		nested, err = g.pool.syntax.NewNode(ap.Filename(), generated.N_AssumeProve, []*generated.SyntaxTreeNode{parts[0], nested, parts[2], parts[3]})
		if err != nil {
			t.Fatal(err)
		}
	}
	before := len(g.table.stack)
	_, err = g.assumeProve(nested, module, nil, 1)
	if !errors.Is(err, ErrPortIncomplete) || g.apDepth != 0 || len(g.apDeclScopes) != 0 || len(g.table.stack) != before {
		t.Fatalf("AP depth cleanup: %v", err)
	}
}

func TestUseAssumeProveFactKeepsDirectContextAndInProof(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(parsedModule(t, "---- MODULE M ----\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM ASSUME NEW x PROVE TRUE\nUSE TRUE\n====\n")
	var ap, use *generated.SyntaxTreeNode
	var walk func(*generated.SyntaxTreeNode)
	walk = func(n *generated.SyntaxTreeNode) {
		if n.Kind() == generated.N_AssumeProve {
			ap = n
		}
		if n.Kind() == generated.N_UseOrHide {
			use = n
		}
		for _, c := range n.Heirs() {
			walk(c)
		}
	}
	walk(syntax)
	if ap == nil || use == nil {
		t.Fatal("missing syntax")
	}
	tree, err := g.pool.syntax.NewNode(use.Filename(), generated.N_UseOrHide, []*generated.SyntaxTreeNode{use.Heirs()[0], ap})
	if err != nil {
		t.Fatal(err)
	}
	record, err := g.useOrHide(tree, module, 1)
	if err != nil {
		t.Fatal(err)
	}
	fact := record.facts[0].(*assumeProveNode)
	if !fact.inProof || g.table.top.lookup(contextKey{name: g.pool.intern("x")}) != fact.assumptions[0].(*newSymbNode).declaration {
		t.Fatal("direct AP fact context")
	}
}

func TestProofDefResolvesOperatorSynonym(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\na (+) b == a\nTHEOREM TRUE BY DEF \\oplus\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	proof := module.theorems[0].proof.(*leafProofNode)
	if len(proof.defs) != 1 || proof.defs[0] != module.definitions[0] {
		t.Fatal("operator synonym DEF")
	}
}

func TestNullTheoremBodyReferenceStaysIncomplete(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. 1 \\oplus 2\n<1>2. <1>1\n<1> QED OBVIOUS\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, err := g.module(tree)
	if !errors.Is(err, ErrPortIncomplete) || len(g.table.errors.Messages()) == 0 {
		t.Fatalf("null-body recovery: %v %v", err, g.table.errors.Messages())
	}
}

func TestQualifiedDefDiagnosticsContinue(t *testing.T) {
	ctx := stdcontext.Background()
	pool := newNamePool()
	external := map[*name]leafNode{}
	errs := &Errors{}
	baseGen, _ := newOwnedGenerator(ctx, 0, pool, external, errs)
	base, err := baseGen.module(parsedModule(t, "---- MODULE A ----\nF == TRUE\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	external[base.name] = base
	g, _ := newOwnedGenerator(ctx, 0, pool, external, errs)
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. DEFINE I == INSTANCE A\n<1>2. TRUE BY DEF I!Missing, I(1)!F\n<1> QED OBVIOUS\n====\n")
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := errs.Messages()
	if len(msgs) != 5 || msgs[0].Code.Value != 4200 || msgs[1].Code.Value != 4200 || msgs[2].Code.Value != 4005 || msgs[3].Code.Value != 4200 || msgs[4].Format != "Empty BY" {
		t.Fatalf("qualified DEF diagnostics: %v", msgs)
	}
	if len(module.theorems[0].proof.(*nonLeafProofNode).steps) != 3 {
		t.Fatal("diagnostic did not continue")
	}
}

func TestQualifiedDefSubexpressionDiagnostics(t *testing.T) {
	ctx := stdcontext.Background()
	pool := newNamePool()
	external := map[*name]leafNode{}
	errs := &Errors{}
	baseGen, _ := newOwnedGenerator(ctx, 0, pool, external, errs)
	base, err := baseGen.module(parsedModule(t, "---- MODULE A ----\nF == <<1, 2>>\nTHEOREM T == ASSUME NEW x PROVE TRUE\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	external[base.name] = base
	g, _ := newOwnedGenerator(ctx, 0, pool, external, errs)
	_, err = g.module(parsedModule(t, "---- MODULE M ----\nI == INSTANCE A\nTHEOREM TRUE BY DEF I!F!1, I!T!1\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	msgs := errs.Messages()
	if len(msgs) != 4 || msgs[0].Code.Value != 4200 || msgs[1].Format != "Selected a NEW declaration as an expression or operator." || msgs[2].Code.Value != 4200 || msgs[3].Format != "Empty BY" {
		t.Fatalf("qualified subexpression DEF diagnostics: %v", msgs)
	}
}

func TestParameterizedProofInstanceSharingAndSelectors(t *testing.T) {
	ctx := stdcontext.Background()
	pool := newNamePool()
	external := map[*name]leafNode{}
	errs := &Errors{}
	baseGen, _ := newOwnedGenerator(ctx, 0, pool, external, errs)
	base, err := baseGen.module(parsedModule(t, "---- MODULE A ----\nCONSTANT a\nF == a\nTHEOREM T == a = a\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	external[base.name] = base
	g, _ := newOwnedGenerator(ctx, 0, pool, external, errs)
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. DEFINE I(y) == INSTANCE A WITH a <- y\n<1>2. I(1)!F BY I(1)!T DEF I!F\n<1> QED OBVIOUS\n====\n")
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	pf := module.theorems[0].proof.(*nonLeafProofNode)
	f := pf.context.lookup(contextKey{name: pool.intern("I!F")}).(*userOpDefNode)
	thm := pf.context.lookup(contextKey{name: pool.intern("I!T")}).(*thmOrAssumpDefNode)
	fbody := f.body.(*substInNode)
	tbody := thm.body.(*apSubstInNode)
	if len(fbody.substs) != 1 || len(tbody.substs) != 1 || &fbody.substs[0] != &tbody.substs[0] || fbody.substs[0] != pf.insts[0].substs[0] {
		t.Fatal("substitution array sharing")
	}
	leaf := pf.steps[1].(*theoremNode).proof.(*leafProofNode)
	if len(leaf.facts) != 1 || leaf.facts[0].(*opApplNode).operator != thm || len(leaf.defs) != 1 || leaf.defs[0] != f || len(errs.Messages()) != 0 {
		t.Fatalf("instance selector binding: %v", errs.Messages())
	}
}

func TestProofAllocationInvalidatesLabelHistory(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. TAKE x, y\n<1>2. L(x, y):: TRUE\n<1> QED OBVIOUS\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, err := g.module(tree)
	if err != nil || g.pool.java.exact || g.labelOrderFallbacks != 1 {
		t.Fatalf("unproved UID order: %v, fallbacks %d", err, g.labelOrderFallbacks)
	}
	// The cursor gives no Java order here; the list prints in the stable
	// fallback order (label_extra_order.go) and claims no UID history.
	extras := 0
	for _, d := range g.table.errors.Messages() {
		if d.Code.Value == 4332 && d.Parameters[0].Text == "Label L declares extra parameter(s)  x  y  " {
			extras++
		}
	}
	if extras != 1 {
		t.Fatalf("label extras: %+v", g.table.errors.Messages())
	}
}

func TestInstantiatedTheoremFactNewKeepsSourceIdentity(t *testing.T) {
	ctx := stdcontext.Background()
	pool := newNamePool()
	external := map[*name]leafNode{}
	errs := &Errors{}
	baseGen, _ := newOwnedGenerator(ctx, 0, pool, external, errs)
	base, err := baseGen.module(parsedModule(t, "---- MODULE A ----\nCONSTANT a\nTHEOREM T == ASSUME NEW x PROVE x = a\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	external[base.name] = base
	g, _ := newOwnedGenerator(ctx, 0, pool, external, errs)
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. DEFINE I(y) == INSTANCE A WITH a <- y\n<1>2. TRUE BY I(1)!T!1\n<1> QED OBVIOUS\n====\n")
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	pf := module.theorems[0].proof.(*nonLeafProofNode)
	fact := pf.steps[1].(*theoremNode).proof.(*leafProofNode).facts[0]
	if fact != base.theorems[0].body.(*assumeProveNode).assumptions[0] || len(errs.Messages()) != 0 {
		t.Fatal("selected NEW should return before substitution wrapper reconstruction")
	}
}

func TestSyntheticProofOperatorsAndLocations(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. HAVE TRUE\n<1>2. CASE TRUE OBVIOUS\n<1>3. WITNESS 1, 2\n<1> QED OBVIOUS\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	pf := module.theorems[0].proof.(*nonLeafProofNode)
	syntaxSteps := pf.tree.Heirs()
	names := []string{"$Have", "$Pfcase", "$Witness", "$Qed"}
	counts := []int{1, 1, 2, 0}
	for i, node := range pf.steps {
		step := node.(*theoremNode)
		app := step.body.(*opApplNode)
		if step.tree != syntaxSteps[i] || app.tree != syntaxSteps[i].Heirs()[1] || app.operator.header().name.text != names[i] || len(app.operands) != counts[i] {
			t.Fatalf("synthetic step %d", i)
		}
	}
}

func TestSufficesAPInvalidLeadingSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. SUFFICES ASSUME TRUE PROVE TRUE\n<1>2. TRUE BY <1>1!@\n<1> QED OBVIOUS\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Format != "Accessing non-existent subexpression of a SUFFICES" {
		t.Fatalf("selector diagnostic: %v", msgs)
	}
	if module.theorems[0].proof.(*nonLeafProofNode).steps[1].(*theoremNode).proof.(*leafProofNode).facts[0] != g.nullOAN {
		t.Fatal("selector recovery")
	}
}

func TestDefApplicationRecoveryAndSelectorAbort(t *testing.T) {
	for _, tc := range []struct {
		name, entry string
		abort       bool
	}{{"plain", "F(1)", false}, {"qualified", "I!F(1)", true}} {
		t.Run(tc.name, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\nF(x) == x\nTHEOREM TRUE BY DEF "+tc.entry+"\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			_, err := g.module(tree)
			msgs := g.table.errors.Messages()
			if tc.abort {
				var aborted *Abort
				if !errors.As(err, &aborted) || len(msgs) != 1 || msgs[0].Format != "Internal error: IdPrefixElement has other than 2 or 3 heirs." {
					t.Fatalf("abort: %v %v", err, msgs)
				}
				return
			}
			if err != nil || len(msgs) != 4 || msgs[0].Format != "Was expecting a GeneralId." || msgs[1].Code.Value != 4005 || msgs[2].Code.Value != 4200 || msgs[3].Format != "Empty BY" {
				t.Fatalf("recovery: %v %v", err, msgs)
			}
		})
	}
}

func TestDefTheoremAndAliasSubexpressionDiagnostics(t *testing.T) {
	for _, tc := range []struct{ name, source, first string }{
		{"theorem NEW", "THEOREM T == ASSUME NEW x PROVE TRUE\nTHEOREM TRUE BY DEF T!1", "Selected a NEW declaration as an expression or operator."},
		{"declaration", "CONSTANT x\nTHEOREM TRUE BY DEF x!1", "Cannot take subexpression of `%s'."},
		{"arguments", "F(x) == x\nTHEOREM TRUE BY DEF F(1)!1", "Selector `%s' should not have argument(s)."},
		{"theorem arguments", "THEOREM T == TRUE\nTHEOREM TRUE BY DEF T!(1)", "Selector `%s' should not have argument(s)."},
		{"step alias", "THEOREM TRUE\n<1>1. USE TRUE\n<1>2. TRUE BY DEF <1>1!1\n<1> QED OBVIOUS", "Cannot take subexpression of `%s'."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\n"+tc.source+"\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			_, err := g.module(tree)
			if err != nil {
				t.Fatal(err)
			}
			msgs := g.table.errors.Messages()
			if len(msgs) != 3 || msgs[0].Format != tc.first || msgs[1].Code.Value != 4200 || msgs[2].Format != "Empty BY" {
				t.Fatalf("DEF selection: %v", msgs)
			}
		})
	}
}

func TestDefOperatorLabelSelectionDiagnostics(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == L:: TRUE\nTHEOREM TRUE BY DEF F!L, F!Missing\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 4 || msgs[0].Code.Value != 4200 || msgs[1].Format != "Cannot find label `%s'." || msgs[2].Code.Value != 4200 || msgs[3].Format != "Empty BY" {
		t.Fatalf("label DEF diagnostics: %v", msgs)
	}
}

func TestDefSelectsOperatorLocalDefinition(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nG == LET F == TRUE IN F\nTHEOREM TRUE BY DEF G!:!F\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	if msgs := g.table.errors.Messages(); len(msgs) != 0 {
		t.Fatalf("local DEF diagnostics: %v", msgs)
	}
	var root *userOpDefNode
	for _, def := range module.definitions {
		if op, ok := def.(*userOpDefNode); ok && op.name == g.pool.intern("G") {
			root = op
		}
	}
	if root == nil {
		t.Fatal("missing G")
	}
	local := root.body.(*letInNode).context.lookup(contextKey{name: g.pool.intern("F")})
	proof := module.theorems[0].proof.(*leafProofNode)
	if len(proof.defs) != 1 || proof.defs[0] != local {
		t.Fatal("DEF did not retain selected local symbol")
	}
}

func TestDefSelectsLocalDefinitionThroughLabel(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nG == L:: LET F == TRUE IN F\nTHEOREM TRUE BY DEF G!L!:!F\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	if msgs := g.table.errors.Messages(); len(msgs) != 0 {
		t.Fatalf("label DEF diagnostics: %v", msgs)
	}
	var root *userOpDefNode
	for _, def := range module.definitions {
		if op, ok := def.(*userOpDefNode); ok && op.name == g.pool.intern("G") {
			root = op
		}
	}
	if root == nil {
		t.Fatal("missing G")
	}
	local := root.labels.get(g.pool.intern("L")).body.(*letInNode).context.lookup(contextKey{name: g.pool.intern("F")})
	proof := module.theorems[0].proof.(*leafProofNode)
	if len(proof.defs) != 1 || proof.defs[0] != local {
		t.Fatal("DEF did not retain labeled local symbol")
	}
}

func TestDefSelectsTheoremLocalOperator(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == LET F == TRUE IN F\nTHEOREM TRUE BY DEF T!:!F\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	if msgs := g.table.errors.Messages(); len(msgs) != 0 {
		t.Fatalf("local DEF diagnostics: %v", msgs)
	}
	local := module.theorems[0].body.(*letInNode).context.lookup(contextKey{name: g.pool.intern("F")})
	proof := module.theorems[1].proof.(*leafProofNode)
	if len(proof.defs) != 1 || proof.defs[0] != local {
		t.Fatal("DEF did not retain selected local symbol")
	}
}

func TestDefTrailingColonReportsNonDefinition(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == TRUE\nTHEOREM TRUE BY DEF F!:\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4200 || msgs[1].Format != "Empty BY" {
		t.Fatalf("colon DEF diagnostics: %v", msgs)
	}
}

func TestDefStructuralSelectionDiagnostics(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == <<<<1>>>>\nTHEOREM TRUE BY DEF F!1!1, F!3\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 4 || msgs[0].Code.Value != 4200 || msgs[1].Code.Value != 4005 || msgs[2].Code.Value != 4200 || msgs[3].Format != "Empty BY" {
		t.Fatalf("structural DEF diagnostics: %v", msgs)
	}
}

func TestIllegalLabelInvariantAborts(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == TRUE\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	label := &labelNode{goal: module.theorems[0].definition}
	_, err = g.illegalProofLabelRef(label, tree)
	var abort *Abort
	if !errors.As(err, &abort) || len(g.table.errors.Messages()) != 1 || g.table.errors.Messages()[0].Format != "Internal error: Expecting label to be in AssumeProveNode, but it's not." {
		t.Fatalf("label invariant: %v %v", err, g.table.errors.Messages())
	}
}

func TestOperatorAPLabelFactVersusExpression(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(parsedModule(t, "---- MODULE M ----\nF == L:: TRUE\nTHEOREM T == ASSUME TRUE PROVE TRUE\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	root := module.definitions[0].(*userOpDefNode)
	label := root.labels.get(g.pool.intern("L"))
	label.body = module.theorems[0].body
	label.isAssumeProve = true
	syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE BY F!L\n====\n")
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_TerminalProof {
			return n
		}
		for _, c := range n.Heirs() {
			if v := find(c); v != nil {
				return v
			}
		}
		return nil
	}
	selector := find(syntax).Heirs()[1]
	fact, err := g.proofFactSelector(selector, 1)
	if err != nil {
		t.Fatal(err)
	}
	if app, ok := fact.(*opApplNode); !ok || app.operator.header().name.text != "$Nop" || app.operands[0] != label {
		t.Fatal("AP label fact")
	}
	structuralSyntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE BY F!L!1\n====\n")
	structural := find(structuralSyntax).Heirs()[1]
	selected, err := g.proofFactSelector(structural, 1)
	if err != nil {
		t.Fatal(err)
	}
	ap := label.body.(*assumeProveNode)
	app, ok := selected.(*opApplNode)
	if !ok || app.operator.header().name.text != "$Nop" || app.operands[0] != ap.assumptions[0] {
		t.Fatal("structural selection through AP label")
	}
	expr, err := g.expression(selector, 1)
	if err != nil || expr != g.nullOAN {
		t.Fatalf("AP label expression: %v", err)
	}
	if msgs := g.table.errors.Messages(); len(msgs) != 1 || msgs[0].Code.Value != 4355 {
		t.Fatalf("AP expression diagnostic: %v", msgs)
	}
}

func TestQualifiedProofFactMatchesHigherOrderArguments(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name + ".tla", Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	root := input("Root", "I(Op(_)) == INSTANCE A WITH c <- Op\nTHEOREM TRUE BY I(Missing)!F\nAfter == TRUE")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, _ string) (modulegraph.Input, error) {
		return input("A", "CONSTANT c(_)\nF == TRUE"), nil
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, _, module, _, err := analyzeFlatExternalRoots(stdcontext.Background(), a, 1000)
	if err != nil || len(details) != 2 || details[0].Code.Value != 4200 || details[1].Code.Value != 4271 || details[1].Parameters[1].Text != "I!F" || !module.owner.history.exact || !module.owner.history.reuse(module.context.lookup(contextKey{name: module.owner.intern("After")})).known {
		t.Fatalf("error=%v diagnostics=%v", err, details)
	}
}
