// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"errors"
	"testing"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

// rsLeft is the left operand of the module's final ASSUME e = ....
func rsLeft(t *testing.T, g *generator, m *moduleNode, err error) *opApplNode {
	t.Helper()
	if err != nil || m == nil {
		t.Fatalf("analyze: %v (site %q)", err, g.incomplete.site)
	}
	if d := renderedDiagnostics(t, g); len(d) != 0 {
		t.Fatalf("diagnostics %q", d)
	}
	assume, ok := m.topLevel[len(m.topLevel)-1].(*assumeNode)
	if !ok {
		t.Fatalf("top level %T", m.topLevel[len(m.topLevel)-1])
	}
	eq, ok := assume.expression.(*opApplNode)
	if !ok || len(eq.operands) != 2 {
		t.Fatalf("assumption %#v", assume.expression)
	}
	left, ok := eq.operands[0].(*opApplNode)
	if !ok {
		t.Fatalf("left operand %#v", eq.operands[0])
	}
	return left
}

// rsLambda is the LAMBDA operator of an application's only argument.
func rsLambda(t *testing.T, app *opApplNode) *userOpDefNode {
	t.Helper()
	if len(app.operands) != 1 {
		t.Fatalf("operands %d", len(app.operands))
	}
	arg, ok := app.operands[0].(*opArgNode)
	if !ok {
		t.Fatalf("argument %#v", app.operands[0])
	}
	lambda, ok := arg.operator.(*userOpDefNode)
	if !ok || lambda.name.text != "LAMBDA" {
		t.Fatalf("argument operator %#v", arg.operator)
	}
	return lambda
}

func rsDefinition(t *testing.T, m *moduleNode, text string) *userOpDefNode {
	t.Helper()
	for _, d := range testUserDefinitions(t, m.definitions) {
		if d.name.text == text {
			return d
		}
	}
	t.Fatalf("no definition %s", text)
	return nil
}

func rsParams(t *testing.T, got []*formalParamNode, want ...*formalParamNode) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d params, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("param %d is %s, want the source formal %s", i, got[i].name.text, want[i].name.text)
		}
	}
}

// A LET definition selected in operator mode and followed by a colon keeps
// its own formals and body: FindingOpName adds the definition's parameters,
// the colon leaves curNode at the body, and the LAMBDA takes both
// (Generator.java:960-990, 1128-1136, 1993-2028).
func TestRemainingSelectorLetOperandLambda(t *testing.T) {
	g, m, err := esAnalyze(t, lcU+"T(P(_)) == P(1)\nASSUME T(U!1!Op!:) = 1")
	lambda := rsLambda(t, rsLeft(t, g, m, err))
	conj, ok := rsDefinition(t, m, "U").body.(*opApplNode)
	if !ok {
		t.Fatalf("U body %#v", rsDefinition(t, m, "U").body)
	}
	let, ok := conj.operands[0].(*letInNode)
	if !ok {
		t.Fatalf("first conjunct %#v", conj.operands[0])
	}
	op, ok := let.context.lookup(contextKey{name: g.pool.intern("Op")}).(*userOpDefNode)
	if !ok {
		t.Fatal("no LET definition Op")
	}
	rsParams(t, lambda.params, op.params...)
	if lambda.body != op.body {
		t.Fatalf("LAMBDA body %#v, want Op's body", lambda.body)
	}
}

// After a LET definition the labels are that definition's: the LAMBDA takes
// the root, label, LET-definition and inner-label formals in order, and its
// body is the selected label (Generator.java:1064-1118, 1993-2028). The
// operator token is looked up after resolveSynonym.
func TestRemainingSelectorLetLabelLambda(t *testing.T) {
	body := "K(q) == \\E x \\in {q} : lab(x) :: LET a (+) b == {inner(y) :: <<y, a, b>> : y \\in {x}}\n" +
		"                                 IN 1 (+) 2\n" +
		"Use(Op(_, _, _, _, _)) == Op(1, 2, 3, 4, 5)\nASSUME Use(K!lab!:!\\oplus!inner) = 1"
	g, m, err := esAnalyze(t, body)
	lambda := rsLambda(t, rsLeft(t, g, m, err))
	k := rsDefinition(t, m, "K")
	lab := k.labels.get(g.pool.intern("lab"))
	if lab == nil {
		t.Fatal("no label lab")
	}
	let, ok := lab.body.(*letInNode)
	if !ok || len(let.definitions) != 1 {
		t.Fatalf("label body %#v", lab.body)
	}
	plus, ok := let.definitions[0].(*userOpDefNode)
	if !ok {
		t.Fatalf("LET definition %#v", let.definitions[0])
	}
	inner := plus.labels.get(g.pool.intern("inner"))
	if inner == nil {
		t.Fatal("no label inner")
	}
	rsParams(t, lambda.params, k.params[0], lab.params[0], plus.params[0], plus.params[1], inner.params[0])
	if lambda.body != inner {
		t.Fatalf("LAMBDA body %#v, want the inner label", lambda.body)
	}
}

