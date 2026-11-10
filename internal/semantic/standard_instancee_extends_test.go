// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package semantic

import (
	stdcontext "context"
	"reflect"
	"testing"

	"github.com/jwriteclub/prudence/internal/modulegraph"
	"github.com/jwriteclub/prudence/internal/standard"
)

// These tests cover predicate Q of the standard LOCAL INSTANCE profile: an
// INSTANCE target whose EXTENDS closure is pinned, parameter-free and has no
// unit with both EXTENDS and INSTANCE. Over the pinned library the one new
// shape is TLCExt `LOCAL INSTANCE Integers` with Integers `EXTENDS Naturals`.
// Java citations are to the pinned dc6a944 tla2sany sources:
//
//   - Generator.java:2335-2376 (processExtendsList) merges each extendee's
//     context through Context.mergeExtendContext (Context.java:370-436): the
//     extendee's pair list oldest first (reversePairList), skipping local
//     symbols and names already present, table.put for the rest.
//   - Generator.java:5415,5430,5498-5507 copy every non-builtin, non-local
//     OpDefNode of the instancee context in Hashtable enumeration order
//     (Context.java:249-259) with origin getOriginallyDefinedInModuleNode and
//     source getSource (OpDefNode.java:481-516,666-668).
//   - SpecObj.java:930-990 acquires units; the stale instantiationFound flag
//     adds the freshly selected extendee to the same owner's instancees.

// Definition names in source order, read from the pinned module texts.
var (
	tlcDefs      = []string{"Print", "PrintT", "Assert", "JavaTime", "TLCGet", "TLCSet", ":>", "@@", "Permutations", "SortSeq", "RandomElement", "Any", "ToString", "TLCEval"}
	integersDefs = []string{"Int", "-."}
	tlcExtDefs   = []string{"AssertEq", "AssertError", "TLCGetOrDefault", "TLCGetAndSet", "Trace", "CounterExample", "ToTrace", "TLCModelValue", "TLCDefer", "TLCNoOp", "PickSuccessor", "TLCCache", "TLCFP", "TLCEvalDefinition"}
)

// mergeExtendKeys transcribes the key effect of Context.mergeExtendContext
// (Context.java:370-398): walk the extendee's pair list oldest first and put
// every key the receiving table does not contain. Present keys (the shared
// builtins here) leave the table unchanged. src lists non-local entries only.
func mergeExtendKeys(dst, src []string) []string {
	present := map[string]bool{}
	for _, k := range dst {
		present[k] = true
	}
	out := append([]string{}, dst...)
	for _, k := range src {
		if !present[k] {
			present[k] = true
			out = append(out, k)
		}
	}
	return out
}

// integersContextInsertion is the Hashtable insertion sequence of Integers'
// module context: the duplicated builtins (moduleContextPrefix), then the
// EXTENDS Naturals merge, then Integers' own definitions. Naturals' context is
// a Context.duplicate of the builtins, which keeps the global pair order
// (Context.java:455-475), followed by its definitions, so its oldest-first pair
// list is the builtin descriptor order and then naturalsDefs.
func integersContextInsertion() []string {
	var naturalsPairs []string
	for _, d := range builtinDescriptors() {
		naturalsPairs = append(naturalsPairs, d.text)
	}
	naturalsPairs = append(naturalsPairs, naturalsDefs...)
	return append(mergeExtendKeys(moduleContextPrefix(), naturalsPairs), integersDefs...)
}

func moduleByName(t *testing.T, roots []*moduleNode, name string) *moduleNode {
	t.Helper()
	for _, m := range roots {
		if m.name.text == name {
			return m
		}
	}
	t.Fatalf("module %s not generated", name)
	return nil
}

func contextKeyOrder(c *context) []string {
	var out []string
	for _, v := range c.orderedValues() {
		out = append(out, v.header().name.text)
	}
	return out
}

// The three standard prefixes before TLCExt, shared by the stale-edge cases.
var tlcExtPrefix = []unitSchedule{
	{name: "Naturals", module: 304},
	{name: "Sequences", module: 379, blocks: []instanceBlock{{"Naturals", 380, 393}}},
	{name: "FiniteSets", module: 538, blocks: []instanceBlock{{"Naturals", 539, 552}, {"Sequences", 553, 562}}},
	{name: "TLC", module: 616, blocks: []instanceBlock{{"Naturals", 617, 630}, {"Sequences", 631, 640}, {"FiniteSets", 641, 644}}},
	{name: "Integers", module: 813},
}

