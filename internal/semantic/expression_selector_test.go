// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"fmt"
	"os"
	"strings"
	"testing"
)

// Each case is run through the real parser and Generator producers. UIDs
// are the javaHistory tickets, counted from 305 after the module bootstrap.

type esCase struct {
	label, body string
	exact       bool
	first       string // first history invalidation when !exact
	count       uint64 // final history count when exact
	diags       []string
	check       func(t *testing.T, g *generator, m *moduleNode)
}

func esAnalyze(t *testing.T, body string) (*generator, *moduleNode, error) {
	t.Helper()
	return analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\n"+body+"\n====\n"), 0)
}

func runESCase(t *testing.T, c esCase) {
	t.Helper()
	g, m, err := esAnalyze(t, c.body)
	if err != nil || m == nil {
		t.Fatalf("analyze: %v (site %q)", err, g.incomplete.site)
	}
	h := &g.pool.history
	got := renderedDiagnostics(t, g)
	if os.Getenv("ES_DUMP") != "" {
		fmt.Printf("ESDUMP %s exact=%v first=%q count=%d diags=%#v\n", c.label, h.exact, h.firstUnknown, h.count, got)
		return
	}
	if h.exact != c.exact || (!c.exact && h.firstUnknown != c.first) || (c.exact && h.count != c.count) {
		t.Fatalf("history exact=%v first=%q count=%d", h.exact, h.firstUnknown, h.count)
	}
	if strings.Join(got, "\n") != strings.Join(c.diags, "\n") {
		t.Fatalf("diagnostics\n got %q\nwant %q", got, c.diags)
	}
	if c.check != nil {
		c.check(t, g, m)
	}
}

func esDef(m *moduleNode, i int) *userOpDefNode { return m.definitions[i].(*userOpDefNode) }

func esUID(t *testing.T, g *generator, node any, uid uint32, class string, kind int) {
	t.Helper()
	got := g.pool.history.reuse(node)
	if !got.known || got.uid != uid || got.class != class || got.kind != kind {
		t.Fatalf("ticket %#v, want %s/%d at %d", got, class, kind, uid)
	}
}

// A function application whose function expression is missing parses as an
// N_FcnAppl whose first heir is the "[" token. generateExpression reaches
// its default case for that token (Generator.java:3496-3505), reports 4003
// and returns null; N_FcnAppl then returns null before any argument or node
// (3303-3307). The operator definition stores the null body (2660, 2674), so
// the only UID taken is the OpDefNode's.
func TestExpressionFunctionApplicationMissingFunction(t *testing.T) {
	cases := []esCase{
		{label: "three operands", body: "op == [1,2,3]", exact: true, count: 306,
			diags: []string{"2:7-2:7 Unsupported expression type `['."}},
		{label: "one operand", body: "op == [x]", exact: true, count: 306,
			diags: []string{"2:7-2:7 Unsupported expression type `['."}},
		{label: "two operands", body: "op == [x, y]", exact: true, count: 306,
			diags: []string{"2:7-2:7 Unsupported expression type `['."}},
		{label: "infix operand", body: "op == 1 = [x]", exact: true, count: 308,
			diags: []string{"2:11-2:11 Unsupported expression type `['."}},
	}
	for _, c := range cases {
		c.check = func(t *testing.T, g *generator, m *moduleNode) {
			def := esDef(m, 0)
			esUID(t, g, def, uint32(c.count-1), "OpDefNode", 5)
			if c.label != "infix operand" && def.body != nil {
				t.Fatalf("body %T, want the stored null", def.body)
			}
		}
		t.Run(c.label, func(t *testing.T) { runESCase(t, c) })
	}
}

// A null function expression returns null immediately (Generator.java:
// 3303-3307): the argument "3 ** 4" is never generated, so its unresolved
// operator is not reported.
func TestExpressionFunctionApplicationNullReceiver(t *testing.T) {
	runESCase(t, esCase{label: "null receiver", body: "op == (1 ++ 2)[3 ** 4]", exact: true, count: 306,
		diags: []string{
			"2:10-2:11 Could not find declaration or definition of symbol '++'.",
			"2:8-2:13 Couldn't resolve infix operator symbol `++'.",
		},
		check: func(t *testing.T, g *generator, m *moduleNode) {
			esUID(t, g, esDef(m, 0), 305, "OpDefNode", 5)
		}})
}

