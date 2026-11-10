// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"errors"
	"fmt"
	"testing"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

// rrEvent is one expected source-ordered SemanticNode allocation, located in
// the finished graph.
type rrEvent struct {
	class string
	kind  int
	node  func(g *generator, m *moduleNode) any
}

func rrSym(text string) func(*generator, *moduleNode) leafNode {
	return func(g *generator, m *moduleNode) leafNode {
		n, err := g.table.resolve(g.pool.intern(text), false)
		if err != nil {
			return nil
		}
		return n
	}
}
func rrDef(i int) func(*generator, *moduleNode) *userOpDefNode {
	return func(_ *generator, m *moduleNode) *userOpDefNode { return m.definitions[i].(*userOpDefNode) }
}
func rrBody(i int) func(*generator, *moduleNode) exprNode {
	return func(g *generator, m *moduleNode) exprNode { return rrDef(i)(g, m).body }
}
func rrOperand(path func(*generator, *moduleNode) exprNode, j int) func(*generator, *moduleNode) exprNode {
	return func(g *generator, m *moduleNode) exprNode {
		return path(g, m).(*opApplNode).operands[j].(exprNode)
	}
}
func rrParam(i, j int) func(*generator, *moduleNode) *formalParamNode {
	return func(g *generator, m *moduleNode) *formalParamNode { return rrDef(i)(g, m).params[j] }
}
func rrDefEvent(i int) rrEvent {
	return rrEvent{"OpDefNode", 5, func(g *generator, m *moduleNode) any { return rrDef(i)(g, m) }}
}
func rrParamEvent(i, j int) rrEvent {
	return rrEvent{"FormalParamNode", 11, func(g *generator, m *moduleNode) any { return rrParam(i, j)(g, m) }}
}
func rrAppl(path func(*generator, *moduleNode) exprNode) rrEvent {
	return rrEvent{"OpApplNode", 9, func(g *generator, m *moduleNode) any { return path(g, m) }}
}
func rrDecl(text string, kind int) rrEvent {
	return rrEvent{"OpDeclNode", kind, func(g *generator, m *moduleNode) any { return rrSym(text)(g, m) }}
}
func rrLet(i int) func(*generator, *moduleNode) *letInNode {
	return func(g *generator, m *moduleNode) *letInNode { return rrBody(i)(g, m).(*letInNode) }
}
func rrTop(i int) func(*generator, *moduleNode) *useOrHideNode {
	return func(_ *generator, m *moduleNode) *useOrHideNode { return m.topLevel[i].(*useOrHideNode) }
}

type rrCase struct {
	label, body string
	first       uint32 // UID of the first listed event
	events      []rrEvent
	count       uint64
	codes       string
	check       func(t *testing.T, g *generator, m *moduleNode)
}

func runResolutionCase(t *testing.T, c rrCase) {
	t.Helper()
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\n"+c.body+"\n====\n"), 0)
	if err != nil || m == nil {
		t.Fatalf("analyze: %v", err)
	}
	h := &g.pool.history
	if !h.exact || h.count != c.count {
		t.Fatalf("exact=%v count=%d first=%q", h.exact, h.count, h.firstUnknown)
	}
	if got := diagnosticCodes(g); got != c.codes {
		t.Fatalf("codes %s, want %s", got, c.codes)
	}
	for i, e := range c.events {
		if e.node == nil {
			continue // unreachable from the graph; covered by position and count
		}
		got := h.reuse(e.node(g, m))
		if !got.known || got.uid != c.first+uint32(i) || got.class != e.class || got.kind != e.kind {
			t.Fatalf("event %d (%s/%d): %#v", i, e.class, e.kind, got)
		}
	}
	if c.check != nil {
		c.check(t, g, m)
	}
}

func diagnosticCodes(g *generator) string {
	var codes []int
	for _, d := range g.table.errors.Messages() {
		codes = append(codes, d.Code.Value)
	}
	return fmt.Sprint(codes)
}

func renderedDiagnostics(t *testing.T, g *generator) []string {
	t.Helper()
	var out []string
	for _, d := range g.table.errors.Messages() {
		s, err := d.Render()
		if err != nil {
			t.Fatal(err)
		}
		l := d.Location
		out = append(out, fmt.Sprintf("%d:%d-%d:%d %s", l.BeginLine, l.BeginColumn, l.EndLine, l.EndColumn, s))
	}
	return out
}

func rrNilBody(i int) func(*testing.T, *generator, *moduleNode) {
	return func(t *testing.T, g *generator, m *moduleNode) {
		if rrBody(i)(g, m) != nil {
			t.Fatalf("definition %d body is not null", i)
		}
	}
}
func rrNilOperand(path func(*generator, *moduleNode) exprNode, j int) func(*testing.T, *generator, *moduleNode) {
	return func(t *testing.T, g *generator, m *moduleNode) {
		if path(g, m).(*opApplNode).operands[j] != nil {
			t.Fatalf("operand %d is not null", j)
		}
	}
}

