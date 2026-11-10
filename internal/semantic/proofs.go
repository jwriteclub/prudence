// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ semantic proof nodes, Copyright 2003 Compaq Corporation
// and Microsoft Corporation, under the MIT license; see LICENSE.
package semantic

import (
	"slices"
	"unicode/utf16"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

// nonLeafProofNode keeps the source's separate step and instance vectors.
// Instances are retained for eventual level checking, not getChildren or XML.
type nonLeafProofNode struct {
	semanticHeader
	steps   []levelNode
	insts   []*instanceNode
	context *context
}

func (n *nonLeafProofNode) children() []levelNode { return append([]levelNode(nil), n.steps...) }
func (n *nonLeafProofNode) walkChildren() []levelNode {
	out := n.children()
	if n.context != nil {
		for i := len(n.context.order) - 1; i >= 0; i-- {
			for j := len(n.context.order[i]) - 1; j >= 0; j-- {
				key := n.context.order[i][j]
				if !key.module {
					out = append(out, n.context.table[key].value)
				}
			}
		}
	}
	return out
}

type useOrHideNode struct {
	semanticHeader
	facts      []levelNode
	defs       []leafNode
	only, hide bool
	stepName   *name
}

func (n *useOrHideNode) children() []levelNode     { return append([]levelNode(nil), n.facts...) }
func (n *useOrHideNode) walkChildren() []levelNode { return n.children() }

type defStepNode struct {
	semanticHeader
	stepNumber *name
	defs       []definitionNode
}

func (n *defStepNode) children() []levelNode {
	out := make([]levelNode, 0, len(n.defs))
	for _, def := range n.defs {
		out = append(out, def)
	}
	return out
}
func (n *defStepNode) walkChildren() []levelNode { return n.children() }

// InstanceNode.getChildren visits substitution expressions. Java's method
// named walkGaph does not override walkGraph, so its graph walk has no edges.
func (n *instanceNode) children() []levelNode {
	out := make([]levelNode, 0, len(n.substs))
	for _, subst := range n.substs {
		if subst == nil {
			out = append(out, nil)
			continue
		}
		child, _ := subst.expr.(levelNode)
		out = append(out, child)
	}
	return out
}
func (n *instanceNode) walkChildren() []levelNode { return nil }

// useOrHide builds the common BY/USE/HIDE record. Fact and DEF selectors that
// cannot yet be represented retain an explicit incomplete result.
func (g *generator) useOrHide(tree *generated.SyntaxTreeNode, module *moduleNode, depth int) (_ *useOrHideNode, resultErr error) {
	defer func() {
		if resultErr != nil {
			g.pool.history.invalidate("proof reference traversal incomplete")
		}
	}()
	if err := g.check(depth); err != nil {
		return nil, err
	}
	if tree == nil || module == nil {
		return nil, g.unported("generator.construct.use-or-hide")
	}
	h := tree.Heirs()
	if len(h) == 0 || h[0] == nil {
		return nil, g.unported("generator.construct.use-or-hide")
	}
	kind, next := 31, 1
	if h[0].Kind() == 69 {
		kind = 32
	}
	if h[0].Kind() == 75 {
		next++
	}
	facts := make([]levelNode, 0)
	defs := make([]leafNode, 0)
	only := false
	if next >= len(h) {
		loc, err := g.diagnosticLocation(tree)
		if err != nil {
			return nil, err
		}
		if err = addError(g.table.errors, 4003, loc, "Empty BY, USE, or HIDE"); err != nil {
			return nil, err
		}
	} else {
		if h[next] == nil {
			return nil, g.unported("generator.construct.use-or-hide")
		}
		if h[next].Kind() == 64 {
			only = true
			next++
		}
		inDefs := false
		for next < len(h) {
			if err := g.check(depth + 1); err != nil {
				return nil, err
			}
			entry := h[next]
			if entry == nil {
				return nil, g.unported("generator.construct.use-or-hide")
			}
			if entry.Kind() == 66 && !inDefs {
				inDefs = true
				next++
				continue
			}
			if entry.Kind() == 88 {
				return nil, g.unported("generator.construct.use-or-hide")
			}
			if entry.Kind() == 55 {
				next++
				if next >= len(h) || h[next] == nil {
					return nil, g.unported("generator.construct.use-or-hide")
				}
				name, err := g.name(h[next])
				if err != nil {
					return nil, err
				}
				found, err := g.table.resolve(name, true)
				if err != nil {
					return nil, err
				}
				if found == nil && name == module.name {
					found = module
				}
				if found == nil {
					loc, err := g.diagnosticLocation(h[next])
					if err != nil {
						return nil, err
					}
					if err = addError(g.table.errors, 4005, loc, "Module `%s' used without being extended or instantiated.", nameParam(name)); err != nil {
						return nil, err
					}
				} else if _, ok := found.(*moduleNode); !ok {
					return nil, g.unported("generator.construct.use-or-hide")
				} else if inDefs {
					defs = append(defs, found)
				} else {
					facts = append(facts, found)
				}
			} else if inDefs {
				def, err := g.proofDefSelector(entry, depth+1)
				if err != nil {
					return nil, err
				}
				if def != nil {
					defs = append(defs, def)
				}
			} else if entry.Kind() == generated.N_AssumeProve {
				ap, err := g.assumeProve(entry, module, g.currentGoal, depth+1)
				if err != nil {
					return nil, err
				}
				facts = append(facts, ap)
			} else {
				if entry.Kind() == generated.N_GeneralId {
					fact, err := g.proofFactSelector(entry, depth+1)
					if err != nil {
						return nil, err
					}
					facts = append(facts, fact)
					next++
					if next < len(h) && h[next] != nil && h[next].Kind() == 88 {
						next++
					}
					continue
				}
				expr, err := g.expression(entry, depth+1)
				if err != nil {
					return nil, err
				}
				facts = append(facts, expr)
			}
			next++
			if next < len(h) && h[next] != nil && h[next].Kind() == 88 {
				next++
			}
		}
	}
	ticket := g.pool.history.event("UseOrHideNode", kind)
	sh, err := newSemanticHeader(g.pool, kind, tree)
	if err != nil {
		return nil, err
	}
	node := &useOrHideNode{semanticHeader: sh, facts: facts, defs: defs, only: only, hide: kind == 32}
	g.pool.history.attach(node, ticket)
	return node, nil
}

func (g *generator) leafProof(tree *generated.SyntaxTreeNode, module *moduleNode, depth int) (*leafProofNode, error) {
	if err := g.check(depth); err != nil {
		return nil, err
	}
	if tree == nil {
		return nil, g.unported("generator.construct.leaf-proof")
	}
	heirs := tree.Heirs()
	next := 0
	if len(heirs) == 0 || heirs[0] == nil {
		return nil, g.unported("generator.construct.leaf-proof")
	}
	if heirs[0].Kind() == 75 {
		next++
	}
	if next >= len(heirs) || heirs[next] == nil {
		return nil, g.unported("generator.construct.leaf-proof")
	}
	token := heirs[next].Kind()
	facts := make([]levelNode, 0)
	defs := make([]leafNode, 0)
	var only bool
	if token == 63 {
		g.pool.java.invalidate()
		use, err := g.useOrHide(tree, module, depth+1)
		if err != nil {
			return nil, err
		}
		facts, defs, only = use.facts, use.defs, use.only
		if len(facts)+len(defs) == 0 {
			loc, err := g.diagnosticLocation(tree)
			if err != nil {
				return nil, err
			}
			if err = addError(g.table.errors, 4003, loc, "Empty BY"); err != nil {
				return nil, err
			}
		}
	} else if token != 71 && token != 72 {
		return nil, g.unported("generator.construct.leaf-proof")
	}
	ticket := g.pool.history.event("LeafProofNode", 33)
	sh, err := newSemanticHeader(g.pool, 33, tree)
	if err != nil {
		return nil, err
	}
	node := &leafProofNode{semanticHeader: sh, omitted: token == 72, obvious: token == 71, facts: facts, defs: defs, only: only}
	g.pool.history.attach(node, ticket)
	return node, nil
}

// simpleProofSymbol accepts only the direct selector path; multi-part,
// unresolved and recovery cases remain incomplete until their modes are ported.
func (g *generator) simpleProofSymbol(tree *generated.SyntaxTreeNode) (leafNode, error) {
	if tree == nil || tree.Kind() != generated.N_GeneralId {
		return nil, g.unported("generator.construct.simple-proof-symbol")
	}
	h := tree.Heirs()
	if len(h) != 2 || h[0] == nil || len(h[0].Heirs()) != 0 || h[1] == nil || h[1].Kind() == generated.N_IdentifierTuple {
		return nil, g.unported("generator.construct.simple-proof-symbol")
	}
	n, err := g.proofSelectorName(h[1])
	if err != nil {
		return nil, err
	}
	v, err := g.table.resolve(n, false)
	if err != nil {
		return nil, err
	}
	if v == nil {
		loc, err := g.diagnosticLocation(h[1])
		if err != nil {
			return nil, err
		}
		if err = addError(g.table.errors, 4200, loc, "Unknown operator: `%s'.", nameParam(n)); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return v, nil
}

func (g *generator) simpleProofDef(tree *generated.SyntaxTreeNode) (leafNode, error) {
	v, err := g.simpleProofSymbol(tree)
	if err != nil {
		return nil, err
	}
	return g.proofDefSymbol(tree, v)
}

func (g *generator) proofDefSymbol(tree *generated.SyntaxTreeNode, v leafNode) (leafNode, error) {
	loc, err := g.diagnosticLocation(tree)
	if err != nil {
		return nil, err
	}
	switch n := v.(type) {
	case *userOpDefNode:
		if n.kind == 5 || n.kind == 6 {
			return n, nil
		}
		if n.kind == 37 {
			if _, ok := n.stepNode.(*defStepNode); ok {
				return n, nil
			}
			if err = addError(g.table.errors, 4004, loc, "DEF clause entry refers to a non-definition step."); err != nil {
				return nil, err
			}
		}
	case *thmOrAssumpDefNode:
		if !n.name.displayExact || len(n.name.text) == 0 {
			return nil, g.unported("generator.construct.proof-def-symbol")
		}
		if n.name.text[0] != '<' {
			return n, nil
		}
	}
	if err = addError(g.table.errors, 4200, loc, "DEF clause entry should describe a defined operator."); err != nil {
		return nil, err
	}
	return nil, nil
}

func (g *generator) simpleProofFact(tree *generated.SyntaxTreeNode) (levelNode, error) {
	v, err := g.simpleProofSymbol(tree)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return g.nullOAN, nil
	}
	if n, ok := v.(*userOpDefNode); ok && n.kind == 37 {
		return g.nonFactProofAlias(tree, true)
	}
	if nodeArity(v) != 0 {
		if nodeArity(v) < 0 {
			return nil, g.unported("generator.construct.simple-proof-fact")
		}
		return g.badArity(v.header().name, nodeArity(v), tree)
	}
	if n, ok := v.(*userOpDefNode); ok && n.kind == 6 {
		return n, nil
	}
	ticket := g.pool.history.event("OpApplNode", 9)
	app, err := newOpAppl(g.pool, v, make([]exprOrOpArg, 0), tree)
	if app != nil {
		g.pool.history.attach(app, ticket)
	}
	if err != nil {
		return nil, err
	}
	if n, ok := v.(*userOpDefNode); ok && n.kind == 5 {
		app.subExpressionOf = n
	}
	return app, nil
}

// proof generates theorem and non-theorem steps in source order.
func (g *generator) proof(tree *generated.SyntaxTreeNode, module *moduleNode, depth int) (_ levelNode, err error) {
	defer func() {
		if err != nil {
			g.pool.history.invalidate("proof traversal incomplete")
		}
	}()
	if err = g.check(depth); err != nil {
		return nil, err
	}
	if tree == nil || module == nil {
		return nil, g.unported("generator.construct.proof")
	}
	if tree.Kind() == generated.N_TerminalProof {
		return g.leafProof(tree, module, depth+1)
	}
	if tree.Kind() != generated.N_Proof {
		return nil, g.unported("generator.construct.proof")
	}
	g.pool.java.invalidate()
	pfContext := newContext(g.pool)
	if err = g.table.push(pfContext); err != nil {
		return nil, err
	}
	stackDepth := len(g.table.stack) - 1
	labelDepth := len(g.labelScopes)
	defer func() {
		g.labelScopes = g.labelScopes[:labelDepth]
		for len(g.table.stack) > stackDepth {
			if popErr := g.table.pop(); err == nil && popErr != nil {
				err = popErr
			}
		}
	}()
	heirs := tree.Heirs()
	offset := 0
	if len(heirs) == 0 {
		return nil, g.unported("generator.construct.proof")
	}
	if heirs[0] != nil && heirs[0].Kind() == 75 {
		offset = 1
	}
	steps := make([]levelNode, 0, len(heirs)-offset)
	instances := make([]*instanceNode, 0)
	var prevInfix bool
	var prevRHS exprNode
	var sufficesContexts int
	for _, stepTree := range heirs[offset:] {
		if err = g.check(depth + 1); err != nil {
			return nil, err
		}
		if stepTree == nil || stepTree.Kind() != generated.N_ProofStep {
			return nil, g.unported("generator.construct.proof")
		}
		p := stepTree.Heirs()
		if len(p) < 2 || len(p) > 3 || p[0] == nil || p[1] == nil {
			return nil, g.unported("generator.construct.proof")
		}
		number, e := g.proofStepName(p[0])
		if e != nil {
			return nil, e
		}
		var def *thmOrAssumpDefNode
		if number != nil {
			g.labelScopes = append(g.labelScopes, labelFrame{})
			ticket := g.pool.history.event("ThmOrAssumpDefNode", 23)
			def, e = newThmOrAssumpDef(g.pool, number, p[0])
			if def != nil {
				g.pool.history.attach(def, ticket)
			}
			if e != nil {
				return nil, e
			}
		}
		bodyTree := p[1]
		if bodyTree.Kind() == generated.N_DefStep || bodyTree.Kind() == generated.N_UseOrHide || bodyTree.Kind() == generated.N_NonLocalInstance {
			prevInfix = false
			if len(p) != 2 {
				return nil, g.unported("generator.construct.proof")
			}
			var step levelNode
			if bodyTree.Kind() == generated.N_DefStep {
				step, e = g.proofDefStep(bodyTree, number, module, &instances, depth+1)
			} else if bodyTree.Kind() == generated.N_NonLocalInstance {
				var instance *instanceNode
				e = g.unnamedInstance(bodyTree, module, false, &instance, depth+1)
				if e == nil {
					instance.stepName = number
					step = instance
				}
			} else {
				var use *useOrHideNode
				use, e = g.useOrHide(bodyTree, module, depth+1)
				if e == nil {
					use.tree, use.stepName = stepTree, number
					if len(use.facts)+len(use.defs) == 0 {
						var loc Location
						loc, e = g.diagnosticLocation(bodyTree)
						if e == nil {
							e = addError(g.table.errors, 4003, loc, "Empty USE or HIDE statement.")
						}
					}
					if e == nil {
						e = g.proofFactCheck(use)
					}
					step = use
				}
			}
			if e != nil {
				return nil, e
			}
			if number != nil {
				ticket := g.pool.history.event("OpDefNode", 37)
				header, e := newNodeHeader(g.pool, number, 37, stepTree)
				if e != nil {
					return nil, e
				}
				alias := &userOpDefNode{nodeHeader: header, arity: 0, stepNode: step, module: module, levels: newLeafLevel()}
				g.pool.history.attach(alias, ticket)
				if _, e = g.table.addSymbol(number, alias); e != nil {
					return nil, e
				}
				alias.labels = g.labelScopes[len(g.labelScopes)-1].labels
				g.labelScopes = g.labelScopes[:len(g.labelScopes)-1]
			}
			steps = append(steps, step)
			continue
		}
		bodyParts := bodyTree.Heirs()
		var body levelNode
		suffices := false
		var ap *assumeProveNode
		var apContext *context
		var pickContext *context
		if bodyTree.Kind() != generated.N_AssertStep {
			prevInfix = false
		}
		switch bodyTree.Kind() {
		case generated.N_AssertStep:
			index := 0
			if len(bodyParts) > 0 && bodyParts[0] != nil && bodyParts[0].Kind() == 84 {
				index++
				suffices = true
			}
			if index >= len(bodyParts) || bodyParts[index] == nil {
				return nil, g.unported("generator.construct.proof")
			}
			if bodyParts[index].Kind() == generated.N_AssumeProve {
				apContext = newContext(g.pool)
				if e = g.table.push(apContext); e != nil {
					return nil, e
				}
				g.currentGoal = def
				ap, e = g.assumeProve(bodyParts[index], module, def, depth+1)
				g.currentGoal = nil
				if e != nil {
					return nil, e
				}
				if e = g.table.pop(); e != nil {
					return nil, e
				}
				ap.suffices = suffices
				body = ap
				prevInfix = false
				break
			}
			exprTree := bodyParts[index]
			var expr exprNode
			if !suffices && prevInfix && isProofAtLHS(exprTree) {
				expr, e = g.proofAtExpression(exprTree, prevRHS, depth+1)
			} else {
				expr, e = g.expression(exprTree, depth+1)
			}
			if e != nil {
				return nil, e
			}
			body = expr
			if suffices {
				body, e = g.builtinApplication("$Suffices", []exprNode{expr}, bodyTree)
				if e != nil {
					return nil, e
				}
			} else {
				prevInfix = false
				if exprTree.Kind() == generated.N_InfixExpr {
					if app, ok := expr.(*opApplNode); ok && app != nil && len(app.operands) > 1 {
						// Generator.java:6181-6191 casts getArgs()[1] to
						// ExprNode; a null operand (an operand whose
						// generation failed after an error) is kept as a null
						// prevRHS, which a later @ step stores in its $Nop.
						if rhs := app.operands[1]; rhs == nil {
							prevRHS = nil
						} else if e, ok := rhs.(exprNode); ok {
							prevRHS = e
						} else {
							return nil, g.unported("generator.construct.proof")
						}
						prevInfix = true
					}
				}
			}
		case generated.N_HaveStep, generated.N_CaseStep:
			if len(bodyParts) < 2 || bodyParts[1] == nil {
				return nil, g.unported("generator.construct.proof")
			}
			expr, e := g.expression(bodyParts[1], depth+1)
			if e != nil {
				return nil, e
			}
			op := "$Have"
			if bodyTree.Kind() == generated.N_CaseStep {
				op = "$Pfcase"
			}
			body, e = g.builtinApplication(op, []exprNode{expr}, bodyTree)
			if e != nil {
				return nil, e
			}
		case generated.N_TakeStep, generated.N_PickStep:
			if len(bodyParts) < 2 || bodyParts[1] == nil {
				return nil, g.unported("generator.construct.proof")
			}
			isPick := bodyTree.Kind() == generated.N_PickStep
			if isPick {
				pickContext = newContext(g.pool)
				if e = g.table.push(pickContext); e != nil {
					return nil, e
				}
			}
			var params []*formalParamNode
			var bounded [][]*formalParamNode
			var tuples []bool
			var ranges []exprNode
			var predicateIndex int
			if bodyParts[1].Kind() == generated.N_QuantBound {
				count, next := 1, 2
				for next < len(bodyParts) && bodyParts[next] != nil && bodyParts[next].Kind() == 88 {
					if next+1 >= len(bodyParts) || bodyParts[next+1] == nil || bodyParts[next+1].Kind() != generated.N_QuantBound {
						return nil, g.unported("generator.construct.proof")
					}
					count++
					next += 2
				}
				bounded, tuples, ranges, e = g.boundArguments(bodyParts, depth+1, 1, count)
				if e != nil {
					return nil, e
				}
				params = flattenFormal(bounded)
				predicateIndex = next + 1
			} else {
				count := 1
				for 2*count < len(bodyParts) && bodyParts[2*count] != nil && bodyParts[2*count].Kind() == 88 {
					count++
				}
				params = make([]*formalParamNode, count)
				for i := range params {
					index := 2*i + 1
					if index >= len(bodyParts) || bodyParts[index] == nil {
						return nil, g.unported("generator.construct.proof")
					}
					name, e := g.name(bodyParts[index])
					if e != nil {
						return nil, e
					}
					ticket := g.pool.history.event("FormalParamNode", 11)
					params[i], e = newFormalParam(g.pool, name, 0, bodyParts[index], g.table, module)
					if params[i] != nil {
						g.pool.history.attach(params[i], ticket)
					}
					if e != nil {
						return nil, e
					}
				}
				predicateIndex = 2*count + 1
			}
			var args []exprNode
			op := "$Take"
			if isPick {
				op = "$Pick"
				if predicateIndex >= len(bodyParts) || bodyParts[predicateIndex] == nil || predicateIndex == 0 || bodyParts[predicateIndex-1] == nil || bodyParts[predicateIndex-1].Kind() != 89 {
					return nil, g.unported("generator.construct.proof")
				}
				expr, e := g.expressionWithBindings(bodyParts[predicateIndex], params, depth+1)
				if e != nil {
					return nil, e
				}
				args = []exprNode{expr}
				if e = g.table.pop(); e != nil {
					return nil, e
				}
			}
			app, e := g.builtinApplication(op, args, bodyTree)
			if e != nil {
				return nil, e
			}
			if bounded != nil {
				app.bounded, app.tupleBounds, app.ranges = bounded, tuples, ranges
			} else {
				app.unbounded = params
			}
			body = app
		case generated.N_WitnessStep:
			if len(bodyParts) < 2 || len(bodyParts)%2 != 0 {
				return nil, g.unported("generator.construct.proof")
			}
			args := make([]exprNode, 0, len(bodyParts)/2)
			for i := 1; i < len(bodyParts); i += 2 {
				if bodyParts[i] == nil || (i > 1 && (bodyParts[i-1] == nil || bodyParts[i-1].Kind() != 88)) {
					return nil, g.unported("generator.construct.proof")
				}
				expr, e := g.expression(bodyParts[i], depth+1)
				if e != nil {
					return nil, e
				}
				args = append(args, expr)
			}
			body, e = g.builtinApplication("$Witness", args, bodyTree)
			if e != nil {
				return nil, e
			}
		case generated.N_QEDStep:
			body, e = g.builtinApplication("$Qed", nil, bodyTree)
			if e != nil {
				return nil, e
			}
		default:
			loc, e := g.diagnosticLocation(tree)
			if e != nil {
				return nil, e
			}
			code, ok := CodeFromValue(4003)
			if !ok {
				return nil, g.unported("generator.construct.proof")
			}
			abort, e := g.table.errors.Add(code, &loc, "Internal error: Unexpected SyntaxTreeNode kind: %s", Parameter{Kind: IntParameter, Integer: int64(stepTree.Kind())})
			if e != nil {
				return nil, e
			}
			return nil, abort
		}
		if def != nil {
			def.theorem, def.body, def.module = true, body, module
			if _, e = g.table.addSymbol(def.name, def); e != nil {
				return nil, e
			}
			def.labels = g.labelScopes[len(g.labelScopes)-1].labels
			g.labelScopes = g.labelScopes[:len(g.labelScopes)-1]
			def.suffices = suffices
		}
		var subproof levelNode
		if len(p) == 3 {
			if p[2] == nil {
				return nil, g.unported("generator.construct.proof")
			}
			if ap != nil && !suffices {
				if e = g.table.push(apContext); e != nil {
					return nil, e
				}
			}
			subproof, e = g.proof(p[2], module, depth+1)
			if _, aborted := e.(*proofAbortError); aborted {
				// The enclosing proof continues after the nested
				// generateProof returns null, with that proof's contexts
				// and label frame still pushed (Generator.java:6153-6159,
				// 6399): not modeled.
				return nil, g.unported("generator.construct.proof-at-expression")
			}
			if e != nil {
				return nil, e
			}
			if ap != nil && !suffices {
				if e = g.table.pop(); e != nil {
					return nil, e
				}
			}
		}
		if ap != nil {
			if suffices {
				if e = g.table.push(apContext); e != nil {
					return nil, e
				}
				sufficesContexts++
			}
			ap.inProof = false
		}
		ticket := g.pool.history.event("TheoremNode", 12)
		header, e := newSemanticHeader(g.pool, 12, stepTree)
		if e != nil {
			return nil, e
		}
		theorem := &theoremNode{semanticHeader: header, body: body, proof: subproof, definition: def, module: module, suffices: suffices}
		g.pool.history.attach(theorem, ticket)
		if def != nil {
			def.statement = theorem
		}
		steps = append(steps, theorem)
		if pickContext != nil {
			for _, symbol := range pickContext.orderedValues() {
				if symbol == nil || symbol.header().name == nil {
					return nil, g.unported("generator.construct.proof")
				}
				if _, e = g.table.addSymbol(symbol.header().name, symbol); e != nil {
					return nil, e
				}
			}
		}
	}
	for i := 0; i < sufficesContexts; i++ {
		c := g.table.top
		if c == nil {
			return nil, g.unported("generator.construct.proof")
		}
		for _, symbol := range c.orderedValues() {
			if symbol == nil || symbol.header().name == nil {
				return nil, g.unported("generator.construct.proof")
			}
			pfContext.add(contextKey{name: symbol.header().name}, symbol)
		}
		if err = g.table.pop(); err != nil {
			return nil, err
		}
	}
	ticket := g.pool.history.event("NonLeafProofNode", 34)
	header, err := newSemanticHeader(g.pool, 34, tree)
	if err != nil {
		return nil, err
	}
	node := &nonLeafProofNode{semanticHeader: header, steps: steps, insts: instances, context: pfContext}
	g.pool.history.attach(node, ticket)
	return node, nil
}

func (g *generator) proofStepName(tree *generated.SyntaxTreeNode) (*name, error) {
	if tree == nil || tree.Image() == nil {
		return nil, g.unported("generator.construct.proof-step-name")
	}
	if original := tree.OriginalImage(); original != nil && !sameProofImage(original.Units(), tree.Image().Units()) {
		units := original.Units()
		if len(units) > 1 && (units[1] == '*' || units[1] == '+') {
			loc, err := g.diagnosticLocation(tree)
			if err != nil {
				return nil, err
			}
			if err = addError(g.table.errors, 4350, loc, "<*> and <+> cannot be used for a named step."); err != nil {
				return nil, err
			}
		}
	}
	switch tree.Kind() {
	case 290, 291:
		return g.name(tree)
	case 292:
		units := tree.Image().Units()
		for i, c := range units {
			if c == '.' {
				return g.pool.internUnits(units[:i]), nil
			}
		}
		return nil, g.unported("generator.construct.proof-step-name")
	default:
		return nil, nil
	}
}

// proofAbortError is generateProof's early null return after an @ step
// whose infix operator does not resolve (Generator.java:6151-6159). It
// never leaves the generator: the nested-proof caller turns it into an
// incomplete stop, and statement accepts it only where nothing generated
// afterwards can observe the contexts Java leaves pushed.
type proofAbortError struct{}

func (*proofAbortError) Error() string { return "proof generation returned null" }

func isProofAtLHS(tree *generated.SyntaxTreeNode) bool {
	if tree == nil || tree.Kind() != generated.N_InfixExpr {
		return false
	}
	h := tree.Heirs()
	if len(h) == 0 || h[0] == nil {
		return false
	}
	lhs := h[0].Heirs()
	return len(lhs) > 1 && lhs[0] != nil && len(lhs[0].Heirs()) == 0 && lhs[1] != nil && lhs[1].Kind() == 289 && lhs[1].Image() != nil && lhs[1].Image().String() == "@"
}

func (g *generator) proofAtExpression(tree *generated.SyntaxTreeNode, previous exprNode, depth int) (exprNode, error) {
	if err := g.check(depth); err != nil {
		return nil, err
	}
	parts := tree.Heirs()
	if len(parts) != 3 || parts[0] == nil || parts[1] == nil || parts[2] == nil {
		return nil, g.unported("generator.construct.proof-at-expression")
	}
	id := parts[1].Heirs()
	if len(id) != 2 || id[0] == nil || len(id[0].Heirs()) != 0 || id[1] == nil || id[1].Image() == nil {
		return nil, g.unported("generator.construct.proof-at-expression")
	}
	raw := id[1].Image().Units()
	canonical, err := g.pool.syntax.ResolveSynonym(g.pool.syntax.UniqueStringOfUnits(raw))
	if err != nil {
		return nil, g.unported("generator.precondition.synonym-lookup")
	}
	name := g.pool.internUnits(canonical.Units())
	op, err := g.table.resolve(name, false)
	if err != nil {
		return nil, err
	}
	if op == nil {
		rawName := g.pool.internUnits(raw)
		if !rawName.displayExact {
			return nil, g.unported("generator.mapping.inexact-display-name")
		}
		// generateGenID's finalAppend reports the unresolved operator at
		// the GenID node (Generator.java:6150, 276-284) before the step
		// reports it at the expression (6151-6158).
		loc, err := g.diagnosticLocation(parts[1])
		if err != nil {
			return nil, err
		}
		if err = addError(g.table.errors, 4004, loc, "Could not find declaration or definition of symbol '%s'.", nameParam(rawName)); err != nil {
			return nil, err
		}
		loc, err = g.diagnosticLocation(tree)
		if err != nil {
			return nil, err
		}
		if err = addError(g.table.errors, 4004, loc, "Couldn't resolve infix operator symbol `%s'.", nameParam(rawName)); err != nil {
			return nil, err
		}
		// generateProof then returns null at once: the remaining steps are
		// not generated and the proof's contexts are not popped
		// (Generator.java:6159). The caller decides whether that is
		// observable.
		return nil, &proofAbortError{}
	}
	if nodeArity(op) != 2 {
		return nil, g.unported("generator.construct.proof-at-expression")
	}
	rhs, err := g.expression(parts[2], depth+1)
	if err != nil {
		return nil, err
	}
	nop, err := g.builtinApplication("$Nop", []exprNode{previous}, parts[0])
	if err != nil {
		return nil, err
	}
	ticket := g.pool.history.event("OpApplNode", 9)
	app, err := newOpAppl(g.pool, op, []exprOrOpArg{nop, rhs}, tree)
	if app != nil {
		g.pool.history.attach(app, ticket)
	}
	return app, err
}

func sameProofImage(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (g *generator) proofDefStep(tree *generated.SyntaxTreeNode, number *name, module *moduleNode, instances *[]*instanceNode, depth int) (*defStepNode, error) {
	if err := g.check(depth); err != nil {
		return nil, err
	}
	parts := tree.Heirs()
	offset := 0
	if len(parts) > 0 && parts[0] != nil && parts[0].Kind() == 65 {
		offset = 1
	}
	if offset == len(parts) {
		return nil, g.unported("generator.construct.proof-def-step")
	}
	defs := make([]definitionNode, 0, len(parts)-offset)
	for _, syntax := range parts[offset:] {
		if syntax == nil {
			return nil, g.unported("generator.construct.proof-def-step")
		}
		var definition definitionNode
		var err error
		switch syntax.Kind() {
		case generated.N_OperatorDefinition:
			definition, err = g.operator(syntax, module, depth+1)
			if err == nil {
				module.definitions = append(module.definitions, definition)
				module.bodyDefinitions = append(module.bodyDefinitions, definition)
			}
		case generated.N_FunctionDefinition:
			definition, err = g.functionDefinition(syntax, module, depth+1)
		case generated.N_ModuleDefinition:
			var capture moduleDefinitionOutput
			err = g.moduleDefinition(syntax, module, &capture, depth+1)
			if err == nil {
				if capture.marker == nil || capture.instance == nil || instances == nil {
					return nil, g.unported("generator.construct.proof-def-step")
				}
				definition = capture.marker
				*instances = append(*instances, capture.instance)
			}
		default:
			return nil, g.unported("generator.construct.proof-def-step")
		}
		if err != nil {
			return nil, err
		}
		if definition == nil {
			return nil, g.unported("generator.construct.proof-def-step")
		}
		defs = append(defs, definition)
	}
	ticket := g.pool.history.event("DefStepNode", 36)
	header, err := newSemanticHeader(g.pool, 36, tree)
	if err != nil {
		return nil, err
	}
	node := &defStepNode{semanticHeader: header, stepNumber: number, defs: defs}
	g.pool.history.attach(node, ticket)
	return node, nil
}

func (g *generator) proofFactCheck(node *useOrHideNode) error {
	if !node.hide {
		return nil
	}
	for _, fact := range node.facts {
		if fact == nil {
			return g.unported("generator.construct.proof-fact-check")
		}
		app, ok := fact.(*opApplNode)
		if !ok || app == nil {
			continue
		}
		if _, ok := app.operator.(*thmOrAssumpDefNode); ok {
			continue
		}
		loc, err := g.diagnosticLocation(app.tree)
		if err != nil {
			return err
		}
		if err = addError(g.table.errors, 4357, loc, "The only expression allowed as a fact in a HIDE is \nthe name of a theorem, assumption, or step."); err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) nonFactProofAlias(tree *generated.SyntaxTreeNode, fact bool) (exprNode, error) {
	loc, err := g.diagnosticLocation(tree)
	if err != nil {
		return nil, err
	}
	message := "Step number of non-expression step used as an expression."
	if fact {
		message = "Step number of non-fact used as a fact"
	}
	if err = addError(g.table.errors, 4004, loc, "%s", exactStringParam(message)); err != nil {
		return nil, err
	}
	return g.nullOAN, nil
}

// OpDefNode.getChildren exposes only the body; walkGraph additionally visits
// parameters and the numbered step backlink in source order.
func (n *userOpDefNode) children() []levelNode { return []levelNode{n.body} }
func (n *userOpDefNode) walkChildren() []levelNode {
	out := make([]levelNode, 0, len(n.params)+2)
	for _, p := range n.params {
		if p != nil {
			out = append(out, p)
		}
	}
	if n.body != nil {
		out = append(out, n.body)
	}
	if n.stepNode != nil {
		out = append(out, n.stepNode)
	}
	return out
}

// validateProofSelector mirrors Selector.finish, which classifies every
// selector before resolving its first name. Unknown structural tokens abort.
func (g *generator) validateProofSelector(id []*generated.SyntaxTreeNode) error {
	nodes := make([]*generated.SyntaxTreeNode, 0, len(id[0].Heirs())+1)
	for _, part := range id[0].Heirs() {
		if part == nil || len(part.Heirs()) < 1 {
			return g.unported("generator.construct.validate-proof-selector")
		}
		nodes = append(nodes, part.Heirs()[0])
	}
	nodes = append(nodes, id[1])
	for _, node := range nodes {
		if node == nil {
			return g.unported("generator.construct.validate-proof-selector")
		}
		if node.Kind() != generated.N_StructOp || len(node.Heirs()) != 0 {
			continue
		}
		if node.Image() == nil {
			return g.unported("generator.construct.validate-proof-selector")
		}
		image := node.Image().String()
		if image == ">>" || image == "<<" || image == ":" || image == "@" {
			continue
		}
		loc, err := g.diagnosticLocation(node)
		if err != nil {
			return err
		}
		code, ok := CodeFromValue(4003)
		if !ok {
			return g.unported("generator.construct.validate-proof-selector")
		}
		// Generator.java:443 passes the String stn.getImage(). Its source units
		// are the image's; claim them only when the display text is lossless,
		// so the existing rendering is unchanged.
		param := Parameter{Kind: StringParameter, Text: image}
		if slices.Equal(utf16.Encode([]rune(image)), node.Image().Units()) {
			param = exactStringParam(image)
		}
		abort, err := g.table.errors.Add(code, &loc, "Internal error: Unexpected selector `%s'.", param)
		if err != nil {
			return err
		}
		return abort
	}
	return nil
}

func (g *generator) proofFactSelector(tree *generated.SyntaxTreeNode, depth int) (levelNode, error) {
	if err := g.check(depth); err != nil {
		return nil, err
	}
	if tree == nil || tree.Kind() != generated.N_GeneralId {
		return nil, g.unported("generator.construct.proof-fact-selector")
	}
	id := tree.Heirs()
	if len(id) != 2 || id[0] == nil || id[1] == nil {
		return nil, g.unported("generator.construct.proof-fact-selector")
	}
	if err := g.validateProofSelector(id); err != nil {
		return nil, err
	}
	prefix := id[0].Heirs()
	if len(prefix) == 0 {
		return g.simpleProofFact(tree)
	}
	first := prefix[0].Heirs()
	if len(first) < 2 || first[0] == nil {
		return nil, g.unported("generator.construct.proof-fact-selector")
	}
	name, err := g.name(first[0])
	if err != nil {
		return nil, err
	}
	root, err := g.table.resolve(name, false)
	if err != nil {
		return nil, err
	}
	if root == nil {
		return g.unresolvedSelector(prefix, id[1], depth)
	}
	if alias, ok := root.(*userOpDefNode); ok && alias.kind == 37 {
		return g.rejectProofAliasSubexpression(first[0], alias)
	}
	var carried []exprOrOpArg
	if marker, ok := root.(*userOpDefNode); ok && marker.kind == 6 {
		// Discover the first non-instance segment without evaluating arguments.
		var units []uint16
		allMarkers := true
		theoremTarget := false
		var selectedRoot *userOpDefNode
		for _, segment := range prefix {
			parts := segment.Heirs()
			if len(parts) < 2 || parts[0] == nil || parts[0].Kind() != 289 {
				allMarkers = false
				break
			}
			n, e := g.name(parts[0])
			if e != nil {
				return nil, e
			}
			if len(units) > 0 {
				units = append(units, '!')
			}
			units = append(units, n.units...)
			v, e := g.table.resolve(g.pool.internUnits(units), false)
			if e != nil {
				return nil, e
			}
			m, ok := v.(*userOpDefNode)
			if !ok || m.kind != 6 {
				_, theoremTarget = v.(*thmOrAssumpDefNode)
				selectedRoot, _ = v.(*userOpDefNode)
				allMarkers = false
				break
			}
		}
		if !allMarkers && !theoremTarget {
			if !g.simpleLabelSelectorHistory(id, nil) && !g.simpleNumericSelectorHistory(id) && !g.simpleQualifiedLabelFactHistory(id) {
				g.pool.history.invalidate("unproved compound fact selector history")
			}
			return g.expressionFactSelector(tree, id, selectedRoot, depth)
		}
		if allMarkers && !g.simpleQualifiedFactHistory(id) {
			g.pool.history.invalidate("unproved qualified fact history")
		}
		outer := g.selectorSubsts
		g.selectorSubsts = nil
		defer func() { g.selectorSubsts = outer }()
		index, selected, names, args, failure, e := g.qualifiedSelectorPrefix(tree, prefix, id[1], depth, false)
		if e != nil {
			return nil, e
		}
		if failure != nil {
			return failure, nil
		}
		carried = args
		if index == len(prefix) {
			if id[1].Kind() != 289 {
				return nil, g.unported("generator.construct.proof-fact-selector")
			}
			last, e := g.proofSelectorName(id[1])
			if e != nil {
				return nil, e
			}
			var full []uint16
			for _, part := range names {
				if len(full) > 0 {
					full = append(full, '!')
				}
				full = append(full, part.units...)
			}
			full = append(full, '!')
			full = append(full, last.units...)
			target, e := g.table.resolve(g.pool.internUnits(full), false)
			if e != nil {
				return nil, e
			}
			if target == nil {
				return g.unresolvedQualifiedSelector(prefix, id[1], len(prefix), depth)
			}
			if nodeArity(target) != len(carried) {
				return g.badArity(target.header().name, nodeArity(target)-len(carried), tree)
			}
			if op, ok := target.(*userOpDefNode); ok && op.kind == 6 {
				return op, nil
			}
			if carried == nil {
				carried = make([]exprOrOpArg, 0)
			}
			if op, ok := target.(*userOpDefNode); ok {
				return g.applySelectedOperator(tree, target, op, carried)
			}
			ticket := g.pool.history.event("OpApplNode", 9)
			app, e := newOpAppl(g.pool, target, carried, tree)
			if app != nil {
				g.pool.history.attach(app, ticket)
			}
			if e != nil {
				return nil, e
			}
			app.subExpressionOf = target
			return app, nil
		}
		prefix, root = prefix[index:], selected
	}
	if theorem, ok := root.(*thmOrAssumpDefNode); ok {
		outer := g.selectorSubsts
		g.selectorSubsts = nil
		defer func() { g.selectorSubsts = outer }()
		return g.theoremLevelSelector(tree, prefix, id[1], nil, theorem, carried, depth, 0, true)
	}
	if op, ok := root.(*userOpDefNode); ok {
		if !g.simpleLabelSelectorHistory(id, nil) && !g.simpleNumericSelectorHistory(id) {
			g.pool.history.invalidate("unproved compound fact selector history")
		}
		return g.expressionFactSelector(tree, id, op, depth)
	}
	return nil, g.unported("generator.construct.proof-fact-selector")
}

// simpleQualifiedLabelFactHistory covers I!F!L for a zero-arity instance
// and definition. Existing source substitution prefixes are replayed by
// selectorBody and wrapSelectorBody before the final $Nop application.
func (g *generator) simpleQualifiedLabelFactHistory(id []*generated.SyntaxTreeNode) bool {
	if !g.pool.history.exact || len(id) != 2 || id[0] == nil || id[1] == nil || id[1].Kind() != 289 || id[1].Image() == nil {
		return false
	}
	prefix := id[0].Heirs()
	if len(prefix) != 2 {
		return false
	}
	first, second := prefix[0].Heirs(), prefix[1].Heirs()
	if len(first) != 2 || len(second) != 2 || first[0] == nil || second[0] == nil || first[0].Kind() != 289 || second[0].Kind() != 289 || first[0].Image() == nil || second[0].Image() == nil {
		return false
	}
	name := g.pool.lookupUnits(first[0].Image().Units())
	if name == nil {
		return false
	}
	value, err := g.table.resolve(name, false)
	marker, ok := value.(*userOpDefNode)
	if err != nil || !ok || marker.kind != 6 || marker.arity != 0 || !g.pool.history.reuse(marker).known {
		return false
	}
	units := append(append(append([]uint16{}, name.units...), '!'), second[0].Image().Units()...)
	full := g.pool.lookupUnits(units)
	if full == nil {
		return false
	}
	value, err = g.table.resolve(full, false)
	root, ok := value.(*userOpDefNode)
	if err != nil || !ok || root.kind != 5 || root.arity != 0 || !root.defined || !g.pool.history.reuse(root).known {
		return false
	}
	body := root.body
	for {
		w, ok := body.(*substInNode)
		if !ok {
			break
		}
		if !g.pool.history.reuse(w).known {
			return false
		}
		body = w.body
	}
	labelName := g.pool.lookupUnits(id[1].Image().Units())
	if labelName == nil {
		return false
	}
	label := root.labels.get(labelName)
	return label != nil && body == label && len(label.params) == 0 && !label.isAssumeProve && g.pool.history.reuse(label).known
}

// simpleQualifiedFactHistory follows known instance markers and a final
// definition. Marker arguments are generated once by qualifiedSelectorPrefix.
func (g *generator) simpleQualifiedFactHistory(id []*generated.SyntaxTreeNode) bool {
	if !g.pool.history.exact || len(id) != 2 || id[0] == nil || id[1] == nil || id[1].Image() == nil {
		return false
	}
	prefix := id[0].Heirs()
	if len(prefix) == 0 {
		return false
	}
	var units []uint16
	count := 0
	for _, segment := range prefix {
		parts := segment.Heirs()
		if (len(parts) != 2 && len(parts) != 3) || parts[0] == nil || parts[0].Kind() != 289 || parts[0].Image() == nil {
			return false
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
		marker, ok := value.(*userOpDefNode)
		if err != nil || !ok || marker.kind != 6 || !g.pool.history.reuse(marker).known || len(marker.params) != marker.arity {
			return false
		}
		supplied := 0
		if len(parts) == 3 {
			if parts[1] == nil || parts[1].Kind() != generated.N_OpArgs {
				return false
			}
			args := parts[1].Heirs()
			if len(args) < 3 || len(args)%2 == 0 {
				return false
			}
			supplied = (len(args) - 1) / 2
		}
		if count+supplied != marker.arity {
			return false
		}
		count = marker.arity
	}
	// Plain identifiers are already canonical. Other operator spellings remain
	// guarded here so preflight never interns a name before marker arguments.
	if id[1].Kind() != 289 {
		return false
	}
	units = append(units, '!')
	units = append(units, id[1].Image().Units()...)
	full := g.pool.lookupUnits(units)
	if full == nil {
		return true
	}
	value, err := g.table.resolve(full, false)
	if err != nil {
		return false
	}
	if value == nil {
		return true
	}
	if !g.pool.history.reuse(value).known || nodeArity(value) != count {
		return false
	}
	switch target := value.(type) {
	case *userOpDefNode:
		return target.kind == 5 && len(target.params) == count
	case *thmOrAssumpDefNode:
		return len(target.params) == count
	default:
		return false
	}
}

func (g *generator) proofDefSelector(tree *generated.SyntaxTreeNode, depth int) (leafNode, error) {
	if err := g.check(depth); err != nil {
		return nil, err
	}
	if tree == nil {
		return nil, g.unported("generator.construct.proof-def-selector")
	}
	if tree.Kind() == generated.N_OpApplication {
		return g.proofDefApplication(tree)
	}
	if tree.Kind() != generated.N_GeneralId {
		return g.malformedProofDef(tree)
	}
	id := tree.Heirs()
	if len(id) != 2 || id[0] == nil || id[1] == nil {
		return nil, g.unported("generator.construct.proof-def-selector")
	}
	if err := g.validateProofSelector(id); err != nil {
		return nil, err
	}
	prefix := id[0].Heirs()
	if len(prefix) == 0 {
		return g.simpleProofDef(tree)
	}
	first := prefix[0].Heirs()
	if len(first) < 2 || first[0] == nil {
		return nil, g.unported("generator.construct.proof-def-selector")
	}
	for _, segment := range prefix {
		parts := segment.Heirs()
		if len(parts) == 3 && parts[0] != nil {
			if _, err := g.rejectSelectorArgumentsInOperatorMode(parts[0]); err != nil {
				return nil, err
			}
			return g.rejectedProofDef(tree)
		}
	}
	firstName, err := g.proofSelectorName(first[0])
	if err != nil {
		return nil, err
	}
	root, err := g.table.resolve(firstName, false)
	if err != nil {
		return nil, err
	}
	if root == nil {
		if _, err = g.unresolvedQualifiedSelector(prefix, id[1], 0, depth); err != nil {
			return nil, err
		}
		return g.rejectedProofDef(tree)
	}
	if marker, ok := root.(*userOpDefNode); !ok || marker.kind != 6 {
		return g.proofDefSubselector(tree, prefix, id[1], root, depth)
	}
	var units []uint16
	for i, segment := range prefix {
		if err := g.check(depth + i + 1); err != nil {
			return nil, err
		}
		parts := segment.Heirs()
		if len(parts) < 2 || len(parts) > 3 || parts[0] == nil || parts[0].Kind() != 289 {
			return nil, g.unported("generator.construct.proof-def-selector")
		}
		if len(parts) == 3 {
			if _, err := g.rejectSelectorArgumentsInOperatorMode(parts[0]); err != nil {
				return nil, err
			}
			return g.rejectedProofDef(tree)
		}
		n, err := g.proofSelectorName(parts[0])
		if err != nil {
			return nil, err
		}
		if len(units) > 0 {
			units = append(units, '!')
		}
		units = append(units, n.units...)
		value, err := g.table.resolve(g.pool.internUnits(units), false)
		if err != nil {
			return nil, err
		}
		if value == nil {
			if _, err = g.unresolvedQualifiedSelector(prefix, id[1], i, depth); err != nil {
				return nil, err
			}
			return g.rejectedProofDef(tree)
		}
		marker, ok := value.(*userOpDefNode)
		if !ok || marker.kind != 6 {
			return g.proofDefSubselector(tree, prefix[i:], id[1], value, depth+i)
		}
	}
	last, err := g.proofSelectorName(id[1])
	if err != nil {
		return nil, err
	}
	units = append(units, '!')
	units = append(units, last.units...)
	value, err := g.table.resolve(g.pool.internUnits(units), false)
	if err != nil {
		return nil, err
	}
	if value == nil {
		if _, err = g.unresolvedQualifiedSelector(prefix, id[1], len(prefix), depth); err != nil {
			return nil, err
		}
		return g.rejectedProofDef(tree)
	}
	return g.proofDefSymbol(tree, value)
}

// proofDefSubselector continues after resolving a module-instance prefix.
func (g *generator) proofDefSubselector(tree *generated.SyntaxTreeNode, prefix []*generated.SyntaxTreeNode, last *generated.SyntaxTreeNode, root leafNode, depth int) (leafNode, error) {
	if len(prefix) == 0 {
		return nil, g.unported("generator.construct.proof-def-subselector")
	}
	first := prefix[0].Heirs()
	if len(first) < 2 || first[0] == nil {
		return nil, g.unported("generator.construct.proof-def-subselector")
	}
	var err error
	if len(first) == 3 {
		if _, err = g.rejectSelectorArgumentsInOperatorMode(first[0]); err != nil {
			return nil, err
		}
		return g.rejectedProofDef(tree)
	}
	if _, handled, e := g.rejectDeclaredSubexpression(root, first[0]); handled {
		if e != nil {
			return nil, e
		}
		return g.rejectedProofDef(tree)
	}
	if theorem, ok := root.(*thmOrAssumpDefNode); ok {
		outer := g.selectorSubsts
		g.selectorSubsts = nil
		defer func() { g.selectorSubsts = outer }()
		selected, err := g.theoremLevelSelector(tree, prefix, last, nil, theorem, nil, depth, -1, false)
		if err != nil {
			return nil, err
		}
		if symbol, ok := selected.(leafNode); ok {
			return g.proofDefSymbol(tree, symbol)
		}
		return g.rejectedProofDef(tree)
	}
	if alias, ok := root.(*userOpDefNode); ok && alias.kind == 37 {
		if _, err = g.rejectProofAliasSubexpression(first[0], alias); err != nil {
			return nil, err
		}
		return g.rejectedProofDef(tree)
	}
	if op, ok := root.(*userOpDefNode); ok && op.kind == 5 && len(prefix) > 1 && len(first) == 2 {
		if !op.defined {
			if _, err = g.rejectUndefinedSubexpression(op, first[0]); err != nil {
				return nil, err
			}
			return g.rejectedProofDef(tree)
		}
		outer := g.selectorSubsts
		g.selectorSubsts = nil
		defer func() { g.selectorSubsts = outer }()
		body, err := g.selectorBody(op.body, depth+1)
		if err != nil {
			return nil, err
		}
		if len(prefix) == 2 && last.Kind() == 289 {
			part := prefix[1].Heirs()
			if len(part) == 2 && part[0] != nil && part[0].Kind() == generated.N_StructOp && len(part[0].Heirs()) == 0 && part[0].Image() != nil && part[0].Image().String() == ":" {
				for d := depth + 1; ; d++ {
					if e := g.check(d); e != nil {
						return nil, e
					}
					label, ok := body.(*labelNode)
					if !ok {
						break
					}
					body = label.expressionBody()
				}
				localBody, ok := body.(*letInNode)
				if !ok {
					loc, e := g.diagnosticLocation(part[0])
					if e != nil {
						return nil, e
					}
					if e = addError(g.table.errors, 4005, loc, "A name selector here must be from a LET clause."); e != nil {
						return nil, e
					}
					return g.rejectedProofDef(tree)
				}
				name, e := g.name(last)
				if e != nil {
					return nil, e
				}
				if local := localBody.context.lookup(contextKey{name: name}); local != nil {
					return g.proofDefSymbol(tree, local)
				}
				loc, e := g.diagnosticLocation(last)
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4200, loc, "Unknown operator: `%s'.", nameParam(name)); e != nil {
					return nil, e
				}
				return g.rejectedProofDef(tree)
			}
		}
		for _, segment := range prefix[1:] {
			if segment == nil {
				return nil, g.unported("generator.construct.proof-def-subselector")
			}
			parts := segment.Heirs()
			if len(parts) < 2 || parts[0] == nil {
				return nil, g.unported("generator.construct.proof-def-subselector")
			}
		}
		var bound []*formalParamNode
		var boundArgs []*generated.SyntaxTreeNode
		local, err := g.structuralLetTarget(body, prefix, last, &bound, &boundArgs, depth, true)
		if err == nil {
			return g.proofDefSymbol(tree, local)
		}
		if failure, ok := err.(*selectorTraversalError); ok {
			if _, err = g.rejectSelectorTraversal(failure); err != nil {
				return nil, err
			}
			return g.rejectedProofDef(tree)
		}
		if supplied, ok := err.(*selectorSuppliedArgumentError); ok {
			if _, err = g.rejectSelectorArgumentsInOperatorMode(supplied.tree); err != nil {
				return nil, err
			}
			return g.rejectedProofDef(tree)
		}
		if missing, ok := err.(*selectorNullError); ok {
			if _, err = g.nullSelected(missing.tree); err != nil {
				return nil, err
			}
			return g.rejectedProofDef(tree)
		}
		if err != ErrPortIncomplete {
			return nil, err
		}
		g.incomplete.forget() // the alternative below consumes this origin
		var structural []*generated.SyntaxTreeNode
		for _, segment := range prefix[1:] {
			parts := segment.Heirs()
			if len(parts) == 3 {
				if _, err = g.rejectSelectorArgumentsInOperatorMode(parts[0]); err != nil {
					return nil, err
				}
				return g.rejectedProofDef(tree)
			}
			if len(parts) != 2 {
				return nil, g.unported("generator.construct.proof-def-subselector")
			}
			if parts[0].Kind() != generated.N_StructOp && parts[0].Kind() != generated.N_OpArgs {
				selected, e := g.definitionLevelSelector(tree, prefix, last, nil, op, op.params, body, op.labels, nil, depth, -1, false)
				if e != nil {
					return nil, e
				}
				if symbol, ok := selected.(leafNode); ok {
					return g.proofDefSymbol(tree, symbol)
				}
				return g.rejectedProofDef(tree)
			}
			structural = append(structural, parts[0])
		}
		if last.Kind() != generated.N_StructOp && last.Kind() != generated.N_OpArgs {
			return nil, g.unported("generator.construct.proof-def-subselector")
		}
		structural = append(structural, last)
		if _, err = g.numericSubexpression(tree, structural, op, body, op.params, nil, depth+1, -1, true); err != nil {
			return nil, err
		}
		return g.rejectedProofDef(tree)
	}
	if op, ok := root.(*userOpDefNode); ok && op.kind == 5 && len(prefix) == 1 && len(first) == 2 {
		if last.Kind() == 289 {
			return g.proofDefLabel(tree, first[0], last, op)
		}
		if last.Kind() == generated.N_StructOp || last.Kind() == generated.N_OpArgs {
			if !op.defined {
				if _, err = g.rejectUndefinedSubexpression(op, first[0]); err != nil {
					return nil, err
				}
				return g.rejectedProofDef(tree)
			}
			if last.Kind() == generated.N_StructOp && len(last.Heirs()) == 0 && last.Image() != nil && last.Image().String() == ":" {
				selected, e := g.definitionLevelSelector(tree, prefix, last, nil, op, op.params, op.body, op.labels, nil, depth, -1, false)
				if e != nil {
					return nil, e
				}
				if symbol, ok := selected.(leafNode); ok {
					return g.proofDefSymbol(tree, symbol)
				}
				return g.rejectedProofDef(tree)
			}
			if _, err = g.numericSubexpression(tree, []*generated.SyntaxTreeNode{last}, op, op.body, op.params, nil, depth+1, -1, true); err != nil {
				return nil, err
			}
			return g.rejectedProofDef(tree)
		}
	}

	return nil, g.unported("generator.construct.proof-def-subselector")
}

func (g *generator) rejectProofAliasSubexpression(tree *generated.SyntaxTreeNode, alias *userOpDefNode) (exprNode, error) {
	if alias == nil || alias.name == nil || !alias.name.displayExact {
		return nil, g.unported("generator.construct.reject-proof-alias-subexpression")
	}
	loc, err := g.diagnosticLocation(tree)
	if err != nil {
		return nil, err
	}
	if err = addError(g.table.errors, 4005, loc, "Cannot take subexpression of `%s'.", nameStringParam(alias.name)); err != nil {
		return nil, err
	}
	return g.nullOAN, nil
}

// Ordinary labels share expression selector traversal. An AP label can also
// select its clauses when consumed as a fact.
func (g *generator) expressionFactSelector(tree *generated.SyntaxTreeNode, id []*generated.SyntaxTreeNode, root *userOpDefNode, depth int) (levelNode, error) {
	if root == nil || root.kind != 5 {
		return nil, g.unported("generator.construct.expression-fact-selector")
	}
	type frame struct {
		labels *labelSet
		depth  int
	}
	stack := []frame{{root.labels, depth + 1}}
	seen := map[*labelSet]bool{}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if err := g.check(f.depth); err != nil {
			return nil, err
		}
		if f.labels == nil || seen[f.labels] {
			continue
		}
		seen[f.labels] = true
		for _, label := range f.labels.ordered() {
			if label == nil {
				return nil, g.unported("generator.construct.expression-fact-selector")
			}
			if label.isAssumeProve && (len(id[0].Heirs()) != 1 || id[1].Kind() != 289) {
				prefix := id[0].Heirs()
				if len(prefix) == 0 {
					return nil, g.unported("generator.construct.expression-fact-selector")
				}
				first := prefix[0].Heirs()
				if len(first) < 2 || first[0] == nil {
					return nil, g.unported("generator.construct.expression-fact-selector")
				}
				name, e := g.proofSelectorName(first[0])
				if e != nil {
					return nil, e
				}
				value, e := g.table.resolve(name, false)
				if e != nil {
					return nil, e
				}
				outer := g.selectorSubsts
				g.selectorSubsts = nil
				defer func() { g.selectorSubsts = outer }()
				var carried []exprOrOpArg
				if value != root {
					marker, ok := value.(*userOpDefNode)
					if !ok || marker.kind != 6 {
						return nil, g.unported("generator.construct.expression-fact-selector")
					}
					index, selected, _, args, failure, e := g.qualifiedSelectorPrefix(tree, prefix, id[1], depth, false)
					if e != nil {
						return nil, e
					}
					if failure != nil {
						return failure, nil
					}
					if selected != root || index >= len(prefix) {
						return nil, g.unported("generator.construct.expression-fact-selector")
					}
					prefix, carried = prefix[index:], args
				}
				return g.definitionLevelSelector(tree, prefix, id[1], nil, root, root.params, root.body, root.labels, carried, depth, 0, true)
			}
			stack = append(stack, frame{label.labels, f.depth + 1})
		}
	}
	return g.simpleLabelSelectorMode(tree, id, nil, depth, true)
}

func (g *generator) proofSelectorName(tree *generated.SyntaxTreeNode) (*name, error) {
	raw, err := g.name(tree)
	if err != nil {
		return nil, err
	}
	canonical, err := g.pool.syntax.ResolveSynonym(g.pool.syntax.UniqueStringOfUnits(raw.units))
	if err != nil {
		return nil, g.unported("generator.precondition.synonym-lookup")
	}
	return g.pool.internUnits(canonical.Units()), nil
}

func (g *generator) rejectedProofDef(tree *generated.SyntaxTreeNode) (leafNode, error) {
	loc, err := g.diagnosticLocation(tree)
	if err != nil {
		return nil, err
	}
	if err = addError(g.table.errors, 4200, loc, "DEF clause entry should describe a defined operator."); err != nil {
		return nil, err
	}
	return nil, nil
}

// genIdToSelector receives an OpApplication for malformed DEF entries. The
// source has distinct diagnostic paths for no prefix and the representable
// two/three-prefix leaf-name cases, and aborts for the single-prefix case.
func (g *generator) proofDefApplication(tree *generated.SyntaxTreeNode) (leafNode, error) {
	h := tree.Heirs()
	if len(h) != 2 || h[0] == nil || h[1] == nil || h[0].Kind() != generated.N_GeneralId || h[1].Kind() != generated.N_OpArgs {
		return nil, g.unported("generator.construct.proof-def-application")
	}
	id := h[0].Heirs()
	if len(id) != 2 || id[0] == nil {
		return nil, g.unported("generator.construct.proof-def-application")
	}
	count := len(id[0].Heirs())
	if count == 0 {
		loc, err := g.diagnosticLocation(tree)
		if err != nil {
			return nil, err
		}
		if err = addError(g.table.errors, 4003, loc, "Was expecting a GeneralId."); err != nil {
			return nil, err
		}
		if _, err = g.rejectSelectorArgumentsInOperatorMode(h[1]); err != nil {
			return nil, err
		}
		return g.rejectedProofDef(tree)
	}
	if count == 1 || count > 3 {
		loc, err := g.diagnosticLocation(id[0])
		if err != nil {
			return nil, err
		}
		code, ok := CodeFromValue(4003)
		if !ok {
			return nil, g.unported("generator.construct.proof-def-application")
		}
		abort, err := g.table.errors.Add(code, &loc, "Internal error: IdPrefixElement has other than 2 or 3 heirs.")
		if err != nil {
			return nil, err
		}
		return nil, abort
	}
	// The malformed OpApplication is passed directly to genIdToSelector.
	// With two or three prefix elements and a leaf final name, Java first
	// encounters that leaf's empty heirs, then classifies the first prefix
	// element as an unexpected selector. Its default selector code is zero.
	if (count == 2 || count == 3) && id[1] != nil && len(id[1].Heirs()) == 0 {
		prefix := id[0].Heirs()
		if prefix[0] == nil || prefix[0].Kind() != generated.N_IdPrefixElement || prefix[0].Image() == nil {
			return nil, g.unported("generator.construct.proof-def-application")
		}
		loc, err := g.diagnosticLocation(tree)
		if err != nil {
			return nil, err
		}
		if err = addError(g.table.errors, 4003, loc, "Was expecting a GeneralId."); err != nil {
			return nil, err
		}
		loc, err = g.diagnosticLocation(prefix[0])
		if err != nil {
			return nil, err
		}
		if err = addError(g.table.errors, 4003, loc, "Unexpected token found."); err != nil {
			return nil, err
		}
		if count == 3 {
			if _, err = g.rejectSelectorArgumentsInOperatorMode(prefix[0]); err != nil {
				return nil, err
			}
		} else {
			name, e := g.name(prefix[0])
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4005, loc, "Need name or step number here, not `%s'.", nameParam(name)); e != nil {
				return nil, e
			}
		}
		return g.rejectedProofDef(tree)
	}
	return nil, g.unported("generator.construct.proof-def-application")
}

func (g *generator) invalidSelectedProofDef(tree *generated.SyntaxTreeNode) (exprNode, error) {
	if _, err := g.rejectedProofDef(tree); err != nil {
		return nil, err
	}
	return g.nullOAN, nil
}

func (g *generator) proofDefLabel(tree, rootSyntax, labelSyntax *generated.SyntaxTreeNode, root *userOpDefNode) (leafNode, error) {
	if !root.defined {
		if _, err := g.rejectUndefinedSubexpression(root, rootSyntax); err != nil {
			return nil, err
		}
		return g.rejectedProofDef(tree)
	}
	name, err := g.name(labelSyntax)
	if err != nil {
		return nil, err
	}
	label := root.labels.get(name)
	if label == nil {
		loc, err := g.diagnosticLocation(labelSyntax)
		if err != nil {
			return nil, err
		}
		if err = addError(g.table.errors, 4004, loc, "Cannot find label `%s'.", nameStringParam(name)); err != nil {
			return nil, err
		}
		return g.rejectedProofDef(tree)
	}
	if label.goal != nil {
		illegal, err := g.illegalProofLabelRef(label, labelSyntax)
		if err != nil {
			return nil, err
		}
		if illegal {
			loc, err := g.diagnosticLocation(labelSyntax)
			if err != nil {
				return nil, err
			}
			if err = addError(g.table.errors, 4005, loc, "Accessing subexpression labeled `%s' of ASSUME/PROVE clause within the scope of a declaration\n from outside that declaration's scope.", nameStringParam(name)); err != nil {
				return nil, err
			}
		}
	}
	return g.rejectedProofDef(tree)
}

func (g *generator) illegalProofLabelRef(label *labelNode, tree *generated.SyntaxTreeNode) (bool, error) {
	if label == nil {
		return false, g.unported("generator.construct.illegal-proof-label-ref")
	}
	if label.goal == nil {
		return false, nil
	}
	ap, ok := label.goal.body.(*assumeProveNode)
	if !ok || ap == nil {
		loc, err := g.diagnosticLocation(tree)
		if err != nil {
			return false, err
		}
		code, ok := CodeFromValue(4003)
		if !ok {
			return false, g.unported("generator.construct.illegal-proof-label-ref")
		}
		abort, err := g.table.errors.Add(code, &loc, "Internal error: Expecting label to be in AssumeProveNode, but it's not.")
		if err != nil {
			return false, err
		}
		return false, abort
	}
	if label.goalClause < 0 || label.goalClause >= len(ap.declarations) {
		return false, g.unported("generator.construct.illegal-proof-label-ref")
	}
	return ap.declarations[label.goalClause] && label.goal.suffices == ap.inProof, nil
}
