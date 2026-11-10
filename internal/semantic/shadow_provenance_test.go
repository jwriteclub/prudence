// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Test schedules are derived from TLA+ SpecObj.java, SANY.java,
// Generator.java, Context.java, SimpleFilenameToStream.java and FileUtil.java,
// Copyright 2003 Compaq Corporation and 2003 Microsoft Corporation, MIT; see
// LICENSE.
package semantic

import (
	stdcontext "context"
	"fmt"
	"reflect"
	"testing"

	"github.com/jwriteclub/prudence/internal/modulegraph"
	"github.com/jwriteclub/prudence/internal/standard"
)

// Gates S3-S6 of the workspace standard-name provenance admission G1c′
// (general_driver_profile.go; the design note is in the development
// repository). Lemma R: in the phase2 and level
// harnesses Java's SimpleFilenameToStream tries the user locator (case/, the
// JVM working directory) before the classpath StandardModules entry
// (SimpleFilenameToStream.java:73-96,166-171), FileUtil.createNamedInputStream
// falls back to MonolithSpecExtractor only when both miss (FileUtil.java:
// 342-404), and SpecObj.findOrCreateParsedUnit binds each module name once
// (SpecObj.java:240-270). The adapter labels the staged file Workspace with
// Path N.tla. The flag isStandardModule (SimpleFilenameToStream.java:302-305)
// reaches only ModuleNode.setStandard, which nothing in tla2sany reads, and
// Phase2Probe's syntax membership (Phase2Probe.java:158). So a workspace
// standard-name unit generates exactly as any other workspace unit, and with
// identical bytes exactly as the standard unit: only syntax membership may
// differ. UID tables are derived by hand under the rules listed at the top of
// general_driver_profile_test.go, or by equality with an already-proved
// standard-profile schedule, never read from Go output.

// shadowWorkspace relabels the pinned embedded source as a workspace file, as
// the adapter does for a staged case/N.tla with the JAR's bytes.
func shadowWorkspace(name string) modulegraph.Input {
	in := pinnedStandard(name)
	in.Origin = modulegraph.OriginWorkspace
	return in
}

// ---- S3: mixed schedule over a workspace Naturals with a distinct body. ----

// The GD-H1 graph with F renamed Naturals: Naturals is a workspace unit with
// a simple body, not the embedded bytes. Java reads case/Naturals.tla
// (user locator, lib=false) and never the JAR entry, so the schedule is H1's.
var s3Deps = map[string]string{
	"A":        "INSTANCE E\nAdef == Edef",
	"B":        "INSTANCE A\nBdef == Adef",
	"E":        "EXTENDS Naturals\nEdef == Ndef",
	"Naturals": "Ndef == TRUE",
}

