// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package semantic

import (
	stdcontext "context"
	"errors"
	"reflect"
	"testing"

	"github.com/jwriteclub/prudence/internal/modulegraph"
	"github.com/jwriteclub/prudence/internal/standard"
)

// These tests drive the real acquisition and flat driver over the pinned
// embedded standard modules. Expected schedules are derived from the pinned
// Java sources (dc6a944):
//
//   - SpecObj.java:930-983 selects every unresolved EXTENDS before any INSTANCE;
//     SpecObj.java:312-334 then emits extendees, instancees and self in
//     postorder, skipping names already present.
//   - SANY.java:421-470 constructs one Generator per unit in that order; its
//     constructor allocates nullODN, nullOAN, nullOpArg and nullLabelNode
//     (Generator.java:2140-2145) before generateModule's ModuleNode.
//   - A top-level LOCAL INSTANCE of a parameter-free standard module allocates
//     the processSubst template (Generator.java:5273), one OpDefNode per
//     non-local definition of the instancee context in Hashtable enumeration
//     order (Generator.java:5415,5430,5498-5504), and the InstanceNode
//     (Generator.java:5623).

var standardProfileLimits = modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 1000000, MaxUnits: 100, MaxEdges: 1000, MaxNodes: 1000000, MaxSourceBytes: 1 << 20, MaxTotalBytes: 1 << 24}

var errStandardTestMissing = errors.New("module not staged")

