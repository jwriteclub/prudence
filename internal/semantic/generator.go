// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ Generator.java, Copyright 2003 Compaq Corporation and
// Microsoft Corporation, under the MIT license; see LICENSE.
package semantic

import (
	stdcontext "context"
	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/recursion"
	"strconv"
	"unicode/utf16"
)

// generator is request-owned. Unsupported syntax returns ErrPortIncomplete;
// constructing this subset does not assert that semantic checking is complete.
type generator struct {
	ctx                             stdcontext.Context
	pool                            *namePool
	table                           *symbolTable
	maxDepth                        int
	letLevel                        int
	unresolved                      [101]int
	unresolvedSum, recursiveSection int
	functions                       []functionFrame
	excepts                         []exceptFrame
	labelScopes                     []labelFrame
	selectorSubsts                  []*substInNode
	javaLabel                       *generated.SyntaxTreeNode
	javaFormals                     map[*formalParamNode]bool
	nullLabel                       *labelNode
	nullOAN                         *opApplNode
	nullOpArg                       *opArgNode
	apDepth                         int
	inAssume                        *opDeclNode
	apDeclScopes                    []bool
	currentGoal                     *thmOrAssumpDefNode
	goalClause                      int
	incomplete                      incompleteOrigin // private origin telemetry; write-only
	// labelOrderFallbacks counts 4332 extra-parameter lists printed in the
	// stable fallback order (label_extra_order.go); telemetry only.
	labelOrderFallbacks int
	// nullResultConsumer is set by nullableExpression for exactly the next
	// expression call; see resolution_history.go.
	nullResultConsumer bool
	// finalOuterUnit is set while the module loop generates the last
	// non-separator unit of an outermost module. Nothing generated after it
	// can observe the contexts an aborted proof leaves pushed in Java.
	finalOuterUnit bool
}

type labelFrame struct {
	labels   *labelSet
	paramSeq [][]*formalParamNode
}
type exceptFrame struct{ except, component *opApplNode }
type functionFrame struct {
	name        *name
	application *opApplNode
}

func newGenerator(ctx stdcontext.Context, maxDepth int) (*generator, error) {
	if ctx == nil || maxDepth < 0 {
		return nil, ErrPortIncomplete
	}
	return newOwnedGenerator(ctx, maxDepth, newNamePool(), make(map[*name]leafNode), &Errors{})
}

func newOwnedGenerator(ctx stdcontext.Context, maxDepth int, p *namePool, external map[*name]leafNode, errors *Errors) (*generator, error) {
	if ctx == nil || maxDepth < 0 || p == nil || external == nil || errors == nil {
		return nil, ErrPortIncomplete
	}
	p.java.generator()
	nullTree := p.syntax.NewBuiltinSyntaxTreeNode(p.syntax.UniqueStringOf("***I do not exist***"))
	ht := p.history.event("OpDefNode", 0)
	h, e := newNodeHeader(p, p.intern("nullODN"), 0, nullTree)
	if e != nil {
		return nil, e
	}
	nullDef := &userOpDefNode{nodeHeader: h, arity: -2, levels: newLeafLevel()}
	p.history.attach(nullDef, ht)
	ht = p.history.event("OpApplNode", 9)
	eh, e := newExprHeader(p, 9, nullTree)
	if e != nil {
		return nil, e
	}
	nullOAN := &opApplNode{exprHeader: eh, operator: nullDef, operands: make([]exprOrOpArg, 0), ranges: make([]exprNode, 0)}
	p.history.attach(nullOAN, ht)
	ht = p.history.event("OpArgNode", 8)
	oh, e := newSemanticHeader(p, 8, nullTree)
	if e != nil {
		return nil, e
	}
	nullOpArg := &opArgNode{semanticHeader: oh, name: p.intern("nullOpArg"), arity: -2, levels: newLeafLevel()}
	p.history.attach(nullOpArg, ht)
	ht = p.history.event("LabelNode", 29)
	lh, e := newExprHeader(p, 29, nullTree)
	if e != nil {
		return nil, e
	}
	nullLabel := &labelNode{exprHeader: lh, name: p.intern("nullLabelNode"), params: make([]*formalParamNode, 0), body: nullOAN}
	p.history.attach(nullLabel, ht)
	t, e := newTable(p, external, errors)
	if e != nil {
		return nil, e
	}
	if maxDepth == 0 {
		maxDepth = recursion.DefaultMaxDepth
	}
	return &generator{ctx: ctx, pool: p, table: t, maxDepth: maxDepth, nullOAN: nullOAN, nullOpArg: nullOpArg, nullLabel: nullLabel}, nil
}
func (g *generator) check(depth int) error {
	// Propagating errors never re-enter a check, so a recorded origin here
	// belongs to an error an alternative path consumed.
	g.incomplete.forget()
	if e := g.ctx.Err(); e != nil {
		g.pool.history.invalidate("cancelled")
		return e
	}
	if depth > g.maxDepth {
		g.pool.history.invalidate("depth limit")
		return &DepthLimitError{Limit: g.maxDepth}
	}
	return nil
}

// DepthLimitError reports a semantic traversal budget stop.
type DepthLimitError struct{ Limit int }

func (e *DepthLimitError) Error() string        { return "semantic recursion depth limit exceeded" }
func (e *DepthLimitError) Is(target error) bool { return target == ErrPortIncomplete }

func (g *generator) name(n *generated.SyntaxTreeNode) (*name, error) {
	if n == nil || n.Image() == nil {
		return nil, g.unported("generator.precondition.name-image")
	}
	return g.pool.internUnits(n.Image().Units()), nil
}
func (g *generator) expression(tree *generated.SyntaxTreeNode, depth int) (exprNode, error) {
	nullConsumer := g.nullResultConsumer
	g.nullResultConsumer = false
	if e := g.check(depth); e != nil {
		return nil, e
	}
	if tree == nil {
		return nil, g.unported("generator.precondition.nil-expression")
	}
	children := tree.Heirs()
	// Until a source branch is covered, stop certifying before it can allocate.
	switch tree.Kind() {
	case generated.N_Except, generated.N_UnboundQuant, generated.N_UnboundOrBoundChoose, generated.N_SubsetOf, generated.N_LetIn, generated.N_Case, generated.N_FcnAppl, generated.N_BoundQuant, generated.N_SetOfAll, generated.N_FcnConst, generated.N_ActionExpr, generated.N_FairnessExpr, generated.N_RecordComponent, generated.N_RcdConstructor, generated.N_SetOfRcds, generated.N_Number, generated.N_Real, generated.N_String, generated.N_OpApplication, generated.N_ParenExpr, generated.N_InfixExpr, generated.N_PrefixExpr, generated.N_PostfixExpr, generated.N_SetEnumerate, generated.N_Tuple, generated.N_Times, generated.N_IfThenElse, generated.N_ConjList, generated.N_DisjList, generated.N_SetOfFcns:
	case generated.N_GeneralId:
		if len(children) != 2 {
			g.pool.history.invalidate("unproved general identifier")
		} else if len(children[0].Heirs()) != 0 {
			if !g.simpleInstanceSelectorHistory(children) && !g.qualifiedExpressionHistory(children, nil) && !g.simpleLabelSelectorHistory(children, nil) && !g.simpleNumericSelectorHistory(children) && !g.simpleDirectTheoremSelectorHistory(children, nil) {
				g.pool.history.invalidate("unproved general identifier selector")
			}
		} else if children[1].Image() == nil {
			g.pool.history.invalidate("unproved general identifier")
		}
	case generated.N_Label:
	default:
		// The default case allocates nothing and returns null
		// (unsupportedExpression), so only a null-safe consumer keeps history.
		if tree.Kind() == generated.N_Lambda || !nullConsumer {
			g.pool.history.invalidate("unproved expression producer")
		}
	}
	switch tree.Kind() {
	case generated.N_Number:
		if len(children) != 1 || children[0].Image() == nil {
			return nil, g.unported("generator.construct.expression-number")
		}
		ticket := g.pool.history.event("NumeralNode", 16)
		n, err := newNumeral(g.pool, children[0].Image().String(), tree)
		if n != nil {
			g.pool.history.attach(n, ticket)
		}
		return n, err
	case generated.N_Real:
		if len(children) != 3 || children[0].Image() == nil || children[2].Image() == nil {
			return nil, g.unported("generator.construct.expression-decimal")
		}
		ticket := g.pool.history.event("DecimalNode", 17)
		n, err := newDecimal(g.pool, children[0].Image().String(), children[2].Image().String(), tree)
		if n != nil {
			g.pool.history.attach(n, ticket)
		}
		return n, err
	case generated.N_String:
		ticket := g.pool.history.event("StringNode", 18)
		n, err := newString(g.pool, tree, true)
		if n != nil {
			g.pool.history.attach(n, ticket)
		}
		return n, err
	case generated.N_GeneralId:
		if len(children) != 2 {
			return nil, g.unported("generator.construct.expression-general-identifier")
		}
		if len(children[0].Heirs()) != 0 {
			return g.simpleLabelSelector(tree, children, nil, depth)
		}
		if children[1].Kind() == generated.N_IdentifierTuple {
			name, e := g.name(children[1])
			if e != nil {
				return nil, e
			}
			loc, e := g.diagnosticLocation(children[1])
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4003, loc, "Unexpected token found."); e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4005, loc, "Need name or step number here, not `%s'.", nameParam(name)); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		name, e := g.name(children[1])
		if e != nil {
			return nil, e
		}
		if name.text == "@" {
			if len(g.excepts) == 0 {
				loc, e := g.diagnosticLocation(tree)
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4261, loc, "@ used where its meaning is not defined."); e != nil {
					return nil, e
				}
				return g.nullOAN, nil
			}
			frame := g.excepts[len(g.excepts)-1]
			ticket := g.pool.history.event("AtNode", 19)
			h, e := newExprHeader(g.pool, 19, frame.component.tree)
			if e != nil {
				return nil, e
			}
			node := &atNode{exprHeader: h, except: frame.except, component: frame.component}
			g.pool.history.attach(node, ticket)
			return node, nil
		}
		name, e = g.canonicalName(name)
		if e != nil {
			return nil, e
		}
		op, e := g.resolveOperator(name, children[1])
		if e != nil || (op != nil && !g.pool.history.reuse(op).known) {
			g.pool.history.invalidate("unproved identifier resolution")
		}
		if e != nil {
			return nil, e
		}
		if op == nil {
			return g.nullOAN, nil
		}
		if alias, ok := op.(*userOpDefNode); ok && alias.kind == 37 {
			return g.nonFactProofAlias(tree, false)
		}
		if marker, ok := op.(*userOpDefNode); ok && marker.kind == 6 {
			return g.incompleteInstance(name, marker, children[1], nil, depth)
		}
		if nodeArity(op) != 0 {
			return g.badArity(name, nodeArity(op), tree)
		}
		if rejected, err := g.rejectAPExpression(op, tree); rejected || err != nil {
			if err != nil {
				return nil, err
			}
			return g.nullOAN, nil
		}
		ticket := g.pool.history.event("OpApplNode", 9)
		result, e := newOpAppl(g.pool, op, make([]exprOrOpArg, 0), tree)
		if result != nil {
			g.pool.history.attach(result, ticket)
		}
		if e != nil {
			return nil, e
		}
		g.recursionCheck(op.header().name)
		if user, ok := op.(*userOpDefNode); ok && user.kind == 5 {
			result.subExpressionOf = user
		}
		return result, nil
	case generated.N_OpApplication:
		if len(children) != 2 || children[0].Kind() != generated.N_GeneralId || children[1].Kind() != generated.N_OpArgs {
			g.pool.history.invalidate("unproved application syntax")
			return nil, g.unported("generator.construct.expression-operator-application")
		}
		id := children[0].Heirs()
		if len(id) != 2 {
			return nil, g.unported("generator.construct.expression-operator-application")
		}
		if len(id[0].Heirs()) != 0 {
			if !g.simpleLabelSelectorHistory(id, children[1]) && !g.qualifiedExpressionHistory(id, children[1]) && !g.simpleDirectTheoremSelectorHistory(id, children[1]) {
				g.pool.history.invalidate("unproved application selector")
			}
			return g.simpleLabelSelector(tree, id, children[1], depth)
		}
		name, e := g.name(id[1])
		if e != nil {
			return nil, e
		}
		name, e = g.canonicalName(name)
		if e != nil {
			return nil, e
		}
		op, e := g.resolveOperator(name, id[1])
		if e != nil || (op != nil && !g.pool.history.reuse(op).known) {
			g.pool.history.invalidate("unproved application resolution")
		}
		if e != nil {
			return nil, e
		}
		if op == nil {
			return g.nullOAN, nil
		}
		if alias, ok := op.(*userOpDefNode); ok && alias.kind == 37 {
			parts := children[1].Heirs()
			if len(parts) < 3 || len(parts)%2 == 0 {
				return nil, g.unported("generator.construct.expression-operator-application")
			}
			return g.badArity(name, nodeArity(alias), children[1])
		}
		if marker, ok := op.(*userOpDefNode); ok && marker.kind == 6 {
			if len(marker.params) != marker.arity {
				g.pool.history.invalidate("unproved instance selector")
			}
			return g.incompleteInstance(name, marker, id[1], children[1], depth)
		}
		parts := children[1].Heirs()
		if len(parts) < 3 || len(parts)%2 == 0 {
			return nil, g.unported("generator.construct.expression-operator-application")
		}
		if nodeArity(op) != (len(parts)-1)/2 {
			return g.badArity(name, nodeArity(op), children[1])
		}
		args := make([]exprOrOpArg, 0, (len(parts)-1)/2)
		for i := 1; i < len(parts)-1; i += 2 {
			expected := 0
			if user, ok := op.(*userOpDefNode); ok {
				if (i-1)/2 >= len(user.params) {
					args = append(args, g.nullOAN)
					continue
				}
				expected = user.params[(i-1)/2].arity
			}

			arg, e := g.expressionOrOperatorArgument(parts[i], expected, (i-1)/2, op, id[1], depth+1)
			if e != nil {
				return nil, e
			}
			args = append(args, arg)
		}
		if user, ok := op.(*userOpDefNode); ok && len(user.params) != len(args) {
			loc, e := g.diagnosticLocation(tree)
			if e != nil {
				return nil, e
			}
			e = addError(g.table.errors, 4004, loc, "Wrong number of arguments (%s) given to operator '%s', \nwhich requires %s arguments.", Parameter{Kind: IntParameter, Integer: int64(len(args))}, nameParam(user.name), Parameter{Kind: IntParameter, Integer: int64(len(user.params))})
			if e != nil {
				return nil, e
			}
		} else if user, ok := op.(*userOpDefNode); ok {
			for i, param := range user.params {
				if param.arity > 0 {
					if !matchingOpArgOperand(args[i], param.arity) {
						loc, e := g.diagnosticLocation(tree)
						if e != nil {
							return nil, e
						}
						e = addError(g.table.errors, 4271, loc, "Argument number %s to operator '%s' \nshould be a %s-parameter operator.", Parameter{Kind: IntParameter, Integer: int64(i + 1)}, nameParam(user.name), Parameter{Kind: IntParameter, Integer: int64(param.arity)})
						if e != nil {
							return nil, e
						}
					}
				}
			}
		}
		if rejected, err := g.rejectAPExpression(op, tree); rejected || err != nil {
			g.pool.history.invalidate("unproved application rejection")
			if err != nil {
				return nil, err
			}
			return g.nullOAN, nil
		}
		ticket := g.pool.history.event("OpApplNode", 9)
		result, e := newOpAppl(g.pool, op, args, tree)
		if result != nil {
			g.pool.history.attach(result, ticket)
		}
		if e != nil {
			return nil, e
		}
		if user, ok := op.(*userOpDefNode); ok && user.kind == 5 {
			result.subExpressionOf = user
		}
		return result, nil
	case generated.N_InfixExpr, generated.N_PrefixExpr, generated.N_PostfixExpr:
		if len(children) < 2 || len(children) > 3 {
			g.pool.history.invalidate("unproved operator syntax")
			return nil, g.unported("generator.construct.expression-operator-expression")
		}
		opIndex := 1
		indexes := []int{0, 2}
		if tree.Kind() == generated.N_PrefixExpr {
			opIndex = 0
			indexes = []int{1}
		} else if tree.Kind() == generated.N_PostfixExpr {
			indexes = []int{0}
		}
		if opIndex >= len(children) {
			return nil, g.unported("generator.construct.expression-operator-expression")
		}
		id := children[opIndex].Heirs()
		if len(id) != 2 || len(id[0].Heirs()) != 0 || id[1].Image() == nil {
			g.pool.history.invalidate("unproved operator selector")
			return nil, g.unported("generator.construct.expression-operator-expression")
		}
		units := id[1].Image().Units()
		if tree.Kind() == generated.N_PrefixExpr && string(id[1].Image().String()) == "-" {
			units = append(units, '.')
		}
		u, e := g.pool.syntax.ResolveSynonym(g.pool.syntax.UniqueStringOfUnits(units))
		if e != nil {
			return nil, g.unported("generator.precondition.synonym-lookup")
		}
		rawName := g.pool.internUnits(units)
		name := g.pool.internUnits(u.Units())
		op, e := g.table.resolve(name, false)
		if !unresolvedOperatorHistoryAdmitted(e, op, nullConsumer, rawName) && (e != nil || op == nil || !g.pool.history.reuse(op).known) {
			g.pool.history.invalidate("unproved operator resolution")
		}
		if e != nil {
			return nil, e
		}
		if op == nil {
			if !rawName.displayExact {
				return nil, g.unported("generator.mapping.inexact-display-name")
			}
			loc, e := g.diagnosticLocation(children[opIndex])
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4004, loc, "Could not find declaration or definition of symbol '%s'.", nameParam(rawName)); e != nil {
				return nil, e
			}
			loc, e = g.diagnosticLocation(tree)
			if e != nil {
				return nil, e
			}
			fixity := "infix"
			if tree.Kind() == generated.N_PrefixExpr {
				fixity = "prefix"
			} else if tree.Kind() == generated.N_PostfixExpr {
				fixity = "postfix"
			}
			if e = addError(g.table.errors, 4004, loc, "Couldn't resolve "+fixity+" operator symbol `%s'.", nameParam(rawName)); e != nil {
				return nil, e
			}
			return nil, nil
		}
		if nodeArity(op) != len(indexes) {
			g.pool.history.invalidate("unproved operator arity")
			return nil, g.unported("generator.construct.expression-operator-expression")
		}
		args := make([]exprNode, 0, len(indexes))
		for _, i := range indexes {
			if i >= len(children) {
				return nil, g.unported("generator.construct.expression-operator-expression")
			}
			arg, e := g.nullableExpression(children[i], depth+1)
			if e != nil {
				return nil, e
			}
			args = append(args, arg)
		}
		ticket := g.pool.history.event("OpApplNode", 9)
		result, err := newOpAppl(g.pool, op, expressionOperands(args), tree)
		if result != nil {
			g.pool.history.attach(result, ticket)
		}
		return result, err
	case generated.N_SetEnumerate, generated.N_Tuple, generated.N_Times, generated.N_IfThenElse, generated.N_ConjList, generated.N_DisjList, generated.N_SetOfFcns:
		var builtin string
		var nodes []*generated.SyntaxTreeNode
		switch tree.Kind() {
		case generated.N_SetEnumerate, generated.N_Tuple:
			if len(children) < 2 || (len(children) > 2 && len(children)%2 == 0) {
				return nil, g.unported("generator.construct.expression-collection-form")
			}
			builtin = "$SetEnumerate"
			if tree.Kind() == generated.N_Tuple {
				builtin = "$Tuple"
			}
			for i := 1; i < len(children)-1; i += 2 {
				nodes = append(nodes, children[i])
			}
		case generated.N_Times:
			if len(children) == 0 || len(children)%2 == 0 {
				return nil, g.unported("generator.construct.expression-collection-form")
			}
			builtin = "$CartesianProd"
			for i := 0; i < len(children); i += 2 {
				nodes = append(nodes, children[i])
			}
		case generated.N_IfThenElse:
			if len(children) != 6 {
				return nil, g.unported("generator.construct.expression-collection-form")
			}
			builtin = "$IfThenElse"
			nodes = []*generated.SyntaxTreeNode{children[1], children[3], children[5]}
		case generated.N_SetOfFcns:
			if len(children) != 5 {
				return nil, g.unported("generator.construct.expression-collection-form")
			}
			builtin = "$SetOfFcns"
			nodes = []*generated.SyntaxTreeNode{children[1], children[3]}
		default:
			builtin = "$ConjList"
			if tree.Kind() == generated.N_DisjList {
				builtin = "$DisjList"
			}
			for _, child := range children {
				parts := child.Heirs()
				if len(parts) != 2 {
					return nil, g.unported("generator.construct.expression-collection-form")
				}
				nodes = append(nodes, parts[1])
			}
		}
		args := make([]exprNode, 0, len(nodes))
		for _, node := range nodes {
			arg, e := g.nullableExpression(node, depth+1)
			if e != nil {
				return nil, e
			}
			args = append(args, arg)
		}
		op := g.pool.builtins.lookup(contextKey{name: g.pool.intern(builtin)})
		if op == nil {
			return nil, g.unported("generator.construct.expression-collection-form")
		}
		ticket := g.pool.history.event("OpApplNode", 9)
		h, e := newExprHeader(g.pool, 9, tree)
		if e != nil {
			return nil, e
		}
		result := &opApplNode{exprHeader: h, operator: op, operands: expressionOperands(args), ranges: make([]exprNode, 0)}
		g.pool.history.attach(result, ticket)
		return result, nil
	case generated.N_Lambda:
		loc, e := g.diagnosticLocation(tree)
		if e != nil {
			return nil, e
		}
		if e = addError(g.table.errors, 4275, loc, "LAMBDA expression used where an expression is required."); e != nil {
			return nil, e
		}
		return nil, nil
	case generated.N_Label:
		label, err := g.generateLabel(tree, children, depth)
		if err != nil {
			return nil, err
		}
		if n, ok := label.(*labelNode); ok && n.isAssumeProve {
			loc, e := g.diagnosticLocation(tree)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4004, loc, "Labeled ASSUME/PROVE used where an expression is required."); e != nil {
				return nil, e
			}
		}
		return label, nil
	case generated.N_Except:
		return g.exceptExpression(tree, children, depth)
	case generated.N_Case:
		if len(children) < 2 || len(children)%2 != 0 {
			return nil, g.unported("generator.construct.expression-case")
		}
		pairs := make([]exprNode, 0, len(children)/2)
		for i := 1; i < len(children); i += 2 {
			arm := children[i]
			parts := arm.Heirs()
			if len(parts) != 3 {
				return nil, g.unported("generator.construct.expression-case")
			}
			var condition exprNode
			var e error
			if arm.Kind() != generated.N_OtherArm {
				condition, e = g.expression(parts[0], depth+1)
				if e != nil {
					return nil, e
				}
			}
			value, e := g.expression(parts[2], depth+1)
			if e != nil {
				return nil, e
			}
			pair, e := g.builtinApplication("$Pair", []exprNode{condition, value}, arm)
			if e != nil {
				return nil, e
			}
			pairs = append(pairs, pair)
		}
		return g.builtinApplication("$Case", pairs, tree)
	case generated.N_FcnAppl:
		if len(children) == 0 {
			return nil, g.unported("generator.construct.expression-function-application")
		}
		// Generator.java:3303-3307 returns null as soon as the function
		// expression is null, before any argument or node, so the
		// receiver's consumer is this expression's own consumer.
		g.nullResultConsumer = nullConsumer
		receiver, e := g.expression(children[0], depth+1)
		if e != nil {
			return nil, e
		}
		if receiver == nil {
			if !nullConsumer {
				g.pool.history.invalidate("unproved null function receiver")
			}
			return nil, nil
		}
		if len(children) < 4 || len(children)%2 != 0 {
			return nil, g.unported("generator.construct.expression-function-application")
		}
		count := (len(children) - 2) / 2
		if application, ok := receiver.(*opApplNode); ok {
			g.recursionCheck(application.operator.header().name)
			if def, ok := application.operator.(*userOpDefNode); ok {
				if spec, ok := def.body.(*opApplNode); ok && spec.operator != nil && (spec.operator.header().name.text == "$NonRecursiveFcnSpec" || spec.operator.header().name.text == "$RecursiveFcnSpec") && count >= 2 {
					if len(spec.tupleBounds) != len(spec.bounded) {
						return nil, g.unported("generator.construct.expression-function-application")
					}
					// OpApplNode.getNumberOfBoundedBoundSymbols counts each
					// non-tuple symbol and each tuple as one parameter.
					params := 0
					for i, bound := range spec.bounded {
						if spec.tupleBounds[i] {
							params++
						} else {
							params += len(bound)
						}
					}
					if params != count {
						loc, e := g.diagnosticLocation(tree)
						if e != nil {
							return nil, e
						}
						e = addError(g.table.errors, 4260, loc, "Function '%s' is defined with %s parameters, but is applied to %s arguments.", nameParam(def.name), Parameter{Kind: IntParameter, Integer: int64(params)}, Parameter{Kind: IntParameter, Integer: int64(count)})
						if e != nil {
							return nil, e
						}
						return g.nullOAN, nil
					}
				}
			}
		}
		args := make([]exprNode, count)
		for i := range args {
			args[i], e = g.expression(children[2+2*i], depth+1)
			if e != nil {
				return nil, e
			}
		}
		var index exprNode = args[0]
		if count > 1 {
			index, e = g.builtinApplication("$Tuple", args, tree)
			if e != nil {
				return nil, e
			}
		}
		return g.builtinApplication("$FcnApply", []exprNode{receiver, index}, tree)
	case generated.N_LetIn:
		return g.letIn(tree, children, depth)
	case generated.N_UnboundOrBoundChoose, generated.N_SubsetOf:
		return g.choiceOrSubset(tree, children, depth)
	case generated.N_UnboundQuant:
		return g.unboundQuantifier(tree, children, depth)
	case generated.N_BoundQuant:
		if len(children) < 4 {
			return nil, g.unported("generator.construct.expression-bound-quantifier")
		}
		op := "$BoundedForall"
		if children[0].Kind() == 47 {
			op = "$BoundedExists"
		} else if children[0].Kind() != 49 {
			return nil, g.unported("generator.construct.expression-bound-quantifier")
		}
		return g.boundForm(tree, children, depth, 1, (len(children)-2)/2, len(children)-1, op)
	case generated.N_SetOfAll:
		if len(children) < 5 {
			return nil, g.unported("generator.construct.expression-set-of-all")
		}
		return g.boundForm(tree, children, depth, 3, (len(children)-3)/2, 1, "$SetOfAll")
	case generated.N_FcnConst:
		if len(children) < 5 {
			return nil, g.unported("generator.construct.expression-function-constructor")
		}
		return g.boundForm(tree, children, depth, 1, (len(children)-3)/2, len(children)-2, "$FcnConstructor")
	case generated.N_ActionExpr, generated.N_FairnessExpr:
		if len(children) < 4 {
			return nil, g.unported("generator.construct.expression-action-or-fairness")
		}
		var op string
		// These are the SANY token kinds LAB, LSB, SF and WF.
		switch children[0].Kind() {
		case 102:
			op = "$AngleAct"
		case 97:
			op = "$SquareAct"
		case 59:
			op = "$SF"
		case 86:
			op = "$WF"
		default:
			return nil, g.unported("generator.construct.expression-action-or-fairness")
		}
		first, e := g.expression(children[1], depth+1)
		if e != nil {
			return nil, e
		}
		second, e := g.expression(children[3], depth+1)
		if e != nil {
			return nil, e
		}
		return g.builtinApplication(op, []exprNode{first, second}, tree)
	case generated.N_RecordComponent:
		if len(children) != 3 {
			return nil, g.unported("generator.construct.expression-record-component")
		}
		receiver, e := g.expression(children[0], depth+1)
		if e != nil {
			return nil, e
		}
		ticket := g.pool.history.event("StringNode", 18)
		field, e := newString(g.pool, children[2], false)
		if field != nil {
			g.pool.history.attach(field, ticket)
		}
		if e != nil {
			return nil, e
		}
		return g.builtinApplication("$RcdSelect", []exprNode{receiver, field}, tree)
	case generated.N_RcdConstructor, generated.N_SetOfRcds:
		if len(children) < 2 || (len(children) > 2 && len(children)%2 == 0) {
			return nil, g.unported("generator.construct.expression-record-form")
		}
		labels := make(map[*name]bool)
		pairs := make([]exprNode, 0, (len(children)-1)/2)
		for i := 1; i < len(children)-1; i += 2 {
			parts := children[i].Heirs()
			if len(parts) != 3 {
				return nil, g.unported("generator.construct.expression-record-form")
			}
			ticket := g.pool.history.event("StringNode", 18)
			field, e := newString(g.pool, parts[0], false)
			if field != nil {
				g.pool.history.attach(field, ticket)
			}
			if e != nil {
				return nil, e
			}
			if labels[field.value] {
				loc, e := g.diagnosticLocation(parts[0])
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4262, loc, "Non-unique fields in constructor."); e != nil {
					return nil, e
				}
			}
			labels[field.value] = true
			value, e := g.expression(parts[2], depth+1)
			if e != nil {
				return nil, e
			}
			pair, e := g.builtinApplication("$Pair", []exprNode{field, value}, children[i])
			if e != nil {
				return nil, e
			}
			pairs = append(pairs, pair)
		}
		op := "$RcdConstructor"
		if tree.Kind() == generated.N_SetOfRcds {
			op = "$SetOfRcds"
		}
		result, e := g.builtinApplication(op, pairs, tree)
		if e != nil {
			return nil, e
		}
		if g.table.module == nil {
			return nil, g.unported("generator.construct.expression-record-form")
		}
		g.table.module.records = append(g.table.module.records, result)
		return result, nil
	case generated.N_ParenExpr:
		if len(children) != 3 {
			return nil, g.unported("generator.construct.expression-parenthesized")
		}
		// Generator.java:3078-3079 returns the inner result unchanged, so
		// the enclosing consumer's null handling applies to it.
		g.nullResultConsumer = nullConsumer
		return g.expression(children[1], depth+1)
	}
	return g.unsupportedExpression(tree)
}