// Unresolved infix, prefix and postfix operators (Generator.java:3080-3139)
// return null after two 4004 diagnostics and allocate nothing; the
// consumers below store the null unchanged. Bootstrap ends at the ModuleNode,
// UID 304, so the first module event is 305.
var unresolvedOperatorCases = []rrCase{
	{label: "infix-definition-body", body: "A == 1 + 2\nB == TRUE", first: 305,
		events: []rrEvent{rrDefEvent(0), rrAppl(rrBody(1)), rrDefEvent(1)}, count: 308, codes: "[4004 4004]", check: rrNilBody(0)},
	{label: "prefix-definition-body", body: "A == -TRUE", first: 305,
		events: []rrEvent{rrDefEvent(0)}, count: 306, codes: "[4004 4004]", check: rrNilBody(0)},
	{label: "postfix-definition-body", body: "A == TRUE^+", first: 305,
		events: []rrEvent{rrDefEvent(0)}, count: 306, codes: "[4004 4004]", check: rrNilBody(0)},
	{label: "builtin-infix-operand", body: "A == TRUE /\\ (1 + 2)", first: 305,
		events: []rrEvent{rrAppl(rrOperand(rrBody(0), 0)), rrAppl(rrBody(0)), rrDefEvent(0)}, count: 308, codes: "[4004 4004]", check: rrNilOperand(rrBody(0), 1)},
	{label: "user-infix-operand", body: "a ++ b == a\nG == TRUE ++ (1 + 2)", first: 305,
		events: []rrEvent{rrParamEvent(0, 0), rrParamEvent(0, 1), rrAppl(rrBody(0)), rrDefEvent(0), rrAppl(rrOperand(rrBody(1), 0)), rrAppl(rrBody(1)), rrDefEvent(1)},
		count:  312, codes: "[4004 4004]", check: rrNilOperand(rrBody(1), 1)},
	{label: "set-enumeration", body: "A == {1 + 2, TRUE}", first: 305,
		events: []rrEvent{rrAppl(rrOperand(rrBody(0), 1)), rrAppl(rrBody(0)), rrDefEvent(0)}, count: 308, codes: "[4004 4004]", check: rrNilOperand(rrBody(0), 0)},
	{label: "tuple", body: "A == <<TRUE, 1 + 2>>", first: 305,
		events: []rrEvent{rrAppl(rrOperand(rrBody(0), 0)), rrAppl(rrBody(0)), rrDefEvent(0)}, count: 308, codes: "[4004 4004]", check: rrNilOperand(rrBody(0), 1)},
	{label: "if-then-else", body: "A == IF 1 + 2 THEN TRUE ELSE FALSE", first: 305,
		events: []rrEvent{rrAppl(rrOperand(rrBody(0), 1)), rrAppl(rrOperand(rrBody(0), 2)), rrAppl(rrBody(0)), rrDefEvent(0)}, count: 309, codes: "[4004 4004]", check: rrNilOperand(rrBody(0), 0)},
	{label: "conjunction-list", body: "A == /\\ 1 + 2\n     /\\ TRUE", first: 305,
		events: []rrEvent{rrAppl(rrOperand(rrBody(0), 1)), rrAppl(rrBody(0)), rrDefEvent(0)}, count: 308, codes: "[4004 4004]", check: rrNilOperand(rrBody(0), 0)},
	{label: "disjunction-list", body: "A == \\/ TRUE\n     \\/ 1 + 2", first: 305,
		events: []rrEvent{rrAppl(rrOperand(rrBody(0), 0)), rrAppl(rrBody(0)), rrDefEvent(0)}, count: 308, codes: "[4004 4004]", check: rrNilOperand(rrBody(0), 1)},
	{label: "set-of-functions", body: "A == [1 + 2 -> TRUE]", first: 305,
		events: []rrEvent{rrAppl(rrOperand(rrBody(0), 1)), rrAppl(rrBody(0)), rrDefEvent(0)}, count: 308, codes: "[4004 4004]", check: rrNilOperand(rrBody(0), 0)},
	{label: "cartesian-product", body: "A == (1 + 2) \\X TRUE", first: 305,
		events: []rrEvent{rrAppl(rrOperand(rrBody(0), 1)), rrAppl(rrBody(0)), rrDefEvent(0)}, count: 308, codes: "[4004 4004]", check: rrNilOperand(rrBody(0), 0)},
	{label: "let-definition-body", body: "A == LET B == 1 + 2 IN TRUE", first: 305,
		events: []rrEvent{
			{"OpDefNode", 5, func(g *generator, m *moduleNode) any { return rrLet(1)(g, m).definitions[0] }},
			rrAppl(func(g *generator, m *moduleNode) exprNode { return rrLet(1)(g, m).body }),
			{"LetInNode", 10, func(g *generator, m *moduleNode) any { return rrLet(1)(g, m) }},
			rrDefEvent(1),
		}, count: 309, codes: "[4004 4004]",
		check: func(t *testing.T, g *generator, m *moduleNode) {
			if rrLet(1)(g, m).definitions[0].(*userOpDefNode).body != nil {
				t.Fatal("LET body is not null")
			}
		}},
	{label: "recursive-completion", body: "RECURSIVE R(_)\nR(x) == x + 1", first: 307,
		events: []rrEvent{rrDefEvent(0), rrParamEvent(0, 0)}, count: 309, codes: "[4004 4004]", check: rrNilBody(0)},
	// Census shapes (RevPostfixBelow, RevLabelPost): the outer operator is
	// resolved first, so nothing beneath it is generated.
	{label: "census-postfix-below-junction", body: "CONSTANT x, y, S\nA == /\\ x\n   ^+", first: 305,
		events: []rrEvent{rrDecl("x", 2), rrDecl("y", 2), rrDecl("S", 2), rrDefEvent(0)}, count: 309, codes: "[4004 4004]", check: rrNilBody(0)},
	{label: "census-unresolved-before-label", body: "CONSTANT x, y, S\nA == 1 + lab :: x^+", first: 305,
		events: []rrEvent{rrDecl("x", 2), rrDecl("y", 2), rrDecl("S", 2), rrDefEvent(0)}, count: 309, codes: "[4004 4004]", check: rrNilBody(0)},
}

