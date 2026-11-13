// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// The graph follows TLA+ semantic node classes, Copyright 2003 Compaq
// Corporation and Microsoft Corporation, under the MIT license.
package semantic

import (
	stdcontext "context"
	"fmt"
	"reflect"
	"sort"
	"strconv"

	parser "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/modulegraph"
)

type SourceSpan struct {
	Source string `json:"source"`
	Start  [2]int `json:"start"`
	End    [2]int `json:"end"`
}
type Binder struct {
	Params []uint64 `json:"params"`
	Domain uint64   `json:"domain,omitempty"`
	Tuple  bool     `json:"tuple"`
}
type Substitution struct {
	Target   uint64      `json:"target"`
	Value    uint64      `json:"value"`
	Implicit bool        `json:"implicit"`
	Span     *SourceSpan `json:"span,omitempty"`
}
type ResolvedNode struct {
	ID               uint64            `json:"id"`
	Kind             string            `json:"kind"`
	Name             string            `json:"name,omitempty"`
	Level            *int              `json:"level,omitempty"`
	Span             *SourceSpan       `json:"span,omitempty"`
	Standard         bool              `json:"standard,omitempty"`
	Symbols          map[string]uint64 `json:"symbols,omitempty"`
	Modules          []uint64          `json:"nestedModules,omitempty"`
	Extends          []uint64          `json:"extends,omitempty"`
	Variables        []uint64          `json:"variables,omitempty"`
	Constants        []uint64          `json:"constants,omitempty"`
	Definitions      []uint64          `json:"definitions,omitempty"`
	Instances        []uint64          `json:"instances,omitempty"`
	Assumptions      []uint64          `json:"assumptions,omitempty"`
	Statements       []uint64          `json:"statements,omitempty"`
	Module           uint64            `json:"module,omitempty"`
	Arity            *int              `json:"arity,omitempty"`
	Body             uint64            `json:"body,omitempty"`
	SourceDefinition uint64            `json:"sourceDefinition,omitempty"`
	Local            bool              `json:"local,omitempty"`
	Recursive        bool              `json:"recursive,omitempty"`
	Params           []uint64          `json:"params,omitempty"`
	Value            *string           `json:"value,omitempty"`
	Operator         uint64            `json:"operator,omitempty"`
	Args             []uint64          `json:"args,omitempty"`
	Binders          []Binder          `json:"binders,omitempty"`
	Except           uint64            `json:"except,omitempty"`
	Component        uint64            `json:"component,omitempty"`
	FromModule       uint64            `json:"fromModule,omitempty"`
	ToModule         uint64            `json:"toModule,omitempty"`
	Substitutions    []Substitution    `json:"substitutions,omitempty"`
	Labels           []uint64          `json:"labels,omitempty"`
}
type ResolvedGraph struct {
	Root    uint64         `json:"root"`
	Modules []uint64       `json:"modules"`
	Nodes   []ResolvedNode `json:"nodes"`
}

// ExportResolved materializes IDs from the same request-owned semantic objects
// and level checker as CheckFlatExternal. A stopped check yields no graph.
func ExportResolved(ctx stdcontext.Context, a *modulegraph.Acquisition, maxDepth int) (*ResolvedGraph, error) {
	var g checkedGraph
	out := checkFlatExternal(ctx, a, maxDepth, true, &g)
	if out.Err != nil {
		return nil, fmt.Errorf("%s: %w", out.Site, out.Err)
	}
	for _, d := range out.Details {
		if d.Code.Severity == Error {
			msg, e := d.Render()
			if e != nil {
				return nil, e
			}
			return nil, fmt.Errorf("%s:%d:%d: %s", d.Location.Name, d.Location.BeginLine, d.Location.BeginColumn, msg)
		}
	}
	if !out.Levels || out.Aborted || g.root == nil {
		return nil, fmt.Errorf("incomplete semantic or level check")
	}
	p := resolvedProjector{seen: make(map[levelNode]bool), checker: g.checker}
	graph := &ResolvedGraph{Root: p.ref(g.root), Modules: resolvedRefs(&p, g.modules)}
	for i := 0; i < len(p.queue); i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, err := p.project(p.queue[i])
		if err != nil {
			return nil, err
		}
		graph.Nodes = append(graph.Nodes, n)
	}
	sort.Slice(graph.Nodes, func(i, j int) bool { return graph.Nodes[i].ID < graph.Nodes[j].ID })
	return graph, nil
}