// operatorHeaderHistoryAdmitted names the processOperator headers whose
// allocation schedule is source-proved (Generator.java:2494-2704): the four
// parser LHS kinds, with zero either absent or the single LOCAL token that
// NewNodeOptional builds. new Context, pushContext, getUS and
// Operators.resolveSynonym allocate no SemanticNode; the header allocates one
// FormalParamNode per formal in header order, and LOCAL only sets
// OpDefNode.local (OpDefNode.java:481-516). Any other shape stays unproved.
func operatorHeaderHistoryAdmitted(lhsKind int, zero []*generated.SyntaxTreeNode) bool {
	switch lhsKind {
	case generated.N_IdentLHS, generated.N_PrefixLHS, generated.N_InfixLHS, generated.N_PostfixLHS:
	default:
		return false
	}
	if zero == nil {
		return true
	}
	return len(zero) == 1 && zero[0] != nil && zero[0].Kind() == localTokenKind
}

// localTokenKind is the parser's LOCAL token kind (sanyTok_LOCAL).
const localTokenKind = 54

func (g *generator) operator(tree *generated.SyntaxTreeNode, module *moduleNode, depth int) (_ *userOpDefNode, err error) {
	children := tree.One()
	if len(children) != 3 {
		return nil, g.unported("generator.construct.operator")
	}
	lhs := children[0].Heirs()
	if len(lhs) == 0 {
		return nil, g.unported("generator.construct.operator")
	}
	if e := g.table.push(newContext(g.pool)); e != nil {
		return nil, e
	}
	scopeOpen := true
	defer func() {
		if err != nil {
			g.pool.history.invalidate("operator traversal incomplete")
		}
		if scopeOpen {
			if pop := g.table.pop(); err == nil && pop != nil {
				err = pop
			}
		}
	}()
	params := make([]*formalParamNode, 0)
	if !operatorHeaderHistoryAdmitted(children[0].Kind(), tree.Zero()) {
		g.pool.history.invalidate("unproved operator header")
	}
	// The legacy direct-allocation helper keeps its own narrower contract.
	if children[0].Kind() != generated.N_IdentLHS || len(tree.Zero()) != 0 {
		g.pool.java.invalidate()
	}
	formalNames := make(map[*name]bool)
	// addParam is one FormalParamNode constructor call (Generator.java:2547,
	// 2557, 2564, 2567, 2574): one SemanticNode UID, then SymbolTable.addSymbol
	// against the parameter context. A rejected duplicate keeps its UID and is
	// still a parameter (SymbolTable.java:126-160).
	addParam := func(node *generated.SyntaxTreeNode, name *name, arity int) error {
		if arity != 0 || node.Kind() != generated.N_IdentDecl || formalNames[name] {
			g.pool.java.invalidate()
		}
		formalNames[name] = true
		ticket := g.pool.history.event("FormalParamNode", 11)
		p, e := newFormalParam(g.pool, name, arity, node, g.table, module)
		if p != nil {
			g.pool.history.attach(p, ticket)
		}
		if e != nil {
			return e
		}
		g.pool.java.formal(p)
		params = append(params, p)
		return nil
	}
	// Prefix, infix and postfix LHS operands are arity-0 formals built from
	// the operand's own syntax node (Generator.java:2557, 2564, 2567, 2574).
	addOperandParam := func(node *generated.SyntaxTreeNode) error {
		name, e := g.name(node)
		if e != nil {
			return e
		}
		return addParam(node, name, 0)
	}
	var nameNode *generated.SyntaxTreeNode
	canonical := false
	switch children[0].Kind() {
	case generated.N_IdentLHS:
		nameNode = lhs[0]
		if len(lhs) > 2 {
			for i := 2; i < len(lhs); i += 2 {
				decl := lhs[i]
				parts := decl.Heirs()
				if len(parts) == 0 {
					return nil, g.unported("generator.construct.operator")
				}
				var nameNode *generated.SyntaxTreeNode
				arity := 0
				synonym := false
				switch decl.Kind() {
				case generated.N_IdentDecl:
					if len(parts) != 1 {
						g.pool.java.invalidate()
					}
					nameNode = parts[0]
					arity = (len(parts) - 1) / 2
				case generated.N_PrefixDecl:
					nameNode = parts[0]
					arity = 1
				case generated.N_PostfixDecl:
					if len(parts) < 2 {
						return nil, g.unported("generator.construct.operator")
					}
					nameNode = parts[1]
					arity = 1
				case generated.N_InfixDecl:
					// Generator.java:2525-2533 resolves the synonym before the
					// constructor, so the canonical name is bound.
					if len(parts) < 2 {
						return nil, g.unported("generator.construct.operator")
					}
					nameNode = parts[1]
					arity = 2
					synonym = true
				default:
					g.pool.history.invalidate("unproved formal header")
					return nil, g.unported("generator.construct.operator")
				}
				name, e := g.name(nameNode)
				if e != nil {
					return nil, e
				}
				if synonym {
					u, e := g.pool.syntax.ResolveSynonym(g.pool.syntax.UniqueStringOfUnits(name.units))
					if e != nil {
						return nil, g.unported("generator.precondition.synonym-lookup")
					}
					name = g.pool.internUnits(u.Units())
				}
				if e := addParam(decl, name, arity); e != nil {
					return nil, e
				}
			}
		}
	case generated.N_PrefixLHS:
		if len(lhs) != 2 {
			return nil, g.unported("generator.construct.operator")
		}
		nameNode = lhs[0]
		canonical = true
		if e := addOperandParam(lhs[1]); e != nil {
			return nil, e
		}
	case generated.N_InfixLHS:
		if len(lhs) != 3 {
			return nil, g.unported("generator.construct.operator")
		}
		nameNode = lhs[1]
		canonical = true
		if e := addOperandParam(lhs[0]); e != nil {
			return nil, e
		}
		if e := addOperandParam(lhs[2]); e != nil {
			return nil, e
		}
	case generated.N_PostfixLHS:
		if len(lhs) != 2 {
			return nil, g.unported("generator.construct.operator")
		}
		nameNode = lhs[1]
		canonical = true
		if e := addOperandParam(lhs[0]); e != nil {
			return nil, e
		}
	default:
		return nil, g.unported("generator.construct.operator")
	}
	n, e := g.name(nameNode)
	if e != nil {
		return nil, e
	}
	if canonical {
		u, e := g.pool.syntax.ResolveSynonym(g.pool.syntax.UniqueStringOfUnits(n.units))
		if e != nil {
			return nil, g.unported("generator.precondition.synonym-lookup")
		}
		n = g.pool.internUnits(u.Units())
	}
	old, e := g.table.resolve(n, false)
	if e != nil {
		return nil, e
	}
	if old != nil {
		if !existingDefinitionHistoryAdmitted(old, g.pool.history.reuse(old).known) {
			g.pool.history.invalidate("unproved existing definition")
		}
		g.pool.java.invalidate()
	}
	var recursive *userOpDefNode
	wrongLevel := false
	if candidate, ok := old.(*userOpDefNode); ok && candidate.inRecursive && !candidate.defined {
		if candidate.letInLevel != g.letLevel {
			if !n.displayExact {
				return nil, g.unported("generator.mapping.inexact-display-name")
			}
			loc, e := g.diagnosticLocation(tree)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4293, loc, "Recursive operator %s defined at wrong LET/IN level.", nameStringParam(n)); e != nil {
				return nil, e
			}
			wrongLevel = true
		} else {
			matches := len(candidate.params) == len(params)
			// Source assigns on each iteration, so the last formal determines
			// the result when the parameter counts match.
			if matches {
				for _, p := range params {
					matches = p.arity == 0
				}
			}
			if !matches {
				loc, e := g.diagnosticLocation(tree)
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4292, loc, "Definition of %s has different arity than its RECURSIVE declaration.", nameParam(candidate.name)); e != nil {
					return nil, e
				}
			}
			candidate.params = params
			recursive = candidate
		}
	}
	if old != nil && recursive == nil && !wrongLevel {
		if !n.displayExact {
			return nil, g.unported("generator.mapping.inexact-display-name")
		}
		loc, e := nodeLocation(&userOpDefNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{owner: g.pool, tree: tree}, name: n}})
		if e != nil {
			return nil, e
		}
		if e = addError(g.table.errors, 4201, loc, "Operator %s already defined or declared.", nameStringParam(n)); e != nil {
			return nil, e
		}
	}
	// Only a direct label with an unqualified TRUE/FALSE leaf is audited.
	g.javaLabel = nil
	g.javaFormals = nil
	label := children[2]
	if label.Kind() == generated.N_Label && len(label.Heirs()) == 3 {
		leaf := label.Heirs()[2]
		parts := leaf.Heirs()
		if leaf.Kind() == generated.N_GeneralId && len(parts) == 2 && len(parts[0].Heirs()) == 0 && parts[1].Image() != nil && (parts[1].Image().String() == "TRUE" || parts[1].Image().String() == "FALSE") {
			g.javaLabel = label
			g.javaFormals = make(map[*formalParamNode]bool)
			for _, p := range params {
				g.javaFormals[p] = true
			}
		}
	}
	if g.javaLabel == nil {
		g.pool.java.invalidate()
	}
	g.labelScopes = append(g.labelScopes, labelFrame{})
	body, e := g.nullableExpression(children[2], depth+1)
	labels := g.labelScopes[len(g.labelScopes)-1].labels
	g.labelScopes = g.labelScopes[:len(g.labelScopes)-1]
	if e != nil {
		return nil, e
	}
	// Definitions are inserted after popping the parameter scope.
	if e = g.table.pop(); e != nil {
		return nil, e
	}
	scopeOpen = false
	var result *userOpDefNode
	if recursive != nil {
		result = recursive
		if e = g.endRecursive(result, body, tree); e != nil {
			return nil, e
		}
	} else {
		ticket := g.pool.history.event("OpDefNode", 5)
		result, e = newUserOpDef(g.pool, n, params, len(tree.Zero()) != 0, body, module, g.table, tree)
		if result != nil {
			g.pool.history.attach(result, ticket)
		}
		if result != nil {
			result.letInLevel = g.letLevel
			if g.unresolvedSum > 0 {
				result.inRecursiveSection = g.unresolved[g.letLevel] > 0
				result.recursiveSection = g.recursiveSection
				module.recursiveSectionDefs = append(module.recursiveSectionDefs, result)
			}
		}
	}
	if result != nil {
		result.labels = labels
	}
	return result, e
}

// onlySeparatorsAfter reports whether every module body heir after index is
// a SEPARATOR, which generateModule skips (Generator.java:2302-2311).
func onlySeparatorsAfter(heirs []*generated.SyntaxTreeNode, index int) bool {
	for _, h := range heirs[index+1:] {
		if h == nil || h.Kind() != 36 {
			return false
		}
	}
	return true
}

func (g *generator) module(tree *generated.SyntaxTreeNode) (*moduleNode, error) {
	return g.moduleExtending(tree, nil)
}

func (g *generator) moduleExtending(tree *generated.SyntaxTreeNode, extendees []*moduleNode) (*moduleNode, error) {
	return g.moduleInScope(tree, nil, extendees, 1)
}

func (g *generator) moduleInScope(tree *generated.SyntaxTreeNode, parent *moduleNode, extendees []*moduleNode, depth int) (result *moduleNode, resultErr error) {
	defer func() {
		if resultErr != nil {
			g.pool.history.invalidate("module traversal incomplete")
		}
	}()
	if e := g.check(depth); e != nil {
		return nil, e
	}
	if tree == nil || tree.Kind() != generated.N_Module {
		return nil, g.unported("generator.construct.module-in-scope")
	}
	children := tree.Heirs()
	if len(children) < 3 {
		return nil, g.unported("generator.construct.module-in-scope")
	}
	header := children[0].Heirs()
	extends := children[1].Heirs()
	if len(header) < 2 || len(extends)%2 != 0 || (extendees != nil && len(extends)/2 != len(extendees)) {
		return nil, g.unported("generator.construct.module-in-scope")
	}
	for i, extendee := range extendees {
		if extendee == nil || extendee.owner != g.pool {
			return nil, g.unported("generator.construct.module-in-scope")
		}
		ref, err := g.name(extends[2*i+1])
		if err != nil || ref != extendee.name {
			return nil, g.unported("generator.construct.module-in-scope")
		}
	}
	name, e := g.name(header[1])
	if e != nil {
		return nil, e
	}
	if len(extendees) != 0 || len(extends) != 0 {
		g.pool.java.invalidate()
	}
	g.pool.java.module() // first module
	mt := g.pool.history.event("ModuleNode", 1)
	m, e := newModule(g.pool, name, tree, g.table.top)
	if e != nil {
		return nil, e
	}
	g.pool.history.attach(m, mt)
	if parent != nil {
		parent.bodyDefinitions = append(parent.bodyDefinitions, m)
	}
	if e = g.table.setModuleNode(m); e != nil {
		return nil, e
	}
	for i := 1; i < len(extends); i += 2 {
		var extendee *moduleNode
		if extendees != nil {
			extendee = extendees[(i-1)/2]
		} else {
			n, err := g.name(extends[i])
			if err != nil {
				return m, err
			}
			v, err := g.table.resolve(n, true)
			if err != nil {
				return m, err
			}
			var ok bool
			extendee, ok = v.(*moduleNode)
			if !ok || extendee == nil {
				return m, g.unported("generator.construct.module-in-scope")
			}
		}
		if extendee.owner != g.pool {
			return m, g.unported("generator.construct.module-in-scope")
		}
		if !g.pool.history.reuse(extendee).known {
			g.pool.history.invalidate("unproved extendee identity")
		}
		if _, e = m.context.mergeExtend(extendee.context, g.table.errors); e != nil {
			return m, e
		}
		m.extendees = append(m.extendees, extendee)
		m.assumptions = append(m.assumptions, extendee.assumptions...)
		m.theorems = append(m.theorems, extendee.theorems...)
		m.topLevel = append(m.topLevel, extendee.topLevel...)
	}

	for index, def := range children[2].Heirs() {
		if e = g.check(1); e != nil {
			return m, e
		}
		if def.Kind() == 36 {
			continue
		}
		if def.Kind() != generated.N_OperatorDefinition && def.Kind() != generated.N_Module && def.Kind() != generated.N_VariableDeclaration && def.Kind() != generated.N_ParamDeclaration && def.Kind() != generated.N_Recursive && def.Kind() != generated.N_FunctionDefinition && def.Kind() != generated.N_Assumption && def.Kind() != generated.N_Theorem && def.Kind() != generated.N_Instance && def.Kind() != generated.N_ModuleDefinition && !moduleBodyHistoryAdmitted(def.Kind()) {
			g.pool.history.invalidate("unproved module body producer")
		}
		if def.Kind() != generated.N_OperatorDefinition {
			g.pool.java.invalidate()
		}
		if def.Kind() == generated.N_Module {
			if g.unresolvedSum > 0 {
				loc, err := g.diagnosticLocation(def)
				if err != nil {
					return m, err
				}
				if err = addError(g.table.errors, 4294, loc, "%s may not appear within a recursive definition section.", exactStringParam("A MODULE ")); err != nil {
					return m, err
				}
			}
			old := g.table
			childTable, err := newInternalTable(old.external, old.errors, old)
			if err != nil {
				return m, err
			}
			if err = childTable.push(newContext(g.pool)); err != nil {
				return m, err
			}
			g.table = childTable
			child, err := g.moduleInScope(def, m, nil, depth+1)
			g.table = old
			if err != nil {
				return m, err
			}
			if _, err = old.addModule(child.name, child); err != nil {
				return m, err
			}
			m.recursiveSectionDefs = append(m.recursiveSectionDefs, child.recursiveSectionDefs...)
			continue
		}
		if def.Kind() == generated.N_Assumption || def.Kind() == generated.N_Theorem {
			if g.unresolvedSum > 0 {
				kind := "An ASSUME"
				if def.Kind() == generated.N_Theorem {
					kind = "A THEOREM"
				}
				loc, err := g.diagnosticLocation(def)
				if err != nil {
					return m, err
				}
				if err = addError(g.table.errors, 4294, loc, "%s may not appear within a recursive definition section.", exactStringParam(kind)); err != nil {
					return m, err
				}
			}
			g.finalOuterUnit = parent == nil && onlySeparatorsAfter(children[2].Heirs(), index)
			e = g.statement(def, m, depth+1)
			g.finalOuterUnit = false
			if e != nil {
				return m, e
			}
			continue
		}
		if def.Kind() == generated.N_Proof {
			if g.unresolvedSum > 0 {
				loc, err := g.diagnosticLocation(def)
				if err != nil {
					return m, err
				}
				if err = addError(g.table.errors, 4294, loc, "%s may not appear within a recursive definition section.", exactStringParam("A proof")); err != nil {
					return m, err
				}
			}
			continue
		}
		if def.Kind() == generated.N_UseOrHide {
			g.pool.java.invalidate()
			if g.unresolvedSum > 0 {
				loc, err := g.diagnosticLocation(def)
				if err != nil {
					return m, err
				}
				if err = addError(g.table.errors, 4294, loc, "%s may not appear within a recursive definition section.", exactStringParam("A USE or HIDE")); err != nil {
					return m, err
				}
			}
			use, err := g.useOrHide(def, m, depth+1)
			if err != nil {
				return m, err
			}
			if err = g.proofFactCheck(use); err != nil {
				return m, err
			}
			if len(use.facts)+len(use.defs) == 0 {
				loc, err := g.diagnosticLocation(def)
				if err != nil {
					return m, err
				}
				if err = addError(g.table.errors, 4003, loc, "Empty USE or HIDE statement."); err != nil {
					return m, err
				}
			}
			m.topLevel = append(m.topLevel, use)
			continue
		}
		if def.Kind() == generated.N_Instance {
			if e = g.localInstance(def, m); e != nil {
				return m, e
			}
			continue
		}
		if def.Kind() == generated.N_ModuleDefinition {
			if e = g.moduleDefinition(def, m, nil, 1); e != nil {
				return m, e
			}
			continue
		}
		if def.Kind() == generated.N_FunctionDefinition {
			if _, e = g.functionDefinition(def, m, 1); e != nil {
				return m, e
			}
			continue
		}
		if def.Kind() == generated.N_Recursive {
			if e = g.recursiveDeclaration(def, m); e != nil {
				return m, e
			}
			continue
		}
		if def.Kind() == generated.N_VariableDeclaration || def.Kind() == generated.N_ParamDeclaration {
			if g.unresolvedSum > 0 {
				kind := "A declaration"
				if def.Kind() == generated.N_VariableDeclaration {
					kind = "A VARIABLE declaration"
				}
				loc, e := g.diagnosticLocation(def)
				if e != nil {
					return m, e
				}
				if e = addError(g.table.errors, 4294, loc, "%s may not appear within a recursive definition section.", exactStringParam(kind)); e != nil {
					return m, e
				}
			}
			if e = g.declarations(def, m); e != nil {
				return m, e
			}
			continue
		}
		if def.Kind() != generated.N_OperatorDefinition {
			return m, g.unported("generator.construct.module-in-scope")
		}
		d, e := g.operator(def, m, 1)
		if e != nil {
			return m, e
		}
		m.definitions = append(m.definitions, d)
		m.bodyDefinitions = append(m.bodyDefinitions, d)
		g.pool.java.invalidate()
	}
	if e = g.checkUndefinedRecursive(m); e != nil {
		return m, e
	}
	return m, nil
}

func (g *generator) declarations(tree *generated.SyntaxTreeNode, m *moduleNode) (err error) {
	defer func() {
		if err != nil {
			g.pool.history.invalidate("declaration traversal incomplete")
		}
	}()
	parts := tree.Heirs()
	if len(parts) < 2 || len(parts)%2 != 0 {
		return g.unported("generator.construct.declarations")
	}
	for i := 1; i < len(parts); i += 2 {
		node := parts[i]
		kind, level, arity := 3, 1, 0
		named := node
		if tree.Kind() == generated.N_ParamDeclaration {
			kind, level = 2, 0
			h := node.Heirs()
			if len(h) == 0 {
				return g.unported("generator.construct.declarations")
			}
			switch node.Kind() {
			case generated.N_IdentDecl:
				named = h[0]
				arity = (len(h) - 1) / 2
			case generated.N_PrefixDecl:
				named = h[0]
				arity = 1
			case generated.N_InfixDecl:
				if len(h) < 2 {
					return g.unported("generator.construct.declarations")
				}
				named = h[1]
				arity = 2
			case generated.N_PostfixDecl:
				if len(h) < 2 {
					return g.unported("generator.construct.declarations")
				}
				named = h[1]
				arity = 1
			default:
				return g.unported("generator.construct.declarations")
			}
		}
		name, e := g.name(named)
		if e != nil {
			return e
		}
		if e = g.atDeclaration(tree, node); e != nil {
			return e
		}
		if kind == 2 {
			u, e := g.pool.syntax.ResolveSynonym(g.pool.syntax.UniqueStringOfUnits(name.units))
			if e != nil {
				return g.unported("generator.precondition.synonym-lookup")
			}
			name = g.pool.internUnits(u.Units())
		}
		ticket := g.pool.history.event("OpDeclNode", kind)
		decl, err := newOpDecl(g.pool, name, kind, level, arity, node, g.table, m)
		if decl != nil {
			g.pool.history.attach(decl, ticket)
		}
		if err != nil {
			g.pool.history.invalidate("declaration traversal incomplete")
			return err
		}
	}
	return nil
}

func (g *generator) diagnosticLocation(tree *generated.SyntaxTreeNode) (Location, error) {
	return nodeLocation(&userOpDefNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{owner: g.pool, tree: tree}}})
}

// canonicalName follows selectorToNode's synonym resolution before lookup.
func (g *generator) canonicalName(n *name) (*name, error) {
	u, err := g.pool.syntax.ResolveSynonym(g.pool.syntax.UniqueStringOfUnits(n.units))
	if err != nil {
		return nil, err
	}
	return g.pool.internUnits(u.Units()), nil
}
func (g *generator) resolveOperator(name *name, tree *generated.SyntaxTreeNode) (leafNode, error) {
	op, e := g.table.resolve(name, false)
	if e != nil || op != nil {
		return op, e
	}
	if !name.displayExact || name.text == "@" {
		return nil, g.unported("generator.construct.resolve-operator")
	}
	loc, e := g.diagnosticLocation(tree)
	if e != nil {
		return nil, e
	}
	e = addError(g.table.errors, 4200, loc, "Unknown operator: `%s'.", nameParam(name))
	return nil, e
}
func (g *generator) badArity(name *name, arity int, tree *generated.SyntaxTreeNode) (exprNode, error) {
	if !name.displayExact || arity < 0 {
		return nil, g.unported("generator.construct.bad-arity")
	}
	loc, e := g.diagnosticLocation(tree)
	if e != nil {
		return nil, e
	}
	if e = addError(g.table.errors, 4204, loc, "The operator %s requires %s arguments.", nameStringParam(name), Parameter{Kind: IntParameter, Integer: int64(arity)}); e != nil {
		return nil, e
	}
	return g.nullOAN, nil
}

func (g *generator) builtinApplication(name string, args []exprNode, tree *generated.SyntaxTreeNode) (*opApplNode, error) {
	op := g.pool.builtins.lookup(contextKey{name: g.pool.intern(name)})
	if op == nil {
		return nil, g.unported("generator.precondition.builtin-unavailable")
	}
	ticket := g.pool.history.event("OpApplNode", 9)
	h, e := newExprHeader(g.pool, 9, tree)
	if e != nil {
		return nil, e
	}
	result := &opApplNode{exprHeader: h, operator: op, operands: expressionOperands(args), ranges: make([]exprNode, 0)}
	g.pool.history.attach(result, ticket)
	return result, nil
}

