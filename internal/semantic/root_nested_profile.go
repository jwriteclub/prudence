// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"

	"github.com/jwriteclub/prudence/internal/modulegraph"
)

// rootNestedProfile names the driver profile for root-local nesting
// (predicate N). Only the root unit has inner modules, and the graph with
// them removed lies in an existing profile. Java citations are to the pinned
// dc6a944 tla2sany sources.
//
// Predicate N, over the root document R and inner(R), every inner module of R
// at any depth:
//
//	N0  the existing premises: a fresh entry and history, stable resolution
//	    with Java's selected source and standard bit, ASCII module names, every
//	    acquired unit in Order, and a successful acquisition;
//	N1  no unit other than R has inner modules;
//	N2  every reference of every m in inner(R) is bound to an inner module of
//	    R, or is an unbound INSTANCE that m resolves internally;
//	N3  every root-module reference is bound to the unit Linked under its name,
//	    or is an unbound INSTANCE that R's root module resolves internally;
//	N4  no m in inner(R) has the name of an acquired unit;
//	N5  no unit is monolith-extracted;
//	N6  the reduced view V lies in the linear, flat-EXTENDS or standard LOCAL
//	    INSTANCE profile exactly as those predicates are written. V gives R its
//	    acquired root-module references (N3 without the internal INSTANCEs),
//	    every other unit its References(), and exempts R's inner modules from
//	    the predicates' nested-module checks. R's Edges() equal V(R) in kind,
//	    name and order.
//
// Reduction theorem. Under N, acquisition on the real graph G is, step for
// step, acquisition on V, on both sides. Each search (SpecObj.java:439-512,
// 619-701; nested.go next) sees R's EXTENDS names, which are acquired-unit
// references in G and V alike, because R's header cannot see its own inner
// modules (ParseUnit.java:437-467). R's extra INSTANCE names are internal, so
// both searches skip them without effect. Recursion into R's inner modules
// finds nothing, since every name there is bound to an inner module or is
// internal (N2), and that subtree never reaches a unit. Each step therefore
// selects the same owner, name and kind, links the same edge, runs the same
// cycle test and binds the same name. Propagation (SpecObj.java:787-882)
// binds nothing, because no target has inner modules (N1) and R is never a
// target. By induction the whole run on G equals the run on V, and the
// existing profile theorem for V gives Java(V) = Go(V); hence Java and Go
// agree on G in acquisition, unit bindings, vectors, cycle outcomes,
// semanticAnalysisVector = Order, per-unit Generator inputs and the level
// boundary schedule. Non-root Generators are V's by N1. In R's Generator no
// inner name is a key of the external table (N4, and only unit roots are
// published, SANY.java:421-470), so inner names resolve lexically as in the
// single-unit path, unit names resolve through the table as in V, and the
// nested schedule is the already-modeled one: one ModuleNode UID per inner
// module (Generator.java:2192), no Generator quartet, and Contexts and
// SymbolTables that are not SemanticNodes (Generator.java:2238-2252).
//
// L-ID. The one identity-hashed enumeration on this path is getModules in
// getModulesIndirectlyExtending (SpecObj.java:725-769). Every write it
// drives is bindIfNotBound(n, v) with v depending only on the extendee and n
// (SpecObj.java:807-882), and the extender set is complete before the first
// write, so neither the order nor the multiplicity of extenders changes any
// binding, edge, error or order. Under N the path binds nothing at all.
//
// The profile reads the acquisition only. It never changes the acquisition,
// trees, references, edges, generation inputs or publication, invokes no
// analysis and allocates no semantic node. Its walks own their budget,
// bounded by the acquisition's visit and depth limits, and stop on
// cancellation.
const rootNestedProfile = "root-nested"

// rootNestedReducedView is the first reason when N0-N5 hold and the reduced
// view lies outside all three profiles; the verdict then carries each reason.
const rootNestedReducedView = "reduced view outside every profile"

// rootNestedVerdict is the read-only outcome of predicate N. profile is the
// admitting sub-profile, or "" when why holds the first failed condition.
type rootNestedVerdict struct {
	profile                string
	why                    string
	linear, flat, standard string // the reduced view's own rejections
}