func TestHistoryUnresolvedOperatorSchedules(t *testing.T) {
	for _, c := range unresolvedOperatorCases {
		t.Run(c.label, func(t *testing.T) { runResolutionCase(t, c) })
	}
}

// The two diagnostics keep Java's order, locations and raw spelling: the
// GenID node first (finalAppend), then the whole expression. Unary minus is
// reported as "-." (Generator.java:266-267).
func TestHistoryUnresolvedOperatorDiagnostics(t *testing.T) {
	for _, c := range []struct {
		body string
		want []string
	}{
		{"A == 1 + 2", []string{
			"2:8-2:8 Could not find declaration or definition of symbol '+'.",
			"2:6-2:10 Couldn't resolve infix operator symbol `+'.",
		}},
		{"A == -TRUE", []string{
			"2:6-2:6 Could not find declaration or definition of symbol '-.'.",
			"2:6-2:10 Couldn't resolve prefix operator symbol `-.'.",
		}},
		{"A == TRUE^+", []string{
			"2:10-2:11 Could not find declaration or definition of symbol '^+'.",
			"2:6-2:11 Couldn't resolve postfix operator symbol `^+'.",
		}},
	} {
		g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\n"+c.body+"\n====\n"), 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := renderedDiagnostics(t, g); fmt.Sprintf("%q", got) != fmt.Sprintf("%q", c.want) {
			t.Errorf("%q:\n got %q\nwant %q", c.body, got, c.want)
		}
	}
}

// Controls: a null result reaching any consumer not proved to store it, and
// the first later unproved producer, keep invalidating with unchanged
// diagnostics.
func TestHistoryUnresolvedOperatorControlsStayUnproved(t *testing.T) {
	for _, c := range []struct {
		body, first, codes string
	}{
		{"A == lab :: (1 + 2)", "unproved operator resolution", "[4004 4004]"},
		{"F(x) == x\nA == F(1 + 2)", "unproved operator resolution", "[4004 4004]"},
		{"F(x) == x\nA == F((1 + 2))", "unproved operator resolution", "[4004 4004]"},
		{"THEOREM 1 + 2", "unproved operator resolution", "[4004 4004]"},
		{"A == LET B == 1 IN 1 + 2", "unproved operator resolution", "[4004 4004]"},
		{"A == {x \\in {} : x + 1}", "unproved operator resolution", "[4004 4004]"},
		{"A == [x \\in {} |-> x + 1]", "unproved operator resolution", "[4004 4004]"},
		{"A == CASE 1 + 2 -> TRUE", "unproved operator resolution", "[4004 4004]"},
		// Census shape P21: the admitted null body is next met by the
		// selector producer's own guard.
		{"A == 1 + 2\nB == A!1!:\nC == A!:!1!:", "unproved general identifier selector", "[4004 4004 4005 4005]"},
		// Sticky: a prior invalidation is not lifted by later admitted shapes.
		{"A == lab :: (1 + 2)\nB == 1 + 2\na \\cup b == a\nCONSTANT f\nf[x \\in {}] == x\nHIDE B", "unproved operator resolution", "[4004 4004 4004 4004 4201 4202 4201 4200 4357]"},
	} {
		g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\n"+c.body+"\n====\n"), 0)
		if err != nil {
			t.Fatal(err)
		}
		h := &g.pool.history
		if h.exact || h.firstUnknown != c.first {
			t.Errorf("%q: exact=%v first=%q", c.body, h.exact, h.firstUnknown)
		}
		if got := diagnosticCodes(g); got != c.codes {
			t.Errorf("%q: codes %s", c.body, got)
		}
	}
}

