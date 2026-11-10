// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ Generator.java, Copyright 2003 Compaq Corporation and
// Microsoft Corporation, under the MIT license; see LICENSE.
package semantic

import generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"

type subst struct {
	op       *opDeclNode
	expr     exprOrOpArg
	exprTree *generated.SyntaxTreeNode
	implicit bool
}
type substInNode struct {
	exprHeader
	substs                                  []*subst
	body                                    exprNode
	instantiatingModule, instantiatedModule *moduleNode
}

// simpleInstanceHistory guards malformed source forms before template events.
func (g *generator) simpleInstanceHistory(_ *moduleNode, children []*generated.SyntaxTreeNode) {
	if g.pool.history.exact && (len(children) < 2 || len(children)%2 != 0) {
		g.pool.history.invalidate("unproved instance syntax")
	}
}

func (g *generator) wrapInstanceBody(tree *generated.SyntaxTreeNode, template *substInNode, body exprNode, current, target *moduleNode) (*substInNode, error) {
	ticket := g.pool.history.event("SubstInNode", 13)
	h, err := newExprHeader(g.pool, 13, tree)
	if err != nil {
		return nil, err
	}
	if body == nil {
		loc, err := g.diagnosticLocation(tree)
		if err != nil {
			return nil, err
		}
		if err = addError(g.table.errors, 4003, loc, "Substitution error, probably due to error \nin module being instantiated."); err != nil {
			return nil, err
		}
	}
	result := &substInNode{exprHeader: h, substs: template.substs, body: body, instantiatingModule: current, instantiatedModule: target}
	g.pool.history.attach(result, ticket)
	return result, nil
}

