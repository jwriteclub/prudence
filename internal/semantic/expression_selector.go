// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ Generator.java, Copyright 2003 Compaq Corporation and
// Microsoft Corporation, under the MIT license; see LICENSE.
package semantic

import (
	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

// Source-derived expression and selector branches, pinned to Generator.java
// at dc6a944. Each helper names the Java lines it follows; shapes outside
// them keep their caller's explicit incomplete boundary.

// unsupportedExpression is the default case of generateExpressionOrLAP
// (Generator.java:3496-3505): it reports INTERNAL_ERROR (4003, recorded, not
// thrown) at the node and returns null without allocating a SemanticNode.
// The parser reaches it with a bare token, for example the "[" of a function
// application whose function expression is missing.
func (g *generator) unsupportedExpression(tree *generated.SyntaxTreeNode) (exprNode, error) {
	if tree.Image() == nil {
		return nil, g.unported("generator.construct.expression-kind")
	}
	image, e := g.name(tree)
	if e != nil {
		return nil, e
	}
	loc, e := g.diagnosticLocation(tree)
	if e != nil {
		return nil, e
	}
	if e = addError(g.table.errors, 4003, loc, "Unsupported expression type `%s'.", nameStringParam(image)); e != nil {
		return nil, e
	}
	return nil, nil
}

// selectorOperatorKind returns the Java node kind that selectorToNode's
// FindingSubExpr branch tests for the operator of an OpApplNode
// (Generator.java:1162-1180). Only node types whose kind is carried exactly
// in their header are mapped.
func selectorOperatorKind(op leafNode) (int, bool) {
	switch v := op.(type) {
	case *builtinOpDefNode:
		return v.kind, true
	case *userOpDefNode:
		return v.kind, true
	case *formalParamNode:
		return v.kind, true
	case *opDeclNode:
		return v.kind, true
	case *thmOrAssumpDefNode:
		return v.kind, true
	}
	return 0, false
}

// selectsOperandByArity is the first test of that branch: a FormalParamKind,
// ConstantDeclKind or UserDefinedOpKind operator selects an operand by
// ArgNum over the operator's arity (Generator.java:1166-1177).
func selectsOperandByArity(kind int) bool {
	return kind == kindFormal || kind == 2 || kind == 5
}

// noOperandSelection is the final else of that branch (Generator.java:
// 1394-1400): an operator of any other non-built-in kind, such as the null
// OpDefNode (kind 0) of nullOAN, a VARIABLE, or a proof step, has no
// selectable operand. It reports and returns nullOAN without allocating.
func (g *generator) noOperandSelection(selector *generated.SyntaxTreeNode) (exprNode, error) {
	text, e := selectorItemText(selector)
	if e != nil {
		return nil, e
	}
	loc, e := g.diagnosticLocation(selector)
	if e != nil {
		return nil, e
	}
	if e = addError(g.table.errors, 4005, loc, "Choosing operand `%s' of subexpression with no operands.", asciiStringParam(text)); e != nil {
		return nil, e
	}
	return g.nullOAN, nil
}

// selectorItemText is selectorItemToString (Generator.java:2099-2108):
// "!(...)" for an OpArgs selector, "argument selector n" for a numbered
// one, and otherwise the selector's own image.
func selectorItemText(selector *generated.SyntaxTreeNode) (string, error) {
	if selector.Kind() == generated.N_OpArgs {
		return "!(...)", nil
	}
	if len(selector.Heirs()) == 0 && selector.Image() != nil {
		return selector.Image().String(), nil
	}
	_, rendered, err := structuralIndex(selector, int(^uint(0)>>1))
	return rendered, err
}

// letSelectorName is the name a NameSel selector looks up in a LET context:
// Selector.finish classifies IDENTIFIER and the four operator-token kinds as
// NameSel (Generator.java:409-418) and FindingOpName resolves the synonym
// before the lookup (Generator.java:735-738, then 777). Proof-step lexemes
// are NameSel too but cannot name a LET definition; they stay outside.
func (g *generator) letSelectorName(n *generated.SyntaxTreeNode) (*name, bool, error) {
	switch n.Kind() {
	case 289: // IDENTIFIER
		raw, e := g.name(n)
		return raw, e == nil, e
	case generated.N_InfixOp, generated.N_NonExpPrefixOp, generated.N_PostfixOp, generated.N_PrefixOp:
		raw, e := g.name(n)
		if e != nil {
			return nil, false, e
		}
		canonical, e := g.canonicalName(raw)
		if e != nil {
			return nil, false, g.unported("generator.precondition.synonym-lookup")
		}
		return canonical, true, nil
	}
	return nil, false, nil
}

// isLetSelectorName reports whether n is a selector that letSelectorName
// accepts.
func isLetSelectorName(n *generated.SyntaxTreeNode) bool {
	switch n.Kind() {
	case 289, generated.N_InfixOp, generated.N_NonExpPrefixOp, generated.N_PostfixOp, generated.N_PrefixOp:
		return true
	}
	return false
}

// isColonSelector reports a "!:" selector (ColonSel, Generator.java:438).
func isColonSelector(n *generated.SyntaxTreeNode) bool {
	return n.Kind() == generated.N_StructOp && len(n.Heirs()) == 0 && n.Image() != nil && n.Image().String() == ":"
}