// processOperator on a name that already resolves (Generator.java:2598-2674):
// the error is reported, the body generated, and one new OpDefNode is
// constructed whose addSymbol declines the binding. The schedule equals a
// fresh definition's.
var existingDefinitionCases = []rrCase{
	{label: "user-redefinition", body: "a ++ b == a\na ++ b == b", first: 305,
		events: []rrEvent{rrParamEvent(0, 0), rrParamEvent(0, 1), rrAppl(rrBody(0)), rrDefEvent(0), rrParamEvent(1, 0), rrParamEvent(1, 1), rrAppl(rrBody(1)), rrDefEvent(1)},
		count:  313, codes: "[4201 4801]",
		check: func(t *testing.T, g *generator, m *moduleNode) {
			if rrSym("++")(g, m) != rrDef(0)(g, m) || len(m.definitions) != 2 {
				t.Fatal("redefinition replaced the binding")
			}
		}},
	{label: "builtin-redefinition", body: "a \\cup b == a", first: 305,
		events: []rrEvent{rrParamEvent(0, 0), rrParamEvent(0, 1), rrAppl(rrBody(0)), rrDefEvent(0)},
		count:  309, codes: "[4201 4202]",
		check: func(t *testing.T, g *generator, m *moduleNode) {
			// \cup's canonical name is \union (Operators.resolveSynonym).
			key := contextKey{name: g.pool.intern("\\union")}
			if _, ok := g.pool.builtins.lookup(key).(*builtinOpDefNode); !ok || rrDef(0)(g, m).name != key.name || m.context.lookup(key) == rrDef(0)(g, m) {
				t.Fatal("builtin binding replaced")
			}
		}},
	{label: "local-then-global", body: "LOCAL F(a) == a\nF(b) == b", first: 305,
		events: []rrEvent{rrParamEvent(0, 0), rrAppl(rrBody(0)), rrDefEvent(0), rrParamEvent(1, 0), rrAppl(rrBody(1)), rrDefEvent(1)},
		count:  311, codes: "[4201 4801]",
		check: func(t *testing.T, g *generator, m *moduleNode) {
			if rrSym("F")(g, m) != rrDef(0)(g, m) || !rrDef(0)(g, m).local || rrDef(1)(g, m).local {
				t.Fatal("LOCAL binding")
			}
		}},
	// The name resolves to the definition's own formal while the parameter
	// context is pushed; after popContext the name is free, so addSymbol
	// binds the new definition (census shape x(x(_)) == "s").
	{label: "own-formal", body: "x(x(_)) == \"s\"", first: 305,
		events: []rrEvent{rrParamEvent(0, 0), {"StringNode", 18, func(g *generator, m *moduleNode) any { return rrBody(0)(g, m) }}, rrDefEvent(0)},
		count:  308, codes: "[4201]",
		check: func(t *testing.T, g *generator, m *moduleNode) {
			if rrSym("x")(g, m) != rrDef(0)(g, m) || rrParam(0, 0)(g, m).arity != 1 {
				t.Fatal("own-formal binding")
			}
		}},
	{label: "constant", body: "CONSTANT C\nC == 1", first: 305,
		events: []rrEvent{rrDecl("C", 2), {"NumeralNode", 16, func(g *generator, m *moduleNode) any { return rrBody(0)(g, m) }}, rrDefEvent(0)},
		count:  308, codes: "[4201 4201]",
		check: func(t *testing.T, g *generator, m *moduleNode) {
			if _, ok := rrSym("C")(g, m).(*opDeclNode); !ok {
				t.Fatal("declaration binding replaced")
			}
		}},
	{label: "theorem", body: "THEOREM T == TRUE\nT == 1", first: 308,
		events: []rrEvent{{"NumeralNode", 16, func(g *generator, m *moduleNode) any { return rrBody(1)(g, m) }}, rrDefEvent(1)},
		count:  310, codes: "[4201 4201]",
		check: func(t *testing.T, g *generator, m *moduleNode) {
			if _, ok := rrSym("T")(g, m).(*thmOrAssumpDefNode); !ok {
				t.Fatal("theorem binding replaced")
			}
		}},
	// A RECURSIVE placeholder seen from another LET level (2639-2644):
	// 4293, odn = null, a new OpDefNode whose addSymbol keeps the
	// placeholder (4801). The IN expression applies the placeholder, which
	// the module-level definition later completes without a new node.
	{label: "recursive-wrong-level", body: "RECURSIVE F(_)\nG == LET F(x) == x IN F(TRUE)\nF(x) == x", first: 305,
		// LET definitions are also module definitions: [LET F, G, F].
		events: []rrEvent{
			{"OpDeclNode", 2, nil},       // buildParameter, not in a table
			{"FormalParamNode", 11, nil}, // placeholder formal, replaced by setParams
			rrDefEvent(2),
			rrParamEvent(0, 0), rrAppl(rrBody(0)), rrDefEvent(0),
			rrAppl(func(g *generator, m *moduleNode) exprNode {
				return rrLet(1)(g, m).body.(*opApplNode).operands[0].(exprNode)
			}),
			rrAppl(func(g *generator, m *moduleNode) exprNode { return rrLet(1)(g, m).body }),
			{"LetInNode", 10, func(g *generator, m *moduleNode) any { return rrLet(1)(g, m) }},
			rrDefEvent(1),
			rrParamEvent(2, 0), rrAppl(rrBody(2)),
		},
		count: 317, codes: "[4293 4801]",
		check: func(t *testing.T, g *generator, m *moduleNode) {
			placeholder := rrDef(2)(g, m)
			if rrLet(1)(g, m).body.(*opApplNode).operator != placeholder || rrSym("F")(g, m) != placeholder || !placeholder.defined || rrLet(1)(g, m).definitions[0] != rrDef(0)(g, m) {
				t.Fatal("wrong-level binding")
			}
		}},
	// Error order: processOperator's 4201 precedes the body's diagnostics;
	// addSymbol's 4201 follows them.
	{label: "error-order", body: "CONSTANT C\nC == 1 + 2", first: 305,
		events: []rrEvent{rrDecl("C", 2), rrDefEvent(0)}, count: 307, codes: "[4201 4004 4004 4201]", check: rrNilBody(0)},
	// A LET definition of a module-level name: 4201, a new OpDefNode that
	// addSymbol declines (4801); IN applies the module definition. LET
	// definitions are also module definitions: [F, LET F, G].
	{label: "let-shadow", body: "F == 1\nG == LET F == 2 IN F", first: 305,
		events: []rrEvent{
			{"NumeralNode", 16, func(g *generator, m *moduleNode) any { return rrBody(0)(g, m) }}, rrDefEvent(0),
			{"NumeralNode", 16, func(g *generator, m *moduleNode) any { return rrBody(1)(g, m) }}, rrDefEvent(1),
			rrAppl(func(g *generator, m *moduleNode) exprNode { return rrLet(2)(g, m).body }),
			{"LetInNode", 10, func(g *generator, m *moduleNode) any { return rrLet(2)(g, m) }},
			rrDefEvent(2),
		},
		count: 312, codes: "[4201 4801]",
		check: func(t *testing.T, g *generator, m *moduleNode) {
			if rrLet(2)(g, m).body.(*opApplNode).operator != rrDef(0)(g, m) || rrSym("F")(g, m) != rrDef(0)(g, m) {
				t.Fatal("LET shadow binding")
			}
		}},
}