func TestShadowProvenanceS3MixedWorkspaceNaturals(t *testing.T) {
	a := gdAcquire(t, h1Root, s3Deps)
	if got := unitNames(a.Units); !reflect.DeepEqual(got, []string{"R", "A", "B", "E", "Naturals"}) {
		t.Fatalf("acquisition %v", got)
	}
	nat := a.Units[4]
	if nat.Input.Origin != modulegraph.OriginWorkspace || nat.Input.Path != "Naturals.tla" || nat.Input.View != nil || string(nat.Input.Text) != "---- MODULE Naturals ----\nNdef == TRUE\n====\n" || standard.Lookup("Naturals") == nil {
		t.Fatalf("workspace Naturals input %+v", nat.Input)
	}
	edges := map[string]string{"R": "E:A E:B ", "A": "I:E ", "B": "I:A ", "E": "E:Naturals ", "Naturals": ""}
	for _, d := range a.Units {
		if edgeText(d) != edges[d.Name] {
			t.Fatalf("%s edges %q, want %q", d.Name, edgeText(d), edges[d.Name])
		}
	}
	java := checkGDReplay(t, a)
	if want := []string{"R E:A", "R E:B", "A I:E", "E EI:Naturals", "B I:A"}; !reflect.DeepEqual(java.selections, want) {
		t.Fatalf("selections %v, want %v", java.selections, want)
	}
	if !reflect.DeepEqual(java.order, []string{"Naturals", "E", "A", "B", "R"}) {
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
		if !reflect.DeepEqual(generated, []string{"Naturals", "E", "A", "B", "R"}) {
			t.Fatalf("generation order %v", generated)
		}
		N, E, A, B, R := gdModule(t, r, "Naturals"), gdModule(t, r, "E"), gdModule(t, r, "A"), gdModule(t, r, "B"), gdModule(t, r, "R")
		if N.standard {
			t.Fatal("workspace Naturals carries the standard bit")
		}
		ndef, edef, adef, bdef, rdef := gdDef(t, N, "Ndef"), gdDef(t, E, "Edef"), gdDef(t, A, "Adef"), gdDef(t, B, "Bdef"), gdDef(t, R, "Rdef")
		rbody := gdBody(t, rdef)
		var rows []gdRow
		rows = append(rows, gdQuartet(300)...)
		rows = append(rows, gdRow{304, "ModuleNode", N}, gdRow{305, "OpApplNode", ndef.body}, gdRow{306, "OpDefNode", ndef})
		rows = append(rows, gdQuartet(307)...)
		rows = append(rows, gdRow{311, "ModuleNode", E}, gdRow{312, "OpApplNode", edef.body}, gdRow{313, "OpDefNode", edef})
		rows = append(rows, gdQuartet(314)...)
		rows = append(rows, gdRow{318, "ModuleNode", A}, gdRow{319, "SubstInNode", nil}, gdRow{320, "InstanceNode", A.instances[0]}, gdRow{321, "OpApplNode", adef.body}, gdRow{322, "OpDefNode", adef})
		rows = append(rows, gdQuartet(323)...)
		rows = append(rows, gdRow{327, "ModuleNode", B}, gdRow{328, "SubstInNode", nil}, gdRow{329, "InstanceNode", B.instances[0]}, gdRow{330, "OpApplNode", bdef.body}, gdRow{331, "OpDefNode", bdef})
		rows = append(rows, gdQuartet(332)...)
		rows = append(rows, gdRow{336, "ModuleNode", R}, gdRow{337, "OpApplNode", rbody.operands[0]}, gdRow{338, "OpApplNode", rbody.operands[1]}, gdRow{339, "OpApplNode", rbody}, gdRow{340, "OpDefNode", rdef})
		checkGDTable(t, r.h, rows, 341)
		if gdDef(t, A, "Edef") != edef || gdDef(t, A, "Ndef") != ndef || gdDef(t, B, "Adef") != adef || gdDef(t, B, "Edef") != edef ||
			gdDef(t, R, "Ndef") != ndef || gdDef(t, R, "Edef") != edef || gdDef(t, R, "Adef") != adef || gdDef(t, R, "Bdef") != bdef {
			t.Fatal("shared definitions were copied")
		}
		if rbody.operands[0].(*opApplNode).operator != adef || rbody.operands[1].(*opApplNode).operator != bdef || gdBody(t, edef).operator != ndef {
			t.Fatal("operators bound to the wrong definitions")
		}
		// No embedded Naturals operator is in scope: the workspace body
		// replaced the standard source entirely.
		if symbolByText(R.context, "Nat") != nil || symbolByText(E.context, "..") != nil {
			t.Fatal("embedded Naturals definitions leaked into the workspace graph")
		}
		orderE := keepNames(javaHashtableOrder(withPrefix("Ndef", "Edef")), []string{"Ndef", "Edef"})
		orderA := keepNames(javaHashtableOrder(withPrefix(append(append([]string{}, orderE...), "Adef")...)), []string{"Ndef", "Edef", "Adef"})
		for _, c := range []struct {
			m      *moduleNode
			insert []string
		}{
			{N, withPrefix("Ndef")},
			{E, withPrefix("Ndef", "Edef")},
			{A, withPrefix(append(append([]string{}, orderE...), "Adef")...)},
			{B, withPrefix(append(append([]string{}, orderA...), "Bdef")...)},
			{R, withPrefix(append(append([]string{}, orderE...), "Adef", "Bdef", "Rdef")...)},
		} {
			if got, want := goContextKeys(c.m.context), gdContextKeys(c.insert); !reflect.DeepEqual(got, want) {
				t.Fatalf("%s context keys\n got %v\nwant %v", c.m.name.text, got, want)
			}
		}
		// Every older profile rejects; the standard predicate on the shadow.
		if r.trace.linearRejection != "unit with multiple module references" || r.trace.flatRejection != "non-EXTENDS module reference" || r.trace.standardWhy != "workspace unit shadows a standard module" || r.trace.rootNested.why != "root unit has no inner modules" {
			t.Fatalf("older rejections %+v", r.trace)
		}
	}
	o := observeLevelCandidate(stdcontext.Background(), a, 1000)
	if o.err != nil || !o.bound || o.cause.driverProfile != generalDriverProfile || o.cause.driverRejection != "none" || o.cause.historyState != "exact" || !reflect.DeepEqual(o.cause.boundaries, []string{"supported", "supported", "supported", "supported", "supported"}) {
		t.Fatalf("observation %+v", o.cause)
	}
}

