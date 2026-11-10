// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Test schedules are derived from TLA+ SpecObj.java, SANY.java,
// Generator.java, Context.java and SymbolTable.java, Copyright 2003 Compaq
// Corporation and 2003 Microsoft Corporation, MIT; see LICENSE.
package semantic

import (
	stdcontext "context"
	stderrors "errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jwriteclub/prudence/internal/modulegraph"
	"github.com/jwriteclub/prudence/internal/standard"
)

// Gates GD-H1..H7 of the general driver profile (general_driver_profile.go;
// the design note is in the development repository).
// Every UID table below is derived by hand from the pinned dc6a944 sources,
// not read from Go output:
//
//   - bootstrap 0-299, then per unit in Order the Generator quartet nullODN,
//     nullOAN, nullOpArg, nullLabelNode (Generator.java:2140-2145), the
//     ModuleNode (:2192), then its body in text order (operands before their
//     application, the OpDefNode after its body, :2660, :2674);
//   - EXTENDS merges Context pairs only (Generator.java:2336-2379,
//     Context.java:370-442) and allocates nothing; an identical node already
//     present is skipped silently (Context.java:400);
//   - INSTANCE allocates the processSubst template SubstInNode (:5411,
//     :5273), one OpApplNode per implicit substitution, then per definition of
//     the instancee context in Hashtable order (:5415, :5430): a parameter-free
//     instancee shares non-LOCAL definitions with no UID (:5508-5509, and
//     SymbolTable.java:131 makes an identical addSymbol a no-op); a parametric
//     instancee shares a definition whose origin is parameter-free
//     (:5475-5476) and wraps and clones one whose origin is parametric
//     (:5482-5486); then the InstanceNode (:5623);
//   - an inner module allocates only its ModuleNode (:2238-2263), no quartet.

func gdAcquire(t *testing.T, root string, deps map[string]string) *modulegraph.Acquisition {
	t.Helper()
	staged := map[string]modulegraph.Input{}
	for n, b := range deps {
		staged[n] = workspaceModule(n, b)
	}
	return acquireOverStandards(t, workspaceModule("R", root), staged)
}

type gdRun struct {
	trace   flatProfileTrace
	root    *moduleNode
	roots   []*moduleNode
	details []Detail
	h       *javaHistory
}

func runGD(t *testing.T, a *modulegraph.Acquisition, level bool) gdRun {
	t.Helper()
	return runProfile(t, a, level, generalDriverProfile)
}

func runProfile(t *testing.T, a *modulegraph.Acquisition, level bool, admission string) gdRun {
	t.Helper()
	var r gdRun
	var err error
	r.details, _, r.root, r.roots, err = analyzeFlatExternalProfile(stdcontext.Background(), a, 1000, level, nil, &r.trace)
	if err != nil || r.root == nil {
		t.Fatalf("driver: err=%v trace=%+v", err, r.trace)
	}
	if r.trace.admission != admission {
		t.Fatalf("admission %q, want %q (trace %+v)", r.trace.admission, admission, r.trace)
	}
	r.h = &r.root.owner.history
	return r
}

func gdModule(t *testing.T, r gdRun, name string) *moduleNode {
	t.Helper()
	return moduleByName(t, r.roots, name)
}

func gdDef(t *testing.T, m *moduleNode, name string) *userOpDefNode {
	t.Helper()
	d, ok := symbolByText(m.context, name).(*userOpDefNode)
	if !ok || d == nil {
		t.Fatalf("%s has no definition %s", m.name.text, name)
	}
	return d
}

func gdBody(t *testing.T, d *userOpDefNode) *opApplNode {
	t.Helper()
	b, ok := d.body.(*opApplNode)
	if !ok {
		t.Fatalf("%s body is %T", d.name.text, d.body)
	}
	return b
}

// gdRow is one constructor event: its class and, where reachable, the Go
// node that must hold it.
type gdRow struct {
	uid   uint32
	class string
	node  any
}

func gdQuartet(base uint32) []gdRow {
	return []gdRow{{base, "OpDefNode", nil}, {base + 1, "OpApplNode", nil}, {base + 2, "OpArgNode", nil}, {base + 3, "LabelNode", nil}}
}

// checkGDTable asserts the complete table: every event from 300 to final-1
// is listed, attached with the listed class, to the listed node.
func checkGDTable(t *testing.T, h *javaHistory, rows []gdRow, final uint64) {
	t.Helper()
	if !h.exact || h.count != final {
		t.Fatalf("history exact=%v count=%d first=%q, want count %d", h.exact, h.count, h.firstUnknown, final)
	}
	byUID := historyByUID(t, h)
	listed := map[uint32]bool{}
	for _, r := range rows {
		if listed[r.uid] {
			t.Fatalf("uid %d listed twice", r.uid)
		}
		listed[r.uid] = true
		e, ok := byUID[r.uid]
		if !ok || e.ticket.class != r.class {
			t.Fatalf("uid %d: %+v, want %s", r.uid, e.ticket, r.class)
		}
		if r.node != nil && e.node != r.node {
			t.Fatalf("uid %d (%s) attached to %T %p, want %p", r.uid, r.class, e.node, e.node, r.node)
		}
	}
	for uid := uint32(300); uint64(uid) < final; uid++ {
		if !listed[uid] {
			t.Fatalf("uid %d missing from the derived table (%+v)", uid, byUID[uid].ticket)
		}
	}
}

func gdContextKeys(insertion []string) []javaTableKey {
	return javaHashtableKeyOrder(symbolKeys(insertion))
}

func withPrefix(names ...string) []string {
	return append(moduleContextPrefix(), names...)
}

// ---- The Java-driver replay (I1-I5, I10). ----

// gdSelection splits a replay selection "path KIND:target".
func gdSelection(s string) (owner, kind, target string) {
	f := strings.Fields(s)
	k := strings.SplitN(f[1], ":", 2)
	return strings.SplitN(f[0], "!", 2)[0], k[0], k[1]
}

func gdUnitNames(a *modulegraph.Acquisition, units []*modulegraph.Unit) []string {
	out := []string{}
	for _, u := range units {
		for _, d := range a.Units {
			if d.Unit == u {
				out = append(out, d.Name)
			}
		}
	}
	return out
}

func gdModulePaths(m *modulegraph.ModuleRef, prefix string, out map[string]*modulegraph.ModuleRef) {
	path := m.Name
	if prefix != "" {
		path = prefix + "!" + m.Name
	}
	out[path] = m
	for _, c := range m.Children {
		gdModulePaths(c, path, out)
	}
}