// TLCExt's two blocks: TLC (template, 14 copies, InstanceNode) and Integers
// (template, 12 copies of Naturals' definitions and 2 of Integers', InstanceNode).
var tlcExtUnit = unitSchedule{name: "TLCExt", module: 825, blocks: []instanceBlock{{"TLC", 826, 841}, {"Integers", 842, 857}}}

func scheduleWith(prefix []unitSchedule, rest ...unitSchedule) []unitSchedule {
	return append(append([]unitSchedule{}, prefix...), rest...)
}

// instanceeCase is one source-backed real shape over the pinned library.
type instanceeCase struct {
	name, root  string
	acquisition []string
	edges       map[string]string
	schedule    []unitSchedule
	final       uint64
	stale       map[string][]string // Java-only instancee extras by owner
}

var tlcExtEdges = map[string]string{"TLCExt": "I:TLC I:Integers ", "TLC": "I:Naturals I:Sequences I:FiniteSets ", "Integers": "E:Naturals ", "Naturals": "", "Sequences": "I:Naturals ", "FiniteSets": "I:Naturals I:Sequences "}

func withEdges(extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range tlcExtEdges {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func instanceeCases() []instanceeCase {
	return []instanceeCase{
		{
			// Stale edge, new target: Integers is first reached through TLCExt's
			// INSTANCE, and its EXTENDS loads Naturals fresh (3 of 66 cases).
			name:        "stale new Naturals",
			root:        "EXTENDS TLCExt",
			acquisition: []string{"Root", "TLCExt", "TLC", "Integers", "Naturals", "Sequences", "FiniteSets"},
			edges:       withEdges(map[string]string{"Root": "E:TLCExt "}),
			schedule:    scheduleWith(tlcExtPrefix, tlcExtUnit, unitSchedule{name: "Root", module: 974}),
			final:       975,
			stale:       map[string][]string{"Integers": {"Naturals"}},
		},
		{
			// Stale edge, reused target (63 of 66 cases).
			name:        "stale reused Naturals",
			root:        "EXTENDS Naturals, TLC, TLCExt",
			acquisition: []string{"Root", "Naturals", "TLC", "TLCExt", "Sequences", "FiniteSets", "Integers"},
			edges:       withEdges(map[string]string{"Root": "E:Naturals E:TLC E:TLCExt "}),
			schedule:    scheduleWith(tlcExtPrefix, tlcExtUnit, unitSchedule{name: "Root", module: 974}),
			final:       975,
			stale:       map[string][]string{"Integers": {"Naturals"}},
		},
		{
			// No stale edge (33 cases): a user EXTENDS resolves Integers and its
			// Naturals before the first instantiation. Integers' span moves ahead
			// of Sequences, FiniteSets and TLC, which shift by +12; TLCExt's
			// blocks keep the same UIDs as in the stale cases.
			name:        "nonstale Integers first",
			root:        "EXTENDS Integers, TLC, TLCExt",
			acquisition: []string{"Root", "Integers", "TLC", "TLCExt", "Naturals", "Sequences", "FiniteSets"},
			edges:       withEdges(map[string]string{"Root": "E:Integers E:TLC E:TLCExt "}),
			schedule: []unitSchedule{
				{name: "Naturals", module: 304},
				{name: "Integers", module: 379},
				{name: "Sequences", module: 391, blocks: []instanceBlock{{"Naturals", 392, 405}}},
				{name: "FiniteSets", module: 550, blocks: []instanceBlock{{"Naturals", 551, 564}, {"Sequences", 565, 574}}},
				{name: "TLC", module: 628, blocks: []instanceBlock{{"Naturals", 629, 642}, {"Sequences", 643, 652}, {"FiniteSets", 653, 656}}},
				tlcExtUnit,
				{name: "Root", module: 974},
			},
			final: 975,
			stale: map[string][]string{},
		},
		{
			// Root EXTENDS _Possible: TLCExt is itself an instancee, with
			// Integers below it (5 of the 99 cases include _Possible).
			name:        "_Possible",
			root:        "EXTENDS _Possible",
			acquisition: []string{"Root", "_Possible", "TLC", "TLCExt", "Naturals", "Sequences", "FiniteSets", "Integers"},
			edges:       withEdges(map[string]string{"Root": "E:_Possible ", "_Possible": "I:TLC I:TLCExt I:Naturals I:Sequences "}),
			schedule: scheduleWith(tlcExtPrefix, tlcExtUnit,
				unitSchedule{name: "_Possible", module: 974, blocks: []instanceBlock{{"TLC", 975, 990}, {"TLCExt", 991, 1006}, {"Naturals", 1007, 1020}, {"Sequences", 1021, 1030}}},
				unitSchedule{name: "Root", module: 1165}),
			final: 1166,
			stale: map[string][]string{"Integers": {"Naturals"}},
		},
	}
}

func checkAcquiredShape(t *testing.T, a *modulegraph.Acquisition, c instanceeCase) {
	t.Helper()
	if got := unitNames(a.Units); !reflect.DeepEqual(got, c.acquisition) {
		t.Fatalf("acquisition order %v, want %v", got, c.acquisition)
	}
	for _, d := range a.Units {
		if got := edgeText(d); got != c.edges[d.Name] {
			t.Fatalf("%s edges %q, want %q", d.Name, got, c.edges[d.Name])
		}
	}
}

// H1. Root EXTENDS TLCExt: the merged Integers context, its copy order into
// TLCExt and the complete constructor schedule, fresh and level-enabled.
func TestStandardInstanceeExtendsIntegersCopyOrder(t *testing.T) {
	c := instanceeCases()[0]
	a := acquireOverStandards(t, workspaceModule("Root", c.root), nil)
	checkAcquiredShape(t, a, c)
	for _, level := range []bool{false, true} {
		trace, root, roots := runStandardProfile(t, a, level)
		if trace.admission != standardLocalInstanceProfile || !trace.bootstrapExact || trace.units != 7 || trace.exit != "completed" || trace.standardWhy != "" {
			t.Fatalf("trace %+v", trace)
		}
		h := &root.owner.history
		if want := map[bool]javaHistoryProfile{false: freshPhase2Profile, true: levelEnabledProfile}[level]; h.profile != want {
			t.Fatalf("profile %d, want %d", h.profile, want)
		}
		checkStandardSchedule(t, h, roots, c.schedule, c.final)
		if root.owner.java.exact {
			t.Fatal("legacy direct-allocation cursor admitted a multi-unit graph")
		}
		naturals, integers, tlcExt := moduleByName(t, roots, "Naturals"), moduleByName(t, roots, "Integers"), moduleByName(t, roots, "TLCExt")

		// The merged context: 72 builtins + 12 merged + 2 own entries, in the
		// independently transcribed Hashtable enumeration order, capacity 191.
		insertion := integersContextInsertion()
		if len(insertion) != 86 || len(moduleContextPrefix()) != 72 {
			t.Fatalf("transcribed insertion has %d entries", len(insertion))
		}
		// Control: the full enumeration discriminates insertion order (the
		// reversed sequence enumerates differently), although the 14 copy
		// names occupy distinct buckets at capacity 191, so their relative
		// order follows from the entry set and hashes alone.
		reversed := make([]string, len(insertion))
		for i, k := range insertion {
			reversed[len(insertion)-1-i] = k
		}
		if reflect.DeepEqual(javaHashtableOrder(reversed), javaHashtableOrder(insertion)) {
			t.Fatal("transcription is insensitive to insertion order")
		}
		if got, want := contextKeyOrder(integers.context), javaHashtableOrder(insertion); !reflect.DeepEqual(got, want) {
			t.Fatalf("Integers context order\n got %q\nwant %q", got, want)
		}
		if len(integers.context.order) != 191 || integers.context.orderCount != 86 {
			t.Fatalf("Integers context capacity %d count %d", len(integers.context.order), integers.context.orderCount)
		}
		if len(integers.extendees) != 1 || integers.extendees[0] != naturals || !integers.parameterFree() || !integers.standard || !integers.instantiated {
			t.Fatalf("Integers extendees %v parameterFree %v", integers.extendees, integers.parameterFree())
		}
		// Merged entries are Naturals' own nodes (shared, not copied).
		for _, n := range naturalsDefs {
			own, merged := symbolByText(naturals.context, n), symbolByText(integers.context, n)
			def, ok := merged.(*userOpDefNode)
			if !ok || own != merged || def.local || def.module != naturals || def.source != nil {
				t.Fatalf("Integers context %s is not Naturals' definition", n)
			}
		}
		for _, n := range integersDefs {
			def, ok := symbolByText(integers.context, n).(*userOpDefNode)
			if !ok || def.local || def.module != integers || def.source != nil {
				t.Fatalf("Integers own %s", n)
			}
		}

		// The Integers block: origin Naturals for 12 copies, Integers for 2;
		// every copy LOCAL, sharing body and source, bound in TLCExt.
		in := tlcExt.instances[1]
		origins := map[string]int{}
		byUID := historyByUID(t, h)
		for uid := uint32(843); uid < 857; uid++ {
			clone := byUID[uid].node.(*userOpDefNode)
			origins[clone.module.name.text]++
			if !clone.local || clone.source == nil || clone.source.local || clone.body != clone.source.body || clone.source.module != clone.module || symbolByText(tlcExt.context, clone.name.text) != clone {
				t.Fatalf("copy %s at %d", clone.name.text, uid)
			}
		}
		if in.module != integers || h.reuse(in).uid != 857 {
			t.Fatal("TLCExt's second instance is not Integers' InstanceNode")
		}
		if !reflect.DeepEqual(origins, map[string]int{"Naturals": 12, "Integers": 2}) {
			t.Fatalf("copy origins %v", origins)
		}
		// TLCExt's own definitions stay non-local; the copies stay local.
		for _, n := range tlcExtDefs {
			if def, ok := symbolByText(tlcExt.context, n).(*userOpDefNode); !ok || def.local || def.module != tlcExt || def.source != nil {
				t.Fatalf("TLCExt own %s", n)
			}
		}
	}

	// Level boundaries: every unit enters, completes and is supported, and the
	// boundary check allocates nothing, so the level-enabled history stays exact.
	o := observeLevelCandidate(stdcontext.Background(), a, 1000)
	cause := o.cause
	if o.err != nil || !o.finished || !o.bound || !o.levelComplete || !o.generationComplete || len(o.generation) != 0 || len(o.level) != 0 {
		t.Fatalf("observation %+v", o)
	}
	if cause.driverProfile != standardLocalInstanceProfile || cause.driverRejection != "none" || cause.historyState != "exact" || cause.historyPhase != "none" || cause.completion != "none" || cause.exit != "completed" {
		t.Fatalf("cause %+v", cause)
	}
	if len(o.attempts) != 7 || len(cause.boundaries) != 7 {
		t.Fatalf("attempts %d boundaries %v", len(o.attempts), cause.boundaries)
	}
	for i, at := range o.attempts {
		if !at.entered || !at.completed || !at.correct || cause.boundaries[i] != "supported" || at.module.name.text != c.schedule[i].name {
			t.Fatalf("attempt %d: %+v %s", i, at, cause.boundaries[i])
		}
	}
}

// The remaining real shapes, including the nonstale and _Possible cases.
func TestStandardInstanceeExtendsCases(t *testing.T) {
	for _, c := range instanceeCases() {
		t.Run(c.name, func(t *testing.T) {
			a := acquireOverStandards(t, workspaceModule("Root", c.root), nil)
			checkAcquiredShape(t, a, c)
			trace, root, roots := runStandardProfile(t, a, false)
			if trace.admission != standardLocalInstanceProfile || trace.standardWhy != "" || trace.exit != "completed" || trace.units != len(c.schedule) {
				t.Fatalf("trace %+v", trace)
			}
			checkStandardSchedule(t, &root.owner.history, roots, c.schedule, c.final)
		})
	}
}

// Every standard unit keeps the span it has under an already-proved profile:
// Naturals, Sequences, FiniteSets and TLC as in mainStandardSchedule, and
// Integers as in the linear chain Root EXTENDS Integers. Only TLCExt's span
// (its two blocks and its own bodies) is new, and is identical across cases.
func TestStandardInstanceeExtendsSpansMatchProvedProfiles(t *testing.T) {
	spans := func(s []unitSchedule, final uint64) map[string]uint64 {
		out := map[string]uint64{}
		for i, u := range s {
			next := final
			if i+1 < len(s) {
				next = uint64(s[i+1].module)
			}
			out[u.name] = next - uint64(u.module)
		}
		return out
	}
	want := spans(mainStandardSchedule, 831)
	trace, root, roots := runStandardProfile(t, acquireOverStandards(t, workspaceModule("Root", "EXTENDS Integers"), nil), false)
	if trace.admission != "linear" {
		t.Fatalf("linear Integers: %+v", trace)
	}
	linear := []unitSchedule{{name: "Naturals", module: 304}, {name: "Integers", module: 379}, {name: "Root", module: 391}}
	checkStandardSchedule(t, &root.owner.history, roots, linear, 392)
	want["Integers"] = spans(linear, 392)["Integers"]
	var tlcExtSpan uint64
	for _, c := range instanceeCases() {
		got := spans(c.schedule, c.final)
		for _, n := range []string{"Naturals", "Sequences", "FiniteSets", "TLC", "Integers"} {
			if got[n] != want[n] {
				t.Fatalf("%s: %s span %d, proved profile gives %d", c.name, n, got[n], want[n])
			}
		}
		if tlcExtSpan == 0 {
			tlcExtSpan = got["TLCExt"]
		} else if got["TLCExt"] != tlcExtSpan {
			t.Fatalf("%s: TLCExt span %d, other cases %d", c.name, got["TLCExt"], tlcExtSpan)
		}
	}
}

// javaParseUnit and javaSpecReplay are a bounded, test-local transcription of
// SpecObj.java:423-424 (the flag fields), :430-511 and :613-701 (the two
// searches, without inner modules), :776-777 and :787-791 (bindIfNotBound in
// the owner's context; inner-module propagation is inert without children),
// :930-990 (the loop, stale flag included), :354-404 (the cycle test, which
// mutates the extendee vector with appendNoRepeats by identity) and :312-336
// (calculateDependencies). Module name lists come from the acquired documents'
// references, in syntax order.
type javaParseUnit struct {
	name                  string
	extNames, instNames   []string
	context               map[string]*javaParseUnit
	extendees, instancees []*javaParseUnit
}

type javaLoggedEdge struct {
	unit  *javaParseUnit
	extra bool
}

type javaSpecReplay struct {
	docs                               map[string]*modulegraph.Document
	parseUnits                         map[string]*javaParseUnit
	acquired                           []string
	root                               *javaParseUnit
	nextName                           string
	nextOwner                          *javaParseUnit
	extentionFound, instantiationFound bool
	stale                              bool
	loggedExt                          map[*javaParseUnit][]*javaParseUnit
	loggedInst                         map[*javaParseUnit][]javaLoggedEdge
	order                              []string
}

func (r *javaSpecReplay) load(t *testing.T, name string) *javaParseUnit {
	t.Helper()
	d := r.docs[name]
	if d == nil {
		t.Fatalf("replay: %s not acquired by Go", name)
	}
	u := &javaParseUnit{name: name, context: map[string]*javaParseUnit{}}
	for _, ref := range d.References() {
		if ref.Kind == modulegraph.Extends {
			u.extNames = append(u.extNames, ref.Name)
		} else {
			u.instNames = append(u.instNames, ref.Name)
		}
	}
	r.parseUnits[name] = u
	r.acquired = append(r.acquired, name)
	return u
}

func (r *javaSpecReplay) findExtention(cur *javaParseUnit, visited map[*javaParseUnit]bool) bool {
	if visited[cur] {
		r.extentionFound = false
		return false
	}
	visited[cur] = true
	for _, n := range cur.extNames {
		if cur.context[n] == nil {
			r.nextName, r.nextOwner, r.extentionFound = n, cur, true
			return true
		}
	}
	for _, n := range cur.extNames {
		if r.findExtention(cur.context[n], visited) {
			r.extentionFound = true
			return true
		}
	}
	for _, n := range cur.instNames {
		if next := cur.context[n]; next != nil && r.findExtention(next, visited) {
			r.extentionFound = true
			return true
		}
	}
	r.extentionFound = false
	return false
}

func (r *javaSpecReplay) findInstantiation(cur *javaParseUnit, visited map[*javaParseUnit]bool) bool {
	if visited[cur] {
		r.instantiationFound = false
		return false
	}
	visited[cur] = true
	for _, n := range cur.instNames {
		if cur.context[n] == nil {
			r.nextName, r.nextOwner, r.instantiationFound = n, cur, true
			return true
		}
	}
	for _, n := range cur.extNames {
		if r.findInstantiation(cur.context[n], visited) {
			r.instantiationFound = true
			return true
		}
	}
	for _, n := range cur.instNames {
		if next := cur.context[n]; next != nil && r.findInstantiation(next, visited) {
			r.instantiationFound = true
			return true
		}
	}
	r.instantiationFound = false
	return false
}

func javaVectorContains(v []*javaParseUnit, x *javaParseUnit) bool {
	for _, y := range v {
		if y == x {
			return true
		}
	}
	return false
}

func (r *javaSpecReplay) nonCircularity(t *testing.T, target, candidate *javaParseUnit, visited map[*javaParseUnit]bool) {
	t.Helper()
	if visited[candidate] {
		return
	}
	visited[candidate] = true
	for _, x := range candidate.instancees {
		if !javaVectorContains(candidate.extendees, x) {
			candidate.extendees = append(candidate.extendees, x)
		}
	}
	for i := 0; i < len(candidate.extendees); i++ {
		if candidate.extendees[i] == target {
			t.Fatalf("replay: circular dependency through %s", target.name)
		}
		r.nonCircularity(t, target, candidate.extendees[i], visited)
	}
}

func (r *javaSpecReplay) calculateDependencies(u *javaParseUnit) {
	for i := 0; i < len(u.extendees); i++ {
		r.calculateDependencies(u.extendees[i])
	}
	for i := 0; i < len(u.instancees); i++ {
		r.calculateDependencies(u.instancees[i])
	}
	for _, n := range r.order {
		if n == u.name {
			return
		}
	}
	r.order = append(r.order, u.name)
}

// replayJavaAcquisition runs the transcription. With stale false it resets
// instantiationFound whenever an extension is found, which is Go's one kind
// per iteration (nested.go), so both variants can be compared.
func replayJavaAcquisition(t *testing.T, a *modulegraph.Acquisition, stale bool) *javaSpecReplay {
	t.Helper()
	r := &javaSpecReplay{docs: map[string]*modulegraph.Document{}, parseUnits: map[string]*javaParseUnit{}, stale: stale, loggedExt: map[*javaParseUnit][]*javaParseUnit{}, loggedInst: map[*javaParseUnit][]javaLoggedEdge{}}
	for _, d := range a.Units {
		r.docs[d.Name] = d
	}
	r.root = r.load(t, a.Root.Name)
	for iterations := 0; r.findExtention(r.root, map[*javaParseUnit]bool{}) || r.findInstantiation(r.root, map[*javaParseUnit]bool{}); iterations++ {
		if iterations > 4*len(a.Units) {
			t.Fatal("replay: acquisition did not terminate")
		}
		u := r.parseUnits[r.nextName]
		if u == nil {
			u = r.load(t, r.nextName)
		}
		owner := r.nextOwner
		if !r.stale && r.extentionFound {
			r.instantiationFound = false
		}
		if r.extentionFound {
			owner.extendees = append(owner.extendees, u)
			r.loggedExt[owner] = append(r.loggedExt[owner], u)
		}
		if r.instantiationFound {
			owner.instancees = append(owner.instancees, u)
			r.loggedInst[owner] = append(r.loggedInst[owner], javaLoggedEdge{unit: u, extra: r.extentionFound})
		}
		r.nonCircularity(t, u, u, map[*javaParseUnit]bool{})
		if r.extentionFound || r.instantiationFound {
			if owner.context[u.name] == nil {
				owner.context[u.name] = u
			}
		}
	}
	r.calculateDependencies(r.root)
	return r
}

func javaUnitNames(v []*javaParseUnit) []string {
	out := []string{}
	for _, u := range v {
		out = append(out, u.name)
	}
	return out
}

// The Java-vector replay: acquisition, logical edges, extras, mutated
// extendee vectors and postorder against Go's acquisition, edges and Order.
func TestStandardInstanceeExtendsJavaVectorReplay(t *testing.T) {
	cases := instanceeCases()
	cases = append(cases, instanceeCase{name: "main EXTENDS-free profile", acquisition: []string{"Root", "Spec", "TLC", "FiniteSets", "Naturals", "Sequences"}, schedule: mainStandardSchedule, stale: map[string][]string{}})
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var a *modulegraph.Acquisition
			if c.root == "" {
				a = standardProfileGraph(t)
			} else {
				a = acquireOverStandards(t, workspaceModule("Root", c.root), nil)
			}
			java := replayJavaAcquisition(t, a, true)
			goKind := replayJavaAcquisition(t, a, false)
			if !reflect.DeepEqual(java.acquired, c.acquisition) || !reflect.DeepEqual(goKind.acquired, c.acquisition) {
				t.Fatalf("replayed acquisition %v / %v, want %v", java.acquired, goKind.acquired, c.acquisition)
			}
			extras := map[string][]string{}
			for _, d := range a.Units {
				u := java.parseUnits[d.Name]
				goExt, goInst := []string{}, []string{}
				for _, e := range d.Edges() {
					if e.Kind == modulegraph.Extends {
						goExt = append(goExt, e.Name)
					} else {
						goInst = append(goInst, e.Name)
					}
				}
				inst := []string{}
				for _, e := range java.loggedInst[u] {
					if !e.extra {
						inst = append(inst, e.unit.name)
						continue
					}
					extras[d.Name] = append(extras[d.Name], e.unit.name)
					if !javaVectorContains(java.loggedExt[u], e.unit) {
						t.Fatalf("%s extra %s is not an extendee of the same owner", d.Name, e.unit.name)
					}
				}
				if ext := javaUnitNames(java.loggedExt[u]); !reflect.DeepEqual(ext, goExt) {
					t.Fatalf("%s Java extendees %v, Go %v", d.Name, ext, goExt)
				}
				if !reflect.DeepEqual(inst, goInst) {
					t.Fatalf("%s Java instancees without extras %v, Go %v", d.Name, inst, goInst)
				}
				// E5: the in-place mutated extendee vectors are equal.
				if got, want := javaUnitNames(u.extendees), javaUnitNames(goKind.parseUnits[d.Name].extendees); !reflect.DeepEqual(got, want) {
					t.Fatalf("%s mutated extendees %v, Go-kind replay %v", d.Name, got, want)
				}
				if len(goKind.loggedInst[goKind.parseUnits[d.Name]]) != len(goInst) {
					t.Fatalf("%s Go-kind replay has extras", d.Name)
				}
			}
			if !reflect.DeepEqual(extras, c.stale) {
				t.Fatalf("stale extras %v, want %v", extras, c.stale)
			}
			var want []string
			for _, s := range c.schedule {
				want = append(want, s.name)
			}
			units, err := a.Order(stdcontext.Background())
			if err != nil {
				t.Fatal(err)
			}
			var goOrder []string
			for _, u := range units {
				for _, d := range a.Units {
					if d.Unit == u {
						goOrder = append(goOrder, d.Name)
					}
				}
			}
			if !reflect.DeepEqual(java.order, want) || !reflect.DeepEqual(goKind.order, want) || !reflect.DeepEqual(goOrder, want) {
				t.Fatalf("postorder Java %v, Go-kind %v, Go Order %v, want %v", java.order, goKind.order, goOrder, want)
			}
		})
	}
}