// The root itself may carry a standard name at any path: Java opens the root
// by its relative case path and binds it under its module name before any
// import is resolved (SpecObj.java:240-270), as acquisition binds it in byName.
func TestShadowProvenanceS3StandardNamedRoot(t *testing.T) {
	deps := map[string]modulegraph.Input{"A": workspaceModule("A", "Ax == TRUE"), "B": workspaceModule("B", "Bx == TRUE")}
	for _, path := range []string{"Naturals.tla", "cases/x/Naturals.tla"} {
		t.Run(path, func(t *testing.T) {
			root := workspaceModule("Naturals", "EXTENDS A\nINSTANCE B\nNx == Ax")
			root.Path = path
			a := acquireOverStandards(t, root, deps)
			if got := unitNames(a.Units); !reflect.DeepEqual(got, []string{"Naturals", "A", "B"}) || edgeText(a.Root) != "E:A I:B " {
				t.Fatalf("acquisition %v edges %q", got, edgeText(a.Root))
			}
			java := checkGDReplay(t, a)
			if !reflect.DeepEqual(java.order, []string{"A", "B", "Naturals"}) {
				t.Fatalf("order %v", java.order)
			}
			for _, level := range []bool{false, true} {
				r := runGD(t, a, level)
				if len(r.details) != 0 {
					t.Fatalf("diagnostics %+v", r.details)
				}
				A, B, N := gdModule(t, r, "A"), gdModule(t, r, "B"), gdModule(t, r, "Naturals")
				ax, bx, nx := gdDef(t, A, "Ax"), gdDef(t, B, "Bx"), gdDef(t, N, "Nx")
				if gdDef(t, N, "Bx") != bx || gdDef(t, N, "Ax") != ax || gdBody(t, nx).operator != ax || r.root != N {
					t.Fatal("sharing or root publication")
				}
				var rows []gdRow
				rows = append(rows, gdQuartet(300)...)
				rows = append(rows, gdRow{304, "ModuleNode", A}, gdRow{305, "OpApplNode", ax.body}, gdRow{306, "OpDefNode", ax})
				rows = append(rows, gdQuartet(307)...)
				rows = append(rows, gdRow{311, "ModuleNode", B}, gdRow{312, "OpApplNode", bx.body}, gdRow{313, "OpDefNode", bx})
				rows = append(rows, gdQuartet(314)...)
				rows = append(rows, gdRow{318, "ModuleNode", N}, gdRow{319, "SubstInNode", nil}, gdRow{320, "InstanceNode", N.instances[0]}, gdRow{321, "OpApplNode", nx.body}, gdRow{322, "OpDefNode", nx})
				checkGDTable(t, r.h, rows, 323)
			}
		})
	}
}

// The D-02e0aabc2e65 shape: the root EXTENDS Naturals, TLC; FiniteSets is a
// staged workspace file with bytes other than the JAR's. Java reads Naturals,
// TLC and Sequences from the JAR and TLC's LOCAL INSTANCE FiniteSets from
// case/FiniteSets.tla; FiniteSets' own instances reuse the existing units.
const s3dFiniteSets = "LOCAL INSTANCE Naturals\nLOCAL INSTANCE Sequences\nIsFiniteSet(S) == TRUE\nCardinality(S) == TRUE"