func gdRefPath(m *modulegraph.ModuleRef) string {
	if m.Parent == nil {
		return m.Name
	}
	return gdRefPath(m.Parent) + "!" + m.Name
}

// checkGDReplay runs the test-only transcription of SpecObj.loadSpec
// (replayNested: fields extentionFound/instantiationFound, identity vectors,
// the appendNoRepeats cycle test and calculateDependencies without a visited
// set) and checks it against Go's real acquisition, vectors and Order.
func checkGDReplay(t *testing.T, a *modulegraph.Acquisition) *nestedReplay {
	t.Helper()
	java := replayNested(t, a, false, false)
	// I1: acquisition and per-unit selections equal Go's units and edges.
	if !reflect.DeepEqual(java.acquired, unitNames(a.Units)) {
		t.Fatalf("I1: replayed acquisition %v, Go %v", java.acquired, unitNames(a.Units))
	}
	genuine := map[string][]string{}
	stale := map[string][]string{}
	phaseI := map[string]bool{}
	for _, s := range java.selections {
		owner, kind, target := gdSelection(s)
		switch kind {
		case "E", "EI":
			// I4: no genuine EXTENDS after a genuine INSTANCE of the same unit.
			if phaseI[owner] {
				t.Fatalf("I4: %s has EXTENDS %s after an INSTANCE (%v)", owner, target, java.selections)
			}
			genuine[owner] = append(genuine[owner], "E:"+target)
			if kind == "EI" {
				stale[owner] = append(stale[owner], target)
			}
		case "I":
			phaseI[owner] = true
			genuine[owner] = append(genuine[owner], "I:"+target)
		default:
			t.Fatalf("selection %q", s)
		}
	}
	for _, d := range a.Units {
		var goEdges []string
		for _, e := range d.Edges() {
			k := "E:"
			if e.Kind == modulegraph.Instance {
				k = "I:"
			}
			goEdges = append(goEdges, k+e.Name)
		}
		if !reflect.DeepEqual(genuine[d.Name], goEdges) {
			t.Fatalf("I1: %s genuine selections %v, Go edges %v", d.Name, genuine[d.Name], goEdges)
		}
		// I3: ext_J = ext_G after the mutation; inst_J = S(u) ++ inst_G.
		u := java.units[d.Name]
		ext, inst := []string{}, []string{}
		for _, x := range u.extendees {
			ext = append(ext, x.name)
		}
		for _, x := range u.instancees {
			inst = append(inst, x.name)
		}
		goExt, goInst := gdUnitNames(a, d.Unit.Extendees()), gdUnitNames(a, d.Unit.Instancees())
		wantInst := append(append([]string{}, stale[d.Name]...), goInst...)
		if !reflect.DeepEqual(ext, goExt) || !reflect.DeepEqual(inst, wantInst) {
			t.Fatalf("I3: %s Java ext %v inst %v; Go ext %v inst %v stale %v", d.Name, ext, inst, goExt, goInst, stale[d.Name])
		}
		for _, s := range stale[d.Name] {
			if !javaNameIn(ext, s) {
				t.Fatalf("I4: %s stale extra %s is not its own extendee", d.Name, s)
			}
		}
	}
	// I2: every module context binding equals Go's, for each reference and
	// each key the replay bound.
	refs := map[string]*modulegraph.ModuleRef{}
	for _, d := range a.Units {
		paths := map[string]*modulegraph.ModuleRef{}
		gdModulePaths(d.RootModule, "", paths)
		for p, m := range paths {
			refs[d.Name+":"+p] = m
		}
	}
	for _, p := range java.all {
		m := refs[p.unit.name+":"+p.path()]
		if m == nil {
			t.Fatalf("I2: no Go module for %s:%s", p.unit.name, p.path())
		}
		names := map[string]bool{}
		for k := range p.context {
			names[k] = true
		}
		for _, r := range m.References() {
			names[r.Name] = true
		}
		for n := range names {
			j, g := p.context[n], m.Bound(n)
			if (j == nil) != (g == nil) || (j != nil && (j.unit.name != g.Document.Name || j.path() != gdRefPath(g))) {
				t.Fatalf("I2: %s:%s binds %s to Java %v, Go %v", p.unit.name, p.path(), n, j, g)
			}
		}
	}
	// I5: semanticAnalysisVector = Order.
	units, err := a.Order(stdcontext.Background())
	if err != nil {
		t.Fatal(err)
	}
	if goOrder := gdUnitNames(a, units); !reflect.DeepEqual(java.order, goOrder) {
		t.Fatalf("I5: postorder Java %v, Go %v", java.order, goOrder)
	}
	// I10: the reversed getModules enumeration changes nothing.
	back := replayNested(t, a, false, true)
	if !reflect.DeepEqual(back.selections, java.selections) || !reflect.DeepEqual(back.contextState(false), java.contextState(false)) || !reflect.DeepEqual(back.order, java.order) {
		t.Fatal("I10: getModules enumeration order changed the replay")
	}
	return java
}

func javaNameIn(v []string, s string) bool {
	for _, x := range v {
		if x == s {
			return true
		}
	}
	return false
}

// ---- GD-H1: the §6 mixed graph. ----

var h1Deps = map[string]string{
	"A": "INSTANCE E\nAdef == Edef",
	"B": "INSTANCE A\nBdef == Adef",
	"E": "EXTENDS F\nEdef == Fdef",
	"F": "Fdef == TRUE",
}

const h1Root = "EXTENDS A, B\nRdef == Adef /\\ Bdef"