// A null result that reaches a consumer whose null handling is not audited
// keeps its diagnostic but no longer certifies history.
func TestExpressionFunctionApplicationUnprovedConsumer(t *testing.T) {
	runESCase(t, esCase{label: "LET body", body: "op == LET a == 1 IN [1]", first: "unproved expression producer",
		diags: []string{"2:21-2:21 Unsupported expression type `['."}})
}

// generateLabel reads only heirs()[1] of the label's GeneralId and of each
// argument's (Generator.java:6869, 6880, 6894), so a prefixed label has the
// same name, parameters, diagnostics and UID schedule as the bare label.
func TestGenerateLabelIgnoresIdPrefix(t *testing.T) {
	pairs := [][2]string{
		{"op == \\A x \\in {} : A!B!lbl(x) :: x", "op == \\A x \\in {} : lbl(x) :: x"},
		{"op == \\A x \\in {} : A!B!C!lbl(x) :: x", "op == \\A x \\in {} : lbl(x) :: x"},
		{"op == \\A x \\in {} : A!lbl(x, x) :: x", "op == \\A x \\in {} : lbl(x, x) :: x"},
	}
	for _, p := range pairs {
		t.Run(p[0], func(t *testing.T) {
			g1, m1, e1 := esAnalyze(t, p[0])
			g2, m2, e2 := esAnalyze(t, p[1])
			if e1 != nil || e2 != nil {
				t.Fatalf("analyze: %v / %v", e1, e2)
			}
			h1, h2 := &g1.pool.history, &g2.pool.history
			if !h1.exact || !h2.exact || h1.count != h2.count {
				t.Fatalf("history %v/%d vs %v/%d", h1.exact, h1.count, h2.exact, h2.count)
			}
			d1, d2 := renderedDiagnostics(t, g1), renderedDiagnostics(t, g2)
			if len(d1) != len(d2) {
				t.Fatalf("diagnostics %q vs %q", d1, d2)
			}
			for i := range d1 {
				// Locations differ by the prefix width; codes and texts may not.
				if d1[i][strings.Index(d1[i], " "):] != d2[i][strings.Index(d2[i], " "):] {
					t.Fatalf("diagnostic %d: %q vs %q", i, d1[i], d2[i])
				}
			}
			l1, l2 := esFindLabel(esDef(m1, 0).body), esFindLabel(esDef(m2, 0).body)
			if l1 == nil || l2 == nil || l1.name.text != l2.name.text || len(l1.params) != len(l2.params) {
				t.Fatalf("labels %+v vs %+v", l1, l2)
			}
			u1, u2 := h1.reuse(l1), h2.reuse(l2)
			if !u1.known || u1 != u2 {
				t.Fatalf("label tickets %#v vs %#v", u1, u2)
			}
			for i := range l1.params {
				if h1.reuse(l1.params[i]) != h2.reuse(l2.params[i]) {
					t.Fatalf("param %d tickets differ", i)
				}
			}
		})
	}
}

func esFindLabel(n exprNode) *labelNode {
	switch v := n.(type) {
	case *labelNode:
		return v
	case *opApplNode:
		for _, o := range v.operands {
			if e, ok := o.(exprNode); ok {
				if l := esFindLabel(e); l != nil {
					return l
				}
			}
		}
	}
	return nil
}

// FindingSubExpr on an OpApplNode (Generator.java:1162-1180, 1394-1400):
// FormalParam, ConstantDecl and UserDefinedOp operators select by ArgNum over
// their arity; every other non-built-in kind has no operand. The null
// OpDefNode of nullOAN has kind 0, a VARIABLE kind 3. Neither error allocates.
func TestNumericSelectorOperatorKinds(t *testing.T) {
	cases := []esCase{
		{label: "nullOAN body", body: "Op == @\nTHEOREM Op!1 = 1", exact: true, count: 309,
			diags: []string{
				"2:7-2:7 @ used where its meaning is not defined.",
				"3:12-3:12 Choosing operand `argument selector 1' of subexpression with no operands.",
			}},
		{label: "variable", body: "VARIABLE v\nOp == v\nTHEOREM Op!1 = v", exact: true, count: 311,
			diags: []string{"4:12-4:12 Choosing operand `argument selector 1' of subexpression with no operands."}},
		{label: "variable <<", body: "VARIABLE v\nOp == v\nTHEOREM Op!<< = v", exact: true, count: 311,
			diags: []string{"4:12-4:13 Choosing operand `<<' of subexpression with no operands."}},
		{label: "variable @", body: "VARIABLE v\nOp == v\nTHEOREM Op!@ = v", exact: true, count: 311,
			diags: []string{"4:12-4:12 Choosing operand `@' of subexpression with no operands."}},
		{label: "variable (...)", body: "VARIABLE v\nOp == v\nTHEOREM Op!(1) = v", exact: true, count: 311,
			diags: []string{"4:12-4:14 Choosing operand `!(...)' of subexpression with no operands."}},
		{label: "constant", body: "CONSTANT c\nOp == c\nTHEOREM Op!1 = c", exact: true, count: 311,
			diags: []string{"4:12-4:12 Nonexistent operand specified by `argument selector 1'."}},
		{label: "constant @", body: "CONSTANT c\nOp == c\nTHEOREM Op!@ = c", first: "unproved general identifier selector",
			diags: []string{"4:12-4:12 Nonexistent operand specified by `@'."}},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) { runESCase(t, c) })
	}
}