func (g *generator) processSubst(tree *generated.SyntaxTreeNode, children []*generated.SyntaxTreeNode, target, current *moduleNode, depth int) (*substInNode, error) {
	ticket := g.pool.history.event("SubstInNode", 13)
	h, e := newExprHeader(g.pool, 13, tree)
	if e != nil {
		return nil, e
	}
	template := &substInNode{exprHeader: h, substs: make([]*subst, 0), instantiatingModule: current, instantiatedModule: target}
	g.pool.history.attach(template, ticket)
	var decls []*opDeclNode
	for _, v := range target.context.orderedValues() {
		if d, ok := v.(*opDeclNode); ok {
			decls = append(decls, d)
		}
	}
	for _, d := range decls {
		if e = g.check(depth); e != nil {
			return nil, e
		}
		rhs, err := g.table.resolve(d.name, false)
		if err != nil {
			return nil, err
		}
		if rhs == nil {
			continue
		}
		if !g.pool.history.reuse(rhs).known {
			g.pool.history.invalidate("unproved implicit substitution resolution")
		}
		var expr exprOrOpArg
		if d.arity == 0 {
			ticket := g.pool.history.event("OpApplNode", 9)
			eh, err := newExprHeader(g.pool, 9, tree)
			if err != nil {
				return nil, err
			}
			expr = &opApplNode{exprHeader: eh, operator: rhs, operands: make([]exprOrOpArg, 0), ranges: make([]exprNode, 0)}
			g.pool.history.attach(expr, ticket)
			if nodeArity(rhs) != 0 {
				loc, err := g.diagnosticLocation(tree)
				if err != nil {
					return nil, err
				}
				switch rhs.(type) {
				case *opDeclNode:
					err = addError(g.table.errors, 4004, loc, "Operator used with the wrong number of arguments.")
				case *userOpDefNode, *builtinOpDefNode:
					err = addError(g.table.errors, 4004, loc, "Wrong number of arguments (%s) given to operator '%s', \nwhich requires %s arguments.", Parameter{Kind: IntParameter, Integer: 0}, nameParam(rhs.header().name), Parameter{Kind: IntParameter, Integer: int64(nodeArity(rhs))})
				}
				if err != nil {
					return nil, err
				}
			}
		} else {
			ticket := g.pool.history.event("OpArgNode", 8)
			expr, err = newOpArg(g.pool, rhs, tree, current)
			if err == nil {
				g.pool.history.attach(expr, ticket)
			}
			if err != nil {
				return nil, err
			}
		}
		template.substs = append(template.substs, &subst{op: d, expr: expr, implicit: true})
	}
	for i := 3; i < len(children); i += 2 {
		parts := children[i].Heirs()
		if len(parts) != 3 {
			return nil, g.unported("generator.construct.process-subst")
		}
		name, err := g.name(parts[0])
		if err != nil {
			return nil, err
		}
		d, ok := target.context.lookup(contextKey{name: name}).(*opDeclNode)
		if !ok {
			g.pool.history.invalidate("unproved substitution target")
			loc, err := g.diagnosticLocation(parts[0])
			if err != nil {
				return nil, err
			}
			err = addError(g.table.errors, 4242, loc, "Identifier '%s' is not a legal target of a substitution. \nA legal target must be a declared CONSTANT or VARIABLE in the module being instantiated. \n(Also, check for warnings about multiple declarations of this same identifier.)", nameParam(name))
			if err != nil {
				return nil, err
			}
			continue
		}
		var rhs exprOrOpArg
		if d.arity == 0 {
			rhs, err = g.expression(parts[2], depth+1)
		} else {
			rhs, err = g.substitutionOpArg(parts[2], d, current, depth+1)
		}
		if err != nil {
			return nil, err
		}
		if err = g.checkSubstitutionArity(d, rhs); err != nil {
			return nil, err
		}
		found := false
		for _, s := range template.substs {
			if s.op.name == name {
				found = true
				if !s.implicit {
					loc, err := g.diagnosticLocation(parts[2])
					if err != nil {
						return nil, err
					}
					if err = addError(g.table.errors, 4241, loc, "Multiple substitutions for symbol '%s' in substitution.", nameStringParam(name)); err != nil {
						return nil, err
					}
				} else {
					s.expr = rhs
					s.exprTree = parts[2]
					s.implicit = false
				}
				break
			}
		}
		if !found {
			template.substs = append(template.substs, &subst{op: d, expr: rhs, exprTree: parts[2]})
		}
	}
	for _, s := range template.substs {
		if s.implicit {
			if err := g.checkSubstitutionArity(s.op, s.expr); err != nil {
				return nil, err
			}
		}
	}
	for _, d := range decls {
		found := false
		for _, s := range template.substs {
			if s.op.name == d.name {
				found = true
				break
			}
		}
		if !found {
			loc, err := g.diagnosticLocation(tree)
			if err != nil {
				return nil, err
			}
			dl, err := nodeLocation(d)
			if err != nil {
				return nil, err
			}
			if err = addError(g.table.errors, 4240, loc, "Substitution missing for symbol %s declared at %s \nand instantiated in module %s.", nameParam(d.name), locParam(dl), nameParam(current.name)); err != nil {
				return nil, err
			}
		}
	}
	return template, nil
}
func (g *generator) checkSubstitutionArity(d *opDeclNode, rhs exprOrOpArg) error {
	actual := 0
	if a, ok := rhs.(*opArgNode); ok {
		actual = a.arity
	}
	if actual == d.arity {
		return nil
	}
	loc, err := g.diagnosticLocation(rhs.argumentHeader().tree)
	if err != nil {
		return err
	}
	return addError(g.table.errors, 4243, loc, "An operator must be substituted for symbol '%s', and it must have arity %s.", nameParam(d.name), Parameter{Kind: IntParameter, Integer: int64(d.arity)})
}
func (g *generator) substitutionOpArg(tree *generated.SyntaxTreeNode, target *opDeclNode, current *moduleNode, depth int) (exprOrOpArg, error) {
	if tree.Kind() == generated.N_Lambda {
		parts := tree.Heirs()
		if len(parts) < 4 || len(parts)%2 != 0 {
			return nil, g.unported("generator.construct.substitution-op-arg")
		}
		return g.lambdaArgument(tree, tree, (len(parts)-2)/2, 0, nil, depth)
	}
	general := tree.Kind() == generated.N_GeneralId
	var n *name
	var rawName *name
	if general {
		parts := tree.Heirs()
		if len(parts) != 2 {
			return nil, g.unported("generator.construct.substitution-op-arg")
		}
		if len(parts[0].Heirs()) != 0 {
			if !g.qualifiedOperatorArgumentHistory(parts) && !g.directLetOperatorArgumentHistory(parts) && !g.selectorOperatorArgumentHistory(parts, target.arity) {
				g.pool.history.invalidate("unproved substitution selector")
			}
			selected, err := g.labelOperatorArgument(tree, parts, target.arity, depth)
			if err != nil {
				return nil, err
			}
			if _, ok := selected.(*opArgNode); !ok {
				return g.nullOpArg, nil
			}
			return selected, nil
		}
		var e error
		n, e = g.name(parts[1])
		if e != nil {
			return nil, e
		}
	} else {
		switch tree.Kind() {
		case generated.N_GenInfixOp, generated.N_GenPrefixOp, generated.N_GenNonExpPrefixOp, generated.N_GenPostfixOp:
		default:
			g.pool.history.invalidate("unproved substitution argument")
			loc, err := g.diagnosticLocation(tree)
			if err != nil {
				return nil, err
			}
			if err = addError(g.table.errors, 4004, loc, "Arity %s operator (not an expression) is expected \nto substitute for CONSTANT '%s'.", Parameter{Kind: IntParameter, Integer: int64(target.arity)}, nameParam(target.name)); err != nil {
				return nil, err
			}
			return g.nullOpArg, nil
		}
		parts := tree.Heirs()
		if len(parts) != 2 || len(parts[0].Heirs()) != 0 {
			return nil, g.unported("generator.construct.substitution-op-arg")
		}
		var e error
		n, e = g.name(parts[1])
		if e != nil {
			return nil, e
		}
		rawName = n
		u, e := g.pool.syntax.ResolveSynonym(g.pool.syntax.UniqueStringOfUnits(n.units))
		if e != nil {
			return nil, e
		}
		n = g.pool.internUnits(u.Units())
	}
	var op leafNode
	var e error
	if general {
		op, e = g.resolveOperator(n, tree)
	} else {
		op, e = g.table.resolve(n, false)
		if e == nil && op == nil {
			loc, err := g.diagnosticLocation(tree)
			if err != nil {
				return nil, err
			}
			e = addError(g.table.errors, 4004, loc, "Could not find declaration or definition of symbol '%s'.", nameParam(rawName))
		}
	}
	if e != nil {
		return nil, e
	}
	if op == nil || !g.pool.history.reuse(op).known {
		g.pool.history.invalidate("unproved substitution resolution")
	}
	if op == nil {
		return g.nullOpArg, nil
	}
	if general && nodeArity(op) != target.arity {
		loc, e := g.diagnosticLocation(tree)
		if e != nil {
			return nil, e
		}
		if e = addError(g.table.errors, 4271, loc, "Expected arity %s but found operator of arity %s.", Parameter{Kind: IntParameter, Integer: int64(target.arity)}, Parameter{Kind: IntParameter, Integer: int64(nodeArity(op))}); e != nil {
			return nil, e
		}
		return g.nullOpArg, nil
	}
	ticket := g.pool.history.event("OpArgNode", 8)
	result, err := newOpArg(g.pool, op, tree, current)
	if result != nil {
		g.pool.history.attach(result, ticket)
	}
	return result, err
}

type instanceNode struct {
	semanticHeader
	stepName *name
	name     *name
	params   []*formalParamNode
	local    bool
	module   *moduleNode
	substs   []*subst
}