func workspaceModule(name, body string) modulegraph.Input {
	return modulegraph.Input{Path: name + ".tla", Origin: modulegraph.OriginWorkspace, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
}

func pinnedStandard(name string) modulegraph.Input {
	return modulegraph.Input{Path: name + ".tla", Text: standard.Lookup(name), Origin: modulegraph.OriginStandard}
}

// acquireOverStandards resolves staged inputs first, then the embedded
// standard namespace, then a monolith view of the root: the adapter's order.
func acquireOverStandards(t *testing.T, root modulegraph.Input, staged map[string]modulegraph.Input) *modulegraph.Acquisition {
	t.Helper()
	budget := 1 << 20
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(ctx stdcontext.Context, n string) (modulegraph.Input, error) {
		if in, ok := staged[n]; ok {
			return in, nil
		}
		if standard.Lookup(n) != nil {
			return pinnedStandard(n), nil
		}
		if view, found, e := modulegraph.ExtractMonolith(ctx, root, n, &budget); e != nil || found {
			return view, e
		}
		return modulegraph.Input{}, errStandardTestMissing
	}, standardProfileLimits)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func runStandardProfile(t *testing.T, a *modulegraph.Acquisition, levelEnabled bool) (flatProfileTrace, *moduleNode, []*moduleNode) {
	t.Helper()
	var trace flatProfileTrace
	_, _, root, roots, err := analyzeFlatExternalProfile(stdcontext.Background(), a, 1000, levelEnabled, nil, &trace)
	if err != nil || root == nil {
		t.Fatalf("driver: err=%v trace=%+v", err, trace)
	}
	return trace, root, roots
}

// javaStringHash is String.hashCode over UTF-16 units; UniqueString.hashCode
// returns it (util/UniqueString.java:209-211 at dc6a944).
func javaStringHash(s string) int32 {
	var h int32
	for _, u := range []rune(s) {
		h = 31*h + int32(u)
	}
	return h
}

// javaHashtableOrder is an independent transcription of java.util.Hashtable
// put/addEntry/rehash and its elements() enumeration: initial capacity 11,
// load factor 0.75, rehash before insertion once count >= threshold to
// 2*capacity+1 walking old buckets high to low, head insertion, and
// enumeration from the highest bucket down, each chain from its head.
// Context.java:144 creates the table; Context.java:249-259 enumerates it.
func javaHashtableOrder(keys []string) []string {
	type entry struct {
		hash int32
		key  string
		next *entry
	}
	table := make([]*entry, 11)
	threshold, count := 8, 0
	present := map[string]bool{}
	for _, k := range keys {
		if present[k] {
			continue // put replaces the value in place
		}
		present[k] = true
		h := javaStringHash(k)
		if count >= threshold {
			old := table
			table = make([]*entry, 2*len(old)+1)
			threshold = len(table) * 3 / 4
			for i := len(old) - 1; i >= 0; i-- {
				for e := old[i]; e != nil; {
					n := e.next
					j := int(e.hash&0x7fffffff) % len(table)
					e.next = table[j]
					table[j] = e
					e = n
				}
			}
		}
		i := int(h&0x7fffffff) % len(table)
		table[i] = &entry{hash: h, key: k, next: table[i]}
		count++
	}
	var out []string
	for i := len(table) - 1; i >= 0; i-- {
		for e := table[i]; e != nil; e = e.next {
			out = append(out, e.key)
		}
	}
	return out
}

// Definition names in source order, read from the pinned module texts.
var (
	naturalsDefs   = []string{"Nat", "+", "-", "*", "^", "<", ">", "\\leq", "\\geq", "%", "\\div", ".."}
	sequencesDefs  = []string{"Seq", "Len", "\\o", "Append", "Head", "Tail", "SubSeq", "SelectSeq"}
	finiteSetsDefs = []string{"IsFiniteSet", "Cardinality"}
)

// moduleContextPrefix is the insertion sequence every external module context
// starts with: SymbolTable.java:52 duplicates the builtin context by walking
// its pair list from the newest entry (Context.java:455-475), so the builtins
// enter the new Hashtable in reverse of their global insertion order. That
// order is the Context.initialize sequence the bootstrap schedule pins.
func moduleContextPrefix() []string {
	ds := builtinDescriptors()
	out := make([]string, 0, len(ds))
	for i := len(ds) - 1; i >= 0; i-- {
		out = append(out, ds[i].text)
	}
	return out
}

// standardCloneOrders derives each instancee's copy order. After the builtin
// prefix, a module context receives its EXTENDS merges (Generator.java:2364,
// transcribed by integersContextInsertion), then its LOCAL INSTANCE copies
// (addSymbol in the OpDefNode constructor, OpDefNode.java:513-515) before its
// own definitions. Builtins and local copies are skipped when that module is
// itself instantiated (Generator.java:5430); merged extendee definitions are
// not local and are copied.
func standardCloneOrders() map[string][]string {
	ctx := func(parts ...[]string) []string {
		out := moduleContextPrefix()
		for _, p := range parts {
			out = append(out, p...)
		}
		return javaHashtableOrder(out)
	}
	nat := keepNames(ctx(naturalsDefs), naturalsDefs)
	seq := keepNames(ctx(nat, sequencesDefs), sequencesDefs)
	fs := keepNames(ctx(nat, seq, finiteSetsDefs), finiteSetsDefs)
	tlc := keepNames(ctx(nat, seq, fs, tlcDefs), tlcDefs)
	integers := keepNames(javaHashtableOrder(integersContextInsertion()), append(append([]string{}, naturalsDefs...), integersDefs...))
	tlcExt := keepNames(ctx(tlc, integers, tlcExtDefs), tlcExtDefs)
	return map[string][]string{"Naturals": nat, "Sequences": seq, "FiniteSets": fs, "TLC": tlc, "Integers": integers, "TLCExt": tlcExt}
}

func keepNames(order, keep []string) []string {
	want := map[string]bool{}
	for _, k := range keep {
		want[k] = true
	}
	var out []string
	for _, k := range order {
		if want[k] {
			out = append(out, k)
		}
	}
	return out
}

type uidEntry struct {
	node   any
	ticket javaHistoryTicket
}

func historyByUID(t *testing.T, h *javaHistory) map[uint32]uidEntry {
	t.Helper()
	out := map[uint32]uidEntry{}
	for n, tk := range h.tickets {
		if _, dup := out[tk.uid]; dup {
			t.Fatalf("uid %d attached twice", tk.uid)
		}
		out[tk.uid] = uidEntry{node: n, ticket: tk}
	}
	return out
}

type instanceBlock struct {
	target      string
	subst, inst uint32
}

type unitSchedule struct {
	name   string
	module uint32
	blocks []instanceBlock
}

// checkStandardSchedule asserts the generation order, every constructor
// quartet, contiguous unit boundaries and each LOCAL INSTANCE block.
func checkStandardSchedule(t *testing.T, h *javaHistory, roots []*moduleNode, want []unitSchedule, final uint64) {
	t.Helper()
	if !h.exact || h.count != final {
		t.Fatalf("history exact=%v count=%d first=%q, want count %d", h.exact, h.count, h.firstUnknown, final)
	}
	if len(roots) != len(want) {
		t.Fatalf("generated %d units, want %d", len(roots), len(want))
	}
	byUID := historyByUID(t, h)
	clones := standardCloneOrders()
	quartet := []struct {
		class string
		kind  int
	}{{"OpDefNode", 0}, {"OpApplNode", 9}, {"OpArgNode", 8}, {"LabelNode", 29}}
	for i, w := range want {
		m := roots[i]
		if m.name.text != w.name {
			t.Fatalf("unit %d is %s, want %s", i, m.name.text, w.name)
		}
		if got := h.reuse(m); !got.known || got.uid != w.module || got.class != "ModuleNode" || got.kind != 1 {
			t.Fatalf("%s module ticket %+v, want %d", w.name, got, w.module)
		}
		for j, q := range quartet {
			e, ok := byUID[w.module-4+uint32(j)]
			if !ok || e.ticket.class != q.class || e.ticket.kind != q.kind {
				t.Fatalf("%s constructor event %d: %+v", w.name, j, e.ticket)
			}
		}
		if i == 0 && w.module != 304 {
			t.Fatalf("first Generator does not follow the 300-event bootstrap")
		}
		if len(m.instances) != len(w.blocks) {
			t.Fatalf("%s has %d instances, want %d", w.name, len(m.instances), len(w.blocks))
		}
		for j, b := range w.blocks {
			in := m.instances[j]
			if in.module.name.text != b.target || !in.local || len(in.substs) != 0 || !in.module.standard || !in.module.instantiated {
				t.Fatalf("%s instance %d: target=%s local=%v substs=%d", w.name, j, in.module.name.text, in.local, len(in.substs))
			}
			if got := h.reuse(in); got.uid != b.inst || got.class != "InstanceNode" || got.kind != 21 {
				t.Fatalf("%s INSTANCE %s ticket %+v, want %d", w.name, b.target, got, b.inst)
			}
			if e := byUID[b.subst]; e.ticket.class != "SubstInNode" || e.ticket.kind != 13 {
				t.Fatalf("%s INSTANCE %s template %+v", w.name, b.target, e.ticket)
			}
			expected := clones[b.target]
			if uint32(len(expected)) != b.inst-b.subst-1 {
				t.Fatalf("%s INSTANCE %s: %d copies between %d and %d, source gives %d", w.name, b.target, b.inst-b.subst-1, b.subst, b.inst, len(expected))
			}
			for k, name := range expected {
				e := byUID[b.subst+1+uint32(k)]
				clone, ok := e.node.(*userOpDefNode)
				if !ok || e.ticket.class != "OpDefNode" || e.ticket.kind != 5 || clone.name.text != name {
					t.Fatalf("%s INSTANCE %s copy %d: %+v, want %s", w.name, b.target, k, e.ticket, name)
				}
				original, _ := symbolByText(in.module.context, name).(*userOpDefNode)
				if original == nil || original == clone || !clone.local || clone.source != original || clone.module != original.module || clone.body != original.body || symbolByText(m.context, name) != clone {
					t.Fatalf("%s copy %s is not the Generator.java:5504 clone", w.name, name)
				}
			}
		}
	}
}

func unitNames(units []*modulegraph.Document) []string {
	var out []string
	for _, d := range units {
		out = append(out, d.Name)
	}
	return out
}

func edgeText(d *modulegraph.Document) string {
	s := ""
	for _, e := range d.Edges() {
		k := "E"
		if e.Kind == modulegraph.Instance {
			k = "I"
		}
		s += k + ":" + e.Name + " "
	}
	return s
}

const standardProfileRoot = "EXTENDS Spec, TLC, FiniteSets\nInv == Cardinality({1}) = Len(x)"

func standardProfileGraph(t *testing.T) *modulegraph.Acquisition {
	return acquireOverStandards(t, workspaceModule("Root", standardProfileRoot), map[string]modulegraph.Input{
		"Spec": workspaceModule("Spec", "EXTENDS Naturals, Sequences\nVARIABLE x\nInit == x = <<>>"),
	})
}

// mainStandardSchedule is the measured use case: two user EXTENDS paths over
// TLC, FiniteSets, Sequences and Naturals. Module UIDs follow from the
// constructor arithmetic above plus each body's already-proved producers;
// TestStandardLocalInstanceBodiesMatchProvedProfiles cross-checks the spans.
var mainStandardSchedule = []unitSchedule{
	{name: "Naturals", module: 304},
	{name: "Sequences", module: 379, blocks: []instanceBlock{{"Naturals", 380, 393}}},
	{name: "Spec", module: 538},
	{name: "FiniteSets", module: 548, blocks: []instanceBlock{{"Naturals", 549, 562}, {"Sequences", 563, 572}}},
	{name: "TLC", module: 626, blocks: []instanceBlock{{"Naturals", 627, 640}, {"Sequences", 641, 650}, {"FiniteSets", 651, 654}}},
	{name: "Root", module: 823},
}

func TestStandardLocalInstanceProfileSchedule(t *testing.T) {
	a := standardProfileGraph(t)
	// Acquisition: every EXTENDS first (Root's, then Spec's), then INSTANCE.
	if got := unitNames(a.Units); !reflect.DeepEqual(got, []string{"Root", "Spec", "TLC", "FiniteSets", "Naturals", "Sequences"}) {
		t.Fatalf("acquisition order %v", got)
	}
	edges := map[string]string{"Root": "E:Spec E:TLC E:FiniteSets ", "Spec": "E:Naturals E:Sequences ", "TLC": "I:Naturals I:Sequences I:FiniteSets ", "FiniteSets": "I:Naturals I:Sequences ", "Naturals": "", "Sequences": "I:Naturals "}
	for _, d := range a.Units {
		if got := edgeText(d); got != edges[d.Name] {
			t.Fatalf("%s edges %q, want %q", d.Name, got, edges[d.Name])
		}
	}
	for _, level := range []bool{false, true} {
		trace, root, roots := runStandardProfile(t, a, level)
		if trace.admission != standardLocalInstanceProfile || !trace.bootstrapExact || trace.units != 6 || trace.exit != "completed" || trace.standardWhy != "" {
			t.Fatalf("trace %+v", trace)
		}
		h := &root.owner.history
		if want := map[bool]javaHistoryProfile{false: freshPhase2Profile, true: levelEnabledProfile}[level]; h.profile != want {
			t.Fatalf("profile %d, want %d", h.profile, want)
		}
		checkStandardSchedule(t, h, roots, mainStandardSchedule, 831)
		if root.owner.java.exact {
			t.Fatal("legacy direct-allocation cursor admitted a multi-unit graph")
		}
	}
}

// The unit spans of the new profile equal those of already-admitted profiles:
// Naturals alone (single unit) and the linear chain Chain -> Sequences ->
// Naturals, whose prefix is exactly the multi-unit graph's first two units.
func TestStandardLocalInstanceBodiesMatchProvedProfiles(t *testing.T) {
	single := acquireOverStandards(t, pinnedStandard("Naturals"), nil)
	trace, root, _ := runStandardProfile(t, single, false)
	if trace.admission != "linear" || !root.owner.history.exact || root.owner.history.count != 375 {
		t.Fatalf("single Naturals: %+v count=%d", trace, root.owner.history.count)
	}
	chain := acquireOverStandards(t, workspaceModule("Chain", "EXTENDS Sequences"), nil)
	trace, root, roots := runStandardProfile(t, chain, false)
	if trace.admission != "linear" {
		t.Fatalf("chain admission %+v", trace)
	}
	checkStandardSchedule(t, &root.owner.history, roots, append(append([]unitSchedule{}, mainStandardSchedule[:2]...), unitSchedule{name: "Chain", module: 538}), 539)
}

// Multiple user paths share standard targets: each unit is generated once, in
// SpecObj.calculateDependencies postorder.
func TestStandardLocalInstanceProfileUserDiamond(t *testing.T) {
	a := acquireOverStandards(t, workspaceModule("Root", "EXTENDS A, B\nR == Head(<<Cardinality({})>>)"), map[string]modulegraph.Input{
		"A": workspaceModule("A", "EXTENDS Sequences\nX == Len(<<>>)"),
		"B": workspaceModule("B", "EXTENDS FiniteSets, TLC\nY == IsFiniteSet({})"),
	})
	if got := unitNames(a.Units); !reflect.DeepEqual(got, []string{"Root", "A", "B", "Sequences", "FiniteSets", "TLC", "Naturals"}) {
		t.Fatalf("acquisition order %v", got)
	}
	trace, root, roots := runStandardProfile(t, a, false)
	if trace.admission != standardLocalInstanceProfile {
		t.Fatalf("trace %+v", trace)
	}
	var order []string
	for _, m := range roots {
		order = append(order, m.name.text)
	}
	if !reflect.DeepEqual(order, []string{"Naturals", "Sequences", "A", "FiniteSets", "TLC", "B", "Root"}) {
		t.Fatalf("generation order %v", order)
	}
	// A's body X == Len(<<>>) allocates the tuple, the application and the
	// OpDefNode (539-541), two fewer events than Spec's body in the main case,
	// so every later unit keeps its span and shifts by -2. B's and Root's
	// bodies allocate three and five events.
	want := []unitSchedule{
		mainStandardSchedule[0], mainStandardSchedule[1],
		{name: "A", module: 538},
		{name: "FiniteSets", module: 546, blocks: []instanceBlock{{"Naturals", 547, 560}, {"Sequences", 561, 570}}},
		{name: "TLC", module: 624, blocks: []instanceBlock{{"Naturals", 625, 638}, {"Sequences", 639, 648}, {"FiniteSets", 649, 652}}},
		{name: "B", module: 821},
		{name: "Root", module: 829},
	}
	checkStandardSchedule(t, &root.owner.history, roots, want, 835)
}

func TestStandardLocalInstanceLevelObservation(t *testing.T) {
	o := observeLevelCandidate(stdcontext.Background(), standardProfileGraph(t), 1000)
	c := o.cause
	if o.err != nil || !o.finished || !o.bound || !o.levelComplete || !o.generationComplete || len(o.generation) != 0 || len(o.level) != 0 {
		t.Fatalf("observation %+v", o)
	}
	if c.driverProfile != standardLocalInstanceProfile || c.driverRejection != "none" || c.historyState != "exact" || c.historyPhase != "none" || c.completion != "none" || c.exit != "completed" {
		t.Fatalf("cause %+v", c)
	}
	if len(o.attempts) != 6 || len(c.boundaries) != 6 {
		t.Fatalf("attempts %d boundaries %v", len(o.attempts), c.boundaries)
	}
	for i, a := range o.attempts {
		if !a.entered || !a.completed || !a.correct || c.boundaries[i] != "supported" || a.module.name.text != mainStandardSchedule[i].name {
			t.Fatalf("attempt %d: %+v %s", i, a, c.boundaries[i])
		}
	}
}

// Every excluded shape keeps the older rejection and stays unprofiled. The
// former "instancee with EXTENDS" control (EXTENDS TLCExt) is admitted under
// predicate Q; its shadow and unpinned-closure variants replace it here.
func TestStandardLocalInstanceControlsStayRejected(t *testing.T) {
	alteredSequences := pinnedStandard("Sequences")
	alteredSequences.Text = append(append([]byte{}, alteredSequences.Text...), '\n')
	relabelled := pinnedStandard("Sequences")
	relabelled.Path = "lib/Sequences.tla"
	shadow := pinnedStandard("FiniteSets")
	shadow.Origin = modulegraph.OriginWorkspace
	integersShadow := pinnedStandard("Integers")
	integersShadow.Origin = modulegraph.OriginWorkspace
	naturalsShadow := pinnedStandard("Naturals")
	naturalsShadow.Origin = modulegraph.OriginWorkspace
	alteredIntegers := pinnedStandard("Integers")
	alteredIntegers.Text = append(append([]byte{}, alteredIntegers.Text...), '\n')
	relabelledNaturals := pinnedStandard("Naturals")
	relabelledNaturals.Path = "lib/Naturals.tla"
	cases := []struct {
		name, root string
		staged     map[string]modulegraph.Input
		rootInput  *modulegraph.Input
		why        string
		admitted   string // set only where a later profile now admits the graph
	}{
		{name: "workspace shadow", root: "EXTENDS TLC", staged: map[string]modulegraph.Input{"FiniteSets": shadow}, why: "workspace unit shadows a standard module", admitted: generalDriverProfile},
		{name: "unpinned standard bytes", root: "EXTENDS TLC", staged: map[string]modulegraph.Input{"Sequences": alteredSequences}, why: "standard unit is not the pinned source"},
		{name: "unpinned standard label", root: "EXTENDS TLC", staged: map[string]modulegraph.Input{"Sequences": relabelled}, why: "standard unit is not the pinned source"},
		{name: "workspace shadow of instancee", root: "EXTENDS TLCExt", staged: map[string]modulegraph.Input{"Integers": integersShadow}, why: "workspace unit shadows a standard module", admitted: generalDriverProfile},
		{name: "workspace shadow in instancee closure", root: "EXTENDS TLCExt", staged: map[string]modulegraph.Input{"Naturals": naturalsShadow}, why: "workspace unit shadows a standard module", admitted: generalDriverProfile},
		{name: "unpinned instancee bytes", root: "EXTENDS TLCExt", staged: map[string]modulegraph.Input{"Integers": alteredIntegers}, why: "standard unit is not the pinned source"},
		{name: "unpinned instancee closure label", root: "EXTENDS TLCExt", staged: map[string]modulegraph.Input{"Naturals": relabelledNaturals}, why: "standard unit is not the pinned source"},
		// The standard predicate still rejects each user INSTANCE below with the
		// same first reason. The general driver, consulted after all four older
		// profiles, now admits them: workspace and pinned standard units only,
		// no inner modules (G0-G3).
		{name: "user LOCAL INSTANCE of an extending standard", root: "EXTENDS TLCExt\nLOCAL INSTANCE Integers", why: "INSTANCE outside a pinned standard unit", admitted: generalDriverProfile},
		{name: "user LOCAL INSTANCE with inherited parameters", root: "EXTENDS TLC\nLOCAL INSTANCE RealTime", why: "INSTANCE outside a pinned standard unit", admitted: generalDriverProfile},
		{name: "user INSTANCE", root: "EXTENDS TLC\nINSTANCE Naturals", why: "INSTANCE outside a pinned standard unit", admitted: generalDriverProfile},
		{name: "user LOCAL INSTANCE", root: "EXTENDS TLC\nLOCAL INSTANCE Sequences", why: "INSTANCE outside a pinned standard unit", admitted: generalDriverProfile},
		{name: "user named INSTANCE", root: "EXTENDS TLC\nN == INSTANCE Naturals", why: "INSTANCE outside a pinned standard unit", admitted: generalDriverProfile},
		{name: "user parameterized INSTANCE", root: "EXTENDS TLC\nI(p) == INSTANCE Naturals", why: "INSTANCE outside a pinned standard unit", admitted: generalDriverProfile},
		{name: "user INSTANCE WITH", root: "EXTENDS FiniteSets\nCONSTANT c\nINSTANCE M WITH k <- c", staged: map[string]modulegraph.Input{"M": workspaceModule("M", "CONSTANT k")}, why: "INSTANCE outside a pinned standard unit", admitted: generalDriverProfile},
		{name: "proof-local INSTANCE", root: "EXTENDS TLC\nTHEOREM TRUE\n<1>1. INSTANCE Naturals\n<1>2. QED\n  OBVIOUS", why: "INSTANCE outside a pinned standard unit", admitted: generalDriverProfile},
		{name: "user INSTANCE in dependency", root: "EXTENDS A, TLC", staged: map[string]modulegraph.Input{"A": workspaceModule("A", "INSTANCE Naturals")}, why: "INSTANCE outside a pinned standard unit", admitted: generalDriverProfile},
		// The standard predicate still rejects this graph with the same first
		// reason. Root-local nesting, consulted after it, now admits it: Inner
		// has no references and its name is no unit's (N1-N5), Root's only
		// reference is the acquired E:TLC, and the reduced view without Inner
		// is the main standard shape (N6).
		{name: "nested module", root: "EXTENDS TLC\n---- MODULE Inner ----\nX == 1\n====", why: "nested module definitions", admitted: rootNestedProfile + "/" + standardLocalInstanceProfile},
		// Repeated targets lie inside the general driver's domain (§7.4).
		{name: "repeated reference", root: "EXTENDS TLC, Naturals, TLC", why: "repeated module reference", admitted: generalDriverProfile},
		{name: "monolith unit", rootInput: &modulegraph.Input{Path: "Root.tla", Origin: modulegraph.OriginWorkspace, Text: []byte("---- MODULE Root ----\nEXTENDS Wanted, TLC\n====\n---- MODULE Wanted ----\nX == 1\n====\n")}, why: "monolith-extracted unit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := workspaceModule("Root", tc.root)
			if tc.rootInput != nil {
				root = *tc.rootInput
			}
			a := acquireOverStandards(t, root, tc.staged)
			var trace flatProfileTrace
			_, _, top, _, _ := analyzeFlatExternalProfile(stdcontext.Background(), a, 1000, false, nil, &trace)
			admission := "rejected"
			if tc.admitted != "" {
				admission = tc.admitted
			}
			if trace.admission != admission || trace.standardWhy != tc.why {
				t.Fatalf("admission %q standard rejection %q, want %q %q (trace %+v)", trace.admission, trace.standardWhy, admission, tc.why, trace)
			}
			if tc.admitted != "" {
				if !trace.exact || top == nil {
					t.Fatalf("newly admitted graph lost its history: %+v", trace)
				}
				return
			}
			if trace.profile != 0 || trace.exact || (top != nil && top.owner.history.profile != 0) {
				t.Fatalf("rejected graph was profiled: %+v", trace)
			}
		})
	}
	o := observeLevelCandidate(stdcontext.Background(), acquireOverStandards(t, workspaceModule("Root", "EXTENDS TLC\nINSTANCE Naturals"), nil), 1000)
	if o.cause.driverProfile != generalDriverProfile || o.cause.driverRejection != "none" || o.cause.historyState != "exact" {
		t.Fatalf("cause %+v", o.cause)
	}
	// A workspace shadow with the resolver's label is still rejected by the
	// standard predicate; the general driver now admits it (G1c′). The
	// rejected-shadow telemetry is pinned in shadow_provenance_test.go.
	o = observeLevelCandidate(stdcontext.Background(), acquireOverStandards(t, workspaceModule("Root", "EXTENDS TLC\nINSTANCE Naturals"), map[string]modulegraph.Input{"FiniteSets": shadow}), 1000)
	if o.cause.driverProfile != generalDriverProfile || o.cause.driverRejection != "none" || o.cause.historyState != "exact" {
		t.Fatalf("cause %+v", o.cause)
	}
}

