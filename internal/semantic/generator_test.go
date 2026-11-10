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
	"unicode/utf16"
)

func parsedModule(t *testing.T, source string) *generated.SyntaxTreeNode {
	t.Helper()
	r, e := generated.Parse(stdcontext.Background(), []byte(source), generated.ParseOptions{})
	if e != nil || !r.Accepted || r.Tree == nil {
		if len(r.Errors) > 0 {
			t.Fatalf("parse: %s", r.Errors[0].ReportedError().String())
		}
		t.Fatalf("parse: %#v %v", r, e)
	}
	return r.Tree
}
func TestGeneratorLiteralDefinitions(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nN == 1\nLOCAL D == (1.20)\nS == \"hi\"\n====\n")
	g, e := newGenerator(stdcontext.Background(), 0)
	if e != nil {
		t.Fatal(e)
	}
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(testUserDefinitions(t, m.definitions)) != 3 || testUserDefinitions(t, m.definitions)[0].name.text != "N" || !testUserDefinitions(t, m.definitions)[1].local || testUserDefinitions(t, m.definitions)[2].name.text != "S" {
		t.Fatal(testUserDefinitions(t, m.definitions))
	}
	if _, ok := testUserDefinitions(t, m.definitions)[0].body.(*numeralNode); !ok {
		t.Fatal("numeral body")
	}
	if d, ok := testUserDefinitions(t, m.definitions)[1].body.(*decimalNode); !ok || d.mantissa != 120 || d.exponent != -2 {
		t.Fatal("decimal body")
	}
	if s, ok := testUserDefinitions(t, m.definitions)[2].body.(*stringNode); !ok || s.value.text != "hi" {
		t.Fatal("string body")
	}
}
func TestGeneratorDuplicateAndRejectedSelectorScope(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nN == 1\nN == 2\nF(x) == x!foo\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(testUserDefinitions(t, m.definitions)) != 3 || len(g.table.stack) != 1 || g.table.top != g.table.base {
		t.Fatal("scope or definition list")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 3 || msgs[2].Code.Value != 4005 || msgs[2].Format != "Cannot take subexpression of `%s'." {
		t.Fatalf("diagnostics: %#v", msgs)
	}
	if s, _ := g.table.resolve(g.pool.intern("x"), false); s != nil {
		t.Fatal("leaked parameter")
	}
}
func TestGeneratorCancellation(t *testing.T) {
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	g, _ := newGenerator(ctx, 0)
	cancel()
	_, e := g.module(nil)
	if !errors.Is(e, stdcontext.Canceled) {
		t.Fatal(e)
	}
}

func TestGeneratorResolvedNames(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nN == 1\nF(x) == x\nT == TRUE\nQ == N\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(testUserDefinitions(t, m.definitions)) != 4 {
		t.Fatal(len(testUserDefinitions(t, m.definitions)))
	}
	f := testUserDefinitions(t, m.definitions)[1]
	x, ok := f.body.(*opApplNode)
	if !ok || x.operator != f.params[0] || len(x.operands) != 0 {
		t.Fatal("formal binding")
	}
	tr, ok := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	if !ok || tr.operator.header().name.text != "TRUE" {
		t.Fatal("builtin binding")
	}
	q, ok := testUserDefinitions(t, m.definitions)[3].body.(*opApplNode)
	if !ok || q.operator != testUserDefinitions(t, m.definitions)[0] || q.subExpressionOf != testUserDefinitions(t, m.definitions)[0] {
		t.Fatal("definition binding")
	}
}

func TestGeneratorDeclarationsAndApplications(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT C\nVARIABLE v\nId(x) == x\nUse == Id(C)\nNext == Id(v)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(testUserDefinitions(t, m.definitions)) != 3 || !m.hasDeclarations(2) || !m.hasDeclarations(3) {
		t.Fatal("declarations")
	}
	for i, want := range []string{"C", "v"} {
		app, ok := testUserDefinitions(t, m.definitions)[i+1].body.(*opApplNode)
		if !ok || app.operator != testUserDefinitions(t, m.definitions)[0] || len(app.operands) != 1 {
			t.Fatal("application")
		}
		arg, ok := app.operands[0].(*opApplNode)
		if !ok || arg.operator.header().name.text != want {
			t.Fatal("argument binding")
		}
	}
}

func TestGeneratorNameAndArityErrorsContinue(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nId(x) == x\nA == Missing\nB == Id\nC == Id(1, 2)\nD == 1\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(testUserDefinitions(t, m.definitions)) != 5 {
		t.Fatal(len(testUserDefinitions(t, m.definitions)))
	}
	for _, d := range testUserDefinitions(t, m.definitions)[1:4] {
		if d.body != g.nullOAN {
			t.Fatal("expected source-defined null application")
		}
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 3 || msgs[0].Code.Value != 4200 || msgs[1].Code.Value != 4204 || msgs[2].Code.Value != 4204 {
		t.Fatalf("diagnostics: %#v", msgs)
	}
}

func TestGeneratorNotationAndSynonyms(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nVARIABLE v\nE == 1 = 1\nN == ~TRUE\nP == v'\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	for i, want := range []string{"=", "\\lnot", "'"} {
		app, ok := testUserDefinitions(t, m.definitions)[i].body.(*opApplNode)
		if !ok || app.operator.header().name.text != want {
			t.Fatalf("operator %s: %#v", want, testUserDefinitions(t, m.definitions)[i].body)
		}
	}
}

func TestGeneratorBuiltinExpressionConstructors(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nS == {1, 2}\nT == <<1, 2>>\nI == IF TRUE THEN 1 ELSE 2\nF == [BOOLEAN -> BOOLEAN]\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatalf("%v after %d definitions", e, len(testUserDefinitions(t, m.definitions)))
	}
	for i, want := range []string{"$SetEnumerate", "$Tuple", "$IfThenElse", "$SetOfFcns"} {
		app, ok := testUserDefinitions(t, m.definitions)[i].body.(*opApplNode)
		if !ok || app.operator.header().name.text != want {
			t.Fatalf("%s: %#v", want, testUserDefinitions(t, m.definitions)[i].body)
		}
	}
}

func TestGeneratorEmptyCollectionsAndLists(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nS == {}\nT == <<>>\nC == /\\ TRUE\n     /\\ FALSE\nD == \\/ TRUE\n     \\/ FALSE\nX == BOOLEAN \\X BOOLEAN\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	for i, want := range []string{"$SetEnumerate", "$Tuple", "$ConjList", "$DisjList", "$CartesianProd"} {
		app, ok := testUserDefinitions(t, m.definitions)[i].body.(*opApplNode)
		if !ok || app.operator.header().name.text != want {
			t.Fatalf("%s: %#v", want, testUserDefinitions(t, m.definitions)[i].body)
		}
	}
}

func TestGeneratorRecordsAndDuplicateFields(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nR == [a |-> 1, a |-> 2]\nS == [a: BOOLEAN]\nF == R.a\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.records) != 2 || m.records[0] != testUserDefinitions(t, m.definitions)[0].body || m.records[1] != testUserDefinitions(t, m.definitions)[1].body {
		t.Fatal("record ordering")
	}
	r := m.records[0]
	if len(r.operands) != 2 {
		t.Fatal("fields")
	}
	pair, ok := r.operands[0].(*opApplNode)
	if !ok || pair.operator.header().name.text != "$Pair" {
		t.Fatal("pair")
	}
	field, ok := pair.operands[0].(*stringNode)
	if !ok || field.value.text != "a" {
		t.Fatal("field")
	}
	sel, ok := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	if !ok || sel.operator.header().name.text != "$RcdSelect" {
		t.Fatal("selection")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4262 {
		t.Fatal(msgs)
	}
}