func TestGeneralDriverH1MixedSchedule(t *testing.T) {
	a := gdAcquire(t, h1Root, h1Deps)
	// Acquisition (SpecObj.java:930-983): R→A, R→B by extension; A→E by
	// instantiation; E→F by extension with instantiationFound still true
	// (stale); B→A by instantiation, reusing A.
	if got := unitNames(a.Units); !reflect.DeepEqual(got, []string{"R", "A", "B", "E", "F"}) {
		t.Fatalf("acquisition %v", got)
	}
	edges := map[string]string{"R": "E:A E:B ", "A": "I:E ", "B": "I:A ", "E": "E:F ", "F": ""}
	for _, d := range a.Units {
		if edgeText(d) != edges[d.Name] {
			t.Fatalf("%s edges %q, want %q", d.Name, edgeText(d), edges[d.Name])
		}
	}
	java := checkGDReplay(t, a)
	if want := []string{"R E:A", "R E:B", "A I:E", "E EI:F", "B I:A"}; !reflect.DeepEqual(java.selections, want) {
		t.Fatalf("selections %v, want %v", java.selections, want)
	}
	// The cycle test from A at iteration 5 mutates ext(A) = [] ++ [E]; the
	// stale extra leaves inst_J(E) = [F] and inst_G(E) = [].
	wantExt := map[string][]string{"R": {"A", "B"}, "A": {"E"}, "B": {}, "E": {"F"}, "F": {}}
	wantInst := map[string][]string{"R": {}, "A": {"E"}, "B": {"A"}, "E": {}, "F": {}}
	for _, d := range a.Units {
		if got := gdUnitNames(a, d.Unit.Extendees()); !reflect.DeepEqual(got, wantExt[d.Name]) {
			t.Fatalf("Go ext(%s) %v, want %v", d.Name, got, wantExt[d.Name])
		}
		if got := gdUnitNames(a, d.Unit.Instancees()); !reflect.DeepEqual(got, wantInst[d.Name]) {
			t.Fatalf("Go inst(%s) %v, want %v", d.Name, got, wantInst[d.Name])
		}
	}
	if got := javaUnitNamesReplay(java.units["E"].instancees); !reflect.DeepEqual(got, []string{"F"}) {
		t.Fatalf("inst_J(E) %v", got)
	}
	if !reflect.DeepEqual(java.order, []string{"F", "E", "A", "B", "R"}) {
		t.Fatalf("order %v", java.order)
	}
	for _, level := range []bool{false, true} {
		r := runGD(t, a, level)
		if len(r.details) != 0 {
			t.Fatalf("diagnostics %+v", r.details)
		}
		var generated []string
		for _, m := range r.roots {
			generated = append(generated, m.name.text)
		}
		if !reflect.DeepEqual(generated, []string{"F", "E", "A", "B", "R"}) {
			t.Fatalf("generation order %v", generated)
		}
		F, E, A, B, R := gdModule(t, r, "F"), gdModule(t, r, "E"), gdModule(t, r, "A"), gdModule(t, r, "B"), gdModule(t, r, "R")
		fdef, edef, adef, bdef, rdef := gdDef(t, F, "Fdef"), gdDef(t, E, "Edef"), gdDef(t, A, "Adef"), gdDef(t, B, "Bdef"), gdDef(t, R, "Rdef")
		rbody := gdBody(t, rdef)
		var rows []gdRow
		rows = append(rows, gdQuartet(300)...)
		rows = append(rows, gdRow{304, "ModuleNode", F}, gdRow{305, "OpApplNode", fdef.body}, gdRow{306, "OpDefNode", fdef})
		rows = append(rows, gdQuartet(307)...)
		rows = append(rows, gdRow{311, "ModuleNode", E}, gdRow{312, "OpApplNode", edef.body}, gdRow{313, "OpDefNode", edef})
		rows = append(rows, gdQuartet(314)...)
		rows = append(rows, gdRow{318, "ModuleNode", A}, gdRow{319, "SubstInNode", nil}, gdRow{320, "InstanceNode", A.instances[0]}, gdRow{321, "OpApplNode", adef.body}, gdRow{322, "OpDefNode", adef})
		rows = append(rows, gdQuartet(323)...)
		rows = append(rows, gdRow{327, "ModuleNode", B}, gdRow{328, "SubstInNode", nil}, gdRow{329, "InstanceNode", B.instances[0]}, gdRow{330, "OpApplNode", bdef.body}, gdRow{331, "OpDefNode", bdef})
		rows = append(rows, gdQuartet(332)...)
		rows = append(rows, gdRow{336, "ModuleNode", R}, gdRow{337, "OpApplNode", rbody.operands[0]}, gdRow{338, "OpApplNode", rbody.operands[1]}, gdRow{339, "OpApplNode", rbody}, gdRow{340, "OpDefNode", rdef})
		checkGDTable(t, r.h, rows, 341)
		// Sharing: the parameter-free instancee definitions and both merges
		// carry the original nodes.
		if gdDef(t, A, "Edef") != edef || gdDef(t, A, "Fdef") != fdef || gdDef(t, B, "Adef") != adef || gdDef(t, B, "Edef") != edef ||
			gdDef(t, R, "Fdef") != fdef || gdDef(t, R, "Edef") != edef || gdDef(t, R, "Adef") != adef || gdDef(t, R, "Bdef") != bdef {
			t.Fatal("shared definitions were copied")
		}
		if rbody.operands[0].(*opApplNode).operator != adef || rbody.operands[1].(*opApplNode).operator != bdef || gdBody(t, edef).operator != fdef {
			t.Fatal("operators bound to the wrong definitions")
		}
		// Context key order against the independent Hashtable transcription.
		// E: builtins, merge F (Fdef), Edef. A: builtins, E's shared
		// definitions in E's enumeration order, Adef. B likewise over A. R:
		// merge A adds orderE and Adef; merge B finds them identical
		// (Context.java:400) and adds exactly Bdef; then Rdef.
		orderE := keepNames(javaHashtableOrder(withPrefix("Fdef", "Edef")), []string{"Fdef", "Edef"})
		orderA := keepNames(javaHashtableOrder(withPrefix(append(append([]string{}, orderE...), "Adef")...)), []string{"Fdef", "Edef", "Adef"})
		for _, c := range []struct {
			m      *moduleNode
			insert []string
		}{
			{E, withPrefix("Fdef", "Edef")},
			{A, withPrefix(append(append([]string{}, orderE...), "Adef")...)},
			{B, withPrefix(append(append([]string{}, orderA...), "Bdef")...)},
			{R, withPrefix(append(append([]string{}, orderE...), "Adef", "Bdef", "Rdef")...)},
		} {
			if got, want := goContextKeys(c.m.context), gdContextKeys(c.insert); !reflect.DeepEqual(got, want) {
				t.Fatalf("%s context keys\n got %v\nwant %v", c.m.name.text, got, want)
			}
		}
	}
	// Five level boundaries, each supported with zero events.
	o := observeLevelCandidate(stdcontext.Background(), a, 1000)
	if o.err != nil || !o.bound || o.cause.driverProfile != generalDriverProfile || o.cause.historyState != "exact" || !reflect.DeepEqual(o.cause.boundaries, []string{"supported", "supported", "supported", "supported", "supported"}) {
		t.Fatalf("observation %+v", o.cause)
	}
	// Before this profile all four older predicates rejected the graph.
	r := runGD(t, a, false)
	if r.trace.linearRejection != "unit with multiple module references" || r.trace.flatRejection != "non-EXTENDS module reference" || r.trace.standardWhy != "INSTANCE outside a pinned standard unit" || r.trace.rootNested.why != "root unit has no inner modules" {
		t.Fatalf("older rejections %+v", r.trace)
	}
}

