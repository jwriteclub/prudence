// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"

	"github.com/jwriteclub/prudence/internal/modulegraph"
	"github.com/jwriteclub/prudence/internal/standard"
)

// generalDriverProfile names the general driver profile GD (strict domain).
// It admits the existing scheduler itself, acquisition, Graph.Link and its
// cycle test, Order, then one Generator per unit, for any mix of user EXTENDS
// and INSTANCE edges inside the guarded domain D_G. It is consulted only
// after the linear, flat-EXTENDS, standard LOCAL INSTANCE and root-nested
// profiles all reject, and it has no post-generation guard of its own:
// producers keep every invalidation they already make, exactly as under the
// linear profile. Java citations are to the pinned dc6a944 tla2sany sources.
//
// D_G is a successful acquisition under a fresh entry and history that
// satisfies, with the provenance premise P4 (stable resolution with Java's
// selected source and standard bit; not checked here):
//
//	G0  Order covers every acquired unit; every root module and tree is
//	    present; module names are ASCII; requested, header and unit names
//	    agree (acquisition already enforces the last; it is re-read here);
//	G1  no monolith-extracted unit (G1a); every standard-origin unit is the
//	    pinned embedded source (G1b); a unit with an embedded standard name
//	    and a non-standard origin has origin Workspace, no View, and is the
//	    root or has Path exactly Name+".tla" (G1c′);
//	G2  no unit other than the root has inner modules;
//	G3  no inner module of the root has an acquired unit's name.
//
// Theorem G. On D_G, Java and Go agree on the acquisition sequence and every
// module-context binding, the extendee vectors, cycle outcomes and mutated
// vectors, semanticAnalysisVector = Order, the external table each Generator
// sees, one Generator quartet per unit and the level boundaries. Java's
// instancee vectors differ from Go's only by stale-flag extras that the same
// owner's extendee vector already holds. The lemmas:
//
//	L1  search equality: both searches are transcribed (SpecObj.java:439-512,
//	    619-702; nested.go next), and the extentionFound/instantiationFound
//	    fields (:420-424) are written but never read by a search;
//	L2  each step binds one name in the owner's context on both sides;
//	L3  propagation (SpecObj.java:787-882) binds nothing under G2, because no
//	    acquired target other than the root has inner modules and the root is
//	    never a successful target; Go returns before its chain guard;
//	L4  per-unit phase: every genuine EXTENDS edge of a unit precedes every
//	    genuine INSTANCE edge of that unit, since an INSTANCE edge follows a
//	    failed extension search that visited every module of the owner unit,
//	    and bindings are never removed. A stale extra (:956 then :961) follows
//	    its own extendee;
//	L5  vector invariant: ext_J(u) = ext_G(u), inst_J(u) = S(u) ++ inst_G(u)
//	    with S(u) the stale prefix already in ext(u), so appendNoRepeats
//	    (:381-382) mutates both sides identically;
//	L6  postorder: calculateDependencies (:312-336) emits the DFS postorder of
//	    the first-occurrence sequence ext(u) ++ inst(u), which by L4 and L5 is
//	    Go's ext_G(u) ++ inst_G(u) as Order (relationships.go) walks it;
//	L7  the driver (SANY.java:421-470) runs one Generator per unit in that
//	    order, publishes only the unit root, then level checks when errors
//	    succeed; the :425 skip never fires;
//	L8  with G2 and G3, Generator module-name resolution partitions into the
//	    root's own inner modules and the external table, as in the single-unit
//	    and root-nested cases (SymbolTable.java:104-114, :227-249);
//	L9  acquisition errors are excluded; generator and level diagnostics
//	    arise per unit in Order, aborts stop both drivers (SANY.java:503);
//	L10 the only identity-hashed enumeration (getModules, SpecObj.java:749)
//	    is effect-free by L3.
//
// P4's precedence half is discharged for the phase2 and level harnesses by
// Lemma R (shadow-provenance design note): under H1-H6 both sides
// read the staged case/N.tla before the embedded or JAR standard source. It
// is not a property of the public API, whose caller declares Origin and
// chooses the source; P4 remains an explicit premise there.
//
// The predicate reads the acquisition only. It changes no acquisition, tree,
// reference, edge, generation input or publication, invokes no analysis and
// allocates no semantic node. The inner-module walk owns its budget, bounded
// by the acquisition's visit and depth limits, and stops on cancellation.
// Unspecified or relabelled shadows, dependency nesting, monolith views and
// inner/unit name collisions stay outside this profile.
const generalDriverProfile = "general-driver"