func TestShadowProvenanceS3D02e0aabc2e65Shape(t *testing.T) {
	root := workspaceModule("Root", "EXTENDS Naturals, TLC\nRdef == TRUE")
	fs := workspaceModule("FiniteSets", s3dFiniteSets)
	a := acquireOverStandards(t, root, map[string]modulegraph.Input{"FiniteSets": fs})
	if got := unitNames(a.Units); !reflect.DeepEqual(got, []string{"Root", "Naturals", "TLC", "Sequences", "FiniteSets"}) {
		t.Fatalf("acquisition %v", got)
	}
	edges := map[string]string{"Root": "E:Naturals E:TLC ", "Naturals": "", "TLC": "I:Naturals I:Sequences I:FiniteSets ", "Sequences": "I:Naturals ", "FiniteSets": "I:Naturals I:Sequences "}
	for _, d := range a.Units {
		if edgeText(d) != edges[d.Name] {
			t.Fatalf("%s edges %q, want %q", d.Name, edgeText(d), edges[d.Name])
		}
		wantOrigin := modulegraph.OriginStandard
		if d.Name == "Root" || d.Name == "FiniteSets" {
			wantOrigin = modulegraph.OriginWorkspace
		}
		if d.Input.Origin != wantOrigin {
			t.Fatalf("%s origin %d", d.Name, d.Input.Origin)
		}
	}
	java := checkGDReplay(t, a)
	if !reflect.DeepEqual(java.order, []string{"Naturals", "Sequences", "FiniteSets", "TLC", "Root"}) {
		t.Fatalf("order %v", java.order)
	}
	// The reference: the same root over the pinned FiniteSets is the main
	// standard shape, admitted by the standard LOCAL INSTANCE profile whose
	// schedule is proved by TestStandardLocalInstanceProfileSchedule.
	ref := acquireOverStandards(t, root, nil)
	clones := standardCloneOrders()
	for _, level := range []bool{false, true} {
		rr := runProfile(t, ref, level, standardLocalInstanceProfile)
		r := runGD(t, a, level)
		if len(r.details) != 0 || len(rr.details) != 0 {
			t.Fatalf("diagnostics %+v / %+v", r.details, rr.details)
		}
		if r.trace.standardWhy != "workspace unit shadows a standard module" {
			t.Fatalf("standard rejection %q", r.trace.standardWhy)
		}
		refModule := func(name string) uint32 {
			got := rr.h.reuse(gdModule(t, rr, name))
			if !got.known {
				t.Fatalf("reference %s has no module ticket", name)
			}
			return got.uid
		}
		fsStart, tlcStart := refModule("FiniteSets")-4, refModule("TLC")-4
		refByUID := historyByUID(t, rr.h)
		N, S, F, T, R := gdModule(t, r, "Naturals"), gdModule(t, r, "Sequences"), gdModule(t, r, "FiniteSets"), gdModule(t, r, "TLC"), gdModule(t, r, "Root")
		var rows []gdRow
		// Naturals and Sequences: the reference prefix, class for class.
		for uid := uint32(300); uid < fsStart; uid++ {
			rows = append(rows, gdRow{uid, refByUID[uid].ticket.class, nil})
		}
		rows[refModule("Naturals")-300].node = N
		rows[refModule("Sequences")-300].node = S
		// FiniteSets, by hand: quartet, module, each LOCAL INSTANCE template,
		// the Generator.java:5504 LOCAL clones in the instancee's Hashtable
		// order and the InstanceNode, then each definition's formal, body and
		// OpDefNode.
		k := fsStart
		rows = append(rows, gdQuartet(k)...)
		rows = append(rows, gdRow{k + 4, "ModuleNode", F})
		k += 5
		for i, target := range []string{"Naturals", "Sequences"} {
			rows = append(rows, gdRow{k, "SubstInNode", nil})
			k++
			for _, name := range clones[target] {
				rows = append(rows, gdRow{k, "OpDefNode", symbolByText(F.context, name)})
				k++
			}
			rows = append(rows, gdRow{k, "InstanceNode", F.instances[i]})
			k++
		}
		for _, name := range []string{"IsFiniteSet", "Cardinality"} {
			d := gdDef(t, F, name)
			rows = append(rows, gdRow{k, "FormalParamNode", d.params[0]}, gdRow{k + 1, "OpApplNode", d.body}, gdRow{k + 2, "OpDefNode", d})
			k += 3
		}
		// TLC and Root: the reference suffix shifted by the FiniteSets
		// difference. FiniteSets keeps its two non-LOCAL names in the same
		// insertion order, so TLC's LOCAL INSTANCE FiniteSets clones the same
		// keys in the same Hashtable order.
		delta := k - tlcStart
		final := rr.h.count - uint64(tlcStart) + uint64(k)
		for uid := tlcStart; uint64(uid) < rr.h.count; uid++ {
			rows = append(rows, gdRow{uid + delta, refByUID[uid].ticket.class, nil})
		}
		rdef := gdDef(t, R, "Rdef")
		for i := range rows {
			switch rows[i].uid {
			case refModule("TLC") + delta:
				rows[i].node = T
			case refModule("Root") + delta:
				rows[i].node = R
			case uint32(final) - 2:
				rows[i].node = rdef.body
			case uint32(final) - 1:
				rows[i].node = rdef
			}
		}
		if refModule("Root")+delta != uint32(final)-3 {
			t.Fatalf("Root segment: module %d final %d", refModule("Root")+delta, final)
		}
		checkGDTable(t, r.h, rows, final)
		if gdDef(t, T, "IsFiniteSet").source != gdDef(t, F, "IsFiniteSet") || gdDef(t, R, "Print") != gdDef(t, T, "Print") {
			t.Fatal("TLC's FiniteSets clone or Root's merge")
		}
	}
}

// ---- S4: origin-blind differential on identical bytes. ----

type shadowTwinView struct {
	admission, first string
	exact            bool
	count            uint64
	rows             []string
	order            []string
	contexts         map[string][]javaTableKey
	details          []Detail
}