func javaUnitNamesReplay(v []*replayUnit) []string {
	out := []string{}
	for _, u := range v {
		out = append(out, u.name)
	}
	return out
}

// ---- GD-H3: user merged parametric instancee. ----

func TestGeneralDriverH3MergedParametricInstancee(t *testing.T) {
	deps := map[string]string{"A": "EXTENDS B\nCONSTANT N\nAdef == N", "B": "Bdef == TRUE", "C": "Cdef == TRUE"}
	for _, c := range []struct {
		name, root, admission string
		selections, order     []string
		shift                 uint32 // events before B: C's 7 in the mixed variant
	}{
		// The §8 fixture is a single-reference chain, so linear keeps
		// precedence; it pins the same schedule under the older profile.
		{"chain (linear)", "CONSTANT N\nINSTANCE A", "linear", []string{"R I:A", "A EI:B"}, []string{"B", "A", "R"}, 0},
		// R's extra EXTENDS C defeats every older profile; GD admits it.
		{"mixed (general driver)", "EXTENDS C\nCONSTANT N\nINSTANCE A", generalDriverProfile, []string{"R E:C", "R I:A", "A EI:B"}, []string{"C", "B", "A", "R"}, 7},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := gdAcquire(t, c.root, deps)
			java := checkGDReplay(t, a)
			// R→A by instantiation, then A→B by extension with a stale extra.
			if !reflect.DeepEqual(java.selections, c.selections) || !reflect.DeepEqual(java.order, c.order) {
				t.Fatalf("selections %v order %v", java.selections, java.order)
			}
			for _, level := range []bool{false, true} {
				r := runProfile(t, a, level, c.admission)
				if len(r.details) != 0 {
					t.Fatalf("diagnostics %+v", r.details)
				}
				B, A, R := gdModule(t, r, "B"), gdModule(t, r, "A"), gdModule(t, r, "R")
				bdef, adef := gdDef(t, B, "Bdef"), gdDef(t, A, "Adef")
				aN, rN := symbolByText(A.context, "N"), symbolByText(R.context, "N")
				inst := R.instances[0]
				if len(inst.substs) != 1 || !inst.substs[0].implicit || inst.substs[0].op != aN || inst.substs[0].expr.(*opApplNode).operator != rN {
					t.Fatalf("implicit substitution %+v", inst.substs)
				}
				clone := gdDef(t, R, "Adef")
				if clone == adef || clone.source != adef || gdDef(t, R, "Bdef") != bdef {
					t.Fatal("Adef must be wrapped and cloned, Bdef shared (Generator.java:5475-5486)")
				}
				var rows []gdRow
				rRefs := []string{"N"}
				if c.shift != 0 {
					C := gdModule(t, r, "C")
					cdef := gdDef(t, C, "Cdef")
					rows = append(rows, gdQuartet(300)...)
					rows = append(rows, gdRow{304, "ModuleNode", C}, gdRow{305, "OpApplNode", cdef.body}, gdRow{306, "OpDefNode", cdef})
					rRefs = []string{"Cdef", "N"}
				}
				k := c.shift
				rows = append(rows, gdQuartet(300+k)...)
				rows = append(rows, gdRow{304 + k, "ModuleNode", B}, gdRow{305 + k, "OpApplNode", bdef.body}, gdRow{306 + k, "OpDefNode", bdef})
				rows = append(rows, gdQuartet(307+k)...)
				rows = append(rows, gdRow{311 + k, "ModuleNode", A}, gdRow{312 + k, "OpDeclNode", aN}, gdRow{313 + k, "OpApplNode", adef.body}, gdRow{314 + k, "OpDefNode", adef})
				rows = append(rows, gdQuartet(315+k)...)
				rows = append(rows, gdRow{319 + k, "ModuleNode", R}, gdRow{320 + k, "OpDeclNode", rN}, gdRow{321 + k, "SubstInNode", nil}, gdRow{322 + k, "OpApplNode", inst.substs[0].expr},
					gdRow{323 + k, "SubstInNode", clone.body}, gdRow{324 + k, "OpDefNode", clone}, gdRow{325 + k, "InstanceNode", inst})
				checkGDTable(t, r.h, rows, 326+uint64(k))
				// Copy order over A's merged context and the resulting R context.
				orderA := keepNames(javaHashtableOrder(withPrefix("Bdef", "N", "Adef")), []string{"Bdef", "Adef"})
				if got := keepNames(contextKeyOrder(A.context), []string{"Bdef", "Adef"}); !reflect.DeepEqual(got, orderA) {
					t.Fatalf("A copy order %v, want %v", got, orderA)
				}
				if got, want := goContextKeys(A.context), gdContextKeys(withPrefix("Bdef", "N", "Adef")); !reflect.DeepEqual(got, want) {
					t.Fatalf("A context keys\n got %v\nwant %v", got, want)
				}
				if got, want := goContextKeys(R.context), gdContextKeys(withPrefix(append(rRefs, orderA...)...)); !reflect.DeepEqual(got, want) {
					t.Fatalf("R context keys\n got %v\nwant %v", got, want)
				}
			}
		})
	}
}

// ---- GD-H4: cross-kind repeated target (RFlag2). ----

func TestGeneralDriverH4CrossKindRepeated(t *testing.T) {
	a := gdAcquire(t, "EXTENDS Naturals\nINSTANCE Naturals", nil)
	if len(a.Root.References()) != 2 || edgeText(a.Root) != "E:Naturals " {
		t.Fatalf("references %d edges %q", len(a.Root.References()), edgeText(a.Root))
	}
	checkGDReplay(t, a)
	g, single, err := analyzeOwned(stdcontext.Background(), parsedModule(t, string(standard.Lookup("Naturals"))), 1000)
	if err != nil || single == nil || !g.pool.history.exact || g.pool.history.count != 375 {
		t.Fatalf("single-unit Naturals: err=%v count=%d", err, g.pool.history.count)
	}
	sUID := historyByUID(t, &g.pool.history)
	for _, level := range []bool{false, true} {
		r := runGD(t, a, level)
		if len(r.details) != 0 {
			t.Fatalf("diagnostics %+v", r.details)
		}
		N, R := gdModule(t, r, "Naturals"), gdModule(t, r, "R")
		var rows []gdRow
		for uid := uint32(300); uid < 375; uid++ {
			rows = append(rows, gdRow{uid, sUID[uid].ticket.class, nil})
		}
		rows[4].node = N
		rows = append(rows, gdQuartet(375)...)
		rows = append(rows, gdRow{379, "ModuleNode", R}, gdRow{380, "SubstInNode", nil}, gdRow{381, "InstanceNode", R.instances[0]})
		checkGDTable(t, r.h, rows, 382)
		// The INSTANCE re-adds the merged nodes: identical, so silent.
		for _, n := range naturalsDefs {
			if symbolByText(R.context, n) != symbolByText(N.context, n) {
				t.Fatalf("%s not shared", n)
			}
		}
		if R.instances[0].module != N {
			t.Fatal("instance target")
		}
	}
}

