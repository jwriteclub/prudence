// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ SpecObj.java, Copyright 2003 Compaq Corporation and
// 2003 Microsoft Corporation, MIT; see LICENSE.

// Package modulegraph models ordered relationships between already resolved
// external parse units. It does not resolve modules or run semantic analysis.
package modulegraph

import (
	"context"
	"errors"
	"fmt"
)

var ErrIncomplete = errors.New("module relationships incomplete")

// BudgetPass names the acquisition pass whose work budget ran out.
type BudgetPass string

// Each pass keeps its own counter, checked against the same limit.
const (
	// PassSyntaxNodes counts syntax-tree nodes walked across every acquired
	// unit (AcquisitionLimits.MaxNodes).
	PassSyntaxNodes BudgetPass = "syntax-node"
	// PassDiscovery counts module-discovery visits within one unit.
	PassDiscovery BudgetPass = "module discovery"
	// PassResolution counts module-resolution and propagation visits across
	// the acquisition.
	PassResolution BudgetPass = "module resolution"
	// PassCycle counts the visits of one link's cycle search.
	PassCycle BudgetPass = "cycle check"
	// PassOrder counts the visits of one dependency-order traversal.
	PassOrder BudgetPass = "dependency order"
)

// BudgetError reports that an acquisition pass exceeded its node or visit
// budget. Observed is the pass's running count at the stop (Limit+1), not the
// work the whole pass would need. It wraps ErrIncomplete.
type BudgetError struct {
	Pass            BudgetPass
	Limit, Observed int
}

func (e *BudgetError) Error() string {
	return fmt.Sprintf("module acquisition %s pass made %d visits, exceeding its budget of %d", e.Pass, e.Observed, e.Limit)
}

func (e *BudgetError) Unwrap() error { return ErrIncomplete }

// Unit is one external parse unit, not an inner module. nameKey is the caller's
// exact UTF-16-unit identity encoded losslessly as an opaque string. filename is
// retained for the cycle path; it is a label, never a filesystem path to open.
type Unit struct {
	nameKey               string
	filename              string
	extendees, instancees []*Unit
	owner                 *Graph
}

func (u *Unit) NameKey() string  { return u.nameKey }
func (u *Unit) Filename() string { return u.filename }

// Extendees returns a copy of the unit's extendee vector as it stands,
// including the cycle test's in-place appendNoRepeats mutation.
func (u *Unit) Extendees() []*Unit { return append([]*Unit(nil), u.extendees...) }

// Instancees returns a copy of the unit's instancee vector.
func (u *Unit) Instancees() []*Unit { return append([]*Unit(nil), u.instancees...) }

type EdgeKind uint8

const (
	Extends EdgeKind = iota + 1
	Instance
)

type CycleError struct{ Path []string }

func (*CycleError) Error() string { return "circular module dependencies" }

type Graph struct {
	units     map[string]*Unit
	maxDepth  int
	maxVisits int
	maxUnits  int
	maxEdges  int
	edges     int
	failed    bool
}

func New(maxDepth, maxVisits, maxUnits, maxEdges int) (*Graph, error) {
	if maxDepth <= 0 || maxVisits <= 0 || maxUnits <= 0 || maxEdges <= 0 {
		return nil, ErrIncomplete
	}
	return &Graph{units: make(map[string]*Unit), maxDepth: maxDepth, maxVisits: maxVisits, maxUnits: maxUnits, maxEdges: maxEdges}, nil
}
func (g *Graph) Unit(key, filename string) (*Unit, error) {
	if g == nil || g.failed || key == "" {
		return nil, ErrIncomplete
	}
	if u := g.units[key]; u != nil {
		return u, nil
	}
	if len(g.units) >= g.maxUnits {
		return nil, ErrIncomplete
	}
	u := &Unit{nameKey: key, filename: filename, owner: g}
	g.units[key] = u
	return u, nil
}
func (g *Graph) valid(u *Unit) bool {
	return g != nil && !g.failed && u != nil && u.owner == g && g.units[u.nameKey] == u
}
func (g *Graph) check(ctx context.Context, depth int) error {
	if ctx == nil || depth > g.maxDepth {
		return ErrIncomplete
	}
	return ctx.Err()
}

// Link follows loadSpec: append the edge, then run the cycle search starting
// at its target. On error the caller must discard the graph, as SANY aborts.
func (g *Graph) Link(ctx context.Context, from, to *Unit, kind EdgeKind) error {
	if !g.valid(from) || !g.valid(to) {
		return ErrIncomplete
	}
	if err := g.check(ctx, 1); err != nil {
		return err
	}
	if g.edges >= g.maxEdges {
		return ErrIncomplete
	}
	switch kind {
	case Extends:
		from.extendees = append(from.extendees, to)
	case Instance:
		from.instancees = append(from.instancees, to)
	default:
		return ErrIncomplete
	}
	g.edges++
	visits := 0
	err := g.cycle(ctx, to, to, map[*Unit]bool{}, []*Unit{to}, 1, &visits)
	if err != nil {
		g.failed = true
	}
	return err
}
func (g *Graph) cycle(ctx context.Context, target, current *Unit, visited map[*Unit]bool, path []*Unit, depth int, visits *int) error {
	if err := g.check(ctx, depth); err != nil {
		return err
	}
	*visits++
	if *visits > g.maxVisits {
		return &BudgetError{Pass: PassCycle, Limit: g.maxVisits, Observed: *visits}
	}
	if visited[current] {
		return nil
	}
	visited[current] = true
	// SpecObj takes the actual extendees Vector and appendNoRepeats mutates it.
	for _, u := range current.instancees {
		found := false
		for _, v := range current.extendees {
			if u == v {
				found = true
				break
			}
		}
		if !found {
			current.extendees = append(current.extendees, u)
		}
	}
	for _, next := range current.extendees {
		if next == target {
			out := make([]string, 0, len(path)+1)
			for _, u := range path {
				out = append(out, u.filename)
			}
			out = append(out, target.filename)
			return &CycleError{Path: out}
		}
		if err := g.cycle(ctx, target, next, visited, append(path, next), depth+1, visits); err != nil {
			return err
		}
	}
	return nil
}

// Order returns postorder, retaining SANY's extendee-before-instancee traversal
// and name-based deduplication. This assumes every Link succeeded; unresolved
// relationships are not accepted as a complete dependency closure here.
func (g *Graph) Order(ctx context.Context, root *Unit) ([]*Unit, error) {
	if !g.valid(root) {
		return nil, ErrIncomplete
	}
	out := make([]*Unit, 0)
	done := map[string]bool{}
	finished := map[*Unit]bool{}
	active := map[*Unit]bool{}
	visits := 0
	var visit func(*Unit, int) error
	visit = func(u *Unit, depth int) error {
		if err := g.check(ctx, depth); err != nil {
			return err
		}
		if finished[u] {
			return nil
		}
		visits++
		if visits > g.maxVisits {
			return &BudgetError{Pass: PassOrder, Limit: g.maxVisits, Observed: visits}
		}
		if active[u] {
			return ErrIncomplete
		}
		active[u] = true
		for _, v := range u.extendees {
			if err := visit(v, depth+1); err != nil {
				return err
			}
		}
		for _, v := range u.instancees {
			if err := visit(v, depth+1); err != nil {
				return err
			}
		}
		delete(active, u)
		finished[u] = true
		if !done[u.nameKey] {
			done[u.nameKey] = true
			out = append(out, u)
		}
		return nil
	}
	if err := visit(root, 1); err != nil {
		return nil, err
	}
	return out, nil
}