func describeHistoryNode(n any) string {
	switch v := n.(type) {
	case *moduleNode:
		return "module " + v.name.text
	case *userOpDefNode:
		return fmt.Sprintf("def %s local=%v", v.name.text, v.local)
	default:
		return fmt.Sprintf("%T", n)
	}
}

func shadowTwinRun(t *testing.T, a *modulegraph.Acquisition, level bool) shadowTwinView {
	t.Helper()
	var trace flatProfileTrace
	details, _, root, roots, err := analyzeFlatExternalProfile(stdcontext.Background(), a, 1000, level, nil, &trace)
	if err != nil || root == nil {
		t.Fatalf("driver: err=%v trace=%+v", err, trace)
	}
	h := &root.owner.history
	v := shadowTwinView{admission: trace.admission, first: h.firstUnknown, exact: h.exact, count: h.count, contexts: map[string][]javaTableKey{}, details: details}
	byUID := historyByUID(t, h)
	for uid := uint32(0); uint64(uid) < h.count; uid++ {
		if e, ok := byUID[uid]; ok {
			v.rows = append(v.rows, fmt.Sprintf("%d %s %d %s", uid, e.ticket.class, e.ticket.kind, describeHistoryNode(e.node)))
		}
	}
	for _, m := range roots {
		v.order = append(v.order, m.name.text)
		v.contexts[m.name.text] = goContextKeys(m.context)
	}
	return v
}

func checkShadowTwins(t *testing.T, workspace, pinned *modulegraph.Acquisition, admissions [2]string) {
	t.Helper()
	for _, level := range []bool{false, true} {
		w, p := shadowTwinRun(t, workspace, level), shadowTwinRun(t, pinned, level)
		if w.admission != admissions[0] || p.admission != admissions[1] {
			t.Fatalf("admissions %q / %q, want %v", w.admission, p.admission, admissions)
		}
		w.admission, p.admission = "", ""
		// Every pair here is exact and diagnostic-free, so the comparison
		// covers the whole constructor history rather than a shared prefix.
		if !w.exact || len(w.details) != 0 {
			t.Fatalf("level=%v: twin not exact (first %q) or has diagnostics %+v", level, w.first, w.details)
		}
		if !reflect.DeepEqual(w, p) {
			t.Fatalf("level=%v: workspace and standard twins differ\nworkspace %+v\nstandard  %+v", level, w, p)
		}
	}
	wo, po := observeLevelCandidate(stdcontext.Background(), workspace, 1000), observeLevelCandidate(stdcontext.Background(), pinned, 1000)
	if wo.err != nil || po.err != nil || wo.cause.driverProfile != admissions[0] || po.cause.driverProfile != admissions[1] {
		t.Fatalf("observations %+v / %+v", wo.cause, po.cause)
	}
	wo.cause.driverProfile, po.cause.driverProfile = "", ""
	if !reflect.DeepEqual(wo.cause, po.cause) || !reflect.DeepEqual(wo.generation, po.generation) || !reflect.DeepEqual(wo.level, po.level) ||
		wo.examined != po.examined || wo.finished != po.finished || wo.bound != po.bound || wo.levelComplete != po.levelComplete || wo.generationComplete != po.generationComplete || len(wo.attempts) != len(po.attempts) {
		t.Fatalf("level observations differ\nworkspace %+v\nstandard  %+v", wo, po)
	}
	for i := range wo.attempts {
		x, y := wo.attempts[i], po.attempts[i]
		if x.module.name.text != y.module.name.text || x.entered != y.entered || x.completed != y.completed || x.skipped != y.skipped || x.correct != y.correct || (x.err == nil) != (y.err == nil) {
			t.Fatalf("attempt %d differs: %+v / %+v", i, x, y)
		}
	}
}

