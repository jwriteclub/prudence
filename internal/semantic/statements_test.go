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

func TestTheoremDefinitionClassAndOrigin(t *testing.T) {
	p := newNamePool()
	c := newContext(p)
	m, err := newModule(p, p.intern("M"), nil, c)
	if err != nil {
		t.Fatal(err)
	}
	original, err := newThmOrAssumpDef(p, p.intern("T"), nil)
	if err != nil {
		t.Fatal(err)
	}
	original.module = m
	clone, err := newThmOrAssumpDef(p, p.intern("I!T"), nil)
	if err != nil {
		t.Fatal(err)
	}
	clone.module, clone.source, clone.local = m, original, true
	if nodeClassOf(original) != classTheorem || nodeArity(original) != 0 || !nodeLocal(clone) || !sameOriginallyDefined(original, clone) {
		t.Fatal("theorem identity or classification")
	}
	if _, ok := any(original).(definitionNode); ok {
		t.Fatal("theorem definition classified as operator definition")
	}
}

func TestExpressionStatementsAndPublication(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nASSUME A == TRUE\nAXIOM FALSE\nTHEOREM T == A\nTHEOREM TRUE OBVIOUS\nTHEOREM FALSE OMITTED\n====\n")
	g, err := newGenerator(stdcontext.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.assumptions) != 2 || len(m.theorems) != 3 || len(m.topLevel) != 5 || len(m.definitions) != 2 {
		t.Fatalf("statement counts: %#v", m)
	}
	if m.assumptions[0].definition.statement != m.assumptions[0] || !m.assumptions[1].axiom || m.theorems[0].definition.statement != m.theorems[0] {
		t.Fatal("statement backlinks or axiom flag")
	}
	if m.theorems[0].proof != nil || !m.theorems[1].proof.(*leafProofNode).obvious || !m.theorems[2].proof.(*leafProofNode).omitted {
		t.Fatal("leaf proof forms")
	}
	app := m.theorems[0].body.(*opApplNode)
	if app.operator != m.assumptions[0].definition {
		t.Fatal("named assumption reference")
	}
}

func TestAssumeProveDeclarationScope(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT S\nTHEOREM T == ASSUME NEW x \\in S, x = x PROVE x = x\nF == S\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	ap, ok := m.theorems[0].body.(*assumeProveNode)
	if !ok {
		t.Fatalf("body %T", m.theorems[0].body)
	}
	if ap.goal != m.theorems[0].definition || ap.inProof || len(ap.assumptions) != 2 || !ap.declarations[1] || !ap.declarations[2] {
		t.Fatal("AP metadata")
	}
	d := ap.assumptions[0].(*newSymbNode)
	if d.declaration.name.text != "x" || d.set == nil || d.declaration.kind != 24 {
		t.Fatal("NEW declaration")
	}
	if x, _ := g.table.resolve(g.pool.intern("x"), false); x != nil {
		t.Fatal("NEW leaked")
	}
	if len(g.table.errors.Messages()) != 0 {
		t.Fatal(g.table.errors.Messages())
	}
}

func TestAssumeProveExpressionRejectedAndByGenerated(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == ASSUME NEW x PROVE x = x\nF == T\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.theorems) != 1 || len(g.table.errors.Messages()) != 1 || g.table.errors.Messages()[0].Code.Value != 4355 {
		t.Fatal("missing AP expression diagnostic")
	}
	if m.definitions[1].(*userOpDefNode).body != g.nullOAN {
		t.Fatal("missing recovery expression")
	}
	tree = parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE BY TRUE\n====\n")
	g, _ = newGenerator(stdcontext.Background(), 0)
	m, err = g.module(tree)
	if err != nil {
		t.Fatalf("BY proof: %v", err)
	}
	proof, ok := m.theorems[0].proof.(*leafProofNode)
	if !ok || len(proof.facts) != 1 {
		t.Fatal("missing BY fact")
	}
}