func TestHistoryExistingDefinitionSchedules(t *testing.T) {
	for _, c := range existingDefinitionCases {
		t.Run(c.label, func(t *testing.T) { runResolutionCase(t, c) })
	}
}

// processFunction on a name that already resolves (Generator.java:2791-2912):
// formals, the self FormalParamNode without a table, the function
// application, then no OpDefNode. A non-OpDefNode target appends nothing and
// does not push the parameter context, so the body's x is unknown (4200).
var existingFunctionCases = []rrCase{
	{label: "bound-formal", body: "f[f \\in {TRUE}] == TRUE\nG == TRUE", first: 305, count: 313, codes: "[4201]",
		check: func(t *testing.T, g *generator, m *moduleNode) {
			if len(m.definitions) != 1 || rrDef(0)(g, m).name.text != "G" || rrSym("f")(g, m) != nil {
				t.Fatal("function collision appended or bound")
			}
		}},
	{label: "constant", body: "CONSTANT f\nf[x \\in {TRUE}] == x", first: 305, count: 311, codes: "[4201 4200]",
		events: []rrEvent{rrDecl("f", 2)},
		check: func(t *testing.T, g *generator, m *moduleNode) {
			if len(m.definitions) != 0 {
				t.Fatal("function collision appended")
			}
		}},
	{label: "theorem", body: "THEOREM f == TRUE\nf[x \\in {TRUE}] == TRUE", first: 305, count: 314, codes: "[4201]"},
	// Error order: bounds first, then the function-name error, then the body.
	// The null body is stored by setArgs.
	{label: "error-order", body: "CONSTANT f\nf[x \\in {1 + 2}] == 1 + 2", first: 305, count: 310, codes: "[4004 4004 4201 4004 4004]",
		events: []rrEvent{rrDecl("f", 2)}},
	{label: "null-body", body: "f[x \\in {TRUE}] == 1 + 2", first: 305, count: 311, codes: "[4004 4004]",
		events: []rrEvent{
			{"OpApplNode", 9, func(g *generator, m *moduleNode) any {
				return rrBody(0)(g, m).(*opApplNode).ranges[0].(*opApplNode).operands[0]
			}},
			{"OpApplNode", 9, func(g *generator, m *moduleNode) any { return rrBody(0)(g, m).(*opApplNode).ranges[0] }},
			{"FormalParamNode", 11, func(g *generator, m *moduleNode) any { return rrBody(0)(g, m).(*opApplNode).bounded[0][0] }},
			{"FormalParamNode", 11, nil},
			{"OpApplNode", 9, func(g *generator, m *moduleNode) any { return rrBody(0)(g, m) }},
			rrDefEvent(0),
		},
		check: func(t *testing.T, g *generator, m *moduleNode) {
			if rrBody(0)(g, m).(*opApplNode).operands[0] != nil {
				t.Fatal("function body is not null")
			}
		}},
	// Census shapes from the grammar-sentence corpus.
	{label: "census-if-domain", body: "x [ x \\in IF \"s\" THEN \"s\" ELSE \"s\" ] == \"s\"", first: 305, count: 313, codes: "[4201]"},
	{label: "census-tuple-domain", body: "x [ << x , x >> \\in \"s\" ] == \"s\"", first: 305, count: 311, codes: "[4201 4201]"},
}