func TestShadowProvenanceS4OriginBlindDifferential(t *testing.T) {
	// The Json fixture: root Json (a standard name) over workspace Naturals,
	// Sequences, FiniteSets and TLC carrying the embedded bytes, against the
	// same root over the pinned standard units. Both twins reach the general
	// driver: the standard predicate rejects the workspace root named Json.
	json := shadowWorkspace("Json")
	staged := map[string]modulegraph.Input{}
	for _, n := range []string{"Naturals", "Sequences", "FiniteSets", "TLC"} {
		staged[n] = shadowWorkspace(n)
	}
	checkShadowTwins(t, acquireOverStandards(t, json, staged), acquireOverStandards(t, json, nil), [2]string{generalDriverProfile, generalDriverProfile})
	// The D-02e0aabc2e65 shape with the embedded FiniteSets bytes: the
	// workspace twin is admitted by the general driver, the standard twin by
	// the standard LOCAL INSTANCE profile, and every output still agrees.
	root := workspaceModule("Root", "EXTENDS Naturals, TLC\nRdef == TRUE")
	checkShadowTwins(t, acquireOverStandards(t, root, map[string]modulegraph.Input{"FiniteSets": shadowWorkspace("FiniteSets")}), acquireOverStandards(t, root, nil), [2]string{generalDriverProfile, standardLocalInstanceProfile})
	// A mixed user graph whose only shadow is a dependency's EXTENDS target.
	mixed := map[string]modulegraph.Input{"A": workspaceModule("A", "EXTENDS Naturals\nAx == 1 + 2"), "B": workspaceModule("B", "Bx == TRUE")}
	withShadow := map[string]modulegraph.Input{"Naturals": shadowWorkspace("Naturals")}
	for k, v := range mixed {
		withShadow[k] = v
	}
	mroot := workspaceModule("R", "EXTENDS A\nINSTANCE B\nRx == Ax")
	checkShadowTwins(t, acquireOverStandards(t, mroot, withShadow), acquireOverStandards(t, mroot, mixed), [2]string{generalDriverProfile, generalDriverProfile})
}

// ---- S5: negatives keep their first reason. ----