func TestTheoremInstanceAndExtendsIdentity(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	files := map[string]modulegraph.Input{"A": input("A", "THEOREM T == TRUE\nASSUME H == FALSE"), "B": input("B", "INSTANCE A"), "C": input("C", "I == INSTANCE A")}
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("Root", "EXTENDS B, C"), func(_ stdcontext.Context, n string) (modulegraph.Input, error) { return files[n], nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, _, root, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil || len(details) != 0 {
		t.Fatalf("%v %v", err, details)
	}
	b, c := root.extendees[0], root.extendees[1]
	original := b.definitions[0].(*thmOrAssumpDefNode)
	var qualified *thmOrAssumpDefNode
	for _, d := range c.definitions {
		if x, ok := d.(*thmOrAssumpDefNode); ok && x.source == original {
			qualified = x
		}
	}
	if qualified == nil || qualified.instantiatedFrom == nil || qualified.body != original.body || qualified.statement != nil {
		t.Fatal("qualified theorem origin")
	}
	if len(b.topLevel) != 1 || len(c.topLevel) != 1 || len(root.topLevel) != 2 {
		t.Fatal("instance top level order")
	}
}

func TestTheoremExpressionSelectorsAndScope(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == ASSUME L::TRUE, NEW x, P::TRUE PROVE TRUE\nA == T!1\nB == T!L\nC == T!P\nD == T!3\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.definitions) != 5 {
		t.Fatal("definition count")
	}
	if len(g.table.errors.Messages()) != 2 {
		t.Fatalf("diagnostics: %#v", g.table.errors.Messages())
	}
	for _, i := range []int{1, 2} {
		if m.definitions[i].(*userOpDefNode).body == g.nullOAN {
			t.Fatalf("selector %d rejected", i)
		}
	}
	for _, i := range []int{3, 4} {
		if m.definitions[i].(*userOpDefNode).body != g.nullOAN {
			t.Fatalf("selector %d escaped NEW scope", i)
		}
	}
}

func TestNestedAssumeProveDeclarationAndBoxing(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == ASSUME ASSUME NEW x, L::TRUE PROVE TRUE PROVE TRUE\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.theorems) != 1 || len(g.table.errors.Messages()) != 1 || g.table.errors.Messages()[0].Code.Value != 4334 {
		t.Fatalf("nested label: %#v", g.table.errors.Messages())
	}
	outer := m.theorems[0].body.(*assumeProveNode)
	inner := outer.assumptions[0].(*assumeProveNode)
	if inner.goal != nil || !inner.inProof || outer.inProof {
		t.Fatal("nested AP ownership")
	}
	if x, _ := g.table.resolve(g.pool.intern("x"), false); x != nil {
		t.Fatal("nested NEW escaped")
	}
}

func TestQualifiedTheoremSelectorRebuildsSubstitution(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("B", "I(y) == INSTANCE A WITH x <- y\nF == I(1)!T!1"), func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		return input("A", "CONSTANT x\nTHEOREM T == x = 1"), nil
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, _, root, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil || len(details) != 0 {
		t.Fatalf("%v %#v", err, details)
	}
	var imported *thmOrAssumpDefNode
	var f *userOpDefNode
	for _, d := range root.definitions {
		switch x := d.(type) {
		case *thmOrAssumpDefNode:
			imported = x
		case *userOpDefNode:
			if x.name.text == "F" {
				f = x
			}
		}
	}
	if imported == nil || len(imported.params) != 1 || imported.instantiatedFrom == nil || f == nil {
		t.Fatal("missing imported theorem")
	}
	app := f.body.(*opApplNode)
	lambda, ok := app.operator.(*userOpDefNode)
	if !ok {
		t.Fatalf("wrapper operator %T", app.operator)
	}
	if _, ok = lambda.body.(*substInNode); !ok {
		t.Fatalf("wrapper body %T", lambda.body)
	}
}

func TestBoxedAssumeProveDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		source  string
		message string
	}{
		{"THEOREM []ASSUME TRUE PROVE TRUE", "[]ASSUME matched by PROVE instead of []PROVE"},
		{"THEOREM ASSUME TRUE []PROVE TRUE", "ASSUME matched by []PROVE instead of PROVE"},
		{"THEOREM ASSUME []ASSUME TRUE []PROVE TRUE PROVE TRUE", "[]ASSUME used within the scope of an ordinary ASSUME's assumptions"},
	} {
		tree := parsedModule(t, "---- MODULE M ----\n"+tc.source+"\n====\n")
		g, _ := newGenerator(stdcontext.Background(), 0)
		_, err := g.module(tree)
		if err != nil {
			t.Fatal(err)
		}
		msgs := g.table.errors.Messages()
		if len(msgs) != 1 || msgs[0].Format != tc.message {
			t.Fatalf("%s: %#v", tc.source, msgs)
		}
	}
}

