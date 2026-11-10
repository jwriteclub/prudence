// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	"strings"
	"testing"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

// Source-derived tests for remaining_recovery.go and the recovery branches
// it serves. Each case runs through the real parser and Generator producers;
// UIDs are javaHistory tickets counted from 305 after the module bootstrap.

func recoveryDecl(t *testing.T, m *moduleNode, text string, nth int) *opDeclNode {
	t.Helper()
	var found []*opDeclNode
	for _, v := range m.context.orderedValues() {
		if d, ok := v.(*opDeclNode); ok && d.name.text == text {
			found = append(found, d)
		}
	}
	if nth >= len(found) {
		t.Fatalf("declaration %q #%d not found (%d)", text, nth, len(found))
	}
	return found[nth]
}

// processVariables reports "@" as a declared variable and still constructs
// and inserts the OpDeclNode (Generator.java:2386-2399). processParameters
// tests the declaration node's image rather than the name (2463), so CONSTANT
// @ is accepted with no diagnostic (buildParameter 2457). The error takes no
// UID; each declaration takes one, also when SymbolTable.addSymbol reports a
// conflict afterwards.
func TestAtDeclarationRecovery(t *testing.T) {
	cases := []esCase{
		{label: "variable", body: "VARIABLE @", exact: true, count: 306,
			diags: []string{"2:10-2:10 Attempted to declare '@' as a variable."},
			check: func(t *testing.T, g *generator, m *moduleNode) {
				d := recoveryDecl(t, m, "@", 0)
				esUID(t, g, d, 305, "OpDeclNode", 3)
			}},
		{label: "constant", body: "CONSTANT @", exact: true, count: 306,
			check: func(t *testing.T, g *generator, m *moduleNode) {
				esUID(t, g, recoveryDecl(t, m, "@", 0), 305, "OpDeclNode", 2)
			}},
		{label: "constant after another", body: "CONSTANT c, @", exact: true, count: 307,
			check: func(t *testing.T, g *generator, m *moduleNode) {
				esUID(t, g, recoveryDecl(t, m, "c", 0), 305, "OpDeclNode", 2)
				esUID(t, g, recoveryDecl(t, m, "@", 0), 306, "OpDeclNode", 2)
			}},
		{label: "variable among others", body: "VARIABLES x, @, y", exact: true, count: 308,
			diags: []string{"2:14-2:14 Attempted to declare '@' as a variable."},
			check: func(t *testing.T, g *generator, m *moduleNode) {
				esUID(t, g, recoveryDecl(t, m, "x", 0), 305, "OpDeclNode", 3)
				esUID(t, g, recoveryDecl(t, m, "@", 0), 306, "OpDeclNode", 3)
				esUID(t, g, recoveryDecl(t, m, "y", 0), 307, "OpDeclNode", 3)
			}},
		// Java (SANY 2.2 on this source): two errors for the variables, a
		// Multiply-defined error for the constant and a warning for the
		// second variable, which follows its own "@" error.
		{label: "redeclared", body: "VARIABLE @\nCONSTANTS c, @\nVARIABLES v, @", exact: true, count: 310,
			diags: []string{
				"2:10-2:10 Attempted to declare '@' as a variable.",
				"3:14-3:14 Multiply-defined symbol '@': this definition or declaration conflicts \nwith the one at line 2, col 10 to line 2, col 10 of module M.",
				"4:14-4:14 Attempted to declare '@' as a variable.",
				"4:14-4:14 Multiple declarations or definitions for symbol @.  \nThis duplicates the one at line 2, col 10 to line 2, col 10 of module M.",
			}},
		// The declaration does not change how "@" is generated: the
		// N_GeneralId test for "@" precedes any lookup (3172-3181).
		{label: "use after constant", body: "CONSTANT @\nop == @", exact: true, count: 307,
			diags: []string{"3:7-3:7 @ used where its meaning is not defined."}},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) { runESCase(t, c) })
	}
}