func TestGeneratorOperatorConstantDeclaration(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT F(_)\nA == F(1)\nB == F\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	app, ok := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	if !ok || nodeArity(app.operator) != 1 || len(app.operands) != 1 {
		t.Fatal("constant application")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4204 {
		t.Fatal(msgs)
	}
}

func TestGeneratorActionForms(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nVARIABLE x\nA == [x' = x]_x\nB == <<x' = x>>_x\nC == WF_x(x' = x)\nD == SF_x(x' = x)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatalf("%v after %d definitions", e, len(testUserDefinitions(t, m.definitions)))
	}
	for i, want := range []string{"$SquareAct", "$AngleAct", "$WF", "$SF"} {
		app, ok := testUserDefinitions(t, m.definitions)[i].body.(*opApplNode)
		if !ok || app.operator.header().name.text != want {
			t.Fatalf("%s: %#v", want, testUserDefinitions(t, m.definitions)[i].body)
		}
	}
}

func TestGeneratorUnboundedQuantifierScope(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nQ == \\A x, y : x = y\nR == x\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	q, ok := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	if !ok || q.operator.header().name.text != "$UnboundedForall" || len(q.unbounded) != 2 {
		t.Fatal("quantifier")
	}
	eq, ok := q.operands[0].(*opApplNode)
	if !ok {
		t.Fatal("body")
	}
	for i, arg := range eq.operands {
		v, ok := arg.(*opApplNode)
		if !ok || v.operator != q.unbounded[i] {
			t.Fatal("quantifier binding")
		}
	}
	if testUserDefinitions(t, m.definitions)[1].body != g.nullOAN || len(g.table.errors.Messages()) != 1 || g.table.errors.Messages()[0].Code.Value != 4200 {
		t.Fatal("scope leaked")
	}
}

func TestGeneratorBoundedQuantifierScope(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nQ == \\A x \\in {1}, y \\in {x}: x = y\nR == x\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	q, ok := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	if !ok || q.operator.header().name.text != "$BoundedForall" || len(q.bounded) != 2 || len(q.ranges) != 2 {
		t.Fatal("bounded")
	}
	eq, ok := q.operands[0].(*opApplNode)
	if !ok {
		t.Fatal("body")
	}
	for i, arg := range eq.operands {
		v, ok := arg.(*opApplNode)
		if !ok || v.operator != q.bounded[i][0] {
			t.Fatal("bound symbol")
		}
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4200 || msgs[1].Code.Value != 4200 {
		t.Fatal("bound scope diagnostics", msgs)
	}
}

func TestGeneratorTupleQuantifier(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nQ == \\E <<x, y>> \\in {<<1, 2>>}: x = y\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	q, ok := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	if !ok || q.operator.header().name.text != "$BoundedExists" || len(q.bounded) != 1 || len(q.bounded[0]) != 2 || q.bounded[0][0].name.text != "x" || q.bounded[0][1].name.text != "y" || !q.tupleBounds[0] {
		t.Fatal("tuple binding")
	}
}

func TestGeneratorSetAndFunctionBoundForms(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nS == {x : x \\in {1}}\nF == [x \\in {1} |-> x]\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	for i, want := range []string{"$SetOfAll", "$FcnConstructor"} {
		a, ok := testUserDefinitions(t, m.definitions)[i].body.(*opApplNode)
		if !ok || a.operator.header().name.text != want || len(a.bounded) != 1 || len(a.ranges) != 1 {
			t.Fatal(want)
		}
		body, ok := a.operands[0].(*opApplNode)
		if !ok || body.operator != a.bounded[0][0] {
			t.Fatal("binding")
		}
	}
}

func TestGeneratorChooseAndSubset(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nA == CHOOSE x : x = 1\nB == CHOOSE y \\in {1} : y = 1\nC == {z \\in {1} : z = 1}\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	for i, want := range []string{"$UnboundedChoose", "$BoundedChoose", "$SubsetOf"} {
		a, ok := testUserDefinitions(t, m.definitions)[i].body.(*opApplNode)
		if !ok || a.operator.header().name.text != want {
			t.Fatal(want)
		}
		var param *formalParamNode
		if i == 0 {
			param = a.unbounded[0]
		} else {
			param = a.bounded[0][0]
		}
		body, ok := a.operands[0].(*opApplNode)
		if !ok {
			t.Fatal("body")
		}
		v, ok := body.operands[0].(*opApplNode)
		if !ok || v.operator != param {
			t.Fatal("binding")
		}
	}
}

func TestGeneratorTupleChooseAndSubset(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nA == CHOOSE <<x, y>> \\in {<<1, 2>>}: x = y\nB == {<<x, y>> \\in {<<1, 2>>}: x = y}\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range testUserDefinitions(t, m.definitions) {
		a, ok := d.body.(*opApplNode)
		if !ok || len(a.bounded) != 1 || len(a.bounded[0]) != 2 || !a.tupleBounds[0] {
			t.Fatal("tuple binding")
		}
	}
}

func TestGeneratorFunctionApplication(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT F\nA == F[1]\nB == F[1, 2]\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	a := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	b := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	if a.operator.header().name.text != "$FcnApply" || b.operator.header().name.text != "$FcnApply" {
		t.Fatal("function applications")
	}
	if _, ok := a.operands[1].(*numeralNode); !ok {
		t.Fatal("single argument")
	}
	tuple, ok := b.operands[1].(*opApplNode)
	if !ok || tuple.operator.header().name.text != "$Tuple" || len(tuple.operands) != 2 || tuple.tree != b.tree {
		t.Fatal("tuple argument")
	}
}

func TestGeneratorCaseOther(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nA == CASE TRUE -> 1 [] OTHER -> 2\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	a, ok := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	if !ok || a.operator.header().name.text != "$Case" || len(a.operands) != 2 {
		t.Fatal("case")
	}
	other, ok := a.operands[1].(*opApplNode)
	if !ok || other.operands[0] != nil || other.operator.header().name.text != "$Pair" {
		t.Fatal("OTHER arm")
	}
}

func TestGeneratorOperatorNotationDeclarations(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT _+_\nA == 1 + 2\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	a, ok := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	if !ok || a.operator.header().name.text != "+" || nodeArity(a.operator) != 2 {
		t.Fatal("infix declaration binding")
	}
}

func TestGeneratorLetScopeAndDefinitionOrder(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nA == LET F(x) == x\n         G == F(1)\n     IN G\nB == G\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(testUserDefinitions(t, m.definitions)) != 4 || testUserDefinitions(t, m.definitions)[0].name.text != "F" || testUserDefinitions(t, m.definitions)[1].name.text != "G" || testUserDefinitions(t, m.definitions)[2].name.text != "A" {
		t.Fatal("module definition ordering")
	}
	let, ok := testUserDefinitions(t, m.definitions)[2].body.(*letInNode)
	if !ok || len(testUserDefinitions(t, let.definitions)) != 2 || testUserDefinitions(t, let.definitions)[0].letInLevel != 1 {
		t.Fatal("let")
	}
	body, ok := let.body.(*opApplNode)
	if !ok || body.operator != testUserDefinitions(t, let.definitions)[1] {
		t.Fatal("let binding")
	}
	if testUserDefinitions(t, m.definitions)[3].body != g.nullOAN || len(g.table.errors.Messages()) != 1 {
		t.Fatal("let scope leaked")
	}
}

func TestGeneratorInfixDefinition(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nx + y == x\nA == 1 + 2\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	op := testUserDefinitions(t, m.definitions)[0]
	if op.name.text != "+" || len(op.params) != 2 {
		t.Fatal("definition")
	}
	body, ok := op.body.(*opApplNode)
	if !ok || body.operator != op.params[0] {
		t.Fatal("formal")
	}
	app, ok := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	if !ok || app.operator != op || len(app.operands) != 2 || app.subExpressionOf != nil {
		t.Fatal("infix binding")
	}
}

func TestGeneratorPrefixPostfixDefinitions(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\n~ x == x\ny ^+ == y\nA == ~ TRUE\nB == 2^+\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(testUserDefinitions(t, m.definitions)) != 4 {
		t.Fatal(len(testUserDefinitions(t, m.definitions)))
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4201 || msgs[1].Code.Value != 4202 {
		t.Fatal("builtin diagnostics", msgs)
	}
	for i := 0; i < 2; i++ {
		d := testUserDefinitions(t, m.definitions)[i]
		if len(d.params) != 1 {
			t.Fatal("formal")
		}
		body, ok := d.body.(*opApplNode)
		if !ok || body.operator != d.params[0] {
			t.Fatal("formal binding")
		}
		app, ok := testUserDefinitions(t, m.definitions)[i+2].body.(*opApplNode)
		if !ok {
			t.Fatal("notation application")
		}
		if i == 0 {
			if app.operator.header().name.text != "\\lnot" {
				t.Fatal("builtin binding")
			}
		} else if app.operator != d {
			t.Fatal("postfix binding")
		}
	}
}

func TestGeneratorHigherOrderArguments(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nApply(Op(_), x) == Op(x)\nG(y) == y\nT == TRUE\nA == Apply(G, 1)\nB == Apply(1, 1)\nC == Apply(T, 1)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	apply := testUserDefinitions(t, m.definitions)[0]
	if len(apply.params) != 2 || apply.params[0].arity != 1 {
		t.Fatal("formal operator")
	}
	body, ok := apply.body.(*opApplNode)
	if !ok || body.operator != apply.params[0] {
		t.Fatal("formal operator binding")
	}
	app, ok := testUserDefinitions(t, m.definitions)[3].body.(*opApplNode)
	if !ok || app.subExpressionOf != apply {
		t.Fatal("application")
	}
	arg, ok := app.operands[0].(*opArgNode)
	if !ok || arg.operator != testUserDefinitions(t, m.definitions)[1] || arg.arity != 1 || arg.module != m {
		t.Fatal("operator argument")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 4 || msgs[0].Code.Value != 4270 || msgs[1].Code.Value != 4271 || msgs[2].Code.Value != 4271 || msgs[3].Code.Value != 4271 {
		t.Fatalf("diagnostic order: %#v", msgs)
	}
}

func TestGeneratorHigherOrderMatching(t *testing.T) {
	source, err := os.ReadFile("../../testdata/higher_order/Matching.tla")
	if err != nil {
		t.Fatal(err)
	}
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(parsedModule(t, string(source)))
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 3 || msgs[0].Code.Value != 4271 || msgs[1].Code.Value != 4271 || msgs[2].Code.Value != 4271 {
		t.Fatalf("diagnostic order: %#v", msgs)
	}
	if msgs[0].Location.BeginLine != 7 || msgs[1].Location.BeginLine != 9 || msgs[2].Location.BeginLine != 9 || msgs[1].Parameters[0].Integer != 1 || msgs[2].Parameters[0].Integer != 2 {
		t.Fatalf("diagnostic location or argument order: %#v", msgs)
	}
	defs := testUserDefinitions(t, m.definitions)
	if _, ok := defs[3].body.(*opApplNode); !ok {
		t.Fatal("rejected argument application must be retained")
	}
}

func TestGeneratorInfixFormalParameter(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nApply(_+_, x) == x + x\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	d := testUserDefinitions(t, m.definitions)[0]
	if len(d.params) != 2 || d.params[0].name.text != "+" || d.params[0].arity != 2 {
		t.Fatal("formal arity")
	}
	body, ok := d.body.(*opApplNode)
	if !ok || body.operator != d.params[0] {
		t.Fatal("formal binding")
	}
}

func TestGeneratorRecursiveOperatorAndUndefined(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nRECURSIVE F(_), U(_)\nCONSTANT C\nF(x) == IF x = 0 THEN 0 ELSE F(x)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.recursiveDecls) != 2 || len(testUserDefinitions(t, m.definitions)) != 1 || testUserDefinitions(t, m.definitions)[0] != m.recursiveDecls[0] || !testUserDefinitions(t, m.definitions)[0].defined || m.recursiveDecls[1].defined {
		t.Fatal("recursive declaration lifecycle")
	}
	f := testUserDefinitions(t, m.definitions)[0]
	ite, ok := f.body.(*opApplNode)
	if !ok {
		t.Fatal("body")
	}
	call, ok := ite.operands[2].(*opApplNode)
	if !ok || call.operator != f {
		t.Fatal("recursive identity")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4294 || msgs[1].Code.Value != 4291 {
		t.Fatalf("recursive diagnostics: %#v", msgs)
	}
	out := projectGraph(g, m)
	if out.Truncated {
		t.Fatal("projection")
	}
}
func TestGeneratorRecursiveLetScope(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nA == LET RECURSIVE F(_)\n         F(x) == F(x)\n     IN F(1)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	let, ok := testUserDefinitions(t, m.definitions)[1].body.(*letInNode)
	if !ok || len(testUserDefinitions(t, let.definitions)) != 1 || !testUserDefinitions(t, let.definitions)[0].inRecursive || testUserDefinitions(t, let.definitions)[0].letInLevel != 1 {
		t.Fatal("recursive LET")
	}
}

func TestGeneratorMutualRecursionAndArityBoundary(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nRECURSIVE F(_), G(_)\nF(x) == G(x)\nG(x) == F(x)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	f, h := testUserDefinitions(t, m.definitions)[0], testUserDefinitions(t, m.definitions)[1]
	a := f.body.(*opApplNode)
	b := h.body.(*opApplNode)
	if a.operator != h || b.operator != f || g.unresolvedSum != 0 {
		t.Fatal("mutual binding")
	}
	tree = parsedModule(t, "---- MODULE N ----\nRECURSIVE F(_, _)\nF(x) == x\n====\n")
	g, _ = newGenerator(stdcontext.Background(), 0)
	_, e = g.module(tree)
	msgs := g.table.errors.Messages()
	if e != nil || len(msgs) != 1 || msgs[0].Code.Value != 4292 {
		t.Fatal("arity mismatch", e, msgs)
	}
}
func TestGeneratorRecursiveLetCounterReset(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nA == LET RECURSIVE F(_) IN 1\nB == LET RECURSIVE G(_) G(x) == G(x) IN 1\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4291 || g.unresolvedSum != 0 {
		t.Fatal("recursive scope counters", msgs)
	}
}

func TestGeneratorRecursiveMismatchContinues(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nRECURSIVE F(_, _)\nF(x) == F(x, x)\nA == 1\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4292 || msgs[1].Code.Value != 4004 || len(testUserDefinitions(t, m.definitions)) != 2 {
		t.Fatal("mismatch continuation", msgs)
	}
}

func TestGeneratorRecursiveWrongLetLevel(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nRECURSIVE F(_)\nA == LET F(x) == x IN F(1)\nF(x) == x\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4293 || msgs[1].Code.Value != 4801 || !m.recursiveDecls[0].defined {
		t.Fatal("wrong level", msgs)
	}
}

func TestGeneratorFunctionDefinitionsAndRecursion(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nf[x \\in {1}] == x\ng[x \\in {1}] == g[x]\nA == f[1, 2]\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(testUserDefinitions(t, m.definitions)) != 3 {
		t.Fatal("definitions")
	}
	f := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	if f.operator.header().name.text != "$NonRecursiveFcnSpec" || f.unbounded != nil || len(f.bounded) != 1 {
		t.Fatal("nonrecursive function")
	}
	body := f.operands[0].(*opApplNode)
	if body.operator != f.bounded[0][0] {
		t.Fatal("function formal")
	}
	recursive := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	if recursive.operator.header().name.text != "$RecursiveFcnSpec" || len(recursive.unbounded) != 1 {
		t.Fatal("recursive function")
	}
	application := recursive.operands[0].(*opApplNode)
	receiver := application.operands[0].(*opApplNode)
	if receiver.operator != recursive.unbounded[0] {
		t.Fatal("recursive self binding")
	}
	if testUserDefinitions(t, m.definitions)[2].body != g.nullOAN || len(g.table.errors.Messages()) != 1 || g.table.errors.Messages()[0].Code.Value != 4260 {
		t.Fatal("function arity diagnostic")
	}
}
func TestGeneratorDeclaredRecursiveFunction(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nRECURSIVE f\nf[x \\in {1}] == f[x]\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(testUserDefinitions(t, m.definitions)) != 1 || testUserDefinitions(t, m.definitions)[0] != m.recursiveDecls[0] || !testUserDefinitions(t, m.definitions)[0].defined {
		t.Fatal("declared recursive function")
	}
	spec := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	if spec.operator.header().name.text != "$RecursiveFcnSpec" || len(g.table.errors.Messages()) != 0 {
		t.Fatal("function recursion")
	}
}

func TestGeneratorFunctionDefinitionErrors(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nRECURSIVE f(_)\nf[x \\in {1}] == x\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4292 || msgs[1].Code.Value != 4291 || len(testUserDefinitions(t, m.definitions)) != 1 || m.recursiveDecls[0].defined {
		t.Fatal("recursive function arity", msgs)
	}
	tree = parsedModule(t, "---- MODULE N ----\nf[x \\in {1}] == x\nf[x \\in {1}] == x\n====\n")
	g, _ = newGenerator(stdcontext.Background(), 0)
	m, e = g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs = g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4201 || len(testUserDefinitions(t, m.definitions)) != 2 || testUserDefinitions(t, m.definitions)[0] != testUserDefinitions(t, m.definitions)[1] {
		t.Fatal("duplicate function", msgs)
	}
}
func TestGeneratorLetFunctionDefinition(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nA == LET f[x \\in {1}] == x IN f[1]\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(testUserDefinitions(t, m.definitions)) != 2 {
		t.Fatal("definition order")
	}
	let, ok := testUserDefinitions(t, m.definitions)[1].body.(*letInNode)
	if !ok || len(testUserDefinitions(t, let.definitions)) != 1 || testUserDefinitions(t, let.definitions)[0] != testUserDefinitions(t, m.definitions)[0] {
		t.Fatal("LET function")
	}
}

func TestGeneratorExceptAndAtReferences(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT f\nA == [f EXCEPT ![1] = @, !.a = @]\nB == @\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	outer, ok := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	if !ok || outer.operator.header().name.text != "$Except" || len(outer.operands) != 3 {
		t.Fatal("EXCEPT")
	}
	for _, operand := range outer.operands[1:] {
		pair, ok := operand.(*opApplNode)
		if !ok {
			t.Fatal("pair")
		}
		at, ok := pair.operands[1].(*atNode)
		if !ok || at.except != outer || at.component != pair || at.tree != pair.tree {
			t.Fatal("@ references")
		}
	}
	if testUserDefinitions(t, m.definitions)[1].body != g.nullOAN || len(g.table.errors.Messages()) != 1 || g.table.errors.Messages()[0].Code.Value != 4261 {
		t.Fatal("unbound @")
	}
	out := projectGraph(g, m)
	if out.Truncated {
		t.Fatal("cyclic graph")
	}
}
func TestGeneratorNestedExceptAtContext(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT f\nA == [f EXCEPT ![1] = [@ EXCEPT ![2] = @]]\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	outer := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	outerPair := outer.operands[1].(*opApplNode)
	inner := outerPair.operands[1].(*opApplNode)
	base := inner.operands[0].(*atNode)
	innerPair := inner.operands[1].(*opApplNode)
	rhs := innerPair.operands[1].(*atNode)
	if base.except != outer || base.component != outerPair || rhs.except != inner || rhs.component != innerPair {
		t.Fatal("nested @ context")
	}
}

func TestGeneratorExceptMultiIndexAndComponents(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT f\nA == [f EXCEPT ![1, 2].a[3] = 4]\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	outer := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	pair := outer.operands[1].(*opApplNode)
	seq := pair.operands[0].(*opApplNode)
	if len(seq.operands) != 3 {
		t.Fatal("components")
	}
	tuple, ok := seq.operands[0].(*opApplNode)
	if !ok || tuple.operator.header().name.text != "$Tuple" || len(tuple.operands) != 2 {
		t.Fatal("multi-index")
	}
	field, ok := seq.operands[1].(*stringNode)
	if !ok || field.value.text != "a" {
		t.Fatal("field")
	}
}

func TestGeneratorExpressionLabels(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nA == outer:: (inner:: 1)\nQ == \\A x \\in {1}: mark(x):: x = x\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	a := testUserDefinitions(t, m.definitions)[0]
	outer, ok := a.body.(*labelNode)
	if !ok || a.labels.get(g.pool.intern("outer")) != outer {
		t.Fatal("outer label")
	}
	inner, ok := outer.body.(*labelNode)
	if !ok || outer.labels.get(g.pool.intern("inner")) != inner {
		t.Fatal("nested label")
	}
	q := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	mark, ok := q.operands[0].(*labelNode)
	if !ok || len(mark.params) != 1 || mark.params[0] != q.bounded[0][0] || len(g.table.errors.Messages()) != 0 {
		t.Fatal("quantified label")
	}
	out := projectGraph(g, m)
	if out.Truncated {
		t.Fatal("label projection")
	}
}
func TestGeneratorLabelDiagnostics(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nQ == \\A x \\in {1}: mark:: x = x\nA == <<dup:: 1, dup:: 2>>\nCONSTANT f\nB == [f EXCEPT ![1] = bad:: 2]\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 3 || msgs[0].Code.Value != 4331 || msgs[1].Code.Value != 4336 || msgs[2].Code.Value != 4335 {
		t.Fatalf("label diagnostics: %#v", msgs)
	}
}

func TestGeneratorLabelParameterErrors(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT C\nQ == \\A x \\in {1}: mark(x, x):: x = x\nA == other(C):: 1\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 3 || msgs[0].Code.Value != 4330 || msgs[1].Code.Value != 4332 || msgs[2].Code.Value != 4332 {
		t.Fatalf("label parameters: %#v", msgs)
	}
}

func TestGeneratorSameNameLabelResiduals(t *testing.T) {
	tests := []struct {
		name, definition string
		codes            []int
		extra            string
	}{
		{"unresolved identities", "F == L(x,x)::TRUE", []int{4332, 4332, 4332}, "Label L declares extra parameter(s)  x  x  "},
		{"resolved repetition", "F(x) == L(x,x)::TRUE", []int{4330, 4332}, "Label L declares extra parameter(s)  x  "},
		{"removal and ordering", "F == \\A x, y: L(x,x,z,z)::TRUE", []int{4332, 4332, 4330, 4331, 4332}, "Label L declares extra parameter(s)  z  z  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, _ := newGenerator(stdcontext.Background(), 0)
			m, err := g.module(parsedModule(t, "---- MODULE M ----\n"+tt.definition+"\n====\n"))
			if err != nil {
				t.Fatal(err)
			}
			msgs := g.table.errors.Messages()
			if len(msgs) != len(tt.codes) {
				t.Fatalf("diagnostics: %#v", msgs)
			}
			for i, code := range tt.codes {
				if msgs[i].Code.Value != code {
					t.Fatalf("diagnostic %d: %#v", i, msgs[i])
				}
			}
			last := msgs[len(msgs)-1]
			if len(last.Parameters) != 1 || last.Parameters[0].Text != tt.extra || !last.Parameters[0].UTF16Present || last.Parameters[0].TextUnrepresentable {
				t.Fatalf("extra diagnostic: %#v", last)
			}
			if tt.name == "unresolved identities" {
				label := testUserDefinitions(t, m.definitions)[0].body.(*labelNode)
				if label.params[0] == label.params[1] {
					t.Fatal("dummy formals merged")
				}
			}
		})
	}
}

// A direct generator has no Java cursor, so distinct residual names print in
// the stable fallback order (label_extra_order.go).
func TestGeneratorDistinctNameLabelResidualsFallback(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, err := g.module(parsedModule(t, "---- MODULE M ----\nF(b,a) == L(b,a)::TRUE\n====\n"))
	msgs := g.table.errors.Messages()
	if err != nil || g.labelOrderFallbacks != 1 || len(msgs) != 1 || msgs[0].Parameters[0].Text != "Label L declares extra parameter(s)  a  b  " {
		t.Fatalf("distinct residual names: %v %+v", err, msgs)
	}
}

func TestGeneratorSimpleLabelSelectors(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nA == outer:: (inner:: 1)\nB == A!outer!inner\nC == A!missing\nQ == \\A x \\in {1}: mark(x):: x = x\nD == Q!mark\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	outer := testUserDefinitions(t, m.definitions)[0].body.(*labelNode)
	inner := outer.body.(*labelNode)
	selected, ok := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	if !ok || selected.operator.header().name.text != "$Nop" || selected.operands[0] != inner || selected.subExpressionOf != testUserDefinitions(t, m.definitions)[0] {
		t.Fatal("label selection")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4004 || msgs[1].Code.Value != 4337 {
		t.Fatal("selector diagnostics", msgs)
	}
}

func TestGeneratorParameterizedDefinitionLabelSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nA(x) == mark:: x\nB == A(1)!mark\nC == A!mark\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	root := testUserDefinitions(t, m.definitions)[0]
	label := root.labels.get(g.pool.intern("mark"))
	app, ok := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	if !ok || app.subExpressionOf != root || len(app.operands) != 1 {
		t.Fatal("selected application")
	}
	lambda, ok := app.operator.(*userOpDefNode)
	if !ok || lambda.name.text != "LAMBDA" || lambda.body != label || lambda.params[0] != root.params[0] {
		t.Fatal("selector lambda")
	}
	if testUserDefinitions(t, m.definitions)[2].body != g.nullOAN || len(g.table.errors.Messages()) != 1 || g.table.errors.Messages()[0].Code.Value != 4204 {
		t.Fatal("selector arity")
	}
}

func TestGeneratorParameterizedLabelSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nS(x) == \\A y \\in {1}: mark(y):: x = y\nA == S(1)!mark(2)\nB == S(1)!mark(2, 3)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	root := testUserDefinitions(t, m.definitions)[0]
	mark := root.labels.get(g.pool.intern("mark"))
	app, ok := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	if !ok || len(app.operands) != 2 || app.subExpressionOf != root {
		t.Fatal("selected application")
	}
	lambda, ok := app.operator.(*userOpDefNode)
	if !ok || lambda.body != mark || len(lambda.params) != 2 || lambda.params[0] != root.params[0] || lambda.params[1] != mark.params[0] {
		t.Fatal("selected formals")
	}
	if len(g.table.errors.Messages()) != 1 || g.table.errors.Messages()[0].Code.Value != 4337 {
		t.Fatal("label arity")
	}
}

func TestGeneratorNestedParameterizedLabels(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nQ == \\A x \\in {1}: first(x):: \\A y \\in {2}: second(y):: x = y\nA == Q!first(1)!second(2)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	root := testUserDefinitions(t, m.definitions)[0]
	first := root.labels.get(g.pool.intern("first"))
	second := first.labels.get(g.pool.intern("second"))
	app := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	if len(lambda.params) != 2 || lambda.params[0] != first.params[0] || lambda.params[1] != second.params[0] || lambda.body != second || len(app.operands) != 2 || len(g.table.errors.Messages()) != 0 {
		t.Fatal("nested selected bindings")
	}
}

func TestGeneratorUnresolvedNotationNullBody(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nA == 1 + 2\nB == 1\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(testUserDefinitions(t, m.definitions)) != 2 || testUserDefinitions(t, m.definitions)[0].body != nil || len(msgs) != 2 || msgs[0].Code.Value != 4004 || msgs[1].Code.Value != 4004 {
		t.Fatal("notation recovery", msgs)
	}
}

func TestGeneratorNotationOperatorArguments(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nApply(Op(_,_)) == Op(1, 2)\nA == Apply(\\cup)\nB == Apply(\\oplus)\nUnary(Op(_)) == Op(1)\nC == Unary(\\cup)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	app := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	arg, ok := app.operands[0].(*opArgNode)
	if !ok || arg.arity != 2 || arg.operator.header().name.text != "\\union" || arg.module != m {
		t.Fatal("notation operator binding")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 4 || msgs[0].Code.Value != 4004 || msgs[1].Code.Value != 4271 || msgs[2].Code.Value != 4004 || msgs[3].Code.Value != 4271 {
		t.Fatalf("diagnostic order: %#v", msgs)
	}
	if _, e := msgs[2].Render(); e != nil {
		t.Fatal(e)
	}
	if p := msgs[2].Parameters[3]; p.Kind != SemanticNodeParameter || !p.UTF16Present || p.TextUnrepresentable {
		t.Fatal("operator location provenance", p)
	}
}

func TestGeneratorLambdaOperatorArgument(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nApply(Op(_,_)) == Op(1, 2)\nA == Apply(LAMBDA x, y : x)\nB == Apply(LAMBDA z : z)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	app := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	arg := app.operands[0].(*opArgNode)
	def := arg.operator.(*userOpDefNode)
	body := def.body.(*opApplNode)
	if def.name.text != "LAMBDA" || arg.arity != 2 || body.operator != def.params[0] || arg.module != m {
		t.Fatal("lambda binding")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4274 || msgs[1].Code.Value != 4271 {
		t.Fatal(msgs)
	}
	if msgs[0].Location.BeginLine != 4 || msgs[0].Location.BeginColumn != 6 || msgs[0].Location.EndColumn != 10 {
		t.Fatal("lambda diagnostic operator span", msgs[0].Location)
	}
}

func TestGeneratorRecursiveNotation(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nRECURSIVE _\\oplus_\nx \\oplus y == IF x = 0 THEN y ELSE x \\oplus y\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(testUserDefinitions(t, m.definitions)) != 1 || len(m.recursiveDecls) != 1 || testUserDefinitions(t, m.definitions)[0] != m.recursiveDecls[0] || !testUserDefinitions(t, m.definitions)[0].defined || testUserDefinitions(t, m.definitions)[0].arity != 2 {
		t.Fatal("recursive notation identity")
	}
	if len(g.table.errors.Messages()) != 0 {
		t.Fatal(g.table.errors.Messages())
	}
}

func TestGeneratorRecursiveUnaryNotationDeclarations(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nRECURSIVE -. _, _ ^+\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.recursiveDecls) != 2 || m.recursiveDecls[0].arity != 1 || m.recursiveDecls[1].arity != 1 {
		t.Fatal("unary recursive declarations")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4291 || msgs[1].Code.Value != 4291 {
		t.Fatal(msgs)
	}
}

func TestGeneratorHigherOrderLabelSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nApply(Op(_)) == mark:: Op(1)\nF(x) == x\nA == Apply(F)!mark\nB == Apply(1)!mark\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	root := testUserDefinitions(t, m.definitions)[0]
	app := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	arg := app.operands[0].(*opArgNode)
	if app.subExpressionOf != root || lambda.body != root.labels.get(g.pool.intern("mark")) || lambda.params[0] != root.params[0] || arg.operator != testUserDefinitions(t, m.definitions)[1] {
		t.Fatal("higher-order selection")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4270 || msgs[1].Code.Value != 4271 {
		t.Fatal(msgs)
	}
}

func TestGeneratorLambdaWhereExpressionRequired(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nApply(x) == x\nA == Apply(LAMBDA x : x)\nB == 1\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(testUserDefinitions(t, m.definitions)) != 3 || len(msgs) != 1 || msgs[0].Code.Value != 4275 {
		t.Fatal("lambda expression recovery", msgs)
	}
	app := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	if len(app.operands) != 1 || app.operands[0] != nil {
		t.Fatal("null expression argument")
	}
}

func TestGeneratorLabelAsOperatorArgument(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(x) == mark:: x\nApply(Op(_)) == Op(1)\nA == Apply(F!mark)\nB == Apply(F!missing)\nApply2(Op(_,_)) == Op(1,2)\nC == Apply2(F!mark)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	app := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	arg := app.operands[0].(*opArgNode)
	lambda := arg.operator.(*userOpDefNode)
	if lambda.body != testUserDefinitions(t, m.definitions)[0].labels.get(g.pool.intern("mark")) || lambda.params[0] != testUserDefinitions(t, m.definitions)[0].params[0] {
		t.Fatal("selected operator binding")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 4 || msgs[0].Code.Value != 4004 || msgs[1].Code.Value != 4271 || msgs[2].Code.Value != 4271 || msgs[3].Code.Value != 4271 {
		t.Fatal(msgs)
	}
}

func TestGeneratorNestedLabelAsOperatorArgument(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nQ == \\A x \\in {1}: first(x):: \\A y \\in {2}: second(y):: x = y\nApply(Op(_,_)) == Op(1,2)\nA == Apply(Q!first!second)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	first := testUserDefinitions(t, m.definitions)[0].labels.get(g.pool.intern("first"))
	second := first.labels.get(g.pool.intern("second"))
	app := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	lambda := app.operands[0].(*opArgNode).operator.(*userOpDefNode)
	if lambda.body != second || len(lambda.params) != 2 || lambda.params[0] != first.params[0] || lambda.params[1] != second.params[0] || len(g.table.errors.Messages()) != 0 {
		t.Fatal("nested selected operator formals")
	}
}

func TestGeneratorNullDefinitionStructuralSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == 1 + 2\nA == F!1!1\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) < 2 || msgs[len(msgs)-1].Code.Value != 4005 || msgs[len(msgs)-1].Format != "Subexpression selection failed, probably due to an error in the selected expression." || testUserDefinitions(t, m.definitions)[1].body != g.nullOAN {
		t.Fatalf("diagnostics: %#v", msgs)
	}
}

func TestGeneratorNumericSubexpression(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(x) == <<x, 2>>\nA == F(1)!1\nB == F(1)!3\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	root := testUserDefinitions(t, m.definitions)[0]
	tuple := root.body.(*opApplNode)
	app := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	if lambda.body != tuple.operands[0] || lambda.params[0] != root.params[0] || app.subExpressionOf != root {
		t.Fatal("numeric selector binding")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4005 || testUserDefinitions(t, m.definitions)[2].body != g.nullOAN {
		t.Fatal("numeric selector error", msgs)
	}
}

func TestGeneratorUnknownSelectedOperatorReportsCompoundName(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nApply(Op(_)) == Op(1)\nA == Apply(Missing!label)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, e := g.module(tree)
	msgs := g.table.errors.Messages()
	if e != nil || len(msgs) != 2 || msgs[0].Code.Value != 4200 || msgs[0].Parameters[0].Text != "Missing!label" || msgs[1].Code.Value != 4271 {
		t.Fatalf("compound lookup diagnostics: %v %#v", e, msgs)
	}
}

func TestGeneratorNestedNumericSubexpressions(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(x) == << <<x, 2>>, 3 >>\nA == F(1)!1!2\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	outer := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	inner := outer.operands[0].(*opApplNode)
	app := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	if lambda.body != inner.operands[1] || app.subExpressionOf != testUserDefinitions(t, m.definitions)[0] || len(g.table.errors.Messages()) != 0 {
		t.Fatal("nested numeric selection")
	}
}

func TestGeneratorNumericSelectionLetAndLiteral(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == LET X == 1 IN X\nA == F!1\nB == F!2\nG == 2\nC == G!1\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	let := testUserDefinitions(t, m.definitions)[1].body.(*letInNode)
	app := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	if app.operands[0] != let.body || app.subExpressionOf != testUserDefinitions(t, m.definitions)[1] {
		t.Fatal("LET body selection")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4005 || msgs[1].Code.Value != 4005 || testUserDefinitions(t, m.definitions)[3].body != g.nullOAN || testUserDefinitions(t, m.definitions)[5].body != g.nullOAN {
		t.Fatal("selection diagnostics", msgs)
	}
}

func TestGeneratorNumericSelectionBoundRange(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == \\A x \\in {1}: x = 1\nA == F!1\nB == F!2\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	quantifier := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	app := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	if app.operands[0] != quantifier.ranges[0] || app.subExpressionOf != testUserDefinitions(t, m.definitions)[0] {
		t.Fatal("bound range selection")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4005 || testUserDefinitions(t, m.definitions)[2].body != g.nullOAN {
		t.Fatal("bound selector error", msgs)
	}
}

func TestGeneratorStructuralSelectorsSpecialOperators(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT f\nR == [a |-> 1, b |-> 2]\nRA == R!2\nC == CASE TRUE -> 1 [] OTHER -> 2\nCA == C!2!2\nCB == C!2!1\nCC == C!1\nE == [f EXCEPT ![1] = 2]\nEA == E!1\nEB == E!2\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	record := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	recordPair := record.operands[1].(*opApplNode)
	if testUserDefinitions(t, m.definitions)[1].body.(*opApplNode).operands[0] != recordPair.operands[1] {
		t.Fatal("record field value")
	}
	caseExpr := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	other := caseExpr.operands[1].(*opApplNode)
	if testUserDefinitions(t, m.definitions)[3].body.(*opApplNode).operands[0] != other.operands[1] {
		t.Fatal("CASE value")
	}
	except := testUserDefinitions(t, m.definitions)[6].body.(*opApplNode)
	if testUserDefinitions(t, m.definitions)[7].body.(*opApplNode).operands[0] != except.operands[0] {
		t.Fatal("EXCEPT base")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 3 || msgs[0].Code.Value != 4005 || msgs[1].Code.Value != 4005 || msgs[2].Code.Value != 4005 {
		t.Fatal("special selector errors", msgs)
	}
}

func TestGeneratorFirstAndLastSelectors(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == <<1, 2>>\nA == F!<<\nB == F!>>\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	tuple := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	if testUserDefinitions(t, m.definitions)[1].body.(*opApplNode).operands[0] != tuple.operands[0] || testUserDefinitions(t, m.definitions)[2].body.(*opApplNode).operands[0] != tuple.operands[1] {
		t.Fatal("first/last selection")
	}
	if len(g.table.errors.Messages()) != 0 {
		t.Fatal(g.table.errors.Messages())
	}
}

func TestGeneratorLabelThenStructuralSelection(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == \\A x \\in {1}: mark(x):: <<x, 2>>\nA == F!mark(1)!2\nB == F!mark(1)!3\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	root := testUserDefinitions(t, m.definitions)[0]
	label := root.labels.get(g.pool.intern("mark"))
	tuple := label.body.(*opApplNode)
	app := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	if lambda.body != tuple.operands[1] || len(lambda.params) != 1 || lambda.params[0] != label.params[0] || app.subExpressionOf != root {
		t.Fatal("mixed selector binding")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4005 || testUserDefinitions(t, m.definitions)[2].body != g.nullOAN {
		t.Fatal("mixed selector error", msgs)
	}
}

func TestGeneratorStructuralSelectorHigherOrderRoot(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(Op(_)) == <<Op(1), 2>>\nId(x) == x\nA == F(Id)!1\nB == F(1)!1\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	root := testUserDefinitions(t, m.definitions)[0]
	app := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	if lambda.params[0] != root.params[0] || app.operands[0].(*opArgNode).operator != testUserDefinitions(t, m.definitions)[1] || app.subExpressionOf != root {
		t.Fatal("higher-order structural selector")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4270 || msgs[1].Code.Value != 4271 {
		t.Fatal("wrapper match order", msgs)
	}
}

func TestGeneratorQuantifierArgumentSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == \\A x \\in {1}: x = 1\nA == F!(1)\nB == F!(1, 2)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	quantifier := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	app := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	if len(lambda.params) != 1 || lambda.params[0] != quantifier.bounded[0][0] || lambda.body != quantifier.operands[0] || len(app.operands) != 1 || app.subExpressionOf != testUserDefinitions(t, m.definitions)[0] {
		t.Fatal("quantifier argument selector")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4005 || testUserDefinitions(t, m.definitions)[2].body != g.nullOAN {
		t.Fatal("quantifier selector arity", msgs)
	}
}

func TestGeneratorBoundBodyAsOperatorArgument(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == \\A x \\in {1}: x = 1\nApply(Op(_)) == Op(1)\nA == Apply(F!@)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	quantifier := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	app := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	lambda := app.operands[0].(*opArgNode).operator.(*userOpDefNode)
	if len(lambda.params) != 1 || lambda.params[0] != quantifier.bounded[0][0] || lambda.body != quantifier.operands[0] || len(g.table.errors.Messages()) != 0 {
		t.Fatal("bound body selection")
	}
}

func TestGeneratorQuantifierAtSelectorWhereExpressionRequired(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == \\A x \\in {1}: x = 1\nA == F!@\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4004 || testUserDefinitions(t, m.definitions)[1].body != g.nullOAN {
		t.Fatal("missing selector arguments", msgs)
	}
}

func TestGeneratorUnboundedQuantifierSelectors(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == \\A x : x = 1\nApply(Op(_)) == Op(1)\nA == F!(2)\nB == Apply(F!@)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	quantifier := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	expr := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode).operator.(*userOpDefNode)
	op := testUserDefinitions(t, m.definitions)[3].body.(*opApplNode).operands[0].(*opArgNode).operator.(*userOpDefNode)
	if len(quantifier.unbounded) != 1 || expr.params[0] != quantifier.unbounded[0] || op.params[0] != quantifier.unbounded[0] || expr.body != quantifier.operands[0] || op.body != quantifier.operands[0] || len(g.table.errors.Messages()) != 0 {
		t.Fatal("unbounded selectors")
	}
}

func TestGeneratorInvalidBodySelectors(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == <<1,2>>\nA == F!(1)\nB == F!@\nL == LET X == 1 IN X\nC == L!(1)\nQ == \\A x : x = 1\nD == Q!1\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 4 {
		t.Fatal(msgs)
	}
	for _, msg := range msgs {
		if msg.Code.Value != 4005 {
			t.Fatal(msgs)
		}
	}
	text, e := msgs[0].Render()
	if e != nil || text != "Nonexistent operand specified by `!(...)'." {
		t.Fatal(text, e)
	}
	text, e = msgs[1].Render()
	if e != nil || text != "Nonexistent operand specified by `@'." {
		t.Fatal(text, e)
	}
}

func TestGeneratorStructuralSelectionAsOperatorArgument(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(x) == <<x, 2>>\nApply(Op(_)) == Op(1)\nA == Apply(F!1)\nB == Apply(F!3)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	root := testUserDefinitions(t, m.definitions)[0]
	tuple := root.body.(*opApplNode)
	app := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	lambda := app.operands[0].(*opArgNode).operator.(*userOpDefNode)
	if lambda.body != tuple.operands[0] || len(lambda.params) != 1 || lambda.params[0] != root.params[0] {
		t.Fatal("structural operator selection")
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4005 || msgs[1].Code.Value != 4271 {
		t.Fatal("structural operator errors", msgs)
	}
}

func TestGeneratorMixedSelectionAsOperatorArgument(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nQ == \\A x \\in {1}: mark(x):: <<x, 2>>\nApply(Op(_)) == Op(1)\nA == Apply(Q!mark!2)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	label := testUserDefinitions(t, m.definitions)[0].labels.get(g.pool.intern("mark"))
	tuple := label.body.(*opApplNode)
	app := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	lambda := app.operands[0].(*opArgNode).operator.(*userOpDefNode)
	if lambda.body != tuple.operands[1] || len(lambda.params) != 1 || lambda.params[0] != label.params[0] || len(g.table.errors.Messages()) != 0 {
		t.Fatal("mixed operator selection")
	}
}

func TestGeneratorColonSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(x) == x\nApply(Op(_)) == Op(1)\nA == F(2)!:\nB == Apply(F!:)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	root := testUserDefinitions(t, m.definitions)[0]
	expr := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode).operator.(*userOpDefNode)
	op := testUserDefinitions(t, m.definitions)[3].body.(*opApplNode).operands[0].(*opArgNode).operator.(*userOpDefNode)
	if expr.body != root.body || op.body != root.body || expr.params[0] != root.params[0] || op.params[0] != root.params[0] || len(g.table.errors.Messages()) != 0 {
		t.Fatal("colon selector")
	}
}

func TestGeneratorMixedColonErrors(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nG(x) == <<x, 2>>\nA == G(1)!1!:\nB == G(1)!:!1\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4005 || msgs[1].Code.Value != 4005 || testUserDefinitions(t, m.definitions)[1].body != g.nullOAN || testUserDefinitions(t, m.definitions)[2].body != g.nullOAN {
		t.Fatal("colon errors", msgs)
	}
	text, e := msgs[0].Render()
	if e != nil || text != "!: should not follow an operand selector." {
		t.Fatal(text, e)
	}
}

// The recursive declaration count includes duplicates even when insertion
// retains the original binding. Undefined diagnostics follow declaration order.
func TestGeneratorRecursiveDuplicateDeclarationRecovery(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nRECURSIVE F(_), F(_), U\nF(x) == x\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 3 || msgs[0].Code.Value != 4801 || msgs[1].Code.Value != 4291 || msgs[2].Code.Value != 4291 {
		t.Fatalf("duplicate diagnostics: %#v", msgs)
	}
	if len(m.recursiveDecls) != 3 || len(testUserDefinitions(t, m.definitions)) != 1 || testUserDefinitions(t, m.definitions)[0] != m.recursiveDecls[0] || m.recursiveDecls[1].defined || m.recursiveDecls[2].defined || g.unresolvedSum != 0 {
		t.Fatal("duplicate declaration recovery")
	}
}

// Nested unresolved declarations share a section; after leaving the LET,
// its counter is removed without consuming the outer declaration.
func TestGeneratorRecursiveNestedSectionCounters(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nRECURSIVE F(_)\nA == LET RECURSIVE G(_) IN 1\nF(x) == x\nRECURSIVE H\nH == 1\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4291 {
		t.Fatalf("nested diagnostics: %#v", msgs)
	}
	if len(m.recursiveDecls) != 3 {
		t.Fatal("declaration count")
	}
	f, inner, h := m.recursiveDecls[0], m.recursiveDecls[1], m.recursiveDecls[2]
	if f.recursiveSection != inner.recursiveSection || h.recursiveSection != f.recursiveSection+1 || inner.letInLevel != 1 || !f.defined || inner.defined || !h.defined || g.unresolvedSum != 0 {
		t.Fatal("nested recursive sections")
	}
}

// processOperator's Java loop assigns paramsMatch on each iteration. The
// last formal therefore controls equal-count higher-order mismatch reporting.
func TestGeneratorRecursiveFormalArityOrdering(t *testing.T) {
	cases := []struct {
		source       string
		wantMismatch bool
	}{
		{"F(G(_), x) == x", false},
		{"F(x, G(_)) == x", true},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\nRECURSIVE F(_, _)\n"+tc.source+"\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			m, e := g.module(tree)
			if e != nil {
				t.Fatal(e)
			}
			msgs := g.table.errors.Messages()
			if tc.wantMismatch {
				if len(msgs) != 1 || msgs[0].Code.Value != 4292 {
					t.Fatalf("diagnostics: %#v", msgs)
				}
			} else if len(msgs) != 0 {
				t.Fatalf("diagnostics: %#v", msgs)
			}
			if len(testUserDefinitions(t, m.definitions)) != 1 || testUserDefinitions(t, m.definitions)[0] != m.recursiveDecls[0] || !testUserDefinitions(t, m.definitions)[0].defined || g.unresolvedSum != 0 {
				t.Fatal("recursive continuation")
			}
		})
	}
}

func TestGeneratorRecursiveDuplicateDifferentArity(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nRECURSIVE F(_), F(_, _)\nF(x) == x\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4201 || msgs[1].Code.Value != 4291 {
		t.Fatalf("diagnostics: %#v", msgs)
	}
	if len(m.recursiveDecls) != 2 || testUserDefinitions(t, m.definitions)[0] != m.recursiveDecls[0] || m.recursiveDecls[1].defined || g.unresolvedSum != 0 {
		t.Fatal("duplicate recovery")
	}
}

func TestGeneratorLetContextNameSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == LET G == 1 IN G\nA == F!:!G\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	local, root := testUserDefinitions(t, m.definitions)[0], testUserDefinitions(t, m.definitions)[1]
	app := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	if app.operator != local || app.subExpressionOf != root || len(app.operands) != 0 || len(g.table.errors.Messages()) != 0 {
		t.Fatal("LET name selector")
	}
}

func TestGeneratorLetContextOperatorArguments(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == LET G(x) == x IN G(1)\nA == F!:!G(2)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	local, root := testUserDefinitions(t, m.definitions)[0], testUserDefinitions(t, m.definitions)[1]
	app := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	if app.operator != local || app.subExpressionOf != root || len(app.operands) != 1 || len(g.table.errors.Messages()) != 0 {
		t.Fatal("LET operator arguments")
	}
}

func TestGeneratorLetContextOperatorArity(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == LET G(x) == x IN G(1)\nA == F!:!G\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4204 {
		t.Fatalf("LET operator arity diagnostics: %#v", msgs)
	}
}

func TestGeneratorLetContextSelectorErrors(t *testing.T) {
	for _, tc := range []struct {
		body string
		code int
	}{
		{"1", 4005},
		{"outer:: 1", 4005},
		{"LET G == 1 IN G", 4200},
		{"outer:: LET G == 1 IN G", 4200},
	} {
		t.Run(tc.body, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\nF == "+tc.body+"\nA == F!:!Missing\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			_, e := g.module(tree)
			if e != nil {
				t.Fatal(e)
			}
			msgs := g.table.errors.Messages()
			if len(msgs) != 1 || msgs[0].Code.Value != tc.code {
				t.Fatalf("selector diagnostics: %#v", msgs)
			}
		})
	}
}

func TestGeneratorLetContextHigherOrderArgument(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == LET G(Op(_)) == Op(1) IN G(LAMBDA x : x)\nA == F!:!G(LAMBDA y : y)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	app, ok := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	if !ok || app.operator != testUserDefinitions(t, m.definitions)[0] || len(app.operands) != 1 {
		t.Fatal("LET higher-order selection")
	}
	arg, ok := app.operands[0].(*opArgNode)
	if !ok || arg.arity != 1 || len(g.table.errors.Messages()) != 0 {
		t.Fatal("LET higher-order argument")
	}
}

func TestGeneratorParameterizedLetContextSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(x) == LET G(y) == x IN G(x)\nA == F(1)!:!G(2)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	local, root := testUserDefinitions(t, m.definitions)[0], testUserDefinitions(t, m.definitions)[1]
	selected := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	lambda, ok := selected.operator.(*userOpDefNode)
	if !ok || lambda.name.text != "LAMBDA" || len(lambda.params) != 2 || len(selected.operands) != 2 || selected.subExpressionOf != root {
		t.Fatal("selector wrapper")
	}
	body, ok := lambda.body.(*opApplNode)
	if !ok || body.operator != local || len(body.operands) != 1 {
		t.Fatal("selected local operator")
	}
	formal, ok := body.operands[0].(*opApplNode)
	if !ok || formal.operator != lambda.params[1] || lambda.params[0] != root.params[0] || lambda.params[1] == local.params[0] || len(g.table.errors.Messages()) != 0 {
		t.Fatal("cloned selector parameter")
	}
}

func TestGeneratorParameterizedLetContextHigherOrderSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(x) == LET G(Op(_)) == Op(x) IN G(LAMBDA a : a)\nA == F(1)!:!G(LAMBDA b : b)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	selected := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	lambda := selected.operator.(*userOpDefNode)
	body := lambda.body.(*opApplNode)
	arg, ok := body.operands[0].(*opArgNode)
	if !ok || arg.operator != lambda.params[1] || arg.arity != 1 || len(g.table.errors.Messages()) != 0 {
		t.Fatal("cloned higher-order selector parameter")
	}
}

func TestGeneratorLabeledLetContextSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == outer:: inner:: LET G == 1 IN G\nA == F!:!G\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	app, ok := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	if !ok || app.operator != testUserDefinitions(t, m.definitions)[0] || app.subExpressionOf != testUserDefinitions(t, m.definitions)[1] || len(g.table.errors.Messages()) != 0 {
		t.Fatal("labeled LET context selection")
	}
}

