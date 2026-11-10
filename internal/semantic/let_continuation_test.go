// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"strings"
	"testing"
)

// Tests for the shared FindingOpName exit (let_continuation.go): a LET-local
// operator that is not the last selector (Generator.java:884-1029). All
// cases run the real parser and Generator; UIDs are javaHistory tickets.
// Expected values are derived from the pinned Java source, cited per case.

const lcCOp = "COp(q) == lab :: LET f(a) == m :: a IN 1\n"
const lcCOpTwo = "COp(q) == lab :: LET f(a) == LET g(b) == n :: {a, b} IN 2 IN 1\n"
const lcCOp2 = "COp2(q, p) ==\n   CHOOSE x : lab(x) ::  LET a ++ b ==  {lab(y):: {y, a, b} : y \\in {1}}\n                         IN  <<q, 99 ++ p >>\n"
const lcR = "R == \\A x \\in {1} : LET f(c) == c IN x\n"
const lcU = "U == /\\ LET Op(x) == {x} IN 44\n     /\\ TRUE\n"

func lcDef(t *testing.T, g *generator, m *moduleNode, name string) *userOpDefNode {
	t.Helper()
	if d, ok := m.context.lookup(contextKey{name: g.pool.intern(name)}).(*userOpDefNode); ok {
		return d
	}
	t.Fatalf("no definition %s", name)
	return nil
}

// lcLet unwraps labels and binders' single operand down to the first LET/IN.
func lcLet(t *testing.T, n levelNode) *letInNode {
	t.Helper()
	for n != nil {
		switch v := n.(type) {
		case *letInNode:
			return v
		case *labelNode:
			n = v.body
		case *opApplNode:
			if len(v.operands) == 0 {
				t.Fatalf("no LET under %#v", v)
			}
			n, _ = v.operands[0].(exprNode)
		default:
			t.Fatalf("no LET under %T", n)
		}
	}
	t.Fatal("no LET")
	return nil
}

func lcLocal(t *testing.T, g *generator, let *letInNode, name string) *userOpDefNode {
	t.Helper()
	d, ok := let.context.lookup(contextKey{name: g.pool.intern(name)}).(*userOpDefNode)
	if !ok {
		t.Fatalf("no LET definition %s", name)
	}
	return d
}

// lcWrapper returns the left operand of the last statement's "=" as the
// selected-expression wrapper OpApplNode over a LAMBDA.
func lcWrapper(t *testing.T, m *moduleNode) (*opApplNode, *userOpDefNode) {
	t.Helper()
	var body levelNode
	switch v := m.topLevel[len(m.topLevel)-1].(type) {
	case *theoremNode:
		body = v.body
	case *assumeNode:
		body = v.expression
	default:
		t.Fatalf("top level %T", v)
	}
	eq, ok := body.(*opApplNode)
	if !ok || len(eq.operands) != 2 {
		t.Fatalf("statement body %#v", body)
	}
	w, ok := eq.operands[0].(*opApplNode)
	if !ok {
		t.Fatalf("wrapper %#v", eq.operands[0])
	}
	lambda, ok := w.operator.(*userOpDefNode)
	if !ok || lambda.name.text != "LAMBDA" {
		t.Fatalf("wrapper operator %#v", w.operator)
	}
	return w, lambda
}

func lcParams(t *testing.T, lambda *userOpDefNode, want ...*formalParamNode) {
	t.Helper()
	if len(lambda.params) != len(want) {
		t.Fatalf("LAMBDA has %d params, want %d", len(lambda.params), len(want))
	}
	for i := range want {
		if lambda.params[i] != want[i] {
			t.Fatalf("LAMBDA param %d is %p (%s), want the source formal %p (%s)", i, lambda.params[i], lambda.params[i].name.text, want[i], want[i].name.text)
		}
	}
}

func lcNumerals(t *testing.T, w *opApplNode, want ...string) {
	t.Helper()
	if len(w.operands) != len(want) {
		t.Fatalf("wrapper has %d operands, want %d", len(w.operands), len(want))
	}
	for i, text := range want {
		n, ok := w.operands[i].(*numeralNode)
		if !ok || n.image != text {
			t.Fatalf("operand %d %#v, want Numeral %s", i, w.operands[i], text)
		}
	}
}

// lcSchedule checks that nodes took consecutive tickets in the given order.
func lcSchedule(t *testing.T, g *generator, order ...struct {
	node  any
	class string
	kind  int
}) {
	t.Helper()
	first := g.pool.history.reuse(order[0].node).uid
	for i, o := range order {
		esUID(t, g, o.node, first+uint32(i), o.class, o.kind)
	}
}