// referenceView is the read-only input of the profile predicates: each unit's
// root-module references and whether its inner modules count as nesting.
// actualView reads the acquisition as it is. A reduced view replaces only the
// root's references and exempts only the root's inner modules.
type referenceView struct {
	root     *modulegraph.Document
	rootRefs []modulegraph.Reference
	reduced  bool
}

func actualView() referenceView { return referenceView{} }

func (v referenceView) references(d *modulegraph.Document) []modulegraph.Reference {
	if v.reduced && d == v.root {
		return append([]modulegraph.Reference(nil), v.rootRefs...)
	}
	return d.References()
}

// nested reports inner modules that the predicates must reject. The caller
// has already checked d.RootModule.
func (v referenceView) nested(d *modulegraph.Document) bool {
	if v.reduced && d == v.root {
		return false
	}
	return len(d.RootModule.Children) != 0
}

// rootNestedInspection is the request-owned budget of one predicate run.
type rootNestedInspection struct {
	ctx      stdcontext.Context
	maxDepth int
	left     int
}

func (in *rootNestedInspection) spend(n int) bool {
	if in.ctx == nil || in.ctx.Err() != nil || n < 0 || n > in.left {
		in.left = 0
		return false
	}
	in.left -= n
	return true
}

// internal charges one visit per top-level statement of m, the most that the
// internal-resolution scan can read, and runs it on that budget.
func (in *rootNestedInspection) internal(m *modulegraph.ModuleRef, name string) (bool, bool) {
	cost := len(m.Tree.Heirs()[2].Heirs()) + 1
	if !in.spend(cost) {
		return false, false
	}
	internal, err := m.ResolvesInternally(in.ctx, name, cost)
	return internal, err == nil
}

const rootNestedInspectionStopped = "nested inspection cancelled or over budget"