func TestGeneratorLetContextHigherOrderArityError(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == LET G(Op(_,_)) == Op(1, 2) IN G(LAMBDA x, y : x)\nA == F!:!G(LAMBDA z : z)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4274 || msgs[1].Code.Value != 4271 {
		t.Fatalf("LET higher-order arity diagnostics: %#v", msgs)
	}
}

func TestGeneratorLetContextAsOperatorArgument(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == LET G(x) == x IN G(1)\nApply(Op(_)) == Op(1)\nA == Apply(F!:!G)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	app := testUserDefinitions(t, m.definitions)[3].body.(*opApplNode)
	arg, ok := app.operands[0].(*opArgNode)
	if !ok || arg.operator != testUserDefinitions(t, m.definitions)[0] || arg.arity != 1 || len(g.table.errors.Messages()) != 0 {
		t.Fatal("LET context operator argument")
	}
}

func TestGeneratorLetContextOperatorArgumentArity(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == LET G(x) == x IN G(1)\nApply(Op(_,_)) == Op(1, 2)\nA == Apply(F!:!G)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4271 || msgs[1].Code.Value != 4271 {
		t.Fatalf("LET operator argument diagnostics: %#v", msgs)
	}
}

func TestGeneratorParameterizedLetContextAsOperatorArgument(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(x) == LET G(y) == x IN G(x)\nApply(Op(_,_)) == Op(1, 2)\nA == Apply(F!:!G)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	arg := testUserDefinitions(t, m.definitions)[3].body.(*opApplNode).operands[0].(*opArgNode)
	lambda := arg.operator.(*userOpDefNode)
	body := lambda.body.(*opApplNode)
	formal := body.operands[0].(*opApplNode)
	if arg.arity != 2 || len(lambda.params) != 2 || lambda.params[0] != testUserDefinitions(t, m.definitions)[1].params[0] || lambda.params[1] == testUserDefinitions(t, m.definitions)[0].params[0] || formal.operator != lambda.params[1] || body.operator != testUserDefinitions(t, m.definitions)[0] || len(g.table.errors.Messages()) != 0 {
		t.Fatal("parameterized LET operator argument")
	}
}