type lcTicket = struct {
	node  any
	class string
	kind  int
}

// T1: COp(1)!lab!:!f(4)!m. Index 0 generates N1 (922); lab is a label of
// COp (1049-1127); the colon is admitted before a NameSel (1128-1136); f is
// a non-final FindingOpName in the LET context: arity, then N4 (884-925),
// params += f's own formal a (991-994), FollowingLabels (998-1003); m is a
// label of f. The terminal builds LAMBDA(params [q, a], body label m) and
// its application to [N1, N4] with subExprOf COp (2006-2081): four tickets,
// three fewer than the final form COp(1)!lab!:!f(4), which also clones f's
// formal, applies it and applies f (1712-1770).
func TestLetContinuationLabelTarget(t *testing.T) {
	runESCase(t, esCase{label: "final form", body: lcCOp + "THEOREM COp(1)!lab!:!f(4) = 4", exact: true, count: 324})
	runESCase(t, esCase{label: "label after LET operator", body: lcCOp + "THEOREM COp(1)!lab!:!f(4)!m = 4", exact: true, count: 321,
		check: func(t *testing.T, g *generator, m *moduleNode) {
			root := lcDef(t, g, m, "COp")
			f := lcLocal(t, g, lcLet(t, root.body), "f")
			w, lambda := lcWrapper(t, m)
			lcParams(t, lambda, root.params[0], f.params[0])
			lcNumerals(t, w, "1", "4")
			if lambda.body != f.labels.get(g.pool.intern("m")) {
				t.Fatalf("LAMBDA body %#v, want f's label m", lambda.body)
			}
			if w.subExpressionOf != root {
				t.Fatalf("subExpressionOf %#v", w.subExpressionOf)
			}
			lcSchedule(t, g, lcTicket{w.operands[0], "NumeralNode", 16}, lcTicket{w.operands[1], "NumeralNode", 16}, lcTicket{lambda, "OpDefNode", 5}, lcTicket{w, "OpApplNode", 9})
		}})
}

// Two successive LET continuations reuse the same transition; the frame of
// g carries COp's, f's and g's formals and arguments. A final operator
// target clones only the last operator's formals (1712-1770); a final label
// target clones nothing. The difference is again three tickets.
func TestLetContinuationSuccessive(t *testing.T) {
	runESCase(t, esCase{label: "final operator after two LETs", body: lcCOpTwo + "THEOREM COp(1)!lab!:!f(4)!:!g(5) = 4", exact: true, count: 331,
		check: func(t *testing.T, g *generator, m *moduleNode) {
			root := lcDef(t, g, m, "COp")
			f := lcLocal(t, g, lcLet(t, root.body), "f")
			gl := lcLocal(t, g, lcLet(t, f.body), "g")
			w, lambda := lcWrapper(t, m)
			if len(lambda.params) != 3 || lambda.params[0] != root.params[0] || lambda.params[1] != f.params[0] || lambda.params[2] == gl.params[0] {
				t.Fatal("non-final formals are the operators' own; only g's formal is cloned")
			}
			inner := lambda.body.(*opApplNode)
			if inner.operator != gl {
				t.Fatalf("LAMBDA body applies %#v", inner.operator)
			}
			lcNumerals(t, w, "1", "4", "5")
			lcSchedule(t, g, lcTicket{w.operands[0], "NumeralNode", 16}, lcTicket{w.operands[1], "NumeralNode", 16}, lcTicket{w.operands[2], "NumeralNode", 16},
				lcTicket{lambda.params[2], "FormalParamNode", 11}, lcTicket{inner.operands[0], "OpApplNode", 9}, lcTicket{inner, "OpApplNode", 9}, lcTicket{lambda, "OpDefNode", 5}, lcTicket{w, "OpApplNode", 9})
		}})
	runESCase(t, esCase{label: "label after two LETs", body: lcCOpTwo + "THEOREM COp(1)!lab!:!f(4)!:!g(5)!n = 4", exact: true, count: 328,
		check: func(t *testing.T, g *generator, m *moduleNode) {
			root := lcDef(t, g, m, "COp")
			f := lcLocal(t, g, lcLet(t, root.body), "f")
			gl := lcLocal(t, g, lcLet(t, f.body), "g")
			w, lambda := lcWrapper(t, m)
			lcParams(t, lambda, root.params[0], f.params[0], gl.params[0])
			lcNumerals(t, w, "1", "4", "5")
			if lambda.body != gl.labels.get(g.pool.intern("n")) || w.subExpressionOf != root {
				t.Fatal("body is g's label n, subExprOf the root")
			}
			lcSchedule(t, g, lcTicket{w.operands[0], "NumeralNode", 16}, lcTicket{w.operands[1], "NumeralNode", 16}, lcTicket{w.operands[2], "NumeralNode", 16}, lcTicket{lambda, "OpDefNode", 5}, lcTicket{w, "OpApplNode", 9})
		}})
	// A colon straight after the root switches into its LET (1549-1566).
	runESCase(t, esCase{label: "root colon then label", body: "R2 == LET f(a) == lab :: {a} IN 1\nTHEOREM R2!:!f(3)!lab = {3}", exact: true, count: 320,
		check: func(t *testing.T, g *generator, m *moduleNode) {
			root := lcDef(t, g, m, "R2")
			f := lcLocal(t, g, lcLet(t, root.body), "f")
			w, lambda := lcWrapper(t, m)
			lcParams(t, lambda, f.params[0])
			lcNumerals(t, w, "3")
			if lambda.body != f.labels.get(g.pool.intern("lab")) {
				t.Fatal("body is f's label")
			}
		}})
}

