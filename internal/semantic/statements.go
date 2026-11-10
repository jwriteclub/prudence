// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ semantic node classes, Copyright 2003 Compaq Corporation
// and Microsoft Corporation, under the MIT license; see LICENSE.
package semantic

import generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"

// levelNode includes statements as well as expressions and symbols. It does
// not make statements valid expressions or context entries.
type levelNode interface{ levelHeader() *semanticHeader }

func (h *semanticHeader) levelHeader() *semanticHeader { return h }

type assumeNode struct {
	semanticHeader
	expression exprNode
	definition *thmOrAssumpDefNode
	module     *moduleNode
	axiom      bool
}
type theoremNode struct {
	semanticHeader
	body       levelNode
	proof      levelNode
	definition *thmOrAssumpDefNode
	module     *moduleNode
	suffices   bool
}
type thmOrAssumpDefNode struct {
	nodeHeader
	theorem          bool
	suffices         bool
	body             levelNode
	params           []*formalParamNode
	local            bool
	module           *moduleNode
	source           *thmOrAssumpDefNode
	instantiatedFrom *moduleNode
	labels           *labelSet
	statement        levelNode
	levels           leafLevel
}

func (n *thmOrAssumpDefNode) header() *nodeHeader { return &n.nodeHeader }

type assumeProveNode struct {
	semanticHeader
	assumptions              []levelNode
	prove                    exprNode
	declarations             []bool
	goal                     *thmOrAssumpDefNode
	inProof, suffices, boxed bool
}
type newSymbNode struct {
	semanticHeader
	declaration *opDeclNode
	set         exprNode
}
type leafProofNode struct {
	semanticHeader
	omitted, obvious bool
	facts            []levelNode
	defs             []leafNode
	only             bool
}
type apSubstInNode struct {
	semanticHeader
	substs                                  []*subst
	body                                    levelNode
	instantiatingModule, instantiatedModule *moduleNode
}

func newThmOrAssumpDef(pool *namePool, name *name, tree *generated.SyntaxTreeNode) (*thmOrAssumpDefNode, error) {
	h, err := newNodeHeader(pool, name, 23, tree)
	if err != nil {
		return nil, err
	}
	return &thmOrAssumpDefNode{nodeHeader: h, levels: newLeafLevel()}, nil
}

// statement handles the expression forms; ASSUME/PROVE and nonterminal proof
// generation remain guarded until their scope and proof nodes are modeled.
func (g *generator) statement(tree *generated.SyntaxTreeNode, module *moduleNode, depth int) (err error) {
	defer func() {
		if err != nil {
			g.pool.history.invalidate("statement traversal incomplete")
		}
	}()
	if err := g.check(depth); err != nil {
		return err
	}
	parts := tree.Heirs()
	if len(parts) < 2 {
		return g.unported("generator.construct.statement")
	}
	isTheorem := tree.Kind() == generated.N_Theorem
	last := len(parts) - 1
	hasProof := isTheorem && (parts[last].Kind() == generated.N_Proof || parts[last].Kind() == generated.N_TerminalProof)
	bodyIndex := last
	if hasProof {
		bodyIndex--
	}
	if bodyIndex < 1 {
		return g.unported("generator.construct.statement")
	}
	named := bodyIndex > 1
	var def *thmOrAssumpDefNode
	if named {
		labelDepth := len(g.labelScopes)
		g.labelScopes = append(g.labelScopes, labelFrame{})
		defer func() { g.labelScopes = g.labelScopes[:labelDepth] }()
		if isTheorem {
			n, e := g.name(parts[1])
			if e != nil {
				return e
			}
			ticket := g.pool.history.event("ThmOrAssumpDefNode", 23)
			def, err = newThmOrAssumpDef(g.pool, n, tree)
			if def != nil {
				g.pool.history.attach(def, ticket)
			}
			if err != nil {
				return err
			}
		}
	}
	var body levelNode
	var ap *assumeProveNode
	if parts[bodyIndex].Kind() == generated.N_AssumeProve {
		if !isTheorem {
			return g.unported("generator.construct.statement")
		}
		if err = g.table.push(newContext(g.pool)); err != nil {
			return err
		}
		defer func() { _ = g.table.pop() }()
		g.currentGoal = def
		ap, err = g.assumeProve(parts[bodyIndex], module, def, depth+1)
		g.currentGoal = nil
		if err != nil {
			return err
		}
		body = ap
	} else if isTheorem {
		body, err = g.expression(parts[bodyIndex], depth+1)
		if err != nil {
			return err
		}
	} else {
		// processAssumption (Generator.java:5737-5761) stores the expression
		// generated at 5747 unchanged, even when it is null after an error:
		// the ThmOrAssumpDefNode constructor (ThmOrAssumpDefNode.java:
		// 140-177), addAssumption (ModuleNode.java:611-620) and the
		// AssumeNode constructor (AssumeNode.java:57-68) only assign it.
		var expr exprNode
		expr, err = g.nullableExpression(parts[bodyIndex], depth+1)
		if err != nil {
			return err
		}
		if expr != nil {
			body = expr
		}
	}
	if named {
		if def == nil {
			n, e := g.name(parts[1])
			if e != nil {
				return e
			}
			ticket := g.pool.history.event("ThmOrAssumpDefNode", 23)
			def, err = newThmOrAssumpDef(g.pool, n, tree)
			if def != nil {
				g.pool.history.attach(def, ticket)
			}
			if err != nil {
				return err
			}
		}
		def.theorem, def.body, def.module = isTheorem, body, module
		var apContext *context
		if ap != nil {
			apContext = g.table.top
			if err = g.table.pop(); err != nil {
				return err
			}
		}
		_, err = g.table.addSymbol(def.name, def)
		if err == nil {
			def.labels = g.labelScopes[len(g.labelScopes)-1].labels
			g.labelScopes = g.labelScopes[:len(g.labelScopes)-1]
		}
		if apContext != nil {
			if e := g.table.push(apContext); e != nil {
				return e
			}
		}
		if err != nil {
			return err
		}
		module.definitions = append(module.definitions, def)
	}
	if isTheorem {
		var proof levelNode
		if hasProof {
			final := g.finalOuterUnit
			g.finalOuterUnit = false
			proof, err = g.proof(parts[last], module, depth+1)
			if _, aborted := err.(*proofAbortError); aborted {
				// generateProof returned null (Generator.java:6159);
				// processTheorem stores the null proof (5718-5730). The
				// contexts Java leaves pushed are unobservable only when
				// nothing in this generator follows: the last unit of an
				// outermost module, whose external context is the base
				// context (SymbolTable.java:73).
				if !final {
					return g.unported("generator.construct.proof-at-expression")
				}
				proof, err = nil, nil
			}
			if err != nil {
				return err
			}
		}
		ticket := g.pool.history.event("TheoremNode", 12)
		h, e := newSemanticHeader(g.pool, 12, tree)
		if e != nil {
			return e
		}
		if ap != nil {
			ap.inProof = false
		}
		n := &theoremNode{semanticHeader: h, body: body, proof: proof, definition: def, module: module}
		g.pool.history.attach(n, ticket)
		if def != nil {
			def.statement = n
		}
		module.theorems = append(module.theorems, n)
		module.topLevel = append(module.topLevel, n)
	} else {
		ticket := g.pool.history.event("AssumeNode", 20)
		h, e := newSemanticHeader(g.pool, 20, tree)
		if e != nil {
			return e
		}
		var expr exprNode
		if body != nil {
			var ok bool
			if expr, ok = body.(exprNode); !ok {
				return g.unported("generator.construct.statement")
			}
		}
		n := &assumeNode{semanticHeader: h, expression: expr, definition: def, module: module, axiom: parts[0].Image() != nil && parts[0].Image().String() == "AXIOM"}
		g.pool.history.attach(n, ticket)
		if def != nil {
			def.statement = n
		}
		module.assumptions = append(module.assumptions, n)
		module.topLevel = append(module.topLevel, n)
	}
	return nil
}