// moduleDefinitionOutput is processModuleDefinition's caller-owned defs and
// insts vectors (Generator.java:4739-4752). A nil output is Java's null
// vectors: the definitions and instance go onto the module-level lists. A
// non-nil output receives the instantiated OpDef and ThmOrAssumpDef clones,
// then the ModuleInstanceKind marker (Generator.java:5051-5065), in defs, and
// the InstanceNode in instance. processLetIn keeps both
// (Generator.java:2998-3000); a proof DEF step keeps the marker and the
// instance and discards the clones (Generator.java:5985-5994).
type moduleDefinitionOutput struct {
	defs     []leafNode
	marker   *userOpDefNode
	instance *instanceNode
}

// localInstance generates an unnamed top-level INSTANCE.
func (g *generator) localInstance(tree *generated.SyntaxTreeNode, current *moduleNode) error {
	return g.unnamedInstance(tree, current, true, nil, 1)
}
func (g *generator) unnamedInstance(tree *generated.SyntaxTreeNode, current *moduleNode, topLevel bool, output **instanceNode, depth int) (resultErr error) {
	defer func() {
		if resultErr != nil {
			g.pool.history.invalidate("instance traversal incomplete")
		}
	}()
	if err := g.check(depth); err != nil {
		return err
	}
	if tree == nil {
		return g.unported("generator.construct.unnamed-instance")
	}
	h := tree.Heirs()
	local := false
	var children []*generated.SyntaxTreeNode
	if topLevel {
		if (len(h) != 1 && (len(h) != 2 || h[0].Kind() != 54)) || h[len(h)-1].Kind() != generated.N_NonLocalInstance {
			return g.unported("generator.construct.unnamed-instance")
		}
		local = len(h) == 2
		children = h[len(h)-1].Heirs()
	} else {
		// Java SyntaxTreeNode.local tests whether zero is non-null; proof
		// instance nodes retain an empty, non-null zero array.
		local = tree.Zero() != nil
		if tree.Kind() != generated.N_NonLocalInstance {
			return g.unported("generator.construct.unnamed-instance")
		}
		children = h
	}
	if len(children) < 2 || (len(children) > 2 && (len(children)%2 != 0 || children[2].Kind() != 87)) {
		return g.unported("generator.construct.unnamed-instance")
	}
	name, err := g.name(children[1])
	if err != nil {
		return err
	}
	targetLeaf, err := g.table.resolve(name, true)
	if err != nil {
		return err
	}
	target, ok := targetLeaf.(*moduleNode)
	if !ok {
		if !topLevel && targetLeaf == nil {
			loc, e := g.diagnosticLocation(children[1])
			if e != nil {
				return e
			}
			code, ok := CodeFromValue(4003)
			if !ok {
				return g.unported("generator.construct.unnamed-instance")
			}
			abort, e := g.table.errors.Add(code, &loc, "Internal error: No context available for module `%s'.", nameStringParam(name))
			if e != nil {
				return e
			}
			return abort
		}
		return g.unported("generator.construct.unnamed-instance")
	}
	g.simpleInstanceHistory(target, children)
	target.instantiated = true
	template, err := g.processSubst(tree, children, target, current, depth+1)
	if err != nil {
		return err
	}
	for _, value := range target.context.orderedValues() {
		if err = g.check(depth + 1); err != nil {
			return err
		}
		original, ok := value.(*userOpDefNode)
		if !ok || original.local || original.kind == 6 {
			continue
		}
		definition := original
		// Generator.java:5464 clones a parameter-free origin of a parameterised
		// instancee only for a LOCAL top-level INSTANCE; Generator.java:5498
		// clones for every LOCAL instance of a parameter-free instancee, proof
		// instances included.
		wrapped := !target.parameterFree() && !original.module.parameterFree()
		if wrapped || (local && (topLevel || target.parameterFree())) {
			body := original.body
			origin := original.module
			if wrapped {
				body, err = g.wrapInstanceBody(tree, template, original.body, current, target)
				if err != nil {
					return err
				}
				origin = current
			}
			// Set source before insertion, as conflicts compare original definitions.
			source := original
			if original.source != nil {
				source = original.source
			}
			ticket := g.pool.history.event("OpDefNode", 5)
			clone, err := newUserOpDef(g.pool, original.name, original.params, local, body, origin, nil, tree)
			if clone != nil {
				g.pool.history.attach(clone, ticket)
			}
			if err != nil {
				return err
			}
			clone.source = source
			clone.labels = original.labels
			definition = clone
		}
		if _, err = g.table.addSymbol(definition.name, definition); err != nil {
			return err
		}
		current.definitions = append(current.definitions, definition)
		current.bodyDefinitions = append(current.bodyDefinitions, definition)
		definition.letInLevel = g.letLevel
		if g.unresolvedSum > 0 {
			definition.inRecursiveSection = g.unresolved[g.letLevel] > 0
			definition.recursiveSection = g.recursiveSection
			current.recursiveSectionDefs = append(current.recursiveSectionDefs, definition)
		}
	}
	for _, value := range target.context.orderedValues() {
		if err = g.check(depth + 1); err != nil {
			return err
		}
		original, ok := value.(*thmOrAssumpDefNode)
		if !ok || original.local {
			continue
		}
		result := original
		if !target.parameterFree() && !original.module.parameterFree() {
			ticket := g.pool.history.event("APSubstInNode", 30)
			h, e := newSemanticHeader(g.pool, 30, tree)
			if e != nil {
				return e
			}
			if original.body == nil {
				loc, e := g.diagnosticLocation(tree)
				if e != nil {
					return e
				}
				if e = addError(g.table.errors, 4004, loc, "Substitution error, probably due to error in module being instantiated."); e != nil {
					return e
				}
			}
			body := &apSubstInNode{semanticHeader: h, substs: template.substs, body: original.body, instantiatingModule: current, instantiatedModule: target}
			g.pool.history.attach(body, ticket)
			ticket = g.pool.history.event("ThmOrAssumpDefNode", 23)
			result, err = newThmOrAssumpDef(g.pool, original.name, tree)
			if result != nil {
				g.pool.history.attach(result, ticket)
			}
			if err != nil {
				return err
			}
			result.theorem, result.body, result.module = original.theorem, body, current
			result.source, result.instantiatedFrom, result.local, result.labels = original.sourceNode(), target, local, original.labels
			if _, err = g.table.addSymbol(result.name, result); err != nil {
				return err
			}
			result.params = original.params
		} else if local && topLevel {
			// Generator.java:5554 and 5597 test localness && topLevel: a proof
			// INSTANCE reuses the instancee's node although local() holds.
			origin := original.module
			copyLabels := true
			if !target.parameterFree() {
				origin = current
				copyLabels = false
			}
			ticket := g.pool.history.event("ThmOrAssumpDefNode", 23)
			result, err = newThmOrAssumpDef(g.pool, original.name, tree)
			if result != nil {
				g.pool.history.attach(result, ticket)
			}
			if err != nil {
				return err
			}
			result.theorem, result.body, result.module = original.theorem, original.body, origin
			result.source, result.instantiatedFrom, result.local = original.sourceNode(), target, true
			if copyLabels {
				result.labels = original.labels
			}
			if _, err = g.table.addSymbol(result.name, result); err != nil {
				return err
			}
			result.params = original.params
		} else if target.parameterFree() {
			if _, err = g.table.addSymbol(result.name, result); err != nil {
				return err
			}
		}
		if topLevel {
			current.definitions = append(current.definitions, result)
			current.bodyDefinitions = append(current.bodyDefinitions, result)
		}
	}
	ticket := g.pool.history.event("InstanceNode", 21)
	header, err := newSemanticHeader(g.pool, 21, tree)
	if err != nil {
		return err
	}
	instance := &instanceNode{semanticHeader: header, local: local, module: target, substs: template.substs}
	g.pool.history.attach(instance, ticket)
	if topLevel {
		current.instances = append(current.instances, instance)
		current.topLevel = append(current.topLevel, instance)
	} else if output != nil {
		*output = instance
	}
	return nil
}