// rootNestedRejection evaluates predicate N, in this order: coverage and
// structure, N1, N5, the ASCII premise of N0, N4, N2, N3, nesting in the
// root, the V edge check, then V against linear, flat-EXTENDS and standard.
func rootNestedRejection(ctx stdcontext.Context, acquisition *modulegraph.Acquisition, order []*modulegraph.Unit, docs map[*modulegraph.Unit]*modulegraph.Document) rootNestedVerdict {
	reject := func(why string) rootNestedVerdict { return rootNestedVerdict{why: why} }
	if acquisition == nil || acquisition.Root == nil || len(order) != len(docs) || len(order) != len(acquisition.Units) {
		return reject("dependency order does not cover acquired units")
	}
	for _, d := range acquisition.Units {
		if d.RootModule == nil || d.RootModule.Tree == nil || len(d.RootModule.Tree.Heirs()) < 3 || d.RootModule.Tree.Heirs()[2] == nil {
			return reject("module structure unavailable")
		}
	}
	root := acquisition.Root
	maxDepth, maxVisits := acquisition.InspectionLimits()
	in := &rootNestedInspection{ctx: ctx, maxDepth: maxDepth, left: maxVisits}
	// N1.
	for _, d := range acquisition.Units {
		if d != root && len(d.RootModule.Children) != 0 {
			return reject("nested module definitions in dependency")
		}
	}
	// N5.
	for _, d := range acquisition.Units {
		if d.Input.View != nil {
			return reject("monolith-extracted unit")
		}
	}
	inner, ok := in.innerModules(root.RootModule)
	if !ok {
		return reject(rootNestedInspectionStopped)
	}
	// N0: the proof's ASCII module-name premise, applied explicitly.
	units := map[string]bool{}
	for _, d := range acquisition.Units {
		if !asciiModuleName(d.Name) {
			return reject("non-ASCII module name")
		}
		units[d.Name] = true
	}
	for _, m := range inner {
		if !asciiModuleName(m.Name) {
			return reject("non-ASCII module name")
		}
	}
	// N4.
	for _, m := range inner {
		if units[m.Name] {
			return reject("inner module named like an acquired unit")
		}
	}
	// N2.
	for _, m := range inner {
		for _, r := range m.References() {
			if !in.spend(1) {
				return reject(rootNestedInspectionStopped)
			}
			if b := m.Bound(r.Name); b != nil {
				if b.Parent == nil || b.Document != root {
					return reject("inner module reference leaves the root unit")
				}
				continue
			}
			if r.Kind != modulegraph.Instance {
				return reject("inner module reference leaves the root unit")
			}
			internal, ok := in.internal(m, r.Name)
			if !ok {
				return reject(rootNestedInspectionStopped)
			}
			if !internal {
				return reject("inner module reference leaves the root unit")
			}
		}
	}
	// N3, building V(R).
	var reduced []modulegraph.Reference
	for _, r := range root.References() {
		if !in.spend(1) {
			return reject(rootNestedInspectionStopped)
		}
		bound, unit := root.RootModule.Bound(r.Name), root.Linked(r.Name)
		if bound != nil {
			if unit == nil || bound != unit.RootModule {
				return reject("root reference neither acquired nor internal")
			}
			reduced = append(reduced, r)
			continue
		}
		if unit != nil || r.Kind != modulegraph.Instance {
			return reject("root reference neither acquired nor internal")
		}
		internal, ok := in.internal(root.RootModule, r.Name)
		if !ok {
			return reject(rootNestedInspectionStopped)
		}
		if !internal {
			return reject("root reference neither acquired nor internal")
		}
	}
	// Without inner modules V is the graph itself, which the existing
	// profiles have already rejected; say so instead of repeating them.
	if len(inner) == 0 {
		return reject("root unit has no inner modules")
	}
	// N6: R's acquired edges are exactly V(R).
	edges := root.Edges()
	if len(edges) != len(reduced) {
		return reject("root edges differ from the reduced view")
	}
	for i, e := range edges {
		if e.Name != reduced[i].Name || e.Kind != reduced[i].Kind {
			return reject("root edges differ from the reduced view")
		}
	}
	// N6: V against the existing predicates, unchanged, in precedence order.
	view := referenceView{root: root, rootRefs: reduced, reduced: true}
	linearWhy, flatWhy := olderProfileRejections(order, acquisition.Units, docs, view)
	if linearWhy == "" {
		return rootNestedVerdict{profile: "linear"}
	}
	if flatWhy == "" {
		return rootNestedVerdict{profile: "flat-extends"}
	}
	standardWhy := standardLocalInstanceRejection(acquisition, order, docs, view)
	if standardWhy == "" {
		return rootNestedVerdict{profile: standardLocalInstanceProfile}
	}
	return rootNestedVerdict{why: rootNestedReducedView, linear: linearWhy, flat: flatWhy, standard: standardWhy}
}

// innerModules returns inner(R) in preorder, within the inspection budget and
// the acquisition's depth limit.
func (in *rootNestedInspection) innerModules(root *modulegraph.ModuleRef) ([]*modulegraph.ModuleRef, bool) {
	type frame struct {
		m     *modulegraph.ModuleRef
		depth int
	}
	var out []*modulegraph.ModuleRef
	var stack []frame
	for i := len(root.Children) - 1; i >= 0; i-- {
		stack = append(stack, frame{root.Children[i], 2})
	}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if f.m == nil || f.m.Tree == nil || len(f.m.Tree.Heirs()) < 3 || f.m.Tree.Heirs()[2] == nil || f.depth > in.maxDepth || !in.spend(1) {
			return nil, false
		}
		out = append(out, f.m)
		for i := len(f.m.Children) - 1; i >= 0; i-- {
			stack = append(stack, frame{f.m.Children[i], f.depth + 1})
		}
	}
	return out, true
}

// asciiModuleName is the proof's premise for String.hashCode and Hashtable
// bucket arithmetic over module names: non-empty, 7-bit units only.
func asciiModuleName(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// rootNestedCause formats the private telemetry clause from fixed reasons.
func rootNestedCause(v rootNestedVerdict) string {
	if v.why == rootNestedReducedView {
		return "reduced view: linear: " + boundedCause(v.linear) + " / flat-extends: " + boundedCause(v.flat) + " / " + standardLocalInstanceProfile + ": " + boundedCause(v.standard)
	}
	return boundedCause(v.why)
}