// The new predicate is consulted only after both older profiles reject.
func TestStandardLocalInstanceKeepsOlderProfiles(t *testing.T) {
	for _, tc := range []struct{ root, admission string }{
		{"EXTENDS Sequences", "linear"},
		{"EXTENDS Naturals, Integers", "flat-extends"},
		{"EXTENDS Naturals, Reals", "flat-extends"},
		{"X == 1", "linear"},
	} {
		trace, root, _ := runStandardProfile(t, acquireOverStandards(t, workspaceModule("Root", tc.root), nil), false)
		if trace.admission != tc.admission || !root.owner.history.exact {
			t.Fatalf("%s: %+v", tc.root, trace)
		}
	}
}

func TestStandardLocalInstanceShapeHelpers(t *testing.T) {
	doc := func(src string) *modulegraph.Document {
		a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), modulegraph.Input{Path: "S.tla", Text: []byte(src)}, func(stdcontext.Context, string) (modulegraph.Input, error) {
			return workspaceModule("M", "CONSTANT k\nF == k"), nil
		}, standardProfileLimits)
		if err != nil {
			t.Fatal(err)
		}
		return a.Root
	}
	d := doc("---- MODULE S ----\nCONSTANT c\nLOCAL INSTANCE M WITH k <- c\nINSTANCE M WITH k <- c\nN == INSTANCE M WITH k <- c\nLOCAL L == INSTANCE M WITH k <- c\nTHEOREM TRUE\n<1>1. INSTANCE M WITH k <- c\n<1>2. QED\n  OBVIOUS\n====\n")
	if got := topLevelLocalInstanceNames(d); len(got) != 0 {
		t.Fatalf("WITH, non-LOCAL, named and proof forms admitted: %d", len(got))
	}
	d = doc("---- MODULE S ----\nLOCAL INSTANCE M\nINSTANCE M\n====\n")
	got := topLevelLocalInstanceNames(d)
	refs := d.References()
	if len(got) != 1 || len(refs) != 2 || !got[refs[0].Tree] || got[refs[1].Tree] {
		t.Fatalf("LOCAL INSTANCE name node not isolated: %v", got)
	}
	if !declaresParameters(d.Linked("M")) || !declaresParameters(doc("---- MODULE S ----\nVARIABLE v\n====\n")) || declaresParameters(doc("---- MODULE S ----\nX == 1\n====\n")) {
		t.Fatal("declaration detection")
	}
	rt := acquireOverStandards(t, workspaceModule("Root", "EXTENDS RealTime"), nil)
	for _, u := range rt.Units {
		if want := u.Name == "RealTime"; declaresParameters(u) != want {
			t.Fatalf("%s parameters = %v", u.Name, !want)
		}
	}
	for _, name := range []string{"Naturals", "Sequences", "FiniteSets", "TLC", "Bags", "Json", "Randomization", "Toolbox"} {
		a := acquireOverStandards(t, workspaceModule("Root", "EXTENDS "+name), nil)
		for _, u := range a.Units {
			if u.Name == name && (declaresParameters(u) || !pinnedStandardSource(u)) {
				t.Fatalf("%s is not a pinned parameter-free standard", name)
			}
		}
	}
}