func (g *generator) moduleDefinition(tree *generated.SyntaxTreeNode, current *moduleNode, out *moduleDefinitionOutput, depth int) (err error) {
	defer func() {
		if err != nil {
			g.pool.history.invalidate("module definition incomplete")
		}
	}()
	one := tree.One()
	if len(one) != 3 || one[0].Kind() != generated.N_IdentLHS || one[2].Kind() != generated.N_NonLocalInstance {
		return g.unported("generator.construct.module-definition")
	}
	lhs := one[0].Heirs()
	children := one[2].Heirs()
	if len(lhs) == 0 || len(children) < 2 {
		return g.unported("generator.construct.module-definition")
	}
	instanceName, e := g.name(lhs[0])
	if e != nil {
		return e
	}
	local := tree.Zero() != nil
	params := make([]*formalParamNode, 0)
	scopeOpen := false
	if len(lhs) > 1 {
		if e = g.table.push(newContext(g.pool)); e != nil {
			return e
		}
		scopeOpen = true
		defer func() {
			if scopeOpen {
				_ = g.table.pop()
			}
		}()
		for i := 2; i < len(lhs)-1; i += 2 {
			decl := lhs[i]
			parts := decl.Heirs()
			if len(parts) == 0 {
				return g.unported("generator.construct.module-definition")
			}
			n := parts[0]
			arity := 0
			switch decl.Kind() {
			case generated.N_IdentDecl:
				arity = (len(parts) - 1) / 2
			case generated.N_PrefixDecl:
				arity = 1
			case generated.N_PostfixDecl:
				if len(parts) < 2 {
					return g.unported("generator.construct.module-definition")
				}
				n = parts[1]
				arity = 1
			case generated.N_InfixDecl:
				if len(parts) < 2 {
					return g.unported("generator.construct.module-definition")
				}
				n = parts[1]
				arity = 2
			default:
				return g.unported("generator.construct.module-definition")
			}
			pn, e := g.name(n)
			if e != nil {
				return e
			}
			ticket := g.pool.history.event("FormalParamNode", 11)
			p, e := newFormalParam(g.pool, pn, arity, decl, g.table, current)
			if p != nil {
				g.pool.history.attach(p, ticket)
			}
			if e != nil {
				return e
			}
			params = append(params, p)
		}
	}
	targetName, e := g.name(children[1])
	if e != nil {
		return e
	}
	v, e := g.table.resolve(targetName, true)
	if e != nil {
		return e
	}
	target, ok := v.(*moduleNode)
	if !ok {
		return g.unported("generator.construct.module-definition")
	}
	g.simpleInstanceHistory(target, children)
	target.instantiated = true
	template, e := g.processSubst(tree, children, target, current, depth+1)
	if e != nil {
		return e
	}
	if len(lhs) > 1 {
		if e = g.table.pop(); e != nil {
			return e
		}
		scopeOpen = false
	}
	for _, v := range target.context.orderedValues() {
		if e = g.check(depth); e != nil {
			return e
		}
		orig, ok := v.(*userOpDefNode)
		if !ok || orig.local || (orig.kind != 5 && orig.kind != 6) {
			continue
		}
		qualified := g.pool.internUnits(append(append(append([]uint16{}, instanceName.units...), uint16('!')), orig.name.units...))
		p := append(append([]*formalParamNode{}, params...), orig.params...)
		var clone *userOpDefNode
		if orig.kind == 6 {
			ticket := g.pool.history.event("OpDefNode", 6)
			clone, e = newModuleMarker(g.pool, qualified, p, local, orig.module, orig.sourceNode(), tree)
			if clone != nil {
				g.pool.history.attach(clone, ticket)
			}
		} else {
			body := orig.body
			if len(template.substs) > 0 {
				body, e = g.wrapInstanceBody(tree, template, orig.body, current, target)
				if e != nil {
					return e
				}
			}
			ticket := g.pool.history.event("OpDefNode", 5)
			clone, e = newUserOpDef(g.pool, qualified, p, local, body, current, nil, tree)
			if clone != nil {
				g.pool.history.attach(clone, ticket)
			}
			if e == nil {
				clone.source = orig.sourceNode()
				clone.labels = orig.labels
				if len(template.substs) > 0 {
					clone.compoundID = append([]*name{instanceName}, orig.compound()...)
				}
				clone.letInLevel = g.letLevel
				if g.unresolvedSum > 0 {
					clone.inRecursiveSection = g.unresolved[g.letLevel] > 0
					clone.recursiveSection = g.recursiveSection
					current.recursiveSectionDefs = append(current.recursiveSectionDefs, clone)
				}
			}
		}
		if e != nil {
			return e
		}
		if _, e = g.table.addSymbol(qualified, clone); e != nil {
			return e
		}
		if out == nil {
			current.definitions = append(current.definitions, clone)
			current.bodyDefinitions = append(current.bodyDefinitions, clone)
		} else {
			out.defs = append(out.defs, clone)
		}
	}
	for _, value := range target.context.orderedValues() {
		if e = g.check(depth); e != nil {
			return e
		}
		original, ok := value.(*thmOrAssumpDefNode)
		if !ok || original.local {
			continue
		}
		qualified := g.pool.internUnits(append(append(append([]uint16{}, instanceName.units...), uint16('!')), original.name.units...))
		body := original.body
		if len(template.substs) > 0 {
			ticket := g.pool.history.event("APSubstInNode", 30)
			h, x := newSemanticHeader(g.pool, 30, tree)
			if x != nil {
				return x
			}
			if body == nil {
				loc, x := g.diagnosticLocation(tree)
				if x != nil {
					return x
				}
				if x = addError(g.table.errors, 4004, loc, "Substitution error, probably due to error in module being instantiated."); x != nil {
					return x
				}
			}
			body = &apSubstInNode{semanticHeader: h, substs: template.substs, body: body, instantiatingModule: current, instantiatedModule: target}
			g.pool.history.attach(body, ticket)
		}
		ticket := g.pool.history.event("ThmOrAssumpDefNode", 23)
		clone, x := newThmOrAssumpDef(g.pool, qualified, tree)
		if clone != nil {
			g.pool.history.attach(clone, ticket)
		}
		if x != nil {
			return x
		}
		clone.theorem, clone.body, clone.module = original.theorem, body, current
		clonedParams := append(append([]*formalParamNode{}, params...), original.params...)
		clone.source, clone.instantiatedFrom, clone.local, clone.labels = original.sourceNode(), target, local, original.labels
		if _, x = g.table.addSymbol(qualified, clone); x != nil {
			return x
		}
		clone.params = clonedParams
		if out == nil {
			current.definitions = append(current.definitions, clone)
			current.bodyDefinitions = append(current.bodyDefinitions, clone)
		} else {
			out.defs = append(out.defs, clone)
		}
	}
	ticket := g.pool.history.event("InstanceNode", 21)
	h, e := newSemanticHeader(g.pool, 21, tree)
	if e != nil {
		return e
	}
	instance := &instanceNode{semanticHeader: h, name: instanceName, params: params, local: local, module: target, substs: template.substs}
	g.pool.history.attach(instance, ticket)
	if out == nil {
		current.instances = append(current.instances, instance)
		current.topLevel = append(current.topLevel, instance)
	} else {
		out.instance = instance
	}
	ticket = g.pool.history.event("OpDefNode", 6)
	marker, e := newModuleMarker(g.pool, instanceName, params, local, current, nil, tree)
	if marker != nil {
		g.pool.history.attach(marker, ticket)
	}
	if e != nil {
		return e
	}
	if _, e = g.table.addSymbol(instanceName, marker); e != nil {
		return e
	}
	if out != nil {
		out.marker = marker
		out.defs = append(out.defs, marker)
	}
	return nil
}