func (g *generator) unboundQuantifier(tree *generated.SyntaxTreeNode, children []*generated.SyntaxTreeNode, depth int) (_ exprNode, err error) {
	if len(children) < 4 {
		return nil, g.unported("generator.construct.unbound-quantifier")
	}
	var op string
	// Token kinds are EXISTS, FORALL, T_EXISTS and T_FORALL.
	switch children[0].Kind() {
	case 47:
		op = "$UnboundedExists"
	case 49:
		op = "$UnboundedForall"
	case 60:
		op = "$TemporalExists"
	case 61:
		op = "$TemporalForall"
	default:
		return nil, g.unported("generator.construct.unbound-quantifier")
	}
	count := (len(children) - 2) / 2
	params := make([]*formalParamNode, count)
	if err = g.table.push(newContext(g.pool)); err != nil {
		return nil, err
	}
	defer func() {
		if e := g.table.pop(); err == nil && e != nil {
			err = e
		}
	}()
	for i := range params {
		node := children[2*i+1]
		name, e := g.name(node)
		if e != nil {
			return nil, e
		}
		ticket := g.pool.history.event("FormalParamNode", 11)
		params[i], e = newFormalParam(g.pool, name, 0, node, g.table, g.table.module)
		if params[i] != nil {
			g.pool.history.attach(params[i], ticket)
		}
		if e != nil {
			return nil, e
		}
	}
	body, e := g.expressionWithBindings(children[len(children)-1], params, depth+1)
	if e != nil {
		return nil, e
	}
	result, e := g.builtinApplication(op, []exprNode{body}, tree)
	if e != nil {
		return nil, e
	}
	result.unbounded = params
	return result, nil
}

func (g *generator) boundArguments(children []*generated.SyntaxTreeNode, depth, offset, count int) (params [][]*formalParamNode, tuples []bool, bounds []exprNode, err error) {
	bounds = make([]exprNode, count)
	params = make([][]*formalParamNode, count)
	tuples = make([]bool, count)
	for i := range bounds {
		index := offset + 2*i
		if index < 0 || index >= len(children) || children[index] == nil {
			return nil, nil, nil, g.unported("generator.construct.bound-arguments")
		}
		parts := children[index].Heirs()
		if len(parts) < 3 || parts[0] == nil || parts[len(parts)-1] == nil {
			return nil, nil, nil, g.unported("generator.construct.bound-arguments")
		}
		bounds[i], err = g.expression(parts[len(parts)-1], depth+1)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	for i := range params {
		parts := children[offset+2*i].Heirs()
		var names []*generated.SyntaxTreeNode
		if parts[0].Kind() == generated.N_IdentifierTuple {
			tuples[i] = true
			tuple := parts[0].Heirs()
			// SANY uses half the heir count; for <<>> this includes the
			// closing token as its sole formal parameter.
			for j := 0; j < len(tuple)/2; j++ {
				if tuple[2*j+1] == nil {
					return nil, nil, nil, g.unported("generator.construct.bound-arguments")
				}
				names = append(names, tuple[2*j+1])
			}
		} else {
			for j := 0; j < len(parts)-2; j += 2 {
				if parts[j] == nil {
					return nil, nil, nil, g.unported("generator.construct.bound-arguments")
				}
				names = append(names, parts[j])
			}
		}
		params[i] = make([]*formalParamNode, len(names))
		for j, node := range names {
			name, e := g.name(node)
			if e != nil {
				return nil, nil, nil, e
			}
			ticket := g.pool.history.event("FormalParamNode", 11)
			params[i][j], e = newFormalParam(g.pool, name, 0, node, g.table, g.table.module)
			if params[i][j] != nil {
				g.pool.history.attach(params[i][j], ticket)
			}
			if e != nil {
				return nil, nil, nil, e
			}
		}
	}
	return params, tuples, bounds, nil
}
func (g *generator) boundForm(tree *generated.SyntaxTreeNode, children []*generated.SyntaxTreeNode, depth, offset, count, bodyIndex int, op string) (_ exprNode, err error) {
	if err = g.table.push(newContext(g.pool)); err != nil {
		return nil, err
	}
	defer func() {
		if e := g.table.pop(); err == nil && e != nil {
			err = e
		}
	}()
	params, tuples, bounds, err := g.boundArguments(children, depth, offset, count)
	if err != nil {
		return nil, err
	}
	body, e := g.expressionWithBindings(children[bodyIndex], flattenFormal(params), depth+1)
	if e != nil {
		return nil, e
	}
	result, e := g.builtinApplication(op, []exprNode{body}, tree)
	if e != nil {
		return nil, e
	}
	result.bounded = params
	result.tupleBounds = tuples
	result.ranges = bounds
	return result, nil
}

func (g *generator) choiceOrSubset(tree *generated.SyntaxTreeNode, children []*generated.SyntaxTreeNode, depth int) (_ exprNode, err error) {
	isSubset := tree.Kind() == generated.N_SubsetOf
	if isSubset && len(children) < 6 || !isSubset && len(children) < 5 {
		return nil, g.unported("generator.construct.choice-or-subset")
	}
	var bound exprNode
	var boundTree *generated.SyntaxTreeNode
	op := "$UnboundedChoose"
	bodyIndex := 4
	if isSubset {
		op = "$SubsetOf"
		boundTree = children[3]
		bodyIndex = 5
	} else {
		parts := children[2].Heirs()
		if len(parts) > 0 {
			if len(parts) < 2 {
				return nil, g.unported("generator.construct.choice-or-subset")
			}
			op = "$BoundedChoose"
			boundTree = parts[1]
		}
	}
	// Subset ranges are evaluated before push; bounded CHOOSE evaluates its
	// range in the newly pushed, still empty scope.
	if isSubset {
		bound, err = g.expression(boundTree, depth+1)
		if err != nil {
			return nil, err
		}
	}
	if err = g.table.push(newContext(g.pool)); err != nil {
		return nil, err
	}
	defer func() {
		if e := g.table.pop(); err == nil && e != nil {
			err = e
		}
	}()
	if !isSubset && boundTree != nil {
		bound, err = g.expression(boundTree, depth+1)
		if err != nil {
			return nil, err
		}
	}
	var names []*generated.SyntaxTreeNode
	tuple := children[1].Kind() == generated.N_IdentifierTuple
	if tuple {
		parts := children[1].Heirs()
		// processChoose and processSubsetOf use the same source indexing as
		// bounded quantifiers, including the closing token for <<>>.
		for i := 0; i < len(parts)/2; i++ {
			names = append(names, parts[2*i+1])
		}
	} else {
		names = append(names, children[1])
	}
	params := make([]*formalParamNode, len(names))
	for i, node := range names {
		name, e := g.name(node)
		if e != nil {
			return nil, e
		}
		paramTree := node
		if !isSubset && boundTree == nil && !tuple {
			paramTree = children[0]
		}
		ticket := g.pool.history.event("FormalParamNode", 11)
		params[i], e = newFormalParam(g.pool, name, 0, paramTree, g.table, g.table.module)
		if params[i] != nil {
			g.pool.history.attach(params[i], ticket)
		}
		if e != nil {
			return nil, e
		}
	}
	body, e := g.expressionWithBindings(children[bodyIndex], params, depth+1)
	if e != nil {
		return nil, e
	}
	result, e := g.builtinApplication(op, []exprNode{body}, tree)
	if e != nil {
		return nil, e
	}
	if boundTree == nil {
		result.unbounded = params
	} else {
		result.bounded = [][]*formalParamNode{params}
		result.tupleBounds = []bool{tuple}
		result.ranges = []exprNode{bound}
	}
	return result, nil
}

func (g *generator) letIn(tree *generated.SyntaxTreeNode, children []*generated.SyntaxTreeNode, depth int) (_ exprNode, err error) {
	// processLetIn raises curLevel while it is below MaxLetInLevel (100) and
	// then stores unresolvedCnt[curLevel] in an array of that length
	// (Generator.java:2967-2978, 7162, 7175). Reaching level 100 therefore
	// throws ArrayIndexOutOfBounds before the "LETs nested more than" abort
	// can be reached; the Java exception is not modeled.
	if len(children) != 4 || g.letLevel >= 99 {
		return nil, g.unported("generator.construct.let-in")
	}
	c := newContext(g.pool)
	if err = g.table.push(c); err != nil {
		return nil, err
	}
	defer func() {
		if e := g.table.pop(); err == nil && e != nil {
			err = e
		}
	}()
	g.letLevel++
	g.unresolved[g.letLevel] = 0
	priorSum := g.unresolvedSum
	levelActive := true
	defer func() {
		if levelActive {
			g.unresolvedSum = priorSum
			g.unresolved[g.letLevel] = 0
			g.letLevel--
		}
	}()
	defs := make([]leafNode, 0)
	instances := make([]*instanceNode, 0)
	for _, syntax := range children[1].Heirs() {
		if err = g.check(depth + 1); err != nil {
			return nil, err
		}
		if syntax.Kind() == generated.N_FunctionDefinition {
			d, e := g.functionDefinition(syntax, g.table.module, depth+1)
			if e != nil {
				return nil, e
			}
			if d != nil {
				defs = append(defs, d)
			}
			continue
		}
		if syntax.Kind() == generated.N_Recursive {
			if e := g.recursiveDeclaration(syntax, g.table.module); e != nil {
				return nil, e
			}
			continue
		}
		if syntax.Kind() == generated.N_ModuleDefinition {
			// processModuleDefinition(tree, defVec, instVec, cm)
			// (Generator.java:2998-3000): the clones and the marker join
			// this LET's definitions and its InstanceNode joins insts; the
			// symbols go into the pushed LET context. The node sequence is
			// the shared constructor's, but no profile has certified it
			// under a LET, so the private histories stop being exact.
			g.pool.history.invalidate("unproved LET module definition")
			g.pool.java.invalidate()
			var out moduleDefinitionOutput
			if e := g.moduleDefinition(syntax, g.table.module, &out, depth+1); e != nil {
				return nil, e
			}
			if out.instance == nil {
				return nil, g.unported("generator.construct.let-in")
			}
			defs = append(defs, out.defs...)
			instances = append(instances, out.instance)
			continue
		}
		if syntax.Kind() != generated.N_OperatorDefinition {
			return nil, g.unported("generator.construct.let-in")
		}
		d, e := g.operator(syntax, g.table.module, depth+1)
		if e != nil {
			return nil, e
		}
		defs = append(defs, d)
		g.table.module.definitions = append(g.table.module.definitions, d)
		g.table.module.bodyDefinitions = append(g.table.module.bodyDefinitions, d)
	}
	if e := g.checkUndefinedRecursive(g.table.module); e != nil {
		return nil, e
	}
	g.letLevel--
	levelActive = false
	body, e := g.expression(children[3], depth+1)
	if e != nil {
		return nil, e
	}
	ticket := g.pool.history.event("LetInNode", 10)
	h, e := newExprHeader(g.pool, 10, tree)
	if e != nil {
		return nil, e
	}
	result := &letInNode{exprHeader: h, definitions: defs, instances: instances, body: body, context: c}
	g.pool.history.attach(result, ticket)
	return result, nil
}

func (g *generator) expressionOrOperatorArgument(tree *generated.SyntaxTreeNode, expected, position int, main leafNode, mainTree *generated.SyntaxTreeNode, depth int) (exprOrOpArg, error) {
	if expected == 0 {
		return g.expression(tree, depth)
	}
	switch tree.Kind() {
	case generated.N_GenInfixOp, generated.N_GenNonExpPrefixOp, generated.N_GenPostfixOp, generated.N_GenPrefixOp, generated.N_Lambda:
	case generated.N_GeneralId:
		parts := tree.Heirs()
		if len(parts) != 2 || (len(parts[0].Heirs()) != 0 && !g.selectorOperatorArgumentHistory(parts, expected)) {
			g.pool.history.invalidate("unproved operator argument selector")
		}
	default:
		// generateExprOrOpArg rejects expression syntax without evaluating it.
	}
	switch tree.Kind() {
	case generated.N_GenInfixOp, generated.N_GenNonExpPrefixOp, generated.N_GenPostfixOp, generated.N_GenPrefixOp:
		parts := tree.Heirs()
		if len(parts) != 2 || len(parts[0].Heirs()) != 0 {
			return nil, g.unported("generator.construct.expression-or-operator-argument")
		}
		name, e := g.name(parts[1])
		if e != nil {
			return nil, e
		}
		u, e := g.pool.syntax.ResolveSynonym(g.pool.syntax.UniqueStringOfUnits(name.units))
		if e != nil {
			return nil, e
		}
		resolvedName := g.pool.internUnits(u.Units())
		op, e := g.table.resolve(resolvedName, false)
		if e != nil {
			return nil, e
		}
		if op != nil && !g.pool.history.reuse(op).known {
			g.pool.history.invalidate("unproved operator argument resolution")
		}
		if op == nil {
			loc, e := g.diagnosticLocation(tree)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4004, loc, "Could not find declaration or definition of symbol '%s'.", nameParam(name)); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		if nodeArity(op) != expected {
			// A recursive placeholder can acquire a different source location
			// before SANY renders its object-valued diagnostic parameter.
			if d, ok := main.(*userOpDefNode); ok && d.inRecursive && !d.defined {
				return nil, g.unported("generator.construct.expression-or-operator-argument")
			}
			loc, e := g.diagnosticLocation(mainTree)
			if e != nil {
				return nil, e
			}
			mainLoc, e := nodeLocation(main)
			if e != nil {
				return nil, e
			}
			mainText, e := renderLocation(mainLoc)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4004, loc, "Operator with incorrect arity passed as argument. \nOperator '%s' of arity %s is argument number %s (counting from 1) to operator `%s', \nbut an operator of arity %s was expected.", nameParam(op.header().name), Parameter{Kind: IntParameter, Integer: int64(nodeArity(op))}, Parameter{Kind: IntParameter, Integer: int64(position + 1)}, Parameter{Kind: SemanticNodeParameter, Identity: main.header().identity, Text: mainText, UTF16Units: utf16.Encode([]rune(mainText)), UTF16Present: true}, Parameter{Kind: IntParameter, Integer: int64(expected)}); e != nil {
				return nil, e
			}
			return g.nullOpArg, nil
		}
		ticket := g.pool.history.event("OpArgNode", 8)
		result, err := newOpArg(g.pool, op, tree, g.table.module)
		if result != nil {
			g.pool.history.attach(result, ticket)
		}
		return result, err
	case generated.N_Lambda:
		return g.lambdaArgument(tree, mainTree, expected, position, main, depth)
	}
	if tree.Kind() != generated.N_GeneralId {
		loc, e := g.diagnosticLocation(tree)
		if e != nil {
			return nil, e
		}
		e = addError(g.table.errors, 4270, loc, "An expression appears as argument number %s (counting from 1) to operator '%s', in a position an operator is required.", Parameter{Kind: IntParameter, Integer: int64(position + 1)}, nameParam(main.header().name))
		if e != nil {
			return nil, e
		}
		return g.nullOAN, nil
	}
	id := tree.Heirs()
	if len(id) != 2 {
		return nil, g.unported("generator.construct.expression-or-operator-argument")
	}
	if len(id[0].Heirs()) != 0 {
		return g.labelOperatorArgument(tree, id, expected, depth)
	}
	name, e := g.name(id[1])
	if e != nil {
		return nil, e
	}
	op, e := g.resolveOperator(name, id[1])
	if e != nil {
		return nil, e
	}
	if op == nil {
		// selectorToNode reports the unknown name and reuses nullOAN.
		return g.nullOAN, nil
	}
	if !g.pool.history.reuse(op).known {
		g.pool.history.invalidate("unproved operator argument resolution")
	}
	if alias, ok := op.(*userOpDefNode); ok && alias.kind == 37 {
		return g.nonFactProofAlias(tree, false)
	}
	if nodeArity(op) != expected {
		loc, e := g.diagnosticLocation(tree)
		if e != nil {
			return nil, e
		}
		e = addError(g.table.errors, 4271, loc, "Expected arity %s but found operator of arity %s.", Parameter{Kind: IntParameter, Integer: int64(expected)}, Parameter{Kind: IntParameter, Integer: int64(nodeArity(op))})
		if e != nil {
			return nil, e
		}
		return g.nullOAN, nil
	}
	if rejected, err := g.rejectAPExpression(op, tree); rejected || err != nil {
		if err != nil {
			return nil, err
		}
		return g.nullOAN, nil
	}
	ticket := g.pool.history.event("OpArgNode", 8)
	result, err := newOpArg(g.pool, op, tree, g.table.module)
	if result != nil {
		g.pool.history.attach(result, ticket)
	}
	return result, err
}

// matchingOpArgOperand follows OpDefNode.match: an operator argument may
// have the required arity but may not itself require an operator argument.
func matchingOpArgOperand(value exprOrOpArg, arity int) bool {
	arg, ok := value.(*opArgNode)
	if !ok || arg == nil || arg.arity != arity {
		return false
	}
	if def, ok := arg.operator.(*userOpDefNode); ok {
		if len(def.params) < def.arity {
			return false
		}
		for _, param := range def.params[:def.arity] {
			if param.arity > 0 {
				return false
			}
		}
	}
	return true
}

// selectorBody retains substitution wrappers while descending through an instance.
// A selector frame is scoped by the caller so nested argument generation cannot
// consume its enclosing selector's prefix.
func (g *generator) selectorBody(body exprNode, depth int) (exprNode, error) {
	for {
		wrapper, ok := body.(*substInNode)
		if !ok {
			return body, nil
		}
		if err := g.check(depth); err != nil {
			return nil, err
		}
		depth++
		g.selectorSubsts = append(g.selectorSubsts, wrapper)
		body = wrapper.body
	}
}

func (g *generator) wrapSelectorBody(body exprNode) (exprNode, error) {
	for i := len(g.selectorSubsts) - 1; i >= 0; i-- {
		prefix := g.selectorSubsts[i]
		if body == nil {
			return nil, g.unported("generator.construct.wrap-selector-body")
		}
		ticket := g.pool.history.event("SubstInNode", 13)
		h, err := newExprHeader(g.pool, 13, prefix.tree)
		if err != nil {
			return nil, err
		}
		body = &substInNode{exprHeader: h, substs: prefix.substs, body: body, instantiatingModule: prefix.instantiatingModule, instantiatedModule: prefix.instantiatedModule}
		g.pool.history.attach(body, ticket)
	}
	return body, nil
}

type selectorTraversalError struct {
	tree    *generated.SyntaxTreeNode
	message string
	unknown *name
	// noOperand reports "Choosing operand ... with no operands" at tree.
	noOperand bool
}

func (*selectorTraversalError) Error() string { return "unsupported selector" }

func (g *generator) rejectSelectorTraversal(failure *selectorTraversalError) (exprNode, error) {
	if failure.noOperand {
		return g.noOperandSelection(failure.tree)
	}
	loc, err := g.diagnosticLocation(failure.tree)
	if err != nil {
		return nil, err
	}
	if failure.unknown != nil {
		if err = addError(g.table.errors, 4200, loc, "Unknown operator: `%s'.", nameParam(failure.unknown)); err != nil {
			return nil, err
		}
	} else if err = addError(g.table.errors, 4005, loc, failure.message); err != nil {
		return nil, err
	}
	return g.nullOAN, nil
}

// rejectSelectorArgumentsInOperatorMode implements selectorToNode's initial
// check: an operator-argument selector cannot itself carry syntax arguments.
func (g *generator) rejectSelectorArgumentsInOperatorMode(node *generated.SyntaxTreeNode) (exprNode, error) {
	var param Parameter
	if node.Kind() == generated.N_OpArgs {
		param = asciiStringParam("!(...)")
	} else {
		n, err := g.name(node)
		if err != nil {
			return nil, err
		}
		param = nameStringParam(n)
	}
	loc, err := g.diagnosticLocation(node)
	if err != nil {
		return nil, err
	}
	if err = addError(g.table.errors, 4005, loc, "Selector `%s' should not have argument(s).", param); err != nil {
		return nil, err
	}
	return g.nullOAN, nil
}

type selectorSuppliedArgumentError struct{ tree *generated.SyntaxTreeNode }

func (*selectorSuppliedArgumentError) Error() string { return "arguments in operator selector" }

type selectorNullError struct{ tree *generated.SyntaxTreeNode }

func (*selectorNullError) Error() string { return "selected null operand" }

type selectorArgumentMismatch struct {
	tree             *generated.SyntaxTreeNode
	actual, expected int
}

func (*selectorArgumentMismatch) Error() string { return "quantifier selector argument mismatch" }

func (g *generator) rejectSelectorArguments(mismatch *selectorArgumentMismatch) (exprNode, error) {
	loc, err := g.diagnosticLocation(mismatch.tree)
	if err != nil {
		return nil, err
	}
	if err = addError(g.table.errors, 4005, loc, "Selector with %s argument(s) used for quantifier with %s bound identifier(s).", Parameter{Kind: IntParameter, Integer: int64(mismatch.actual)}, Parameter{Kind: IntParameter, Integer: int64(mismatch.expected)}); err != nil {
		return nil, err
	}
	return g.nullOAN, nil
}

// structuralLetTarget follows modeled structural operands into a LET context.
// Some binding and special operand layouts remain incomplete.
func (g *generator) structuralLetTarget(body exprNode, prefix []*generated.SyntaxTreeNode, last *generated.SyntaxTreeNode, bound *[]*formalParamNode, boundArgs *[]*generated.SyntaxTreeNode, depth int, operatorMode bool) (*userOpDefNode, error) {
	if len(prefix) < 2 || !isLetSelectorName(last) { // NameSel, Generator.java:409-418
		return nil, g.unported("generator.construct.structural-let-target")
	}
	selectors := make([]*generated.SyntaxTreeNode, 0, len(prefix)-1)
	for _, part := range prefix[1:] {
		pieces := part.Heirs()
		if len(pieces) != 2 || (pieces[0].Kind() != generated.N_StructOp && (boundArgs == nil || pieces[0].Kind() != generated.N_OpArgs)) {
			return nil, g.unported("generator.construct.structural-let-target")
		}
		selectors = append(selectors, pieces[0])
	}
	return g.structuralLetFrom(body, selectors, last, bound, boundArgs, depth, operatorMode)
}

func (g *generator) structuralLetFrom(current exprNode, selectors []*generated.SyntaxTreeNode, last *generated.SyntaxTreeNode, bound *[]*formalParamNode, boundArgs *[]*generated.SyntaxTreeNode, depth int, operatorMode bool) (*userOpDefNode, error) {
	return g.structuralLetFromAllowUndefined(current, selectors, last, bound, boundArgs, depth, operatorMode, false)
}
func (g *generator) structuralLetFromAllowUndefined(current exprNode, selectors []*generated.SyntaxTreeNode, last *generated.SyntaxTreeNode, bound *[]*formalParamNode, boundArgs *[]*generated.SyntaxTreeNode, depth int, operatorMode, allowUndefined bool) (*userOpDefNode, error) {
	if len(selectors) == 0 || !isLetSelectorName(last) { // NameSel, Generator.java:409-418
		return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
	}
	for i := 0; i < len(selectors); i++ {
		selector := selectors[i]
		if operatorMode && selector.Kind() == generated.N_OpArgs {
			return nil, &selectorSuppliedArgumentError{tree: selector}
		}
		if selector.Kind() != generated.N_StructOp && (boundArgs == nil || selector.Kind() != generated.N_OpArgs) {
			return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
		}
		if e := g.check(depth + i + 1); e != nil {
			return nil, e
		}
		if isColonSelector(selector) && !operatorMode {
			if i > 0 {
				// The previous operand selector is followed by a colon
				// (Generator.java:1567-1575), after that index's bound
				// arguments and isNullSelection.
				return nil, &selectorTraversalError{tree: selectors[i-1], message: "!: should not follow an operand selector."}
			}
			if len(selectors) > 1 {
				// A leading colon neither ends the selector nor precedes a
				// NameSel (Generator.java:1130-1137).
				return nil, &selectorTraversalError{tree: selector, message: "`!:' can be used only after a name and either at the end after an\noperator name or before an operator name."}
			}
		}
		if current == nil && !isColonSelector(selector) {
			// FindingSubExpr on a null curNode, as when the selected
			// definition's body is null after an error in it
			// (Generator.java:1138-1145). Every later step leaves a
			// non-null node or has already returned.
			return nil, &selectorTraversalError{tree: selector, message: "Subexpression selection failed, probably due to an error in the selected expression."}
		}
		for labelDepth := depth + i + 2; ; labelDepth++ {
			label, ok := current.(*labelNode)
			if !ok {
				break
			}
			if e := g.check(labelDepth); e != nil {
				return nil, e
			}
			if label.body == nil {
				// isNullSelection after unwrapping a label (Generator.java:1527-1530, 1544-1546, 1553-1556).
				if i == 0 {
					// SANY's stepped-back idx (Generator.java:1530-1544) names the selector
					// before this one; at suffix index 0 that is sel.ops[0], the root name,
					// but callers may start mid-chain, so it is not provable: incomplete.
					return nil, g.unported("generator.construct.label-null-body-first-selector")
				}
				return nil, &selectorNullError{tree: selectors[i-1]}
			}
			current = label.expressionBody()
		}
		switch node := current.(type) {
		case *letInNode:
			// ArgNum(sel, 1) for a LET/IN (Generator.java:1146-1157); a
			// NullSel is -1. A selector structuralIndex cannot classify
			// stays incomplete.
			number := -1
			if selector.Kind() != generated.N_OpArgs {
				var e error
				number, _, e = structuralIndex(selector, 1)
				if e != nil {
					return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
				}
			}
			if number != 1 {
				return nil, &selectorTraversalError{tree: selector, message: "A LET/IN expression has only one operand."}
			}
			current = node.body
			if current == nil {
				return nil, &selectorNullError{tree: selector}
			}
		case *opApplNode:
			if node.operator == nil {
				return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
			}
			if _, builtin := node.operator.(*builtinOpDefNode); !builtin && !isColonSelector(selector) {
				if kind, known := selectorOperatorKind(node.operator); known && !selectsOperandByArity(kind) {
					// Neither an arity-selected nor a built-in operator,
					// such as the null OpDefNode of nullOAN
					// (Generator.java:1394-1400).
					return nil, &selectorTraversalError{tree: selector, noOperand: true}
				}
			}
			if len(node.bounded) != 0 || len(node.unbounded) != 0 {
				if bound != nil && boundArgs != nil && selector.Kind() == generated.N_OpArgs {
					formal := node.unbounded
					if len(node.bounded) != 0 {
						formal = flattenFormal(node.bounded)
					}
					parts := selector.Heirs()
					if len(parts) < 2 || (len(parts) != 2 && len(parts)%2 == 0) || len(node.operands) != 1 {
						return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
					}
					count := (len(parts) - 1) / 2
					if count != len(formal) {
						return nil, &selectorArgumentMismatch{tree: selector, actual: count, expected: len(formal)}
					}
					*bound = append(*bound, formal...)
					for j := 1; j < len(parts)-1; j += 2 {
						*boundArgs = append(*boundArgs, parts[j])
					}
					var ok bool
					current, ok = node.operands[0].(exprNode)
					if !ok || current == nil {
						return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
					}
					continue
				}
				if bound != nil && len(selector.Heirs()) == 0 && selector.Image() != nil && selector.Image().String() == "@" {
					if len(node.operands) != 1 {
						return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
					}
					if len(node.bounded) != 0 {
						*bound = append(*bound, flattenFormal(node.bounded)...)
					} else {
						*bound = append(*bound, node.unbounded...)
					}
					var ok bool
					current, ok = node.operands[0].(exprNode)
					if !ok || current == nil {
						return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
					}
					continue
				}
				number, _, e := structuralIndex(selector, len(node.ranges))
				if e != nil || number < 1 {
					return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
				}
				if node.ranges[number-1] == nil {
					return nil, &selectorNullError{tree: selector}
				}
				current = node.ranges[number-1]
				continue
			}
			if node.operator.header().name == nil {
				return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
			}
			name := node.operator.header().name.text
			switch op := node.operator.(type) {
			case *builtinOpDefNode:
				switch name {
				case "$Tuple", "$SetEnumerate", "$CartesianProd", "$IfThenElse", "$ConjList", "$DisjList", "$SetOfFcns", "$FcnApply", "$RcdConstructor", "$SetOfRcds", "$RcdSelect", "$Case", "$Except":
				default:
					return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
				}
			case *userOpDefNode, *formalParamNode:
				if nodeArity(op) != len(node.operands) {
					return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
				}
			case *opDeclNode:
				if op.kind != 2 || op.arity != len(node.operands) {
					return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
				}
			default:
				return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
			}
			number, _, e := structuralIndex(selector, len(node.operands))
			if e != nil || number < 1 {
				return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
			}
			if name == "$Except" && number > 1 {
				return nil, &selectorTraversalError{tree: selector, message: "Selecting subexpression of an EXCEPT not yet implemented."}
			}
			operand := node.operands[number-1]
			if name == "$Case" || name == "$RcdConstructor" || name == "$SetOfRcds" {
				pair, ok := operand.(*opApplNode)
				if !ok || pair.operator == nil || pair.operator.header().name == nil || pair.operator.header().name.text != "$Pair" || len(pair.operands) != 2 {
					return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
				}
				which := 2
				if name == "$Case" {
					i++
					if i >= len(selectors) {
						return nil, &selectorTraversalError{tree: last, message: "Second selector for CASE subexpression must specify  one of two operands."}
					}
					if selectors[i].Kind() != generated.N_StructOp {
						return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
					}
					if e := g.check(depth + i + 1); e != nil {
						return nil, e
					}
					which, _, e = structuralIndex(selectors[i], 2)
					if e != nil {
						return nil, e
					}
					if which < 1 {
						return nil, &selectorTraversalError{tree: selectors[i], message: "Second selector for CASE subexpression must specify  one of two operands."}
					}
				}
				operand = pair.operands[which-1]
				if name == "$Case" && operand == nil {
					return nil, &selectorTraversalError{tree: selectors[i], message: "Selecting OTHER in a CASE statement."}
				}
			}
			if operand == nil {
				return nil, &selectorNullError{tree: selectors[i]}
			}
			var ok bool
			current, ok = operand.(exprNode)
			if !ok {
				if _, isOpArg := operand.(*opArgNode); isOpArg && i == len(selectors)-1 {
					return nil, &selectorTraversalError{tree: selector, message: "A name selector here must be from a LET clause."}
				}
				return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
			}
		case *numeralNode, *decimalNode, *stringNode, *atNode:
			if operatorMode {
				return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
			}
			// A leaf expression has no operand (Generator.java:1519-1527).
			return nil, &selectorTraversalError{tree: selector, message: "Selecting subexpression of expression that has none."}
		default:
			return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
		}
	}
	for labelDepth := depth + len(selectors) + 1; ; labelDepth++ {
		label, ok := current.(*labelNode)
		if !ok {
			break
		}
		if e := g.check(labelDepth); e != nil {
			return nil, e
		}
		if label.body == nil {
			// isNullSelection after unwrapping a label (Generator.java:1527-1530, 1544-1546, 1553-1556).
			return nil, &selectorNullError{tree: selectors[len(selectors)-1]}
		}
		current = label.expressionBody()
	}
	let, ok := current.(*letInNode)
	if !ok {
		if current == nil {
			return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
		}
		return nil, &selectorTraversalError{tree: selectors[len(selectors)-1], message: "A name selector here must be from a LET clause."}
	}
	return g.letLocalTarget(let, last, allowUndefined)
}