func TestGeneratorLetContextOperatorArgumentErrors(t *testing.T) {
	for _, tc := range []struct {
		body string
		code int
	}{
		{"1", 4005},
		{"LET G == 1 IN G", 4200},
	} {
		t.Run(tc.body, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\nF == "+tc.body+"\nApply(Op(_)) == Op(1)\nA == Apply(F!:!Missing)\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			_, e := g.module(tree)
			if e != nil {
				t.Fatal(e)
			}
			msgs := g.table.errors.Messages()
			if len(msgs) != 2 || msgs[0].Code.Value != tc.code || msgs[1].Code.Value != 4271 {
				t.Fatalf("LET operator selection errors: %#v", msgs)
			}
		})
	}
}

func TestGeneratorNestedLetContextSelectors(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == LET G == LET H == 1 IN H IN G\nA == F!:!G!:!H\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	app := testUserDefinitions(t, m.definitions)[3].body.(*opApplNode)
	if app.operator != testUserDefinitions(t, m.definitions)[0] || app.subExpressionOf != testUserDefinitions(t, m.definitions)[2] || len(g.table.errors.Messages()) != 0 {
		t.Fatal("nested LET contexts")
	}
}

func TestGeneratorNestedLetContextOperatorArgument(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == LET G == LET H(x) == x IN H(1) IN G\nApply(Op(_)) == Op(1)\nA == Apply(F!:!G!:!H)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	arg := testUserDefinitions(t, m.definitions)[4].body.(*opApplNode).operands[0].(*opArgNode)
	if arg.operator != testUserDefinitions(t, m.definitions)[0] || arg.arity != 1 || len(g.table.errors.Messages()) != 0 {
		t.Fatal("nested LET operator argument")
	}
}