// Predicate Q at helper level, over real acquisitions of the pinned library.
func TestInstanceeClosurePredicate(t *testing.T) {
	unit := func(a *modulegraph.Acquisition, name string) *modulegraph.Document {
		for _, d := range a.Units {
			if d.Name == name {
				return d
			}
		}
		t.Fatalf("%s not acquired", name)
		return nil
	}
	closureNames := func(d *modulegraph.Document) []string {
		c, why := instanceeClosure(d, 100)
		if why != "" {
			t.Fatalf("%s closure: %s", d.Name, why)
		}
		return unitNames(c)
	}
	rt := acquireOverStandards(t, workspaceModule("Root", "EXTENDS RealTime"), nil)
	for _, tc := range []struct {
		name    string
		closure []string
		why     string
	}{
		{"Integers", []string{"Integers", "Naturals"}, ""},
		{"Reals", []string{"Reals", "Integers", "Naturals"}, ""},
		{"RealTime", []string{"RealTime", "Reals", "Integers", "Naturals"}, "instancee with CONSTANT or VARIABLE declarations"},
		{"Naturals", []string{"Naturals"}, ""},
	} {
		d := unit(rt, tc.name)
		if got := closureNames(d); !reflect.DeepEqual(got, tc.closure) {
			t.Fatalf("C(%s) = %v, want %v", tc.name, got, tc.closure)
		}
		if got := instanceeClosureRejection(d, len(rt.Units)); got != tc.why {
			t.Fatalf("Q(%s) = %q, want %q", tc.name, got, tc.why)
		}
	}

	// Inherited parameters: X declares nothing itself, but its merged context
	// holds RealTime's VARIABLE, so ModuleNode.isParameterFree is false.
	inherited := acquireOverStandards(t, workspaceModule("X", "EXTENDS RealTime"), nil)
	x := unit(inherited, "X")
	xc, _ := instanceeClosure(x, 100)
	if declaresParameters(x) || !closureDeclaresParameters(xc) {
		t.Fatal("inherited parameter not seen through the closure")
	}
	if got := instanceeClosureRejection(x, 100); got != "instancee EXTENDS closure member is not a pinned standard unit" {
		t.Fatalf("Q(X) = %q", got)
	}
	_, xm, _ := runStandardProfile(t, inherited, false)
	_, im, _ := runStandardProfile(t, acquireOverStandards(t, workspaceModule("Y", "EXTENDS Integers"), nil), false)
	if xm.parameterFree() || !im.parameterFree() {
		t.Fatalf("merged-context parameter freedom: X %v, Y %v", xm.parameterFree(), im.parameterFree())
	}

	// Unpinned member: the real pinned Integers over altered Naturals bytes.
	altered := pinnedStandard("Naturals")
	altered.Text = append(append([]byte{}, altered.Text...), '\n')
	un := acquireOverStandards(t, workspaceModule("Root", "EXTENDS Integers"), map[string]modulegraph.Input{"Naturals": altered})
	if got := instanceeClosureRejection(unit(un, "Integers"), 100); got != "instancee EXTENDS closure member is not a pinned standard unit" {
		t.Fatalf("unpinned closure member: %q", got)
	}

	// Q3: a member owning both EXTENDS and INSTANCE.
	mixed := acquireOverStandards(t, workspaceModule("Root", "EXTENDS M"), map[string]modulegraph.Input{"M": workspaceModule("M", "EXTENDS Naturals\nLOCAL INSTANCE Sequences")})
	mc, _ := instanceeClosure(unit(mixed, "Root"), 100)
	ic, _ := instanceeClosure(unit(rt, "Integers"), 100)
	if !closureMixesReferenceKinds(mc) || closureMixesReferenceKinds(ic) {
		t.Fatal("mixed reference kinds")
	}

	// Q4: a missing target and an exhausted bound both fail closed.
	if _, why := instanceeClosure(nil, 100); why == "" {
		t.Fatal("nil target admitted")
	}
	if _, why := instanceeClosure(unit(rt, "RealTime"), 2); why != "instancee EXTENDS closure is unbound, cyclic or unbounded" {
		t.Fatalf("bounded walk: %q", why)
	}

	// Over the pinned library, every INSTANCE target of a non-trace standard
	// satisfies Q, Q3 holds for every closure, and Integers is the only
	// instanced standard with EXTENDS.
	targets := map[string]bool{}
	for _, name := range []string{"Naturals", "Sequences", "FiniteSets", "TLC", "TLCExt", "Integers", "Reals", "RealTime", "Bags", "Json", "Randomization", "Toolbox", "_Possible"} {
		if standard.Lookup(name) == nil {
			t.Fatalf("%s not embedded", name)
		}
		a := acquireOverStandards(t, workspaceModule("Root", "EXTENDS "+name), nil)
		for _, d := range a.Units {
			c, why := instanceeClosure(d, 100)
			if why != "" || closureMixesReferenceKinds(c) {
				t.Fatalf("%s closure %q mixed=%v", d.Name, why, closureMixesReferenceKinds(c))
			}
			for _, r := range d.References() {
				if r.Kind != modulegraph.Instance || d.Name == "Root" {
					continue
				}
				target := d.Linked(r.Name)
				if got := instanceeClosureRejection(target, len(a.Units)); got != "" {
					t.Fatalf("%s INSTANCE %s: %q", d.Name, r.Name, got)
				}
				if len(target.Edges()) != 0 && target.Edges()[0].Kind == modulegraph.Extends {
					targets[d.Name+"->"+r.Name] = true
				}
			}
		}
	}
	if !reflect.DeepEqual(targets, map[string]bool{"TLCExt->Integers": true}) {
		t.Fatalf("instanced standards with EXTENDS: %v", targets)
	}
}

