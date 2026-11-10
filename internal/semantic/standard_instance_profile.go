// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	"bytes"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/modulegraph"
	"github.com/jwriteclub/prudence/internal/standard"
)

// standardLocalInstanceProfile names the driver profile for flat user EXTENDS
// graphs over pinned standard modules whose only INSTANCE statements are
// top-level, unnamed LOCAL INSTANCEs of standard modules whose EXTENDS closure
// is pinned, parameter-free and free of units mixing EXTENDS with INSTANCE
// (predicate Q, instanceeClosureRejection). Java citations are to the pinned
// dc6a944 tla2sany sources.
//
// Acquisition (SpecObj.java:930-983) searches for an unresolved EXTENDS before
// any INSTANCE. Each search writes but never reads the instantiationFound and
// extentionFound fields (SpecObj.java:423-424), and the short-circuit at :930
// leaves instantiationFound true from the first successful instantiation until
// the loop ends. When a later iteration finds an extension (an instancee's own
// EXTENDS, such as Integers EXTENDS Naturals reached through TLCExt), Java
// appends the target to the fresh owner's extendees (:956) and then the SAME
// object to that owner's instancees (:961). The extra instancee entry
// duplicates an extendee edge of the same owner, so Java's extendee vectors
// equal Go's recorded EXTENDS edges and each instancee vector is Go's with
// extra elements already present in the same owner's extendees. The cycle
// test's appendNoRepeats (:381-382) skips those extras by identity, and
// calculateDependencies (:312-336) revisits them only after their postorder is
// complete, so acquisition, bindings, cycle outcomes, generation order, UIDs
// and level boundaries equal Go's (Graph.Link appends exactly one edge, and
// nested.go resets the kind each iteration). The extendee and instancee
// vectors have no other readers. This predicate checks Go's edges against the
// owner's references. Distinct names, no inner modules and no monolith views
// keep every reference bound to exactly one acquired unit.
const standardLocalInstanceProfile = "standard-local-instance"

// standardLocalInstanceRejection returns "" when the acquired graph lies in the
// profile, or else its first failed condition. It reads only the acquisition,
// through view: actualView is the profile as admitted on its own, and the
// root-nested profile passes its reduced view.
func standardLocalInstanceRejection(acquisition *modulegraph.Acquisition, order []*modulegraph.Unit, docs map[*modulegraph.Unit]*modulegraph.Document, view referenceView) string {
	if acquisition == nil || len(order) != len(docs) || len(order) != len(acquisition.Units) {
		return "dependency order does not cover acquired units"
	}
	// Unit identity first, so a shadow or unpinned source is reported as such
	// rather than as a bad reference from whichever unit names it.
	for _, d := range acquisition.Units {
		if d.RootModule == nil || d.RootModule.Tree == nil || len(d.RootModule.Tree.Heirs()) < 3 {
			return "module structure unavailable"
		}
		if view.nested(d) {
			return "nested module definitions"
		}
		if d.Input.View != nil {
			return "monolith-extracted unit"
		}
		switch d.Input.Origin {
		case modulegraph.OriginStandard:
			if !pinnedStandardSource(d) {
				return "standard unit is not the pinned source"
			}
		case modulegraph.OriginUnspecified, modulegraph.OriginWorkspace:
			if standard.Lookup(d.Name) != nil {
				return "workspace unit shadows a standard module"
			}
		default:
			return "unknown source origin"
		}
	}
	instances := 0
	for _, d := range acquisition.Units {
		pinned := d.Input.Origin == modulegraph.OriginStandard
		refs := view.references(d)
		seen := map[string]bool{}
		for _, r := range refs {
			if seen[r.Name] {
				return "repeated module reference"
			}
			seen[r.Name] = true
		}
		edges := d.Edges()
		if len(edges) != len(refs) {
			return "acquired edges differ from references"
		}
		local := map[*generated.SyntaxTreeNode]bool{}
		if pinned {
			local = topLevelLocalInstanceNames(d)
		}
		for i, r := range refs {
			if edges[i].Name != r.Name || edges[i].Kind != r.Kind {
				return "acquired edges differ from references"
			}
			target := d.Linked(r.Name)
			if target == nil {
				return "unbound module reference"
			}
			if r.Kind == modulegraph.Extends {
				continue
			}
			if r.Kind != modulegraph.Instance {
				return "unknown module reference kind"
			}
			if !pinned {
				return "INSTANCE outside a pinned standard unit"
			}
			if !local[r.Tree] {
				return "INSTANCE is not a top-level unnamed LOCAL INSTANCE without WITH"
			}
			if target.Input.Origin != modulegraph.OriginStandard || !pinnedStandardSource(target) {
				return "INSTANCE target is not a pinned standard unit"
			}
			if why := instanceeClosureRejection(target, len(acquisition.Units)); why != "" {
				return why
			}
			instances++
		}
	}
	if instances == 0 {
		return "no standard LOCAL INSTANCE"
	}
	return ""
}

// pinnedStandardSource requires the embedded resource itself, not a name or an
// asserted origin: the bytes and the resolver's fixed label must both match.
func pinnedStandardSource(d *modulegraph.Document) bool {
	src := standard.Lookup(d.Name)
	return src != nil && d.Input.View == nil && d.Input.Path == d.Name+".tla" && bytes.Equal(src, d.Input.Text)
}

