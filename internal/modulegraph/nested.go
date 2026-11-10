// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ ParseUnit.java and SpecObj.java, Copyright 2003 Compaq
// Corporation and 2003 Microsoft Corporation, MIT; see LICENSE.
package modulegraph

import (
	"context"
	parser "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

// ModuleRef is a lexical occurrence. Its Document owns its tree and bytes.
type ModuleRef struct {
	Name     string
	Tree     *parser.SyntaxTreeNode
	Document *Document
	Parent   *ModuleRef
	Children []*ModuleRef
	refs     []Reference
	bindings map[string]*ModuleRef
}

func discoverModules(ctx context.Context, d *Document, tree *parser.SyntaxTreeNode, lim AcquisitionLimits) (*ModuleRef, error) {
	visits := 0
	check := func(depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		visits++
		if depth > lim.MaxDepth {
			return ErrIncomplete
		}
		if visits > lim.MaxVisits {
			return &BudgetError{Pass: PassDiscovery, Limit: lim.MaxVisits, Observed: visits}
		}
		return nil
	}
	var discover func(*parser.SyntaxTreeNode, *ModuleRef, int) (*ModuleRef, error)
	discover = func(t *parser.SyntaxTreeNode, p *ModuleRef, depth int) (*ModuleRef, error) {
		if err := check(depth); err != nil {
			return nil, err
		}
		if t == nil || t.Kind() != parser.N_Module {
			return nil, ErrIncomplete
		}
		h := t.Heirs()
		if len(h) < 3 || len(h[0].Heirs()) < 2 || h[1].Kind() != parser.N_Extends {
			return nil, ErrIncomplete
		}
		n, _, err := exactName(h[0].Heirs()[1].Image())
		if err != nil {
			return nil, err
		}
		m := &ModuleRef{Name: n, Tree: t, Document: d, Parent: p, bindings: map[string]*ModuleRef{}}
		if p != nil {
			p.Children = append(p.Children, m)
		}
		for i := 1; i < len(h[1].Heirs()); i += 2 {
			x := h[1].Heirs()[i]
			n, _, e := exactName(x.Image())
			if e != nil {
				return nil, e
			}
			m.refs = append(m.refs, Reference{Name: n, Tree: x, Kind: Extends})
		}
		if p != nil {
			for k, v := range p.bindings {
				if err := check(depth); err != nil {
					return nil, err
				}
				m.bindings[k] = v
			}
		}
		for a := m; a.Parent != nil; a = a.Parent {
			for k, v := range a.Parent.bindings {
				if err := check(depth); err != nil {
					return nil, err
				}
				if m.bindings[k] == nil {
					m.bindings[k] = v
				}
			}
			peers := a.Parent.Children
			for _, v := range peers[:len(peers)-1] {
				if err := check(depth); err != nil {
					return nil, err
				}
				if m.bindings[v.Name] == nil {
					m.bindings[v.Name] = v
				}
			}
		}
		var extensions func(*ModuleRef, int) error
		extensions = func(other *ModuleRef, depth int) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			visits++
			if depth > lim.MaxDepth {
				return ErrIncomplete
			}
			if visits > lim.MaxVisits {
				return &BudgetError{Pass: PassDiscovery, Limit: lim.MaxVisits, Observed: visits}
			}
			for _, r := range other.refs {
				if err := check(depth); err != nil {
					return err
				}
				if r.Kind != Extends {
					continue
				}
				if ext := m.bindings[r.Name]; ext != nil {
					for _, c := range ext.Children {
						if err := check(depth); err != nil {
							return err
						}
						if m.bindings[c.Name] == nil {
							m.bindings[c.Name] = c
						}
						if e := extensions(ext, depth+1); e != nil {
							return e
						}
					}
				}
			}
			return nil
		}
		if err = extensions(m, 1); err != nil {
			return nil, err
		}
		for _, stmt := range h[2].Heirs() {
			if err := check(depth); err != nil {
				return nil, err
			}
			if stmt.Kind() == parser.N_Module {
				if _, err = discover(stmt, m, depth+1); err != nil {
					return nil, err
				}
				continue
			}
			// ParseUnit.getInstances scans each non-module body tree in preorder,
			// including substitutions and proof bodies. Its separate internal
			// lookup still examines top-level body entries only.
			type frame struct {
				node  *parser.SyntaxTreeNode
				depth int
			}
			stack := []frame{{stmt, depth + 1}}
			for len(stack) > 0 {
				f := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if err := check(f.depth); err != nil {
					return nil, err
				}
				if f.node == nil || f.node.Kind() == parser.N_Module {
					return nil, ErrIncomplete
				}
				children := f.node.Heirs()
				offset := 0
				if f.node.Kind() == parser.N_NonLocalInstance {
					if len(children) < 2 || children[1] == nil {
						return nil, ErrIncomplete
					}
					n, _, e := exactName(children[1].Image())
					if e != nil {
						return nil, e
					}
					m.refs = append(m.refs, Reference{Name: n, Tree: children[1], Kind: Instance})
					offset = 2
				}
				for i := len(children) - 1; i >= offset; i-- {
					stack = append(stack, frame{children[i], f.depth + 1})
				}
			}
		}
		return m, nil
	}
	return discover(tree, nil, 1)
}