// Semantic headers allocate monotonically in one name pool spanning the
// closure. Queueing a pointer once preserves sharing and terminates @ cycles.
type resolvedProjector struct {
	checker *levelChecker
	seen    map[levelNode]bool
	queue   []levelNode
}

func (p *resolvedProjector) ref(n levelNode) uint64 {
	if n == nil || reflect.ValueOf(n).IsNil() {
		return 0
	}
	if !p.seen[n] {
		p.seen[n] = true
		p.queue = append(p.queue, n)
	}
	return n.levelHeader().identity
}
func resolvedRefs[T levelNode](p *resolvedProjector, xs []T) []uint64 {
	ids := make([]uint64, len(xs))
	for i, x := range xs {
		ids[i] = p.ref(x)
	}
	return ids
}
func resolvedSpan(t *parser.SyntaxTreeNode) *SourceSpan {
	if t == nil {
		return nil
	}
	loc := t.Location()
	if loc == nil || loc.Source() == nil || loc.BeginLine() <= 0 {
		return nil
	}
	return &SourceSpan{Source: loc.Source().String(), Start: [2]int{int(loc.BeginLine()), int(loc.BeginColumn())}, End: [2]int{int(loc.EndLine()), int(loc.EndColumn())}}
}
func (p *resolvedProjector) subs(xs []*subst) ([]Substitution, error) {
	items := make([]Substitution, len(xs))
	for i, s := range xs {
		if s == nil || s.op == nil || s.expr == nil {
			return nil, fmt.Errorf("incomplete INSTANCE substitution")
		}
		items[i] = Substitution{Target: p.ref(s.op), Value: p.ref(s.expr), Implicit: s.implicit, Span: resolvedSpan(s.exprTree)}
	}
	return items, nil
}
func (p *resolvedProjector) project(node levelNode) (ResolvedNode, error) {
	h := node.levelHeader()
	n := ResolvedNode{ID: h.identity, Span: resolvedSpan(h.tree)}
	if s := p.checker.states[node]; s != nil && s.checked > 0 {
		v := s.level
		n.Level = &v
	}
	if s, ok := node.(leafNode); ok {
		name := s.header().name
		if !name.displayExact {
			return n, fmt.Errorf("unrepresentable UTF-16 symbol id %d", n.ID)
		}
		n.Name = name.text
		if arity := nodeArity(s); arity >= -1 {
			n.Arity = &arity
		}
	}
	var err error
	switch v := node.(type) {
	case *moduleNode:
		n.Kind = "module"
		n.Standard = v.standard
		n.Extends = resolvedRefs(p, v.extendees)
		v.hasDeclarations(2)
		v.hasDeclarations(3)
		n.Constants = resolvedRefs(p, v.constantCache)
		n.Variables = resolvedRefs(p, v.variableCache)
		n.Definitions = resolvedRefs(p, v.bodyDefinitions)
		n.Instances = resolvedRefs(p, v.instances)
		n.Statements = resolvedRefs(p, v.topLevel)
		n.Assumptions = resolvedRefs(p, v.assumptions)
		n.Symbols = make(map[string]uint64)
		for _, s := range v.context.orderedValues() {
			if child, ok := s.(*moduleNode); ok {
				n.Modules = append(n.Modules, p.ref(child))
				continue
			}
			n.Symbols[s.header().name.text] = p.ref(s)
		}
	case *builtinOpDefNode:
		n.Kind = "builtin"
		n.Params = resolvedRefs(p, v.params)
		if n.Level == nil && v.levels.checked > 0 {
			level := v.levels.level
			n.Level = &level
		}
	case *opDeclNode:
		switch v.kind {
		case 2:
			n.Kind = "constant"
		case 3:
			n.Kind = "variable"
		default:
			return n, fmt.Errorf("unsupported declaration kind %d at %v", v.kind, n.Span)
		}
		n.Module = p.ref(v.module)
		level := v.levels.level
		n.Level = &level
	case *formalParamNode:
		n.Kind = "formal"
		n.Module = p.ref(v.module)
	case *userOpDefNode:
		n.Kind = "definition"
		if v.kind == 6 {
			n.Kind = "moduleInstanceDefinition"
		}
		n.Params = resolvedRefs(p, v.params)
		n.Body = p.ref(v.body)
		n.Module = p.ref(v.module)
		n.SourceDefinition = p.ref(v.source)
		n.Local = v.local
		n.Recursive = v.inRecursive
		n.Labels = resolvedRefs(p, v.labels.ordered())
		if n.Kind == "definition" && n.Body == 0 {
			return n, fmt.Errorf("missing operator body for %q at %v", n.Name, n.Span)
		}
	case *opApplNode:
		n.Kind = "apply"
		n.Operator = p.ref(v.operator)
		n.Args = resolvedRefs(p, v.operands)
		for _, param := range v.unbounded {
			n.Binders = append(n.Binders, Binder{Params: []uint64{p.ref(param)}})
		}
		if len(v.bounded) != len(v.ranges) || len(v.bounded) != len(v.tupleBounds) {
			return n, fmt.Errorf("incomplete binder vector at %v", n.Span)
		}
		for i, ps := range v.bounded {
			n.Binders = append(n.Binders, Binder{Params: resolvedRefs(p, ps), Domain: p.ref(v.ranges[i]), Tuple: v.tupleBounds[i]})
		}
	case *letInNode:
		n.Kind = "let"
		n.Definitions = resolvedRefs(p, v.definitions)
		n.Instances = resolvedRefs(p, v.instances)
		n.Body = p.ref(v.body)
	case *opArgNode:
		n.Kind = "opArg"
		n.Operator = p.ref(v.operator)
		n.Module = p.ref(v.module)
	case *numeralNode:
		n.Kind = "integer"
		value := strconv.FormatInt(int64(v.value), 10)
		if v.bigValue != nil {
			value = v.bigValue.String()
		}
		n.Value = &value
	case *decimalNode:
		n.Kind = "decimal"
		value := v.integral + "." + v.fractional
		n.Value = &value
	case *stringNode:
		if !v.value.displayExact {
			return n, fmt.Errorf("unrepresentable UTF-16 string at %v", n.Span)
		}
		n.Kind = "string"
		value := v.value.text
		n.Value = &value
	case *atNode:
		n.Kind = "at"
		n.Except = p.ref(v.except)
		n.Component = p.ref(v.component)
	case *labelNode:
		if !v.name.displayExact {
			return n, fmt.Errorf("unrepresentable UTF-16 label name at %v", n.Span)
		}
		n.Kind = "label"
		n.Name = v.name.text
		n.Body = p.ref(v.body)
		n.Params = resolvedRefs(p, v.params)
		n.Labels = resolvedRefs(p, v.labels.ordered())
	case *substInNode:
		n.Kind = "substIn"
		n.Body = p.ref(v.body)
		n.FromModule = p.ref(v.instantiatingModule)
		n.ToModule = p.ref(v.instantiatedModule)
		n.Substitutions, err = p.subs(v.substs)
	case *instanceNode:
		n.Kind = "instance"
		if v.name != nil {
			if !v.name.displayExact {
				return n, fmt.Errorf("unrepresentable UTF-16 instance name at %v", n.Span)
			}
			n.Name = v.name.text
		}
		n.Local = v.local
		n.ToModule = p.ref(v.module)
		n.Params = resolvedRefs(p, v.params)
		n.Substitutions, err = p.subs(v.substs)
	case *assumeNode:
		n.Kind = "assume"
		n.Body = p.ref(v.expression)
		n.Module = p.ref(v.module)
		n.SourceDefinition = p.ref(v.definition)
	case *thmOrAssumpDefNode:
		if v.theorem {
			return n, fmt.Errorf("unsupported theorem %q at %v", n.Name, n.Span)
		}
		n.Kind = "assumeDefinition"
		n.Body = p.ref(v.body)
		n.Module = p.ref(v.module)
		n.Params = resolvedRefs(p, v.params)
		n.Local = v.local
		n.SourceDefinition = p.ref(v.source)
	default:
		return n, fmt.Errorf("unsupported semantic node %T at %v", node, n.Span)
	}
	return n, err
}