func (g *generator) assumeProve(tree *generated.SyntaxTreeNode, module *moduleNode, goal *thmOrAssumpDefNode, depth int) (*assumeProveNode, error) {
	if err := g.check(depth); err != nil {
		return nil, err
	}
	if g.apDepth >= 99 {
		return nil, g.unported("generator.construct.assume-prove")
	}
	parts := tree.Heirs()
	if len(parts) < 2 || len(parts)%2 != 0 {
		return nil, g.unported("generator.construct.assume-prove")
	}
	ticket := g.pool.history.event("AssumeProveNode", 14)
	h, err := newSemanticHeader(g.pool, 14, tree)
	if err != nil {
		return nil, err
	}
	n := &assumeProveNode{semanticHeader: h, inProof: true}
	g.pool.history.attach(n, ticket)
	if g.apDepth == 0 {
		n.goal = goal
	}
	g.apDepth++
	if g.apDepth == 1 {
		g.goalClause = 0
	}
	g.apDeclScopes = append(g.apDeclScopes, false)
	defer func() { g.apDepth--; g.apDeclScopes = g.apDeclScopes[:len(g.apDeclScopes)-1] }()
	if g.apDepth > 1 {
		if err = g.table.push(newContext(g.pool)); err != nil {
			return nil, err
		}
		defer func() { _ = g.table.pop() }()
	}
	if parts[0].Image() == nil || parts[len(parts)-2].Image() == nil {
		return nil, g.unported("generator.construct.assume-prove")
	}
	n.boxed = parts[0].Image().String() == "[]ASSUME"
	prove := parts[len(parts)-2].Image().String()
	mismatch := n.boxed && prove != "[]PROVE" || !n.boxed && prove != "PROVE"
	if mismatch {
		loc, e := g.diagnosticLocation(parts[0])
		if e != nil {
			return nil, e
		}
		message := "ASSUME matched by []PROVE instead of PROVE"
		if n.boxed {
			message = "[]ASSUME matched by PROVE instead of []PROVE"
		}
		if e = addError(g.table.errors, 4005, loc, message); e != nil {
			return nil, e
		}
	}
	sentinelName := g.pool.intern("$$InAssume")
	sentinel, err := g.table.resolve(sentinelName, false)
	if err != nil {
		return nil, err
	}
	if n.boxed && sentinel != nil {
		loc, e := g.diagnosticLocation(parts[0])
		if e != nil {
			return nil, e
		}
		if e = addError(g.table.errors, 4005, loc, "[]ASSUME used within the scope of an ordinary ASSUME's assumptions"); e != nil {
			return nil, e
		}
	} else if !n.boxed && sentinel == nil {
		if g.inAssume == nil {
			hdr, e := newNodeHeader(g.pool, sentinelName, 0, nil)
			if e != nil {
				return nil, e
			}
			g.inAssume = &opDeclNode{nodeHeader: hdr, levels: newLeafLevel()}
			g.pool.history.attach(g.inAssume, g.pool.history.staticDummy)
		}
		if _, err = g.table.addSymbol(sentinelName, g.inAssume); err != nil {
			return nil, err
		}
	}
	count := (len(parts) - 2) / 2
	n.assumptions = make([]levelNode, count)
	n.declarations = make([]bool, count+1)
	for i := 0; i < count; i++ {
		n.declarations[i+1] = n.declarations[i]
		item := parts[2*i+1]
		switch item.Kind() {
		case generated.N_AssumeProve:
			n.assumptions[i], err = g.assumeProve(item, module, nil, depth+1)
		case generated.N_NewSymb:
			n.assumptions[i], err = g.newSymb(item, module, depth+1)
			n.declarations[i+1] = true
			g.apDeclScopes[len(g.apDeclScopes)-1] = true
		case generated.N_Label:
			n.assumptions[i], err = g.generateLabel(item, item.Heirs(), depth+1)
		default:
			n.assumptions[i], err = g.expression(item, depth+1)
		}
		if err != nil {
			return nil, err
		}
		if g.apDepth == 1 {
			g.goalClause++
		}
	}
	n.prove, err = g.expression(parts[len(parts)-1], depth+1)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (g *generator) newSymb(tree *generated.SyntaxTreeNode, module *moduleNode, depth int) (*newSymbNode, error) {
	if err := g.check(depth); err != nil {
		return nil, err
	}
	parts := tree.Heirs()
	if len(parts) < 2 {
		return nil, g.unported("generator.construct.new-symb")
	}
	var set exprNode
	var err error
	// IN is token kind 147 for both ASCII and Unicode membership spellings.
	if parts[len(parts)-2].Kind() == 147 {
		set, err = g.expression(parts[len(parts)-1], depth+1)
		if err != nil {
			return nil, err
		}
	}
	i := 0
	if parts[0].Image() != nil && parts[0].Image().String() == "NEW" {
		i = 1
	}
	if i >= len(parts) || parts[i].Image() == nil {
		return nil, g.unported("generator.construct.new-symb")
	}
	kind, level := 24, 0
	switch parts[i].Image().String() {
	case "CONSTANT":
		kind, level = 24, 0
	case "VARIABLE":
		kind, level = 25, 1
	case "STATE":
		kind, level = 26, 1
	case "ACTION":
		kind, level = 27, 2
	case "TEMPORAL":
		kind, level = 28, 3
	default:
		i = 0
	}
	if i+1 >= len(parts) {
		return nil, g.unported("generator.construct.new-symb")
	}
	decl := parts[i+1]
	h := decl.Heirs()
	if len(h) == 0 {
		return nil, g.unported("generator.construct.new-symb")
	}
	var named *generated.SyntaxTreeNode
	arity := 0
	switch decl.Kind() {
	case generated.N_IdentDecl:
		named = h[0]
		arity = (len(h) - 1) / 2
	case generated.N_PrefixDecl:
		named = h[0]
		arity = 1
	case generated.N_InfixDecl:
		if len(h) < 2 {
			return nil, g.unported("generator.construct.new-symb")
		}
		named = h[1]
		arity = 2
	case generated.N_PostfixDecl:
		if len(h) < 2 {
			return nil, g.unported("generator.construct.new-symb")
		}
		named = h[1]
		arity = 1
	default:
		return nil, g.unported("generator.construct.new-symb")
	}
	name, err := g.name(named)
	if err != nil {
		return nil, err
	}
	canonical, err := g.pool.syntax.ResolveSynonym(g.pool.syntax.UniqueStringOfUnits(name.units))
	if err != nil {
		return nil, g.unported("generator.precondition.synonym-lookup")
	}
	name = g.pool.internUnits(canonical.Units())
	ticket := g.pool.history.event("OpDeclNode", kind)
	d, err := newOpDecl(g.pool, name, kind, level, arity, decl, g.table, module)
	if d != nil {
		g.pool.history.attach(d, ticket)
	}
	if err != nil {
		return nil, err
	}
	ticket = g.pool.history.event("NewSymbNode", 22)
	sh, err := newSemanticHeader(g.pool, 22, tree)
	if err != nil {
		return nil, err
	}
	n := &newSymbNode{semanticHeader: sh, declaration: d, set: set}
	g.pool.history.attach(n, ticket)
	return n, nil
}

func (g *generator) rejectAPExpression(op leafNode, tree *generated.SyntaxTreeNode) (bool, error) {
	def, ok := op.(*thmOrAssumpDefNode)
	if !ok {
		return false, nil
	}
	// Java dereferences the theorem body in expression mode. A null body
	// after an earlier error follows an exception path, not a valid use.
	if def.body == nil {
		return false, g.unported("generator.construct.reject-ap-expression")
	}
	loc, err := g.diagnosticLocation(tree)
	if err != nil {
		return false, err
	}
	if _, ok = def.body.(*assumeProveNode); ok {
		return true, addError(g.table.errors, 4355, loc, "ASSUME/PROVE used where an expression is required.")
	}
	if app, ok := def.body.(*opApplNode); ok && app != nil && app.operator != nil && app.operator.header().name != nil {
		var kind string
		switch app.operator.header().name.text {
		case "$Qed":
			kind = "QED step"
		case "$Pfcase":
			kind = "CASE"
		case "$Have":
			kind = "HAVE"
		case "$Take":
			kind = "TAKE"
		case "$Pick":
			kind = "PICK"
		case "$Witness":
			kind = "WITNESS"
		case "$Suffices":
			kind = "SUFFICES"
		}
		if kind != "" {
			return true, addError(g.table.errors, 4351, loc, "%s proof step selected instead of expression.", exactStringParam(kind))
		}
	}
	return false, nil
}

func (n *thmOrAssumpDefNode) sourceNode() *thmOrAssumpDefNode {
	if n.source != nil {
		return n.source
	}
	return n
}
func (n *moduleNode) thmDefs() []*thmOrAssumpDefNode {
	if n.thmDefCache == nil {
		n.thmDefCache = make([]*thmOrAssumpDefNode, 0)
		for p := n.context.last; p != nil; p = p.prior {
			if d, ok := p.value.(*thmOrAssumpDefNode); ok {
				n.thmDefCache = append(n.thmDefCache, d)
			}
		}
		for i, j := 0, len(n.thmDefCache)-1; i < j; i, j = i+1, j-1 {
			n.thmDefCache[i], n.thmDefCache[j] = n.thmDefCache[j], n.thmDefCache[i]
		}
	}
	return n.thmDefCache
}

// theoremSelector follows source-backed expression selectors. Unsupported
// operator/fact modes retain an incomplete boundary.
func (g *generator) theoremSelector(tree *generated.SyntaxTreeNode, prefix []*generated.SyntaxTreeNode, last, finalArgs *generated.SyntaxTreeNode, root *thmOrAssumpDefNode, carried []exprOrOpArg, depth, expected int) (exprOrOpArg, error) {
	result, err := g.theoremLevelSelector(tree, prefix, last, finalArgs, root, carried, depth, expected, false)
	if err != nil || result == nil {
		return nil, err
	}
	value, ok := result.(exprOrOpArg)
	if !ok {
		return nil, g.unported("generator.construct.theorem-selector")
	}
	return value, nil
}

func (g *generator) theoremLevelSelector(tree *generated.SyntaxTreeNode, prefix []*generated.SyntaxTreeNode, last, finalArgs *generated.SyntaxTreeNode, root *thmOrAssumpDefNode, carried []exprOrOpArg, depth, expected int, fact bool) (levelNode, error) {
	if expected >= 0 && !g.simpleTheoremSelectorHistory(prefix, last, finalArgs, root, carried, expected) {
		g.pool.history.invalidate("unproved theorem selector history")
	}
	return g.definitionLevelSelector(tree, prefix, last, finalArgs, root, root.params, root.body, root.labels, carried, depth, expected, fact)
}

// simpleTheoremSelectorHistory follows represented theorem selectors without
// constructing semantic nodes. Argument producers and wrapper constructors
// independently record their events when the actual traversal runs.
func (g *generator) simpleTheoremSelectorHistory(prefix []*generated.SyntaxTreeNode, last, finalArgs *generated.SyntaxTreeNode, root *thmOrAssumpDefNode, carried []exprOrOpArg, expected int) bool {
	if root == nil {
		return false
	}
	return g.definitionSelectorHistory(prefix, last, finalArgs, root, root.params, root.body, root.labels, carried, expected)
}

func (g *generator) definitionSelectorHistory(prefix []*generated.SyntaxTreeNode, last, finalArgs *generated.SyntaxTreeNode, root leafNode, rootParams []*formalParamNode, body levelNode, rootLabels *labelSet, carried []exprOrOpArg, expected int) bool {
	if expected < 0 || root == nil || !g.pool.history.reuse(root).known || len(prefix) == 0 || last == nil {
		return false
	}
	first := prefix[0].Heirs()
	if len(first) != 2 && len(first) != 3 {
		return false
	}
	countArgs := func(args *generated.SyntaxTreeNode) (int, bool) {
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
	count := len(carried)
	if len(first) == 3 {
		n, ok := countArgs(first[1])
		if !ok || expected > 0 {
			return false
		}
		count += n
	}
	if expected == 0 && count != len(rootParams) {
		return true
	}
	if expected > 0 && len(carried) != 0 {
		return false
	}
	for _, p := range rootParams {
		if !g.pool.history.reuse(p).known {
			return false
		}
	}
	type selection struct{ node, args *generated.SyntaxTreeNode }
	selections := make([]selection, 0, len(prefix))
	for _, segment := range prefix[1:] {
		p := segment.Heirs()
		if (len(p) != 2 && len(p) != 3) || p[0] == nil {
			return false
		}
		item := selection{node: p[0]}
		if len(p) == 3 {
			item.args = p[1]
		}
		selections = append(selections, item)
	}
	selections = append(selections, selection{node: last, args: finalArgs})
	var current levelNode = body
	for {
		switch w := current.(type) {
		case *apSubstInNode:
			if !g.pool.history.reuse(w).known {
				return false
			}
			current = w.body
		case *substInNode:
			if !g.pool.history.reuse(w).known {
				return false
			}
			current = w.body
		default:
			goto unwrappedRoot
		}
	}
unwrappedRoot:
	if current == nil || !g.pool.history.reuse(current).known {
		return false
	}
	labels := rootLabels
	letReady := false
	inAPsuffices := false
	for i := 0; i < len(selections); i++ {
		sel := selections[i]
		node := sel.node
		if node == nil || (expected > 0 && sel.args != nil) {
			return false
		}
		colon := node.Kind() == generated.N_StructOp && len(node.Heirs()) == 0 && node.Image() != nil && node.Image().String() == ":"
		if colon {
			if letReady || sel.args != nil || i+1 == len(selections) {
				return false
			}
			letReady = true
			continue
		}
		if letReady && node.Kind() == 289 {
			for {
				label, ok := current.(*labelNode)
				if !ok {
					break
				}
				if !g.pool.history.reuse(label).known {
					return false
				}
				current = label.body
			}
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
			n, ok := countArgs(sel.args)
			if !ok {
				return false
			}
			if expected == 0 && n != local.arity {
				return true
			}
			for _, p := range local.params {
				if !g.pool.history.reuse(p).known {
					return false
				}
			}
			current, labels, letReady = local.body, local.labels, false
			continue
		}
		if node.Kind() == generated.N_StructOp || node.Kind() == generated.N_OpArgs {
			if sel.args != nil {
				return false
			}
			if expected > 0 && node.Kind() == generated.N_OpArgs {
				return true
			}
			for {
				label, ok := current.(*labelNode)
				if !ok {
					break
				}
				if !g.pool.history.reuse(label).known {
					return false
				}
				current = label.body
			}
			if ap, ok := current.(*assumeProveNode); ok {
				if ap.suffices && !inAPsuffices {
					if node.Kind() == generated.N_StructOp && len(node.Heirs()) == 0 && node.Image() != nil && node.Image().String() == "@" {
						return true
					}
					if node.Kind() == generated.N_OpArgs {
						return true
					}
					index, _, err := structuralIndex(node, 1)
					if err != nil {
						return false
					}
					if index != 1 {
						return true
					}
					inAPsuffices = true
					continue
				}
				inAPsuffices = false
				if node.Kind() == generated.N_OpArgs {
					return true
				}
				index, _, err := structuralIndex(node, len(ap.assumptions)+1)
				if err != nil {
					return false
				}
				if index < 1 {
					return true
				}
				if len(ap.declarations) < index {
					return false
				}
				if ap.declarations[index-1] && (ap.goal == nil || ap.goal.suffices == ap.inProof) {
					return true
				}
				if index <= len(ap.assumptions) {
					current = ap.assumptions[index-1]
				} else {
					current = ap.prove
				}
				if current == nil || !g.pool.history.reuse(current).known {
					return false
				}
				if _, ok := current.(*newSymbNode); ok && i+1 < len(selections) {
					return true
				}
				letReady = true
				continue
			}
			expr, ok := current.(exprNode)
			if !ok {
				return false
			}
			if expected > 0 && i+1 == len(selections) && g.selectorHistoryTerminalOpArg(expr, node) {
				return true
			}
			var second *generated.SyntaxTreeNode
			if i+1 < len(selections) && selections[i+1].args == nil {
				second = selections[i+1].node
			}
			value, isCase, known := g.selectorHistoryCase(expr, node, second)
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
			expr, ok = g.selectorHistoryOperand(expr, node)
			if !ok {
				return false
			}
			current, letReady = expr, true
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
		n, ok := countArgs(sel.args)
		if !ok {
			return false
		}
		if expected == 0 && n != len(label.params) {
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

// definitionLevelSelector traverses a theorem or operator body without creating
// a semantic wrapper for the selected definition.
func (g *generator) definitionLevelSelector(tree *generated.SyntaxTreeNode, prefix []*generated.SyntaxTreeNode, last, finalArgs *generated.SyntaxTreeNode, root leafNode, rootParams []*formalParamNode, body levelNode, rootLabels *labelSet, carried []exprOrOpArg, depth, expected int, fact bool) (levelNode, error) {
	if len(prefix) == 0 {
		return nil, g.unported("generator.construct.definition-level-selector")
	}
	first := prefix[0].Heirs()
	if len(first) != 2 && len(first) != 3 {
		return nil, g.unported("generator.construct.definition-level-selector")
	}
	var argTrees []*generated.SyntaxTreeNode
	if len(first) == 3 {
		if expected != 0 {
			return g.rejectSelectorArgumentsInOperatorMode(first[0])
		}
		if first[1].Kind() != generated.N_OpArgs {
			return nil, g.unported("generator.construct.definition-level-selector")
		}
		parts := first[1].Heirs()
		for i := 1; i < len(parts)-1; i += 2 {
			argTrees = append(argTrees, parts[i])
		}
	}
	if expected == 0 && len(carried)+len(argTrees) != len(rootParams) {
		return g.badArity(root.header().name, len(rootParams)-len(carried), tree)
	}
	args := append([]exprOrOpArg{}, carried...)
	for i, t := range argTrees {
		pos := len(carried) + i
		arg, err := g.expressionOrOperatorArgument(t, rootParams[pos].arity, pos, root, first[0], depth+1)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}
	params := append([]*formalParamNode{}, rootParams...)
	type selection struct{ node, args *generated.SyntaxTreeNode }
	selections := make([]selection, 0, len(prefix))
	for _, p := range prefix[1:] {
		parts := p.Heirs()
		if len(parts) != 2 && len(parts) != 3 {
			return nil, g.unported("generator.construct.definition-level-selector")
		}
		x := selection{node: parts[0]}
		if len(parts) == 3 {
			x.args = parts[1]
		}
		selections = append(selections, x)
	}
	selections = append(selections, selection{node: last, args: finalArgs})
	var current levelNode = body
	labels := rootLabels
	var letContext *context
	var pendingLocal *userOpDefNode
	inAPsuffices := false
	for {
		wrapper, ok := current.(*apSubstInNode)
		if !ok {
			break
		}
		if err := g.check(depth); err != nil {
			return nil, err
		}
		g.selectorSubsts = append(g.selectorSubsts, &substInNode{exprHeader: exprHeader{semanticHeader: wrapper.semanticHeader}, substs: wrapper.substs, instantiatingModule: wrapper.instantiatingModule, instantiatedModule: wrapper.instantiatedModule})
		current = wrapper.body
		depth++
	}
	for i := 0; i < len(selections); i++ {
		sel := selections[i]
		if err := g.check(depth + i + 1); err != nil {
			return nil, err
		}
		if expected != 0 && sel.args != nil {
			return g.rejectSelectorArgumentsInOperatorMode(sel.node)
		}
		if letContext != nil || pendingLocal != nil {
			// FindingOpName in the LET context: an IDENTIFIER or an
			// operator token, after resolveSynonym (Generator.java:735-738,
			// 777).
			if !isLetSelectorName(sel.node) {
				return nil, g.unported("generator.construct.definition-level-selector")
			}
			name, _, e := g.letSelectorName(sel.node)
			if e != nil {
				return nil, e
			}
			var value leafNode
			if pendingLocal != nil {
				value = pendingLocal
				pendingLocal = nil
			} else {
				value = letContext.lookup(contextKey{name: name})
			}
			local, ok := value.(*userOpDefNode)
			if value == nil {
				return g.unknownLetSelector(letContext, name, selections[i].node, depth+i+1, func(j int) *generated.SyntaxTreeNode {
					if i+j < len(selections) {
						return selections[i+j].node
					}
					return nil
				})
			}
			if !ok || len(local.params) != local.arity {
				return nil, g.unported("generator.construct.definition-level-selector")
			}
			if !local.defined {
				var trees []*generated.SyntaxTreeNode
				if sel.args != nil {
					parts := sel.args.Heirs()
					for j := 1; j < len(parts)-1; j += 2 {
						trees = append(trees, parts[j])
					}
				}
				if expected == 0 && len(trees) != local.arity {
					return g.badArity(name, local.arity, sel.node)
				}
				for j, t := range trees {
					if _, e := g.expressionOrOperatorArgument(t, local.params[j].arity, j, local, sel.node, depth+i+1); e != nil {
						return nil, e
					}
				}
				return g.rejectUndefinedSubexpression(local, sel.node)
			}
			if i == len(selections)-1 {
				if expected < 0 {
					return local, nil
				}
				if expected > 0 {
					return g.selectedLocalArgument(tree, local, params, expected)
				}
				return g.selectedLocalExpression(tree, sel.node, sel.args, root, local, params, args, depth+i)
			}
			var localArgs []*generated.SyntaxTreeNode
			if sel.args != nil {
				parts := sel.args.Heirs()
				for j := 1; j < len(parts)-1; j += 2 {
					localArgs = append(localArgs, parts[j])
				}
			}
			if expected == 0 && len(localArgs) != len(local.params) {
				return g.badArity(name, len(local.params), sel.node)
			}
			for j, t := range localArgs {
				arg, e := g.expressionOrOperatorArgument(t, local.params[j].arity, j, local, sel.node, depth+i+1)
				if e != nil {
					return nil, e
				}
				args = append(args, arg)
			}
			params = append(params, local.params...)
			body, e := g.selectorBody(local.body, depth+i+1)
			if e != nil {
				return nil, e
			}
			current = body
			labels = local.labels
			letContext = nil
			continue
		}
		if sel.node.Kind() == generated.N_StructOp || sel.node.Kind() == generated.N_OpArgs {
			if expected != 0 && sel.node.Kind() == generated.N_OpArgs {
				return g.rejectSelectorArgumentsInOperatorMode(sel.node)
			}
			if sel.args != nil {
				return g.rejectSelectorArgumentsInOperatorMode(sel.node)
			}
			for labelDepth := depth + i + 1; ; labelDepth++ {
				label, ok := current.(*labelNode)
				if !ok {
					break
				}
				if err := g.check(labelDepth); err != nil {
					return nil, err
				}
				current = label.body
			}
			if sel.node.Kind() == generated.N_StructOp && len(sel.node.Heirs()) == 0 && sel.node.Image() != nil && sel.node.Image().String() == ":" {
				if i+1 == len(selections) {
					if current == nil {
						// isNullSelection after the admitted colon
						// (Generator.java:1128-1136, 1544-1546).
						return g.nullSelected(sel.node)
					}
					continue
				}
				if !isLetSelectorName(selections[i+1].node) {
					loc, e := g.diagnosticLocation(sel.node)
					if e != nil {
						return nil, e
					}
					if e = addError(g.table.errors, 4005, loc, "`!:' can be used only after a name and either at the end after an\noperator name or before an operator name."); e != nil {
						return nil, e
					}
					return g.nullOAN, nil
				}
				if current == nil {
					return g.nullSelected(sel.node)
				}
				let, ok := current.(*letInNode)
				if !ok {
					loc, e := g.diagnosticLocation(sel.node)
					if e != nil {
						return nil, e
					}
					if e = addError(g.table.errors, 4005, loc, "A name selector here must be from a LET clause."); e != nil {
						return nil, e
					}
					return g.nullOAN, nil
				}
				letContext = let.context
				continue
			}
			if ap, ok := current.(*assumeProveNode); ok {
				if ap.suffices && !inAPsuffices {
					index := -1
					if sel.node.Kind() != generated.N_OpArgs && !(len(sel.node.Heirs()) == 0 && sel.node.Image() != nil && sel.node.Image().String() == "@") {
						var e error
						index, _, e = structuralIndex(sel.node, 1)
						if e != nil {
							return nil, e
						}
					}
					if index != 1 {
						loc, e := g.diagnosticLocation(sel.node)
						if e != nil {
							return nil, e
						}
						if e = addError(g.table.errors, 4005, loc, "Accessing non-existent subexpression of a SUFFICES"); e != nil {
							return nil, e
						}
						return g.nullOAN, nil
					}
					inAPsuffices = true
					continue
				}
				inAPsuffices = false
				if sel.node.Kind() == generated.N_OpArgs {
					return g.badStructuralIndex(sel.node, "!(...)")
				}
				if len(sel.node.Heirs()) == 0 && sel.node.Image() != nil && sel.node.Image().String() == "@" {
					return g.badStructuralIndex(sel.node, "@")
				}
				index, text, err := structuralIndex(sel.node, len(ap.assumptions)+1)
				if err != nil {
					return nil, err
				}
				if index < 0 {
					return g.badStructuralIndex(sel.node, text)
				}
				if ap.declarations[index-1] && (ap.goal == nil || ap.goal.suffices == ap.inProof) {
					loc, e := g.diagnosticLocation(sel.node)
					if e != nil {
						return nil, e
					}
					if e = addError(g.table.errors, 4005, loc, "Accessing ASSUME/PROVE clause within the scope of a declaration\n from outside that declaration's scope."); e != nil {
						return nil, e
					}
					return g.nullOAN, nil
				}
				if index <= len(ap.assumptions) {
					current = ap.assumptions[index-1]
				} else {
					current = ap.prove
				}
				if current == nil {
					// isNullSelection: an assumption (Generator.java:1446-1450)
					// or the PROVE clause (1544-1547) that is null after an
					// error in it, reported at the selector that chose it.
					return g.nullSelected(sel.node)
				}
				if _, ok := current.(*newSymbNode); ok && i < len(selections)-1 {
					loc, e := g.diagnosticLocation(sel.node)
					if e != nil {
						return nil, e
					}
					if e = addError(g.table.errors, 4005, loc, "Selected a subexpression of a NEW clause of an ASSUME."); e != nil {
						return nil, e
					}
					return g.nullOAN, nil
				}
				if i+1 < len(selections) {
					next := selections[i+1].node
					if isLetSelectorName(next) {
						for labelDepth := depth + i + 1; ; labelDepth++ {
							label, ok := current.(*labelNode)
							if !ok {
								break
							}
							if err := g.check(labelDepth); err != nil {
								return nil, err
							}
							current = label.body
							if current == nil {
								// isNullSelection after each label body
								// (Generator.java:1550-1556).
								return g.nullSelected(sel.node)
							}
						}
						local, ok := current.(*letInNode)
						if !ok {
							loc, e := g.diagnosticLocation(sel.node)
							if e != nil {
								return nil, e
							}
							if e = addError(g.table.errors, 4005, loc, "A name selector here must be from a LET clause."); e != nil {
								return nil, e
							}
							return g.nullOAN, nil
						}
						letContext = local.context
					} else if next.Kind() == generated.N_StructOp && len(next.Heirs()) == 0 && next.Image() != nil && next.Image().String() == ":" {
						loc, e := g.diagnosticLocation(sel.node)
						if e != nil {
							return nil, e
						}
						if e = addError(g.table.errors, 4005, loc, "!: should not follow an operand selector."); e != nil {
							return nil, e
						}
						return g.nullOAN, nil
					}
				}
				continue
			}
			if current == nil {
				// An operand selector applied to a null node, as after an
				// error in the selected definition (Generator.java:1138-1145).
				loc, e := g.diagnosticLocation(sel.node)
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4005, loc, "Subexpression selection failed, probably due to an error in the selected expression."); e != nil {
					return nil, e
				}
				return g.nullOAN, nil
			}
			expr, ok := current.(exprNode)
			if !ok {
				return nil, g.unported("generator.construct.definition-level-selector")
			}
			nameIndex := -1
			for j := i + 1; j < len(selections); j++ {
				if isLetSelectorName(selections[j].node) {
					nameIndex = j
					break
				}
			}
			if nameIndex >= 0 {
				var structural []*generated.SyntaxTreeNode
				for _, part := range selections[i:nameIndex] {
					if part.args != nil || (part.node.Kind() != generated.N_StructOp && part.node.Kind() != generated.N_OpArgs) {
						return nil, g.unported("generator.construct.definition-level-selector")
					}
					structural = append(structural, part.node)
				}
				target := selections[nameIndex]
				var bound []*formalParamNode
				var boundArgs []*generated.SyntaxTreeNode
				local, e := g.structuralLetFromAllowUndefined(expr, structural, target.node, &bound, &boundArgs, depth+i, expected != 0, true)
				if e == nil {
					for _, t := range boundArgs {
						arg, err := g.expression(t, depth+1)
						if err != nil {
							return nil, err
						}
						args = append(args, arg)
					}
					params = append(params, bound...)
					pendingLocal = local
					i = nameIndex - 1
					continue
				}
				if failure, ok := e.(*selectorTraversalError); ok {
					if failure.unknown != nil && nameIndex != len(selections)-1 {
						// The undefined name would extend into a compound
						// name with the later NameSel selectors
						// (Generator.java:732-813); that search needs the
						// LET context the structural walk ended in.
						return nil, g.unported("generator.construct.definition-level-selector")
					}
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
			rest := make([]*generated.SyntaxTreeNode, 0, len(selections)-i)
			for _, x := range selections[i:] {
				if x.args != nil || (x.node.Kind() != generated.N_StructOp && x.node.Kind() != generated.N_OpArgs) {
					return nil, g.unported("generator.construct.definition-level-selector")
				}
				rest = append(rest, x.node)
			}
			if expected != 0 {
				return g.numericSubexpression(tree, rest, root, expr, params, nil, depth+i, expected, true)
			}
			return g.numericExpression(tree, rest, root, expr, params, args, depth+i, true)
		}
		name, err := g.name(sel.node)
		if err != nil {
			return nil, err
		}
		label := labels.get(name)
		if label == nil {
			loc, e := g.diagnosticLocation(sel.node)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4004, loc, "Cannot find label `%s'.", nameStringParam(name)); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		if label.goal != nil {
			illegal, e := g.illegalProofLabelRef(label, sel.node)
			if e != nil {
				return nil, e
			}
			if illegal {
				loc, e := g.diagnosticLocation(sel.node)
				if e != nil {
					return nil, e
				}
				if e = addError(g.table.errors, 4005, loc, "Accessing subexpression labeled `%s' of ASSUME/PROVE clause within the scope of a declaration\n from outside that declaration's scope.", nameStringParam(name)); e != nil {
					return nil, e
				}
				return g.nullOAN, nil
			}
		}
		var labelArgs []*generated.SyntaxTreeNode
		if sel.args != nil {
			parts := sel.args.Heirs()
			for j := 1; j < len(parts)-1; j += 2 {
				labelArgs = append(labelArgs, parts[j])
			}
		}
		if expected == 0 && len(labelArgs) != len(label.params) {
			loc, e := g.diagnosticLocation(sel.node)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4337, loc, "Label `%s' used with wrong number of arguments.", nameStringParam(name)); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
		for _, t := range labelArgs {
			arg, e := g.expression(t, depth+1)
			if e != nil {
				return nil, e
			}
			args = append(args, arg)
		}
		params = append(params, label.params...)
		current = label
		labels = label.labels
	}
	if expected > 0 {
		_, isAP := current.(*assumeProveNode)
		_, isNew := current.(*newSymbNode)
		if !isAP && !isNew && len(params) != expected {
			loc, e := g.diagnosticLocation(tree)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4271, loc, "Expected arity %s but found operator of arity %s.", Parameter{Kind: IntParameter, Integer: int64(expected)}, Parameter{Kind: IntParameter, Integer: int64(len(params))}); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
	}
	switch n := current.(type) {
	case *assumeProveNode:
		if fact {
			return n, nil
		}
		loc, e := g.diagnosticLocation(tree)
		if e != nil {
			return nil, e
		}
		if e = addError(g.table.errors, 4005, loc, "Selected ASSUME/PROVE instead of expression."); e != nil {
			return nil, e
		}
		return g.nullOAN, nil
	case *newSymbNode:
		if fact {
			return n, nil
		}
		loc, e := g.diagnosticLocation(tree)
		if e != nil {
			return nil, e
		}
		if e = addError(g.table.errors, 4005, loc, "Selected a NEW declaration as an expression or operator."); e != nil {
			return nil, e
		}
		return g.nullOAN, nil
	case *labelNode:
		if expected < 0 {
			return g.invalidSelectedProofDef(tree)
		}
		if n.isAssumeProve && !fact {
			loc, e := g.diagnosticLocation(tree)
			if e != nil {
				return nil, e
			}
			if e = addError(g.table.errors, 4355, loc, "ASSUME/PROVE used where an expression is required."); e != nil {
				return nil, e
			}
			return g.nullOAN, nil
		}
	}
	if expected < 0 {
		return g.invalidSelectedProofDef(tree)
	}
	expr, ok := current.(exprNode)
	if !ok {
		return nil, g.unported("generator.construct.definition-level-selector")
	}
	if expected > 0 {
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
		wrapped, e := g.wrapSelectorBody(expr)
		if e != nil {
			return nil, e
		}
		return g.selectorLambdaArgument(tree, params, wrapped)
	}
	return g.selectorWrapper(tree, root, expr, params, args)
}

// unknownLetSelector reports an undefined name sought in a LET context.
// FindingOpName extends the undefined name with each later NameSel selector,
// skipping operand selectors, and stops at the first compound name the
// context defines (Generator.java:732-813). The error names the whole
// compound and spans its name parts. A compound the context does define
// continues the selection, which stays incomplete here. next(j) is the
// selector j places after the undefined one, or nil past the end. depth is
// the traversal depth checked for the undefined selector; each later
// selector, skipped operands included, is checked at depth+j before it is
// read or interned, as the enclosing selector loop checks each step.
func (g *generator) unknownLetSelector(letContext *context, first *name, firstNode *generated.SyntaxTreeNode, depth int, next func(int) *generated.SyntaxTreeNode) (levelNode, error) {
	loc, e := g.diagnosticLocation(firstNode)
	if e != nil {
		return nil, e
	}
	unknown := first
	units := append([]uint16{}, first.units...)
	for j := 1; next(j) != nil; j++ {
		if e = g.check(depth + j); e != nil {
			return nil, e
		}
		node := next(j)
		var part *name
		switch node.Kind() {
		case 290, 291: // ProofStepLexeme, ProofImplicitStepLexeme
			part, e = g.name(node)
		default:
			if !isLetSelectorName(node) {
				continue
			}
			part, _, e = g.letSelectorName(node)
		}
		if e != nil {
			return nil, e
		}
		units = append(append(units, '!'), part.units...)
		compound := g.pool.internUnits(units)
		if letContext == nil || letContext.lookup(contextKey{name: compound}) != nil {
			return nil, g.unported("generator.construct.definition-level-selector")
		}
		end, e := g.diagnosticLocation(node)
		if e != nil {
			return nil, e
		}
		loc.EndLine, loc.EndColumn = end.EndLine, end.EndColumn
		unknown = compound
	}
	if unknown != first && !unknown.displayExact {
		return nil, g.unported("generator.mapping.inexact-display-name")
	}
	if e = addError(g.table.errors, 4200, loc, "Unknown operator: `%s'.", nameParam(unknown)); e != nil {
		return nil, e
	}
	return g.nullOAN, nil
}

// children and walkChildren preserve the different orders used by SANY's
// getChildren and walkGraph. Provenance and statement backlinks are excluded.
func (n *assumeNode) children() []levelNode     { return []levelNode{n.expression} }
func (n *assumeNode) walkChildren() []levelNode { return n.children() }
func (n *theoremNode) children() []levelNode {
	if n.proof == nil {
		return []levelNode{n.body}
	}
	return []levelNode{n.body, n.proof}
}
func (n *theoremNode) walkChildren() []levelNode        { return n.children() }
func (n *thmOrAssumpDefNode) children() []levelNode     { return []levelNode{n.body} }
func (n *thmOrAssumpDefNode) walkChildren() []levelNode { return n.children() }
func (n *assumeProveNode) children() []levelNode {
	out := append([]levelNode{}, n.assumptions...)
	return append(out, n.prove)
}
func (n *assumeProveNode) walkChildren() []levelNode { return n.children() }
func (n *newSymbNode) children() []levelNode {
	if n.set == nil {
		return nil
	}
	return []levelNode{n.set}
}
func (n *newSymbNode) walkChildren() []levelNode   { return n.children() }
func (n *leafProofNode) children() []levelNode     { return append([]levelNode(nil), n.facts...) }
func (n *leafProofNode) walkChildren() []levelNode { return n.children() }
func (n *apSubstInNode) children() []levelNode {
	out := make([]levelNode, 0, len(n.substs)+1)
	out = append(out, n.body)
	for _, s := range n.substs {
		if s == nil {
			out = append(out, nil)
			continue
		}
		v, _ := s.expr.(levelNode)
		out = append(out, v)
	}
	return out
}
func (n *apSubstInNode) walkChildren() []levelNode {
	out := make([]levelNode, 0, 2*len(n.substs)+1)
	for _, s := range n.substs {
		if s == nil {
			continue
		}
		if s.op != nil {
			out = append(out, s.op)
		}
		if v, ok := s.expr.(levelNode); ok && v != nil {
			out = append(out, v)
		}
	}
	if n.body != nil {
		out = append(out, n.body)
	}
	return out
}