func (g *generator) incompleteInstance(name *name, marker *userOpDefNode, tree, syntaxArgs *generated.SyntaxTreeNode, depth int) (exprNode, error) {
	var trees []*generated.SyntaxTreeNode
	locTree := tree
	if syntaxArgs != nil {
		locTree = syntaxArgs
		parts := syntaxArgs.Heirs()
		for i := 1; i < len(parts)-1; i += 2 {
			trees = append(trees, parts[i])
		}
	}
	if len(trees) != marker.arity {
		return g.badArity(name, marker.arity, locTree)
	}
	for i, t := range trees {
		if i >= len(marker.params) {
			return nil, g.unported("generator.construct.incomplete-instance")
		}
		if _, e := g.expressionOrOperatorArgument(t, marker.params[i].arity, i, marker, tree, depth+1); e != nil {
			return nil, e
		}
	}
	loc, e := g.diagnosticLocation(tree)
	if e != nil {
		return nil, e
	}
	if e = addError(g.table.errors, 4203, loc, "Operator name %s is incomplete.", nameStringParam(name)); e != nil {
		return nil, e
	}
	return g.nullOAN, nil
}

// unresolvedQualifiedSelector finishes the source compound-name search after
// an instance marker. Unsupported resolved target kinds remain incomplete.
func (g *generator) unresolvedQualifiedSelector(prefix []*generated.SyntaxTreeNode, last *generated.SyntaxTreeNode, missing int, depth int) (exprNode, error) {
	parts := make([]*generated.SyntaxTreeNode, 0, len(prefix)+1)
	for _, segment := range prefix {
		p := segment.Heirs()
		if len(p) != 2 && len(p) != 3 {
			return nil, g.unported("generator.construct.unresolved-qualified-selector")
		}
		parts = append(parts, p[0])
	}
	parts = append(parts, last)
	var units []uint16
	var location Location
	for i, part := range parts {
		if err := g.check(depth + i + 1); err != nil {
			return nil, err
		}
		switch part.Kind() {
		case 289, generated.N_InfixOp, generated.N_NonExpPrefixOp, generated.N_PostfixOp, generated.N_PrefixOp:
			if part.Image() == nil {
				return nil, g.unported("generator.construct.unresolved-qualified-selector")
			}
			normalized, err := g.pool.syntax.ResolveSynonym(g.pool.syntax.UniqueStringOfUnits(part.Image().Units()))
			if err != nil {
				return nil, err
			}
			if len(units) != 0 {
				units = append(units, '!')
			}
			units = append(units, normalized.Units()...)
			loc, err := g.diagnosticLocation(part)
			if err != nil {
				return nil, err
			}
			if location.NamePresent {
				location.EndLine, location.EndColumn = loc.EndLine, loc.EndColumn
			} else {
				location = loc
			}
			if i >= missing {
				v, err := g.table.resolve(g.pool.internUnits(units), false)
				if err != nil {
					return nil, err
				}
				if v != nil {
					return nil, g.unported("generator.construct.unresolved-qualified-selector")
				}
			}
		case generated.N_StructOp, generated.N_OpArgs:
		default:
			return nil, g.unported("generator.construct.unresolved-qualified-selector")
		}
	}
	n := g.pool.internUnits(units)
	if !n.displayExact {
		return nil, g.unported("generator.mapping.inexact-display-name")
	}
	if err := addError(g.table.errors, 4200, location, "Unknown operator: `%s'.", nameParam(n)); err != nil {
		return nil, err
	}
	return g.nullOAN, nil
}