// ---- GD-H5: inner references to external units. ----

func TestGeneralDriverH5InnerReferencesToUnits(t *testing.T) {
	deps := map[string]string{"X": "XX == TRUE", "Y": "YY == TRUE"}
	inner := "---- MODULE I ----\nEXTENDS X\nIX == XX\n===="
	for _, c := range []struct {
		name, root string
		selections []string
		edges      string
		instances  int
	}{
		// C1: the inner owner acquires X by extension; R's INSTANCE Y follows.
		{"C1", "INSTANCE Y\n" + inner, []string{"R!I E:X", "R I:Y"}, "E:X I:Y ", 1},
		// C2: X is bound in I's context only, so R's INSTANCE X is selected
		// again and reuses X; Order is X, Y, R, not the textual Y, X.
		{"C2", "INSTANCE Y\nINSTANCE X\n" + inner, []string{"R!I E:X", "R I:Y", "R I:X"}, "E:X I:Y I:X ", 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := gdAcquire(t, c.root, deps)
			java := checkGDReplay(t, a)
			if !reflect.DeepEqual(java.selections, c.selections) || edgeText(a.Root) != c.edges || !reflect.DeepEqual(java.order, []string{"X", "Y", "R"}) {
				t.Fatalf("selections %v edges %q order %v", java.selections, edgeText(a.Root), java.order)
			}
			for _, level := range []bool{false, true} {
				r := runGD(t, a, level)
				if len(r.details) != 0 {
					t.Fatalf("diagnostics %+v", r.details)
				}
				X, Y, R := gdModule(t, r, "X"), gdModule(t, r, "Y"), gdModule(t, r, "R")
				I := innerModule(t, R, "I")
				xx, yy, ix := gdDef(t, X, "XX"), gdDef(t, Y, "YY"), gdDef(t, I, "IX")
				if gdBody(t, ix).operator != xx || len(R.instances) != c.instances {
					t.Fatal("inner EXTENDS X did not merge X's definition")
				}
				var rows []gdRow
				rows = append(rows, gdQuartet(300)...)
				rows = append(rows, gdRow{304, "ModuleNode", X}, gdRow{305, "OpApplNode", xx.body}, gdRow{306, "OpDefNode", xx})
				rows = append(rows, gdQuartet(307)...)
				rows = append(rows, gdRow{311, "ModuleNode", Y}, gdRow{312, "OpApplNode", yy.body}, gdRow{313, "OpDefNode", yy})
				rows = append(rows, gdQuartet(314)...)
				rows = append(rows, gdRow{318, "ModuleNode", R}, gdRow{319, "SubstInNode", nil}, gdRow{320, "InstanceNode", R.instances[0]})
				next := uint32(321)
				if c.instances == 2 {
					rows = append(rows, gdRow{321, "SubstInNode", nil}, gdRow{322, "InstanceNode", R.instances[1]})
					next = 323
				}
				// I: one ModuleNode, no quartet; the merge allocates nothing.
				rows = append(rows, gdRow{next, "ModuleNode", I}, gdRow{next + 1, "OpApplNode", ix.body}, gdRow{next + 2, "OpDefNode", ix})
				checkGDTable(t, r.h, rows, uint64(next)+3)
			}
		})
	}
}

// ---- GD-H6: the replay matrix. ----

func TestGeneralDriverH6ReplayMatrix(t *testing.T) {
	type fixture struct {
		name, admission string
		a               *modulegraph.Acquisition
	}
	single, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), modulegraph.Input{Path: "Root.tla", Text: []byte(singleNestedSource)}, nil, standardProfileLimits)
	if err != nil {
		t.Fatal(err)
	}
	fixtures := []fixture{
		{"H1", generalDriverProfile, gdAcquire(t, h1Root, h1Deps)},
		{"H2 mixed witness", generalDriverProfile, mixedWitnessAcquire(t)},
		{"H3 chain", "linear", gdAcquire(t, "CONSTANT N\nINSTANCE A", map[string]string{"A": "EXTENDS B\nCONSTANT N\nAdef == N", "B": "Bdef == TRUE"})},
		{"H3 mixed", generalDriverProfile, gdAcquire(t, "EXTENDS C\nCONSTANT N\nINSTANCE A", map[string]string{"A": "EXTENDS B\nCONSTANT N\nAdef == N", "B": "Bdef == TRUE", "C": "Cdef == TRUE"})},
		{"H4", generalDriverProfile, gdAcquire(t, "EXTENDS Naturals\nINSTANCE Naturals", nil)},
		{"H5 C1", generalDriverProfile, gdAcquire(t, "INSTANCE Y\n---- MODULE I ----\nEXTENDS X\n====", map[string]string{"X": "XX == 1", "Y": "YY == 1"})},
		{"H5 C2", generalDriverProfile, gdAcquire(t, "INSTANCE Y\nINSTANCE X\n---- MODULE I ----\nEXTENDS X\n====", map[string]string{"X": "XX == 1", "Y": "YY == 1"})},
		{"diamond", generalDriverProfile, gdAcquire(t, "EXTENDS A, B", map[string]string{"A": "EXTENDS C\nAx == Cx", "B": "INSTANCE C\nBx == Cx", "C": "Cx == TRUE"})},
		{"shared instancee", generalDriverProfile, gdAcquire(t, "INSTANCE A\nINSTANCE B", map[string]string{"A": "INSTANCE C", "B": "INSTANCE C", "C": "Cx == TRUE"})},
		{"stale chain", "linear", gdAcquire(t, "INSTANCE A", map[string]string{"A": "EXTENDS B", "B": "EXTENDS C", "C": "Cx == TRUE"})},
		{"standard main", standardLocalInstanceProfile, standardProfileGraph(t)},
		{"root-nested H1", "root-nested/linear", acquireOverStandards(t, workspaceModule("Root", "EXTENDS Naturals\n"+h1RootBody), nil)},
		{"root-nested single unit", "root-nested/linear", single},
		{"root-nested flat", "root-nested/flat-extends", acquireOverStandards(t, workspaceModule("Root", "EXTENDS A, B\n"+flatNestedBody), flatNestedStaged())},
		{"root-nested standard", "root-nested/" + standardLocalInstanceProfile, acquireOverStandards(t, workspaceModule("Root", "EXTENDS Sequences, FiniteSets\n"+standardNestedBody), nil)},
	}
	for _, c := range causeControls {
		if c.nilAcq || c.nilCtx || c.cancelled || c.depth != 1000 {
			continue
		}
		fixtures = append(fixtures, fixture{"cause " + c.name, "", causeAcquire(t, c.root)})
	}
	for _, c := range instanceeCases() {
		if c.root != "" {
			fixtures = append(fixtures, fixture{"instancee " + c.name, "", acquireOverStandards(t, workspaceModule("Root", c.root), nil)})
		}
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			checkGDReplay(t, f.a)
			if f.admission == "" {
				return
			}
			var trace flatProfileTrace
			_, _, top, _, err := analyzeFlatExternalProfile(stdcontext.Background(), f.a, 1000, false, nil, &trace)
			if err != nil || top == nil || trace.admission != f.admission {
				t.Fatalf("admission %q, want %q (err %v)", trace.admission, f.admission, err)
			}
			if f.admission == generalDriverProfile && !trace.exact {
				t.Fatalf("history %+v", trace)
			}
		})
	}
}