func TestGeneratorNestedLetContextArguments(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == LET G == LET H(x) == x IN H(1) IN G\nA == F!:!G!:!H(2)\nB == F!:!G!:!H\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	app := testUserDefinitions(t, m.definitions)[3].body.(*opApplNode)
	msgs := g.table.errors.Messages()
	if app.operator != testUserDefinitions(t, m.definitions)[0] || app.subExpressionOf != testUserDefinitions(t, m.definitions)[2] || len(app.operands) != 1 || len(msgs) != 1 || msgs[0].Code.Value != 4204 {
		t.Fatalf("nested LET arguments: %#v", msgs)
	}
}

func TestGeneratorParameterizedNestedLetContexts(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(x) == LET G(y) == LET H(z) == x IN H(y) IN G(x)\nA == F(1)!:!G(2)!:!H(3)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	app := testUserDefinitions(t, m.definitions)[3].body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	body := lambda.body.(*opApplNode)
	formal := body.operands[0].(*opApplNode)
	if app.subExpressionOf != testUserDefinitions(t, m.definitions)[2] || len(app.operands) != 3 || len(lambda.params) != 3 || lambda.params[0] != testUserDefinitions(t, m.definitions)[2].params[0] || lambda.params[1] != testUserDefinitions(t, m.definitions)[1].params[0] || lambda.params[2] == testUserDefinitions(t, m.definitions)[0].params[0] || body.operator != testUserDefinitions(t, m.definitions)[0] || formal.operator != lambda.params[2] || len(g.table.errors.Messages()) != 0 {
		t.Fatal("parameterized nested LET selection")
	}
}

func TestGeneratorParameterizedNestedLetOperatorArgument(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(x) == LET G(y) == LET H(z) == x IN H(y) IN G(x)\nApply(Op(_,_,_)) == Op(1, 2, 3)\nA == Apply(F!:!G!:!H)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	arg := testUserDefinitions(t, m.definitions)[4].body.(*opApplNode).operands[0].(*opArgNode)
	lambda := arg.operator.(*userOpDefNode)
	body := lambda.body.(*opApplNode)
	formal := body.operands[0].(*opApplNode)
	if arg.arity != 3 || len(lambda.params) != 3 || lambda.params[0] != testUserDefinitions(t, m.definitions)[2].params[0] || lambda.params[1] != testUserDefinitions(t, m.definitions)[1].params[0] || lambda.params[2] == testUserDefinitions(t, m.definitions)[0].params[0] || body.operator != testUserDefinitions(t, m.definitions)[0] || formal.operator != lambda.params[2] || len(g.table.errors.Messages()) != 0 {
		t.Fatal("parameterized nested LET operator argument")
	}
}

func TestGeneratorNestedLetContextErrors(t *testing.T) {
	for _, tc := range []struct {
		body string
		code int
	}{
		{"1", 4005},
		{"LET H == 1 IN H", 4200},
	} {
		t.Run(tc.body, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\nF == LET G == "+tc.body+" IN G\nA == F!:!G!:!Missing\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			_, e := g.module(tree)
			if e != nil {
				t.Fatal(e)
			}
			msgs := g.table.errors.Messages()
			if len(msgs) != 1 || msgs[0].Code.Value != tc.code {
				t.Fatalf("nested LET selection diagnostics: %#v", msgs)
			}
		})
	}
}

func TestGeneratorStructuralLetContextSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == <<LET G(x) == x IN G(1), 0>>\nApply(Op(_)) == Op(1)\nA == F!1!G(2)\nB == Apply(F!1!G)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	app := testUserDefinitions(t, m.definitions)[3].body.(*opApplNode)
	arg := testUserDefinitions(t, m.definitions)[4].body.(*opApplNode).operands[0].(*opArgNode)
	if app.operator != testUserDefinitions(t, m.definitions)[0] || app.subExpressionOf != testUserDefinitions(t, m.definitions)[1] || len(app.operands) != 1 || arg.operator != testUserDefinitions(t, m.definitions)[0] || len(g.table.errors.Messages()) != 0 {
		t.Fatal("structural LET selector")
	}
}

func TestGeneratorStructuralLetContextArityErrors(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == <<LET G(x) == x IN G(1), 0>>\nApply(Op(_,_)) == Op(1, 2)\nA == F!1!G\nB == Apply(F!1!G)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 3 || msgs[0].Code.Value != 4204 || msgs[1].Code.Value != 4271 || msgs[2].Code.Value != 4271 {
		t.Fatalf("structural LET arity diagnostics: %#v", msgs)
	}
}

func TestGeneratorParameterizedStructuralLetSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(x) == <<LET G(y) == x IN G(x), 0>>\nApply(Op(_,_)) == Op(1, 2)\nA == F(1)!1!G(2)\nB == Apply(F!1!G)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	app := testUserDefinitions(t, m.definitions)[3].body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	arg := testUserDefinitions(t, m.definitions)[4].body.(*opApplNode).operands[0].(*opArgNode)
	argLambda := arg.operator.(*userOpDefNode)
	if app.subExpressionOf != testUserDefinitions(t, m.definitions)[1] || len(app.operands) != 2 || len(lambda.params) != 2 || lambda.params[0] != testUserDefinitions(t, m.definitions)[1].params[0] || lambda.body.(*opApplNode).operator != testUserDefinitions(t, m.definitions)[0] || arg.arity != 2 || len(argLambda.params) != 2 || argLambda.body.(*opApplNode).operator != testUserDefinitions(t, m.definitions)[0] || len(g.table.errors.Messages()) != 0 {
		t.Fatal("parameterized structural LET selection")
	}
}

func TestGeneratorNestedLetOperatorArgumentErrors(t *testing.T) {
	for _, tc := range []struct {
		body string
		code int
	}{
		{"1", 4005},
		{"LET H == 1 IN H", 4200},
	} {
		t.Run(tc.body, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\nF == LET G == "+tc.body+" IN G\nApply(Op(_)) == Op(1)\nA == Apply(F!:!G!:!Missing)\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			_, e := g.module(tree)
			if e != nil {
				t.Fatal(e)
			}
			msgs := g.table.errors.Messages()
			if len(msgs) != 2 || msgs[0].Code.Value != tc.code || msgs[1].Code.Value != 4271 {
				t.Fatalf("nested LET operator argument errors: %#v", msgs)
			}
		})
	}
}

func TestGeneratorLabelStructuralLetSelectors(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(x) == \\A y \\in {1}: mark(y):: <<LET G(z) == x IN G(y), 0>>\nApply(Op(_,_,_)) == Op(1, 2, 3)\nA == F(1)!mark(2)!1!G(3)\nB == Apply(F!mark!1!G)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	root := testUserDefinitions(t, m.definitions)[1]
	app := testUserDefinitions(t, m.definitions)[3].body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	arg := testUserDefinitions(t, m.definitions)[4].body.(*opApplNode).operands[0].(*opArgNode)
	argLambda := arg.operator.(*userOpDefNode)
	if app.subExpressionOf != root || len(app.operands) != 3 || len(lambda.params) != 3 || lambda.body.(*opApplNode).operator != testUserDefinitions(t, m.definitions)[0] || arg.arity != 3 || len(argLambda.params) != 3 || argLambda.body.(*opApplNode).operator != testUserDefinitions(t, m.definitions)[0] || len(g.table.errors.Messages()) != 0 {
		t.Fatalf("label structural LET selection: %#v", g.table.errors.Messages())
	}
}

func TestGeneratorStructuralLetThroughApplicationsAndPairs(t *testing.T) {
	for _, tc := range []struct{ body, selector string }{
		{"H(LET G(x) == x IN G(1))", "!1"},
		{"[a |-> LET G(x) == x IN G(1)]", "!1"},
		{"[a : LET G(x) == x IN {1}]", "!1"},
		{"CASE TRUE -> LET G(x) == x IN G(1) [] OTHER -> 0", "!1!2"},
		{"\\A x \\in (LET G(y) == y IN {1}): TRUE", "!1"},
		{"[(LET G(x) == x IN [a |-> 1]) EXCEPT !.a = 2]", "!1"},
		{"(LET G(x) == x IN [a |-> 1]).a", "!1"},
	} {
		t.Run(tc.selector+tc.body, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\nH(x) == x\nF == "+tc.body+"\nApply(Op(_)) == Op(1)\nA == F"+tc.selector+"!G(2)\nB == Apply(F"+tc.selector+"!G)\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			m, e := g.module(tree)
			if e != nil {
				t.Fatal(e)
			}
			if len(g.table.errors.Messages()) != 0 {
				t.Fatalf("diagnostics: %#v", g.table.errors.Messages())
			}
			app := testUserDefinitions(t, m.definitions)[len(testUserDefinitions(t, m.definitions))-2].body.(*opApplNode)
			arg := testUserDefinitions(t, m.definitions)[len(testUserDefinitions(t, m.definitions))-1].body.(*opApplNode).operands[0].(*opArgNode)
			if len(app.operands) != 1 || arg.arity != 1 {
				t.Fatal("selected operator application and argument")
			}
		})
	}
}

func TestGeneratorQuantifierBodyLetOperatorArgument(t *testing.T) {
	for _, quantifier := range []string{"\\A y \\in {1}", "\\A y"} {
		t.Run(quantifier, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\nF == "+quantifier+": LET G(x) == <<y, x>> IN G(1)\nApply(Op(_,_)) == Op(1, 2)\nA == Apply(F!@!G)\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			m, e := g.module(tree)
			if e != nil {
				t.Fatal(e)
			}
			if len(g.table.errors.Messages()) != 0 {
				t.Fatalf("diagnostics: %#v", g.table.errors.Messages())
			}
			root := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
			var bound *formalParamNode
			if len(root.bounded) != 0 {
				bound = root.bounded[0][0]
			} else {
				bound = root.unbounded[0]
			}
			arg := testUserDefinitions(t, m.definitions)[3].body.(*opApplNode).operands[0].(*opArgNode)
			lambda := arg.operator.(*userOpDefNode)
			if arg.arity != 2 || len(lambda.params) != 2 || lambda.params[0] != bound || lambda.body.(*opApplNode).operator != testUserDefinitions(t, m.definitions)[0] {
				t.Fatal("quantifier binding identity")
			}
		})
	}
}