// qualifiedSelectorPrefix walks instance markers until it reaches a definition.
// Positive-arity operator selection follows selectorToNode: segment arguments
// are not generated in this mode; the final operator's complete arity is used.
func (g *generator) qualifiedSelectorPrefix(tree *generated.SyntaxTreeNode, prefix []*generated.SyntaxTreeNode, last *generated.SyntaxTreeNode, depth int, operatorMode bool) (int, leafNode, []*name, []exprOrOpArg, exprNode, error) {
	var names []*name
	var units []uint16
	var args []exprOrOpArg
	found := 0
	for index, segment := range prefix {
		if err := g.check(depth + index + 1); err != nil {
			return 0, nil, nil, nil, nil, err
		}
		parts := segment.Heirs()
		if (len(parts) != 2 && len(parts) != 3) || parts[0].Kind() != 289 {
			return 0, nil, nil, nil, nil, g.unported("generator.construct.qualified-selector-prefix")
		}
		if operatorMode && len(parts) == 3 {
			failure, err := g.rejectSelectorArgumentsInOperatorMode(parts[0])
			return 0, nil, nil, nil, failure, err
		}
		n, err := g.name(parts[0])
		if err != nil {
			return 0, nil, nil, nil, nil, err
		}
		if len(units) != 0 {
			units = append(units, '!')
		}
		units = append(units, n.units...)
		full := g.pool.internUnits(units)
		v, err := g.table.resolve(full, false)
		if err != nil {
			return 0, nil, nil, nil, nil, err
		}
		if v == nil {
			failure, err := g.unresolvedQualifiedSelector(prefix, last, index, depth)
			return 0, nil, nil, nil, failure, err
		}
		if theorem, ok := v.(*thmOrAssumpDefNode); ok {
			return index, theorem, names, args, nil, nil
		}
		op, ok := v.(*userOpDefNode)
		if !ok {
			return 0, nil, nil, nil, nil, g.unported("generator.construct.qualified-selector-prefix")
		}
		if op.kind != 6 {
			return index, op, names, args, nil, nil
		}
		names = append(names, n)
		if operatorMode {
			found = op.arity
			continue
		}
		var trees []*generated.SyntaxTreeNode
		location := tree
		if len(parts) == 3 {
			if parts[1].Kind() != generated.N_OpArgs {
				return 0, nil, nil, nil, nil, g.unported("generator.construct.qualified-selector-prefix")
			}
			location = parts[1]
			ps := parts[1].Heirs()
			for i := 1; i < len(ps)-1; i += 2 {
				trees = append(trees, ps[i])
			}
		}
		if found+len(trees) != op.arity {
			failure, err := g.badArity(full, op.arity-found, location)
			return 0, nil, nil, nil, failure, err
		}
		for i, t := range trees {
			pos := found + i
			if pos >= len(op.params) {
				return 0, nil, nil, nil, nil, g.unported("generator.construct.qualified-selector-prefix")
			}
			a, err := g.expressionOrOperatorArgument(t, op.params[pos].arity, pos, op, parts[0], depth+1)
			if err != nil {
				return 0, nil, nil, nil, nil, err
			}
			args = append(args, a)
		}
		found = op.arity
	}
	return len(prefix), nil, names, args, nil, nil
}