// T2, the test209 shape U!1!Op(42)!:. Index 1 selects the LET conjunct;
// Op is non-final, so N42 is generated, params += Op's x and curNode is
// Op's body (FindingSubExpr, 998-1003); the final colon is admitted after
// FindingOpName (1130-1137). The terminal is LAMBDA(params [x], body
// Op.body) applied to [N42]: three consecutive tickets. H1 admits the final
// colon in the history predicate, so history is exact.
func TestLetContinuationFinalColon(t *testing.T) {
	runESCase(t, esCase{label: "expression", body: lcU + "ASSUME U!1!Op(42)!: = {42}", exact: true, count: 321,
		check: func(t *testing.T, g *generator, m *moduleNode) {
			root := lcDef(t, g, m, "U")
			op := lcLocal(t, g, lcLet(t, root.body), "Op")
			w, lambda := lcWrapper(t, m)
			lcParams(t, lambda, op.params[0])
			lcNumerals(t, w, "42")
			if lambda.body != op.body || w.subExpressionOf != root {
				t.Fatal("body is Op's body by pointer, subExprOf U")
			}
			lcSchedule(t, g, lcTicket{w.operands[0], "NumeralNode", 16}, lcTicket{lambda, "OpDefNode", 5}, lcTicket{w, "OpApplNode", 9})
		}})
	// A structural selector after the LET operator descends Op's body.
	runESCase(t, esCase{label: "structural", body: "U == /\\ LET Op(x) == {x, 2} IN 44\n     /\\ TRUE\nASSUME U!1!Op(42)!1 = 42", exact: true, count: 321,
		check: func(t *testing.T, g *generator, m *moduleNode) {
			root := lcDef(t, g, m, "U")
			op := lcLocal(t, g, lcLet(t, root.body), "Op")
			w, lambda := lcWrapper(t, m)
			lcParams(t, lambda, op.params[0])
			lcNumerals(t, w, "42")
			if lambda.body != op.body.(*opApplNode).operands[0] {
				t.Fatal("body is the first element of Op's body")
			}
		}})
	// Parameter context: the argument is G's own formal.
	runESCase(t, esCase{label: "parameter", body: lcU + "G(z) == U!1!Op(z)!:", exact: true, count: 319,
		check: func(t *testing.T, g *generator, m *moduleNode) {
			gdef := lcDef(t, g, m, "G")
			w := gdef.body.(*opApplNode)
			arg, ok := w.operands[0].(*opApplNode)
			if !ok || arg.operator != gdef.params[0] {
				t.Fatalf("operand %#v, want an application of G's z", w.operands[0])
			}
		}})
}

