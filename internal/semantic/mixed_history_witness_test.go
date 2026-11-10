// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"testing"

	"github.com/jwriteclub/prudence/internal/modulegraph"
)

// Mixed EXTENDS+INSTANCE witness for the bounded predicate P of the
// mixed-history design note (in the development repository). Copied from
// fixture d6c180d as gate GD-H2: the general-driver profile
// now admits the witness, and the driver's own history must equal the
// source-derived schedule that the test-only replay below reproduces.
const (
	mixedWitnessRoot = "---- MODULE MHRoot ----\nEXTENDS MHExt\nINSTANCE MHInst\nSpec == Init /\\ Next\n====\n"
	mixedWitnessExt  = "---- MODULE MHExt ----\nCONSTANT N\nVARIABLE x\nInit == x = N\n====\n"
	mixedWitnessInst = "---- MODULE MHInst ----\nCONSTANT N\nVARIABLE x\nNext == x = N\n====\n"
)

func mixedWitnessAcquire(t *testing.T) *modulegraph.Acquisition {
	t.Helper()
	files := map[string]string{"MHExt": mixedWitnessExt, "MHInst": mixedWitnessInst}
	in := func(name, text string) modulegraph.Input {
		return modulegraph.Input{Path: name + ".tla", Origin: modulegraph.OriginWorkspace, Text: []byte(text)}
	}
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), in("MHRoot", mixedWitnessRoot), func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		text, ok := files[n]
		if !ok {
			return modulegraph.Input{}, ErrPortIncomplete
		}
		return in(n, text), nil
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func mixedWitnessOrder(t *testing.T, a *modulegraph.Acquisition) []*modulegraph.Document {
	t.Helper()
	order, err := a.Order(stdcontext.Background())
	if err != nil {
		t.Fatal(err)
	}
	docs := map[*modulegraph.Unit]*modulegraph.Document{}
	for _, d := range a.Units {
		docs[d.Unit] = d
	}
	out := make([]*modulegraph.Document, 0, len(order))
	for _, u := range order {
		out = append(out, docs[u])
	}
	return out
}

// SpecObj.loadSpec acquires MHRoot, MHExt (extension search), MHInst
// (instantiation search); calculateDependencies generates MHExt, MHInst, MHRoot.
func TestMixedWitnessGeneralDriverAdmits(t *testing.T) {
	a := mixedWitnessAcquire(t)
	var acquired, generated []string
	for _, d := range a.Units {
		acquired = append(acquired, d.Name)
	}
	for _, d := range mixedWitnessOrder(t, a) {
		generated = append(generated, d.Name)
	}
	if len(acquired) != 3 || acquired[0] != "MHRoot" || acquired[1] != "MHExt" || acquired[2] != "MHInst" {
		t.Fatalf("acquisition %v", acquired)
	}
	if len(generated) != 3 || generated[0] != "MHExt" || generated[1] != "MHInst" || generated[2] != "MHRoot" {
		t.Fatalf("generation %v", generated)
	}
	refs := a.Root.References()
	if len(refs) != 2 || refs[0].Name != "MHExt" || refs[0].Kind != modulegraph.Extends || refs[1].Name != "MHInst" || refs[1].Kind != modulegraph.Instance {
		t.Fatalf("references %+v", refs)
	}
	// The general driver admits it: exact enabled-entry tickets give the
	// level checker its identities, and all three boundaries are supported.
	o := observeLevelCandidate(stdcontext.Background(), a, 1000)
	c := o.cause
	if o.err != nil || !o.finished || !o.bound || !o.levelComplete || !o.generationComplete || len(o.generation) != 0 || len(o.level) != 0 || len(o.attempts) != 3 {
		t.Fatalf("observation: %+v", o)
	}
	if c.exit != "completed" || c.driverProfile != generalDriverProfile || c.driverRejection != "none" || c.historyState != "exact" || c.completion != "none" || len(c.boundaries) != 3 || c.boundaries[0] != "supported" || c.boundaries[1] != "supported" || c.boundaries[2] != "supported" {
		t.Fatalf("cause: %+v", c)
	}
	// The driver's own history, under both profiles, is the derived schedule.
	for _, level := range []bool{false, true} {
		var trace flatProfileTrace
		_, _, root, roots, err := analyzeFlatExternalProfile(stdcontext.Background(), a, 1000, level, nil, &trace)
		if err != nil || root == nil || trace.admission != generalDriverProfile || len(roots) != 3 {
			t.Fatalf("driver: err=%v trace=%+v", err, trace)
		}
		h := &root.owner.history
		if !h.exact || h.count != 337 {
			t.Fatalf("driver history exact=%v count=%d first=%q", h.exact, h.count, h.firstUnknown)
		}
		checkMixedWitnessSchedule(t, h, root.owner, roots[0], roots[1], roots[2])
	}
}