// ---- Precedence, exclusions, predicate bounds and limits. ----

// The four older profiles keep precedence and their post-guards: the general
// driver is never consulted for a graph they admit, including one a
// post-guard then invalidates.
func TestGeneralDriverKeepsOlderProfiles(t *testing.T) {
	for _, tc := range []struct{ root, admission string }{
		{"EXTENDS Sequences", "linear"},
		{"EXTENDS Naturals, Integers", "flat-extends"},
		{"EXTENDS TLC", standardLocalInstanceProfile},
		{"EXTENDS TLCExt", standardLocalInstanceProfile},
		{"EXTENDS Naturals\n" + h1RootBody, "root-nested/linear"},
		{"INSTANCE Naturals", "linear"},
	} {
		var trace flatProfileTrace
		_, _, top, _, err := analyzeFlatExternalProfile(stdcontext.Background(), acquireOverStandards(t, workspaceModule("Root", tc.root), nil), 1000, false, nil, &trace)
		if err != nil || top == nil || trace.admission != tc.admission || !trace.exact || trace.generalDriver != "" {
			t.Fatalf("%q: %+v", tc.root, trace)
		}
	}
	own := acquireOverStandards(t, workspaceModule("Root", "EXTENDS Sequences, FiniteSets\n---- MODULE M ----\nX == TRUE\n====\nINSTANCE M"), nil)
	var trace flatProfileTrace
	if _, _, top, _, err := analyzeFlatExternalProfile(stdcontext.Background(), own, 1000, false, nil, &trace); err != nil || top == nil {
		t.Fatal(err)
	}
	if trace.admission != rootNestedProfile+"/"+standardLocalInstanceProfile || trace.exact || trace.firstUnknown != "unproved standard LOCAL INSTANCE target" || trace.generalDriver != "" {
		t.Fatalf("post-guard trace %+v", trace)
	}
}

func TestGeneralDriverExclusionsStayRejected(t *testing.T) {
	alteredSequences := pinnedStandard("Sequences")
	alteredSequences.Text = append(append([]byte{}, alteredSequences.Text...), '\n')
	relabelledNaturals := pinnedStandard("Naturals")
	relabelledNaturals.Path = "lib/Naturals.tla"
	plainA := workspaceModule("A", "X == 1")
	nestedA := workspaceModule("A", "---- MODULE I ----\nX == 1\n====")
	for _, tc := range []struct {
		name, root string
		staged     map[string]modulegraph.Input
		rootInput  *modulegraph.Input
		why        string
	}{
		{name: "G2 dependency with inner module", root: "EXTENDS A\nINSTANCE B", staged: map[string]modulegraph.Input{"A": nestedA, "B": workspaceModule("B", "Y == 1")}, why: "nested module definitions in dependency"},
		{name: "G2 C3 foreign inner instance", root: "EXTENDS A\n---- MODULE J ----\nINSTANCE I\n====", staged: map[string]modulegraph.Input{"A": nestedA}, why: "nested module definitions in dependency"},
		{name: "G3 C4 inner module named like a unit", root: "EXTENDS Inner\n---- MODULE Inner ----\nX == 1\n====", staged: map[string]modulegraph.Input{"Inner": workspaceModule("Inner", "Y == 1")}, why: "inner module named like an acquired unit"},
		{name: "G3 inner module named like the root", root: "INSTANCE A\n---- MODULE R ----\nX == 1\n====", staged: map[string]modulegraph.Input{"A": plainA}, why: "inner module named like an acquired unit"},
		{name: "G1a monolith sibling", rootInput: &modulegraph.Input{Path: "R.tla", Origin: modulegraph.OriginWorkspace, Text: []byte("---- MODULE R ----\nEXTENDS W\nINSTANCE A\n====\n---- MODULE W ----\nY == 1\n====\n")}, staged: map[string]modulegraph.Input{"A": plainA}, why: "monolith-extracted unit"},
		{name: "G1b unpinned standard bytes", root: "EXTENDS TLC\nINSTANCE A", staged: map[string]modulegraph.Input{"Sequences": alteredSequences, "A": plainA}, why: "unpinned standard unit"},
		{name: "G1b unpinned standard label", root: "EXTENDS Naturals\nINSTANCE A", staged: map[string]modulegraph.Input{"Naturals": relabelledNaturals, "A": plainA}, why: "unpinned standard unit"},
		// The two strict G1c shadow negatives are admitted under G1c′; they
		// and the remaining shadow negatives are in shadow_provenance_test.go
		// (S5, S6).
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := workspaceModule("R", tc.root)
			if tc.rootInput != nil {
				root = *tc.rootInput
			}
			a := acquireOverStandards(t, root, tc.staged)
			var trace flatProfileTrace
			_, _, top, _, _ := analyzeFlatExternalProfile(stdcontext.Background(), a, 1000, false, nil, &trace)
			if trace.admission != "rejected" || trace.generalDriver != tc.why {
				t.Fatalf("admission %q general-driver %q, want %q (trace %+v)", trace.admission, trace.generalDriver, tc.why, trace)
			}
			if trace.profile != 0 || trace.exact || (top != nil && top.owner.history.profile != 0) {
				t.Fatalf("rejected graph was profiled: %+v", trace)
			}
		})
	}
	// Acquisition failures never reach a profile. A cycle aborts.
	_, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), workspaceModule("R", "EXTENDS A"), func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		return workspaceModule("A", "INSTANCE R"), nil
	}, standardProfileLimits)
	var cycle *modulegraph.CycleError
	if !stderrors.As(err, &cycle) {
		t.Fatalf("cycle: %v", err)
	}
	// §7.1(a): Go's propagation chain guard fails closed on two direct
	// extenders of J (nested.go), an acquisition error.
	_, err = modulegraph.AcquireFlatExternal(stdcontext.Background(), workspaceModule("R", "---- MODULE J ----\nEXTENDS C\n====\n---- MODULE K ----\nEXTENDS J\n====\n---- MODULE L ----\nEXTENDS J\n===="), func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		return workspaceModule("C", "---- MODULE I ----\n===="), nil
	}, standardProfileLimits)
	if !stderrors.Is(err, modulegraph.ErrIncomplete) {
		t.Fatalf("chain guard: %v", err)
	}
}