// References returns this module's own EXTENDS and INSTANCE references in
// discovery order. The slice is a copy; Trees are shared read-only.
func (m *ModuleRef) References() []Reference { return append([]Reference(nil), m.refs...) }

// Bound returns the module that discovery, acquisition or propagation bound
// to name in this module's context, or nil. A unit's root module has no
// Parent; an inner module does.
func (m *ModuleRef) Bound(name string) *ModuleRef { return m.bindings[name] }

// ResolvesInternally reports whether an unbound INSTANCE of name is skipped as
// internal, as SpecObj.instanceResolvesToInternalModule (SpecObj.java:528-600)
// decides: the first top-level INSTANCE or module definition naming it must
// follow a top-level inner module of that name. The check owns its visit
// budget and reads no acquisition counter, so it never spends or changes the
// acquisition budget.
func (m *ModuleRef) ResolvesInternally(ctx context.Context, name string, maxVisits int) (bool, error) {
	if ctx == nil || m == nil || m.Tree == nil || maxVisits <= 0 || len(m.Tree.Heirs()) < 3 || m.Tree.Heirs()[2] == nil {
		return false, ErrIncomplete
	}
	visits := 0
	return m.internalInstance(name, func(int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		visits++
		if visits > maxVisits {
			return ErrIncomplete
		}
		return nil
	}, 1)
}

func (m *ModuleRef) internalInstance(name string, check func(int) error, depth int) (bool, error) {
	seen := map[string]bool{}
	for _, s := range m.Tree.Heirs()[2].Heirs() {
		if err := check(depth); err != nil {
			return false, err
		}
		if s.Kind() == parser.N_Module {
			h := s.Heirs()
			if len(h) > 0 && len(h[0].Heirs()) > 1 {
				n, _, _ := exactName(h[0].Heirs()[1].Image())
				seen[n] = true
			}
			continue
		}
		if s.Kind() == parser.N_Instance || s.Kind() == parser.N_ModuleDefinition {
			h := s.Heirs()
			if len(h) > 0 {
				h = h[len(h)-1].Heirs()
				if len(h) > 1 {
					n, _, _ := exactName(h[1].Image())
					if n == name {
						return seen[name], nil
					}
				}
			}
		}
	}
	return false, nil
}