// Null bodies met while selecting (Generator.java:1128-1145, 1527-1546).
func TestSelectorNullBodies(t *testing.T) {
	cases := []esCase{
		// The admitted !: leaves curNode null; isNullSelection reports it.
		{label: "colon after null body", body: "Foo(u) == 1 ** u\nASSUME Foo(5)!:", exact: true, count: 309,
			diags: []string{
				"2:13-2:14 Could not find declaration or definition of symbol '**'.",
				"2:11-2:16 Couldn't resolve infix operator symbol `**'.",
				"3:15-3:15 An unexpected null node specified by ':'.\nThis is probably due to a previous error.",
			}},
		// FollowingLabels sets curNode to the null label body; the next
		// selector reports the failed selection.
		{label: "selected label", body: "Q == lab :: 1 ** 2\nASSUME Q!lab!1", first: "unproved operator resolution",
			diags: []string{
				"2:15-2:16 Could not find declaration or definition of symbol '**'.",
				"2:13-2:18 Couldn't resolve infix operator symbol `**'.",
				"3:14-3:14 Subexpression selection failed, probably due to an error in the selected expression.",
			}},
		// A label reached by descent is unwrapped in FindingSubExpr and
		// its null body is reported by isNullSelection with idx stepped back
		// (Generator.java:1530-1544), at the selector that selected the label.
		// Recorded SANY agrees on D-71fc60f55bf7 (144:26, not 144:28).
		{label: "descended label", body: "Q == {lab :: 1 ** 2}\nASSUME Q!1!1", first: "unproved operator resolution",
			diags: []string{
				"2:16-2:17 Could not find declaration or definition of symbol '**'.",
				"2:14-2:19 Couldn't resolve infix operator symbol `**'.",
				"3:10-3:10 An unexpected null node specified by 'argument selector 1'.\nThis is probably due to a previous error.",
			}},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) { runESCase(t, c) })
	}
}

const esCOp = "COp(q, p) ==\n   CHOOSE x : lab(x) :: LET a ++ b == {a, b, q, p, x}\n                        IN <<q, 99 ++ p>>\n"

// A NameSel that names a LET definition may be an operator token
// (Selector.finish, Generator.java:409-418); FindingOpName resolves its
// synonym and looks it up in the LET context (735-738, 777).
func TestLetOperatorSymbolSelectors(t *testing.T) {
	cases := []esCase{
		{label: "structural then ++", body: esCOp + "THEOREM COp(1, 2)!(3)!++(4, 5) = {}", first: "unproved application selector",
			check: esLocalSelection("++", 5)},
		{label: "label colon ++", body: esCOp + "THEOREM COp(1, 2)!lab(3)!:!++(4, 5) = {}", first: "unproved application selector",
			check: esLocalSelection("++", 5)},
		{label: "label colon identifier", body: "COp(q) == lab :: LET f(a) == a IN 1\nTHEOREM COp(1)!lab!:!f(4) = 4", exact: true, count: 323,
			check: esLabelColonSchedule},
		{label: "unknown symbol", body: esCOp + "THEOREM COp(1, 2)!(3)!**(4, 5) = {}", first: "unproved application selector",
			diags: []string{"5:23-5:24 Unknown operator: `**'."}},
		{label: "label colon not LET", body: "D == lab :: 1\nTHEOREM D!lab!:!++(1, 2) = 1", first: "unproved application selector",
			diags: []string{"3:15-3:15 A name selector here must be from a LET clause."}},
		{label: "label colon null body", body: "D == lab :: 1 ** 2\nTHEOREM D!lab!:!f = 1", first: "unproved operator resolution",
			diags: []string{
				"2:15-2:16 Could not find declaration or definition of symbol '**'.",
				"2:13-2:18 Couldn't resolve infix operator symbol `**'.",
				"3:15-3:15 An unexpected null node specified by ':'.\nThis is probably due to a previous error.",
			}},
		// ArgNum(NullSel, 1) for the LET/IN under the label is -1.
		{label: "LET operand arguments", body: "C2 == lab :: LET a ++ b == a IN 1\nTHEOREM C2!(3)!++(1, 2) = 1", first: "unproved application selector",
			diags: []string{"3:12-3:14 A LET/IN expression has only one operand."}},
		// The label's body is reached by unwrapping in the LET descent.
		{label: "descended null label", body: "Q == \\A i \\in {} : lab :: 1 ** 2\nTHEOREM Q!(1)!foo = 1", first: "unproved operator resolution",
			diags: []string{
				"2:29-2:30 Could not find declaration or definition of symbol '**'.",
				"2:27-2:32 Couldn't resolve infix operator symbol `**'.",
				"2:20-2:32 Label lab must contain formal parameter `i'.",
				"3:11-3:13 An unexpected null node specified by '!(...)'.\nThis is probably due to a previous error.",
			}},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) { runESCase(t, c) })
	}
}