// generateProof casts the right operand of an infix step to ExprNode
// (Generator.java:6181-6191). An operand whose generation failed after an
// error is null, so prevRHS is null, and a following "@" step stores it in
// its $Nop application (6162-6170): the right side first, then $Nop
// (constructor 3), then the infix application.
func TestProofNullInfixOperand(t *testing.T) {
	steps := func(m *moduleNode) []levelNode {
		return m.theorems[0].proof.(*nonLeafProofNode).steps
	}
	nop := func(t *testing.T, g *generator, m *moduleNode, step int) {
		t.Helper()
		body := steps(m)[step].(*theoremNode).body.(*opApplNode)
		left, right := body.operands[0].(*opApplNode), body.operands[1]
		if len(left.operands) != 1 || left.operands[0] != nil {
			t.Fatalf("$Nop operands %#v, want the stored null", left.operands)
		}
		if !g.pool.history.exact {
			return // an invalidated history attaches no later tickets
		}
		r := g.pool.history.reuse(right)
		esUID(t, g, left, r.uid+1, "OpApplNode", 9)
		esUID(t, g, body, r.uid+2, "OpApplNode", 9)
	}
	cases := []esCase{
		{label: "null operand then at step", body: "THEOREM TRUE\n<1>1. 1 = (2 ++ 3)\n<1>2. @ = 4\n<1> QED",
			exact: true, count: 319,
			diags: []string{
				"3:14-3:15 Could not find declaration or definition of symbol '++'.",
				"3:12-3:17 Couldn't resolve infix operator symbol `++'.",
			},
			check: func(t *testing.T, g *generator, m *moduleNode) { nop(t, g, m, 1) }},
		{label: "null at-step operand then at step", body: "THEOREM TRUE\n<1>1. {1} \\subseteq {2}\n<1>2. @ = 1 ++ 2\n<1>3. @ = 3\n<1> QED",
			first: "unproved operator resolution",
			diags: []string{
				"4:13-4:14 Could not find declaration or definition of symbol '++'.",
				"4:11-4:16 Couldn't resolve infix operator symbol `++'.",
			},
			check: func(t *testing.T, g *generator, m *moduleNode) { nop(t, g, m, 2) }},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) { runESCase(t, c) })
	}
}

// generateUseOrHide passes a DEF entry that is not a GeneralId to
// genIdToSelector (Generator.java:6680-6700, 506-579), then Selector.finish
// (398-475) and selectorToNode with expectedArity -1 (704-760). Every case
// stops at selector index 0 with nullOAN and no SemanticNode.
func TestMalformedProofDefEntries(t *testing.T) {
	cases := []esCase{
		// Infix: the left operand's empty IdPrefix ends the prefix loop; the
		// last selector is the N_GenInfixOp.
		{label: "infix", body: "HIDE DEF x \\cup y", exact: true, count: 306,
			diags: []string{
				"2:10-2:17 Was expecting a GeneralId.",
				"2:12-2:15 Unexpected token found.",
				"2:12-2:15 Need name or step number here, not `N_GenInfixOp'.",
				"2:10-2:17 DEF clause entry should describe a defined operator.",
				"2:1-2:17 Empty USE or HIDE statement.",
			}},
		{label: "record component", body: "HIDE DEF r.f", exact: true, count: 306,
			diags: []string{
				"2:10-2:12 Was expecting a GeneralId.",
				"2:11-2:11 Unexpected token found.",
				"2:11-2:11 Need name or step number here, not `.'.",
				"2:10-2:12 DEF clause entry should describe a defined operator.",
				"2:1-2:12 Empty USE or HIDE statement.",
			}},
		// The inner record component supplies the prefix: its GeneralId
		// yields the IdPrefix selector (a null location), its "." ends the
		// loop, and the outer "." is the last selector.
		{label: "nested record component in BY", body: "THEOREM TRUE BY DEF a.b.c", exact: true, count: 309,
			diags: []string{
				"2:21-2:25 Was expecting a GeneralId.",
				"2147483647:2147483647--2147483648:-2147483648 Unexpected token found.",
				"2:24-2:24 Unexpected token found.",
				"2147483647:2147483647--2147483648:-2147483648 Need name or step number here, not `N_IdPrefix'.",
				"2:21-2:25 DEF clause entry should describe a defined operator.",
				"2:14-2:25 Empty BY",
			}},
		// A tuple's "<<" has no heirs, so the prefix loop is empty and the
		// last selector is the first component.
		{label: "tuple", body: "HIDE DEF <<1, 2>>", exact: true, count: 306,
			diags: []string{
				"2:12-2:12 Unexpected token found.",
				"2:12-2:12 Need name or step number here, not `N_Number'.",
				"2:10-2:17 DEF clause entry should describe a defined operator.",
				"2:1-2:17 Empty USE or HIDE statement.",
			}},
		// ((x \cup y) \cup z) \cup w: the first prefix element x \cup y has
		// three heirs, so x is a selector with arguments, which an
		// expectedArity of -1 rejects first. The two empty IdPrefix selectors
		// report the same message at the same null location, which
		// Errors.addMessage records once (Errors.java:162).
		{label: "selector with arguments", body: "HIDE DEF x \\cup y \\cup z \\cup w", exact: true, count: 306,
			diags: []string{
				"2:10-2:10 Unexpected token found.",
				"2147483647:2147483647--2147483648:-2147483648 Unexpected token found.",
				"2:26-2:29 Unexpected token found.",
				"2:10-2:10 Selector `N_GeneralId' should not have argument(s).",
				"2:10-2:31 DEF clause entry should describe a defined operator.",
				"2:1-2:31 Empty USE or HIDE statement.",
			}},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) { runESCase(t, c) })
	}
}