func TestStatementProjectionRetainsNewDeclarationAndIdentity(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT S\nTHEOREM T == ASSUME NEW x \\in S PROVE x = x OBVIOUS\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	graph := projectGraph(g, m)
	if graph.Truncated {
		t.Fatal("truncated graph")
	}
	classes := map[string]int{}
	for _, n := range graph.Nodes {
		classes[n.Class]++
	}
	for _, c := range []string{"TheoremNode", "ThmOrAssumpDefNode", "AssumeProveNode", "NewSymbNode", "OpDeclNode", "LeafProofNode"} {
		if classes[c] == 0 {
			t.Fatalf("missing %s", c)
		}
	}
}

func TestStatementPublicationAndRecovery(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == T\nASSUME A == Missing\nASSUME A == TRUE\nTHEOREM T\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 3 || msgs[0].Code.Value != 4200 || msgs[1].Code.Value != 4200 || msgs[2].Code.Value != 4801 {
		t.Fatalf("diagnostic order: %#v", msgs)
	}
	if len(m.theorems) != 2 || len(m.assumptions) != 2 || m.theorems[0].body != g.nullOAN || m.assumptions[0].expression != g.nullOAN {
		t.Fatal("ordinary recovery")
	}
	a, _ := g.table.resolve(g.pool.intern("A"), false)
	if a != m.assumptions[0].definition {
		t.Fatal("duplicate replaced binding")
	}
	if m.theorems[1].body.(*opApplNode).operator != m.theorems[0].definition {
		t.Fatal("theorem not published after body")
	}
}