// The post-generation guard rejects instances outside the proved copy path.
func TestStandardInstancesProvedGuard(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want bool
	}{
		{"---- MODULE R ----\nX == 1\n====\n", true},
		{"---- MODULE R ----\n---- MODULE I ----\nX == 1\n====\nLOCAL INSTANCE I\n====\n", false},
		{"---- MODULE R ----\n---- MODULE I ----\nCONSTANT k\nX == k\n====\nCONSTANT c\nLOCAL INSTANCE I WITH k <- c\n====\n", false},
	} {
		_, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, tc.src), 1000)
		if err != nil {
			t.Fatal(err)
		}
		if got := standardInstancesProved(m); got != tc.want {
			t.Fatalf("%q: %v", tc.src, got)
		}
	}
	m := &moduleNode{instances: []*instanceNode{{local: false, module: &moduleNode{standard: true}}}}
	if standardInstancesProved(m) {
		t.Fatal("non-LOCAL instance admitted")
	}
}

// Cancellation and the depth limit still stop an admitted graph before a root
// is published; a depth stop also invalidates the history.
func TestStandardLocalInstanceLimits(t *testing.T) {
	a := standardProfileGraph(t)
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	var trace flatProfileTrace
	if _, _, root, _, err := analyzeFlatExternalProfile(ctx, a, 1000, false, nil, &trace); err == nil || root != nil || trace.exact {
		t.Fatalf("cancelled run: err=%v trace=%+v", err, trace)
	}
	trace = flatProfileTrace{}
	if _, _, root, _, err := analyzeFlatExternalProfile(stdcontext.Background(), a, 4, false, nil, &trace); err == nil || root != nil || trace.admission != standardLocalInstanceProfile || trace.exact {
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
		if err == nil || root != nil || trace.units != stop {
			t.Fatalf("cancel after %d: err=%v units=%d", stop, err, trace.units)
		}
	}
}