func TestShadowProvenanceS5Negatives(t *testing.T) {
	unspecified := pinnedStandard("Naturals")
	unspecified.Origin = modulegraph.OriginUnspecified
	unspecifiedDistinct := workspaceModule("Naturals", "Ndef == TRUE")
	unspecifiedDistinct.Origin = modulegraph.OriginUnspecified
	relabelled := shadowWorkspace("Naturals")
	relabelled.Path = "lib/Naturals.tla"
	nestedShadow := workspaceModule("Naturals", "---- MODULE I ----\nX == 1\n====\nNdef == TRUE")
	alteredSequences := pinnedStandard("Sequences")
	alteredSequences.Text = append(append([]byte{}, alteredSequences.Text...), '\n')
	plainA := workspaceModule("A", "X == 1")
	unspecifiedRoot := workspaceModule("Naturals", "EXTENDS A\nINSTANCE B")
	unspecifiedRoot.Origin = modulegraph.OriginUnspecified
	for _, tc := range []struct {
		name      string
		root      string
		rootInput *modulegraph.Input
		staged    map[string]modulegraph.Input
		why       string
	}{
		{name: "unspecified shadow, embedded bytes", root: "EXTENDS Naturals\nINSTANCE A", staged: map[string]modulegraph.Input{"Naturals": unspecified, "A": plainA}, why: "unspecified unit shadows a standard module"},
		{name: "unspecified shadow, distinct bytes", root: "EXTENDS Naturals\nINSTANCE A", staged: map[string]modulegraph.Input{"Naturals": unspecifiedDistinct, "A": plainA}, why: "unspecified unit shadows a standard module"},
		{name: "unspecified root with a standard name", rootInput: &unspecifiedRoot, staged: map[string]modulegraph.Input{"A": plainA, "B": workspaceModule("B", "Y == 1")}, why: "unspecified unit shadows a standard module"},
		{name: "workspace shadow labelled lib/Naturals.tla", root: "EXTENDS Naturals\nINSTANCE A", staged: map[string]modulegraph.Input{"Naturals": relabelled, "A": plainA}, why: "workspace unit shadows a standard module"},
		{name: "admissible shadow with G2", root: "EXTENDS Naturals\nINSTANCE A", staged: map[string]modulegraph.Input{"Naturals": nestedShadow, "A": plainA}, why: "nested module definitions in dependency"},
		{name: "unspecified shadow precedes G2", root: "EXTENDS A\nINSTANCE Naturals", staged: map[string]modulegraph.Input{"Naturals": unspecified, "A": workspaceModule("A", "---- MODULE I ----\nX == 1\n====")}, why: "unspecified unit shadows a standard module"},
		{name: "unpinned standard precedes a shadow", root: "EXTENDS TLC\nINSTANCE A", staged: map[string]modulegraph.Input{"Sequences": alteredSequences, "FiniteSets": shadowWorkspace("FiniteSets"), "A": plainA}, why: "unpinned standard unit"},
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
	// The private level telemetry keeps its five clauses for a shadow every
	// profile still rejects.
	relabelledFS := shadowWorkspace("FiniteSets")
	relabelledFS.Path = "lib/FiniteSets.tla"
	o := observeLevelCandidate(stdcontext.Background(), acquireOverStandards(t, workspaceModule("Root", "EXTENDS TLC\nINSTANCE Naturals"), map[string]modulegraph.Input{"FiniteSets": relabelledFS}), 1000)
	if want := "linear: unit with multiple module references; flat-extends: non-EXTENDS module reference; standard-local-instance: workspace unit shadows a standard module; root-nested: root unit has no inner modules; general-driver: workspace unit shadows a standard module"; o.cause.driverRejection != want || o.cause.historyState != "unprofiled" || o.bound {
		t.Fatalf("cause %+v", o.cause)
	}
	// A monolith-extracted unit named like a standard module: the resolver
	// here prefers the root's sibling, which the adapter never does, so the
	// view reaches the predicate and G1a rejects it first.
	src := []byte("---- MODULE R ----\nEXTENDS Naturals\nINSTANCE A\n====\n---- MODULE Naturals ----\nNdef == TRUE\n====\n")
	rootInput := modulegraph.Input{Path: "R.tla", Origin: modulegraph.OriginWorkspace, Text: src}
	budget := 1 << 20
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), rootInput, func(ctx stdcontext.Context, n string) (modulegraph.Input, error) {
		if n == "A" {
			return plainA, nil
		}
		view, found, e := modulegraph.ExtractMonolith(ctx, rootInput, n, &budget)
		if e != nil || !found {
			t.Fatalf("monolith %s: found=%v err=%v", n, found, e)
		}
		return view, nil
	}, standardProfileLimits)
	if err != nil {
		t.Fatal(err)
	}
	if d := a.Units[1]; d.Name != "Naturals" || d.Input.View == nil || d.Input.Origin != modulegraph.OriginWorkspace {
		t.Fatalf("monolith shadow input %+v", d.Input)
	}
	var trace flatProfileTrace
	if _, _, _, _, _ = analyzeFlatExternalProfile(stdcontext.Background(), a, 1000, false, nil, &trace); trace.admission != "rejected" || trace.generalDriver != "monolith-extracted unit" {
		t.Fatalf("monolith shadow: %+v", trace)
	}

	// The predicate directly, on the admitted S3 graph restored after each
	// mutation: origin, label and view of the shadow, and the root clause.
	g := gdAcquire(t, h1Root, s3Deps)
	order, err := g.Order(stdcontext.Background())
	if err != nil {
		t.Fatal(err)
	}
	docs := map[*modulegraph.Unit]*modulegraph.Document{}
	for _, d := range g.Units {
		docs[d.Unit] = d
	}
	bg := stdcontext.Background()
	nat := g.Units[4]
	check := func(label, want string, mutate func(), restore func()) {
		t.Helper()
		mutate()
		got := generalDriverRejection(bg, g, order, docs)
		restore()
		if got != want {
			t.Fatalf("%s: %q, want %q", label, got, want)
		}
	}
	check("admitted", "", func() {}, func() {})
	check("unspecified", "unspecified unit shadows a standard module", func() { nat.Input.Origin = modulegraph.OriginUnspecified }, func() { nat.Input.Origin = modulegraph.OriginWorkspace })
	check("relabelled", "workspace unit shadows a standard module", func() { nat.Input.Path = "lib/Naturals.tla" }, func() { nat.Input.Path = "Naturals.tla" })
	check("case variant label", "workspace unit shadows a standard module", func() { nat.Input.Path = "naturals.tla" }, func() { nat.Input.Path = "Naturals.tla" })
	check("view", "monolith-extracted unit", func() { nat.Input.View = &modulegraph.SourceView{} }, func() { nat.Input.View = nil })
	check("standard origin, distinct bytes", "unpinned standard unit", func() { nat.Input.Origin = modulegraph.OriginStandard }, func() { nat.Input.Origin = modulegraph.OriginWorkspace })
	if why := generalDriverRejection(bg, g, order, docs); why != "" {
		t.Fatalf("restored fixture rejected: %q", why)
	}
}

// ---- S6: held controls and the flipped strict negatives. ----