// T3, operator-token LET names (SubrefLL's COp2). Six formals in index
// order [q, p, x, a, b, y]: root (991), x from the root label or the CHOOSE
// bound (1104-1110 or 1349-1373), the local's own a, b (991-994), y from
// the local's label or its set-former bound. History stays inexact: H2
// needs a non-interning resolved-name lookup, which does not exist.
func TestLetContinuationOperatorToken(t *testing.T) {
	type shape struct {
		label, selector, first string
		boundX, boundY         bool
	}
	for _, c := range []shape{
		{"label colon then label", "COp2(1, 2)!lab(3)!:!++(9, 10)!lab(11)", "unproved application selector", false, false},
		{"bound then label", "COp2(1, 2)!(3)!++(9, 10)!lab(11)", "unproved application selector", true, false},
		{"label colon then bound", "COp2(1, 2)!lab(3)!:!++(9, 10)!(11)", "unproved general identifier selector", false, true},
	} {
		runESCase(t, esCase{label: c.label, body: lcCOp2 + "THEOREM " + c.selector + " = {}", first: c.first,
			check: func(t *testing.T, g *generator, m *moduleNode) {
				root := lcDef(t, g, m, "COp2")
				choose := root.body.(*opApplNode)
				rootLabel := root.labels.get(g.pool.intern("lab"))
				plus := lcLocal(t, g, lcLet(t, root.body), "++")
				setOf := plus.body.(*opApplNode)
				plusLabel := plus.labels.get(g.pool.intern("lab"))
				x := rootLabel.params[0]
				if c.boundX {
					x = choose.unbounded[0]
				}
				y := plusLabel.params[0]
				var body levelNode = plusLabel
				if c.boundY {
					y = flattenFormal(setOf.bounded)[0]
					body = setOf.operands[0].(exprNode)
				}
				w, lambda := lcWrapper(t, m)
				lcParams(t, lambda, root.params[0], root.params[1], x, plus.params[0], plus.params[1], y)
				lcNumerals(t, w, "1", "2", "3", "9", "10", "11")
				if lambda.body != body {
					t.Fatalf("LAMBDA body %#v", lambda.body)
				}
			}})
	}
}

// T4, errors in source order. Each Java selector error is terminal; only
// argument generation records non-terminal diagnostics.
func TestLetContinuationErrors(t *testing.T) {
	cases := []esCase{
		// A numeral has no operand (1519-1527), final or not.
		{label: "has none", body: "K == 1\nTHEOREM K!<<!foo = 1", exact: true, count: 310,
			diags: []string{"3:11-3:12 Selecting subexpression of expression that has none."}},
		{label: "has none, non-final", body: "K == 1\nTHEOREM K!<<!foo!: = 1", exact: true, count: 310,
			diags: []string{"3:11-3:12 Selecting subexpression of expression that has none."}},
		// Index 1 binds x and generates zz (1349-1373), then reports the
		// colon that follows it (1567-1575); f(1) is never reached.
		{label: "bound then colon", body: lcR + "THEOREM R!(zz)!:!f(1) = 1", first: "unproved application selector",
			diags: []string{"3:12-3:13 Unknown operator: `zz'.", "3:11-3:14 !: should not follow an operand selector."}},
		// A leading colon before an operand selector (1130-1137).
		{label: "leading colon", body: lcR + "THEOREM R!:!1!f = 1", first: "unproved general identifier selector",
			diags: []string{"3:11-3:11 `!:' can be used only after a name and either at the end after an\noperator name or before an operator name."}},
		// Unknown with no later NameSel: Java consumes the rest without
		// generating Nope's arguments and reports at Nope (741-814).
		{label: "unknown local", body: lcU + "ASSUME U!1!Nope(1)!: = 1", exact: true, count: 317,
			diags: []string{"4:12-4:15 Unknown operator: `Nope'."}},
		// Arity before arguments (906-914): at the selector without
		// arguments, at the OpArgs with them; no argument node is built
		// (the same count as "unknown local", whose RHS is identical).
		{label: "missing arguments", body: lcU + "ASSUME U!1!Op!: = 1", exact: true, count: 317,
			diags: []string{"4:8-4:15 The operator Op requires 1 arguments."}},
		{label: "extra arguments", body: lcU + "ASSUME U!1!Op(1, 2)!: = 1", exact: true, count: 317,
			diags: []string{"4:14-4:19 The operator Op requires 1 arguments."}},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) { runESCase(t, c) })
	}
}