func TestHistoryExistingFunctionSchedules(t *testing.T) {
	for _, c := range existingFunctionCases {
		t.Run(c.label, func(t *testing.T) { runResolutionCase(t, c) })
	}
	// The bound-formal schedule: bounds before formals (processQuantBoundArgs,
	// Generator.java:2718-2741), the self formal (308, unreachable after
	// makeNonRecursive), the function application, the body, then G.
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nf[f \\in {TRUE}] == TRUE\nG == TRUE\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	var spec *opApplNode
	for node, ticket := range h.tickets {
		if app, ok := node.(*opApplNode); ok && ticket.uid >= 305 && app.operator != nil && app.operator.header().name.text == "$NonRecursiveFcnSpec" {
			spec = app
		}
	}
	if spec == nil || len(spec.bounded) != 1 || len(spec.bounded[0]) != 1 || len(spec.ranges) != 1 {
		t.Fatal("function application not found")
	}
	bound, domain := spec.bounded[0][0], spec.ranges[0].(*opApplNode)
	want := []struct {
		node  any
		class string
		uid   uint32
	}{
		{domain.operands[0], "OpApplNode", 305}, {domain, "OpApplNode", 306}, {bound, "FormalParamNode", 307},
		{spec, "OpApplNode", 309}, {spec.operands[0], "OpApplNode", 310},
		{rrBody(0)(g, m), "OpApplNode", 311}, {rrDef(0)(g, m), "OpDefNode", 312},
	}
	for i, w := range want {
		got := h.reuse(w.node)
		if !got.known || got.class != w.class || got.uid != w.uid {
			t.Fatalf("bound-formal event %d: %#v", i, got)
		}
	}
	if spec.unbounded != nil {
		t.Fatal("non-recursive self formal retained")
	}
}