// generalDriverRejection returns "" when the acquisition lies in D_G, else
// the first failed guard in this order: coverage and structure, G1a, G1b,
// G1c′, G2, the bounded inner(R) walk, the G0 name premises, G3.
func generalDriverRejection(ctx stdcontext.Context, acquisition *modulegraph.Acquisition, order []*modulegraph.Unit, docs map[*modulegraph.Unit]*modulegraph.Document) string {
	// G0: coverage and structure.
	if acquisition == nil || acquisition.Root == nil || len(order) != len(docs) || len(order) != len(acquisition.Units) {
		return "dependency order does not cover acquired units"
	}
	for _, d := range acquisition.Units {
		if d == nil || d.RootModule == nil || d.RootModule.Tree == nil || len(d.RootModule.Tree.Heirs()) < 3 || d.RootModule.Tree.Heirs()[2] == nil {
			return "module structure unavailable"
		}
	}
	for _, u := range order {
		if docs[u] == nil {
			return "dependency order does not cover acquired units"
		}
	}
	// G1a.
	for _, d := range acquisition.Units {
		if d.Input.View != nil {
			return "monolith-extracted unit"
		}
	}
	// G1b.
	for _, d := range acquisition.Units {
		switch d.Input.Origin {
		case modulegraph.OriginStandard:
			if !pinnedStandardSource(d) {
				return "unpinned standard unit"
			}
		case modulegraph.OriginUnspecified, modulegraph.OriginWorkspace:
		default:
			return "unknown source origin"
		}
	}
	// G1c′: a unit with an embedded standard name and a non-standard origin
	// is admitted only as the workspace source SANY itself selects. Under the
	// harness premises H1-H6 (Lemma R), Java's SimpleFilenameToStream reads
	// case/N.tla before the JAR's StandardModules entry
	// (SimpleFilenameToStream.java:73-96,166-171; FileUtil.java:342-404) and
	// SpecObj.findOrCreateParsedUnit binds each name once (SpecObj.java:
	// 240-270), exactly as the adapter labels that file Workspace with Path
	// N.tla. Unspecified provenance stays rejected; a non-root workspace
	// shadow must carry the resolver's fixed label, mirroring the label half
	// of pinnedStandardSource. Bytes are never compared or substituted.
	for _, d := range acquisition.Units {
		if d.Input.Origin == modulegraph.OriginStandard || standard.Lookup(d.Name) == nil {
			continue
		}
		if d.Input.Origin != modulegraph.OriginWorkspace {
			return "unspecified unit shadows a standard module"
		}
		if d.Input.View != nil || (d != acquisition.Root && d.Input.Path != d.Name+".tla") {
			return "workspace unit shadows a standard module"
		}
	}
	// G2.
	root := acquisition.Root
	for _, d := range acquisition.Units {
		if d != root && len(d.RootModule.Children) != 0 {
			return "nested module definitions in dependency"
		}
	}
	maxDepth, maxVisits := acquisition.InspectionLimits()
	in := &rootNestedInspection{ctx: ctx, maxDepth: maxDepth, left: maxVisits}
	if !in.spend(0) {
		return rootNestedInspectionStopped
	}
	inner, ok := in.innerModules(root.RootModule)
	if !ok {
		return rootNestedInspectionStopped
	}
	// G0: ASCII names, then requested, header and unit names.
	units := map[string]bool{}
	for _, d := range acquisition.Units {
		if !asciiModuleName(d.Name) {
			return "non-ASCII module name"
		}
		units[d.Name] = true
	}
	for _, m := range inner {
		if !asciiModuleName(m.Name) {
			return "non-ASCII module name"
		}
	}
	for _, d := range acquisition.Units {
		if d.RootModule.Name != d.Name {
			return "requested, header and unit names differ"
		}
		for _, e := range d.Edges() {
			if t := d.Linked(e.Name); t == nil || t.Name != e.Name {
				return "requested, header and unit names differ"
			}
		}
	}
	// G3.
	for _, m := range inner {
		if units[m.Name] {
			return "inner module named like an acquired unit"
		}
	}
	return ""
}