func TestGeneratorQuantifierBodyLetExpression(t *testing.T) {
	for _, quantifier := range []string{"\\A y \\in {1}", "\\A y"} {
		t.Run(quantifier, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\nF == "+quantifier+": LET G(x) == <<y, x>> IN G(1)\nA == F!(2)!G(3)\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			m, e := g.module(tree)
			if e != nil {
				t.Fatal(e)
			}
			if len(g.table.errors.Messages()) != 0 {
				t.Fatalf("diagnostics: %#v", g.table.errors.Messages())
			}
			root := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
			var bound *formalParamNode
			if len(root.bounded) != 0 {
				bound = root.bounded[0][0]
			} else {
				bound = root.unbounded[0]
			}
			app := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
			lambda := app.operator.(*userOpDefNode)
			if len(app.operands) != 2 || len(lambda.params) != 2 || lambda.params[0] != bound || lambda.body.(*opApplNode).operator != testUserDefinitions(t, m.definitions)[0] {
				t.Fatal("quantifier argument and binding identity")
			}
		})
	}
}

func TestGeneratorLabelQuantifierBodyLetExpression(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == mark:: \\A y \\in {1}: LET G(x) == <<y, x>> IN G(1)\nA == F!mark!(2)!G(3)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(g.table.errors.Messages()) != 0 {
		t.Fatalf("diagnostics: %#v", g.table.errors.Messages())
	}
	label := testUserDefinitions(t, m.definitions)[1].body.(*labelNode)
	bound := label.body.(*opApplNode).bounded[0][0]
	app := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	if len(app.operands) != 2 || len(lambda.params) != 2 || lambda.params[0] != bound || lambda.body.(*opApplNode).operator != testUserDefinitions(t, m.definitions)[0] {
		t.Fatal("label quantifier selection identity")
	}
}

func TestGeneratorQuantifierLetSelectorArityError(t *testing.T) {
	for _, tc := range []struct{ body, selection string }{
		{"\\A y \\in {1}: LET G(x) == x IN G(1)", "F!(1, 2)!G(3)"},
		{"mark:: \\A y: LET G(x) == x IN G(1)", "F!mark!(1, 2)!G(3)"},
	} {
		t.Run(tc.selection, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\nF == "+tc.body+"\nA == "+tc.selection+"\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			_, e := g.module(tree)
			if e != nil {
				t.Fatal(e)
			}
			msgs := g.table.errors.Messages()
			if len(msgs) != 1 || msgs[0].Code.Value != 4005 || msgs[0].Format != "Selector with %s argument(s) used for quantifier with %s bound identifier(s)." || msgs[0].Parameters[0].Integer != 2 || msgs[0].Parameters[1].Integer != 1 {
				t.Fatalf("selector arity diagnostics: %#v", msgs)
			}
		})
	}
}

func TestGeneratorNestedQuantifierLetArgumentOrder(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == \\A x \\in {1}: \\A y \\in {2}: LET G(z) == <<x, y, z>> IN G(1)\nA == F!(4)!(5)!G(6)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(g.table.errors.Messages()) != 0 {
		t.Fatalf("diagnostics: %#v", g.table.errors.Messages())
	}
	outer := testUserDefinitions(t, m.definitions)[1].body.(*opApplNode)
	inner := outer.operands[0].(*opApplNode)
	app := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	if len(lambda.params) != 3 || lambda.params[0] != outer.bounded[0][0] || lambda.params[1] != inner.bounded[0][0] || len(app.operands) != 3 {
		t.Fatal("nested binding identity")
	}
	for i, value := range []int32{4, 5, 6} {
		if app.operands[i].(*numeralNode).value != value {
			t.Fatalf("argument %d out of order", i)
		}
	}
}

func TestGeneratorQuantifierLetOperatorArgumentWithSuppliedSelector(t *testing.T) {
	for _, tc := range []struct{ body, selection string }{
		{"\\A y \\in {1}: LET G(x) == <<y, x>> IN G(1)", "F!(2)!G"},
		{"mark:: \\A y: LET G(x) == <<y, x>> IN G(1)", "F!mark!(2)!G"},
	} {
		t.Run(tc.selection, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\nF == "+tc.body+"\nApply(Op(_,_)) == Op(1, 2)\nA == Apply("+tc.selection+")\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			m, e := g.module(tree)
			if e != nil {
				t.Fatal(e)
			}
			msgs := g.table.errors.Messages()
			if len(msgs) != 2 || msgs[0].Code.Value != 4005 || msgs[0].Format != "Selector `%s' should not have argument(s)." || msgs[0].Parameters[0].Text != "!(...)" || msgs[1].Code.Value != 4271 {
				t.Fatalf("diagnostics: %#v", msgs)
			}
			if testUserDefinitions(t, m.definitions)[3].body.(*opApplNode).operands[0] != g.nullOAN {
				t.Fatal("operator selector did not recover with null operand")
			}
		})
	}
}

func TestGeneratorQuantifierLetSelectorDiagnosticOrdering(t *testing.T) {
	for _, tc := range []struct {
		definition, selection string
		codes                 []int
	}{
		{"Apply(Op(_,_)) == Op(1, 2)", "Apply(F!(Missing)!G)", []int{4005, 4271}},
		{"", "F!@!G(3)", []int{4004}},
	} {
		t.Run(tc.selection, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\nF == \\A y \\in {1}: LET G(x) == <<y, x>> IN G(1)\n"+tc.definition+"\nA == "+tc.selection+"\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			_, e := g.module(tree)
			if e != nil {
				t.Fatal(e)
			}
			msgs := g.table.errors.Messages()
			if len(msgs) != len(tc.codes) {
				t.Fatalf("diagnostics: %#v", msgs)
			}
			for i, code := range tc.codes {
				if msgs[i].Code.Value != code {
					t.Fatalf("diagnostics: %#v", msgs)
				}
			}
		})
	}
}

func TestGeneratorSpecialOperandLetSelectorErrors(t *testing.T) {
	for _, tc := range []struct{ body, selection, message string }{
		{"[[a |-> 1] EXCEPT !.a = 2]", "F!2!G", "Selecting subexpression of an EXCEPT not yet implemented."},
		{"CASE TRUE -> 1 [] OTHER -> 2", "F!1!G", "Second selector for CASE subexpression must specify  one of two operands."},
		{"CASE TRUE -> 1 [] OTHER -> 2", "F!2!1!G", "Selecting OTHER in a CASE statement."},
	} {
		t.Run(tc.selection, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\nF == "+tc.body+"\nA == "+tc.selection+"\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			_, e := g.module(tree)
			if e != nil {
				t.Fatal(e)
			}
			msgs := g.table.errors.Messages()
			if len(msgs) != 1 || msgs[0].Code.Value != 4005 || msgs[0].Format != tc.message {
				t.Fatalf("selector diagnostics: %#v", msgs)
			}
		})
	}
}

func TestGeneratorEmptyTupleBoundRetainsClosingToken(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nop == \\A <<>> \\in STRING : TRUE\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(g.table.errors.Messages()) != 0 {
		t.Fatalf("diagnostics: %#v", g.table.errors.Messages())
	}
	q := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	if len(q.bounded) != 1 || len(q.bounded[0]) != 1 || !q.tupleBounds[0] || q.bounded[0][0].name.text != ">>" {
		t.Fatal("empty tuple bound")
	}
	loc, e := g.diagnosticLocation(q.bounded[0][0].tree)
	if e != nil || loc.BeginLine != 2 || loc.BeginColumn != 12 || loc.EndColumn != 13 {
		t.Fatalf("closing token location: %#v %v", loc, e)
	}
}

func TestGeneratorTupleAsExpressionDiagnostics(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nop == {<<x, y>> \\in Nat}\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 3 || msgs[0].Code.Value != 4003 || msgs[0].Format != "Unexpected token found." || msgs[1].Code.Value != 4005 || msgs[1].Format != "Need name or step number here, not `%s'." || msgs[1].Parameters[0].Text != "N_IdentifierTuple" || msgs[2].Code.Value != 4200 || msgs[2].Parameters[0].Text != "Nat" {
		t.Fatalf("diagnostics: %#v", msgs)
	}
	if !msgs[1].Parameters[0].UTF16Present || string(utf16.Decode(msgs[1].Parameters[0].UTF16Units)) != "N_IdentifierTuple" || msgs[2].Location.BeginLine != 2 || msgs[2].Location.BeginColumn != 21 || msgs[2].Location.EndColumn != 23 || msgs[0].Location.BeginLine != 2 || msgs[0].Location.BeginColumn != 8 || msgs[0].Location.EndColumn != 15 || msgs[1].Location != msgs[0].Location {
		t.Fatalf("tuple locations: %#v", msgs)
	}
}

func TestGeneratorEmptyTupleChooseAndSubsetBindings(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nA == CHOOSE <<>> : TRUE\nB == CHOOSE <<>> \\in STRING : TRUE\nC == {<<>> \\in STRING : TRUE}\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(g.table.errors.Messages()) != 0 {
		t.Fatalf("diagnostics: %#v", g.table.errors.Messages())
	}
	for i, def := range testUserDefinitions(t, m.definitions) {
		app := def.body.(*opApplNode)
		params := app.unbounded
		if i > 0 {
			if len(app.bounded) != 1 || !app.tupleBounds[0] {
				t.Fatal("bounded tuple metadata")
			}
			params = app.bounded[0]
		}
		if len(params) != 1 || params[0].name.text != ">>" {
			t.Fatalf("tuple formal %d: %#v", i, params)
		}
		loc, err := g.diagnosticLocation(params[0].tree)
		if err != nil || loc.BeginLine != i+2 {
			t.Fatalf("tuple token location: %#v %v", loc, err)
		}
	}
}

func TestGeneratorOperatorArgumentSubexpressionSelection(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nApply(Op(_)) == Op(1)\nH(x) == x\nF == Apply(LAMBDA x: x)\nN == Apply(H)\nA == F!1!(2)\nB == F!1!(2, 3)\nC == F!1!1\nD == N!1!(2)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 3 || msgs[0].Code.Value != 4005 || msgs[0].Format != "Selector with %s arguments used for LAMBDA expression taking %s arguments." || msgs[1].Code.Value != 4005 || msgs[1].Format != "Cannot use !%s to select subexpression of a LAMBDA." || msgs[2].Code.Value != 4005 || msgs[2].Format != "Trying to select subexpression of an operator argument." {
		t.Fatalf("diagnostics: %#v", msgs)
	}
	app := testUserDefinitions(t, m.definitions)[4].body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	original := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode).operands[0].(*opArgNode).operator.(*userOpDefNode)
	if len(app.operands) != 1 || app.operands[0].(*numeralNode).value != 2 || len(lambda.params) != 1 || lambda.params[0] != original.params[0] {
		t.Fatal("LAMBDA selection binding")
	}
}

func TestGeneratorTerminalOperatorArgumentSelection(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nApply(Op(_)) == Op(1)\nH(x) == x\nF == Apply(H)\nA == F!1\nB == Apply(F!1)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4005 || msgs[0].Format != "Selected operator argument when expression expected." {
		t.Fatalf("diagnostics: %#v", msgs)
	}
	original := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode).operands[0].(*opArgNode)
	selected := testUserDefinitions(t, m.definitions)[4].body.(*opApplNode).operands[0].(*opArgNode)
	if selected != original {
		t.Fatal("selected operator argument identity")
	}
}

func TestGeneratorTerminalOperatorArgumentWithEnclosingFormals(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nApply(Op(_)) == Op(1)\nH(y) == y\nF(x) == Apply(H)\nUse(Op(_,_)) == Op(1, 2)\nA == Use(F!1)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4004 || msgs[0].Format != "Wrong number of arguments (%s) given to operator '%s', \nwhich requires %s arguments." {
		t.Fatalf("diagnostics: %#v", msgs)
	}
	arg := testUserDefinitions(t, m.definitions)[4].body.(*opApplNode).operands[0].(*opArgNode)
	lambda := arg.operator.(*userOpDefNode)
	if arg.arity != 2 || len(lambda.params) != 2 || lambda.params[0] != testUserDefinitions(t, m.definitions)[2].params[0] || lambda.params[1].name.text != "NewParam0" || lambda.body.(*opApplNode).operator != testUserDefinitions(t, m.definitions)[1] || len(lambda.body.(*opApplNode).operands) != 0 {
		t.Fatal("terminal source wrapper")
	}
	loc, err := g.diagnosticLocation(lambda.params[1].tree)
	if err != nil || loc.Name != "--TLA+ BUILTINS--" || loc.BeginLine != 0 {
		t.Fatalf("synthetic parameter location: %#v %v", loc, err)
	}
}

func TestGeneratorTerminalOperatorArgumentFormalAndDeclaration(t *testing.T) {
	for _, tc := range []struct {
		declaration, definition string
		wantError               bool
	}{
		{"", "F(Op(_)) == Apply(Op)", false},
		{"CONSTANT Op(_)", "F(x) == Apply(Op)", true},
	} {
		t.Run(tc.definition, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\n"+tc.declaration+"\nApply(P(_)) == P(1)\n"+tc.definition+"\nUse(Q(_,_)) == Q(1, 2)\nA == Use(F!1)\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			m, e := g.module(tree)
			if e != nil {
				t.Fatal(e)
			}
			msgs := g.table.errors.Messages()
			if tc.wantError {
				if len(msgs) != 1 || msgs[0].Code.Value != 4004 || msgs[0].Format != "Operator used with the wrong number of arguments." {
					t.Fatalf("diagnostics: %#v", msgs)
				}
			} else if len(msgs) != 1 || msgs[0].Code.Value != 4271 {
				t.Fatalf("diagnostics: %#v", msgs)
			}
			arg := testUserDefinitions(t, m.definitions)[len(testUserDefinitions(t, m.definitions))-1].body.(*opApplNode).operands[0].(*opArgNode)
			lambda := arg.operator.(*userOpDefNode)
			if len(lambda.params) != 2 || lambda.params[1].name.text != "NewParam0" || len(lambda.body.(*opApplNode).operands) != 0 {
				t.Fatal("wrapped operator argument")
			}
		})
	}
}

func TestGeneratorTerminalBuiltinOperatorArgumentWithFormals(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nApply(P(_)) == P(TRUE)\nF(x) == Apply(~)\nUse(Q(_,_)) == Q(TRUE, TRUE)\nA == Use(F!1)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4004 || msgs[0].Format != "Wrong number of arguments (%s) given to operator '%s', \nwhich requires %s arguments." {
		t.Fatalf("diagnostics: %#v", msgs)
	}
	arg := testUserDefinitions(t, m.definitions)[3].body.(*opApplNode).operands[0].(*opArgNode)
	lambda := arg.operator.(*userOpDefNode)
	if len(lambda.params) != 2 || lambda.params[1].name.text != "NewParam0" || lambda.body.(*opApplNode).operator.header().name.text != "\\lnot" {
		t.Fatal("builtin wrapper")
	}
}