// Module-level USE and HIDE (Generator.java:2285-2300) reach the shared
// generateUseOrHide producer; checkIfInRecursiveSection, factCheck, the
// empty-statement check and addTopLevel allocate nothing.
var moduleUseOrHideCases = []rrCase{
	{label: "census-hide-fact", body: "CONSTANT x, y, S\nHIDE x", first: 305,
		events: []rrEvent{rrDecl("x", 2), rrDecl("y", 2), rrDecl("S", 2),
			{"OpApplNode", 9, func(g *generator, m *moduleNode) any { return rrTop(0)(g, m).facts[0] }},
			{"UseOrHideNode", 32, func(g *generator, m *moduleNode) any { return rrTop(0)(g, m) }}},
		count: 310, codes: "[4357]"},
	{label: "census-hide-def", body: "CONSTANT x, y, S\nHIDE DEF x", first: 308,
		events: []rrEvent{{"UseOrHideNode", 32, func(g *generator, m *moduleNode) any { return rrTop(0)(g, m) }}},
		count:  309, codes: "[4200 4003]"},
	{label: "use-def", body: "F == TRUE\nUSE DEF F", first: 305,
		events: []rrEvent{rrAppl(rrBody(0)), rrDefEvent(0), {"UseOrHideNode", 31, func(g *generator, m *moduleNode) any { return rrTop(0)(g, m) }}},
		count:  308, codes: "[]",
		check: func(t *testing.T, g *generator, m *moduleNode) {
			if rrTop(0)(g, m).defs[0] != rrDef(0)(g, m) {
				t.Fatal("DEF entry")
			}
		}},
	{label: "use-fact", body: "F == TRUE\nUSE F", first: 307,
		events: []rrEvent{
			{"OpApplNode", 9, func(g *generator, m *moduleNode) any { return rrTop(0)(g, m).facts[0] }},
			{"UseOrHideNode", 31, func(g *generator, m *moduleNode) any { return rrTop(0)(g, m) }}},
		count: 309, codes: "[]"},
	{label: "use-module", body: "USE MODULE M", first: 305,
		events: []rrEvent{{"UseOrHideNode", 31, func(g *generator, m *moduleNode) any { return rrTop(0)(g, m) }}},
		count:  306, codes: "[]",
		check: func(t *testing.T, g *generator, m *moduleNode) {
			if rrTop(0)(g, m).facts[0] != m {
				t.Fatal("current-module fact")
			}
		}},
	{label: "empty-hide", body: "HIDE", first: 305,
		events: []rrEvent{{"UseOrHideNode", 32, func(g *generator, m *moduleNode) any { return rrTop(0)(g, m) }}},
		count:  306, codes: "[4003 4003]"},
	{label: "recursive-section", body: "RECURSIVE F\nUSE TRUE\nF == TRUE", first: 307,
		events: []rrEvent{
			{"OpApplNode", 9, func(g *generator, m *moduleNode) any { return rrTop(0)(g, m).facts[0] }},
			{"UseOrHideNode", 31, func(g *generator, m *moduleNode) any { return rrTop(0)(g, m) }},
			rrAppl(rrBody(0))},
		count: 310, codes: "[4294]"},
}

func TestHistoryModuleUseOrHideSchedules(t *testing.T) {
	for _, c := range moduleUseOrHideCases {
		t.Run(c.label, func(t *testing.T) { runResolutionCase(t, c) })
	}
	// Census shape FactInst: an inner module, its instance, BY/USE/HIDE of
	// the instance name.
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\n---- MODULE In ----\nFoo == 1\n====\nI == INSTANCE In\nTHEOREM T1 == TRUE\nBY I\nUSE I\nHIDE I\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	if !h.exact || h.count != 319 || diagnosticCodes(g) != "[]" || len(m.topLevel) != 4 {
		t.Fatalf("FactInst: exact=%v count=%d first=%q codes=%s top=%d", h.exact, h.count, h.firstUnknown, diagnosticCodes(g), len(m.topLevel))
	}
	use, hide := m.topLevel[2].(*useOrHideNode), m.topLevel[3].(*useOrHideNode)
	if h.reuse(use).uid != 317 || h.reuse(hide).uid != 318 || use.hide || !hide.hide {
		t.Fatalf("FactInst USE/HIDE: %#v %#v", h.reuse(use), h.reuse(hide))
	}
}

// A null body admitted above, instantiated with substitutions: SubstInNode's
// constructor takes its UID and then reports the substitution error
// (SubstInNode.java:69-81); the wrapper precedes the copied OpDefNode.
func TestHistoryUnresolvedBodyInstanceSubstitution(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\n---- MODULE In ----\nCONSTANT c\nFoo == c + 1\n====\nI == INSTANCE In WITH c <- TRUE\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	copied, ok := rrSym("I!Foo")(g, m).(*userOpDefNode)
	if !ok {
		t.Fatal("instance copy not bound")
	}
	wrapper, ok := copied.body.(*substInNode)
	if !h.exact || !ok || wrapper.body != nil || h.reuse(copied).uid != h.reuse(wrapper).uid+1 || !h.reuse(wrapper).known {
		t.Fatalf("exact=%v first=%q copy=%#v wrapper=%#v", h.exact, h.firstUnknown, h.reuse(copied), h.reuse(wrapper))
	}
	if got := diagnosticCodes(g); got != "[4004 4004 4003]" {
		t.Fatalf("codes %s", got)
	}
}