// Cancellation, the depth limit and a sticky unknown prefix on the TLCExt graph.
func TestStandardInstanceeExtendsLimits(t *testing.T) {
	a := acquireOverStandards(t, workspaceModule("Root", "EXTENDS TLCExt"), nil)
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
	for _, stop := range []int{1, 5, 6} {
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

	// A user unit generated first loses exactness; the admitted graph keeps
	// the first unknown reason and every later unit stays unknown.
	sticky := acquireOverStandards(t, workspaceModule("Root", "EXTENDS A, TLCExt"), map[string]modulegraph.Input{"A": workspaceModule("A", "F == <<TRUE, FALSE>>\nG == F!1!Missing")})
	trace = flatProfileTrace{}
	_, _, root, roots, _ := analyzeFlatExternalProfile(stdcontext.Background(), sticky, 1000, false, nil, &trace)
	if trace.admission != standardLocalInstanceProfile || trace.exact || trace.firstUnknown != "unproved general identifier selector" || len(roots) == 0 || roots[0].name.text != "A" {
		t.Fatalf("sticky prefix trace %+v", trace)
	}
	h := &roots[0].owner.history
	if got := h.reuse(roots[0]); !got.known || got.uid != 304 {
		t.Fatalf("prefix ticket %+v", got)
	}
	for _, m := range roots[1:] {
		if h.reuse(m).known {
			t.Fatalf("%s known after the first unknown", m.name.text)
		}
	}
	_ = root
}