func TestStatementCancellationAndDepthRestoreScopes(t *testing.T) {
	source := "---- MODULE M ----\nTHEOREM T == ASSUME NEW x, ASSUME NEW y PROVE y = y PROVE x = x\n====\n"
	tree := parsedModule(t, source)
	g, _ := newGenerator(stdcontext.Background(), 2)
	_, err := g.module(tree)
	if !errors.Is(err, ErrPortIncomplete) {
		t.Fatalf("depth: %v", err)
	}
	if len(g.table.stack) != 1 || g.apDepth != 0 || len(g.labelScopes) != 0 {
		t.Fatal("scope not restored")
	}
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	g, _ = newGenerator(ctx, 0)
	cancel()
	_, err = g.module(tree)
	if !errors.Is(err, stdcontext.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}

func TestNewKindsArityAndBoundBeforeDeclaration(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT x\nTHEOREM T == ASSUME NEW x \\in {x}, NEW CONSTANT F(_), NEW VARIABLE v, NEW STATE s, NEW ACTION a, NEW TEMPORAL t PROVE TRUE\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	ap := m.theorems[0].body.(*assumeProveNode)
	if len(ap.assumptions) != 6 {
		t.Fatal("NEW count")
	}
	wantKinds := []int{24, 24, 25, 26, 27, 28}
	wantLevels := []int{0, 0, 1, 1, 2, 3}
	for i, item := range ap.assumptions {
		d := item.(*newSymbNode).declaration
		if d.kind != wantKinds[i] || d.levels.level != wantLevels[i] {
			t.Fatalf("NEW %d kind %d level %d", i, d.kind, d.levels.level)
		}
	}
	if ap.assumptions[1].(*newSymbNode).declaration.arity != 1 {
		t.Fatal("operator NEW arity")
	}
	first := ap.assumptions[0].(*newSymbNode)
	set := first.set.(*opApplNode)
	outer := m.context.lookup(contextKey{name: g.pool.intern("x")})
	if set.operands[0].(*opApplNode).operator != outer || first.declaration.identity >= first.identity {
		t.Fatal("bound or allocation order")
	}
}

func TestExtendsDiamondRepeatsStatementIdentity(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	files := map[string]modulegraph.Input{"A": input("A", "THEOREM T == TRUE\n<1>1. TRUE\n<1> QED OBVIOUS\nUSE T\nASSUME FALSE"), "B": input("B", "EXTENDS A"), "C": input("C", "EXTENDS A")}
	acq, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("Root", "EXTENDS B, C"), func(_ stdcontext.Context, n string) (modulegraph.Input, error) { return files[n], nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, _, root, err := analyzeFlatExternalOwned(stdcontext.Background(), acq, 1000)
	if err != nil || len(details) != 0 {
		t.Fatalf("%v %#v", err, details)
	}
	if len(root.theorems) != 2 || root.theorems[0] != root.theorems[1] || len(root.assumptions) != 2 || root.assumptions[0] != root.assumptions[1] || len(root.topLevel) != 6 {
		t.Fatal("diamond copies")
	}
	if root.topLevel[1] != root.topLevel[4] {
		t.Fatal("USE identity lost in diamond")
	}
	if root.theorems[0].proof == nil || root.theorems[0].proof != root.theorems[1].proof {
		t.Fatal("shared proof identity")
	}
	if len(root.thmDefs()) != 1 || root.thmDefs()[0] != root.theorems[0].definition {
		t.Fatal("symbol unification")
	}
}

func TestTheoremLabelRetainsQuantifierFormalIdentity(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == \\A x : L(x):: x = x\nF == T!L(1)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.table.errors.Messages()) != 0 {
		t.Fatal(g.table.errors.Messages())
	}
	d := m.theorems[0].definition
	quant := d.body.(*opApplNode)
	label := d.labels.get(g.pool.intern("L"))
	if label == nil || len(label.params) != 1 || label.params[0] != quant.unbounded[0] {
		t.Fatal("formal identity")
	}
	f := m.definitions[1].(*userOpDefNode)
	if f.body == g.nullOAN {
		t.Fatal("label selection")
	}
}

func TestLocalAndSubstitutedTheoremImports(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	files := map[string]modulegraph.Input{"A": input("A", "CONSTANT x\nTHEOREM T == x = x"), "B": input("B", "CONSTANT x\nINSTANCE A"), "C": input("C", "CONSTANT x\nLOCAL INSTANCE A")}
	acq, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("Root", "EXTENDS B, C"), func(_ stdcontext.Context, n string) (modulegraph.Input, error) { return files[n], nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	_, _, root, err := analyzeFlatExternalOwned(stdcontext.Background(), acq, 1000)
	if err != nil {
		t.Fatal(err)
	}
	b, c := root.extendees[0], root.extendees[1]
	bd := b.definitions[0].(*thmOrAssumpDefNode)
	cd := c.definitions[0].(*thmOrAssumpDefNode)
	if bd.source == nil || bd.source != cd.source || bd.local || !cd.local || bd.instantiatedFrom != cd.instantiatedFrom {
		t.Fatal("import provenance or localness")
	}
	for _, d := range []*thmOrAssumpDefNode{bd, cd} {
		sub, ok := d.body.(*apSubstInNode)
		if !ok || sub.body != d.source.body || len(sub.substs) != 1 {
			t.Fatal("substituted theorem body")
		}
	}
}

func TestStatementChildAndWalkOrders(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT S\nTHEOREM T == ASSUME NEW x \\in S PROVE TRUE OBVIOUS\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	th := m.theorems[0]
	ap := th.body.(*assumeProveNode)
	newNode := ap.assumptions[0].(*newSymbNode)
	if got := th.children(); len(got) != 2 || got[0] != ap || got[1] != th.proof {
		t.Fatal("theorem children")
	}
	if got := newNode.walkChildren(); len(got) != 1 || got[0] != newNode.set {
		t.Fatal("NEW walk follows only its set")
	}
	sub := &apSubstInNode{body: ap, substs: []*subst{{op: newNode.declaration, expr: newNode.set}}}
	c, w := sub.children(), sub.walkChildren()
	if len(c) != 2 || c[0] != ap || c[1] != newNode.set || len(w) != 3 || w[0] != newNode.declaration || w[1] != newNode.set || w[2] != ap {
		t.Fatal("substitution order")
	}
}

func TestParameterizedInstanceReusesParameterFreeTheoremWithoutPublishing(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	files := map[string]modulegraph.Input{"A": input("A", "THEOREM T == L::TRUE"), "B": input("B", "EXTENDS A\nCONSTANT x"), "C": input("C", "CONSTANT x\nINSTANCE B"), "D": input("D", "CONSTANT x\nLOCAL INSTANCE B")}
	acq, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("Root", "EXTENDS C, D"), func(_ stdcontext.Context, n string) (modulegraph.Input, error) { return files[n], nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	_, _, root, err := analyzeFlatExternalOwned(stdcontext.Background(), acq, 1000)
	if err != nil {
		t.Fatal(err)
	}
	c, d := root.extendees[0], root.extendees[1]
	shared := c.definitions[0].(*thmOrAssumpDefNode)
	local := d.definitions[0].(*thmOrAssumpDefNode)
	if c.context.lookup(contextKey{name: root.owner.intern("T")}) != nil {
		t.Fatal("unexpected symbol publication")
	}
	if !local.local || local.source != shared || local.module != d || local.labels != nil || local.body != shared.body {
		t.Fatal("LOCAL clone asymmetry")
	}
}

func TestSourceDerivedStatementFixtures(t *testing.T) {
	for _, tc := range []struct {
		name       string
		incomplete bool
		errors     int
	}{
		{"Statements", false, 0}, {"Scopes", false, 0}, {"UnsupportedProof", false, 0}, {"SelectorScope", false, 1},
	} {
		raw, err := os.ReadFile("../../testdata/assumption-theorem/" + tc.name + ".tla")
		if err != nil {
			t.Fatal(err)
		}
		tree := parsedModule(t, string(raw))
		g, _ := newGenerator(stdcontext.Background(), 0)
		_, err = g.module(tree)
		if errors.Is(err, ErrPortIncomplete) != tc.incomplete || err != nil && !tc.incomplete || len(g.table.errors.Messages()) != tc.errors {
			t.Fatalf("%s: %v %#v", tc.name, err, g.table.errors.Messages())
		}
	}
}

func TestTopLevelLabeledAssumeProveRecovery(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == ASSUME L:: ASSUME NEW x PROVE TRUE PROVE TRUE\n====\n")
	theorem := tree.Heirs()[2].Heirs()[0]
	apTree := theorem.Heirs()[len(theorem.Heirs())-1]
	labelTree := apTree.Heirs()[1]
	if labelTree.Kind() != generated.N_Label {
		t.Fatalf("fixture label kind %d", labelTree.Kind())
	}
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, err := g.module(parsedModule(t, "---- MODULE Empty ----\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	g.labelScopes = append(g.labelScopes, labelFrame{})
	result, err := g.expression(labelTree, 1)
	if err != nil {
		t.Fatal(err)
	}
	label, ok := result.(*labelNode)
	if !ok || !label.isAssumeProve {
		t.Fatalf("body %T", result)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4004 || msgs[0].Format != "Labeled ASSUME/PROVE used where an expression is required." {
		t.Fatalf("diagnostics %#v", msgs)
	}
	ap := label.body.(*assumeProveNode)
	if ap.goal != nil || !ap.inProof || g.table.top.lookup(contextKey{name: g.pool.intern("x")}) == nil {
		t.Fatal("top-level labeled AP constructor/context")
	}
}

func TestTheoremSelectorsInOperatorArgumentMode(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == \\A x : L(x):: <<x, 2>>\nApply(Op(_)) == Op(1)\nUse(Op(_,_)) == Op(1,2)\nA == Apply(T!L)\nB == Apply(T!L!2)\nC == Use(T!L)\nD == Apply(T!L(1))\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 4 || msgs[0].Code.Value != 4271 || msgs[1].Code.Value != 4271 || msgs[2].Code.Value != 4270 || msgs[3].Code.Value != 4271 {
		t.Fatalf("diagnostics %#v", msgs)
	}
	theorem := m.theorems[0].definition
	label := theorem.labels.get(g.pool.intern("L"))
	for _, idx := range []int{3, 4} {
		app := m.definitions[idx].(*userOpDefNode).body.(*opApplNode)
		arg, ok := app.operands[0].(*opArgNode)
		if !ok {
			t.Fatalf("argument %T", app.operands[0])
		}
		lambda := arg.operator.(*userOpDefNode)
		if len(lambda.params) != 1 || lambda.params[0] != label.params[0] {
			t.Fatal("formal identity")
		}
	}
}

func TestTheoremAPMixedNameAndStructuralSelectors(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == ASSUME LET G(x) == x IN G(1), TRUE PROVE TRUE\nApply(Op(_)) == Op(1)\nA == T!1!G(2)\nB == Apply(T!1!G)\nC == T!2!Missing\nD == T!2!:\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Format != "A name selector here must be from a LET clause." || msgs[1].Format != "!: should not follow an operand selector." {
		t.Fatalf("diagnostics %#v", msgs)
	}
	for _, idx := range []int{2, 3} {
		if m.definitions[idx].(*userOpDefNode).body == g.nullOAN {
			t.Fatalf("mixed selector %d rejected", idx)
		}
	}
}

func TestTheoremAPNestedStructuralLetSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == ASSUME <<LET G(x) == x IN G(1), 0>> PROVE TRUE\nApply(Op(_)) == Op(1)\nA == T!1!1!G(2)\nB == Apply(T!1!1!G)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil || len(g.table.errors.Messages()) != 0 {
		t.Fatalf("%v %#v", err, g.table.errors.Messages())
	}
	if m.definitions[2].(*userOpDefNode).body == g.nullOAN || m.definitions[3].(*userOpDefNode).body == g.nullOAN {
		t.Fatal("nested selector rejected")
	}
}

func TestQualifiedTheoremLabelInOperatorArgumentMode(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	acq, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("B", "I(y) == INSTANCE A WITH k <- y\nUse(Op(_,_)) == Op(1,2)\nF == Use(I!T!L)"), func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		return input("A", "CONSTANT k\nTHEOREM T == \\A x : L(x):: <<k,x>>"), nil
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, _, root, err := analyzeFlatExternalOwned(stdcontext.Background(), acq, 1000)
	if err != nil || len(details) != 0 {
		t.Fatalf("%v %#v", err, details)
	}
	var f *userOpDefNode
	var theorem *thmOrAssumpDefNode
	for _, d := range root.definitions {
		switch v := d.(type) {
		case *userOpDefNode:
			if v.name.text == "F" {
				f = v
			}
		case *thmOrAssumpDefNode:
			theorem = v
		}
	}
	arg := f.body.(*opApplNode).operands[0].(*opArgNode)
	lambda := arg.operator.(*userOpDefNode)
	if len(lambda.params) != 2 || lambda.params[0] != theorem.params[0] || lambda.params[1] != theorem.labels.get(root.owner.intern("L")).params[0] {
		t.Fatal("qualified formals")
	}
	if _, ok := lambda.body.(*substInNode); !ok {
		t.Fatal("substitution prefix missing")
	}
}

func TestTheoremLabeledNestedAPToLetSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == ASSUME L:: ASSUME LET G(x) == x IN G(1) PROVE TRUE PROVE TRUE\nApply(Op(_)) == Op(1)\nA == T!L!1!G(2)\nB == Apply(T!L!1!G)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil || len(g.table.errors.Messages()) != 0 {
		t.Fatalf("%v %#v", err, g.table.errors.Messages())
	}
	for _, i := range []int{2, 3} {
		if m.definitions[i].(*userOpDefNode).body == g.nullOAN {
			t.Fatal("labeled AP selector rejected")
		}
	}
}

func TestTheoremAPStructuralLetThenLabelSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == ASSUME <<LET G(x) == L:: <<x,2>> IN G(1), 0>> PROVE TRUE\nUse(Op(_)) == Op(1)\nA == T!1!1!G(2)!L!1\nB == Use(T!1!1!G!L!1)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil || len(g.table.errors.Messages()) != 0 {
		t.Fatalf("%v %#v", err, g.table.errors.Messages())
	}
	for _, i := range []int{2, 3} {
		if m.definitions[i].(*userOpDefNode).body == g.nullOAN {
			t.Fatal("mixed selector rejected")
		}
	}
}

func TestTheoremColonSelectorTransitions(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == ASSUME L:: LET G(x) == x IN G(1) PROVE TRUE\nApply(Op(_)) == Op(1)\nA == T!L!:!G(2)\nB == Apply(T!L!:!G)\nC == T!:\nD == T!:!Missing\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Format != "Selected ASSUME/PROVE instead of expression." || msgs[1].Format != "A name selector here must be from a LET clause." {
		t.Fatalf("diagnostics %#v", msgs)
	}
	for _, i := range []int{2, 3} {
		if m.definitions[i].(*userOpDefNode).body == g.nullOAN {
			t.Fatal("LET colon rejected")
		}
	}
}

func TestTheoremAPOperatorArgumentDiagnosticOrdering(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == ASSUME L:: ASSUME TRUE PROVE TRUE PROVE TRUE\nApply(Op(_)) == Op(1)\nA == Apply(T!L)\nB == Apply(T!L!:)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 4 || msgs[0].Code.Value != 4271 || msgs[1].Code.Value != 4271 || msgs[2].Format != "Selected ASSUME/PROVE instead of expression." || msgs[3].Code.Value != 4271 {
		t.Fatalf("diagnostic order %#v", msgs)
	}
}

func TestTheoremAPInvalidSelectorDoesNotEvaluateArguments(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == ASSUME TRUE PROVE TRUE\nApply(Op(_)) == Op(1)\nA == T!@\nB == T!(Missing)\nC == Apply(T!(Missing))\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 4 || msgs[0].Format != "Nonexistent operand specified by `%s'." || msgs[0].Parameters[0].Text != "@" || msgs[1].Format != "Nonexistent operand specified by `%s'." || msgs[1].Parameters[0].Text != "!(...)" || msgs[2].Format != "Selector `%s' should not have argument(s)." || msgs[3].Code.Value != 4271 {
		t.Fatalf("diagnostics %#v", msgs)
	}
}

func TestTheoremNewSelectorBoundaries(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == ASSUME NEW x, TRUE PROVE TRUE\nApply(Op(_)) == Op(1)\nA == T!1\nB == T!1!L\nC == Apply(T!1)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 4 || msgs[0].Format != "Selected a NEW declaration as an expression or operator." || msgs[1].Format != "Selected a subexpression of a NEW clause of an ASSUME." || msgs[2].Format != "Selected a NEW declaration as an expression or operator." || msgs[3].Code.Value != 4271 {
		t.Fatalf("diagnostics %#v", msgs)
	}
}

func TestDirectAPTheoremOperatorArgumentRejection(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(parsedModule(t, "---- MODULE M ----\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	name := g.pool.intern("T")
	d, err := newThmOrAssumpDef(g.pool, name, nil)
	if err != nil {
		t.Fatal(err)
	}
	formal, err := newFormalParam(g.pool, g.pool.intern("x"), 0, nil, nil, m)
	if err != nil {
		t.Fatal(err)
	}
	h, err := newSemanticHeader(g.pool, 14, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.params = []*formalParamNode{formal}
	d.body = &assumeProveNode{semanticHeader: h}
	d.module = m
	if _, err = g.table.addSymbol(name, d); err != nil {
		t.Fatal(err)
	}
	tree := parsedModule(t, "---- MODULE N ----\nF == T\n====\n")
	body := tree.Heirs()[2].Heirs()[0].Heirs()[2]
	result, err := g.expressionOrOperatorArgument(body, 1, 0, d, body, 1)
	if err != nil || result != g.nullOAN {
		t.Fatalf("%T %v", result, err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4355 {
		t.Fatalf("diagnostics %#v", msgs)
	}
}

func TestTheoremAPLetUndefinedRecursiveSelectorRecovery(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM T == ASSUME <<LET RECURSIVE G(_) IN TRUE>> PROVE TRUE\nApply(Op(_)) == Op(1)\nA == T!1!1!G(Missing)\nB == Apply(T!1!1!G)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	want := []int{4291, 4200, 4005, 4005, 4271}
	if len(msgs) != len(want) {
		t.Fatalf("diagnostics %#v", msgs)
	}
	for i, code := range want {
		if msgs[i].Code.Value != code {
			t.Fatalf("diagnostic %d: %#v", i, msgs)
		}
	}
}
