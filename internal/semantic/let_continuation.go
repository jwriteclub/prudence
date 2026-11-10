// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ Generator.java, Copyright 2003 Compaq Corporation and
// Microsoft Corporation, under the MIT license; see LICENSE.
package semantic

import (
	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

// The shared FindingOpName exit of selectorToNode, pinned to Generator.java
// at dc6a944. The root operator and every LET-local operator that is not the
// last selector leave FindingOpName through the same state (982-1035): one
// params vector, one allArgs vector and one substInPrefix, with subExprOf
// fixed at the root (975-978). selectorFrame carries that state and
// enterLetOperator is the LET-local transition, so the existing label,
// structural and LET-name continuations run unchanged after either.

// labelSelection is one selector after the root: its syntax node and the
// OpArgs written after it, if any.
type labelSelection struct{ node, args *generated.SyntaxTreeNode }

// selectorFrame is selectorToNode's state at a FindingOpName exit.
type selectorFrame struct {
	subExpr leafNode           // subExprOf (975-978); LET steps never change it
	op      *userOpDefNode     // curSymbolNode; its labels serve FollowingLabels (1064)
	body    exprNode           // getBody() after selectorBody pushed SubstIn prefixes (1015-1026)
	params  []*formalParamNode // Java params; every frame owns a fresh slice
	args    []exprOrOpArg      // Java allArgs; every frame owns a fresh slice
	let     bool               // entered through enterLetOperator, not the root
}

// nonFinalLetName reports whether a NameSel that is not the last selector
// occurs in rest: the shape that needs the LET-local FindingOpName
// transition rather than a final LET-name terminal.
func nonFinalLetName(rest []labelSelection) bool {
	for i := 0; i < len(rest)-1; i++ {
		if isLetSelectorName(rest[i].node) {
			return true
		}
	}
	return false
}

// hasLetName reports whether any selector in items is a NameSel.
func hasLetName(items []labelSelection) bool {
	for _, item := range items {
		if isLetSelectorName(item.node) {
			return true
		}
	}
	return false
}

// selectorEffects is a snapshot of every effect a selector path can make
// observable before it gives up: history tickets and invalidation, recorded
// diagnostics, name interning and semantic-node identities.
type selectorEffects struct {
	exact             bool
	first             string
	count, names, ids uint64
	messages          int
}

func (g *generator) selectorEffects() selectorEffects {
	m := selectorEffects{exact: g.pool.history.exact, first: g.pool.history.firstUnknown, count: g.pool.history.count, names: g.pool.next, ids: g.pool.nextNode}
	if g.table.errors != nil {
		m.messages = len(g.table.errors.messages)
	}
	return m
}

// typedSelectorFailure reports whether e is one of the source diagnostics a
// selector traversal returns instead of reporting it directly.
func typedSelectorFailure(e error) bool {
	switch e.(type) {
	case *selectorTraversalError, *selectorSuppliedArgumentError, *selectorNullError, *selectorArgumentMismatch:
		return true
	}
	return false
}

// rejectSelectorFailure reports a typed traversal failure, in the order the
// existing callers dispatch them.
func (g *generator) rejectSelectorFailure(e error) (exprNode, error) {
	switch failure := e.(type) {
	case *selectorTraversalError:
		return g.rejectSelectorTraversal(failure)
	case *selectorSuppliedArgumentError:
		return g.rejectSelectorArgumentsInOperatorMode(failure.tree)
	case *selectorNullError:
		return g.nullSelected(failure.tree)
	case *selectorArgumentMismatch:
		return g.rejectSelectorArguments(failure)
	}
	return nil, e
}

// boundSelectorArguments generates the expressions of the bound selectors a
// traversal reached, in selector order. Java generates each one at its own
// FindingSubExpr index (Generator.java:1349-1373); the traversal and the LET
// lookup between those indices and this call record no diagnostic, allocate
// no SemanticNode and touch no substitution prefix, so generating them here
// yields the same schedule. An argument that aborts stops the sequence.
func (g *generator) boundSelectorArguments(trees []*generated.SyntaxTreeNode, depth int) ([]exprOrOpArg, error) {
	args := make([]exprOrOpArg, 0, len(trees))
	for _, syntax := range trees {
		arg, err := g.expression(syntax, depth+1)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}
	return args, nil
}

// rejectAfterBoundArguments reports a typed traversal failure after first
// generating the bound arguments Java had already generated at earlier
// indices (Generator.java:1371, then the failing check).
func (g *generator) rejectAfterBoundArguments(e error, trees []*generated.SyntaxTreeNode, depth int) (exprNode, error) {
	if _, err := g.boundSelectorArguments(trees, depth); err != nil {
		return nil, err
	}
	return g.rejectSelectorFailure(e)
}

// colonLetTarget follows !:!name from curNode start. The colon is admitted
// because it precedes a NameSel (Generator.java:1128-1136) and leaves
// curNode unchanged; isNullSelection then tests it (1544-1546). Nested
// labels are unwrapped with the same test and the result must be a LET/IN,
// whose context the name is looked up in (1549-1566, then FindingOpName).
// No SemanticNode is allocated.
func (g *generator) colonLetTarget(start levelNode, colon, last *generated.SyntaxTreeNode, depth int) (*userOpDefNode, error) {
	current := start
	for labelDepth := depth + 1; ; labelDepth++ {
		if current == nil {
			return nil, &selectorNullError{tree: colon}
		}
		label, ok := current.(*labelNode)
		if !ok {
			break
		}
		if e := g.check(labelDepth); e != nil {
			return nil, e
		}
		current = label.body
	}
	let, ok := current.(*letInNode)
	if !ok {
		return nil, &selectorTraversalError{tree: colon, message: "A name selector here must be from a LET clause."}
	}
	return g.letLocalTarget(let, last, false)
}

// frameSegment is a FindingSubExpr run from curNode start, then either the
// switch into a LET context or the end of the selector. afterOp is true when
// start is the body of the frame's operator (prevMode FindingOpName) and
// false when start is a selected label (prevMode FollowingLabels).
func (g *generator) frameSegment(tree *generated.SyntaxTreeNode, f selectorFrame, start exprNode, afterOp bool, rest []labelSelection, fact bool, depth int) (exprNode, error) {
	k := -1
	for i, item := range rest {
		if isLetSelectorName(item.node) { // NameSel, Generator.java:409-418
			k = i
			break
		}
	}
	if k < 0 {
		structural := make([]*generated.SyntaxTreeNode, 0, len(rest))
		for _, item := range rest {
			if (item.node.Kind() != generated.N_StructOp && item.node.Kind() != generated.N_OpArgs) || (item.args != nil && item.node.Kind() != generated.N_OpArgs) {
				return nil, g.unported("generator.construct.simple-label-selector-mode")
			}
			structural = append(structural, item.node)
		}
		return g.numericExpression(tree, structural, f.subExpr, start, f.params, f.args, depth, afterOp)
	}
	selectors := make([]*generated.SyntaxTreeNode, 0, k)
	for _, item := range rest[:k] {
		if (item.node.Kind() != generated.N_StructOp && item.node.Kind() != generated.N_OpArgs) || item.args != nil {
			return nil, g.unported("generator.construct.simple-label-selector-mode")
		}
		selectors = append(selectors, item.node)
	}
	final := k == len(rest)-1
	if !final && fact {
		// Fact mode keeps its guard: the non-final continuation is
		// source-proved and tested in expression mode only.
		return nil, g.unported("generator.construct.simple-label-selector-mode")
	}
	named := rest[k]
	var bound []*formalParamNode
	var boundArgs []*generated.SyntaxTreeNode
	var local *userOpDefNode
	var e error
	if len(selectors) == 1 && isColonSelector(selectors[0]) {
		var from levelNode = start
		if label, ok := start.(*labelNode); ok && !afterOp {
			from = label.body // FollowingLabels left curNode at the body (1116-1122)
		}
		local, e = g.colonLetTarget(from, selectors[0], named.node, depth)
	} else {
		local, e = g.structuralLetFrom(start, selectors, named.node, &bound, &boundArgs, depth, false)
	}
	if e != nil {
		if e == ErrPortIncomplete && final {
			// Unchanged final-name boundary: the structural tail the old
			// path fell back to stops at the NameSel with this origin.
			g.incomplete.forget()
			return nil, g.unported("generator.construct.simple-label-selector-mode")
		}
		if failure, ok := e.(*selectorTraversalError); ok && failure.unknown != nil && !final && hasLetName(rest[k+1:]) {
			// Java extends an unknown name by every later NameSel and looks
			// the compound up in the LET context (Generator.java:741-806).
			return nil, g.unported("generator.construct.simple-label-selector-mode")
		}
		if typedSelectorFailure(e) {
			return g.rejectAfterBoundArguments(e, boundArgs, depth)
		}
		return nil, e
	}
	generatedArgs, e := g.boundSelectorArguments(boundArgs, depth)
	if e != nil {
		return nil, e
	}
	params := append(append([]*formalParamNode{}, f.params...), bound...)
	args := append(append([]exprOrOpArg{}, f.args...), generatedArgs...)
	if final {
		return g.selectedLocalExpression(tree, named.node, named.args, f.subExpr, local, params, args, depth)
	}
	next, e := g.enterLetOperator(tree, f, local, named.node, named.args, params, args, depth+k+1)
	if e != nil {
		return nil, e
	}
	if next == nil {
		return g.nullOAN, nil
	}
	return g.frameContinuation(tree, *next, rest[k+1:], fact, depth+k+1)
}

// enterLetOperator is FindingOpName for a LET-local operator that is not the
// last selector (Generator.java:884-1029). The arity check reports before
// any argument is generated (906-914); the arguments are generated in order
// (915-925); the operator's own formals, not clones, join params (991-994);
// its arguments join allArgs (1004-1006); and its body's SubstIn prefixes
// are pushed (1015-1026). A nil frame with a nil error means the source
// diagnostic was recorded and selectorToNode returns nullOAN.
func (g *generator) enterLetOperator(tree *generated.SyntaxTreeNode, f selectorFrame, local *userOpDefNode, named, syntaxArgs *generated.SyntaxTreeNode, params []*formalParamNode, args []exprOrOpArg, depth int) (*selectorFrame, error) {
	if local.kind != 5 || !local.defined || len(local.params) != local.arity {
		// A LET-local INSTANCE or an operator used inside its own
		// definition (962-969) stays outside this transition.
		return nil, g.unported("generator.construct.simple-label-selector-mode")
	}
	var trees []*generated.SyntaxTreeNode
	if syntaxArgs != nil {
		parts := syntaxArgs.Heirs()
		for i := 1; i < len(parts)-1; i += 2 {
			trees = append(trees, parts[i])
		}
	}
	if len(trees) != local.arity {
		location := tree
		if syntaxArgs != nil {
			location = syntaxArgs
		}
		if _, e := g.badArity(local.name, local.arity, location); e != nil {
			return nil, e
		}
		return nil, nil
	}
	localArgs := make([]exprOrOpArg, len(trees))
	for i, arg := range trees {
		var e error
		localArgs[i], e = g.expressionOrOperatorArgument(arg, local.params[i].arity, i, local, named, depth+1)
		if e != nil {
			return nil, e
		}
	}
	body, e := g.selectorBody(local.body, depth+1)
	if e != nil {
		return nil, e
	}
	return &selectorFrame{
		subExpr: f.subExpr,
		op:      local,
		body:    body,
		params:  append(append([]*formalParamNode{}, params...), local.params...),
		args:    append(append([]exprOrOpArg{}, args...), localArgs...),
		let:     true,
	}, nil
}