func TestGeneratorEmptyTupleFunctionBound(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF[<<>> \\in STRING] == TRUE\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	if len(g.table.errors.Messages()) != 0 {
		t.Fatalf("diagnostics: %#v", g.table.errors.Messages())
	}
	app := testUserDefinitions(t, m.definitions)[0].body.(*opApplNode)
	if len(app.bounded) != 1 || len(app.bounded[0]) != 1 || !app.tupleBounds[0] || app.bounded[0][0].name.text != ">>" {
		t.Fatal("empty function-bound tuple")
	}
}

func TestGeneratorStructuralNameSelectionErrors(t *testing.T) {
	for _, tc := range []struct {
		body    string
		code    int
		message string
	}{
		{"<<1>>", 4005, "A name selector here must be from a LET clause."},
		{"<<LET G == 1 IN G>>", 4200, "Unknown operator: `%s'."},
	} {
		t.Run(tc.body, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\nF == "+tc.body+"\nA == F!1!Missing\n====\n")
			g, _ := newGenerator(stdcontext.Background(), 0)
			_, e := g.module(tree)
			if e != nil {
				t.Fatal(e)
			}
			msgs := g.table.errors.Messages()
			if len(msgs) != 1 || msgs[0].Code.Value != tc.code || msgs[0].Format != tc.message {
				t.Fatalf("diagnostics: %#v", msgs)
			}
			if tc.code == 4200 && msgs[0].Parameters[0].Text != "Missing" {
				t.Fatalf("unknown name: %#v", msgs[0])
			}
		})
	}
}

func testUserDefinitions[T leafNode](t *testing.T, defs []T) []*userOpDefNode {
	t.Helper()
	out := make([]*userOpDefNode, len(defs))
	for i, def := range defs {
		var ok bool
		out[i], ok = any(def).(*userOpDefNode)
		if !ok {
			t.Fatalf("definition %d has type %T, want user operator", i, def)
		}
	}
	return out
}

func TestSharedGeneratorExtendsIdentityAndIsolation(t *testing.T) {
	ctx := stdcontext.Background()
	p := newNamePool()
	external := map[*name]leafNode{}
	errs := &Errors{}
	a, e := newOwnedGenerator(ctx, 0, p, external, errs)
	if e != nil {
		t.Fatal(e)
	}
	base, e := a.module(parsedModule(t, "---- MODULE A ----\nN == 1\nLOCAL Hidden == 2\n====\n"))
	if e != nil {
		t.Fatal(e)
	}
	b, e := newOwnedGenerator(ctx, 0, p, external, errs)
	if e != nil {
		t.Fatal(e)
	}
	child, e := b.moduleExtending(parsedModule(t, "---- MODULE B ----\nEXTENDS A\nX == N\n====\n"), []*moduleNode{base})
	if e != nil {
		t.Fatal(e)
	}
	if len(child.extendees) != 1 || child.extendees[0] != base || len(child.definitions) != 1 {
		t.Fatal("inheritance or definitions")
	}
	if child.context.lookup(contextKey{name: p.intern("N")}) != base.context.lookup(contextKey{name: p.intern("N")}) {
		t.Fatal("identity lost")
	}
	if child.context.lookup(contextKey{name: p.intern("Hidden")}) != nil {
		t.Fatal("LOCAL inherited")
	}
	other, _ := newGenerator(ctx, 0)
	if _, e = other.moduleExtending(parsedModule(t, "---- MODULE C ----\nEXTENDS A\n====\n"), []*moduleNode{base}); !errors.Is(e, ErrPortIncomplete) {
		t.Fatal(e)
	}
}