// letLocalTarget is FindingOpName in a LET/IN context (Generator.java:
// 772-806): the NameSel last is looked up in the LET's own context.
func (g *generator) letLocalTarget(let *letInNode, last *generated.SyntaxTreeNode, allowUndefined bool) (*userOpDefNode, error) {
	if let.context == nil {
		return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
	}
	name, _, e := g.letSelectorName(last)
	if e != nil {
		return nil, e
	}
	symbol := let.context.lookup(contextKey{name: name})
	if symbol == nil && name.displayExact {
		return nil, &selectorTraversalError{tree: last, unknown: name}
	}
	local, ok := symbol.(*userOpDefNode)
	if !ok || (!local.defined && !allowUndefined) || len(local.params) != local.arity {
		return nil, g.unported("generator.construct.structural-let-from-allow-undefined")
	}
	return local, nil
}

// directLetChain follows alternating colon/name selectors and retains each
// enclosing operator's formal parameters.
func (g *generator) directLetChain(root *userOpDefNode, rootBody exprNode, prefix []*generated.SyntaxTreeNode, last *generated.SyntaxTreeNode, depth int) (result *userOpDefNode, parameters []*formalParamNode, err error) {
	mark := len(g.selectorSubsts)
	defer func() {
		if err != nil {
			g.selectorSubsts = g.selectorSubsts[:mark]
		}
	}()
	if len(prefix) < 4 || len(prefix)%2 != 0 || last.Kind() != 289 { // IDENTIFIER
		return nil, nil, g.unported("generator.construct.direct-let-chain")
	}
	current := root
	params := append([]*formalParamNode{}, root.params...)
	for i := 1; i < len(prefix); i += 2 {
		if e := g.check(depth + i); e != nil {
			return nil, nil, e
		}
		colon := prefix[i].Heirs()
		if len(colon) != 2 || colon[0].Kind() != generated.N_StructOp || len(colon[0].Heirs()) != 0 || colon[0].Image() == nil || colon[0].Image().String() != ":" {
			return nil, nil, g.unported("generator.construct.direct-let-chain")
		}
		named := last
		if i+1 < len(prefix) {
			parts := prefix[i+1].Heirs()
			if (len(parts) != 2 && len(parts) != 3) || parts[0].Kind() != 289 { // IDENTIFIER
				return nil, nil, g.unported("generator.construct.direct-let-chain")
			}
			if len(parts) == 3 {
				_, err := g.rejectSelectorArgumentsInOperatorMode(parts[0])
				if err != nil {
					return nil, nil, err
				}
				return nil, nil, nil
			}
			named = parts[0]
		}
		body := current.body
		if i == 1 {
			body = rootBody
		} else {
			var err error
			body, err = g.selectorBody(body, depth+i)
			if err != nil {
				return nil, nil, err
			}
		}
		for labelDepth := depth + i + 1; ; labelDepth++ {
			label, ok := body.(*labelNode)
			if !ok {
				break
			}
			if e := g.check(labelDepth); e != nil {
				return nil, nil, e
			}
			body = label.expressionBody()
		}
		let, ok := body.(*letInNode)
		if !ok {
			if body == nil {
				return nil, nil, g.unported("generator.construct.direct-let-chain")
			}
			loc, e := g.diagnosticLocation(colon[0])
			if e != nil {
				return nil, nil, e
			}
			if e = addError(g.table.errors, 4005, loc, "A name selector here must be from a LET clause."); e != nil {
				return nil, nil, e
			}
			return nil, nil, nil
		}
		if let.context == nil {
			return nil, nil, g.unported("generator.construct.direct-let-chain")
		}
		name, e := g.name(named)
		if e != nil {
			return nil, nil, e
		}
		symbol := let.context.lookup(contextKey{name: name})
		if symbol == nil && i+1 == len(prefix) && name.displayExact {
			loc, e := g.diagnosticLocation(named)
			if e != nil {
				return nil, nil, e
			}
			if e = addError(g.table.errors, 4200, loc, "Unknown operator: `%s'.", nameParam(name)); e != nil {
				return nil, nil, e
			}
			return nil, nil, nil
		}
		local, ok := symbol.(*userOpDefNode)
		if !ok || !local.defined || len(local.params) != local.arity {
			return nil, nil, g.unported("generator.construct.direct-let-chain")
		}
		if i+1 < len(prefix) {
			params = append(params, local.params...)
		}
		current = local
	}
	return current, params, nil
}

// unresolvedSelection is the symbol found by selectorToNode's first
// FindingOpName search after its leading name is undefined
// (Generator.java:732-805): each NameSel extends the compound name, the
// arguments of the segments that did not resolve accumulate in tempArgs, and
// the search stops at the first compound name the symbol table resolves.
type unresolvedSelection struct {
	index   int // resolving part; len(prefix) is the final selector
	symbol  leafNode
	name    *name
	leading []*generated.SyntaxTreeNode // tempArgs of the parts before index
}

// unresolvedSelector reports the compound-name search's Unknown operator
// error; a caller that cannot continue from a resolved compound name keeps
// the explicit incomplete boundary.
func (g *generator) unresolvedSelector(prefix []*generated.SyntaxTreeNode, last *generated.SyntaxTreeNode, depth int) (exprNode, error) {
	found, failure, err := g.unresolvedSelectorPrefix(prefix, last, depth)
	if err != nil || found == nil {
		return failure, err
	}
	return nil, g.unported("generator.construct.unresolved-selector")
}

// unresolvedSelectorPrefix follows the source's compound-name search on the
// identifier and structural selector forms. It returns either the resolved
// selection or the recovery node after the Unknown operator error, whose
// location spans the name parts searched (Generator.java:787-813).
func (g *generator) unresolvedSelectorPrefix(prefix []*generated.SyntaxTreeNode, last *generated.SyntaxTreeNode, depth int) (*unresolvedSelection, exprNode, error) {
	parts := make([]*generated.SyntaxTreeNode, 0, len(prefix)+1)
	for i, item := range prefix {
		if err := g.check(depth + i + 1); err != nil {
			return nil, nil, err
		}
		heirs := item.Heirs()
		if len(heirs) != 2 && len(heirs) != 3 {
			return nil, nil, g.unported("generator.construct.unresolved-selector")
		}
		parts = append(parts, heirs[0])
	}
	parts = append(parts, last)
	var units []uint16
	var location Location
	structural := false
	for i, part := range parts {
		if err := g.check(depth + i + 1); err != nil {
			return nil, nil, err
		}
		switch part.Kind() {
		case 289, 290, 291, generated.N_InfixOp, generated.N_NonExpPrefixOp, generated.N_PostfixOp, generated.N_PrefixOp: // NameSel: IDENTIFIER, ProofStepLexeme, ProofImplicitStepLexeme or operator token (Generator.java:409-418)
			if part.Image() == nil {
				return nil, nil, g.unported("generator.construct.unresolved-selector")
			}
			normalized, err := g.pool.syntax.ResolveSynonym(g.pool.syntax.UniqueStringOfUnits(part.Image().Units()))
			if err != nil {
				return nil, nil, g.unported("generator.precondition.synonym-lookup")
			}
			if len(units) != 0 {
				units = append(units, '!')
			}
			units = append(units, normalized.Units()...)
			loc, err := g.diagnosticLocation(part)
			if err != nil {
				return nil, nil, err
			}
			if location.NamePresent {
				location.EndLine, location.EndColumn = loc.EndLine, loc.EndColumn
			} else {
				location = loc
			}
			compound := g.pool.internUnits(units)
			found, err := g.table.resolve(compound, false)
			if err != nil {
				return nil, nil, err
			}
			if found == nil {
				continue
			}
			if structural {
				// A structural selector between the name parts adds its
				// arguments to tempArgs without extending the name.
				return nil, nil, g.unported("generator.construct.unresolved-selector")
			}
			var leading []*generated.SyntaxTreeNode
			for _, item := range prefix[:i] {
				heirs := item.Heirs()
				if len(heirs) != 3 {
					continue
				}
				if heirs[1].Kind() != generated.N_OpArgs {
					return nil, nil, g.unported("generator.construct.unresolved-selector")
				}
				args := heirs[1].Heirs()
				for j := 1; j < len(args)-1; j += 2 {
					leading = append(leading, args[j])
				}
			}
			return &unresolvedSelection{index: i, symbol: found, name: compound, leading: leading}, nil, nil
		case generated.N_StructOp, generated.N_OpArgs:
			if len(units) == 0 {
				return nil, nil, g.unported("generator.construct.unresolved-selector")
			}
			structural = true
		default:
			return nil, nil, g.unported("generator.construct.unresolved-selector")
		}
	}
	name := g.pool.internUnits(units)
	if len(units) == 0 || !name.displayExact {
		return nil, nil, g.unported("generator.construct.unresolved-selector")
	}
	if err := addError(g.table.errors, 4200, location, "Unknown operator: `%s'.", nameParam(name)); err != nil {
		return nil, nil, err
	}
	return nil, g.nullOAN, nil
}

// rejectDeclaredSubexpression follows selectorToNode's declaration branch:
// a declaration or builtin cannot be followed by another selector.
func (g *generator) rejectDeclaredSubexpression(symbol leafNode, syntax *generated.SyntaxTreeNode) (exprNode, bool, error) {
	switch symbol.(type) {
	case *builtinOpDefNode, *formalParamNode, *opDeclNode:
	default:
		return nil, false, nil
	}
	name := symbol.header().name
	if name == nil || !name.displayExact {
		return nil, true, g.unported("generator.construct.reject-declared-subexpression")
	}
	loc, err := g.diagnosticLocation(syntax)
	if err != nil {
		return nil, true, err
	}
	if err = addError(g.table.errors, 4005, loc, "Cannot take subexpression of `%s'.", nameStringParam(name)); err != nil {
		return nil, true, err
	}
	return g.nullOAN, true, nil
}

// rejectUndefinedSubexpression follows the selector check on recursive
// operator placeholders before entering a label or operand.
func (g *generator) rejectUndefinedSubexpression(root *userOpDefNode, syntax *generated.SyntaxTreeNode) (exprNode, error) {
	if root.name == nil || !root.name.displayExact {
		return nil, g.unported("generator.construct.reject-undefined-subexpression")
	}
	loc, err := g.diagnosticLocation(syntax)
	if err != nil {
		return nil, err
	}
	if err = addError(g.table.errors, 4005, loc, "Subexpression of  `%s' used inside the operator's definition.", nameStringParam(root.name)); err != nil {
		return nil, err
	}
	return g.nullOAN, nil
}

// operatorArgumentTarget returns the operator argument for a compound name
// whose final selector resolved: an instance marker is incomplete, and the
// arity must be the expected one (Generator.java:950-958, 1686-1701,
// 1897-1910).
func (g *generator) operatorArgumentTarget(tree, last *generated.SyntaxTreeNode, op leafNode, expected int) (exprOrOpArg, error) {
	if v, ok := op.(*userOpDefNode); ok && v.kind == 6 {
		loc, err := g.diagnosticLocation(last)
		if err != nil {
			return nil, err
		}
		if err = addError(g.table.errors, 4203, loc, "Operator name %s is incomplete.", nameStringParam(v.name)); err != nil {
			return nil, err
		}
		return g.nullOAN, nil
	}
	if nodeArity(op) != expected {
		loc, err := g.diagnosticLocation(tree)
		if err != nil {
			return nil, err
		}
		if err = addError(g.table.errors, 4271, loc, "Expected arity %s but found operator of arity %s.", Parameter{Kind: IntParameter, Integer: int64(expected)}, Parameter{Kind: IntParameter, Integer: int64(nodeArity(op))}); err != nil {
			return nil, err
		}
		return g.nullOAN, nil
	}
	if rejected, err := g.rejectAPExpression(op, tree); rejected || err != nil {
		if err != nil {
			return nil, err
		}
		return g.nullOAN, nil
	}
	ticket := g.pool.history.event("OpArgNode", 8)
	result, err := newOpArg(g.pool, op, tree, g.table.module)
	if result != nil {
		g.pool.history.attach(result, ticket)
	}
	return result, err
}

// labelOperatorArgument handles name-only selectors in operator-argument
// position. Parameters of the root definition and selected labels become
// parameters of SANY's generated LAMBDA operator.
func (g *generator) labelOperatorArgument(tree *generated.SyntaxTreeNode, id []*generated.SyntaxTreeNode, expected, depth int) (exprOrOpArg, error) {
	outerSubsts := g.selectorSubsts
	g.selectorSubsts = nil
	defer func() { g.selectorSubsts = outerSubsts }()
	prefix := id[0].Heirs()
	if len(prefix) == 0 {
		return nil, g.unported("generator.construct.label-operator-argument")
	}
	first := prefix[0].Heirs()
	if len(first) != 2 && len(first) != 3 {
		return nil, g.unported("generator.construct.label-operator-argument")
	}
	if len(first) == 3 {
		return g.rejectSelectorArgumentsInOperatorMode(first[0])
	}
	name, e := g.name(first[0])
	if e != nil {
		return nil, e
	}
	symbol, e := g.table.resolve(name, false)
	if e != nil {
		return nil, e
	}
	if symbol == nil {
		found, failure, err := g.unresolvedSelectorPrefix(prefix, id[1], depth)
		if err != nil || found == nil {
			return failure, err
		}
		// Supplied arguments are checked only at the start of an outer
		// step; the compound search collects the arguments of the later
		// segments it passes in tempArgs, which operator mode never
		// generates (Generator.java:705-713, 787-801, 897-905).
		for _, segment := range prefix[1:min(found.index+1, len(prefix))] {
			if len(segment.Heirs()) == 3 {
				return nil, g.unported("generator.construct.unresolved-selector")
			}
		}
		switch v := found.symbol.(type) {
		case *userOpDefNode:
			if found.index < len(prefix) && v.kind != 5 {
				// A resolved instance marker continues the compound search.
				return nil, g.unported("generator.construct.unresolved-selector")
			}
		case *thmOrAssumpDefNode:
		default:
			return nil, g.unported("generator.construct.unresolved-selector")
		}
		if found.index == len(prefix) {
			return g.operatorArgumentTarget(tree, id[1], found.symbol, expected)
		}
		prefix = prefix[found.index:]
		first = prefix[0].Heirs()
		symbol = found.symbol
	}
	if alias, ok := symbol.(*userOpDefNode); ok && alias.kind == 37 {
		return g.rejectProofAliasSubexpression(first[0], alias)
	}
	if result, handled, err := g.rejectDeclaredSubexpression(symbol, first[0]); handled {
		return result, err
	}
	if theorem, ok := symbol.(*thmOrAssumpDefNode); ok {
		return g.theoremSelector(tree, prefix, id[1], nil, theorem, nil, depth, expected)
	}
	root, ok := symbol.(*userOpDefNode)
	if !ok {
		return nil, g.unported("generator.construct.label-operator-argument")
	}
	if root.kind == 6 {
		index, selected, names, _, failure, err := g.qualifiedSelectorPrefix(tree, prefix, id[1], depth, true)
		if err != nil {
			return nil, err
		}
		if failure != nil {
			return failure, nil
		}
		if index == len(prefix) {
			var units []uint16
			for _, n := range names {
				if len(units) != 0 {
					units = append(units, '!')
				}
				units = append(units, n.units...)
			}
			if id[1].Kind() == generated.N_StructOp || id[1].Kind() == generated.N_OpArgs {
				return nil, g.unported("generator.construct.label-operator-argument")
			}
			last, err := g.name(id[1])
			if err != nil {
				return nil, err
			}
			if len(units) != 0 {
				units = append(units, '!')
			}
			units = append(units, last.units...)
			op, err := g.resolveOperator(g.pool.internUnits(units), tree)
			if err != nil {
				return nil, err
			}
			if op == nil {
				return g.nullOAN, nil
			}
			return g.operatorArgumentTarget(tree, id[1], op, expected)
		}
		prefix = prefix[index:]
		first = prefix[0].Heirs()
		if theorem, ok := selected.(*thmOrAssumpDefNode); ok {
			return g.theoremSelector(tree, prefix, id[1], nil, theorem, nil, depth, expected)
		}
		var ok bool
		root, ok = selected.(*userOpDefNode)
		if !ok {
			return nil, g.unported("generator.construct.label-operator-argument")
		}
	}
	rootBody, e := g.selectorBody(root.body, depth+1)
	if e != nil {
		return nil, e
	}
	if !root.defined {
		return g.rejectUndefinedSubexpression(root, first[0])
	}
	var bound []*formalParamNode
	var boundArgs []*generated.SyntaxTreeNode
	if local, e := g.structuralLetTarget(rootBody, prefix, id[1], &bound, &boundArgs, depth, true); e == nil {
		for _, syntax := range boundArgs {
			if _, err := g.expression(syntax, depth+1); err != nil {
				return nil, err
			}
		}
		params := append(append([]*formalParamNode{}, root.params...), bound...)
		return g.selectedLocalArgument(tree, local, params, expected)
	} else if failure, ok := e.(*selectorTraversalError); ok {
		return g.rejectSelectorTraversal(failure)
	} else if supplied, ok := e.(*selectorSuppliedArgumentError); ok {
		return g.rejectSelectorArgumentsInOperatorMode(supplied.tree)
	} else if missing, ok := e.(*selectorNullError); ok {
		return g.nullSelected(missing.tree)
	} else if mismatch, ok := e.(*selectorArgumentMismatch); ok {
		return g.rejectSelectorArguments(mismatch)
	} else if e != ErrPortIncomplete {
		return nil, e
	}
	g.incomplete.forget() // the alternative below consumes this origin
	if len(prefix) >= 4 {
		local, params, e := g.directLetChain(root, rootBody, prefix, id[1], depth)
		if e == nil {
			if local == nil {
				return g.nullOAN, nil
			}
			arity := len(params) + local.arity
			if arity != expected {
				loc, e := g.diagnosticLocation(tree)
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4271, loc, "Expected arity %s but found operator of arity %s.", Parameter{Kind: IntParameter, Integer: int64(expected)}, Parameter{Kind: IntParameter, Integer: int64(arity)}); e != nil {
					return nil, e
				}
				return g.nullOAN, nil
			}
			return g.selectedLocalArgument(tree, local, params, expected)
		}
		if e != ErrPortIncomplete {
			return nil, e
		}
		g.incomplete.forget() // the alternative below consumes this origin
	}
	if len(prefix) == 2 && id[1].Kind() == 289 { // IDENTIFIER
		pieces := prefix[1].Heirs()
		if len(pieces) == 2 && pieces[0].Kind() == generated.N_StructOp && len(pieces[0].Heirs()) == 0 && pieces[0].Image() != nil && pieces[0].Image().String() == ":" {
			body := rootBody
			for labelDepth := depth + 1; ; labelDepth++ {
				label, ok := body.(*labelNode)
				if !ok {
					break
				}
				if e = g.check(labelDepth); e != nil {
					return nil, e
				}
				body = label.expressionBody()
			}
			let, ok := body.(*letInNode)
			if !ok {
				if body == nil {
					return nil, g.unported("generator.construct.label-operator-argument")
				}
				loc, e := g.diagnosticLocation(pieces[0])
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4005, loc, "A name selector here must be from a LET clause."); e != nil {
					return nil, e
				}
				return g.nullOAN, nil
			}
			if let.context == nil {
				return nil, g.unported("generator.construct.label-operator-argument")
			}
			localName, e := g.name(id[1])
			if e != nil {
				return nil, e
			}
			selectedSymbol := let.context.lookup(contextKey{name: localName})
			if selectedSymbol == nil {
				if !localName.displayExact {
					return nil, g.unported("generator.mapping.inexact-display-name")
				}
				loc, e := g.diagnosticLocation(id[1])
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4200, loc, "Unknown operator: `%s'.", nameParam(localName)); e != nil {
					return nil, e
				}
				return g.nullOAN, nil
			}
			local, ok := selectedSymbol.(*userOpDefNode)
			if !ok || !local.defined || len(local.params) != local.arity {
				return nil, g.unported("generator.construct.label-operator-argument")
			}
			return g.selectedLocalArgument(tree, local, root.params, expected)
		}
	}
	params := append([]*formalParamNode{}, root.params...)
	labels := root.labels
	selectors := make([]*generated.SyntaxTreeNode, 0, len(prefix))
	var selectorHasArgs []bool
	for _, part := range prefix[1:] {
		children := part.Heirs()
		if len(children) != 2 && len(children) != 3 {
			return nil, g.unported("generator.construct.label-operator-argument")
		}
		selectors = append(selectors, children[0])
		selectorHasArgs = append(selectorHasArgs, len(children) == 3)
	}
	selectors = append(selectors, id[1])
	selectorHasArgs = append(selectorHasArgs, false)
	var selected exprNode = rootBody
	for index, node := range selectors {
		if selectorHasArgs[index] {
			return g.rejectSelectorArgumentsInOperatorMode(node)
		}
		if e = g.check(depth + 1); e != nil {
			return nil, e
		}
		if node.Kind() == generated.N_StructOp || node.Kind() == generated.N_OpArgs {
			structural := selectors[index:]
			if len(structural) > 1 && structural[len(structural)-1].Kind() == 289 { // IDENTIFIER
				var bound []*formalParamNode
				var boundArgs []*generated.SyntaxTreeNode
				local, e := g.structuralLetFrom(selected, structural[:len(structural)-1], structural[len(structural)-1], &bound, &boundArgs, depth, true)
				if e == nil {
					for _, syntax := range boundArgs {
						if _, err := g.expression(syntax, depth+1); err != nil {
							return nil, err
						}
					}
					return g.selectedLocalArgument(tree, local, append(params, bound...), expected)
				}
				if failure, ok := e.(*selectorTraversalError); ok {
					return g.rejectSelectorTraversal(failure)
				}
				if supplied, ok := e.(*selectorSuppliedArgumentError); ok {
					return g.rejectSelectorArgumentsInOperatorMode(supplied.tree)
				}
				if missing, ok := e.(*selectorNullError); ok {
					return g.nullSelected(missing.tree)
				}
				if mismatch, ok := e.(*selectorArgumentMismatch); ok {
					return g.rejectSelectorArguments(mismatch)
				}
				if e != ErrPortIncomplete {
					return nil, e
				}
				g.incomplete.forget() // the alternative below consumes this origin
			}
			for _, remaining := range structural {
				if remaining.Kind() != generated.N_StructOp && remaining.Kind() != generated.N_OpArgs {
					// A name after an operand or colon selector names a
					// LET definition, which can be followed by its labels
					// and operands (Generator.java:1121-1137, 1557-1566,
					// 732-1000). The general selector loop starts again
					// from the root; nothing above has been generated.
					result, err := g.definitionLevelSelector(tree, prefix, id[1], nil, root, root.params, rootBody, root.labels, nil, depth, expected, false)
					if err != nil || result == nil {
						return nil, err
					}
					value, ok := result.(exprOrOpArg)
					if !ok {
						return nil, g.unported("generator.construct.label-operator-argument")
					}
					return value, nil
				}
			}
			return g.numericSubexpression(tree, structural, root, selected, params, nil, depth, expected, index == 0)
		}
		labelName, e := g.name(node)
		if e != nil {
			return nil, e
		}
		label := labels.get(labelName)
		if label == nil {
			if !labelName.displayExact {
				return nil, g.unported("generator.mapping.inexact-display-name")
			}
			loc, e := g.diagnosticLocation(node)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4004, loc, "Cannot find label `%s'.", nameStringParam(labelName)); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		illegal, e := g.illegalProofLabelRef(label, node)
		if e != nil {
			return nil, e
		}
		if illegal {
			loc, e := g.diagnosticLocation(node)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4005, loc, "Accessing subexpression labeled `%s' of ASSUME/PROVE clause within the scope of a declaration\n from outside that declaration's scope.", nameStringParam(labelName)); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		selected = label
		params = append(params, label.params...)
		labels = label.labels
	}
	if len(params) != expected {
		loc, e := g.diagnosticLocation(tree)
		if e != nil {
			return nil, e
		}
		if e = addError(g.table.errors, 4271, loc, "Expected arity %s but found operator of arity %s.", Parameter{Kind: IntParameter, Integer: int64(expected)}, Parameter{Kind: IntParameter, Integer: int64(len(params))}); e != nil {
			return nil, e
		}
		return g.nullOAN, nil
	}
	selected, e = g.wrapSelectorBody(selected)
	if e != nil {
		return nil, e
	}
	return g.selectorLambdaArgument(tree, params, selected)
}

