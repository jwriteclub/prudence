// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

// This request-owned test projection is an inventory of modeled fields, not a
// claim that they correspond to the conformance oracle's canonical schema.
import (
	stdcontext "context"
	"encoding/json"
	"sort"
	"testing"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

type projectedGraph struct {
	Root        uint64            `json:"root"`
	Nodes       []projectedNode   `json:"nodes"`
	Syntax      []projectedSyntax `json:"syntax"`
	Diagnostics []Detail          `json:"diagnostics"`
	Unavailable []string          `json:"unavailable"`
	Truncated   bool              `json:"truncated"`
}
type projectedNode struct {
	ID     uint64         `json:"id"`
	Class  string         `json:"class"`
	Kind   int            `json:"kind"`
	Tree   int            `json:"tree"`
	Fields map[string]any `json:"fields"`
}
type projectedSyntax struct {
	ID                             int `json:"id"`
	Kind                           int `json:"kind"`
	Image, OriginalImage, Filename []uint16
	Position                       [4]int32
	Zero, One                      []int
	Comments                       [][]uint16
}
type graphProjector struct {
	nodes              map[uint64]any
	queue              []any
	trees              map[*generated.SyntaxTreeNode]int
	treeQueue          []*generated.SyntaxTreeNode
	maxNodes, maxTrees int
	exceeded           bool
}

func units(u *generated.UniqueString) []uint16 {
	if u == nil {
		return nil
	}
	out := u.Units()
	if out == nil {
		return make([]uint16, 0)
	}
	return out
}
func (p *graphProjector) tree(n *generated.SyntaxTreeNode) int {
	if n == nil {
		return 0
	}
	if id := p.trees[n]; id != 0 {
		return id
	}
	if len(p.treeQueue) >= p.maxTrees {
		p.exceeded = true
		return 0
	}
	id := len(p.treeQueue) + 1
	p.trees[n] = id
	p.treeQueue = append(p.treeQueue, n)
	return id
}
func semanticHeaderOf(n any) *semanticHeader {
	switch v := n.(type) {
	case leafNode:
		return &v.header().semanticHeader
	case exprOrOpArg:
		return v.argumentHeader()
	case levelNode:
		return v.levelHeader()
	}
	return nil
}
func (p *graphProjector) ref(n any) uint64 {
	if n == nil {
		return 0
	}
	h := semanticHeaderOf(n)
	if h == nil {
		return 0
	}
	if _, ok := p.nodes[h.identity]; !ok {
		if len(p.queue) >= p.maxNodes {
			p.exceeded = true
			return 0
		}
		p.nodes[h.identity] = n
		p.queue = append(p.queue, n)
	}
	return h.identity
}
func (p *graphProjector) refs(n []exprNode) []uint64 {
	if n == nil {
		return nil
	}
	r := make([]uint64, len(n))
	for i, v := range n {
		if v != nil {
			r[i] = p.ref(v)
		}
	}
	return r
}
func (p *graphProjector) argRefs(n []exprOrOpArg) []uint64 {
	if n == nil {
		return nil
	}
	r := make([]uint64, len(n))
	for i, v := range n {
		if v != nil {
			r[i] = p.ref(v)
		}
	}
	return r
}
func (p *graphProjector) params(n []*formalParamNode) []uint64 {
	if n == nil {
		return nil
	}
	r := make([]uint64, len(n))
	for i, v := range n {
		if v != nil {
			r[i] = p.ref(v)
		}
	}
	return r
}
func (p *graphProjector) level(l leafLevel) map[string]any {
	collect := func(s map[leafNode]struct{}) []uint64 {
		if s == nil {
			return nil
		}
		ordered := make([]leafNode, 0, len(s))
		for n := range s {
			ordered = append(ordered, n)
		}
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].header().identity < ordered[j].header().identity })
		r := make([]uint64, 0, len(s))
		for _, n := range ordered {
			r = append(r, p.ref(n))
		}
		return r
	}
	return map[string]any{"level": l.level, "checked": l.checked, "levelParams": collect(l.levelParams), "allParams": collect(l.allParams)}
}
func (p *graphProjector) node(n any) projectedNode {
	h := semanticHeaderOf(n)
	r := projectedNode{ID: h.identity, Kind: h.kind, Tree: p.tree(h.tree), Fields: make(map[string]any)}
	f := r.Fields
	if s, ok := n.(leafNode); ok {
		if s.header().name != nil {
			f["nameUnits"] = append([]uint16{}, s.header().name.units...)
		} else {
			f["nameUnits"] = nil
		}
	}
	if e, ok := n.(exprNode); ok {
		if e.expression().subExpressionOf == nil {
			f["subExpressionOf"] = uint64(0)
		} else {
			f["subExpressionOf"] = p.ref(e.expression().subExpressionOf)
		}
	}
	switch v := n.(type) {
	case *moduleNode:
		r.Class = "ModuleNode"
		f["definitions"] = p.leafRefs(v.definitions)
		f["records"] = p.apps(v.records)
		f["instantiated"] = v.instantiated
		refs := make([]uint64, len(v.instances))
		for i, n := range v.instances {
			refs[i] = p.ref(n)
		}
		f["instances"] = refs
		top := make([]uint64, len(v.topLevel))
		for i, n := range v.topLevel {
			top[i] = p.ref(n)
		}
		f["topLevel"] = top
		f["recursiveDecls"] = p.userDefs(v.recursiveDecls)
		f["recursiveSectionDefs"] = p.userDefs(v.recursiveSectionDefs)
		var history []uint64
		for x := v.context.last; x != nil; x = x.prior {
			history = append(history, p.ref(x.value))
		}
		f["contextHistoryNewestFirst"] = history
	case *assumeNode:
		r.Class = "AssumeNode"
		f["body"] = p.ref(v.expression)
		f["axiom"] = v.axiom
		if v.definition != nil {
			f["definition"] = p.ref(v.definition)
		}
	case *theoremNode:
		r.Class = "TheoremNode"
		f["body"] = p.ref(v.body)
		f["suffices"] = v.suffices
		if v.proof != nil {
			f["proof"] = p.ref(v.proof)
		}
		if v.definition != nil {
			f["definition"] = p.ref(v.definition)
		}
	case *thmOrAssumpDefNode:
		r.Class = "ThmOrAssumpDefNode"
		f["theorem"] = v.theorem
		f["suffices"] = v.suffices
		f["body"] = p.ref(v.body)
		f["params"] = p.params(v.params)
		f["local"] = v.local
		f["labelsTableOrder"] = p.labelRefs(v.labels)
		f["module"] = p.refModule(v.module)
		f["source"] = p.ref(v.sourceNode())
		f["instantiatedFrom"] = p.refModule(v.instantiatedFrom)
		if v.statement != nil {
			f["statement"] = p.ref(v.statement)
		}
	case *assumeProveNode:
		r.Class = "AssumeProveNode"
		assumes := make([]uint64, len(v.assumptions))
		for i, n := range v.assumptions {
			assumes[i] = p.ref(n)
		}
		f["assumptions"] = assumes
		f["prove"] = p.ref(v.prove)
		f["declarations"] = v.declarations
		f["boxed"] = v.boxed
		f["inProof"] = v.inProof
		if v.goal != nil {
			f["goal"] = p.ref(v.goal)
		}
	case *newSymbNode:
		r.Class = "NewSymbNode"
		f["declaration"] = p.ref(v.declaration)
		if v.set != nil {
			f["set"] = p.ref(v.set)
		}
	case *leafProofNode:
		r.Class = "LeafProofNode"
		f["omitted"] = v.omitted
		f["obvious"] = v.obvious
		f["facts"] = p.levelRefs(v.facts)
		f["defs"] = p.leafRefs(v.defs)
		f["only"] = v.only
	case *nonLeafProofNode:
		r.Class = "NonLeafProofNode"
		f["steps"] = p.levelRefs(v.steps)
		instances := make([]uint64, len(v.insts))
		for i, instance := range v.insts {
			instances[i] = p.ref(instance)
		}
		f["instances"] = instances
		var bindings []uint64
		if v.context != nil {
			for _, symbol := range v.context.orderedValues() {
				bindings = append(bindings, p.ref(symbol))
			}
		}
		f["context"] = bindings
	case *useOrHideNode:
		r.Class = "UseOrHideNode"
		f["facts"] = p.levelRefs(v.facts)
		f["defs"] = p.leafRefs(v.defs)
		f["only"], f["hide"] = v.only, v.hide
		if v.stepName != nil {
			f["stepNameUnits"] = append([]uint16{}, v.stepName.units...)
		}
	case *defStepNode:
		r.Class = "DefStepNode"
		f["defs"] = p.definitionRefs(v.defs)
		if v.stepNumber != nil {
			f["stepNumberUnits"] = append([]uint16{}, v.stepNumber.units...)
		}
	case *apSubstInNode:
		r.Class = "APSubstInNode"
		f["substs"] = p.substitutions(v.substs)
		f["body"] = p.ref(v.body)
		f["instantiatingModule"] = p.refModule(v.instantiatingModule)
		f["instantiatedModule"] = p.refModule(v.instantiatedModule)
	case *formalParamNode:
		r.Class = "FormalParamNode"
		f["arity"] = v.arity
		f["module"] = p.refModule(v.module)
		f["level"] = p.level(v.levels)
	case *opDeclNode:
		r.Class = "OpDeclNode"
		f["arity"] = v.arity
		f["module"] = p.refModule(v.module)
		f["level"] = p.level(v.levels)
	case *builtinOpDefNode:
		r.Class = "OpDefNode"
		f["labelsTableOrder"] = p.labelRefs(v.labels)
		f["arity"] = v.arity
		f["params"] = p.params(v.params)
		f["defined"] = v.defined
		f["source"] = v.identity
		f["module"] = uint64(0)
		f["maxLevels"] = v.maxLevels
		f["weights"] = v.weights
		f["isLeibniz"] = v.isLeibniz
		f["isLeibnizArg"] = v.isLeibnizArg
		f["level"] = p.level(v.levels)
	case *userOpDefNode:
		r.Class = "OpDefNode"
		f["arity"] = v.arity
		f["params"] = p.params(v.params)
		f["local"] = v.local
		f["labelsTableOrder"] = p.labelRefs(v.labels)
		f["letInLevel"] = v.letInLevel
		f["inRecursive"] = v.inRecursive
		f["inRecursiveSection"] = v.inRecursiveSection
		f["recursiveSection"] = v.recursiveSection
		f["defined"] = v.defined
		if v.body != nil {
			f["body"] = p.ref(v.body)
		} else {
			f["body"] = uint64(0)
		}
		f["module"] = p.refModule(v.module)
		f["source"] = p.ref(v.sourceNode())
		f["stepNode"] = p.ref(v.stepNode)
		var compound [][]uint16
		for _, n := range v.compoundID {
			compound = append(compound, append([]uint16{}, n.units...))
		}
		f["compoundID"] = compound
		f["maxLevels"] = v.maxLevels
		f["weights"] = v.weights
		f["isLeibniz"] = v.isLeibniz
		f["isLeibnizArg"] = v.isLeibnizArg
		f["level"] = p.level(v.levels)
	case *instanceNode:
		r.Class = "InstanceNode"
		if v.name != nil {
			f["nameUnits"] = append([]uint16{}, v.name.units...)
		}
		f["local"] = v.local
		if v.stepName != nil {
			f["stepNameUnits"] = append([]uint16{}, v.stepName.units...)
		}
		f["params"] = p.params(v.params)
		f["module"] = p.refModule(v.module)
		f["substs"] = p.substitutions(v.substs)
	case *substInNode:
		r.Class = "SubstInNode"
		f["substs"] = p.substitutions(v.substs)
		f["body"] = p.ref(v.body)
		f["instantiatingModule"] = p.refModule(v.instantiatingModule)
		f["instantiatedModule"] = p.refModule(v.instantiatedModule)
	case *numeralNode:
		r.Class = "NumeralNode"
		f["image"] = v.image
		f["value"] = v.value
		if v.bigValue != nil {
			f["bigValue"] = v.bigValue.String()
		} else {
			f["bigValue"] = nil
		}
		f["level"] = p.level(v.levels)
	case *decimalNode:
		r.Class = "DecimalNode"
		f["integral"] = v.integral
		f["fractional"] = v.fractional
		f["mantissa"] = v.mantissa
		f["exponent"] = v.exponent
		f["scale"] = v.scale
		if v.bigValue != nil {
			f["bigValue"] = v.bigValue.String()
		} else {
			f["bigValue"] = nil
		}
		f["level"] = p.level(v.levels)
	case *stringNode:
		r.Class = "StringNode"
		f["valueUnits"] = append([]uint16{}, v.value.units...)
		f["level"] = p.level(v.levels)
	case *labelNode:
		r.Class = "LabelNode"
		f["nameUnits"] = append([]uint16{}, v.name.units...)
		f["params"] = p.params(v.params)
		if v.body != nil {
			f["body"] = p.ref(v.body)
		} else {
			f["body"] = uint64(0)
		}
		f["labelsTableOrder"] = p.labelRefs(v.labels)
		f["isAssumeProve"] = v.isAssumeProve
		f["goalClause"] = v.goalClause
		f["level"] = p.level(v.levels)
	case *atNode:
		r.Class = "AtNode"
		f["except"] = p.ref(v.except)
		f["component"] = p.ref(v.component)
		f["level"] = p.level(v.levels)
	case *opArgNode:
		r.Class = "OpArgNode"
		f["operator"] = p.ref(v.operator)
		f["nameUnits"] = append([]uint16{}, v.name.units...)
		f["arity"] = v.arity
		f["module"] = p.refModule(v.module)
		f["level"] = p.level(v.levels)
	case *letInNode:
		r.Class = "LetInNode"
		var definitions []uint64
		if v.definitions != nil {
			definitions = make([]uint64, len(v.definitions))
			for i, d := range v.definitions {
				definitions[i] = p.ref(d)
			}
		}
		f["definitions"] = definitions
		instances := make([]uint64, len(v.instances))
		for i, n := range v.instances {
			instances[i] = p.ref(n)
		}
		f["instances"] = instances
		if v.body != nil {
			f["body"] = p.ref(v.body)
		} else {
			f["body"] = uint64(0)
		}
		history := make([]uint64, 0)
		for x := v.context.last; x != nil; x = x.prior {
			history = append(history, p.ref(x.value))
		}
		f["contextHistoryNewestFirst"] = history
		f["level"] = p.level(v.levels)
	case *opApplNode:
		r.Class = "OpApplNode"
		f["operator"] = p.ref(v.operator)
		f["operands"] = p.argRefs(v.operands)
		f["unbounded"] = p.params(v.unbounded)
		var bounds [][]uint64
		if v.bounded != nil {
			bounds = make([][]uint64, len(v.bounded))
			for i, x := range v.bounded {
				bounds[i] = p.params(x)
			}
		}
		f["bounded"] = bounds
		f["tupleBounds"] = v.tupleBounds
		f["ranges"] = p.refs(v.ranges)
		f["level"] = p.level(v.levels)
	}
	return r
}
func (p *graphProjector) labelRefs(labels *labelSet) []uint64 {
	if labels == nil {
		return nil
	}
	nodes := labels.ordered()
	refs := make([]uint64, 0, len(nodes))
	for _, n := range nodes {
		refs = append(refs, p.ref(n))
	}
	return refs
}
func (p *graphProjector) substitutions(ss []*subst) []map[string]any {
	out := make([]map[string]any, 0, len(ss))
	for _, s := range ss {
		out = append(out, map[string]any{"op": p.ref(s.op), "expr": p.ref(s.expr), "exprTree": p.tree(s.exprTree), "implicit": s.implicit})
	}
	return out
}
func (p *graphProjector) refModule(n *moduleNode) uint64 {
	if n == nil {
		return 0
	}
	return p.ref(n)
}
func (p *graphProjector) levelRefs(n []levelNode) []uint64 {
	if n == nil {
		return nil
	}
	refs := make([]uint64, len(n))
	for i, value := range n {
		refs[i] = p.ref(value)
	}
	return refs
}
func (p *graphProjector) leafRefs(n []leafNode) []uint64 {
	if n == nil {
		return nil
	}
	a := make([]uint64, len(n))
	for i, v := range n {
		a[i] = p.ref(v)
	}
	return a
}
func (p *graphProjector) definitionRefs(n []definitionNode) []uint64 {
	if n == nil {
		return nil
	}
	a := make([]uint64, len(n))
	for i, v := range n {
		a[i] = p.ref(v)
	}
	return a
}
func (p *graphProjector) userDefs(n []*userOpDefNode) []uint64 {
	if n == nil {
		return nil
	}
	a := make([]uint64, len(n))
	for i, v := range n {
		a[i] = p.ref(v)
	}
	return a
}
func (p *graphProjector) apps(n []*opApplNode) []uint64 {
	if n == nil {
		return nil
	}
	a := make([]uint64, len(n))
	for i, v := range n {
		a[i] = p.ref(v)
	}
	return a
}
func projectGraph(g *generator, m *moduleNode) projectedGraph {
	return projectGraphLimited(g, m, 10000, 100000)
}
func projectGraphLimited(g *generator, m *moduleNode, maxNodes, maxTrees int) projectedGraph {
	p := &graphProjector{nodes: make(map[uint64]any), trees: make(map[*generated.SyntaxTreeNode]int), maxNodes: maxNodes, maxTrees: maxTrees}
	out := projectedGraph{Root: p.refModule(m), Diagnostics: g.table.errors.Messages(), Unavailable: []string{"complete-level-information", "complete-node-fields", "oracle-schema-mapping", "transport-byte-and-time-bounds", "rendered-diagnostics", "label-hashtable-enumeration"}}
	for i := 0; i < len(p.queue); i++ {
		out.Nodes = append(out.Nodes, p.node(p.queue[i]))
	}
	for i := 0; i < len(p.treeQueue); i++ {
		n := p.treeQueue[i]
		l := n.Location()
		v := projectedSyntax{ID: i + 1, Kind: n.Kind(), Image: units(n.Image()), OriginalImage: units(n.OriginalImage()), Filename: units(n.Filename()), Position: [4]int32{l.BeginLine(), l.BeginColumn(), l.EndLine(), l.EndColumn()}}
		zero := n.Zero()
		if zero != nil {
			v.Zero = make([]int, 0, len(zero))
		}
		for _, x := range zero {
			v.Zero = append(v.Zero, p.tree(x))
		}
		one := n.One()
		if one != nil {
			v.One = make([]int, 0, len(one))
		}
		for _, x := range one {
			v.One = append(v.One, p.tree(x))
		}
		comments := n.PreComments()
		if comments != nil {
			v.Comments = make([][]uint16, 0, len(comments))
		}
		for _, x := range comments {
			v.Comments = append(v.Comments, units(x))
		}
		out.Syntax = append(out.Syntax, v)
	}
	out.Truncated = p.exceeded
	if p.exceeded {
		out.Unavailable = append(out.Unavailable, "node-or-syntax-limit")
	}
	return out
}
func TestProjectionGraphReferences(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(x) == x\nA == CASE TRUE -> F(1) [] OTHER -> 2\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	out := projectGraph(g, m)
	data, e := json.Marshal(out)
	if e != nil || len(data) == 0 || out.Root == 0 || len(out.Nodes) == 0 || len(out.Syntax) == 0 || len(out.Unavailable) == 0 {
		t.Fatal("projection", e)
	}
}