// An undefined instance name resolves as the compound P!F; P's tempArgs
// precede F's own arguments in one application (Generator.java:732-805,
// 870-946, 1910-1916).
func TestRemainingSelectorUnresolvedCompoundApplication(t *testing.T) {
	src := "---- MODULE Root ----\n---- MODULE N ----\nF(a, b) == a\n====\n---- MODULE M ----\nP(x) == INSTANCE N\n====\nINSTANCE M\nASSUME P(1)!F(2, 3) = 1\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, src), 0)
	app := rsLeft(t, g, m, err)
	op, ok := app.operator.(*userOpDefNode)
	if !ok || op.name.text != "P!F" || op.arity != 3 {
		t.Fatalf("operator %#v", app.operator)
	}
	lcNumerals(t, app, "1", "2", "3")
}

func rsFind(n *generated.SyntaxTreeNode, match func(*generated.SyntaxTreeNode) bool) *generated.SyntaxTreeNode {
	if n == nil {
		return nil
	}
	if match(n) {
		return n
	}
	for _, h := range n.Heirs() {
		if found := rsFind(h, match); found != nil {
			return found
		}
	}
	return nil
}

// unknownLetSelector checks cancellation and the depth budget at every
// suffix step before reading or interning it, skipped operand selectors
// included, as definitionLevelSelector checks each selector step.
func TestRemainingSelectorUnknownLetSelectorChecks(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nA == B!1!c\n====\n")
	ident := func(text string) func(*generated.SyntaxTreeNode) bool {
		return func(n *generated.SyntaxTreeNode) bool {
			return n.Kind() == 289 && n.Image() != nil && n.Image().String() == text
		}
	}
	first, last := rsFind(tree, ident("B")), rsFind(tree, ident("c"))
	operand := rsFind(tree, func(n *generated.SyntaxTreeNode) bool { return n.Kind() == generated.N_StructOp })
	if first == nil || last == nil || operand == nil {
		t.Fatal("selector nodes not found")
	}
	suffix := []*generated.SyntaxTreeNode{first, operand, last}
	next := func(j int) *generated.SyntaxTreeNode {
		if j < len(suffix) {
			return suffix[j]
		}
		return nil
	}
	run := func(ctx stdcontext.Context, maxDepth, depth int) (*generator, error) {
		t.Helper()
		g, err := newGenerator(ctx, maxDepth)
		if err != nil {
			t.Fatal(err)
		}
		_, err = g.unknownLetSelector(newContext(g.pool), g.pool.intern("B"), first, depth, next)
		return g, err
	}
	bg := stdcontext.Background()
	// Within budget: the operand step (depth 6) and the name step (depth 7)
	// fit, and the compound is reported.
	g, err := run(bg, 7, 5)
	if msgs := g.table.errors.Messages(); err != nil || len(msgs) != 1 || msgs[0].Code.Value != 4200 || msgs[0].Parameters[0].Text != "B!c" {
		t.Fatalf("bounded: %v %#v", err, msgs)
	}
	// The skipped operand consumes a step, so the name step exceeds the
	// budget; nothing is reported.
	for _, maxDepth := range []int{6, 5} {
		g, err = run(bg, maxDepth, 5)
		var limit *DepthLimitError
		if !errors.As(err, &limit) || len(g.table.errors.Messages()) != 0 {
			t.Fatalf("depth %d: %v %#v", maxDepth, err, g.table.errors.Messages())
		}
	}
	ctx, cancel := stdcontext.WithCancel(bg)
	cancel()
	if g, err = run(ctx, 0, 0); err != stdcontext.Canceled || len(g.table.errors.Messages()) != 0 {
		t.Fatalf("cancellation: %v %#v", err, g.table.errors.Messages())
	}
}