// esLocalSelection checks that the last theorem's left operand applies the
// LET-local operator through the selected-expression wrapper.
func esLocalSelection(local string, wrapperArgs int) func(*testing.T, *generator, *moduleNode) {
	return func(t *testing.T, g *generator, m *moduleNode) {
		eq := esTheoremBody(t, m.topLevel[len(m.topLevel)-1])
		w, ok := eq.operands[0].(*opApplNode)
		if !ok || w.operator == nil || w.operator.header().name.text != "LAMBDA" || len(w.operands) != wrapperArgs {
			t.Fatalf("wrapper %#v", eq.operands[0])
		}
		lambda := w.operator.(*userOpDefNode)
		inner, ok := lambda.body.(*opApplNode)
		if !ok || inner.operator.header().name.text != local {
			t.Fatalf("selected body %#v", lambda.body)
		}
	}
}

// esLabelColonSchedule pins the FindingOpName sequence for COp(1)!lab!:!f(4)
// (Generator.java:880-905 arguments, 1718-1770 clone, 2009-2079 LAMBDA): the
// root argument, the local argument, the cloned formal and its OpApplNode,
// the application of f, the LAMBDA OpDefNode and the wrapping OpApplNode
// take consecutive UIDs.
func esLabelColonSchedule(t *testing.T, g *generator, m *moduleNode) {
	esLocalSelection("f", 2)(t, g, m)
	w := esTheoremBody(t, m.topLevel[len(m.topLevel)-1]).operands[0].(*opApplNode)
	lambda := w.operator.(*userOpDefNode)
	inner := lambda.body.(*opApplNode)
	if len(lambda.params) != 2 || len(inner.operands) != 1 {
		t.Fatalf("lambda params %d, inner operands %d", len(lambda.params), len(inner.operands))
	}
	order := []struct {
		node  any
		class string
		kind  int
	}{
		{w.operands[0], "NumeralNode", 16},
		{w.operands[1], "NumeralNode", 16},
		{lambda.params[1], "FormalParamNode", 11},
		{inner.operands[0], "OpApplNode", 9},
		{inner, "OpApplNode", 9},
		{lambda, "OpDefNode", 5},
		{w, "OpApplNode", 9},
	}
	first := g.pool.history.reuse(order[0].node).uid
	for i, o := range order {
		esUID(t, g, o.node, first+uint32(i), o.class, o.kind)
	}
	var root *userOpDefNode
	for _, d := range m.definitions {
		if u, ok := d.(*userOpDefNode); ok && u.name.text == "COp" {
			root = u
		}
	}
	if root == nil || lambda.params[0] != root.params[0] {
		t.Fatal("the root formal is reused, not cloned")
	}
}

func esTheoremBody(t *testing.T, n any) *opApplNode {
	t.Helper()
	th, ok := n.(*theoremNode)
	if !ok {
		t.Fatalf("top level %T", n)
	}
	eq, ok := th.body.(*opApplNode)
	if !ok || len(eq.operands) != 2 {
		t.Fatalf("theorem body %#v", th.body)
	}
	return eq
}

// The former incomplete control here, a label extra-parameter order outside
// the bounded Java cursor, now prints its distinct names in the documented
// fallback order (label_extra_order_test.go). The LET-local continuation
// shapes formerly listed here (U!1!Op(42)!:, C!(3)!++(9, 10)!lab,
// C!lab(3)!:!++(9, 10)!lab) are ported and pinned in let_continuation_test.go.