func TestProjectionLimitsAndSharedReferences(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(x) == x\nA == F(1)\nB == Missing\nC == MissingToo\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	out := projectGraph(g, m)
	if out.Truncated {
		t.Fatal("unexpected truncation")
	}
	var module, def, app *projectedNode
	for i := range out.Nodes {
		n := &out.Nodes[i]
		if n.ID == m.identity {
			module = n
		}
		if n.ID == testUserDefinitions(t, m.definitions)[0].identity {
			def = n
		}
		if n.ID == g.nullOAN.identity {
			app = n
		}
	}
	if module == nil || def == nil || def.Fields["module"] != m.identity || app == nil {
		t.Fatal("shared or cyclic reference lost")
	}
	if testUserDefinitions(t, m.definitions)[2].body != g.nullOAN || testUserDefinitions(t, m.definitions)[3].body != g.nullOAN {
		t.Fatal("recovery identity")
	}
	limited := projectGraphLimited(g, m, 1, 1)
	if !limited.Truncated || len(limited.Unavailable) == 0 {
		t.Fatal("limit not explicit")
	}
}
func TestProjectionUTF16AndEmpty(t *testing.T) {
	p := newNamePool()
	u := p.syntax.UniqueStringOfUnits([]uint16{0xd800, 'a'})
	got := units(u)
	if len(got) != 2 || got[0] != 0xd800 || got[1] != 'a' {
		t.Fatal(got)
	}
	empty := units(p.syntax.UniqueStringOf(""))
	if empty == nil || len(empty) != 0 || units(nil) != nil {
		t.Fatal("empty and null collapsed")
	}
}

func TestProjectionOperatorArgument(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(Op(_)) == Op(1)\nG(x) == x\nA == F(G)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	app := testUserDefinitions(t, m.definitions)[2].body.(*opApplNode)
	arg := app.operands[0].(*opArgNode)
	out := projectGraph(g, m)
	for _, n := range out.Nodes {
		if n.ID == arg.identity {
			if n.Class != "OpArgNode" || n.Fields["operator"] != testUserDefinitions(t, m.definitions)[1].identity {
				t.Fatal("operator edge")
			}
			return
		}
	}
	t.Fatal("operator argument missing")
}