// A first prefix element with other than two or three heirs aborts with the
// source's Errors abort (Generator.java:550-561) before any later message.
func TestMalformedProofDefAbort(t *testing.T) {
	g, _, err := esAnalyze(t, "HIDE DEF {1, 2} \\cup a \\cup b")
	abort, ok := err.(*Abort)
	if !ok {
		t.Fatalf("err=%v site=%q", err, g.incomplete.site)
	}
	got := renderedDiagnostics(t, g)
	want := []string{"2:10-2:15 Internal error: IdPrefixElement has other than 2 or 3 heirs."}
	if strings.Join(got, "\n") != strings.Join(want, "\n") || abort.Detail.Format != "Internal error: IdPrefixElement has other than 2 or 3 heirs." {
		t.Fatalf("diagnostics %q abort %+v", got, abort.Detail)
	}
}

// processAssumption stores the generated expression unchanged, also when it
// is null after an error (Generator.java:5737-5761; ThmOrAssumpDefNode.java:
// 140-177; ModuleNode.java:611-620; AssumeNode.java:57-68).
func TestAssumeNullExpression(t *testing.T) {
	cases := []esCase{
		{label: "unresolved infix", body: "ASSUME 1 ++ 2", exact: true, count: 306,
			diags: []string{
				"2:10-2:11 Could not find declaration or definition of symbol '++'.",
				"2:8-2:13 Couldn't resolve infix operator symbol `++'.",
			},
			check: func(t *testing.T, g *generator, m *moduleNode) {
				a := m.assumptions[0]
				if a.expression != nil {
					t.Fatalf("expression %T, want the stored null", a.expression)
				}
				esUID(t, g, a, 305, "AssumeNode", 20)
			}},
		{label: "named", body: "ASSUME A == 1 ++ 2", exact: true, count: 307,
			diags: []string{
				"2:15-2:16 Could not find declaration or definition of symbol '++'.",
				"2:13-2:18 Couldn't resolve infix operator symbol `++'.",
			},
			check: func(t *testing.T, g *generator, m *moduleNode) {
				a := m.assumptions[0]
				if a.expression != nil || a.definition == nil || a.definition.body != nil || a.definition.statement != a {
					t.Fatalf("assumption %+v", a)
				}
				esUID(t, g, a.definition, 305, "ThmOrAssumpDefNode", 23)
				esUID(t, g, a, 306, "AssumeNode", 20)
			}},
		{label: "unsupported expression", body: "ASSUME [1]", exact: true, count: 306,
			diags: []string{"2:8-2:8 Unsupported expression type `['."},
			check: func(t *testing.T, g *generator, m *moduleNode) {
				if m.assumptions[0].expression != nil {
					t.Fatal("expression is not the stored null")
				}
			}},
		{label: "axiom", body: "AXIOM 1 ++ 2", exact: true, count: 306,
			diags: []string{
				"2:9-2:10 Could not find declaration or definition of symbol '++'.",
				"2:7-2:12 Couldn't resolve infix operator symbol `++'.",
			},
			check: func(t *testing.T, g *generator, m *moduleNode) {
				if a := m.assumptions[0]; a.expression != nil || !a.axiom {
					t.Fatalf("assumption %+v", a)
				}
			}},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) { runESCase(t, c) })
	}
}

// A null named assumption instantiated with a missing substitution: the
// instance reports its existing errors (here as Java does for this source).
func TestAssumeNullExpressionInstantiated(t *testing.T) {
	src := "---- MODULE Inner ----\nCONSTANT c\nASSUME A == c < 10\n====\n\nLOCAL INSTANCE Inner\nx == TRUE"
	g, _, err := esAnalyze(t, src)
	if err != nil {
		t.Fatalf("err=%v site=%q", err, g.incomplete.site)
	}
	want := []string{
		"4:15-4:15 Could not find declaration or definition of symbol '<'.",
		"4:13-4:18 Couldn't resolve infix operator symbol `<'.",
		"7:1-7:20 Substitution missing for symbol c declared at line 3, col 10 to line 3, col 10 of module M \nand instantiated in module M.",
		"7:1-7:20 Substitution error, probably due to error in module being instantiated.",
	}
	if got := renderedDiagnostics(t, g); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("diagnostics\n got %q\nwant %q", got, want)
	}
}