// lambdaArgument constructs the source LAMBDA definition in a nested scope.
func (g *generator) lambdaArgument(tree, mainTree *generated.SyntaxTreeNode, expected, position int, main leafNode, depth int) (result exprOrOpArg, err error) {
	children := tree.Heirs()
	if len(children) < 4 || len(children)%2 != 0 {
		return nil, g.unported("generator.construct.lambda-argument")
	}
	arity := (len(children) - 2) / 2
	if err = g.table.push(newContext(g.pool)); err != nil {
		return nil, err
	}
	defer func() {
		if e := g.table.pop(); err == nil && e != nil {
			err = e
		}
	}()
	params := make([]*formalParamNode, arity)
	for i := range params {
		node := children[2*i+1]
		name, e := g.name(node)
		if e != nil {
			return nil, e
		}
		ticket := g.pool.history.event("FormalParamNode", 11)
		params[i], e = newFormalParam(g.pool, name, 0, node, g.table, g.table.module)
		if params[i] != nil {
			g.pool.history.attach(params[i], ticket)
		}
		if e != nil {
			return nil, e
		}
	}
	body, e := g.expressionWithBindings(children[len(children)-1], params, depth+1)
	if e != nil {
		return nil, e
	}
	ticket := g.pool.history.event("OpDefNode", 5)
	def, e := newUserOpDef(g.pool, g.pool.intern("LAMBDA"), params, false, body, g.table.module, nil, tree)
	if def != nil {
		g.pool.history.attach(def, ticket)
	}
	if e != nil {
		return nil, e
	}
	if arity != expected {
		loc, e := g.diagnosticLocation(mainTree)
		if e != nil {
			return nil, e
		}
		if e = addError(g.table.errors, 4274, loc, "Lambda expression with arity %s used as argument %s of operator `%s', \nbut an operator of arity %s is required.", Parameter{Kind: IntParameter, Integer: int64(arity)}, Parameter{Kind: IntParameter, Integer: int64(position + 1)}, nameParam(main.header().name), Parameter{Kind: IntParameter, Integer: int64(expected)}); e != nil {
			return nil, e
		}
		return g.nullOpArg, nil
	}
	ticket = g.pool.history.event("OpArgNode", 8)
	result, e = newOpArg(g.pool, def, tree, g.table.module)
	if result != nil {
		g.pool.history.attach(result, ticket)
	}
	return result, e
}

func (g *generator) recursiveDeclaration(tree *generated.SyntaxTreeNode, m *moduleNode) (err error) {
	defer func() {
		if err != nil {
			g.pool.history.invalidate("recursive declaration incomplete")
		}
	}()
	children := tree.Heirs()
	if len(children) < 2 || len(children)%2 != 0 {
		return g.unported("generator.construct.recursive-declaration")
	}
	if g.unresolvedSum == 0 {
		g.recursiveSection++
	}
	count := len(children) / 2
	g.unresolved[g.letLevel] += count
	g.unresolvedSum += count
	for i := 1; i < len(children); i += 2 {
		decl := children[i]
		parts := decl.Heirs()
		if len(parts) == 0 {
			return g.unported("generator.construct.recursive-declaration")
		}
		named, arity := parts[0], 0
		switch decl.Kind() {
		case generated.N_IdentDecl:
			arity = (len(parts) - 1) / 2
		case generated.N_PrefixDecl:
			arity = 1
		case generated.N_InfixDecl, generated.N_PostfixDecl:
			if len(parts) < 2 {
				return g.unported("generator.construct.recursive-declaration")
			}
			named = parts[1]
			arity = 1
			if decl.Kind() == generated.N_InfixDecl {
				arity = 2
			}
		default:
			return g.unported("generator.construct.recursive-declaration")
		}
		name, e := g.name(named)
		if e != nil {
			return e
		}
		u, e := g.pool.syntax.ResolveSynonym(g.pool.syntax.UniqueStringOfUnits(name.units))
		if e != nil {
			return e
		}
		name = g.pool.internUnits(u.Units())
		// buildParameter constructs an uninserted OpDecl before dummy parameters.
		prior, lookupErr := g.table.resolve(name, false)
		if lookupErr != nil || prior != nil {
			g.pool.history.invalidate("unproved recursive name collision")
		}
		g.pool.history.event("OpDeclNode", 2) // discarded buildParameter result
		if _, e = newOpDecl(g.pool, name, 2, 0, arity, decl, nil, m); e != nil {
			return e
		}
		params := make([]*formalParamNode, arity)
		for j := range params {
			ticket := g.pool.history.event("FormalParamNode", 11)
			h, e := newSemanticHeader(g.pool, kindFormal, nil)
			if e != nil {
				return e
			}
			params[j] = &formalParamNode{nodeHeader: nodeHeader{semanticHeader: h}, module: m, levels: newLeafLevel()}
			g.pool.history.attach(params[j], ticket)
		}
		ticket := g.pool.history.event("OpDefNode", 5)
		h, e := newNodeHeader(g.pool, name, 5, decl)
		if e != nil {
			return e
		}
		n := &userOpDefNode{nodeHeader: h, arity: arity, params: params, module: m, levels: newLeafLevel(), maxLevels: make([]int, arity), weights: make([]int, arity), isLeibniz: true, isLeibnizArg: make([]bool, arity), letInLevel: g.letLevel, inRecursive: true, inRecursiveSection: true, recursiveSection: g.recursiveSection}
		g.pool.history.attach(n, ticket)
		for j := range params {
			n.maxLevels[j] = 3
			n.isLeibnizArg[j] = true
		}
		if _, e = g.table.addSymbol(name, n); e != nil {
			return e
		}
		m.recursiveDecls = append(m.recursiveDecls, n)
		m.recursiveSectionDefs = append(m.recursiveSectionDefs, n)
	}
	return nil
}

// endRecursive mirrors Generator.endOpDefNode. A negative global count is
// a source-level WrongInvocationException, not a user-facing semantic error.
// The public port does not model that Java exception, so retain incomplete.
func (g *generator) endRecursive(n *userOpDefNode, body exprNode, tree *generated.SyntaxTreeNode) error {
	n.defined = true
	n.body = body
	n.tree = tree
	if n.inRecursive {
		g.unresolved[g.letLevel]--
		g.unresolvedSum--
		if g.unresolvedSum < 0 {
			return g.unported("generator.construct.end-recursive")
		}
	}
	return nil
}

func (g *generator) checkUndefinedRecursive(m *moduleNode) error {
	if g.unresolved[g.letLevel] > 0 {
		for _, n := range m.recursiveDecls {
			if n.letInLevel == g.letLevel && n.inRecursive && !n.defined {
				if !n.name.displayExact {
					return g.unported("generator.mapping.inexact-display-name")
				}
				loc, e := g.diagnosticLocation(n.tree)
				if e != nil {
					return e
				}
				if e = addError(g.table.errors, 4291, loc, "Symbol %s declared in RECURSIVE statement but not defined.", nameStringParam(n.name)); e != nil {
					return e
				}
			}
		}
		g.unresolvedSum -= g.unresolved[g.letLevel]
	}
	return nil
}

func (g *generator) recursionCheck(name *name) {
	for i := len(g.functions) - 1; i >= 0; i-- {
		if g.functions[i].name == name {
			g.functions[i].application.operator = g.pool.builtins.lookup(contextKey{name: g.pool.intern("$RecursiveFcnSpec")})
			return
		}
	}
}
func (g *generator) functionDefinition(tree *generated.SyntaxTreeNode, module *moduleNode, depth int) (_ definitionNode, err error) {
	defer func() {
		if err != nil {
			g.pool.history.invalidate("function traversal incomplete")
		}
	}()
	children := tree.One()
	if len(children) < 6 {
		return nil, g.unported("generator.construct.function-definition")
	}
	count := (len(children) - 4) / 2
	c := newContext(g.pool)
	if err = g.table.push(c); err != nil {
		return nil, err
	}
	scopeOpen := true
	defer func() {
		if scopeOpen {
			if e := g.table.pop(); err == nil && e != nil {
				err = e
			}
		}
	}()
	params, tuples, bounds, e := g.boundArguments(children, depth, 2, count)
	if e != nil {
		return nil, e
	}
	name, e := g.name(children[0])
	if e != nil {
		return nil, e
	}
	old, e := g.table.resolve(name, false)
	if e != nil {
		return nil, e
	}
	if old != nil {
		// processFunction constructs the self formal and function application
		// even when an existing symbol causes an arity or redefinition
		// diagnostic; no replacement OpDefNode is constructed on these paths.
		if !existingFunctionHistoryAdmitted(old, g.pool.history.reuse(old).known) {
			g.pool.history.invalidate("unproved existing function")
		}
	}
	var formalTable *symbolTable
	if old == nil {
		formalTable = g.table
	}
	ticket := g.pool.history.event("FormalParamNode", 11)
	self, e := newFormalParam(g.pool, name, 0, tree, formalTable, module)
	if self != nil {
		g.pool.history.attach(self, ticket)
	}
	if e != nil {
		return nil, e
	}
	if e = g.table.pop(); e != nil {
		return nil, e
	}
	scopeOpen = false
	spec, e := g.builtinApplication("$NonRecursiveFcnSpec", make([]exprNode, 0), tree)
	if e != nil {
		return nil, e
	}
	spec.unbounded = []*formalParamNode{self}
	spec.bounded = params
	spec.tupleBounds = tuples
	spec.ranges = bounds
	var def *userOpDefNode
	var builtin *builtinOpDefNode
	if old == nil {
		ticket := g.pool.history.event("OpDefNode", 5)
		def, e = newUserOpDef(g.pool, name, nil, len(tree.Zero()) != 0, spec, module, g.table, tree)
		if def != nil {
			g.pool.history.attach(def, ticket)
		}
		if e != nil {
			return nil, e
		}
		def.letInLevel = g.letLevel
		if g.unresolvedSum > 0 {
			def.inRecursiveSection = g.unresolved[g.letLevel] > 0
			def.recursiveSection = g.recursiveSection
			module.recursiveSectionDefs = append(module.recursiveSectionDefs, def)
		}
	} else {
		if v, ok := old.(*userOpDefNode); ok {
			def = v
		} else if v, ok := old.(*builtinOpDefNode); ok {
			builtin = v
		}
		if def != nil && def.inRecursive && !def.defined {
			if def.letInLevel != g.letLevel {
				loc, e := g.diagnosticLocation(tree)
				if e != nil {
					return nil, e
				}
				if !name.displayExact {
					return nil, g.unported("generator.mapping.inexact-display-name")
				}
				if e = addError(g.table.errors, 4293, loc, "Recursive function %s defined at wrong LET/IN level.", nameStringParam(name)); e != nil {
					return nil, e
				}
				def = nil
			} else if def.arity == 0 {
				if e = g.endRecursive(def, spec, tree); e != nil {
					return nil, e
				}
			} else {
				loc, e := g.diagnosticLocation(tree)
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4292, loc, "Function %s has operator arguments in its RECURSIVE declaration.", nameParam(def.name)); e != nil {
					return nil, e
				}
			}
		} else {
			loc, e := g.diagnosticLocation(tree)
			if e != nil {
				return nil, e
			}
			if !name.displayExact {
				return nil, g.unported("generator.mapping.inexact-display-name")
			}
			if e = addError(g.table.errors, 4201, loc, "Function name `%s' already defined or declared.", nameStringParam(name)); e != nil {
				return nil, e
			}
		}
	}
	var entry definitionNode
	if def != nil {
		entry = def
	} else if builtin != nil {
		entry = builtin
	}
	if entry != nil {
		module.definitions = append(module.definitions, entry)
		module.bodyDefinitions = append(module.bodyDefinitions, entry)
		if e = g.table.push(c); e != nil {
			return nil, e
		}
		scopeOpen = true
	}
	g.functions = append(g.functions, functionFrame{name: name, application: spec})
	defer func() { g.functions = g.functions[:len(g.functions)-1] }()
	g.labelScopes = append(g.labelScopes, labelFrame{})
	// The function body is stored unchanged (see nullableExpression).
	g.nullResultConsumer = true
	body, e := g.expressionWithBindings(children[len(children)-1], flattenFormal(params), depth+1)
	labels := g.labelScopes[len(g.labelScopes)-1].labels
	g.labelScopes = g.labelScopes[:len(g.labelScopes)-1]
	if e != nil {
		return nil, e
	}
	if def != nil {
		def.labels = labels
	} else if builtin != nil {
		builtin.labels = labels
	}
	spec.operands = []exprOrOpArg{body}
	if spec.operator.header().name.text == "$NonRecursiveFcnSpec" {
		spec.unbounded = nil
	}
	return entry, nil
}

func (g *generator) exceptExpression(tree *generated.SyntaxTreeNode, children []*generated.SyntaxTreeNode, depth int) (exprNode, error) {
	if len(children) < 5 || len(children)%2 == 0 {
		return nil, g.unported("generator.construct.except-expression")
	}
	count := (len(children) - 3) / 2
	operands := make([]exprNode, count+1)
	base, e := g.expression(children[1], depth+1)
	if e != nil {
		return nil, e
	}
	operands[0] = base
	outer, e := g.builtinApplication("$Except", operands, tree)
	if e != nil {
		return nil, e
	}
	for i := 0; i < count; i++ {
		specTree := children[3+2*i]
		parts := specTree.Heirs()
		if len(parts) < 4 {
			return nil, g.unported("generator.construct.except-expression")
		}
		components := make([]exprNode, len(parts)-3)
		for j := range components {
			sub := parts[1+j].Heirs()
			if len(sub) < 2 {
				return nil, g.unported("generator.construct.except-expression")
			}
			if sub[0].Kind() == 97 {
				if len(sub) > 3 {
					n := (len(sub) - 1) / 2
					indexes := make([]exprNode, n)
					for k := range indexes {
						indexes[k], e = g.expression(sub[1+2*k], depth+1)
						if e != nil {
							return nil, e
						}
					}
					if 2*j+1 >= len(parts) {
						return nil, g.unported("generator.construct.except-expression")
					}
					components[j], e = g.builtinApplication("$Tuple", indexes, parts[2*j+1])
					if e != nil {
						return nil, e
					}
				} else {
					components[j], e = g.expression(sub[1], depth+1)
					if e != nil {
						return nil, e
					}
				}
			} else {
				ticket := g.pool.history.event("StringNode", 18)
				var field *stringNode
				field, e = newString(g.pool, sub[1], false)
				if field != nil {
					g.pool.history.attach(field, ticket)
				}
				components[j] = field
				if e != nil {
					return nil, e
				}
			}
		}
		seq, e := g.builtinApplication("$Seq", components, specTree)
		if e != nil {
			return nil, e
		}
		pair, e := g.builtinApplication("$Pair", []exprNode{seq, nil}, specTree)
		if e != nil {
			return nil, e
		}
		g.excepts = append(g.excepts, exceptFrame{except: outer, component: pair})
		rhs, e := g.expression(parts[len(parts)-1], depth+1)
		g.excepts = g.excepts[:len(g.excepts)-1]
		if e != nil {
			return nil, e
		}
		pair.operands[1] = rhs
		outer.operands[i+1] = pair
	}
	return outer, nil
}

func flattenFormal(groups [][]*formalParamNode) []*formalParamNode {
	var out []*formalParamNode
	for _, group := range groups {
		out = append(out, group...)
	}
	return out
}
func (g *generator) expressionWithBindings(tree *generated.SyntaxTreeNode, params []*formalParamNode, depth int) (exprNode, error) {
	if len(g.labelScopes) == 0 {
		return g.expression(tree, depth)
	}
	i := len(g.labelScopes) - 1
	g.labelScopes[i].paramSeq = append(g.labelScopes[i].paramSeq, params)
	defer func() { g.labelScopes[i].paramSeq = g.labelScopes[i].paramSeq[:len(g.labelScopes[i].paramSeq)-1] }()
	return g.expression(tree, depth)
}
func (g *generator) generateLabel(tree *generated.SyntaxTreeNode, children []*generated.SyntaxTreeNode, depth int) (exprNode, error) {
	if len(g.labelScopes) == 0 {
		loc, e := g.diagnosticLocation(tree)
		if e != nil {
			return nil, e
		}
		if e = addError(g.table.errors, 4333, loc, "Label not in definition or proof step."); e != nil {
			return nil, e
		}
		return g.nullLabel, nil
	}
	for i := 1; i < len(g.apDeclScopes); i++ {
		if g.apDeclScopes[i] {
			loc, e := g.diagnosticLocation(tree)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4334, loc, "Label not allowed within scope of declaration in nested ASSUME/PROVE."); e != nil {
				return nil, e
			}
			return g.nullLabel, nil
		}
	}
	if len(g.excepts) > 0 {
		loc, e := g.diagnosticLocation(tree)
		if e != nil {
			return nil, e
		}
		if e = addError(g.table.errors, 4335, loc, "Labels inside EXCEPT clauses are not yet implemented."); e != nil {
			return nil, e
		}
		return g.nullLabel, nil
	}
	if len(children) != 3 {
		return nil, g.unported("generator.construct.generate-label")
	}
	if tree != g.javaLabel {
		g.pool.java.invalidate()
	} else {
		g.pool.java.event()
	} // leaf application
	g.labelScopes = append(g.labelScopes, labelFrame{})
	var body levelNode
	var e error
	isAP := children[2].Kind() == generated.N_AssumeProve
	if isAP {
		body, e = g.assumeProve(children[2], g.table.module, nil, depth+1)
	} else {
		body, e = g.expression(children[2], depth+1)
	}
	nested := g.labelScopes[len(g.labelScopes)-1].labels
	g.labelScopes = g.labelScopes[:len(g.labelScopes)-1]
	if e != nil {
		return nil, e
	}
	if tree == g.javaLabel {
		app, ok := body.(*opApplNode)
		leaf := children[2].Heirs()[1].Image().String()
		if !ok || app.operator != g.pool.builtins.lookup(contextKey{name: g.pool.intern(leaf)}) || len(app.operands) != 0 || nested != nil {
			g.pool.java.invalidate()
		}
	}
	var id *generated.SyntaxTreeNode
	var argTrees []*generated.SyntaxTreeNode
	if children[0].Kind() == generated.N_GeneralId {
		id = children[0]
	} else if children[0].Kind() == generated.N_OpApplication {
		parts := children[0].Heirs()
		if len(parts) != 2 {
			return nil, g.unported("generator.construct.generate-label")
		}
		id = parts[0]
		args := parts[1].Heirs()
		for i := 1; i < len(args)-1; i += 2 {
			argTrees = append(argTrees, args[i])
		}
	} else {
		return nil, g.unported("generator.construct.generate-label")
	}
	// Generator.java:6869 and 6880 read only heirs()[1] of the label's
	// GeneralId, so an IdPrefix (A!B!lbl) is ignored, not rejected.
	idParts := id.Heirs()
	if len(idParts) != 2 {
		return nil, g.unported("generator.construct.generate-label")
	}
	name, e := g.name(idParts[1])
	if e != nil {
		return nil, e
	}
	if !name.displayExact {
		return nil, g.unported("generator.mapping.inexact-display-name")
	}
	params := make([]*formalParamNode, len(argTrees))
	for i, arg := range argTrees {
		parts := arg.Heirs()
		// Generator.java:6894 likewise reads only heirs()[1].
		if arg.Kind() != generated.N_GeneralId || len(parts) != 2 {
			return nil, g.unported("generator.construct.generate-label")
		}
		pn, e := g.name(parts[1])
		if e != nil {
			return nil, e
		}
		symbol, e := g.table.resolve(pn, false)
		if e != nil {
			return nil, e
		}
		if p, ok := symbol.(*formalParamNode); ok {
			if !g.pool.history.reuse(p).known {
				g.pool.history.invalidate("unproved label formal")
			}
			if !g.javaFormals[p] {
				g.pool.java.invalidate()
			}
			params[i] = p
		} else {
			g.pool.java.invalidate()
			if !pn.displayExact {
				return nil, g.unported("generator.mapping.inexact-display-name")
			}
			loc, e := g.diagnosticLocation(arg)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4332, loc, "Illegal parameter %s of label `%s'.", nameStringParam(pn), nameStringParam(name)); e != nil {
				return nil, e
			}
			ticket := g.pool.history.event("FormalParamNode", 11)
			params[i], e = newFormalParam(g.pool, pn, 0, arg, nil, g.table.module)
			if params[i] != nil {
				g.pool.history.attach(params[i], ticket)
			}
			if e != nil {
				return nil, e
			}
		}
	}
	g.pool.java.event() // label node
	ticket := g.pool.history.event("LabelNode", 29)
	h, e := newExprHeader(g.pool, 29, tree)
	if e != nil {
		return nil, e
	}
	label := &labelNode{exprHeader: h, name: name, params: params, body: body, labels: nested, isAssumeProve: isAP, goal: g.currentGoal, goalClause: g.goalClause}
	g.pool.history.attach(label, ticket)
	frame := &g.labelScopes[len(g.labelScopes)-1]
	if len(frame.paramSeq) != 0 {
		g.pool.java.invalidate()
	}
	g.pool.java.atCheck = g.pool.java.count
	remaining := make(map[*formalParamNode]bool)
	for _, p := range params {
		if remaining[p] {
			loc, e := g.diagnosticLocation(tree)
			if e != nil {
				return nil, e
			}
			if p.name == nil || !p.name.displayExact {
				return nil, g.unported("generator.construct.generate-label")
			}
			if e = addError(g.table.errors, 4330, loc, "Repeated formal parameter %s \nin label `%s'.", nameStringParam(p.name), nameStringParam(name)); e != nil {
				return nil, e
			}
		}
		remaining[p] = true
	}
	for _, group := range frame.paramSeq {
		for _, p := range group {
			if !remaining[p] {
				loc, e := g.diagnosticLocation(tree)
				if e != nil {
					return nil, e
				}
				if p.name == nil || !p.name.displayExact {
					return nil, g.unported("generator.construct.generate-label")
				}
				if e = addError(g.table.errors, 4331, loc, "Label %s must contain formal parameter `%s'.", nameStringParam(name), nameStringParam(p.name)); e != nil {
					return nil, e
				}
			}
			delete(remaining, p)
		}
	}
	// Same-name residuals need no identity-order evidence.
	var extraName string
	same, first := true, true
	for p := range remaining {
		if p.name == nil || !p.name.displayExact {
			return nil, g.unported("generator.construct.generate-label")
		}
		if !first && p.name.text != extraName {
			same = false
		}
		extraName, first = p.name.text, false
	}
	if len(remaining) > 0 {
		var names []string
		if same {
			for range remaining {
				names = append(names, extraName)
			}
		} else if ordered, ok := g.pool.java.orderedLabelExtras(params, remaining); ok {
			for _, p := range ordered {
				names = append(names, p.name.text)
			}
		} else if names, ok = labelExtrasStableOrder(remaining); ok {
			g.labelOrderFallbacks++
		} else {
			return nil, g.unported("generator.construct.generate-label")
		}
		loc, e := g.diagnosticLocation(tree)
		if e != nil {
			return nil, e
		}
		text := "Label " + name.text + " declares extra parameter(s)  "
		for _, n := range names {
			text += n + "  "
		}
		if e = addError(g.table.errors, 4332, loc, "%s", exactStringParam(text)); e != nil {
			return nil, e
		}
	}

	if frame.labels == nil {
		frame.labels = newLabelSet(g.pool)
	}
	if frame.labels.get(name) != nil {
		loc, e := g.diagnosticLocation(tree)
		if e != nil {
			return nil, e
		}
		if e = addError(g.table.errors, 4336, loc, "Duplicate label `%s'.", nameStringParam(name)); e != nil {
			return nil, e
		}
	} else {
		frame.labels.put(name, label)
	}
	return label, nil
}

type labelSet struct {
	items map[*name]*labelNode
	order *context
}

func newLabelSet(pool *namePool) *labelSet {
	return &labelSet{items: make(map[*name]*labelNode), order: newContext(pool)}
}
func (s *labelSet) get(name *name) *labelNode {
	if s == nil {
		return nil
	}
	return s.items[name]
}
func (s *labelSet) put(name *name, value *labelNode) {
	if s.items[name] == nil {
		s.order.recordKey(contextKey{name: name})
	}
	s.items[name] = value
}
func (s *labelSet) ordered() []*labelNode {
	if s == nil {
		return nil
	}
	out := make([]*labelNode, 0, len(s.items))
	for i := len(s.order.order) - 1; i >= 0; i-- {
		for j := len(s.order.order[i]) - 1; j >= 0; j-- {
			out = append(out, s.items[s.order.order[i][j].name])
		}
	}
	return out
}

// structuralIndex implements SANY's ArgNum for numbers, !<< and !>>.
func structuralIndex(selector *generated.SyntaxTreeNode, arity int) (int, string, error) {
	parts := selector.Heirs()
	if len(parts) == 0 && selector.Image() != nil {
		name := selector.Image().String()
		if name == "<<" && arity > 0 {
			return 1, name, nil
		}
		if name == ">>" && arity == 2 {
			return 2, name, nil
		}
		if name == "<<" || name == ">>" {
			return -1, name, nil
		}
		return 0, "", ErrPortIncomplete
	}
	if len(parts) != 1 || len(parts[0].Heirs()) != 1 || parts[0].Heirs()[0].Image() == nil {
		return 0, "", ErrPortIncomplete
	}
	text := parts[0].Heirs()[0].Image().String()
	number, e := strconv.ParseInt(text, 10, 32)
	if e != nil {
		return 0, "", ErrPortIncomplete
	}
	if number <= 0 || number > int64(arity) {
		return -1, "argument selector " + text, nil
	}
	return int(number), "argument selector " + text, nil
}
func (g *generator) badStructuralIndex(selector *generated.SyntaxTreeNode, text string) (exprNode, error) {
	loc, e := g.diagnosticLocation(selector)
	if e != nil {
		return nil, e
	}
	if e = addError(g.table.errors, 4005, loc, "Nonexistent operand specified by `%s'.", asciiStringParam(text)); e != nil {
		return nil, e
	}
	return g.nullOAN, nil
}

// nullSelected reports the source isNullSelection diagnostic at the
// selector that selected a null operand, using selectorItemToString's text.
func (g *generator) nullSelected(selector *generated.SyntaxTreeNode) (exprNode, error) {
	var text string
	if selector.Kind() == generated.N_OpArgs {
		text = "!(...)"
	} else if len(selector.Heirs()) == 0 && selector.Image() != nil {
		text = selector.Image().String()
	} else {
		_, rendered, err := structuralIndex(selector, int(^uint(0)>>1))
		if err != nil {
			return nil, err
		}
		text = rendered
	}
	loc, err := g.diagnosticLocation(selector)
	if err != nil {
		return nil, err
	}
	if err = addError(g.table.errors, 4003, loc, "An unexpected null node specified by '%s'.\nThis is probably due to a previous error.", asciiStringParam(text)); err != nil {
		return nil, err
	}
	return g.nullOAN, nil
}