// This replays SANY.java:421-470 in test code only: one shared history, a
// fresh Generator per unit in dependency order, publication, then a level
// check that must consume no history events. It does not call or widen the
// driver. Every UID below is derived from the pinned Java sources listed in
// MIXED-HISTORY-WITNESS-RESULT.md, not from Go output.
func TestMixedWitnessSourceDerivedSchedule(t *testing.T) {
	ctx := stdcontext.Background()
	a := mixedWitnessAcquire(t)
	pool := newNamePool()
	pool.history = freshLevelEnabledHistory(ctx)
	h := &pool.history
	identity := func(n leafNode) (uint64, uint32, error) {
		tk := h.reuse(n)
		if !h.exact || !tk.known || n.header().kind != tk.kind {
			return 0, 0, ErrPortIncomplete
		}
		return uint64(tk.uid) + 1, uint32(961+31*tk.kind) + tk.uid, nil
	}
	checker, err := newLevelChecker(ctx, identity, 1000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	external := make(map[*name]leafNode)
	errors := &Errors{}
	modules := map[string]*moduleNode{}
	published := []uint64{311, 322, 337}
	for i, d := range mixedWitnessOrder(t, a) {
		g, err := newOwnedGenerator(ctx, 1000, pool, external, errors)
		if err != nil {
			t.Fatal(err)
		}
		m, err := g.moduleExtending(d.Parse.Tree, nil)
		if err != nil {
			t.Fatal(err)
		}
		external[m.name] = m
		modules[d.Name] = m
		if !h.exact || h.count != published[i] || !errors.Success() || len(errors.Messages()) != 0 {
			t.Fatalf("%s published: exact=%v count=%d first=%q messages=%d", d.Name, h.exact, h.count, h.firstUnknown, len(errors.Messages()))
		}
		checker.errors = errors
		ok, err := checker.check(m, 1)
		if err != nil || !ok || h.count != published[i] || !h.exact || len(errors.Messages()) != 0 {
			t.Fatalf("%s level boundary: ok=%v err=%v count=%d", d.Name, ok, err, h.count)
		}
	}
	checkMixedWitnessSchedule(t, h, pool, modules["MHExt"], modules["MHInst"], modules["MHRoot"])
}

// checkMixedWitnessSchedule asserts UIDs 300-336 of the witness, derived from
// the pinned sources, over one shared history.
func checkMixedWitnessSchedule(t *testing.T, h *javaHistory, pool *namePool, ext, inst, root *moduleNode) {
	t.Helper()
	uid := func(what string, n any, class string, want uint32) {
		t.Helper()
		got := h.reuse(n)
		if !got.known || got.class != class || got.uid != want {
			t.Fatalf("%s: got %+v, want %s %d", what, got, class, want)
		}
	}
	lookup := func(m *moduleNode, s string) leafNode {
		t.Helper()
		n := m.context.lookup(contextKey{name: pool.intern(s)})
		if n == nil {
			t.Fatalf("%s missing from %s", s, m.name.text)
		}
		return n
	}
	for _, c := range []struct {
		m      *moduleNode
		module uint32
	}{{ext, 304}, {inst, 315}} {
		base := c.module
		uid("module", c.m, "ModuleNode", base)
		uid("N", lookup(c.m, "N"), "OpDeclNode", base+1)
		uid("x", lookup(c.m, "x"), "OpDeclNode", base+2)
		def := c.m.definitions[0].(*userOpDefNode)
		body := def.body.(*opApplNode)
		uid("x operand", body.operands[0], "OpApplNode", base+3)
		uid("N operand", body.operands[1], "OpApplNode", base+4)
		uid("=", body, "OpApplNode", base+5)
		uid("definition", def, "OpDefNode", base+6)
	}
	uid("root", root, "ModuleNode", 326)
	if lookup(root, "Init") != ext.definitions[0] || lookup(root, "x") != lookup(ext, "x") || lookup(root, "N") != lookup(ext, "N") {
		t.Fatal("EXTENDS did not share MHExt nodes")
	}
	// 327 is the discarded processSubst template. Hashtable order of the
	// instancee context puts x (bucket 120 of 191) before N (bucket 78).
	if len(inst.context.order) != 191 || inst.context.orderCount != 75 {
		t.Fatalf("instancee table: capacity %d entries %d", len(inst.context.order), inst.context.orderCount)
	}
	substs := root.instances[0].substs
	if len(substs) != 2 || substs[0].op != lookup(inst, "x") || substs[1].op != lookup(inst, "N") || !substs[0].implicit || !substs[1].implicit {
		t.Fatalf("implicit substitutions %+v", substs)
	}
	uid("x <- x", substs[0].expr, "OpApplNode", 328)
	uid("N <- N", substs[1].expr, "OpApplNode", 329)
	if substs[0].expr.(*opApplNode).operator != lookup(ext, "x") || substs[1].expr.(*opApplNode).operator != lookup(ext, "N") {
		t.Fatal("implicit substitution did not resolve to MHExt declarations")
	}
	clone := lookup(root, "Next").(*userOpDefNode)
	if clone == inst.definitions[0] || clone.source != inst.definitions[0] {
		t.Fatal("Next was not cloned from MHInst")
	}
	uid("Next wrapper", clone.body, "SubstInNode", 330)
	uid("Next clone", clone, "OpDefNode", 331)
	uid("instance", root.instances[0], "InstanceNode", 332)
	spec := lookup(root, "Spec").(*userOpDefNode)
	body := spec.body.(*opApplNode)
	uid("Init operand", body.operands[0], "OpApplNode", 333)
	uid("Next operand", body.operands[1], "OpApplNode", 334)
	uid("/\\", body, "OpApplNode", 335)
	uid("Spec", spec, "OpDefNode", 336)
	if body.operands[0].(*opApplNode).operator != ext.definitions[0] || body.operands[1].(*opApplNode).operator != clone {
		t.Fatal("Spec operands bound to the wrong definitions")
	}
}