// The predicate directly: each remaining first reason and cancellation, on a
// test-owned acquisition restored after each mutation.
func TestGeneralDriverPredicateBounds(t *testing.T) {
	a := gdAcquire(t, "INSTANCE Y\n---- MODULE I ----\nEXTENDS X\n====", map[string]string{"X": "XX == 1", "Y": "YY == 1"})
	order, err := a.Order(stdcontext.Background())
	if err != nil {
		t.Fatal(err)
	}
	docs := map[*modulegraph.Unit]*modulegraph.Document{}
	for _, d := range a.Units {
		docs[d.Unit] = d
	}
	bg := stdcontext.Background()
	if why := generalDriverRejection(bg, a, order, docs); why != "" {
		t.Fatalf("admitted fixture rejected: %q", why)
	}
	cancelled, cancel := stdcontext.WithCancel(bg)
	cancel()
	if why := generalDriverRejection(cancelled, a, order, docs); why != rootNestedInspectionStopped {
		t.Fatalf("cancelled: %q", why)
	}
	if why := generalDriverRejection(bg, a, order[:1], docs); why != "dependency order does not cover acquired units" {
		t.Fatalf("coverage: %q", why)
	}
	if why := generalDriverRejection(bg, nil, order, docs); why != "dependency order does not cover acquired units" {
		t.Fatalf("nil acquisition: %q", why)
	}
	dep := a.Units[1]
	check := func(label, want string, mutate, restore func()) {
		t.Helper()
		mutate()
		got := generalDriverRejection(bg, a, order, docs)
		restore()
		if got != want {
			t.Fatalf("%s: %q, want %q", label, got, want)
		}
	}
	rm := dep.RootModule
	check("structure", "module structure unavailable", func() { dep.RootModule = nil }, func() { dep.RootModule = rm })
	origin := dep.Input.Origin
	check("unknown origin", "unknown source origin", func() { dep.Input.Origin = 9 }, func() { dep.Input.Origin = origin })
	name := dep.Name
	check("non-ASCII unit name", "non-ASCII module name", func() { dep.Name = "Xé" }, func() { dep.Name = name })
	check("empty unit name", "non-ASCII module name", func() { dep.Name = "" }, func() { dep.Name = name })
	check("unit name differs from header", "requested, header and unit names differ", func() { dep.Name = "Zed" }, func() { dep.Name = name })
	in := a.Root.RootModule.Children[0]
	check("non-ASCII inner name", "non-ASCII module name", func() { in.Name = "Ι" }, func() { in.Name = "I" })
	check("inner named like a unit", "inner module named like an acquired unit", func() { in.Name = "Y" }, func() { in.Name = "I" })
	if why := generalDriverRejection(bg, a, order, docs); why != "" {
		t.Fatalf("restored fixture rejected: %q", why)
	}
}

// Cancellation and the depth limit through the driver.
func TestGeneralDriverLimits(t *testing.T) {
	a := gdAcquire(t, h1Root, h1Deps)
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	var trace flatProfileTrace
	if _, _, root, _, err := analyzeFlatExternalProfile(ctx, a, 1000, false, nil, &trace); err == nil || root != nil || trace.exact {
		t.Fatalf("cancelled run: err=%v trace=%+v", err, trace)
	}
	trace = flatProfileTrace{}
	if _, _, root, _, err := analyzeFlatExternalProfile(stdcontext.Background(), a, 1, false, nil, &trace); err == nil || root != nil || trace.admission != generalDriverProfile || trace.exact || trace.firstUnknown != "depth limit" {
		t.Fatalf("depth-limited run: err=%v trace=%+v", err, trace)
	}
	for _, stop := range []int{1, 3, 5} {
		ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
		units := 0
		trace = flatProfileTrace{}
		_, _, root, _, err := analyzeFlatExternalProfile(ctx, a, 1000, false, func(*moduleNode, *Errors) error {
			if units++; units == stop {
				cancel()
			}
			return nil
		}, &trace)
		cancel()
		if stop < 5 && (err == nil || root != nil || trace.units != stop || trace.exit != "cancelled before unit") {
			t.Fatalf("cancel after %d: err=%v units=%d exit=%q", stop, err, trace.units, trace.exit)
		}
		if stop == 5 && (err != nil || root == nil) {
			t.Fatalf("cancel after the last unit: err=%v", err)
		}
	}
}

// ---- GD-H7: mixed-driver composition. ----

func gdPublications(t *testing.T, a *modulegraph.Acquisition) (pubs []string, details []Detail, trace flatProfileTrace, err error) {
	t.Helper()
	details, _, _, _, err = analyzeFlatExternalProfile(stdcontext.Background(), a, 1000, true, func(m *moduleNode, e *Errors) error {
		pubs = append(pubs, m.name.text)
		return nil
	}, &trace)
	return
}