// Obligation 1: deferring bound-argument generation past the traversal and
// the LET lookup. Those allocate nothing and record nothing, so on a typed
// error the reached bound arguments are generated first and the error is
// reported after them; unreached arguments are never built. Before this
// change the R!(zz)!2!f and R!(7)!2!f forms reported only the LET/IN error
// (the bound argument was skipped although Java generates it at 1371), and
// the non-final forms stopped at the simple-label-selector-mode guard.
func TestLetContinuationBoundArgumentEffects(t *testing.T) {
	cases := []esCase{
		// T4c: zz's diagnostic, then the LET/IN operand error at "2".
		{label: "unknown bound argument, LET operand", body: lcR + "THEOREM R!(zz)!2!f = 1", exact: true, count: 318,
			diags: []string{"3:12-3:13 Unknown operator: `zz'.", "3:16-3:16 A LET/IN expression has only one operand."}},
		// The same with an allocating argument: one more ticket, N7.
		{label: "numeral bound argument, LET operand", body: lcR + "THEOREM R!(7)!2!f = 1", exact: true, count: 319,
			diags: []string{"3:15-3:15 A LET/IN expression has only one operand."}},
		// Bound argument, then an unknown non-final LET name: Nope's
		// argument is not generated (318 = the "wrong arity" count below).
		{label: "unknown bound argument, unknown local", body: lcR + "THEOREM R!(zz)!Nope(1)!: = 1", exact: true, count: 318,
			diags: []string{"3:12-3:13 Unknown operator: `zz'.", "3:16-3:19 Unknown operator: `Nope'."}},
		{label: "numeral bound argument, unknown local", body: lcR + "THEOREM R!(7)!Nope(1)!: = 1", exact: true, count: 319,
			diags: []string{"3:15-3:18 Unknown operator: `Nope'."}},
		// Bound argument, then a wrong local arity: no local argument node.
		{label: "unknown bound argument, wrong arity", body: lcR + "THEOREM R!(zz)!f!: = 1", exact: true, count: 318,
			diags: []string{"3:12-3:13 Unknown operator: `zz'.", "3:9-3:18 The operator f requires 1 arguments."}},
		{label: "numeral bound argument, wrong arity", body: lcR + "THEOREM R!(7)!f(1, 2)!: = 1", exact: true, count: 319,
			diags: []string{"3:16-3:21 The operator f requires 1 arguments."}},
		// The repaired final path in fact mode: Java's bound generation does
		// not depend on isFact (1349-1373).
		{label: "fact mode", body: lcR + "THEOREM TRUE BY R!(zz)!2!f", exact: true, count: 319,
			diags: []string{"3:20-3:21 Unknown operator: `zz'.", "3:24-3:24 A LET/IN expression has only one operand."}},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) { runESCase(t, c) })
	}
	// An earlier argument that stops generation stops before the later
	// unknown name is reported, and the incomplete origin is the
	// argument's own, not the superseded traversal failure. The argument
	// stops at a mixed-name label residual, which stays incomplete
	// (label_extra_order_test.go).
	t.Run("argument abort first", func(t *testing.T) {
		g, _, err := esAnalyze(t, lcR+"THEOREM R!(LET F == L(x, x, y)::TRUE IN 1)!Nope(1)!: = 1")
		if err != ErrPortIncomplete || g.incomplete.site != "generator.construct.generate-label" {
			t.Fatalf("err=%v site=%q", err, g.incomplete.site)
		}
		for _, d := range renderedDiagnostics(t, g) {
			if strings.Contains(d, "Nope") {
				t.Fatalf("later error reported after the abort: %q", d)
			}
		}
	})
}

// Obligation 2: an incomplete speculative path that already generated
// arguments is not followed by a second evaluation of the same prefix.
// nestedLetExpression generates f's N7, then meets f's null body; the shared
// path would generate N7 again. The first attempt keeps its origin.
func TestLetContinuationNoDoubleEvaluation(t *testing.T) {
	g, _, err := esAnalyze(t, "R == LET f(a) == 1 ** a IN 1\nTHEOREM R!:!f(7)!:!g = 1")
	if err != ErrPortIncomplete || g.incomplete.site != "generator.construct.nested-let-expression" {
		t.Fatalf("err=%v site=%q", err, g.incomplete.site)
	}
	for _, d := range renderedDiagnostics(t, g) {
		if strings.Contains(d, "unexpected null node") {
			t.Fatalf("the shared path ran after the first attempt: %q", d)
		}
	}
	// Without generated arguments the fallback is still taken: the same
	// first attempt fails before any effect and the shared path completes.
	runESCase(t, esCase{label: "no effects, fallback", body: "R2 == LET f(a) == lab :: {a} IN 1\nTHEOREM R2!:!f(3)!lab = {3}", exact: true, count: 320})
}