func (g *generator) rejectFollowingColon(selectors []*generated.SyntaxTreeNode, index int) (bool, error) {
	if index+1 >= len(selectors) {
		return false, nil
	}
	next := selectors[index+1]
	if next.Kind() != generated.N_StructOp || len(next.Heirs()) != 0 || next.Image() == nil || next.Image().String() != ":" {
		return false, nil
	}
	loc, e := g.diagnosticLocation(selectors[index])
	if e != nil {
		return false, e
	}
	if e = addError(g.table.errors, 4005, loc, "!: should not follow an operand selector."); e != nil {
		return false, e
	}
	return true, nil
}

// numericSubexpression follows structural selectors through supported nodes.
func (g *generator) numericSubexpression(tree *generated.SyntaxTreeNode, selectors []*generated.SyntaxTreeNode, root leafNode, start exprNode, params []*formalParamNode, args []exprOrOpArg, depth, operatorArity int, allowColon bool) (exprOrOpArg, error) {
	// selectorToNode builds fresh params and allArgs vectors for each call.
	// Appending into a caller's spare capacity would mutate a prior wrapper's
	// formals or operands when a later selection traverses another binder.
	params = append([]*formalParamNode(nil), params...)
	args = append([]exprOrOpArg(nil), args...)
	current := start
	var pending *opArgNode
	for index := 0; index < len(selectors); index++ {
		selector := selectors[index]
		if operatorArity != 0 && selector.Kind() == generated.N_OpArgs {
			return g.rejectSelectorArgumentsInOperatorMode(selector)
		}
		if selector.Kind() == generated.N_StructOp && len(selector.Heirs()) == 0 && selector.Image() != nil && selector.Image().String() == ":" {
			if !allowColon || index != 0 || len(selectors) != 1 {
				loc, e := g.diagnosticLocation(selector)
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4005, loc, "`!:' can be used only after a name and either at the end after an\noperator name or before an operator name."); e != nil {
					return nil, e
				}
				return g.nullOAN, nil
			}
			if current == nil {
				// The admitted colon leaves curNode unchanged; the
				// isNullSelection test that follows reports the null body
				// (Generator.java:1128-1136, 1544-1546).
				return g.nullSelected(selector)
			}
			continue
		}
		if e := g.check(depth + 1); e != nil {
			return nil, e
		}
		if current == nil && pending == nil {
			loc, e := g.diagnosticLocation(selector)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4005, loc, "Subexpression selection failed, probably due to an error in the selected expression."); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		if pending != nil {
			lambda, ok := pending.operator.(*userOpDefNode)
			if !ok || lambda.name == nil || lambda.name.text != "LAMBDA" {
				loc, e := g.diagnosticLocation(selector)
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4005, loc, "Trying to select subexpression of an operator argument."); e != nil {
					return nil, e
				}
				return g.nullOAN, nil
			}
			at := selector.Kind() == generated.N_StructOp && len(selector.Heirs()) == 0 && selector.Image() != nil && selector.Image().String() == "@"
			if selector.Kind() != generated.N_OpArgs && !at {
				loc, e := g.diagnosticLocation(selector)
				if e != nil {
					return nil, e
				}
				name, e := g.name(selector)
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4005, loc, "Cannot use !%s to select subexpression of a LAMBDA.", nameStringParam(name)); e != nil {
					return nil, e
				}
				return g.nullOAN, nil
			}
			if selector.Kind() == generated.N_OpArgs {
				parts := selector.Heirs()
				if len(parts) < 2 || (len(parts) != 2 && len(parts)%2 == 0) {
					return nil, g.unported("generator.construct.numeric-subexpression")
				}
				count := (len(parts) - 1) / 2
				if count != lambda.arity {
					loc, e := g.diagnosticLocation(selector)
					if e != nil {
						return nil, e
					}
					if e = addError(g.table.errors, 4005, loc, "Selector with %s arguments used for LAMBDA expression taking %s arguments.", Parameter{Kind: IntParameter, Integer: int64(count)}, Parameter{Kind: IntParameter, Integer: int64(lambda.arity)}); e != nil {
						return nil, e
					}
					return g.nullOAN, nil
				}
				for i := 1; i < len(parts)-1; i += 2 {
					argument, e := g.expression(parts[i], depth+1)
					if e != nil {
						return nil, e
					}
					args = append(args, argument)
				}
			}
			params = append(params, lambda.params...)
			current = lambda.body
			pending = nil
			if current == nil {
				return g.nullSelected(selector)
			}
			if rejected, e := g.rejectFollowingColon(selectors, index); e != nil {
				return nil, e
			} else if rejected {
				return g.nullOAN, nil
			}
			continue
		}
		for {
			label, ok := current.(*labelNode)
			if !ok {
				break
			}
			if label.body == nil {
				// isNullSelection after unwrapping a label (Generator.java:1527-1530, 1544-1546, 1553-1556).
				if index == 0 {
					// SANY's stepped-back idx (Generator.java:1530-1544) names the selector
					// before this one; at suffix index 0 that is sel.ops[0], the root name,
					// but callers may start mid-chain, so it is not provable: incomplete.
					return nil, g.unported("generator.construct.label-null-body-first-selector")
				}
				return g.nullSelected(selectors[index-1])
			}
			current = label.expressionBody()
		}
		switch node := current.(type) {
		case *letInNode:
			number := -1
			var e error
			if selector.Kind() != generated.N_OpArgs {
				number, _, e = structuralIndex(selector, 1)
			}
			if e != nil {
				return nil, e
			}
			if number != 1 {
				loc, e := g.diagnosticLocation(selector)
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4005, loc, "A LET/IN expression has only one operand."); e != nil {
					return nil, e
				}
				return g.nullOAN, nil
			}
			if node.body == nil {
				return g.nullSelected(selector)
			}
			current = node.body
			if rejected, e := g.rejectFollowingColon(selectors, index); e != nil {
				return nil, e
			} else if rejected {
				return g.nullOAN, nil
			}
			continue
		case *numeralNode, *decimalNode, *stringNode, *atNode:
			loc, e := g.diagnosticLocation(selector)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4005, loc, "Selecting subexpression of expression that has none."); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		application, ok := current.(*opApplNode)
		if !ok || application.operator == nil {
			return nil, g.unported("generator.construct.numeric-subexpression")
		}
		if selector.Kind() == generated.N_StructOp && len(selector.Heirs()) == 0 && selector.Image() != nil && selector.Image().String() == "@" {
			if _, ok := application.operator.(*builtinOpDefNode); !ok {
				// ArgNum(AtSel, arity) is -1 (Generator.java:1166-1175); any
				// other operator kind has no operand (1394-1400).
				kind, known := selectorOperatorKind(application.operator)
				if !known {
					return nil, g.unported("generator.construct.numeric-subexpression")
				}
				if !selectsOperandByArity(kind) {
					return g.noOperandSelection(selector)
				}
				return g.badStructuralIndex(selector, "@")
			}
			if len(application.bounded) != 0 {
				params = append(params, flattenFormal(application.bounded)...)
			} else if len(application.unbounded) != 0 {
				params = append(params, application.unbounded...)
			} else {
				return g.badStructuralIndex(selector, "@")
			}
			if len(application.operands) != 1 {
				return nil, g.unported("generator.construct.numeric-subexpression")
			}
			if application.operands[0] == nil {
				return g.nullSelected(selector)
			}
			current, ok = application.operands[0].(exprNode)
			if !ok {
				return nil, g.unported("generator.construct.numeric-subexpression")
			}
			if current == nil {
				return g.nullSelected(selector)
			}
			if rejected, e := g.rejectFollowingColon(selectors, index); e != nil {
				return nil, e
			} else if rejected {
				return g.nullOAN, nil
			}
			continue
		}
		if selector.Kind() == generated.N_OpArgs {
			if _, ok := application.operator.(*builtinOpDefNode); !ok {
				// ArgNum(NullSel, arity) is -1 (Generator.java:1166-1175); any
				// other operator kind has no operand (1394-1400).
				kind, known := selectorOperatorKind(application.operator)
				if !known {
					return nil, g.unported("generator.construct.numeric-subexpression")
				}
				if !selectsOperandByArity(kind) {
					return g.noOperandSelection(selector)
				}
				return g.badStructuralIndex(selector, "!(...)")
			}
			var bound []*formalParamNode
			if len(application.bounded) != 0 {
				bound = flattenFormal(application.bounded)
			} else {
				bound = application.unbounded
			}
			if len(bound) == 0 {
				return g.badStructuralIndex(selector, "!(...)")
			}
			parts := selector.Heirs()
			if len(parts) < 3 || len(parts)%2 == 0 {
				return nil, g.unported("generator.construct.numeric-subexpression")
			}
			count := (len(parts) - 1) / 2
			if count != len(bound) {
				loc, e := g.diagnosticLocation(selector)
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4005, loc, "Selector with %s argument(s) used for quantifier with %s bound identifier(s).", Parameter{Kind: IntParameter, Integer: int64(count)}, Parameter{Kind: IntParameter, Integer: int64(len(bound))}); e != nil {
					return nil, e
				}
				return g.nullOAN, nil
			}
			params = append(params, bound...)
			for i := 1; i < len(parts)-1; i += 2 {
				argument, e := g.expression(parts[i], depth+1)
				if e != nil {
					return nil, e
				}
				args = append(args, argument)
			}
			if len(application.operands) != 1 {
				return nil, g.unported("generator.construct.numeric-subexpression")
			}
			if application.operands[0] == nil {
				return g.nullSelected(selector)
			}
			current, ok = application.operands[0].(exprNode)
			if !ok {
				return nil, g.unported("generator.construct.numeric-subexpression")
			}
			if current == nil {
				return g.nullSelected(selector)
			}
			if rejected, e := g.rejectFollowingColon(selectors, index); e != nil {
				return nil, e
			} else if rejected {
				return g.nullOAN, nil
			}
			continue
		}
		if len(application.bounded) != 0 {
			number, text, e := structuralIndex(selector, len(application.ranges))
			if e != nil {
				return nil, e
			}
			if number < 0 {
				return g.badStructuralIndex(selector, text)
			}
			current = application.ranges[number-1]
			if current == nil {
				return g.nullSelected(selector)
			}
			if rejected, e := g.rejectFollowingColon(selectors, index); e != nil {
				return nil, e
			} else if rejected {
				return g.nullOAN, nil
			}
			continue
		}
		if len(application.unbounded) != 0 {
			number, text, e := structuralIndex(selector, len(application.ranges))
			if e != nil {
				return nil, e
			}
			if number < 0 {
				return g.badStructuralIndex(selector, text)
			}
			current = application.ranges[number-1]
			if current == nil {
				return g.nullSelected(selector)
			}
			if rejected, e := g.rejectFollowingColon(selectors, index); e != nil {
				return nil, e
			} else if rejected {
				return g.nullOAN, nil
			}
			continue
		}
		// A FormalParamKind, ConstantDeclKind or UserDefinedOpKind operator
		// selects by ArgNum over its own arity; any other non-built-in kind
		// has no operand (Generator.java:1162-1180, 1394-1400).
		limit := len(application.operands)
		if _, ok := application.operator.(*builtinOpDefNode); !ok {
			kind, known := selectorOperatorKind(application.operator)
			if !known {
				return nil, g.unported("generator.construct.numeric-subexpression")
			}
			if !selectsOperandByArity(kind) {
				return g.noOperandSelection(selector)
			}
			limit = nodeArity(application.operator)
		}
		name := application.operator.header().name
		if name == nil {
			return nil, g.unported("generator.construct.numeric-subexpression")
		}
		number, text, e := structuralIndex(selector, limit)
		if e != nil {
			return nil, e
		}
		if name.text == "$Case" && index == len(selectors)-1 {
			loc, e := g.diagnosticLocation(selector)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4005, loc, "Subexpression of CASE must have form !i!j."); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		if number < 0 {
			return g.badStructuralIndex(selector, text)
		}
		if name.text == "$Except" && number > 1 {
			loc, e := g.diagnosticLocation(selector)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4005, loc, "Selecting subexpression of an EXCEPT not yet implemented."); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		if number > len(application.operands) {
			// curArgs[temp - 1] would throw (Generator.java:1177).
			return nil, g.unported("generator.construct.numeric-subexpression")
		}
		operand := application.operands[number-1]
		if name.text == "$Case" || name.text == "$RcdConstructor" || name.text == "$SetOfRcds" {
			pair, ok := operand.(*opApplNode)
			if !ok || pair.operator == nil || pair.operator.header().name.text != "$Pair" || len(pair.operands) != 2 {
				return nil, g.unported("generator.construct.numeric-subexpression")
			}
			which := 2
			if name.text == "$Case" {
				index++
				if e := g.check(depth + 1); e != nil {
					return nil, e
				}
				which, _, e = structuralIndex(selectors[index], 2)
				if e != nil {
					return nil, e
				}
				if which < 0 {
					loc, e := g.diagnosticLocation(selectors[index])
					if e != nil {
						return nil, e
					}
					if e = addError(g.table.errors, 4005, loc, "Second selector for CASE subexpression must specify  one of two operands."); e != nil {
						return nil, e
					}
					return g.nullOAN, nil
				}
			}
			operand = pair.operands[which-1]
			if operand == nil && name.text == "$Case" {
				loc, e := g.diagnosticLocation(selectors[index])
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4005, loc, "Selecting OTHER in a CASE statement."); e != nil {
					return nil, e
				}
				return g.nullOAN, nil
			}
		}
		if operand == nil {
			return g.nullSelected(selectors[index])
		}
		if opArg, ok := operand.(*opArgNode); ok && opArg != nil {
			if operatorArity < 0 && index == len(selectors)-1 {
				return g.invalidSelectedProofDef(tree)
			}
			if index == len(selectors)-1 {
				loc, e := g.diagnosticLocation(tree)
				if e != nil {
					return nil, e
				}
				if operatorArity == 0 {
					if e = addError(g.table.errors, 4005, loc, "Selected operator argument when expression expected."); e != nil {
						return nil, e
					}
					return g.nullOAN, nil
				}
				arity := len(params) + opArg.arity
				if arity != operatorArity {
					if e = addError(g.table.errors, 4271, loc, "Expected arity %s but found operator of arity %s.", Parameter{Kind: IntParameter, Integer: int64(operatorArity)}, Parameter{Kind: IntParameter, Integer: int64(arity)}); e != nil {
						return nil, e
					}
					return g.nullOAN, nil
				}
				if len(params) != 0 || len(g.selectorSubsts) != 0 {
					// Source wraps the selected operator with synthetic formals,
					// but applies it to the current (empty) op-definition arguments.
					// Model only operator classes with known match behavior.
					selected := opArg.operator
					switch op := selected.(type) {
					case *userOpDefNode:
						if op.arity != len(op.params) || op.name == nil {
							return nil, g.unported("generator.construct.numeric-subexpression")
						}
					case *builtinOpDefNode:
						if op.arity < 0 || op.arity != len(op.params) || op.name == nil {
							return nil, g.unported("generator.construct.numeric-subexpression")
						}
					case *formalParamNode:
					case *opDeclNode:
						if op.kind != 2 {
							return nil, g.unported("generator.construct.numeric-subexpression")
						}
					default:
						return nil, g.unported("generator.construct.numeric-subexpression")
					}
					wrappedParams := append([]*formalParamNode{}, params...)
					for i := 0; i < opArg.arity; i++ {
						name := g.pool.intern("NewParam" + strconv.Itoa(i))
						syntax := g.pool.syntax.NewBuiltinSyntaxTreeNode(g.pool.syntax.UniqueStringOfUnits(name.units))
						ticket := g.pool.history.event("FormalParamNode", 11)
						formal, e := newFormalParam(g.pool, name, 0, syntax, nil, g.table.module)
						if formal != nil {
							g.pool.history.attach(formal, ticket)
						}
						if e != nil {
							return nil, e
						}
						wrappedParams = append(wrappedParams, formal)
					}
					// Java's constructor retains this arity-mismatched node and
					// reports the mismatch from match().
					ticket := g.pool.history.event("OpApplNode", 9)
					h, e := newExprHeader(g.pool, 9, tree)
					if e != nil {
						return nil, e
					}
					body := &opApplNode{exprHeader: h, operator: selected, operands: make([]exprOrOpArg, 0), ranges: make([]exprNode, 0)}
					g.pool.history.attach(body, ticket)
					switch op := selected.(type) {
					case *userOpDefNode:
						if len(op.params) != 0 {
							if e = addError(g.table.errors, 4004, loc, "Wrong number of arguments (%s) given to operator '%s', \nwhich requires %s arguments.", Parameter{Kind: IntParameter, Integer: 0}, nameParam(op.name), Parameter{Kind: IntParameter, Integer: int64(len(op.params))}); e != nil {
								return nil, e
							}
						}
					case *builtinOpDefNode:
						if len(op.params) != 0 {
							if e = addError(g.table.errors, 4004, loc, "Wrong number of arguments (%s) given to operator '%s', \nwhich requires %s arguments.", Parameter{Kind: IntParameter, Integer: 0}, nameParam(op.name), Parameter{Kind: IntParameter, Integer: int64(len(op.params))}); e != nil {
								return nil, e
							}
						}
					case *opDeclNode:
						if op.arity != 0 {
							if e = addError(g.table.errors, 4004, loc, "Operator used with the wrong number of arguments."); e != nil {
								return nil, e
							}
						}
					}
					wrapped, e := g.wrapSelectorBody(body)
					if e != nil {
						return nil, e
					}
					return g.selectorLambdaArgument(tree, wrappedParams, wrapped)
				}
				return opArg, nil
			}
			pending = opArg
		} else {
			selected, ok := operand.(exprNode)
			if !ok || selected == nil {
				return nil, g.unported("generator.construct.numeric-subexpression")
			}
			current = selected
		}
		if rejected, e := g.rejectFollowingColon(selectors, index); e != nil {
			return nil, e
		} else if rejected {
			return g.nullOAN, nil
		}
	}
	if operatorArity < 0 {
		return g.invalidSelectedProofDef(tree)
	}
	if operatorArity > 0 {
		if len(params) != operatorArity {
			loc, e := g.diagnosticLocation(tree)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4271, loc, "Expected arity %s but found operator of arity %s.", Parameter{Kind: IntParameter, Integer: int64(operatorArity)}, Parameter{Kind: IntParameter, Integer: int64(len(params))}); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		current, e := g.wrapSelectorBody(current)
		if e != nil {
			return nil, e
		}
		return g.selectorLambdaArgument(tree, params, current)
	}
	return g.selectorWrapper(tree, root, current, params, args)
}

func (g *generator) numericExpression(tree *generated.SyntaxTreeNode, selectors []*generated.SyntaxTreeNode, root leafNode, start exprNode, params []*formalParamNode, args []exprOrOpArg, depth int, allowColon bool) (exprNode, error) {
	result, e := g.numericSubexpression(tree, selectors, root, start, params, args, depth, 0, allowColon)
	if e != nil {
		return nil, e
	}
	expression, ok := result.(exprNode)
	if !ok {
		return nil, g.unported("generator.construct.numeric-expression")
	}
	return expression, nil
}

// selectorLambdaArgument records the two constructors after substitution
// prefixes have been copied into the selected expression.
func (g *generator) selectorLambdaArgument(tree *generated.SyntaxTreeNode, params []*formalParamNode, body exprNode) (exprOrOpArg, error) {
	ticket := g.pool.history.event("OpDefNode", 5)
	lambda, err := newUserOpDef(g.pool, g.pool.intern("LAMBDA"), params, false, body, g.table.module, nil, tree)
	if lambda != nil {
		g.pool.history.attach(lambda, ticket)
	}
	if err != nil {
		return nil, err
	}
	ticket = g.pool.history.event("OpArgNode", 8)
	result, err := newOpArg(g.pool, lambda, tree, g.table.module)
	if result != nil {
		g.pool.history.attach(result, ticket)
	}
	return result, err
}

// selectorWrapper constructs SANY's selected-expression wrapper and performs
// the formal-argument checks made by the OpApplNode constructor.
func (g *generator) selectorWrapper(tree *generated.SyntaxTreeNode, root leafNode, body exprNode, params []*formalParamNode, args []exprOrOpArg) (exprNode, error) {
	var err error
	body, err = g.wrapSelectorBody(body)
	if err != nil {
		return nil, err
	}
	var lambda *userOpDefNode
	if len(params) > 0 {
		ticket := g.pool.history.event("OpDefNode", 5)
		lambda, err = newUserOpDef(g.pool, g.pool.intern("LAMBDA"), params, false, body, g.table.module, nil, tree)
		if lambda != nil {
			g.pool.history.attach(lambda, ticket)
		}
		if err != nil {
			return nil, err
		}
	}
	if len(params) != len(args) {
		loc, e := g.diagnosticLocation(tree)
		if e != nil {
			return nil, e
		}
		if e = addError(g.table.errors, 4004, loc, "Expected %s arguments but found %s.", Parameter{Kind: IntParameter, Integer: int64(len(params))}, Parameter{Kind: IntParameter, Integer: int64(len(args))}); e != nil {
			return nil, e
		}
		return g.nullOAN, nil
	}
	if len(params) == 0 {
		result, e := g.builtinApplication("$Nop", []exprNode{body}, tree)
		if e != nil {
			return nil, e
		}
		result.subExpressionOf = root
		return result, nil
	}
	ticket := g.pool.history.event("OpApplNode", 9)
	result, e := newOpAppl(g.pool, lambda, args, tree)
	if result != nil {
		g.pool.history.attach(result, ticket)
	}
	if e != nil {
		return nil, e
	}
	for i, param := range params {
		if param.arity > 0 {
			if !matchingOpArgOperand(args[i], param.arity) {
				loc, e := g.diagnosticLocation(tree)
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4271, loc, "Argument number %s to operator '%s' \nshould be a %s-parameter operator.", Parameter{Kind: IntParameter, Integer: int64(i + 1)}, nameParam(lambda.name), Parameter{Kind: IntParameter, Integer: int64(param.arity)}); e != nil {
					return nil, e
				}
			}
		}
	}
	result.subExpressionOf = root
	return result, nil
}

// nestedLetExpression carries each enclosing definition's actual arguments
// and formal identities through alternating LET-context selections.
func (g *generator) nestedLetExpression(tree *generated.SyntaxTreeNode, root *userOpDefNode, rootBody exprNode, prefix []*generated.SyntaxTreeNode, last, finalArgs *generated.SyntaxTreeNode, rootArgs []exprOrOpArg, depth int) (result exprNode, err error) {
	mark := len(g.selectorSubsts)
	defer func() {
		if err != nil {
			g.selectorSubsts = g.selectorSubsts[:mark]
		}
	}()
	if len(prefix) < 4 || len(prefix)%2 != 0 || last.Kind() != 289 { // IDENTIFIER
		return nil, g.unported("generator.construct.nested-let-expression")
	}
	params := append([]*formalParamNode{}, root.params...)
	args := append([]exprOrOpArg{}, rootArgs...)
	current := root
	for i := 1; i < len(prefix); i += 2 {
		if e := g.check(depth + i); e != nil {
			return nil, e
		}
		colon := prefix[i].Heirs()
		if len(colon) != 2 || colon[0].Kind() != generated.N_StructOp || len(colon[0].Heirs()) != 0 || colon[0].Image() == nil || colon[0].Image().String() != ":" {
			return nil, g.unported("generator.construct.nested-let-expression")
		}
		named, syntaxArgs := last, finalArgs
		if i+1 < len(prefix) {
			parts := prefix[i+1].Heirs()
			if (len(parts) != 2 && len(parts) != 3) || parts[0].Kind() != 289 { // IDENTIFIER
				return nil, g.unported("generator.construct.nested-let-expression")
			}
			named, syntaxArgs = parts[0], nil
			if len(parts) == 3 {
				if parts[1].Kind() != generated.N_OpArgs {
					return nil, g.unported("generator.construct.nested-let-expression")
				}
				syntaxArgs = parts[1]
			}
		}
		body := current.body
		if i == 1 {
			body = rootBody
		} else {
			var err error
			body, err = g.selectorBody(body, depth+i)
			if err != nil {
				return nil, err
			}
		}
		for labelDepth := depth + i + 1; ; labelDepth++ {
			label, ok := body.(*labelNode)
			if !ok {
				break
			}
			if e := g.check(labelDepth); e != nil {
				return nil, e
			}
			body = label.expressionBody()
		}
		let, ok := body.(*letInNode)
		if !ok {
			if body == nil {
				return nil, g.unported("generator.construct.nested-let-expression")
			}
			loc, e := g.diagnosticLocation(colon[0])
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4005, loc, "A name selector here must be from a LET clause."); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		if let.context == nil {
			return nil, g.unported("generator.construct.nested-let-expression")
		}
		name, e := g.name(named)
		if e != nil {
			return nil, e
		}
		symbol := let.context.lookup(contextKey{name: name})
		if symbol == nil && i+1 == len(prefix) && name.displayExact {
			loc, e := g.diagnosticLocation(named)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4200, loc, "Unknown operator: `%s'.", nameParam(name)); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		local, ok := symbol.(*userOpDefNode)
		if !ok || !local.defined || len(local.params) != local.arity {
			return nil, g.unported("generator.construct.nested-let-expression")
		}
		var trees []*generated.SyntaxTreeNode
		if syntaxArgs != nil {
			parts := syntaxArgs.Heirs()
			for j := 1; j < len(parts)-1; j += 2 {
				trees = append(trees, parts[j])
			}
		}
		if len(trees) != local.arity {
			location := tree
			if syntaxArgs != nil {
				location = syntaxArgs
			}
			return g.badArity(name, local.arity, location)
		}
		localArgs := make([]exprOrOpArg, len(trees))
		for j, argTree := range trees {
			localArgs[j], e = g.expressionOrOperatorArgument(argTree, local.params[j].arity, j, local, named, depth+i+1)
			if e != nil {
				return nil, e
			}
		}
		if i+1 < len(prefix) {
			params = append(params, local.params...)
			args = append(args, localArgs...)
			current = local
			continue
		}
		if len(params) == 0 && len(g.selectorSubsts) == 0 {
			return g.applySelectedOperator(tree, root, local, localArgs)
		}
		selected, params, e := g.cloneSelectedOperator(tree, local, params)
		if e != nil {
			return nil, e
		}
		return g.selectorWrapper(tree, root, selected, params, append(args, localArgs...))
	}
	return nil, g.unported("generator.construct.nested-let-expression")
}

// applySelectedOperator performs the source OpAppl constructor's
// higher-order argument checks for a directly selected operator.
func (g *generator) applySelectedOperator(tree *generated.SyntaxTreeNode, root leafNode, local *userOpDefNode, args []exprOrOpArg) (exprNode, error) {
	ticket := g.pool.history.event("OpApplNode", 9)
	if len(local.params) != len(args) {
		return nil, g.unported("generator.construct.apply-selected-operator")
	}
	for i, param := range local.params {
		if param.arity > 0 {
			if !matchingOpArgOperand(args[i], param.arity) {
				loc, e := g.diagnosticLocation(tree)
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4271, loc, "Argument number %s to operator '%s' \nshould be a %s-parameter operator.", Parameter{Kind: IntParameter, Integer: int64(i + 1)}, nameParam(local.name), Parameter{Kind: IntParameter, Integer: int64(param.arity)}); e != nil {
					return nil, e
				}
			}
		}
	}
	result, e := newOpAppl(g.pool, local, args, tree)
	if result != nil {
		g.pool.history.attach(result, ticket)
	}
	if e != nil {
		return nil, e
	}
	result.subExpressionOf = root
	return result, nil
}