func checkGDDetail(t *testing.T, d Detail, code int, unit string, line, col, endCol int, format string) {
	t.Helper()
	l := d.Location
	if d.Code.Value != code || l.Name != unit || l.BeginLine != line || l.BeginColumn != col || l.EndLine != line || l.EndColumn != endCol || d.Format != format {
		t.Fatalf("detail %+v, want %d %s %d:%d-%d %q", d, code, unit, line, col, endCol, format)
	}
}

func TestGeneralDriverH7Composition(t *testing.T) {
	// (a) A generator error in a mixed graph's instancee. SANY.java:452-466:
	// G is generated and published despite the error; the level check runs
	// only while semanticErrors.isSuccess(), so G's and R's are skipped.
	// Generator.java:810-813 reports SYMBOL_UNDEFINED (4200) at the name.
	a := gdAcquire(t, "EXTENDS B\nINSTANCE G\nRdef == TRUE", map[string]string{"B": "V == 2", "G": "Gx == Missing"})
	pubs, details, trace, err := gdPublications(t, a)
	if err != nil || trace.admission != generalDriverProfile || !trace.exact || !reflect.DeepEqual(pubs, []string{"B", "G", "R"}) || len(details) != 1 {
		t.Fatalf("generator error: err=%v pubs=%v trace=%+v details=%+v", err, pubs, trace, details)
	}
	checkGDDetail(t, details[0], 4200, "G", 2, 7, 13, "Unknown operator: `%s'.")
	o := observeLevelCandidate(stdcontext.Background(), a, 1000)
	checkCauseInvariants(t, "generator error", o)
	if o.err != nil || !o.finished || !reflect.DeepEqual(o.cause.boundaries, []string{"supported", "skipped", "skipped"}) || o.cause.historyState != "exact" || len(o.generation) != 1 || len(o.level) != 0 ||
		!o.attempts[0].completed || o.attempts[1].completed || o.attempts[2].completed || o.attempts[1].entered {
		t.Fatalf("generator error observation %+v", o)
	}

	// (b) An abort in the middle unit. Generator.java:443 throws the 4003
	// AbortException after the ordinary 4200 in the same unit; SANY.java:503
	// leaves the loop, so G is never published and R never generated.
	a = gdAcquire(t, "EXTENDS B\nINSTANCE G\nRdef == TRUE", map[string]string{"B": "V == 2", "G": "Gx == Missing\nUSE Expr!⟩"})
	pubs, details, trace, err = gdPublications(t, a)
	var abort *Abort
	if !stderrors.As(err, &abort) || trace.admission != generalDriverProfile || trace.exit != "generation" || trace.units != 2 || !reflect.DeepEqual(pubs, []string{"B"}) || len(details) != 2 || !sameDetail(details[1], abort.Detail) {
		t.Fatalf("abort: err=%v pubs=%v trace=%+v details=%+v", err, pubs, trace, details)
	}
	checkGDDetail(t, details[0], 4200, "G", 2, 7, 13, "Unknown operator: `%s'.")
	if details[1].Code.Value != 4003 || details[1].Location.Name != "G" || details[1].Format != "Internal error: Unexpected selector `%s'." {
		t.Fatalf("abort detail %+v", details[1])
	}
	o = observeLevelCandidate(stdcontext.Background(), a, 1000)
	checkCauseInvariants(t, "abort", o)
	if o.finished || o.bound || o.levelComplete || o.generationComplete || len(o.attempts) != 1 || !o.attempts[0].completed || o.cause.exit != "generation" || o.cause.completion != "execution unfinished" {
		t.Fatalf("abort observation %+v", o)
	}

	// (c) A carried prior level error. L's ASSUME y is a 4206
	// (AssumeNode.java:95-98) added to semanticErrors by L's level check;
	// every later unit's check is then skipped, and the history stays
	// invalidated from L's boundary on.
	a = gdAcquire(t, "EXTENDS L\nINSTANCE A\nB == TRUE", map[string]string{"L": "VARIABLE y\nASSUME y", "A": "X == 1"})
	o = observeLevelCandidate(stdcontext.Background(), a, 1000)
	checkCauseInvariants(t, "prior level error", o)
	want := levelCause{exit: "completed", driverProfile: generalDriverProfile, driverRejection: "none", historyState: "invalidated", historyFirst: historyBoundary, historyPhase: "level-boundary", historyUnit: 0, completion: "prior entered level boundary unsupported", completionUnit: 1, boundaries: []string{"level diagnostics added,errors unsuccessful", "skipped", "skipped"}, incomplete: "not-applicable", incompleteCategory: "not-applicable", attemptIncomplete: []string{"not-applicable", "not-applicable", "not-applicable"}}
	if !reflect.DeepEqual(o.cause, want) || len(o.level) != 1 || len(o.generation) != 0 || o.levelComplete || o.bound {
		t.Fatalf("prior level error\n got %+v\nwant %+v", o.cause, want)
	}
	if o.level[0].Code.Value != 4206 || o.level[0].Location.Name != "L" || o.level[0].Format != "Level error: assumptions must be level 0 (Constant), \nbut this one has level %s." {
		t.Fatalf("level detail %+v", o.level[0])
	}
	h := &o.attempts[0].module.owner.history
	if h.reuse(o.attempts[0].module).uid != 304 || h.reuse(o.attempts[1].module).known || h.reuse(o.attempts[2].module).known || o.attempts[1].completed || o.attempts[2].completed {
		t.Fatal("sticky unknown history after the level boundary")
	}

	// (d) A sticky generator invalidation in an earlier unit: P keeps its
	// prefix, no later unit gets a known ticket, and no level field claims
	// completion.
	a = gdAcquire(t, "EXTENDS P\nINSTANCE A", map[string]string{"P": "F == <<TRUE,FALSE>>\nG == F!1!Missing", "A": "X == 1"})
	r := runGD(t, a, false)
	if r.trace.exact || r.trace.firstUnknown != "unproved general identifier selector" || r.roots[0].name.text != "P" || r.h.reuse(r.roots[0]).uid != 304 || r.h.reuse(r.roots[1]).known || r.h.reuse(r.root).known {
		t.Fatalf("sticky prefix: %+v", r.trace)
	}
	o = observeLevelCandidate(stdcontext.Background(), a, 1000)
	checkCauseInvariants(t, "sticky", o)
	if o.bound || o.levelComplete || o.generationComplete || o.cause.historyFirst != "unproved general identifier selector" || o.cause.completion != "history invalidated before publication" || o.cause.completionUnit != 0 {
		t.Fatalf("sticky observation %+v", o.cause)
	}
}