// Obligation 3: a non-final LET operator with a null body (its definition
// failed) leaves curNode null in FindingSubExpr (998-1003, 1015). Java then
// records a clean diagnostic and returns nullOAN: isNullSelection for an
// admitted colon (1128-1136, 1544-1546), "Subexpression selection failed"
// for an operand selector (1137-1145), also when a LET name follows it.
func TestLetContinuationNullBody(t *testing.T) {
	const nullU = "U == /\\ LET Op(x) == 1 ** x IN 44\n     /\\ TRUE\n"
	cases := []esCase{
		{label: "colon", body: nullU + "ASSUME U!1!Op(42)!: = 1", exact: true, count: 316,
			diags: []string{
				"2:24-2:25 Could not find declaration or definition of symbol '**'.",
				"2:22-2:27 Couldn't resolve infix operator symbol `**'.",
				"4:19-4:19 An unexpected null node specified by ':'.\nThis is probably due to a previous error.",
			}},
		{label: "operand", body: nullU + "ASSUME U!1!Op(42)!1 = 1", first: "unproved general identifier selector",
			diags: []string{
				"2:24-2:25 Could not find declaration or definition of symbol '**'.",
				"2:22-2:27 Couldn't resolve infix operator symbol `**'.",
				"4:19-4:19 Subexpression selection failed, probably due to an error in the selected expression.",
			}},
		// The operand selector fails on the null body before the LET name
		// is reached (1137-1145), so no LET switch happens.
		{label: "operand before LET name", body: nullU + "ASSUME U!1!Op(42)!1!g = 1", first: "unproved general identifier selector",
			diags: []string{
				"2:24-2:25 Could not find declaration or definition of symbol '**'.",
				"2:22-2:27 Couldn't resolve infix operator symbol `**'.",
				"4:19-4:19 Subexpression selection failed, probably due to an error in the selected expression.",
			}},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) { runESCase(t, c) })
	}
}

// T5: a substitution context. I!U!1!Op(42)!: carries the instance's SubstIn
// prefix (1015-1026); the terminal copies it around Op's body (1975-1990)
// before the LAMBDA and its application: Numeral, SubstInNode, OpDefNode,
// OpApplNode.
func TestLetContinuationSubstitution(t *testing.T) {
	src := "---- MODULE Root ----\n---- MODULE Inner ----\nCONSTANT k\nU == /\\ LET Op(x) == {x, k} IN 44\n     /\\ TRUE\n====\nI == INSTANCE Inner WITH k <- 1\nASSUME I!U!1!Op(42)!: = {42, 1}\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, src), 0)
	if err != nil {
		t.Fatalf("analyze: %v (site %q)", err, g.incomplete.site)
	}
	if !g.pool.history.exact {
		t.Fatalf("history inexact: %q", g.pool.history.firstUnknown)
	}
	if d := renderedDiagnostics(t, g); len(d) != 0 {
		t.Fatalf("diagnostics %q", d)
	}
	w, lambda := lcWrapper(t, m)
	subst, ok := lambda.body.(*substInNode)
	if !ok {
		t.Fatalf("LAMBDA body %#v, want the copied SubstIn", lambda.body)
	}
	lcNumerals(t, w, "42")
	if len(lambda.params) != 1 {
		t.Fatalf("params %d", len(lambda.params))
	}
	lcSchedule(t, g, lcTicket{w.operands[0], "NumeralNode", 16}, lcTicket{subst, "SubstInNode", 13}, lcTicket{lambda, "OpDefNode", 5}, lcTicket{w, "OpApplNode", 9})
}

// T6 and the guarded shapes: these stay ErrPortIncomplete at their sites.
func TestLetContinuationControlsStayIncomplete(t *testing.T) {
	cases := []struct{ label, body, site string }{
		// An unknown non-final name with a later NameSel: Java looks up the
		// compound Nope!lab (741-806).
		{"unknown compound", lcU + "ASSUME U!1!Nope!lab = 1", "generator.construct.simple-label-selector-mode"},
		// Fact mode keeps the non-final guard.
		{"fact mode", lcU + "THEOREM TRUE BY U!1!Op(42)!:", "generator.construct.simple-label-selector-mode"},
		// A LET-local INSTANCE generates; a selector continuing through its
		// marker (I!k after V!:) is not ported.
		{"LET instance", "---- MODULE Inner ----\nk == 1\n====\nV == LET I == INSTANCE Inner IN 1\nASSUME V!:!I!k = 1", "generator.construct.structural-let-from-allow-undefined"},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			g, _, err := esAnalyze(t, c.body)
			if err != ErrPortIncomplete || (c.site != "" && g.incomplete.site != c.site) {
				t.Fatalf("err=%v site=%q", err, g.incomplete.site)
			}
		})
	}
}