func (a *Acquisition) acquireModules(ctx context.Context, resolve Resolver, byName map[string]*Document, parse func(Input, string) (*Document, error), lim AcquisitionLimits) error {
	visits := 0
	check := func(depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		visits++
		if depth > lim.MaxDepth {
			return ErrIncomplete
		}
		if visits > lim.MaxVisits {
			return &BudgetError{Pass: PassResolution, Limit: lim.MaxVisits, Observed: visits}
		}
		return nil
	}
	var next func(*ModuleRef, map[*ModuleRef]bool, int, EdgeKind) (*ModuleRef, string, error)
	next = func(m *ModuleRef, seen map[*ModuleRef]bool, depth int, kind EdgeKind) (*ModuleRef, string, error) {
		if e := check(depth); e != nil {
			return nil, "", e
		}
		if m == nil || seen[m] {
			return nil, "", nil
		}
		seen[m] = true
		for _, r := range m.refs {
			if e := check(depth); e != nil {
				return nil, "", e
			}
			if r.Kind == kind && m.bindings[r.Name] == nil {
				if kind == Instance {
					internal, err := m.internalInstance(r.Name, check, depth)
					if err != nil {
						return nil, "", err
					}
					if internal {
						continue
					}
				}
				return m, r.Name, nil
			}
		}
		for _, k := range []EdgeKind{Extends, Instance} {
			for _, r := range m.refs {
				if r.Kind == k && m.bindings[r.Name] != nil {
					o, n, e := next(m.bindings[r.Name], seen, depth+1, kind)
					if e != nil || o != nil {
						return o, n, e
					}
				}
			}
		}
		for _, c := range m.Children {
			o, n, e := next(c, seen, depth+1, kind)
			if e != nil || o != nil {
				return o, n, e
			}
		}
		return nil, "", nil
	}
	for {
		kind := Extends
		owner, name, e := next(a.Root.RootModule, map[*ModuleRef]bool{}, 1, kind)
		if e == nil && owner == nil {
			kind = Instance
			owner, name, e = next(a.Root.RootModule, map[*ModuleRef]bool{}, 1, kind)
		}
		if e != nil {
			return e
		}
		if owner == nil {
			return nil
		}
		a.Request, a.RequestOwner = nil, owner
		for _, ref := range owner.refs {
			if ref.Kind == kind && ref.Name == name {
				r := ref
				a.Request = &r
				break
			}
		}
		target := byName[name]
		if target == nil {
			if resolve == nil {
				return ErrResolverUnavailable
			}
			in, e := resolve(ctx, name)
			if e != nil {
				return e
			}
			target, e = parse(in, name)
			if e != nil {
				if a.Failed != nil {
					for _, ref := range owner.refs {
						if ref.Kind == kind && ref.Name == name {
							r := ref
							a.FailedReference = &r
							a.FailedOwner = owner
							break
						}
					}
				}
				return e
			}
			byName[name] = target
			a.Units = append(a.Units, target)
		}
		if kind == Extends {
			a.validate(target)
		}
		if e = a.graph.Link(ctx, owner.Document.Unit, target.Unit, kind); e != nil {
			return e
		}
		owner.bindings[name] = target.RootModule
		owner.Document.linked[name] = target
		owner.Document.edges = append(owner.Document.edges, Reference{Name: name, Kind: kind})
		if kind == Extends {
			// Propagate matching child names through the requester and its descendants.
			// Indirect-extender enumeration is guarded where its source
			// Hashtable order could change the result.
			var propagate func(*ModuleRef, int) error
			propagate = func(m *ModuleRef, depth int) error {
				if e := check(depth); e != nil {
					return e
				}
				for _, r := range m.refs {
					if e := check(depth); e != nil {
						return e
					}
					for _, c := range target.RootModule.Children {
						if e := check(depth); e != nil {
							return e
						}
						if r.Name == c.Name {
							if m.bindings[r.Name] == nil {
								m.bindings[r.Name] = c
							}
							break
						}
					}
				}
				for _, c := range m.Children {
					if e := propagate(c, depth+1); e != nil {
						return e
					}
				}
				return nil
			}
			if len(target.RootModule.Children) == 0 {
				continue
			}
			// Java enumerates a Hashtable of modules at each indirect step.
			// A unique next extender has order-independent behavior; ambiguous
			// enumeration remains incomplete until that table is modeled.
			all := []*ModuleRef{}
			var collect func(*ModuleRef, int) error
			collect = func(m *ModuleRef, depth int) error {
				if e := check(depth); e != nil {
					return e
				}
				all = append(all, m)
				for _, c := range m.Children {
					if e := collect(c, depth+1); e != nil {
						return e
					}
				}
				return nil
			}
			for _, d := range a.Units {
				if e = collect(d.RootModule, 1); e != nil {
					return e
				}
			}
			extenders := []*ModuleRef{owner}
			seen := map[*ModuleRef]bool{owner: true}
			for i := 0; i < len(extenders); i++ {
				var candidate *ModuleRef
				for _, m := range all {
					if e = check(1); e != nil {
						return e
					}
					if seen[m] {
						continue
					}
					for _, r := range m.refs {
						if e = check(1); e != nil {
							return e
						}
						if r.Kind == Extends && m.bindings[r.Name] == extenders[i] {
							if candidate != nil && candidate != m {
								return ErrIncomplete
							}
							candidate = m
							break
						}
					}
				}
				if candidate != nil {
					seen[candidate] = true
					extenders = append(extenders, candidate)
				}
			}
			for _, m := range extenders {
				if e = propagate(m, 1); e != nil {
					return e
				}
			}

		}
	}
}