// selectedLocalExpression applies a LET-local operator after carrying the
// formals and actual arguments accumulated by preceding selections.
func (g *generator) selectedLocalExpression(tree, named, syntaxArgs *generated.SyntaxTreeNode, root leafNode, local *userOpDefNode, params []*formalParamNode, args []exprOrOpArg, depth int) (exprNode, error) {
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
		return g.badArity(local.name, local.arity, location)
	}
	localArgs := make([]exprOrOpArg, len(trees))
	for i, arg := range trees {
		var e error
		localArgs[i], e = g.expressionOrOperatorArgument(arg, local.params[i].arity, i, local, named, depth+1)
		if e != nil {
			return nil, e
		}
	}
	if len(params) == 0 && len(g.selectorSubsts) == 0 {
		return g.applySelectedOperator(tree, root, local, localArgs)
	}
	body, allParams, e := g.cloneSelectedOperator(tree, local, params)
	if e != nil {
		return nil, e
	}
	allArgs := append(append([]exprOrOpArg{}, args...), localArgs...)
	return g.selectorWrapper(tree, root, body, allParams, allArgs)
}

// cloneSelectedOperator implements the FindingOpName constructor sequence:
// clone each formal, construct its argument, then apply the selected operator.
func (g *generator) cloneSelectedOperator(tree *generated.SyntaxTreeNode, local *userOpDefNode, params []*formalParamNode) (*opApplNode, []*formalParamNode, error) {
	allParams := append([]*formalParamNode{}, params...)
	clonedArgs := make([]exprOrOpArg, len(local.params))
	for i, param := range local.params {
		ticket := g.pool.history.event("FormalParamNode", 11)
		clone, err := newFormalParam(g.pool, param.name, param.arity, param.tree, nil, g.table.module)
		if clone != nil {
			g.pool.history.attach(clone, ticket)
		}
		if err != nil {
			return nil, nil, err
		}
		allParams = append(allParams, clone)
		if clone.arity == 0 {
			ticket = g.pool.history.event("OpApplNode", 9)
			clonedArgs[i], err = newOpAppl(g.pool, clone, make([]exprOrOpArg, 0), tree)
		} else {
			ticket = g.pool.history.event("OpArgNode", 8)
			clonedArgs[i], err = newOpArg(g.pool, clone, tree, g.table.module)
		}
		if clonedArgs[i] != nil {
			g.pool.history.attach(clonedArgs[i], ticket)
		}
		if err != nil {
			return nil, nil, err
		}
	}
	ticket := g.pool.history.event("OpApplNode", 9)
	body, err := newOpAppl(g.pool, local, clonedArgs, tree)
	if body != nil {
		g.pool.history.attach(body, ticket)
	}
	return body, allParams, err
}

func (g *generator) selectedLocalArgument(tree *generated.SyntaxTreeNode, local *userOpDefNode, params []*formalParamNode, expected int) (exprOrOpArg, error) {
	arity := len(params) + local.arity
	if arity != expected {
		loc, e := g.diagnosticLocation(tree)
		if e != nil {
			return nil, e
		}
		if e = addError(g.table.errors, 4271, loc, "Expected arity %s but found operator of arity %s.", Parameter{Kind: IntParameter, Integer: int64(expected)}, Parameter{Kind: IntParameter, Integer: int64(arity)}); e != nil {
			return nil, e
		}
		return g.nullOAN, nil
	}
	if len(params) == 0 && len(g.selectorSubsts) == 0 {
		ticket := g.pool.history.event("OpArgNode", 8)
		result, err := newOpArg(g.pool, local, tree, g.table.module)
		if result != nil {
			g.pool.history.attach(result, ticket)
		}
		return result, err
	}
	allParams := append([]*formalParamNode{}, params...)
	clonedArgs := make([]exprOrOpArg, len(local.params))
	for i, param := range local.params {
		ticket := g.pool.history.event("FormalParamNode", 11)
		clone, e := newFormalParam(g.pool, param.name, param.arity, param.tree, nil, g.table.module)
		if clone != nil {
			g.pool.history.attach(clone, ticket)
		}
		if e != nil {
			return nil, e
		}
		allParams = append(allParams, clone)
		if clone.arity == 0 {
			ticket = g.pool.history.event("OpApplNode", 9)
			clonedArgs[i], e = newOpAppl(g.pool, clone, make([]exprOrOpArg, 0), tree)
		} else {
			ticket = g.pool.history.event("OpArgNode", 8)
			clonedArgs[i], e = newOpArg(g.pool, clone, tree, g.table.module)
		}
		if clonedArgs[i] != nil {
			g.pool.history.attach(clonedArgs[i], ticket)
		}
		if e != nil {
			return nil, e
		}
	}
	ticket := g.pool.history.event("OpApplNode", 9)
	body, e := newOpAppl(g.pool, local, clonedArgs, tree)
	if body != nil {
		g.pool.history.attach(body, ticket)
	}
	if e != nil {
		return nil, e
	}
	wrapped, e := g.wrapSelectorBody(body)
	if e != nil {
		return nil, e
	}
	ticket = g.pool.history.event("OpDefNode", 5)
	lambda, e := newUserOpDef(g.pool, g.pool.intern("LAMBDA"), allParams, false, wrapped, g.table.module, nil, tree)
	if lambda != nil {
		g.pool.history.attach(lambda, ticket)
	}
	if e != nil {
		return nil, e
	}
	ticket = g.pool.history.event("OpArgNode", 8)
	result, e := newOpArg(g.pool, lambda, tree, g.table.module)
	if result != nil {
		g.pool.history.attach(result, ticket)
	}
	return result, e
}

// qualifiedExpressionHistory checks direct selection of a known qualified
// definition after one or more instance markers. It never interns names while
// preflighting, so normal argument generation controls name allocation order.
func (g *generator) qualifiedExpressionHistory(children []*generated.SyntaxTreeNode, finalArgs *generated.SyntaxTreeNode) bool {
	if !g.pool.history.exact || len(children) != 2 || children[0] == nil || children[1] == nil || children[1].Kind() != 289 || children[1].Image() == nil {
		return false
	}
	prefix := children[0].Heirs()
	if len(prefix) == 0 {
		return false
	}
	var units []uint16
	found := 0
	for _, segment := range prefix {
		p := segment.Heirs()
		if (len(p) != 2 && len(p) != 3) || p[0] == nil || p[0].Kind() != 289 || p[0].Image() == nil {
			return false
		}
		if len(units) > 0 {
			units = append(units, '!')
		}
		units = append(units, p[0].Image().Units()...)
		name := g.pool.lookupUnits(units)
		if name == nil {
			return false
		}
		value, err := g.table.resolve(name, false)
		if err == nil && value == nil {
			return true
		}
		marker, ok := value.(*userOpDefNode)
		if err != nil || !ok || marker.kind != 6 || len(marker.params) != marker.arity || !g.pool.history.reuse(marker).known {
			return false
		}
		count := 0
		if len(p) == 3 {
			if p[1] == nil || p[1].Kind() != generated.N_OpArgs {
				return false
			}
			args := p[1].Heirs()
			if len(args) < 3 || len(args)%2 == 0 {
				return false
			}
			count = (len(args) - 1) / 2
		}
		if found+count != marker.arity {
			return true
		}
		found = marker.arity
	}
	units = append(units, '!')
	units = append(units, children[1].Image().Units()...)
	name := g.pool.lookupUnits(units)
	if name == nil {
		return true
	}
	value, err := g.table.resolve(name, false)
	if err == nil && value == nil {
		return true
	}
	if err != nil || !g.pool.history.reuse(value).known {
		return false
	}
	switch target := value.(type) {
	case *userOpDefNode:
		if (target.kind != 5 && target.kind != 6) || len(target.params) != target.arity {
			return false
		}
	case *thmOrAssumpDefNode:
	default:
		return false
	}
	if finalArgs != nil {
		if finalArgs.Kind() != generated.N_OpArgs {
			return false
		}
		args := finalArgs.Heirs()
		if len(args) < 3 || len(args)%2 == 0 {
			return false
		}
	}
	// Wrong arity and incomplete-instance recovery both return before any
	// selected application is allocated; otherwise its tracked constructor runs.
	return true
}

// simpleDirectTheoremSelectorHistory finds a theorem after any known instance
// markers and checks the represented selector traversal before it allocates.
func (g *generator) simpleDirectTheoremSelectorHistory(children []*generated.SyntaxTreeNode, finalArgs *generated.SyntaxTreeNode) bool {
	if !g.pool.history.exact || len(children) != 2 || children[0] == nil || children[1] == nil {
		return false
	}
	prefix := children[0].Heirs()
	if len(prefix) == 0 {
		return false
	}
	var units []uint16
	carried := 0
	for i, segment := range prefix {
		p := segment.Heirs()
		if (len(p) != 2 && len(p) != 3) || p[0] == nil || p[0].Kind() != 289 || p[0].Image() == nil {
			return false
		}
		if len(units) > 0 {
			units = append(units, '!')
		}
		units = append(units, p[0].Image().Units()...)
		name := g.pool.lookupUnits(units)
		if name == nil {
			return false
		}
		value, err := g.table.resolve(name, false)
		if err != nil {
			return false
		}
		if theorem, ok := value.(*thmOrAssumpDefNode); ok {
			return g.simpleTheoremSelectorHistory(prefix[i:], children[1], finalArgs, theorem, make([]exprOrOpArg, carried), 0)
		}
		marker, ok := value.(*userOpDefNode)
		if !ok || marker.kind != 6 || len(marker.params) != marker.arity || !g.pool.history.reuse(marker).known {
			return false
		}
		count := 0
		if len(p) == 3 {
			if p[1] == nil || p[1].Kind() != generated.N_OpArgs {
				return false
			}
			args := p[1].Heirs()
			if len(args) < 3 || len(args)%2 == 0 {
				return false
			}
			count = (len(args) - 1) / 2
		}
		if carried+count != marker.arity {
			return false
		}
		carried = marker.arity
	}
	return false
}

// simpleNumericSelectorHistory follows source operand selection through known
// ordinary applications and LET bodies. Traversal reuses selected nodes and
// allocates only the final wrapper; special operators and binders need separate proofs.
func (g *generator) simpleNumericSelectorHistory(children []*generated.SyntaxTreeNode) bool {
	if !g.pool.history.exact || len(children) != 2 || children[0] == nil || children[1] == nil {
		return false
	}
	prefix := children[0].Heirs()
	if len(prefix) == 0 {
		return false
	}
	var units []uint16
	carried := 0
	rootIndex := -1
	var root *userOpDefNode
	for i, segment := range prefix {
		parts := segment.Heirs()
		if (len(parts) != 2 && len(parts) != 3) || parts[0] == nil || parts[0].Kind() != 289 || parts[0].Image() == nil {
			return false
		}
		count := 0
		if len(parts) == 3 {
			if parts[1] == nil || parts[1].Kind() != generated.N_OpArgs {
				return false
			}
			args := parts[1].Heirs()
			if len(args) < 3 || len(args)%2 == 0 {
				return false
			}
			count = (len(args) - 1) / 2
		}
		if len(units) > 0 {
			units = append(units, '!')
		}
		units = append(units, parts[0].Image().Units()...)
		name := g.pool.lookupUnits(units)
		if name == nil {
			return false
		}
		value, err := g.table.resolve(name, false)
		op, ok := value.(*userOpDefNode)
		if err != nil || !ok || !g.pool.history.reuse(op).known || len(op.params) != op.arity {
			return false
		}
		if op.kind == 6 {
			if carried+count != op.arity {
				return true
			}
			carried = op.arity
			continue
		}
		if op.kind != 5 {
			return false
		}
		if carried+count != op.arity || !op.defined {
			return true
		}
		root, rootIndex = op, i
		break
	}
	if root == nil {
		return false
	}
	for _, p := range root.params {
		if !g.pool.history.reuse(p).known {
			return false
		}
	}
	selectors := make([]*generated.SyntaxTreeNode, 0, len(prefix))
	for _, p := range prefix[rootIndex+1:] {
		segment := p.Heirs()
		if len(segment) != 2 || segment[0] == nil {
			return false
		}
		selectors = append(selectors, segment[0])
	}
	selectors = append(selectors, children[1])
	current := root.body
	for {
		wrapper, ok := current.(*substInNode)
		if !ok {
			break
		}
		if !g.pool.history.reuse(wrapper).known {
			return false
		}
		current = wrapper.body
	}
	var pending *opArgNode
	skip := false
	for index, selector := range selectors {
		if skip {
			skip = false
			continue
		}
		colon := selector.Kind() == generated.N_StructOp && len(selector.Heirs()) == 0 && selector.Image() != nil && selector.Image().String() == ":"
		if colon {
			// A colon following an operand is rejected before another allocation.
			return index > 0 || len(selectors) == 1
		}
		if pending != nil {
			lambda, ok := pending.operator.(*userOpDefNode)
			if !ok || lambda.name == nil || lambda.name.text != "LAMBDA" {
				return true
			}
			if !g.pool.history.reuse(lambda).known || len(lambda.params) != lambda.arity {
				return false
			}
			for _, p := range lambda.params {
				if !g.pool.history.reuse(p).known {
					return false
				}
			}
			at := selector.Kind() == generated.N_StructOp && len(selector.Heirs()) == 0 && selector.Image() != nil && selector.Image().String() == "@"
			if selector.Kind() != generated.N_OpArgs && !at {
				return true
			}
			if selector.Kind() == generated.N_OpArgs {
				args := selector.Heirs()
				if len(args) < 2 || (len(args) != 2 && len(args)%2 == 0) {
					return false
				}
				if (len(args)-1)/2 != lambda.arity {
					return true
				}
			}
			current = lambda.body
			if current == nil || !g.pool.history.reuse(current).known {
				return false
			}
			pending = nil
			continue
		}
		for {
			label, ok := current.(*labelNode)
			if !ok {
				break
			}
			if !g.pool.history.reuse(label).known {
				return false
			}
			current = label.expressionBody()
		}
		if let, ok := current.(*letInNode); ok {
			idx, _, err := structuralIndex(selector, 1)
			if err != nil || !g.pool.history.reuse(let).known {
				return false
			}
			if idx != 1 {
				return true
			}
			if let.body == nil || !g.pool.history.reuse(let.body).known {
				return false
			}
			current = let.body
			continue
		}
		switch current.(type) {
		case *numeralNode, *decimalNode, *stringNode, *atNode:
			return g.pool.history.reuse(current).known
		}
		app, ok := current.(*opApplNode)
		if !ok || !g.pool.history.reuse(app).known || app.operator == nil {
			return false
		}
		if len(app.bounded) != 0 || len(app.unbounded) != 0 {
			var bound []*formalParamNode
			if len(app.bounded) != 0 {
				bound = flattenFormal(app.bounded)
			} else {
				bound = app.unbounded
			}
			if len(bound) == 0 {
				return false
			}
			for _, p := range bound {
				if !g.pool.history.reuse(p).known {
					return false
				}
			}
			at := selector.Kind() == generated.N_StructOp && len(selector.Heirs()) == 0 && selector.Image() != nil && selector.Image().String() == "@"
			if selector.Kind() == generated.N_OpArgs || at {
				if selector.Kind() == generated.N_OpArgs {
					args := selector.Heirs()
					if len(args) < 3 || len(args)%2 == 0 {
						return false
					}
					if (len(args)-1)/2 != len(bound) {
						return true
					}
				}
				if len(app.operands) != 1 {
					return false
				}
				var ok bool
				current, ok = app.operands[0].(exprNode)
				if !ok || current == nil || !g.pool.history.reuse(current).known {
					return false
				}
				continue
			}
			idx, _, err := structuralIndex(selector, len(app.ranges))
			if err != nil {
				return false
			}
			if idx < 1 {
				return true
			}
			current = app.ranges[idx-1]
			if current == nil || !g.pool.history.reuse(current).known {
				return false
			}
			continue
		}
		switch op := app.operator.(type) {
		case *builtinOpDefNode:
			if op.name == nil {
				return false
			}
			if op.name.text == "$Case" || op.name.text == "$RcdConstructor" || op.name.text == "$SetOfRcds" {
				if op.name.text == "$Case" && index+1 == len(selectors) {
					return true
				}
				number, _, err := structuralIndex(selector, len(app.operands))
				if err != nil {
					return false
				}
				if number < 1 {
					return true
				}
				pair, ok := app.operands[number-1].(*opApplNode)
				if !ok || pair.operator == nil || pair.operator.header().name == nil || pair.operator.header().name.text != "$Pair" || len(pair.operands) != 2 || !g.pool.history.reuse(pair).known {
					return false
				}
				which := 2
				if op.name.text == "$Case" {
					which, _, err = structuralIndex(selectors[index+1], 2)
					if err != nil {
						return false
					}
					if which < 1 {
						return true
					}
					skip = true
				}
				operand := pair.operands[which-1]
				if operand == nil && op.name.text == "$Case" {
					return true
				}
				current, ok = operand.(exprNode)
				if !ok || current == nil || !g.pool.history.reuse(current).known {
					return false
				}
				continue
			}
			if op.name.text == "$Except" {
				number, _, err := structuralIndex(selector, len(app.operands))
				if err != nil {
					return false
				}
				if number < 1 || number > 1 {
					return true
				}
			}
		}
		// Mirrors numericSubexpression: Generator.java:1162-1180, 1394-1400.
		limit := len(app.operands)
		if _, builtin := app.operator.(*builtinOpDefNode); !builtin {
			kind, known := selectorOperatorKind(app.operator)
			if !known {
				return false
			}
			if !selectsOperandByArity(kind) {
				return true // reported before any allocation
			}
			limit = nodeArity(app.operator)
		}
		idx, _, err := structuralIndex(selector, limit)
		if err != nil {
			return false
		}
		if idx < 1 {
			return true
		}
		if idx > len(app.operands) {
			return false
		}
		if arg, isArg := app.operands[idx-1].(*opArgNode); isArg {
			if arg == nil || !g.pool.history.reuse(arg).known || arg.operator == nil || !g.pool.history.reuse(arg.operator).known {
				return false
			}
			if index+1 == len(selectors) {
				return true
			}
			pending = arg
			continue
		}
		current, ok = app.operands[idx-1].(exprNode)
		if !ok || current == nil || !g.pool.history.reuse(current).known {
			return false
		}
	}
	return true
}

// simpleLabelSelectorHistory follows known instance markers and label chains.
// Arguments are generated by the ordinary producers; selection itself reuses
// the labels and formals, then copies any substitution prefixes and wraps.
func (g *generator) simpleLabelSelectorHistory(children []*generated.SyntaxTreeNode, finalArgs *generated.SyntaxTreeNode) bool {
	if !g.pool.history.exact || len(children) != 2 || children[0] == nil || children[1] == nil {
		return false
	}
	prefix := children[0].Heirs()
	if len(prefix) == 0 {
		return false
	}
	argCount := func(args *generated.SyntaxTreeNode) (int, bool) {
		if args == nil {
			return 0, true
		}
		if args.Kind() != generated.N_OpArgs {
			return 0, false
		}
		p := args.Heirs()
		if len(p) < 3 || len(p)%2 == 0 {
			return 0, false
		}
		return (len(p) - 1) / 2, true
	}
	var units []uint16
	carried := 0
	var root *userOpDefNode
	rootIndex := -1
	for i, segment := range prefix {
		parts := segment.Heirs()
		if (len(parts) != 2 && len(parts) != 3) || parts[0] == nil || parts[0].Kind() != 289 || parts[0].Image() == nil {
			return false
		}
		count := 0
		if len(parts) == 3 {
			var ok bool
			count, ok = argCount(parts[1])
			if !ok {
				return false
			}
		}
		if len(units) != 0 {
			units = append(units, '!')
		}
		units = append(units, parts[0].Image().Units()...)
		name := g.pool.lookupUnits(units)
		if name == nil {
			return true
		}
		value, err := g.table.resolve(name, false)
		if err == nil && value == nil {
			return true
		}
		if err == nil {
			switch node := value.(type) {
			case *builtinOpDefNode, *formalParamNode, *opDeclNode:
				return g.pool.history.reuse(node).known
			case *userOpDefNode:
				if node.kind == 37 {
					return g.pool.history.reuse(node).known
				}
			}
		}
		op, ok := value.(*userOpDefNode)
		if err != nil || !ok || !g.pool.history.reuse(op).known || len(op.params) != op.arity {
			return false
		}
		if op.kind == 6 {
			if carried+count != op.arity {
				return true // badArity returns before evaluating this segment's arguments
			}
			carried = op.arity
			continue
		}
		if op.kind != 5 {
			return false
		}
		if carried+count != op.arity {
			return true
		}
		if !op.defined {
			return true
		}
		root, rootIndex = op, i
		break
	}
	if root == nil || rootIndex+1 >= len(prefix)+1 {
		return false
	}
	for _, p := range root.params {
		if !g.pool.history.reuse(p).known {
			return false
		}
	}
	body := root.body
	for {
		wrapper, ok := body.(*substInNode)
		if !ok {
			break
		}
		if !g.pool.history.reuse(wrapper).known {
			return false
		}
		body = wrapper.body
	}
	if body == nil || !g.pool.history.reuse(body).known {
		return false
	}
	type selection struct{ node, args *generated.SyntaxTreeNode }
	selections := make([]selection, 0, len(prefix)-rootIndex)
	for _, segment := range prefix[rootIndex+1:] {
		parts := segment.Heirs()
		if len(parts) != 2 && len(parts) != 3 {
			return false
		}
		item := selection{node: parts[0]}
		if len(parts) == 3 {
			item.args = parts[1]
		}
		selections = append(selections, item)
	}
	selections = append(selections, selection{node: children[1], args: finalArgs})
	labels := root.labels
	current := body
	letReady := false
	afterLocal := false
	for i := 0; i < len(selections); i++ {
		selection := selections[i]
		node := selection.node
		if node == nil {
			return false
		}
		colon := node.Kind() == generated.N_StructOp && len(node.Heirs()) == 0 && node.Image() != nil && node.Image().String() == ":"
		previousLocal := afterLocal
		afterLocal = false
		if colon {
			if !letReady && selection.args == nil && i+1 == len(selections) && previousLocal && (current == nil || g.pool.history.reuse(current).known) {
				// A final colon after a non-final LET-local operator is
				// admitted (prevMode FindingOpName, Generator.java:
				// 1128-1137) and leaves curNode at that operator's body;
				// only the terminal follows.
				continue
			}
			if letReady || selection.args != nil || i+1 >= len(selections) {
				return false
			}
			letReady = true
			continue
		}
		if letReady && node.Kind() == 289 {
			for {
				switch n := current.(type) {
				case *labelNode:
					if !g.pool.history.reuse(n).known {
						return false
					}
					current = n.expressionBody()
				case *substInNode:
					if !g.pool.history.reuse(n).known {
						return false
					}
					current = n.body
				default:
					goto unwrapped
				}
			}
		unwrapped:
			let, ok := current.(*letInNode)
			if !ok || let.context == nil || !g.pool.history.reuse(let).known || node.Image() == nil {
				return false
			}
			name := g.pool.lookupUnits(node.Image().Units())
			if name == nil {
				return true
			}
			local, ok := let.context.lookup(contextKey{name: name}).(*userOpDefNode)
			if !ok || !local.defined || local.kind != 5 || len(local.params) != local.arity || !g.pool.history.reuse(local).known {
				return false
			}
			count, ok := argCount(selection.args)
			if !ok {
				return false
			}
			if count != local.arity {
				return true
			}
			for _, p := range local.params {
				if !g.pool.history.reuse(p).known {
					return false
				}
			}
			current, labels, letReady, afterLocal = local.body, local.labels, false, true
			continue
		}
		if node.Kind() == generated.N_StructOp || node.Kind() == generated.N_OpArgs {
			if selection.args != nil {
				return false
			}
			var second *generated.SyntaxTreeNode
			if i+1 < len(selections) && selections[i+1].args == nil {
				second = selections[i+1].node
			}
			value, isCase, known := g.selectorHistoryCase(current, node, second)
			if !known {
				return false
			}
			if isCase {
				if second == nil && i+1 < len(selections) {
					return false
				}
				if value == nil {
					return true
				}
				current, letReady = value, true
				i++
				continue
			}
			var ok bool
			current, ok = g.selectorHistoryOperand(current, node)
			if !ok {
				return false
			}
			letReady = true
			continue
		}
		if letReady || node.Kind() != 289 || node.Image() == nil {
			return false
		}
		name := g.pool.lookupUnits(node.Image().Units())
		if name == nil {
			return true
		}
		label := labels.get(name)
		if label == nil {
			return name.displayExact
		}
		if !g.pool.history.reuse(label).known {
			return false
		}
		if label.isAssumeProve {
			return g.definitionSelectorHistory(prefix[rootIndex:], children[1], finalArgs, root, root.params, root.body, root.labels, make([]exprOrOpArg, carried), 0)
		}
		count, ok := argCount(selection.args)
		if !ok {
			return false
		}
		if count != len(label.params) {
			return true
		}
		for _, p := range label.params {
			if !g.pool.history.reuse(p).known {
				return false
			}
		}
		current, labels = label, label.labels
	}

	return true
}