// Branches that stay port-incomplete: Java throws an unchecked exception
// (WrongInvocationException at Generator.java:7318, a NullPointerException in
// UseOrHideNode.factCheck at UseOrHideNode.java:97, ArrayIndexOutOfBounds in
// genIdToSelector at 516, in processAction at 4165 and in processLetIn at
// 2978 once curLevel reaches 100), or the source returns from generateProof
// with its contexts still pushed (6153-6160).
func TestRemainingRecoveryControlsStayIncomplete(t *testing.T) {
	deep := func(n int) string {
		return "op == " + strings.Repeat("LET a == ", n) + "1" + strings.Repeat(" IN a", n)
	}
	cases := []struct{ label, body, site string }{
		{"recursive overdefinition", "A ==\n  LET RECURSIVE f IN\n  LET f(x) == x IN\n  TRUE", "generator.construct.end-recursive"},
		{"hide null fact", "HIDE X + Y", "generator.construct.proof-fact-check"},
		{"def number entry", "THEOREM TRUE BY DEF 0", "generator.construct.proof-def-selector"},
		// generateProof returns null with its contexts pushed
		// (Generator.java:6159); a later unit could observe them. As the
		// module's last unit the abort is ported (root
		// clean_rejection_test.go).
		{"at step unresolved operator", "THEOREM TRUE\n<1>1 1 = 1\n<1>2 @ \\o 1\n<1> QED\nFoo == 1", "generator.construct.proof-at-expression"},
		{"empty action", "VARIABLE y\nP == <<>>_y", "generator.construct.expression-action-or-fairness"},
		{"LET level 100", deep(100), "generator.construct.let-in"},
		{"LET level 101", deep(101), "generator.construct.let-in"},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			g, _, err := esAnalyze(t, c.body)
			if err != ErrPortIncomplete || g.incomplete.site != c.site {
				t.Fatalf("err=%v site=%q", err, g.incomplete.site)
			}
		})
	}
}

// Ninety-nine nested LETs stay below MaxLetInLevel and generate normally.
func TestLetLevelBelowBound(t *testing.T) {
	src := "op == " + strings.Repeat("LET a == ", 99) + "1" + strings.Repeat(" IN a", 99)
	if g, m, err := esAnalyze(t, src); err != nil || m == nil {
		t.Fatalf("err=%v site=%q", err, g.incomplete.site)
	}
}

// defSelectorNumber admits exactly what Integer.parseInt accepts for an
// unsigned ASCII decimal (Generator.java:424): values up to MaxInt32, leading
// zeros included. The range check runs before the multiply, so a 32-bit int
// cannot wrap a too-large numeral back into range.
func TestDefSelectorNumber(t *testing.T) {
	ctx := generated.NewSupportContext()
	selector := func(text string) *generated.SyntaxTreeNode {
		t.Helper()
		leaf := ctx.NewBuiltinSyntaxTreeNode(ctx.UniqueStringOf(text))
		number, err := ctx.NewNode(nil, generated.N_Number, []*generated.SyntaxTreeNode{leaf})
		if err != nil {
			t.Fatal(err)
		}
		op, err := ctx.NewNode(nil, generated.N_StructOp, []*generated.SyntaxTreeNode{number})
		if err != nil {
			t.Fatal(err)
		}
		return op
	}
	cases := []struct {
		text  string
		value int
		ok    bool
	}{
		{"0", 0, true},
		{"7", 7, true},
		{"42", 42, true},
		{"007", 7, true},
		{"2147483647", 2147483647, true},
		{"02147483647", 2147483647, true},
		{"2147483648", 0, false},
		{"2147483650", 0, false},
		{"4294967296", 0, false},
		{"99999999999999999999", 0, false},
		{"", 0, false},
		{"1a", 0, false},
	}
	for _, c := range cases {
		value, ok := defSelectorNumber(selector(c.text))
		if value != c.value || ok != c.ok {
			t.Errorf("defSelectorNumber(%q) = %d, %v; want %d, %v", c.text, value, ok, c.value, c.ok)
		}
	}
}