// directLetOperatorArgumentHistory covers F!:!G when F is a known operator
// whose body is a LET, possibly beneath known instance substitution wrappers.
// The source clones G's formals when a prefix or outer formal is present.
func (g *generator) directLetOperatorArgumentHistory(children []*generated.SyntaxTreeNode) bool {
	if !g.pool.history.exact || len(children) != 2 || len(children[0].Heirs()) != 2 || children[1].Kind() != 289 || children[1].Image() == nil {
		return false
	}
	segments := children[0].Heirs()
	first, second := segments[0].Heirs(), segments[1].Heirs()
	if len(first) != 2 || first[0].Kind() != 289 || first[0].Image() == nil || len(second) != 2 || second[0].Kind() != generated.N_StructOp || second[0].Image() == nil || second[0].Image().String() != ":" {
		return false
	}
	n := g.pool.lookupUnits(first[0].Image().Units())
	if n == nil {
		return false
	}
	v, err := g.table.resolve(n, false)
	root, ok := v.(*userOpDefNode)
	if err != nil || !ok || root.kind != 5 || !root.defined || !g.pool.history.reuse(root).known {
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
	let, ok := body.(*letInNode)
	if !ok || let.context == nil || !g.pool.history.reuse(let).known {
		return false
	}
	n = g.pool.lookupUnits(children[1].Image().Units())
	if n == nil {
		return false
	}
	local, ok := let.context.lookup(contextKey{name: n}).(*userOpDefNode)
	if !ok || !local.defined || len(local.params) != local.arity || !g.pool.history.reuse(local).known {
		return false
	}
	for _, p := range root.params {
		if !g.pool.history.reuse(p).known {
			return false
		}
	}
	for _, p := range local.params {
		if !g.pool.history.reuse(p).known {
			return false
		}
	}
	return true
}

// qualifiedOperatorArgumentHistory covers one known instance marker followed
// by a known operator, without supplied selector arguments. selectorToNode
// resolves the combined symbol and either returns it in a new OpArg or returns
// the recovery node before any constructor when the arity is wrong.
func (g *generator) qualifiedOperatorArgumentHistory(children []*generated.SyntaxTreeNode) bool {
	if !g.pool.history.exact || len(children) != 2 || len(children[0].Heirs()) != 1 || children[1].Image() == nil || children[1].Kind() != 289 {
		return false
	}
	parts := children[0].Heirs()[0].Heirs()
	if len(parts) != 2 || parts[0].Kind() != 289 || parts[0].Image() == nil {
		return false
	}
	name := g.pool.lookupUnits(parts[0].Image().Units())
	if name == nil {
		return false
	}
	v, err := g.table.resolve(name, false)
	marker, ok := v.(*userOpDefNode)
	if err != nil || !ok || marker.kind != 6 || !g.pool.history.reuse(marker).known {
		return false
	}
	units := append(append(append([]uint16{}, name.units...), '!'), children[1].Image().Units()...)
	full := g.pool.lookupUnits(units)
	if full == nil {
		return false
	}
	v, err = g.table.resolve(full, false)
	op, ok := v.(*userOpDefNode)
	return err == nil && ok && op.kind == 5 && g.pool.history.reuse(op).known
}

// simpleInstanceSelectorHistory admits only a single zero-arity instance marker
// followed by its known zero-arity operator. No selector wrapper is reached.
func (g *generator) simpleInstanceSelectorHistory(children []*generated.SyntaxTreeNode) bool {
	if !g.pool.history.exact {
		return false
	}
	if len(children) != 2 || len(children[0].Heirs()) != 1 || children[1].Image() == nil {
		return false
	}
	parts := children[0].Heirs()[0].Heirs()
	if len(parts) != 2 || parts[0].Image() == nil || parts[0].Kind() != 289 {
		return false
	}
	n := g.pool.lookupUnits(parts[0].Image().Units())
	if n == nil {
		return false
	}
	v, err := g.table.resolve(n, false)
	if err != nil {
		return false
	}
	marker, ok := v.(*userOpDefNode)
	if !ok || marker.kind != 6 || marker.arity != 0 || !g.pool.history.reuse(marker).known {
		return false
	}
	full := g.pool.lookupUnits(append(append(append([]uint16{}, n.units...), uint16("!"[0])), children[1].Image().Units()...))
	if full == nil {
		return false
	}
	v, err = g.table.resolve(full, false)
	if err != nil {
		return false
	}
	op, ok := v.(*userOpDefNode)
	return ok && op.kind == 5 && op.arity == 0 && g.pool.history.reuse(op).known
}

func (g *generator) qualifiedInstance(tree *generated.SyntaxTreeNode, children []*generated.SyntaxTreeNode, finalArgs *generated.SyntaxTreeNode, depth int) (exprNode, error) {
	prefix := children[0].Heirs()
	var units []uint16
	var args []exprOrOpArg
	found := 0
	for _, element := range prefix {
		parts := element.Heirs()
		if len(parts) < 2 || len(parts) > 3 || parts[0].Kind() != 289 {
			return nil, g.unported("generator.construct.qualified-instance")
		}
		n, e := g.name(parts[0])
		if e != nil {
			return nil, e
		}
		if len(units) > 0 {
			units = append(units, uint16('!'))
		}
		units = append(units, n.units...)
		full := g.pool.internUnits(units)
		v, e := g.table.resolve(full, false)
		if e != nil {
			return nil, e
		}
		marker, ok := v.(*userOpDefNode)
		if !ok || marker.kind != 6 {
			return nil, g.unported("generator.construct.qualified-instance")
		}
		var argTrees []*generated.SyntaxTreeNode
		location := tree
		if len(parts) == 3 {
			location = parts[1]
			ps := parts[1].Heirs()
			for i := 1; i < len(ps)-1; i += 2 {
				argTrees = append(argTrees, ps[i])
			}
		}
		if found+len(argTrees) != marker.arity {
			return g.badArity(full, marker.arity-found, location)
		}
		for i, t := range argTrees {
			pos := found + i
			if pos >= len(marker.params) {
				return nil, g.unported("generator.construct.qualified-instance")
			}
			a, e := g.expressionOrOperatorArgument(t, marker.params[pos].arity, pos, marker, parts[0], depth+1)
			if e != nil {
				return nil, e
			}
			args = append(args, a)
		}
		found = marker.arity
	}
	if children[1].Kind() == generated.N_StructOp || children[1].Kind() == generated.N_OpArgs {
		// FindingOpName after the instance markers: a selector that is not
		// a NameSel leaves the compound name unchanged (newName is null),
		// nothing resolves, and the loop ends with SYMBOL_UNDEFINED for the
		// markers' compound name at curNameLoc, which spans their name
		// selectors only (Generator.java:732-813, 846). Arguments on the
		// final selector are rejected first (Generator.java:704-711).
		if finalArgs != nil {
			return nil, g.unported("generator.construct.qualified-instance")
		}
		return g.unknownMarkerCompound(prefix, g.pool.internUnits(units))
	}
	last, e := g.name(children[1])
	if e != nil {
		return nil, e
	}
	canonical, e := g.pool.syntax.ResolveSynonym(g.pool.syntax.UniqueStringOfUnits(last.units))
	if e != nil {
		return nil, g.unported("generator.precondition.synonym-lookup")
	}
	units = append(units, uint16('!'))
	units = append(units, canonical.Units()...)
	full := g.pool.internUnits(units)
	v, e := g.table.resolve(full, false)
	if e != nil {
		return nil, e
	}
	if v == nil {
		first := prefix[0].Heirs()
		if len(first) == 0 {
			return nil, g.unported("generator.construct.qualified-instance")
		}
		loc, e := g.diagnosticLocation(first[0])
		if e != nil {
			return nil, e
		}
		end, e := g.diagnosticLocation(children[1])
		if e != nil {
			return nil, e
		}
		loc.EndLine = end.EndLine
		loc.EndColumn = end.EndColumn
		if e = addError(g.table.errors, 4200, loc, "Unknown operator: `%s'.", nameParam(full)); e != nil {
			return nil, e
		}
		return g.nullOAN, nil
	}
	return g.qualifiedTarget(tree, children[1], finalArgs, full, v, found, args, nil, depth)
}

// unknownMarkerCompound reports "Unknown operator" for the compound name of
// a prefix of instance markers, located from the first marker's name to the
// last one's (Generator.java:808-813).
func (g *generator) unknownMarkerCompound(prefix []*generated.SyntaxTreeNode, compound *name) (exprNode, error) {
	if len(prefix) == 0 {
		return nil, g.unported("generator.construct.qualified-instance")
	}
	first, last := prefix[0].Heirs(), prefix[len(prefix)-1].Heirs()
	if len(first) == 0 || len(last) == 0 {
		return nil, g.unported("generator.construct.qualified-instance")
	}
	if !compound.displayExact {
		return nil, g.unported("generator.mapping.inexact-display-name")
	}
	loc, e := g.diagnosticLocation(first[0])
	if e != nil {
		return nil, e
	}
	end, e := g.diagnosticLocation(last[0])
	if e != nil {
		return nil, e
	}
	loc.EndLine, loc.EndColumn = end.EndLine, end.EndColumn
	if e = addError(g.table.errors, 4200, loc, "Unknown operator: `%s'.", nameParam(compound)); e != nil {
		return nil, e
	}
	return g.nullOAN, nil
}

// qualifiedTarget applies the definition or theorem that a compound name
// resolved to at its final selector (Generator.java:870-946, 1897-1916).
// found counts the instance arguments already generated in args; leading
// holds tempArgs syntax trees from undefined segments before the final
// selector's own arguments. The arity error is located at the final
// arguments, or at the selector when there are none.
func (g *generator) qualifiedTarget(tree, last, finalArgs *generated.SyntaxTreeNode, full *name, v leafNode, found int, args []exprOrOpArg, leading []*generated.SyntaxTreeNode, depth int) (exprNode, error) {
	theorem, isTheorem := v.(*thmOrAssumpDefNode)
	op, ok := v.(*userOpDefNode)
	argTrees := append([]*generated.SyntaxTreeNode{}, leading...)
	location := tree
	if finalArgs != nil {
		location = finalArgs
		ps := finalArgs.Heirs()
		for i := 1; i < len(ps)-1; i += 2 {
			argTrees = append(argTrees, ps[i])
		}
	}
	if isTheorem {
		if found+len(argTrees) != len(theorem.params) {
			return g.badArity(full, len(theorem.params)-found, location)
		}
		for i, t := range argTrees {
			pos := found + i
			a, err := g.expressionOrOperatorArgument(t, theorem.params[pos].arity, pos, theorem, last, depth+1)
			if err != nil {
				return nil, err
			}
			args = append(args, a)
		}
		if rejected, err := g.rejectAPExpression(theorem, tree); rejected || err != nil {
			if err != nil {
				return nil, err
			}
			return g.nullOAN, nil
		}
		if args == nil {
			args = make([]exprOrOpArg, 0)
		}
		ticket := g.pool.history.event("OpApplNode", 9)
		result, err := newOpAppl(g.pool, theorem, args, tree)
		if result != nil {
			g.pool.history.attach(result, ticket)
		}
		if err != nil {
			return nil, err
		}
		result.subExpressionOf = theorem
		return result, nil
	}
	if !ok {
		return nil, g.unported("generator.construct.qualified-instance")
	}
	if op.kind != 5 && op.kind != 6 {
		return nil, g.unported("generator.construct.qualified-instance")
	}
	if found+len(argTrees) != op.arity {
		return g.badArity(full, op.arity-found, location)
	}
	for i, t := range argTrees {
		pos := found + i
		if pos >= len(op.params) {
			return nil, g.unported("generator.construct.qualified-instance")
		}
		a, e := g.expressionOrOperatorArgument(t, op.params[pos].arity, pos, op, last, depth+1)
		if e != nil {
			return nil, e
		}
		args = append(args, a)
	}
	if op.kind == 6 {
		loc, e := g.diagnosticLocation(last)
		if e != nil {
			return nil, e
		}
		if e = addError(g.table.errors, 4203, loc, "Operator name %s is incomplete.", nameStringParam(full)); e != nil {
			return nil, e
		}
		return g.nullOAN, nil
	}
	if args == nil {
		args = make([]exprOrOpArg, 0)
	}
	// The Java OpApplNode constructor calls OpDefNode.match after all
	// arguments have been generated, including for instance-qualified calls.
	return g.applySelectedOperator(tree, op, op, args)
}