// Predicates: only the named classes, known identities, admitted consumers
// and kinds pass.
func TestResolutionHistoryPredicates(t *testing.T) {
	p := newNamePool()
	exact := p.intern("+")
	if !unresolvedOperatorHistoryAdmitted(nil, nil, true, exact) {
		t.Fatal("admitted unresolved operator")
	}
	lossy := *exact
	lossy.displayExact = false
	for _, c := range []struct {
		err      error
		op       leafNode
		consumer bool
		raw      *name
	}{
		{errors.New("x"), nil, true, exact},
		{nil, &userOpDefNode{}, true, exact},
		{nil, nil, false, exact},
		{nil, nil, true, &lossy},
		{nil, nil, true, nil},
	} {
		if unresolvedOperatorHistoryAdmitted(c.err, c.op, c.consumer, c.raw) {
			t.Errorf("admitted %+v", c)
		}
	}
	for _, n := range []leafNode{&userOpDefNode{}, &builtinOpDefNode{}, &formalParamNode{}, &opDeclNode{}, &thmOrAssumpDefNode{}} {
		if !existingDefinitionHistoryAdmitted(n, true) || existingDefinitionHistoryAdmitted(n, false) {
			t.Errorf("definition predicate %T", n)
		}
	}
	if existingDefinitionHistoryAdmitted(&moduleNode{}, true) || existingDefinitionHistoryAdmitted(nil, true) {
		t.Error("definition predicate admitted an unproved class")
	}
	for _, n := range []leafNode{&userOpDefNode{}, &formalParamNode{}, &opDeclNode{}, &thmOrAssumpDefNode{}} {
		if !existingFunctionHistoryAdmitted(n, true) || existingFunctionHistoryAdmitted(n, false) {
			t.Errorf("function predicate %T", n)
		}
	}
	if existingFunctionHistoryAdmitted(&builtinOpDefNode{}, true) || existingFunctionHistoryAdmitted(&moduleNode{}, true) || existingFunctionHistoryAdmitted(nil, true) {
		t.Error("function predicate admitted an unproved class")
	}
	if !moduleBodyHistoryAdmitted(generated.N_UseOrHide) || moduleBodyHistoryAdmitted(generated.N_Proof) || moduleBodyHistoryAdmitted(generated.N_OperatorDefinition) || moduleBodyHistoryAdmitted(0) {
		t.Error("module body predicate")
	}
}

// The null-consumer mark is consumed by the next expression call, including
// one that stops at the depth limit, and never leaks to a later sibling.
func TestNullResultConsumerIsConsumed(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nA == TRUE\n====\n"), 0)
	if err != nil || m == nil {
		t.Fatal(err)
	}
	if g.nullResultConsumer {
		t.Fatal("mark survived generation")
	}
	tree := parsedModule(t, "---- MODULE M ----\nA == 1 + 2\n====\n")
	g, _, err = analyzeOwned(stdcontext.Background(), tree, 1)
	if g == nil || err == nil || g.pool.history.exact || g.nullResultConsumer {
		t.Fatalf("depth-limited run: err=%v exact=%v mark=%v", err, g != nil && g.pool.history.exact, g != nil && g.nullResultConsumer)
	}
}

// Countdown cancellation over a source exercising every admitted path: an
// aborted run never stays exact or holds a ticket at or beyond its count.
func TestResolutionHistoryCancellation(t *testing.T) {
	src := parsedModule(t, "---- MODULE M ----\nCONSTANT C\nC == 1 + 2\nA == TRUE /\\ (1 + 2)\na \\cup b == a\nf[f \\in {TRUE}] == TRUE\nHIDE C\n====\n")
	finished := false
	for n := 0; n < 400 && !finished; n++ {
		left := n
		ctx := countdownContext{stdcontext.Background(), &left}
		g, m, err := analyzeOwned(ctx, src, 0)
		if g == nil {
			continue
		}
		h := &g.pool.history
		if err == nil && m != nil && !h.exact {
			if h.firstUnknown != "cancelled" {
				t.Fatalf("countdown %d: completed run first=%q", n, h.firstUnknown)
			}
			continue
		}
		if err == nil && m != nil {
			if h.count != 322 {
				t.Fatalf("countdown %d finished: count=%d", n, h.count)
			}
			finished = true
			continue
		}
		if h.exact {
			t.Fatalf("countdown %d: aborted run still exact", n)
		}
		for node, ticket := range h.tickets {
			if uint64(ticket.uid) >= h.count {
				t.Fatalf("countdown %d: ticket %d beyond count %d for %T", n, ticket.uid, h.count, node)
			}
		}
	}
	if !finished {
		t.Fatal("countdown never finished")
	}
}