// selectorOperatorArgumentHistory checks the represented operator-mode path.
// No selector argument is evaluated in this mode; matching and recovery occur
// before any selected-expression wrapper is constructed.
func (g *generator) selectorOperatorArgumentHistory(children []*generated.SyntaxTreeNode, expected int) bool {
	if !g.pool.history.exact || len(children) != 2 || children[0] == nil || children[1] == nil {
		return false
	}
	prefix := children[0].Heirs()
	if len(prefix) == 0 {
		return false
	}
	var units []uint16
	var root *userOpDefNode
	rootIndex := -1
	for i, segment := range prefix {
		parts := segment.Heirs()
		if len(parts) == 3 {
			return i == 0
		}
		if len(parts) != 2 || parts[0] == nil || parts[0].Kind() != 289 || parts[0].Image() == nil {
			return false
		}
		if len(units) > 0 {
			units = append(units, '!')
		}
		units = append(units, parts[0].Image().Units()...)
		name := g.pool.lookupUnits(units)
		if name == nil {
			return true
		}
		value, err := g.table.resolve(name, false)
		if err == nil && value == nil {
			return true
		}
		if err == nil {
			switch node := value.(type) {
			case *builtinOpDefNode, *formalParamNode, *opDeclNode:
				return g.pool.history.reuse(node).known
			case *userOpDefNode:
				if node.kind == 37 {
					return g.pool.history.reuse(node).known
				}
			}
		}
		if theorem, ok := value.(*thmOrAssumpDefNode); ok && err == nil {
			return g.simpleTheoremSelectorHistory(prefix[i:], children[1], nil, theorem, nil, expected)
		}
		op, ok := value.(*userOpDefNode)
		if err != nil || !ok || !g.pool.history.reuse(op).known || len(op.params) != op.arity {
			return false
		}
		if op.kind == 6 {
			continue
		}
		if op.kind != 5 {
			return false
		}
		if !op.defined {
			return true
		}
		root, rootIndex = op, i
		break
	}
	if root == nil {
		if children[1].Kind() != 289 || children[1].Image() == nil {
			return false
		}
		units = append(units, '!')
		units = append(units, children[1].Image().Units()...)
		name := g.pool.lookupUnits(units)
		if name == nil {
			return true
		}
		value, err := g.table.resolve(name, false)
		if err == nil && value == nil {
			return true
		}
		op, ok := value.(*userOpDefNode)
		return err == nil && ok && op.kind == 5 && g.pool.history.reuse(op).known
	}
	for _, p := range root.params {
		if !g.pool.history.reuse(p).known {
			return false
		}
	}
	current := root.body
	for {
		w, ok := current.(*substInNode)
		if !ok {
			break
		}
		if !g.pool.history.reuse(w).known {
			return false
		}
		current = w.body
	}
	if current == nil || !g.pool.history.reuse(current).known {
		return false
	}
	var selections []*generated.SyntaxTreeNode
	for _, segment := range prefix[rootIndex+1:] {
		parts := segment.Heirs()
		if len(parts) != 2 || parts[0] == nil {
			return false
		}
		selections = append(selections, parts[0])
	}
	selections = append(selections, children[1])
	labels := root.labels
	letReady := false
	for i := 0; i < len(selections); i++ {
		node := selections[i]
		if node == nil {
			return false
		}
		if node.Kind() == generated.N_StructOp && len(node.Heirs()) == 0 && node.Image() != nil && node.Image().String() == ":" {
			if i+1 == len(selections) {
				return i == 0 && !letReady
			}
			if letReady {
				return false
			}
			letReady = true
			continue
		}
		if node.Kind() == 289 && letReady {
			for {
				label, ok := current.(*labelNode)
				if !ok {
					break
				}
				if !g.pool.history.reuse(label).known {
					return false
				}
				current = label.expressionBody()
			}
			let, ok := current.(*letInNode)
			if !ok || let.context == nil || !g.pool.history.reuse(let).known || node.Image() == nil {
				return false
			}
			name := g.pool.lookupUnits(node.Image().Units())
			if name == nil {
				return true
			}
			if let.context.lookup(contextKey{name: name}) == nil {
				return name.displayExact
			}
			local, ok := let.context.lookup(contextKey{name: name}).(*userOpDefNode)
			if !ok || !local.defined || local.kind != 5 || len(local.params) != local.arity || !g.pool.history.reuse(local).known {
				return false
			}
			for _, p := range local.params {
				if !g.pool.history.reuse(p).known {
					return false
				}
			}
			current, labels, letReady = local.body, local.labels, false
			continue
		}
		if node.Kind() == generated.N_OpArgs {
			return true
		}
		if node.Kind() == generated.N_StructOp {
			if i+1 == len(selections) && g.selectorHistoryTerminalOpArg(current, node) {
				return true
			}
			var second *generated.SyntaxTreeNode
			if i+1 < len(selections) {
				second = selections[i+1]
			}
			value, isCase, known := g.selectorHistoryCase(current, node, second)
			if !known {
				return false
			}
			if isCase {
				if value == nil {
					return true
				}
				current, letReady = value, true
				i++
				continue
			}
			var ok bool
			current, ok = g.selectorHistoryOperand(current, node)
			if !ok {
				return false
			}
			letReady = true
			continue
		}
		if letReady || node.Kind() != 289 || node.Image() == nil {
			return false
		}
		name := g.pool.lookupUnits(node.Image().Units())
		if name == nil {
			return true
		}
		label := labels.get(name)
		if label == nil {
			return name.displayExact
		}
		if label.isAssumeProve || !g.pool.history.reuse(label).known {
			return false
		}
		for _, p := range label.params {
			if !g.pool.history.reuse(p).known {
				return false
			}
		}
		current, labels = label, label.labels
	}
	return true
}

// selectorHistoryTerminalOpArg recognizes the reuse or synthetic wrapper of a
// selected operator argument; the constructor sequence is in numericSubexpression.
func (g *generator) selectorHistoryTerminalOpArg(current exprNode, selector *generated.SyntaxTreeNode) bool {
	for {
		label, ok := current.(*labelNode)
		if !ok {
			break
		}
		if !g.pool.history.reuse(label).known {
			return false
		}
		current = label.expressionBody()
	}
	app, ok := current.(*opApplNode)
	if !ok || app.operator == nil || !g.pool.history.reuse(app).known || len(app.bounded) != 0 || len(app.unbounded) != 0 {
		return false
	}
	switch op := app.operator.(type) {
	case *builtinOpDefNode:
		if op.name == nil {
			return false
		}
		switch op.name.text {
		case "$Case", "$Except", "$RcdConstructor", "$SetOfRcds":
			return false
		}
	case *userOpDefNode, *formalParamNode:
		if nodeArity(app.operator) != len(app.operands) {
			return false
		}
	case *opDeclNode:
		if op.kind != 2 || op.arity != len(app.operands) {
			return false
		}
	default:
		return false
	}
	index, _, err := structuralIndex(selector, len(app.operands))
	if err != nil || index < 1 {
		return false
	}
	arg, ok := app.operands[index-1].(*opArgNode)
	if !ok || arg == nil || arg.operator == nil || !g.pool.history.reuse(arg).known || !g.pool.history.reuse(arg.operator).known {
		return false
	}
	switch op := arg.operator.(type) {
	case *userOpDefNode:
		return op.name != nil && op.arity == len(op.params)
	case *builtinOpDefNode:
		return op.name != nil && op.arity >= 0 && op.arity == len(op.params)
	case *formalParamNode:
		return true
	case *opDeclNode:
		return op.kind == 2
	default:
		return false
	}
}

// selectorHistoryCase mirrors the allocation-free two-selector CASE descent.
// A nil value with known=true is a source-defined diagnostic boundary.
func (g *generator) selectorHistoryCase(current exprNode, first, second *generated.SyntaxTreeNode) (value exprNode, isCase, known bool) {
	for {
		label, ok := current.(*labelNode)
		if !ok {
			break
		}
		if !g.pool.history.reuse(label).known {
			return nil, false, false
		}
		current = label.expressionBody()
	}
	app, ok := current.(*opApplNode)
	if !ok || app.operator == nil || app.operator.header().name == nil || app.operator.header().name.text != "$Case" {
		return nil, false, true
	}
	if !g.pool.history.reuse(app).known {
		return nil, true, false
	}
	if second == nil {
		return nil, true, true
	}
	index, _, err := structuralIndex(first, len(app.operands))
	if err != nil {
		return nil, true, false
	}
	if index < 1 {
		return nil, true, true
	}
	pair, ok := app.operands[index-1].(*opApplNode)
	if !ok || pair.operator == nil || pair.operator.header().name == nil || pair.operator.header().name.text != "$Pair" || len(pair.operands) != 2 || !g.pool.history.reuse(pair).known {
		return nil, true, false
	}
	which, _, err := structuralIndex(second, 2)
	if err != nil {
		return nil, true, false
	}
	if which < 1 {
		return nil, true, true
	}
	if pair.operands[which-1] == nil {
		return nil, true, true
	}
	value, ok = pair.operands[which-1].(exprNode)
	return value, true, ok && value != nil && g.pool.history.reuse(value).known
}

// selectorHistoryOperand mirrors the allocation-free operand descent used
// while looking for a LET-local definition. Argument producers run later in
// source order and independently invalidate history on unsupported paths.
func (g *generator) selectorHistoryOperand(current exprNode, selector *generated.SyntaxTreeNode) (exprNode, bool) {
	for {
		label, ok := current.(*labelNode)
		if !ok {
			break
		}
		if !g.pool.history.reuse(label).known {
			return nil, false
		}
		current = label.expressionBody()
	}
	if let, ok := current.(*letInNode); ok {
		index, _, err := structuralIndex(selector, 1)
		if err != nil || index != 1 || !g.pool.history.reuse(let).known || let.body == nil || !g.pool.history.reuse(let.body).known {
			return nil, false
		}
		return let.body, true
	}
	app, ok := current.(*opApplNode)
	if !ok || app.operator == nil || !g.pool.history.reuse(app).known {
		return nil, false
	}
	if len(app.bounded) != 0 || len(app.unbounded) != 0 {
		var bound []*formalParamNode
		if len(app.bounded) > 0 {
			bound = flattenFormal(app.bounded)
		} else {
			bound = app.unbounded
		}
		for _, p := range bound {
			if !g.pool.history.reuse(p).known {
				return nil, false
			}
		}
		at := selector.Kind() == generated.N_StructOp && len(selector.Heirs()) == 0 && selector.Image() != nil && selector.Image().String() == "@"
		if at {
			if len(app.operands) != 1 {
				return nil, false
			}
			expr, ok := app.operands[0].(exprNode)
			return expr, ok && expr != nil && g.pool.history.reuse(expr).known
		}
		if selector.Kind() == generated.N_OpArgs {
			parts := selector.Heirs()
			if len(parts) < 3 || len(parts)%2 == 0 || (len(parts)-1)/2 != len(bound) || len(app.operands) != 1 {
				return nil, false
			}
			expr, ok := app.operands[0].(exprNode)
			return expr, ok && expr != nil && g.pool.history.reuse(expr).known
		}
		index, _, err := structuralIndex(selector, len(app.ranges))
		if err != nil || index < 1 {
			return nil, false
		}
		value := app.ranges[index-1]
		return value, value != nil && g.pool.history.reuse(value).known
	}
	switch op := app.operator.(type) {
	case *builtinOpDefNode:
		if op.name == nil {
			return nil, false
		}
		switch op.name.text {
		case "$Case":
			return nil, false // CASE consumes two selectors.
		case "$RcdConstructor", "$SetOfRcds":
			index, _, err := structuralIndex(selector, len(app.operands))
			if err != nil || index < 1 {
				return nil, false
			}
			pair, ok := app.operands[index-1].(*opApplNode)
			if !ok || pair.operator == nil || pair.operator.header().name == nil || pair.operator.header().name.text != "$Pair" || len(pair.operands) != 2 || !g.pool.history.reuse(pair).known {
				return nil, false
			}
			value, ok := pair.operands[1].(exprNode)
			return value, ok && value != nil && g.pool.history.reuse(value).known
		case "$Except":
			index, _, err := structuralIndex(selector, len(app.operands))
			if err != nil || index != 1 {
				return nil, false
			}
		}
	case *userOpDefNode, *formalParamNode:
		if nodeArity(app.operator) != len(app.operands) {
			return nil, false
		}
	case *opDeclNode:
		if op.kind != 2 || op.arity != len(app.operands) {
			return nil, false
		}
	default:
		return nil, false
	}
	index, _, err := structuralIndex(selector, len(app.operands))
	if err != nil || index < 1 {
		return nil, false
	}
	value, ok := app.operands[index-1].(exprNode)
	return value, ok && value != nil && g.pool.history.reuse(value).known
}

// simpleLabelSelector handles name-only label chains with expression arguments.
// Other selector and higher-order argument forms remain incomplete.
func (g *generator) simpleLabelSelector(tree *generated.SyntaxTreeNode, children []*generated.SyntaxTreeNode, finalArgs *generated.SyntaxTreeNode, depth int) (exprNode, error) {
	return g.simpleLabelSelectorMode(tree, children, finalArgs, depth, false)
}

func (g *generator) simpleLabelSelectorMode(tree *generated.SyntaxTreeNode, children []*generated.SyntaxTreeNode, finalArgs *generated.SyntaxTreeNode, depth int, fact bool) (exprNode, error) {
	outerSubsts := g.selectorSubsts
	g.selectorSubsts = nil
	defer func() { g.selectorSubsts = outerSubsts }()
	prefix := children[0].Heirs()
	if len(prefix) == 0 {
		return nil, g.unported("generator.construct.simple-label-selector-mode")
	}
	first := prefix[0].Heirs()
	if len(first) != 2 && len(first) != 3 {
		return nil, g.unported("generator.construct.simple-label-selector-mode")
	}
	name, e := g.name(first[0])
	if e != nil {
		return nil, e
	}
	symbol, e := g.table.resolve(name, false)
	if e != nil {
		return nil, e
	}
	var instanceArgs []exprOrOpArg
	var leading []*generated.SyntaxTreeNode
	if symbol == nil {
		if fact {
			return g.unresolvedSelector(prefix, children[1], depth)
		}
		found, failure, err := g.unresolvedSelectorPrefix(prefix, children[1], depth)
		if err != nil || found == nil {
			return failure, err
		}
		if found.index == len(prefix) {
			// The final selector resolved: an ordinary application of the
			// compound name to tempArgs (Generator.java:870-946, 1897-1916).
			return g.qualifiedTarget(tree, children[1], finalArgs, found.name, found.symbol, 0, nil, found.leading, depth)
		}
		// A definition or theorem resolved before the final selector: the
		// selection continues from that segment as for a defined first name,
		// with tempArgs preceding the segment's own arguments
		// (Generator.java:870-1000). A resolved instance marker continues the
		// compound search and stays outside this transition.
		switch v := found.symbol.(type) {
		case *userOpDefNode:
			if v.kind != 5 {
				return nil, g.unported("generator.construct.unresolved-selector")
			}
		case *thmOrAssumpDefNode:
		default:
			return nil, g.unported("generator.construct.unresolved-selector")
		}
		prefix = prefix[found.index:]
		first = prefix[0].Heirs()
		name, symbol, leading = found.name, found.symbol, found.leading
	} else if marker, ok := symbol.(*userOpDefNode); ok && marker.kind == 6 {
		// First determine whether the prefix contains an operator definition.
		// If it consists only of instance markers, qualifiedInstance owns
		// argument generation and must do so exactly once.
		var units []uint16
		allMarkers := true
		for _, segment := range prefix {
			parts := segment.Heirs()
			if len(parts) < 2 || parts[0].Kind() != 289 {
				allMarkers = false
				break
			}
			n, err := g.name(parts[0])
			if err != nil {
				return nil, err
			}
			if len(units) != 0 {
				units = append(units, '!')
			}
			units = append(units, n.units...)
			v, err := g.table.resolve(g.pool.internUnits(units), false)
			if err != nil {
				return nil, err
			}
			m, ok := v.(*userOpDefNode)
			if !ok || m.kind != 6 {
				allMarkers = false
				break
			}
		}
		if allMarkers {
			return g.qualifiedInstance(tree, children, finalArgs, depth)
		}
		index, selected, _, carried, failure, err := g.qualifiedSelectorPrefix(tree, prefix, children[1], depth, false)
		if err != nil {
			return nil, err
		}
		if failure != nil {
			return failure, nil
		}
		if index == len(prefix) {
			return g.qualifiedInstance(tree, children, finalArgs, depth)
		}
		prefix = prefix[index:]
		first = prefix[0].Heirs()
		name = selected.header().name
		symbol = selected
		instanceArgs = carried
	}
	if alias, ok := symbol.(*userOpDefNode); ok && alias.kind == 37 {
		return g.rejectProofAliasSubexpression(first[0], alias)
	}
	if result, handled, err := g.rejectDeclaredSubexpression(symbol, first[0]); handled {
		return result, err
	}
	if theorem, ok := symbol.(*thmOrAssumpDefNode); ok {
		if len(leading) != 0 {
			// tempArgs and the segment's own arguments are checked against
			// the theorem's arity before any is generated; tempArgs then
			// become its leading arguments (Generator.java:870-946).
			own := 0
			location := tree
			if len(first) == 3 {
				if first[1].Kind() != generated.N_OpArgs {
					return nil, g.unported("generator.construct.simple-label-selector-mode")
				}
				own = (len(first[1].Heirs()) - 1) / 2
				location = first[1]
			}
			if len(leading)+own != len(theorem.params) {
				return g.badArity(name, len(theorem.params), location)
			}
			for i, t := range leading {
				arg, err := g.expressionOrOperatorArgument(t, theorem.params[i].arity, i, theorem, first[0], depth+1)
				if err != nil {
					return nil, err
				}
				instanceArgs = append(instanceArgs, arg)
			}
		}
		result, err := g.theoremSelector(tree, prefix, children[1], finalArgs, theorem, instanceArgs, depth, 0)
		if err != nil {
			return nil, err
		}
		expr, ok := result.(exprNode)
		if !ok {
			return nil, g.unported("generator.construct.simple-label-selector-mode")
		}
		return expr, nil
	}
	root, ok := symbol.(*userOpDefNode)
	if !ok {
		return nil, g.unported("generator.construct.simple-label-selector-mode")
	}
	argTrees := append([]*generated.SyntaxTreeNode{}, leading...)
	if len(first) == 3 {
		if first[1].Kind() != generated.N_OpArgs {
			return nil, g.unported("generator.construct.simple-label-selector-mode")
		}
		parts := first[1].Heirs()
		for i := 1; i < len(parts)-1; i += 2 {
			argTrees = append(argTrees, parts[i])
		}
	}
	if len(instanceArgs)+len(argTrees) != root.arity {
		location := tree
		if len(first) == 3 {
			location = first[1]
		}
		return g.badArity(name, root.arity-len(instanceArgs), location)
	}
	args := append([]exprOrOpArg{}, instanceArgs...)
	for i, argTree := range argTrees {
		pos := len(instanceArgs) + i
		if pos >= len(root.params) {
			return nil, g.unported("generator.construct.simple-label-selector-mode")
		}
		expected := root.params[pos].arity
		arg, err := g.expressionOrOperatorArgument(argTree, expected, pos, root, first[0], depth+1)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}
	if !root.defined {
		return g.rejectUndefinedSubexpression(root, first[0])
	}
	rootBody, e := g.selectorBody(root.body, depth+1)
	if e != nil {
		return nil, e
	}
	var bound []*formalParamNode
	var boundArgs []*generated.SyntaxTreeNode
	if local, e := g.structuralLetTarget(rootBody, prefix, children[1], &bound, &boundArgs, depth, false); e == nil {
		params := append(append([]*formalParamNode{}, root.params...), bound...)
		for _, syntax := range boundArgs {
			arg, err := g.expression(syntax, depth+1)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
		return g.selectedLocalExpression(tree, children[1], finalArgs, root, local, params, args, depth)
	} else if typedSelectorFailure(e) {
		// The bound arguments of the indices before the failing one were
		// generated first (Generator.java:1349-1373).
		return g.rejectAfterBoundArguments(e, boundArgs, depth)
	} else if e != ErrPortIncomplete {
		return nil, e
	}
	g.incomplete.forget() // the alternative below consumes this origin
	if len(prefix) >= 4 {
		before := g.selectorEffects()
		selected, e := g.nestedLetExpression(tree, root, rootBody, prefix, children[1], finalArgs, args, depth)
		if e == nil {
			return selected, nil
		}
		if e != ErrPortIncomplete || g.selectorEffects() != before {
			// An incomplete attempt that already generated arguments or
			// recorded diagnostics keeps its origin: an alternative would
			// generate the same prefix a second time.
			return nil, e
		}
		g.incomplete.forget() // the alternative below consumes this origin
	}
	// A colon can switch into the context of a LET expression.  Model the
	// source's direct local operator case; other LET selections
	// retain the explicit incomplete boundary.
	if len(prefix) == 2 {
		pieces := prefix[1].Heirs()
		if len(pieces) == 2 && pieces[0].Kind() == generated.N_StructOp && len(pieces[0].Heirs()) == 0 && pieces[0].Image() != nil && pieces[0].Image().String() == ":" && children[1].Kind() == 289 { // IDENTIFIER
			selectedBody := rootBody
			for labelDepth := depth + 1; ; labelDepth++ {
				label, ok := selectedBody.(*labelNode)
				if !ok {
					break
				}
				if e = g.check(labelDepth); e != nil {
					return nil, e
				}
				selectedBody = label.expressionBody()
			}
			let, ok := selectedBody.(*letInNode)
			if !ok {
				if selectedBody == nil {
					return nil, g.unported("generator.construct.simple-label-selector-mode")
				}
				loc, e := g.diagnosticLocation(pieces[0])
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4005, loc, "A name selector here must be from a LET clause."); e != nil {
					return nil, e
				}
				return g.nullOAN, nil
			}
			if let.context == nil {
				return nil, g.unported("generator.construct.simple-label-selector-mode")
			}
			localName, e := g.name(children[1])
			if e != nil {
				return nil, e
			}
			selected := let.context.lookup(contextKey{name: localName})
			if selected == nil {
				if !localName.displayExact {
					return nil, g.unported("generator.mapping.inexact-display-name")
				}
				loc, e := g.diagnosticLocation(children[1])
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4200, loc, "Unknown operator: `%s'.", nameParam(localName)); e != nil {
					return nil, e
				}
				return g.nullOAN, nil
			}
			local, ok := selected.(*userOpDefNode)
			if !ok || !local.defined || len(local.params) != local.arity {
				return nil, g.unported("generator.construct.simple-label-selector-mode")
			}
			var localTrees []*generated.SyntaxTreeNode
			if finalArgs != nil {
				parts := finalArgs.Heirs()
				for i := 1; i < len(parts)-1; i += 2 {
					localTrees = append(localTrees, parts[i])
				}
			}
			if len(localTrees) != local.arity {
				location := tree
				if finalArgs != nil {
					location = finalArgs
				}
				return g.badArity(localName, local.arity, location)
			}
			localArgs := make([]exprOrOpArg, len(localTrees))
			for i, argTree := range localTrees {
				if i >= len(local.params) {
					return nil, g.unported("generator.construct.simple-label-selector-mode")
				}
				localArgs[i], e = g.expressionOrOperatorArgument(argTree, local.params[i].arity, i, local, children[1], depth+1)
				if e != nil {
					return nil, e
				}
			}
			if len(root.params) == 0 && len(g.selectorSubsts) == 0 {
				return g.applySelectedOperator(tree, root, local, localArgs)
			}
			body, params, e := g.cloneSelectedOperator(tree, local, root.params)
			if e != nil {
				return nil, e
			}
			return g.selectorWrapper(tree, root, body, params, append(args, localArgs...))
		}
	}
	if (children[1].Kind() == generated.N_StructOp || children[1].Kind() == generated.N_OpArgs) && finalArgs == nil {
		selectors := make([]*generated.SyntaxTreeNode, 0, len(prefix))
		onlyStructural := true
		for _, part := range prefix[1:] {
			pieces := part.Heirs()
			if len(pieces) != 2 || (pieces[0].Kind() != generated.N_StructOp && pieces[0].Kind() != generated.N_OpArgs) {
				onlyStructural = false
				break
			}
			selectors = append(selectors, pieces[0])
		}
		if onlyStructural {
			selectors = append(selectors, children[1])
			return g.numericExpression(tree, selectors, root, rootBody, root.params, args, depth, true)
		}
	}
	selections := make([]labelSelection, 0, len(prefix))
	for _, item := range prefix[1:] {
		parts := item.Heirs()
		if len(parts) != 2 && len(parts) != 3 {
			return nil, g.unported("generator.construct.simple-label-selector-mode")
		}
		selection := labelSelection{node: parts[0]}
		if len(parts) == 3 {
			selection.args = parts[1]
		}
		selections = append(selections, selection)
	}
	selections = append(selections, labelSelection{node: children[1], args: finalArgs})
	return g.frameContinuation(tree, selectorFrame{subExpr: root, op: root, body: rootBody, params: root.params, args: args}, selections, fact, depth)
}

// frameContinuation runs selectorToNode from a FindingOpName exit: labels of
// the frame's operator (FollowingLabels, 1049-1127), then FindingSubExpr and
// any LET switch through frameSegment, and finally the selected-expression
// terminal. The root frame keeps its old boundary for a structural selector
// straight after the operator unless a non-final LET name follows, since the
// root's structural shapes are handled before this call.
func (g *generator) frameContinuation(tree *generated.SyntaxTreeNode, f selectorFrame, selections []labelSelection, fact bool, depth int) (exprNode, error) {
	root := f.subExpr
	labels := f.op.labels
	var selected *labelNode
	allParams := append([]*formalParamNode{}, f.params...)
	args := append([]exprOrOpArg{}, f.args...)
	var e error
	for index, selection := range selections {
		if selection.node.Kind() == generated.N_StructOp || selection.node.Kind() == generated.N_OpArgs {
			if selected == nil {
				if fact || (!f.let && !nonFinalLetName(selections[index:])) {
					return nil, g.unported("generator.construct.simple-label-selector-mode")
				}
				return g.frameSegment(tree, selectorFrame{subExpr: root, op: f.op, body: f.body, params: allParams, args: args, let: f.let}, f.body, true, selections[index:], fact, depth)
			}
			if selected.body == nil && !isColonSelector(selection.node) {
				// FollowingLabels leaves curNode equal to the label's null
				// body (Generator.java:1116-1122), which this selector
				// reports (1137-1145) before isNullSelection could.
				loc, e := g.diagnosticLocation(selection.node)
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4005, loc, "Subexpression selection failed, probably due to an error in the selected expression."); e != nil {
					return nil, e
				}
				return g.nullOAN, nil
			}
			return g.frameSegment(tree, selectorFrame{subExpr: root, op: f.op, body: f.body, params: allParams, args: args, let: f.let}, selected, false, selections[index:], fact, depth)
		}
		if e = g.check(depth + 1); e != nil {
			return nil, e
		}
		labelName, e := g.name(selection.node)
		if e != nil {
			return nil, e
		}
		if !labelName.displayExact {
			return nil, g.unported("generator.mapping.inexact-display-name")
		}
		selected = labels.get(labelName)
		if selected == nil {
			loc, e := g.diagnosticLocation(selection.node)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4004, loc, "Cannot find label `%s'.", nameStringParam(labelName)); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		illegal, e := g.illegalProofLabelRef(selected, selection.node)
		if e != nil {
			return nil, e
		}
		if illegal {
			loc, e := g.diagnosticLocation(selection.node)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4005, loc, "Accessing subexpression labeled `%s' of ASSUME/PROVE clause within the scope of a declaration\n from outside that declaration's scope.", nameStringParam(labelName)); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		var argNodes []*generated.SyntaxTreeNode
		if selection.args != nil {
			if selection.args.Kind() != generated.N_OpArgs {
				return nil, g.unported("generator.construct.simple-label-selector-mode")
			}
			parts := selection.args.Heirs()
			for i := 1; i < len(parts)-1; i += 2 {
				argNodes = append(argNodes, parts[i])
			}
		}
		if len(selected.params) != len(argNodes) {
			loc, e := g.diagnosticLocation(selection.node)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4337, loc, "Label `%s' used with wrong number of arguments.", nameStringParam(labelName)); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		for _, argNode := range argNodes {
			value, e := g.expression(argNode, depth+1)
			if e != nil {
				return nil, e
			}
			args = append(args, value)
		}
		allParams = append(allParams, selected.params...)
		labels = selected.labels
	}
	if selected != nil && selected.isAssumeProve && !fact {
		loc, err := g.diagnosticLocation(tree)
		if err != nil {
			return nil, err
		}
		if err = addError(g.table.errors, 4355, loc, "ASSUME/PROVE used where an expression is required."); err != nil {
			return nil, err
		}
		return g.nullOAN, nil
	}
	return g.selectorWrapper(tree, root, selected, allParams, args)
}