func TestShadowProvenanceS6HeldAndFlipped(t *testing.T) {
	plainA := workspaceModule("A", "X == 1")
	// The old strict G1c negatives (general_driver_profile_test.go before
	// this admission) are now admitted, with exact history.
	for _, tc := range []struct {
		name, root string
		staged     map[string]modulegraph.Input
	}{
		{"workspace Naturals shadow", "EXTENDS Naturals\nINSTANCE A", map[string]modulegraph.Input{"Naturals": shadowWorkspace("Naturals"), "A": plainA}},
		{"workspace module with a standard name", "INSTANCE A\nINSTANCE Bags", map[string]modulegraph.Input{"A": plainA, "Bags": workspaceModule("Bags", "Z == 1")}},
		{"workspace shadow of a standard instancee", "EXTENDS TLC\nINSTANCE A", map[string]modulegraph.Input{"FiniteSets": shadowWorkspace("FiniteSets"), "A": plainA}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := acquireOverStandards(t, workspaceModule("R", tc.root), tc.staged)
			checkGDReplay(t, a)
			for _, level := range []bool{false, true} {
				r := runGD(t, a, level)
				if !r.trace.exact || r.trace.generalDriver != "" || len(r.details) != 0 {
					t.Fatalf("trace %+v details %+v", r.trace, r.details)
				}
			}
		})
	}
	// Older profiles keep precedence over a shadow they already admit: the
	// general driver is never consulted (no origin check exists there).
	for _, tc := range []struct {
		name, root, admission string
		staged                map[string]modulegraph.Input
	}{
		{"linear over a distinct workspace Naturals", "EXTENDS Naturals", "linear", map[string]modulegraph.Input{"Naturals": workspaceModule("Naturals", "Ndef == TRUE")}},
		{"flat-extends over workspace Naturals bytes", "EXTENDS Naturals, Integers", "flat-extends", map[string]modulegraph.Input{"Naturals": shadowWorkspace("Naturals")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var trace flatProfileTrace
			_, _, top, _, err := analyzeFlatExternalProfile(stdcontext.Background(), acquireOverStandards(t, workspaceModule("Root", tc.root), tc.staged), 1000, false, nil, &trace)
			if err != nil || top == nil || trace.admission != tc.admission || trace.generalDriver != "" {
				t.Fatalf("%+v", trace)
			}
		})
	}
	// Q control: the standard predicate and its post-guard are unchanged; a
	// graph it admits never reaches the general driver, including one its
	// post-guard invalidates, and a shadow still defeats it.
	var trace flatProfileTrace
	if _, _, top, _, err := analyzeFlatExternalProfile(stdcontext.Background(), acquireOverStandards(t, workspaceModule("Root", "EXTENDS TLC"), nil), 1000, false, nil, &trace); err != nil || top == nil || trace.admission != standardLocalInstanceProfile || !trace.exact {
		t.Fatalf("Q control %+v", trace)
	}
	trace = flatProfileTrace{}
	own := acquireOverStandards(t, workspaceModule("Root", "EXTENDS Sequences, FiniteSets\n---- MODULE M ----\nX == TRUE\n====\nINSTANCE M"), nil)
	if _, _, top, _, err := analyzeFlatExternalProfile(stdcontext.Background(), own, 1000, false, nil, &trace); err != nil || top == nil || trace.admission != rootNestedProfile+"/"+standardLocalInstanceProfile || trace.exact || trace.firstUnknown != "unproved standard LOCAL INSTANCE target" || trace.generalDriver != "" {
		t.Fatalf("post-guard control %+v", trace)
	}
	trace = flatProfileTrace{}
	if _, _, top, _, err := analyzeFlatExternalProfile(stdcontext.Background(), acquireOverStandards(t, workspaceModule("Root", "EXTENDS TLC"), map[string]modulegraph.Input{"FiniteSets": shadowWorkspace("FiniteSets")}), 1000, false, nil, &trace); err != nil || top == nil ||
		trace.admission != generalDriverProfile || trace.standardWhy != "workspace unit shadows a standard module" || trace.rootNested.why != "root unit has no inner modules" || !trace.exact {
		t.Fatalf("shadowed Q control %+v", trace)
	}
	// N control: a root-local nesting shape with a workspace Naturals under a
	// dependency. Root-nested still rejects with the shadow as its reduced
	// view's standard reason; the general driver now admits it.
	naturals := shadowWorkspace("Naturals")
	a := acquireOverStandards(t, workspaceModule("Root", "EXTENDS A\nINSTANCE B\n---- MODULE I ----\nX == 1\n===="), map[string]modulegraph.Input{"A": workspaceModule("A", "EXTENDS Naturals"), "B": workspaceModule("B", "Y == 1"), "Naturals": naturals})
	trace = flatProfileTrace{}
	if _, _, top, _, err := analyzeFlatExternalProfile(stdcontext.Background(), a, 1000, false, nil, &trace); err != nil || top == nil || trace.admission != generalDriverProfile || !trace.exact ||
		rootNestedCause(trace.rootNested) != "reduced view: linear: unit with multiple module references / flat-extends: non-EXTENDS module reference / standard-local-instance: workspace unit shadows a standard module" {
		t.Fatalf("N control %+v", trace)
	}
	checkGDReplay(t, a)
}