func TestModuleExtendingRequiresExactReferences(t *testing.T) {
	ctx := stdcontext.Background()
	pool := newNamePool()
	external := map[*name]leafNode{}
	errs := &Errors{}
	generate := func(text string, extendees []*moduleNode) (*moduleNode, error) {
		g, err := newOwnedGenerator(ctx, 0, pool, external, errs)
		if err != nil {
			t.Fatal(err)
		}
		return g.moduleExtending(parsedModule(t, text), extendees)
	}
	a, err := generate("---- MODULE A ----\n====\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := generate("---- MODULE B ----\n====\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	root := "---- MODULE Root ----\nEXTENDS A, B\n====\n"
	for _, refs := range [][]*moduleNode{nil, {a}, {b, a}, {a, b, a}, {a, nil}} {
		if _, err := generate(root, refs); !errors.Is(err, ErrPortIncomplete) {
			t.Fatalf("references %v: %v", refs, err)
		}
	}
	if _, err := generate(root, []*moduleNode{a, b}); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzeFlatExternalOrderedDiamond(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	files := map[string]modulegraph.Input{"A": input("A", "EXTENDS C\nAValue == N"), "B": input("B", "EXTENDS C\nBValue == N"), "C": input("C", "N == 1")}
	a, e := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("Root", "EXTENDS A, B\nR == N"), func(_ stdcontext.Context, n string) (modulegraph.Input, error) { return files[n], nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if e != nil {
		t.Fatal(e)
	}
	details, examined, e := AnalyzeFlatExternal(stdcontext.Background(), a, 0)
	if e != nil || !examined || len(details) != 0 {
		t.Fatalf("%v %v %#v", e, examined, details)
	}
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	if _, _, e = AnalyzeFlatExternal(ctx, a, 0); !errors.Is(e, stdcontext.Canceled) {
		t.Fatal(e)
	}
}

func TestFlatExtendsPublishedOrigin(t *testing.T) {
	input := func(name string, origin uint8, body string) modulegraph.Input {
		return modulegraph.Input{Path: "same", Origin: origin, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	for _, tc := range []struct {
		origin   uint8
		standard bool
	}{{modulegraph.OriginUnspecified, false}, {modulegraph.OriginWorkspace, false}, {modulegraph.OriginStandard, true}} {
		a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("Root", tc.origin, "EXTENDS A"), func(_ stdcontext.Context, _ string) (modulegraph.Input, error) {
			return input("A", modulegraph.OriginWorkspace, "N == 1"), nil
		}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
		if err != nil {
			t.Fatal(err)
		}
		_, examined, root, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 0)
		if err != nil || !examined || root == nil || root.standard != tc.standard || len(root.extendees) != 1 || root.extendees[0].standard {
			t.Fatalf("origin %d: %#v %v", tc.origin, root, err)
		}
	}
}

func TestLocalInstancesPreserveSourceAndIsolation(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	files := map[string]modulegraph.Input{"A": input("A", "X == 1\nLOCAL Hidden == 2"), "B": input("B", "LOCAL INSTANCE A\nLOCAL INSTANCE A"), "C": input("C", "EXTENDS A")}
	a, e := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("Root", "EXTENDS B, C\nR == X"), func(_ stdcontext.Context, n string) (modulegraph.Input, error) { return files[n], nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if e != nil {
		t.Fatal(e)
	}
	details, examined, root, e := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if e != nil || !examined || root == nil || len(details) != 0 {
		t.Fatalf("%v %v %v", e, examined, details)
	}
	b, c := root.extendees[0], root.extendees[1]
	if len(b.instances) != 2 || len(b.definitions) != 2 {
		t.Fatalf("instances %d definitions %d", len(b.instances), len(b.definitions))
	}
	x := c.context.lookup(contextKey{name: root.owner.intern("X")}).(*userOpDefNode)
	first := b.definitions[0].(*userOpDefNode)
	second := b.definitions[1].(*userOpDefNode)
	if x.local || !first.local || first.source != x || second.source != x || first.body != x.body || !sameOriginallyDefined(first, second) {
		t.Fatal("source, local flag or shared body mismatch")
	}
	if root.context.lookup(contextKey{name: root.owner.intern("Hidden")}) != nil {
		t.Fatal("LOCAL leaked")
	}
}

func TestNonlocalConstantFreeInstanceReusesDefinitions(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	files := map[string]modulegraph.Input{"A": input("A", "X == 1\nLOCAL Hidden == 2"), "B": input("B", "INSTANCE A"), "C": input("C", "EXTENDS A")}
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("Root", "EXTENDS B, C\nR == X"), func(_ stdcontext.Context, n string) (modulegraph.Input, error) { return files[n], nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, examined, root, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil || !examined || root == nil || len(details) != 0 {
		t.Fatalf("%v %v %v", err, examined, details)
	}
	b, c := root.extendees[0], root.extendees[1]
	x := c.context.lookup(contextKey{name: root.owner.intern("X")}).(*userOpDefNode)
	if len(b.instances) != 1 || b.instances[0].local || len(b.definitions) != 1 || b.definitions[0] != x {
		t.Fatal("instance did not reuse original definition")
	}
	if root.context.lookup(contextKey{name: root.owner.intern("Hidden")}) != nil {
		t.Fatal("LOCAL leaked")
	}
}

func TestInstancesShareSubstitutionsAndQualifiedIdentity(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	root := input("Root", "CONSTANT X\nI(p) == INSTANCE A WITH Y <- p\nR == I(1)!F(2)")
	dep := input("A", "CONSTANT X, Y\nF(z) == X\nG == Y")
	a, e := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, _ string) (modulegraph.Input, error) { return dep, nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if e != nil {
		t.Fatal(e)
	}
	details, examined, m, e := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if e != nil || !examined || len(details) != 0 {
		t.Fatalf("%v %v %+v", e, examined, details)
	}
	if len(m.instances) != 1 || len(m.instances[0].substs) != 2 {
		t.Fatalf("%+v", m.instances)
	}
	f, ok := m.context.lookup(contextKey{name: m.owner.intern("I!F")}).(*userOpDefNode)
	if !ok {
		t.Fatal("missing qualified definition")
	}
	g, ok := m.context.lookup(contextKey{name: m.owner.intern("I!G")}).(*userOpDefNode)
	if !ok {
		t.Fatal("missing qualified definition")
	}
	fw, ok := f.body.(*substInNode)
	if !ok {
		t.Fatal("missing wrapper")
	}
	gw, ok := g.body.(*substInNode)
	if !ok || fw.substs[0] != gw.substs[0] || fw.substs[1] != m.instances[0].substs[1] {
		t.Fatal("substitutions not shared")
	}
	projection := projectGraph(&generator{table: &symbolTable{errors: &Errors{}}}, m)
	counts := map[string]int{}
	for _, n := range projection.Nodes {
		counts[n.Class]++
	}
	if projection.Truncated || counts["InstanceNode"] != 1 || counts["SubstInNode"] < 2 {
		t.Fatalf("projection omitted instance links: %+v", counts)
	}
	if f.sourceNode() == f || len(f.compound()) != 2 || len(f.params) != 2 {
		t.Fatal("source, compound ID or formal parameters not preserved")
	}
	marker, ok := m.context.lookup(contextKey{name: m.owner.intern("I")}).(*userOpDefNode)
	if !ok || marker.kind != 6 || marker.arity != 1 {
		t.Fatal("missing module marker")
	}
	for _, d := range m.definitions {
		if d == marker {
			t.Fatal("top-level marker appended")
		}
	}
}

func TestInstanceSelectorRetainsSubstitutions(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	root := input("Root", "INSTANCE A WITH X <- 1\nR == F!1\n")
	dep := input("A", "CONSTANT X\nF == <<X>>")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, _ string) (modulegraph.Input, error) { return dep, nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, examined, m, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil || !examined || len(details) != 0 {
		t.Fatalf("%v %v %+v", err, examined, details)
	}
	f := m.context.lookup(contextKey{name: m.owner.intern("F")}).(*userOpDefNode)
	original := f.body.(*substInNode)
	r := m.context.lookup(contextKey{name: m.owner.intern("R")}).(*userOpDefNode)
	outer := r.body.(*opApplNode)
	selected, ok := outer.operands[0].(*substInNode)
	if !ok || selected == original || selected.substs[0] != original.substs[0] || selected.tree != original.tree || selected.instantiatingModule != original.instantiatingModule || selected.instantiatedModule != original.instantiatedModule {
		t.Fatal("selector lost wrapper identity or sharing")
	}
	if _, ok := selected.body.(*opApplNode); !ok {
		t.Fatalf("selected body: %T", selected.body)
	}
}

func TestQualifiedInstanceSelectorAndSubstitutionOperator(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	files := map[string]modulegraph.Input{
		"A": input("A", "CONSTANT X\nF(y) == <<X, y>>"),
		"B": input("B", "CONSTANT Op(_, _)\nG == Op(1, 2)"),
	}
	root := input("Root", "I(p) == INSTANCE A WITH X <- p\nR == I(7)!F(8)!1\nINSTANCE B WITH Op <- I!F\n")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, name string) (modulegraph.Input, error) { return files[name], nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, examined, m, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil || !examined || len(details) != 0 {
		t.Fatalf("%v %v %+v", err, examined, details)
	}
	f := m.context.lookup(contextKey{name: m.owner.intern("I!F")}).(*userOpDefNode)
	r := m.context.lookup(contextKey{name: m.owner.intern("R")}).(*userOpDefNode)
	appl, ok := r.body.(*opApplNode)
	if !ok || len(appl.operands) != 2 || appl.subExpressionOf != f {
		t.Fatalf("qualified selector: %#v", r.body)
	}
	lambda, ok := appl.operator.(*userOpDefNode)
	if !ok || len(lambda.params) != 2 {
		t.Fatal("qualified selector lost parameters")
	}
	if _, ok = lambda.body.(*substInNode); !ok {
		t.Fatal("qualified selector lost substitutions")
	}
	if len(m.instances) != 2 || len(m.instances[1].substs) != 1 {
		t.Fatal("missing second instance")
	}
	rhs, ok := m.instances[1].substs[0].expr.(*opArgNode)
	if !ok || rhs.operator != f || rhs.arity != 2 {
		t.Fatalf("qualified operator RHS: %#v", m.instances[1].substs[0].expr)
	}
}

func TestInstanceLetSelectionRetainsSubstitutions(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	root := input("Root", "INSTANCE A WITH X <- 1\nApply(Op(_)) == Op(2)\nR == F!:!G(3)\nS == Apply(F!:!G)\n")
	dep := input("A", "CONSTANT X\nF == LET G(y) == <<X, y>> IN G(1)")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, _ string) (modulegraph.Input, error) { return dep, nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, examined, m, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil || !examined || len(details) != 0 {
		t.Fatalf("%v %v %+v", err, examined, details)
	}
	orig := m.context.lookup(contextKey{name: m.owner.intern("F")}).(*userOpDefNode).body.(*substInNode)
	check := func(n *userOpDefNode) {
		t.Helper()
		w, ok := n.body.(*substInNode)
		if !ok || w.substs[0] != orig.substs[0] || w.instantiatedModule != orig.instantiatedModule || w.instantiatingModule != orig.instantiatingModule {
			t.Fatalf("missing shared wrapper in %T", n.body)
		}
	}
	r := m.context.lookup(contextKey{name: m.owner.intern("R")}).(*userOpDefNode).body.(*opApplNode)
	check(r.operator.(*userOpDefNode))
	s := m.context.lookup(contextKey{name: m.owner.intern("S")}).(*userOpDefNode).body.(*opApplNode)
	check(s.operands[0].(*opArgNode).operator.(*userOpDefNode))
}

func TestChainedInstanceSelectorWrapperOrder(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	files := map[string]modulegraph.Input{"A": input("A", "CONSTANT X\nF(z) == <<X, z>>"), "B": input("B", "CONSTANT Y\nI(p) == INSTANCE A WITH X <- Y")}
	root := input("Root", "J(q) == INSTANCE B WITH Y <- q\nR == J(1)!I(2)!F(3)!1")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, name string) (modulegraph.Input, error) { return files[name], nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, examined, m, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil || !examined || len(details) != 0 {
		t.Fatalf("%v %v %+v", err, examined, details)
	}
	f := m.context.lookup(contextKey{name: m.owner.intern("J!I!F")}).(*userOpDefNode)
	originalOuter := f.body.(*substInNode)
	originalInner := originalOuter.body.(*substInNode)
	r := m.context.lookup(contextKey{name: m.owner.intern("R")}).(*userOpDefNode).body.(*opApplNode)
	if len(r.operands) != 3 || r.subExpressionOf != f {
		t.Fatalf("lost arguments: %#v", r)
	}
	lambda := r.operator.(*userOpDefNode)
	outer := lambda.body.(*substInNode)
	inner := outer.body.(*substInNode)
	if outer == originalOuter || inner == originalInner || outer.substs[0] != originalOuter.substs[0] || inner.substs[0] != originalInner.substs[0] || outer.tree != originalOuter.tree || inner.tree != originalInner.tree || outer.instantiatingModule != originalOuter.instantiatingModule || inner.instantiatingModule != originalInner.instantiatingModule {
		t.Fatal("wrapper order, source or sharing changed")
	}
	projection := projectGraph(&generator{table: &symbolTable{errors: &Errors{}}}, m)
	count := 0
	for _, node := range projection.Nodes {
		if node.Class == "SubstInNode" {
			count++
		}
	}
	if projection.Truncated || count < 4 {
		t.Fatalf("projection omitted wrappers: %d", count)
	}
	limited := projectGraphLimited(&generator{table: &symbolTable{errors: &Errors{}}}, m, 1, 1)
	if !limited.Truncated {
		t.Fatal("projection did not respect identity bounds")
	}
}

func TestSelectedNullOperandUsesSourceSelector(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF == <<1>>\nR == F!1\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	f := m.context.lookup(contextKey{name: g.pool.intern("F")}).(*userOpDefNode)
	f.body.(*opApplNode).operands[0] = nil
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_StructOp {
			return n
		}
		for _, c := range n.Heirs() {
			if got := find(c); got != nil {
				return got
			}
		}
		return nil
	}
	selector := find(tree)
	if selector == nil {
		t.Fatal("missing selector")
	}
	selected, err := g.numericExpression(tree, []*generated.SyntaxTreeNode{selector}, f, f.body, nil, nil, 1, true)
	if err != nil || selected != g.nullOAN {
		t.Fatalf("%v %T", err, selected)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4003 || msgs[0].Location.BeginLine != 3 || msgs[0].Format != "An unexpected null node specified by '%s'.\nThis is probably due to a previous error." || len(msgs[0].Parameters) != 1 || msgs[0].Parameters[0].Text != "argument selector 1" {
		t.Fatalf("%+v", msgs)
	}
}

func TestInstanceSelectedOperatorOperandRetainsSubstitution(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	dep := input("A", "CONSTANT X\nH(y) == <<X, y>>\nUse(Op(_)) == Op(1)\nF == Use(H)")
	root := input("Root", "INSTANCE A WITH X <- 1\nApply(Op(_)) == Op(1)\nR == Apply(F!1)")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, _ string) (modulegraph.Input, error) { return dep, nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, examined, m, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil || !examined || len(details) != 1 || details[0].Code.Value != 4004 {
		t.Fatalf("%v %v %+v", err, examined, details)
	}
	f := m.context.lookup(contextKey{name: m.owner.intern("F")}).(*userOpDefNode)
	original := f.body.(*substInNode)
	r := m.context.lookup(contextKey{name: m.owner.intern("R")}).(*userOpDefNode).body.(*opApplNode)
	lambda := r.operands[0].(*opArgNode).operator.(*userOpDefNode)
	selected := lambda.body.(*substInNode)
	if len(lambda.params) != 1 || selected.substs[0] != original.substs[0] || selected.tree != original.tree {
		t.Fatal("selected operator argument lost its substitution")
	}
}

func TestSelectorSubstitutionDepthAndCancellation(t *testing.T) {
	g, err := newGenerator(stdcontext.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	var body exprNode = g.nullOAN
	for i := 0; i < 3; i++ {
		h, err := newExprHeader(g.pool, 13, g.nullOAN.tree)
		if err != nil {
			t.Fatal(err)
		}
		body = &substInNode{exprHeader: h, body: body}
	}
	if _, err = g.selectorBody(body, 1); !errors.Is(err, ErrPortIncomplete) {
		t.Fatalf("depth: %v", err)
	}
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	g.ctx = ctx
	if _, err = g.selectorBody(body, 1); !errors.Is(err, stdcontext.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestQualifiedSubstitutionOperatorArityRecovery(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	files := map[string]modulegraph.Input{"A": input("A", "CONSTANT X\nF(y) == <<X, y>>"), "B": input("B", "CONSTANT Op(_)\nG == Op(1)")}
	root := input("Root", "I(p) == INSTANCE A WITH X <- p\nINSTANCE B WITH Op <- I!F")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, name string) (modulegraph.Input, error) { return files[name], nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, examined, m, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil || !examined || m == nil {
		t.Fatalf("%v %v %+v", err, examined, details)
	}
	if len(details) != 2 || details[0].Code.Value != 4271 || details[0].Location.Name != "Root" || details[0].Location.BeginLine != 3 || details[1].Code.Value != 4243 {
		t.Fatalf("unexpected source recovery: %+v", details)
	}
	sentinel, ok := m.instances[1].substs[0].expr.(*opArgNode)
	if !ok || sentinel.arity != -2 {
		t.Fatal("unexpected sentinel")
	}
}

func TestQualifiedSubstitutionRejectsInstanceArguments(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	files := map[string]modulegraph.Input{"A": input("A", "CONSTANT X\nF(y) == <<X, y>>"), "B": input("B", "CONSTANT Op(_, _)\nG == Op(1, 2)")}
	root := input("Root", "I(p) == INSTANCE A WITH X <- p\nINSTANCE B WITH Op <- I(Missing)!F")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, name string) (modulegraph.Input, error) { return files[name], nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, examined, m, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil || !examined || m == nil || len(details) != 2 || details[0].Code.Value != 4005 || details[0].Format != "Selector `%s' should not have argument(s)." || details[0].Location.Name != "Root" || details[0].Location.BeginLine != 3 || details[1].Code.Value != 4243 {
		t.Fatalf("%v %v %+v", err, examined, details)
	}
}

func TestQualifiedSelectorRetainsHigherOrderAndLabelBindings(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	dep := input("A", "CONSTANT Op(_)\nD(z) == <<Op(z)>>\nF == \\A y \\in {1}: mark(y):: <<Op(y), y>>")
	root := input("Root", "Id(x) == x\nI(P(_)) == INSTANCE A WITH Op <- P\nR == I(Id)!D(1)!1\nS == I(Id)!F!mark(2)!1")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, _ string) (modulegraph.Input, error) { return dep, nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, examined, m, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil || !examined || len(details) != 0 {
		t.Fatalf("%v %v %+v", err, examined, details)
	}
	for _, name := range []string{"R", "S"} {
		appl := m.context.lookup(contextKey{name: m.owner.intern(name)}).(*userOpDefNode).body.(*opApplNode)
		lambda := appl.operator.(*userOpDefNode)
		if len(appl.operands) != 2 || len(lambda.params) != 2 || lambda.params[0].arity != 1 {
			t.Fatalf("%s lost marker or selected formal", name)
		}
		if _, ok := appl.operands[0].(*opArgNode); !ok {
			t.Fatalf("%s lost higher-order instance argument", name)
		}
		if _, ok := lambda.body.(*substInNode); !ok {
			t.Fatalf("%s lost substitution wrapper", name)
		}
	}
}

func TestChainedSubstitutionRejectsLaterInstanceArguments(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	files := map[string]modulegraph.Input{
		"A": input("A", "CONSTANT X\nF(y) == <<X, y>>"),
		"B": input("B", "CONSTANT Y\nI(p) == INSTANCE A WITH X <- Y"),
		"C": input("C", "CONSTANT Op(_, _, _)\nG == Op(1, 2, 3)"),
	}
	root := input("Root", "J(q) == INSTANCE B WITH Y <- q\nINSTANCE C WITH Op <- J!I(Missing)!F")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, name string) (modulegraph.Input, error) { return files[name], nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, examined, m, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil || !examined || m == nil || len(details) != 2 || details[0].Code.Value != 4005 || details[0].Parameters[0].Text != "I" || details[0].Location.BeginLine != 3 || details[1].Code.Value != 4243 {
		t.Fatalf("%v %v %+v", err, examined, details)
	}
}

func TestNestedModulesShareLexicalSymbolsAndPublishAfterBody(t *testing.T) {
	tree := parsedModule(t, "---- MODULE Root ----\nCONSTANT C\nBefore == C\n---- MODULE Inner ----\nInside == Before\n====\nINSTANCE Inner\nAfter == Inside\n====\n")
	g, m, err := analyzeOwned(stdcontext.Background(), tree, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.table.errors.Messages()) != 0 {
		t.Fatal(g.table.errors.Messages())
	}
	if len(m.bodyDefinitions) != 4 {
		t.Fatalf("body definitions %d", len(m.bodyDefinitions))
	}
	inner, ok := m.bodyDefinitions[1].(*moduleNode)
	if !ok {
		t.Fatalf("child %T", m.bodyDefinitions[1])
	}
	if inner.context == m.context || inner.standard {
		t.Fatal("child context or standard origin")
	}
	inside := inner.definitions[0].(*userOpDefNode)
	app, ok := inside.body.(*opApplNode)
	if !ok || app.operator != m.definitions[0] {
		t.Fatalf("ancestor identity %#v", inside.body)
	}
	if g.pool.java.exact {
		t.Fatal("nested module must invalidate allocation profile")
	}
}

func TestNestedModuleInstancesAndSubstitutions(t *testing.T) {
	for _, src := range []string{
		"---- MODULE Root ----\nCONSTANT C\n---- MODULE Inner ----\nCONSTANT K\nF == K\n====\nI == INSTANCE Inner WITH K <- C\nUse == I!F\n====\n",
		"---- MODULE Root ----\nCONSTANT C\nLOCAL P == C\n---- MODULE Inner ----\nF == P\n====\nINSTANCE Inner\nUse == F\n====\n",
		"---- MODULE Root ----\n---- MODULE A ----\nX == 1\n====\n---- MODULE B ----\nEXTENDS A\nY == X\n====\nINSTANCE B\nUse == Y\n====\n",
	} {
		g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, src), 0)
		if err != nil || m == nil || len(g.table.errors.Messages()) != 0 {
			t.Fatalf("%s\n%v %#v", src, err, g.table.errors.Messages())
		}
	}
}

func TestNestedModuleConflictAndRecursiveSection(t *testing.T) {
	for _, tc := range []struct {
		src   string
		code  int
		count int
	}{
		{"---- MODULE Root ----\nRECURSIVE F\n---- MODULE Inner ----\nX == 1\n====\n====\n", 4294, 1},
		{"---- MODULE Root ----\n---- MODULE Inner ----\nX == 1\n====\n---- MODULE Inner ----\nY == 2\n====\n====\n", 4223, 2},
	} {
		g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, tc.src), 0)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range g.table.errors.Messages() {
			if d.Code.Value == tc.code {
				found = true
			}
		}
		if !found || len(m.bodyDefinitions) != tc.count {
			t.Fatalf("diagnostics %#v, definitions %d", g.table.errors.Messages(), len(m.bodyDefinitions))
		}
	}
}

func TestNestedModuleInStandardSourceAndImportedChild(t *testing.T) {
	input := func(n, b string, origin uint8) modulegraph.Input {
		return modulegraph.Input{Path: n, Text: []byte("---- MODULE " + n + " ----\n" + b + "\n====\n"), Origin: origin}
	}
	files := map[string]modulegraph.Input{"A": input("A", "---- MODULE Child ----\nX == 1\n====", modulegraph.OriginStandard)}
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("Root", "EXTENDS A, Child\nR == X", modulegraph.OriginWorkspace), func(_ stdcontext.Context, n string) (modulegraph.Input, error) { return files[n], nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, examined, root, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 0)
	if err != nil || !examined || len(details) > 0 {
		t.Fatalf("%v %#v", err, details)
	}
	if len(root.extendees) != 2 || !root.extendees[0].standard || root.extendees[1].standard {
		t.Fatal("nested origin")
	}
}

func TestOperatorAndModuleNamespacesAreDistinct(t *testing.T) {
	src := "---- MODULE Root ----\nInner == 1\n---- MODULE Inner ----\nX == 2\n====\nINSTANCE Inner\nResult == <<Inner, X>>\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, src), 0)
	if err != nil || m == nil || len(g.table.errors.Messages()) != 0 {
		t.Fatalf("%v %#v", err, g.table.errors.Messages())
	}
}

func TestQualifiedInstanceChecksHigherOrderArguments(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	root := input("Root", "I == INSTANCE A\nGood(x) == x\nOk == I!Apply(Good)\nBad == I!Apply(Missing)")
	dep := input("A", "Apply(P(_)) == P(1)")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, _ string) (modulegraph.Input, error) { return dep, nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, examined, _, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil || !examined || len(details) != 2 {
		t.Fatalf("%v %v %+v", err, examined, details)
	}
	if details[0].Code.Value != 4200 || details[1].Code.Value != 4271 || details[1].Location.BeginLine != 5 || details[1].Parameters[1].Text != "I!Apply" {
		t.Fatalf("unexpected diagnostics: %+v", details)
	}
}

func TestFunctionApplicationCountsSymbolsAndTupleBounds(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nf[i, j, k \\in {1}, <<l, m>> \\in {<<1, 2>>}, p \\in {1}] == i\nGood == f[1, 2, 3, <<1, 2>>, 1]\nBad == f[1, 2]\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4260 || msgs[0].Location.BeginLine != 4 || msgs[0].Parameters[1].Integer != 5 {
		t.Fatalf("unexpected diagnostics: %+v", msgs)
	}
}