// topLevelLocalInstanceNames returns the module-name nodes of the top-level
// `LOCAL INSTANCE M` statements without WITH, matching the shape the generator
// accepts (instances.go unnamedInstance) and Generator.java:5351,5365.
func topLevelLocalInstanceNames(d *modulegraph.Document) map[*generated.SyntaxTreeNode]bool {
	out := map[*generated.SyntaxTreeNode]bool{}
	for _, s := range d.RootModule.Tree.Heirs()[2].Heirs() {
		if s == nil || s.Kind() != generated.N_Instance {
			continue
		}
		h := s.Heirs()
		if len(h) != 2 || h[0] == nil || h[0].Kind() != 54 || h[1] == nil || h[1].Kind() != generated.N_NonLocalInstance {
			continue
		}
		if c := h[1].Heirs(); len(c) == 2 && c[1] != nil {
			out[c[1]] = true
		}
	}
	return out
}

// instanceeClosureRejection applies predicate Q to an INSTANCE target T. C(T)
// is T with every unit reached through bound EXTENDS references, transitively.
// Q4: C(T) is fully bound and acyclic. Q1: every member is a pinned standard
// unit. Q2: no member declares a top-level CONSTANT or VARIABLE, so the merged
// instancee context has no OpDeclNode and ModuleNode.isParameterFree holds
// (ModuleNode.java:357-360,384-393; Context.java:370-436 merges only
// non-local extendee entries, and declarations are never LOCAL). Q3: no member
// has both EXTENDS and INSTANCE references. Over the pinned library Q admits
// T = Integers, C(T) = {Integers, Naturals}, beyond the EXTENDS-free targets.
// The walk visits at most limit units and owns its state.
func instanceeClosureRejection(target *modulegraph.Document, limit int) string {
	closure, why := instanceeClosure(target, limit)
	if why != "" {
		return why
	}
	for _, u := range closure {
		if u.Input.Origin != modulegraph.OriginStandard || !pinnedStandardSource(u) {
			return "instancee EXTENDS closure member is not a pinned standard unit"
		}
	}
	if closureDeclaresParameters(closure) {
		return "instancee with CONSTANT or VARIABLE declarations"
	}
	if closureMixesReferenceKinds(closure) {
		return "instancee closure unit has both EXTENDS and INSTANCE"
	}
	return ""
}

// instanceeClosure returns C(T) in depth-first preorder, or the Q4 rejection.
func instanceeClosure(target *modulegraph.Document, limit int) ([]*modulegraph.Document, string) {
	const why = "instancee EXTENDS closure is unbound, cyclic or unbounded"
	if target == nil {
		return nil, why
	}
	state := map[*modulegraph.Document]int{} // 1 on the walk path, 2 finished
	var closure []*modulegraph.Document
	var visit func(*modulegraph.Document, int) bool
	visit = func(u *modulegraph.Document, depth int) bool {
		switch state[u] {
		case 1:
			return false
		case 2:
			return true
		}
		if depth > limit || len(closure) >= limit {
			return false
		}
		state[u] = 1
		closure = append(closure, u)
		for _, r := range u.References() {
			if r.Kind != modulegraph.Extends {
				continue
			}
			next := u.Linked(r.Name)
			if next == nil || !visit(next, depth+1) {
				return false
			}
		}
		state[u] = 2
		return true
	}
	if !visit(target, 0) {
		return nil, why
	}
	return closure, ""
}

// closureDeclaresParameters is effective parameter freedom over C(T): the
// instancee context holds its own declarations and those merged from every
// extendee, so any member's top-level CONSTANT or VARIABLE makes T parametric.
func closureDeclaresParameters(closure []*modulegraph.Document) bool {
	for _, u := range closure {
		if declaresParameters(u) {
			return true
		}
	}
	return false
}

// closureMixesReferenceKinds is Q3, a narrowing clause: no member of C(T)
// owns both EXTENDS and INSTANCE references.
func closureMixesReferenceKinds(closure []*modulegraph.Document) bool {
	for _, u := range closure {
		extends, instance := false, false
		for _, r := range u.References() {
			extends = extends || r.Kind == modulegraph.Extends
			instance = instance || r.Kind == modulegraph.Instance
		}
		if extends && instance {
			return true
		}
	}
	return false
}

// declaresParameters reports a top-level CONSTANT or VARIABLE statement of one
// unit. Without inner modules these are the unit's own contributions to the
// context declarations that ModuleNode.isParameterFree (ModuleNode.java:357-360)
// reads; closureDeclaresParameters adds the extendees' contributions.
func declaresParameters(d *modulegraph.Document) bool {
	for _, s := range d.RootModule.Tree.Heirs()[2].Heirs() {
		if s != nil && (s.Kind() == generated.N_ParamDeclaration || s.Kind() == generated.N_VariableDeclaration) {
			return true
		}
	}
	return false
}

// standardInstancesProved re-checks, after generation, that every top-level
// instance the unit produced took the parameter-free LOCAL copy path
// (Generator.java:5498) from an already published standard module.
func standardInstancesProved(m *moduleNode) bool {
	for _, in := range m.instances {
		if in == nil || !in.local || in.module == nil || !in.module.standard || !in.module.parameterFree() || len(in.substs) != 0 {
			return false
		}
	}
	return true
}
