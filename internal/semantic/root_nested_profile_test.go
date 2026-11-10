// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// The acquisition replay below transcribes TLA+ SpecObj.java and
// ParseUnit.java, Copyright 2003 Compaq Corporation and Microsoft
// Corporation, under the MIT license; see LICENSE.

package semantic

import (
	stdcontext "context"
	"reflect"
	"sort"
	"strings"
	"testing"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/modulegraph"
)

// These tests cover predicate N, root-local nesting. Java citations are to
// the pinned dc6a944 tla2sany sources:
//
//   - Generator.java:2238-2252 generates an inner module inside the outer
//     Generator: a fresh SymbolTable over the outer stack and a fresh Context
//     (neither is a SemanticNode), one ModuleNode (Generator.java:2192,
//     SemanticNode.java:43,53), the body, then SymbolTable.addModule
//     (SymbolTable.java:227-247), which puts a ModuleName key hashing as
//     name.hashCode() (SymbolTable.java:266-273) into the outer context.
//   - processOperator generates the body before the OpDefNode
//     (Generator.java:2660,2674).
//   - ModuleNode.levelCheck checks inner modules first (ModuleNode.java:919-925)
//     in getInnerModules order, which is Context.getModDefs, a Hashtable
//     enumeration (ModuleNode.java:494-503, Context.java:337-348).
//   - processExtendsList merges extendee contexts (Generator.java:2336-2379,
//     Context.java:370-436) and allocates no SemanticNode.

const h1RootBody = "---- MODULE A ----\nX == TRUE\n====\n---- MODULE B ----\nY == TRUE\n====\nZ == TRUE"

func rootNestedTrace(t *testing.T, a *modulegraph.Acquisition, level bool) (flatProfileTrace, *moduleNode, []*moduleNode) {
	t.Helper()
	return runStandardProfile(t, a, level)
}

func rejectedTrace(t *testing.T, a *modulegraph.Acquisition) (flatProfileTrace, *moduleNode) {
	t.Helper()
	var trace flatProfileTrace
	_, _, top, _, _ := analyzeFlatExternalProfile(stdcontext.Background(), a, 1000, false, nil, &trace)
	return trace, top
}

// javaTableKey is a key of a module Context's Hashtable: a UniqueString
// symbol name or a SymbolTable.ModuleName. Both hash as String.hashCode of the
// name; they are never equal to each other (SymbolTable.java:266-273).
type javaTableKey struct {
	name   string
	module bool
}

// javaHashtableKeyOrder is javaHashtableOrder extended to typed keys: the same
// independent transcription of java.util.Hashtable put/rehash/elements.
func javaHashtableKeyOrder(keys []javaTableKey) []javaTableKey {
	type entry struct {
		hash int32
		key  javaTableKey
		next *entry
	}
	table := make([]*entry, 11)
	threshold, count := 8, 0
	present := map[javaTableKey]bool{}
	for _, k := range keys {
		if present[k] {
			continue
		}
		present[k] = true
		h := javaStringHash(k.name)
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
	var out []javaTableKey
	for i := len(table) - 1; i >= 0; i-- {
		for e := table[i]; e != nil; e = e.next {
			out = append(out, e.key)
		}
	}
	return out
}

func symbolKeys(names []string) []javaTableKey {
	out := make([]javaTableKey, len(names))
	for i, n := range names {
		out[i] = javaTableKey{name: n}
	}
	return out
}

// goContextKeys reads Go's Hashtable model in enumeration order, keys included.
func goContextKeys(c *context) []javaTableKey {
	var out []javaTableKey
	for i := len(c.order) - 1; i >= 0; i-- {
		for j := len(c.order[i]) - 1; j >= 0; j-- {
			out = append(out, javaTableKey{name: c.order[i][j].name.text, module: c.order[i][j].module})
		}
	}
	return out
}

// naturalsMergedPrefix is the insertion sequence of a module context that
// EXTENDS Naturals: the duplicated builtins, then the merge of Naturals' pair
// list (integersContextInsertion without Integers' own definitions).
func naturalsMergedPrefix() []string {
	var naturalsPairs []string
	for _, d := range builtinDescriptors() {
		naturalsPairs = append(naturalsPairs, d.text)
	}
	naturalsPairs = append(naturalsPairs, naturalsDefs...)
	return mergeExtendKeys(moduleContextPrefix(), naturalsPairs)
}

func innerModule(t *testing.T, m *moduleNode, name string) *moduleNode {
	t.Helper()
	for _, d := range m.bodyDefinitions {
		if inner, ok := d.(*moduleNode); ok && inner.name.text == name {
			return inner
		}
	}
	t.Fatalf("%s has no inner module %s", m.name.text, name)
	return nil
}

// checkUnitSpans checks every unit before the root with checkStandardSchedule,
// then the root's module ticket, its quartet, and its whole span against the
// single-unit generation of rootSource (the root without its EXTENDS clause,
// which allocates nothing), shifted to the root's quartet.
func checkUnitSpans(t *testing.T, h *javaHistory, roots []*moduleNode, want []unitSchedule, rootSource string, final uint64) {
	t.Helper()
	if !h.exact || h.count != final {
		t.Fatalf("history exact=%v count=%d first=%q, want count %d", h.exact, h.count, h.firstUnknown, final)
	}
	if len(roots) != len(want) {
		t.Fatalf("generated %d units, want %d", len(roots), len(want))
	}
	n := len(want) - 1
	checkStandardSchedule(t, h, roots[:n], want[:n], h.count)
	root, w := roots[n], want[n]
	if got := h.reuse(root); root.name.text != w.name || !got.known || got.uid != w.module || got.class != "ModuleNode" || got.kind != 1 {
		t.Fatalf("root %s ticket %+v, want %s %d", root.name.text, got, w.name, w.module)
	}
	g, single, err := analyzeOwned(stdcontext.Background(), parsedModule(t, rootSource), 1000)
	if err != nil || single == nil || !g.pool.history.exact {
		t.Fatalf("single-unit root: err=%v exact=%v", err, g.pool.history.exact)
	}
	s := &g.pool.history
	if s.reuse(single).uid != 304 {
		t.Fatal("single-unit root ticket")
	}
	shift := w.module - 304
	if uint64(shift)+s.count != final {
		t.Fatalf("root span ends at %d, single-unit count %d shifted by %d", final, s.count, shift)
	}
	gUID, sUID := historyByUID(t, h), historyByUID(t, s)
	for uid := uint32(300); uint64(uid) < s.count; uid++ {
		got, want := gUID[uid+shift].ticket, sUID[uid].ticket
		if got.class != want.class || got.kind != want.kind {
			t.Fatalf("root event %d is %s/%d, single-unit event %d is %s/%d", uid+shift, got.class, got.kind, uid, want.class, want.kind)
		}
	}
}

// H1, the gate: Root EXTENDS Naturals with inner A (X == TRUE), inner B
// (Y == TRUE), then Z == TRUE.
func TestRootNestedH1ContextScheduleAndLevelOrder(t *testing.T) {
	a := acquireOverStandards(t, workspaceModule("Root", "EXTENDS Naturals\n"+h1RootBody), nil)
	if got := unitNames(a.Units); !reflect.DeepEqual(got, []string{"Root", "Naturals"}) {
		t.Fatalf("acquisition %v", got)
	}
	if edgeText(a.Root) != "E:Naturals " || edgeText(a.Units[1]) != "" {
		t.Fatalf("edges %q %q", edgeText(a.Root), edgeText(a.Units[1]))
	}
	for _, level := range []bool{false, true} {
		trace, root, roots := rootNestedTrace(t, a, level)
		if trace.admission != "root-nested/linear" || !trace.bootstrapExact || trace.units != 2 || trace.exit != "completed" {
			t.Fatalf("trace %+v", trace)
		}
		if trace.linearRejection != "nested module definitions in multi-unit graph" || trace.flatRejection != "nested module definitions" || trace.standardWhy != "nested module definitions" {
			t.Fatalf("older profiles %+v", trace)
		}
		h := &root.owner.history
		if want := map[bool]javaHistoryProfile{false: freshPhase2Profile, true: levelEnabledProfile}[level]; h.profile != want {
			t.Fatalf("profile %d, want %d", h.profile, want)
		}
		if root.owner.java.exact {
			t.Fatal("legacy direct-allocation cursor admitted a multi-unit graph")
		}

		// The schedule, from source: Naturals ends at 375 (its single-unit
		// span, TestStandardLocalInstanceBodiesMatchProvedProfiles); Root's
		// quartet 375-378 and ModuleNode 379; each inner module one ModuleNode
		// then its body (TRUE's OpApplNode, then the OpDefNode); then Z.
		checkUnitSpans(t, h, roots, []unitSchedule{{name: "Naturals", module: 304}, {name: "Root", module: 379}}, "---- MODULE Root ----\n"+h1RootBody+"\n====\n", 388)
		innerA, innerB := innerModule(t, root, "A"), innerModule(t, root, "B")
		x, y := testUserDefinitions(t, innerA.definitions)[0], testUserDefinitions(t, innerB.definitions)[0]
		z := symbolByText(root.context, "Z").(*userOpDefNode)
		for _, w := range []struct {
			uid   uint32
			node  any
			class string
			kind  int
		}{
			{380, innerA, "ModuleNode", 1}, {381, x.body, "OpApplNode", 9}, {382, x, "OpDefNode", 5},
			{383, innerB, "ModuleNode", 1}, {384, y.body, "OpApplNode", 9}, {385, y, "OpDefNode", 5},
			{386, z.body, "OpApplNode", 9}, {387, z, "OpDefNode", 5},
		} {
			if got := h.reuse(w.node); !got.known || got.uid != w.uid || got.class != w.class || got.kind != w.kind {
				t.Fatalf("uid %d: %+v, want %s/%d", w.uid, got, w.class, w.kind)
			}
		}
		// No Generator quartet for an inner module.
		byUID := historyByUID(t, h)
		for uid := uint32(379); uid < 388; uid++ {
			if c := byUID[uid].ticket; c.class == "OpArgNode" || c.class == "LabelNode" || (c.class == "OpDefNode" && c.kind == 0) {
				t.Fatalf("constructor-quartet event %+v at %d", c, uid)
			}
		}

		// Root's base context: 72 builtins, 12 merged Naturals entries,
		// ModuleName(A), ModuleName(B), Z; 87 entries at capacity 191.
		insertion := symbolKeys(naturalsMergedPrefix())
		insertion = append(insertion, javaTableKey{"A", true}, javaTableKey{"B", true}, javaTableKey{"Z", false})
		if len(insertion) != 87 {
			t.Fatalf("transcribed insertion has %d keys", len(insertion))
		}
		want := javaHashtableKeyOrder(insertion)
		if got := goContextKeys(root.context); !reflect.DeepEqual(got, want) {
			t.Fatalf("Root context order\n got %v\nwant %v", got, want)
		}
		if len(root.context.order) != 191 || root.context.orderCount != 87 {
			t.Fatalf("Root context capacity %d count %d", len(root.context.order), root.context.orderCount)
		}
		if javaStringHash("A") != 65 || javaStringHash("B") != 66 || 65%191 != 65 {
			t.Fatal("bucket arithmetic")
		}
		// Controls: insertion order and the module bit both matter.
		reversed := make([]javaTableKey, len(insertion))
		for i, k := range insertion {
			reversed[len(insertion)-1-i] = k
		}
		if reflect.DeepEqual(javaHashtableKeyOrder(reversed), want) {
			t.Fatal("transcription is insensitive to insertion order")
		}
		if !reflect.DeepEqual(javaHashtableKeyOrder(symbolKeys(naturalsMergedPrefix())), symbolKeys(javaHashtableOrder(naturalsMergedPrefix()))) {
			t.Fatal("typed transcription differs from javaHashtableOrder on symbol keys")
		}
		// getModDefs over that enumeration: B before A.
		var mods []*moduleNode
		for _, v := range root.context.orderedValues() {
			if m, ok := v.(*moduleNode); ok {
				mods = append(mods, m)
			}
		}
		if len(mods) != 2 || mods[0] != innerB || mods[1] != innerA {
			t.Fatalf("module enumeration %v", mods)
		}
		if level {
			if order := levelVisitOrder(t, root, h, []*moduleNode{innerA, innerB}); !reflect.DeepEqual(order, []string{"B", "A"}) {
				t.Fatalf("level visitation %v, want B before A", order)
			}
		}
	}
	o := observeLevelCandidate(stdcontext.Background(), a, 1000)
	if o.err != nil || !o.finished || !o.bound || !o.levelComplete || !o.generationComplete || len(o.generation) != 0 || len(o.level) != 0 {
		t.Fatalf("observation %+v", o)
	}
	if c := o.cause; c.driverProfile != "root-nested/linear" || c.driverRejection != "none" || c.historyState != "exact" || c.completion != "none" || len(c.boundaries) != 2 || c.boundaries[0] != "supported" || c.boundaries[1] != "supported" {
		t.Fatalf("cause %+v", o.cause)
	}
}

// levelVisitOrder is a bounded test-only observation of the production level
// checker. It reruns a fresh checker over root with work limits 1, 2, ... and
// records the first limit at which each inner module's state exists. The
// checker creates a node's state on entry, so the limits order the visits.
// No production decision reads this.
func levelVisitOrder(t *testing.T, root *moduleNode, h *javaHistory, inner []*moduleNode) []string {
	t.Helper()
	identity := func(n leafNode) (uint64, uint32, error) {
		tk := h.reuse(n)
		if !tk.known || n.header().kind != tk.kind {
			return 0, 0, ErrPortIncomplete
		}
		return uint64(tk.uid) + 1, uint32(961+31*tk.kind) + tk.uid, nil
	}
	var order []string
	seen := map[*moduleNode]bool{}
	for limit := uint64(1); limit < 1<<16 && len(order) < len(inner); limit++ {
		c, err := newLevelChecker(stdcontext.Background(), identity, 1000, limit)
		if err != nil {
			t.Fatal(err)
		}
		c.errors = &Errors{}
		_, err = c.check(root, 1)
		for _, m := range inner {
			if c.states[m] != nil && !seen[m] {
				seen[m] = true
				order = append(order, m.name.text)
			}
		}
		if err == nil {
			break
		}
	}
	// The unbounded check succeeds and is level-correct.
	c, err := newLevelChecker(stdcontext.Background(), identity, 1000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	c.errors = &Errors{}
	if ok, err := c.check(root, 1); err != nil || !ok || len(c.errors.Messages()) != 0 {
		t.Fatalf("level check ok=%v err=%v", ok, err)
	}
	return order
}

// The optional same-bucket check: a root symbol A shares bucket 65 with
// ModuleName(A), in head-insertion order; inner visitation stays B, A.
func TestRootNestedH1SameBucketSymbolAndModuleKey(t *testing.T) {
	body := h1RootBody + "\nA == TRUE"
	a := acquireOverStandards(t, workspaceModule("Root", "EXTENDS Naturals\n"+body), nil)
	trace, root, roots := rootNestedTrace(t, a, true)
	if trace.admission != "root-nested/linear" {
		t.Fatalf("trace %+v", trace)
	}
	h := &root.owner.history
	checkUnitSpans(t, h, roots, []unitSchedule{{name: "Naturals", module: 304}, {name: "Root", module: 379}}, "---- MODULE Root ----\n"+body+"\n====\n", 390)
	insertion := append(symbolKeys(naturalsMergedPrefix()), javaTableKey{"A", true}, javaTableKey{"B", true}, javaTableKey{"Z", false}, javaTableKey{"A", false})
	want := javaHashtableKeyOrder(insertion)
	if got := goContextKeys(root.context); !reflect.DeepEqual(got, want) {
		t.Fatalf("Root context order\n got %v\nwant %v", got, want)
	}
	for i, k := range want {
		if k == (javaTableKey{"A", false}) {
			if i+1 >= len(want) || want[i+1] != (javaTableKey{"A", true}) {
				t.Fatalf("symbol A is not the head before ModuleName(A): %v", want)
			}
		}
	}
	innerA, innerB := innerModule(t, root, "A"), innerModule(t, root, "B")
	if order := levelVisitOrder(t, root, h, []*moduleNode{innerA, innerB}); !reflect.DeepEqual(order, []string{"B", "A"}) {
		t.Fatalf("level visitation %v", order)
	}
	if def, ok := symbolByText(root.context, "A").(*userOpDefNode); !ok || def.module != root {
		t.Fatal("root symbol A")
	}
}

// Declaration order does not decide visitation: declaring B before A gives
// the same Hashtable order, so the check visits B first either way.
func TestRootNestedLevelOrderFollowsHashtableNotText(t *testing.T) {
	body := "---- MODULE B ----\nY == TRUE\n====\n---- MODULE A ----\nX == TRUE\n====\nZ == TRUE"
	a := acquireOverStandards(t, workspaceModule("Root", "EXTENDS Naturals\n"+body), nil)
	trace, root, _ := rootNestedTrace(t, a, true)
	if trace.admission != "root-nested/linear" || !root.owner.history.exact {
		t.Fatalf("trace %+v", trace)
	}
	innerA, innerB := innerModule(t, root, "A"), innerModule(t, root, "B")
	if order := levelVisitOrder(t, root, &root.owner.history, []*moduleNode{innerA, innerB}); !reflect.DeepEqual(order, []string{"B", "A"}) {
		t.Fatalf("level visitation %v", order)
	}
}

const singleNestedSource = "---- MODULE Root ----\n---- MODULE Inner ----\nX == TRUE\n====\nINSTANCE Inner\nI == INSTANCE Inner\n====\n"

// Positive case 1: TestHistoryParameterFreeInstances' source through the
// driver. Its two internal INSTANCEs defeated linear; V has no references.
func TestRootNestedSingleUnitMatchesAnalyzeOwned(t *testing.T) {
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), modulegraph.Input{Path: "Root.tla", Text: []byte(singleNestedSource)}, nil, standardProfileLimits)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Units) != 1 || edgeText(a.Root) != "" || len(a.Root.References()) != 2 {
		t.Fatalf("acquisition %v refs %d", unitNames(a.Units), len(a.Root.References()))
	}
	g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, singleNestedSource), 1000)
	if err != nil || !g.pool.history.exact || g.pool.history.count != 314 {
		t.Fatalf("analyzeOwned: err=%v count=%d", err, g.pool.history.count)
	}
	owned := historyByUID(t, &g.pool.history)
	for _, level := range []bool{false, true} {
		trace, root, _ := rootNestedTrace(t, a, level)
		if trace.admission != "root-nested/linear" || trace.linearRejection != "unit with multiple module references" {
			t.Fatalf("trace %+v", trace)
		}
		h := &root.owner.history
		if !h.exact || h.count != 314 {
			t.Fatalf("history exact=%v count=%d first=%q", h.exact, h.count, h.firstUnknown)
		}
		driven := historyByUID(t, h)
		if len(driven) != len(owned) {
			t.Fatalf("%d tickets, analyzeOwned %d", len(driven), len(owned))
		}
		for uid, w := range owned {
			if got := driven[uid].ticket; got != w.ticket {
				t.Fatalf("uid %d: %+v, analyzeOwned %+v", uid, got, w.ticket)
			}
		}
	}
}

const flatNestedBody = "---- MODULE I1 ----\nP == TRUE\n====\n---- MODULE I2 ----\nEXTENDS I1\nQ == P\n====\nJ == INSTANCE I2"

func flatNestedStaged() map[string]modulegraph.Input {
	return map[string]modulegraph.Input{
		"A": workspaceModule("A", "EXTENDS Naturals\nAX == 1"),
		"B": workspaceModule("B", "BV == TRUE"),
	}
}

// Positive case 2: flat EXTENDS with an inner EXTENDS bound at discovery and
// a named internal INSTANCE.
func TestRootNestedFlatExtends(t *testing.T) {
	a := acquireOverStandards(t, workspaceModule("Root", "EXTENDS A, B\n"+flatNestedBody), flatNestedStaged())
	if got := unitNames(a.Units); !reflect.DeepEqual(got, []string{"Root", "A", "B", "Naturals"}) {
		t.Fatalf("acquisition %v", got)
	}
	for name, want := range map[string]string{"Root": "E:A E:B ", "A": "E:Naturals ", "B": "", "Naturals": ""} {
		for _, d := range a.Units {
			if d.Name == name && edgeText(d) != want {
				t.Fatalf("%s edges %q, want %q", name, edgeText(d), want)
			}
		}
	}
	// The same graph without inner modules: an existing flat-EXTENDS case.
	plain := acquireOverStandards(t, workspaceModule("Root", "EXTENDS A, B"), flatNestedStaged())
	ptrace, proot, proots := rootNestedTrace(t, plain, false)
	if ptrace.admission != "flat-extends" || !proot.owner.history.exact {
		t.Fatalf("plain trace %+v", ptrace)
	}
	var units []unitSchedule
	for _, m := range proots {
		units = append(units, unitSchedule{name: m.name.text, module: proot.owner.history.reuse(m).uid})
	}
	if names := []string{units[0].name, units[1].name, units[2].name, units[3].name}; !reflect.DeepEqual(names, []string{"Naturals", "A", "B", "Root"}) {
		t.Fatalf("plain order %v", names)
	}
	for _, level := range []bool{false, true} {
		trace, root, roots := rootNestedTrace(t, a, level)
		if trace.admission != "root-nested/flat-extends" || trace.exit != "completed" {
			t.Fatalf("trace %+v", trace)
		}
		checkUnitSpans(t, &root.owner.history, roots, units, "---- MODULE Root ----\n"+flatNestedBody+"\n====\n", uint64(units[3].module)+uint64(rootSpan(t, flatNestedBody)))
	}
}

// rootSpan is the number of events from the root ModuleNode to the end of its
// single-unit generation.
func rootSpan(t *testing.T, body string) uint32 {
	t.Helper()
	g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE Root ----\n"+body+"\n====\n"), 1000)
	if err != nil || !g.pool.history.exact {
		t.Fatalf("single-unit root: %v", err)
	}
	return uint32(g.pool.history.count - 304)
}

const standardNestedBody = "---- MODULE M ----\nX == TRUE\n====\nY == TRUE"

// Positive case 3: the reduced view is a standard LOCAL INSTANCE shape.
func TestRootNestedStandardLocalInstance(t *testing.T) {
	a := acquireOverStandards(t, workspaceModule("Root", "EXTENDS Sequences, FiniteSets\n"+standardNestedBody), nil)
	if got := unitNames(a.Units); !reflect.DeepEqual(got, []string{"Root", "Sequences", "FiniteSets", "Naturals"}) {
		t.Fatalf("acquisition %v", got)
	}
	want := map[string]string{"Root": "E:Sequences E:FiniteSets ", "Sequences": "I:Naturals ", "FiniteSets": "I:Naturals I:Sequences ", "Naturals": ""}
	for _, d := range a.Units {
		if edgeText(d) != want[d.Name] {
			t.Fatalf("%s edges %q", d.Name, edgeText(d))
		}
	}
	units := append(append([]unitSchedule{}, tlcExtPrefix[:3]...), unitSchedule{name: "Root", module: 616})
	for _, level := range []bool{false, true} {
		trace, root, roots := rootNestedTrace(t, a, level)
		if trace.admission != rootNestedProfile+"/"+standardLocalInstanceProfile || trace.standardWhy != "nested module definitions" {
			t.Fatalf("trace %+v", trace)
		}
		checkUnitSpans(t, &root.owner.history, roots, units, "---- MODULE Root ----\n"+standardNestedBody+"\n====\n", 616+uint64(rootSpan(t, standardNestedBody)))
	}
	// A reduced view that is a chain is linear first: Root EXTENDS Sequences
	// with an inner module is root-nested/linear, Root at Chain's 538.
	chain := acquireOverStandards(t, workspaceModule("Root", "EXTENDS Sequences\n"+standardNestedBody), nil)
	trace, root, roots := rootNestedTrace(t, chain, false)
	if trace.admission != "root-nested/linear" {
		t.Fatalf("chain trace %+v", trace)
	}
	checkUnitSpans(t, &root.owner.history, roots, append(append([]unitSchedule{}, mainStandardSchedule[:2]...), unitSchedule{name: "Root", module: 538}), "---- MODULE Root ----\n"+standardNestedBody+"\n====\n", 538+uint64(rootSpan(t, standardNestedBody)))
	// The retained post-guard: a root INSTANCE of its own inner module is
	// admitted by the view but invalidated after generation.
	own := acquireOverStandards(t, workspaceModule("Root", "EXTENDS Sequences, FiniteSets\n---- MODULE M ----\nX == TRUE\n====\nINSTANCE M"), nil)
	var otrace flatProfileTrace
	if _, _, top, _, err := analyzeFlatExternalProfile(stdcontext.Background(), own, 1000, false, nil, &otrace); err != nil || top == nil {
		t.Fatalf("own instance: %v", err)
	}
	if otrace.admission != rootNestedProfile+"/"+standardLocalInstanceProfile || otrace.exact || otrace.firstUnknown != "unproved standard LOCAL INSTANCE target" {
		t.Fatalf("own-instance trace %+v", otrace)
	}
}

// Every excluded shape stays unprofiled, with root-nested's first reason.
func TestRootNestedExclusionsStayRejected(t *testing.T) {
	cases := []struct {
		name, root string
		staged     map[string]modulegraph.Input
		rootInput  *modulegraph.Input
		why        string
		general    bool // the general driver, consulted after N, admits the graph
	}{
		{name: "N1 dependency with inner module", root: "EXTENDS A", staged: map[string]modulegraph.Input{"A": workspaceModule("A", "---- MODULE I ----\nX == 1\n====")}, why: "nested module definitions in dependency"},
		{name: "N1 C3 foreign inner instance", root: "EXTENDS A\n---- MODULE J ----\nINSTANCE I\n====", staged: map[string]modulegraph.Input{"A": workspaceModule("A", "---- MODULE I ----\nX == 1\n====")}, why: "nested module definitions in dependency"},
		{name: "N2 inner EXTENDS of a unit", root: "---- MODULE Inner ----\nEXTENDS A\n====", staged: map[string]modulegraph.Input{"A": workspaceModule("A", "X == TRUE")}, why: "inner module reference leaves the root unit", general: true},
		{name: "N2 N-plus duplicate inner EXTENDS", root: "EXTENDS Naturals\n---- MODULE I ----\nEXTENDS Naturals\n====", why: "inner module reference leaves the root unit", general: true},
		{name: "N2 C1 inner owner acquires", root: "INSTANCE Y\n---- MODULE I ----\nEXTENDS X\n====", staged: map[string]modulegraph.Input{"X": workspaceModule("X", "XX == 1"), "Y": workspaceModule("Y", "YY == 1")}, why: "inner module reference leaves the root unit", general: true},
		{name: "N2 inner INSTANCE of a unit", root: "EXTENDS Naturals\n---- MODULE I ----\nINSTANCE Naturals\n====", why: "inner module reference leaves the root unit", general: true},
		{name: "N4 root EXTENDS a staged unit named like an inner module", root: "EXTENDS Inner\n---- MODULE Inner ----\nX == 1\n====", staged: map[string]modulegraph.Input{"Inner": workspaceModule("Inner", "Y == 1")}, why: "inner module named like an acquired unit"},
		{name: "N4 root INSTANCE before the inner module", root: "INSTANCE In\n---- MODULE In ----\nX == 1\n====", staged: map[string]modulegraph.Input{"In": workspaceModule("In", "Y == 1")}, why: "inner module named like an acquired unit"},
		{name: "N4 inner module named like the root", root: "EXTENDS Naturals\n---- MODULE Root ----\nX == 1\n====", why: "inner module named like an acquired unit"},
		{name: "N5 monolith sibling", rootInput: &modulegraph.Input{Path: "Root.tla", Origin: modulegraph.OriginWorkspace, Text: []byte("---- MODULE Root ----\nEXTENDS Wanted\n---- MODULE I ----\nX == 1\n====\n====\n---- MODULE Wanted ----\nY == 1\n====\n")}, why: "monolith-extracted unit"},
		{name: "repeated root reference", root: "EXTENDS Naturals\nINSTANCE Naturals\n---- MODULE I ----\nX == 1\n====", why: "root edges differ from the reduced view", general: true},
		{name: "no inner modules", root: "EXTENDS A\nINSTANCE B", staged: map[string]modulegraph.Input{"A": workspaceModule("A", "X == 1"), "B": workspaceModule("B", "Y == 1")}, why: "root unit has no inner modules", general: true},
		{name: "reduced view with a user INSTANCE", root: "EXTENDS A\nINSTANCE B\n---- MODULE I ----\nX == 1\n====", staged: map[string]modulegraph.Input{"A": workspaceModule("A", "X == 1"), "B": workspaceModule("B", "Y == 1")}, why: rootNestedReducedView, general: true},
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
			if tc.general {
				admission = generalDriverProfile
			}
			if trace.admission != admission || trace.rootNested.why != tc.why || trace.rootNested.profile != "" {
				t.Fatalf("admission %q root-nested %+v, want %q %q", trace.admission, trace.rootNested, admission, tc.why)
			}
			if tc.general {
				if !trace.exact || top == nil {
					t.Fatalf("general-driver graph lost its history: %+v", trace)
				}
				return
			}
			if trace.profile != 0 || trace.exact || (top != nil && top.owner.history.profile != 0) {
				t.Fatalf("rejected graph was profiled: %+v", trace)
			}
		})
	}
	// The composed reduced-view telemetry. The general driver admits this
	// graph, so the clause is pinned on the private trace, then on a variant
	// with a workspace Naturals shadow whose Unspecified origin every profile
	// rejects (the shadow also becomes the reduced view's standard reason).
	// The same variant with origin Workspace is admitted by the general
	// driver (G1c′; shadow_provenance_test.go S6).
	a := acquireOverStandards(t, workspaceModule("Root", "EXTENDS A\nINSTANCE B\n---- MODULE I ----\nX == 1\n===="), map[string]modulegraph.Input{"A": workspaceModule("A", "X == 1"), "B": workspaceModule("B", "Y == 1")})
	trace, _ := rejectedTrace(t, a)
	reduced := "reduced view: linear: unit with multiple module references / flat-extends: non-EXTENDS module reference / standard-local-instance: INSTANCE outside a pinned standard unit"
	if trace.admission != generalDriverProfile || rootNestedCause(trace.rootNested) != reduced {
		t.Fatalf("trace %+v", trace)
	}
	naturals := pinnedStandard("Naturals")
	naturals.Origin = modulegraph.OriginUnspecified
	a = acquireOverStandards(t, workspaceModule("Root", "EXTENDS A\nINSTANCE B\n---- MODULE I ----\nX == 1\n===="), map[string]modulegraph.Input{"A": workspaceModule("A", "EXTENDS Naturals"), "B": workspaceModule("B", "Y == 1"), "Naturals": naturals})
	o := observeLevelCandidate(stdcontext.Background(), a, 1000)
	if want := "linear: unit with multiple module references; flat-extends: nested module definitions; standard-local-instance: nested module definitions; root-nested: reduced view: linear: unit with multiple module references / flat-extends: non-EXTENDS module reference / standard-local-instance: workspace unit shadows a standard module; general-driver: unspecified unit shadows a standard module"; o.cause.driverRejection != want || o.cause.historyState != "unprofiled" {
		t.Fatalf("cause %q", o.cause.driverRejection)
	}
}

// The older profiles keep precedence: graphs they admit never reach N.
func TestRootNestedKeepsOlderProfiles(t *testing.T) {
	for _, tc := range []struct{ root, admission string }{
		{"EXTENDS Sequences", "linear"},
		{"EXTENDS Naturals, Integers", "flat-extends"},
		{"EXTENDS TLC", standardLocalInstanceProfile},
		{"---- MODULE Inner ----\nX == 1\n====\nINSTANCE Inner", "linear"},
		{"---- MODULE Inner ----\nX == 1\n====", "linear"},
	} {
		trace, root, _ := rootNestedTrace(t, acquireOverStandards(t, workspaceModule("Root", tc.root), nil), false)
		if trace.admission != tc.admission || !root.owner.history.exact || trace.rootNested != (rootNestedVerdict{}) {
			t.Fatalf("%q: %+v", tc.root, trace)
		}
	}
}

// Source errors, prior invalidation, cancellation and the depth limit.
func TestRootNestedLimitsAndErrors(t *testing.T) {
	// A generation diagnostic in an inner module: still admitted, the
	// diagnostic is reported and the level check is skipped.
	bad := acquireOverStandards(t, workspaceModule("Root", "EXTENDS Naturals\n---- MODULE A ----\nX == Missing\n====\nZ == TRUE"), nil)
	var trace flatProfileTrace
	details, _, top, _, err := analyzeFlatExternalProfile(stdcontext.Background(), bad, 1000, true, nil, &trace)
	if err != nil || top == nil || trace.admission != "root-nested/linear" || len(details) != 1 || details[0].Code.Value != 4200 || details[0].Location.BeginLine != 4 {
		t.Fatalf("diagnostic run: err=%v trace=%+v details=%+v", err, trace, details)
	}
	// The unknown operator allocates no node: Root 379, A 380, X 381, then
	// Z's body and definition 382-383.
	if h := &top.owner.history; !h.exact || h.count != 384 || h.reuse(top).uid != 379 || h.reuse(innerModule(t, top, "A")).uid != 380 {
		t.Fatalf("diagnostic history exact=%v count=%d", h.exact, h.count)
	}
	o := observeLevelCandidate(stdcontext.Background(), bad, 1000)
	if o.cause.driverProfile != "root-nested/linear" || len(o.generation) == 0 || len(o.attempts) != 2 || !o.attempts[1].skipped {
		t.Fatalf("diagnostic observation %+v", o.cause)
	}

	// Prior invalidation: an unproved body in an earlier unit keeps its
	// prefix and no later unit gets a known ticket.
	prior := acquireOverStandards(t, workspaceModule("Root", "EXTENDS P\n"+h1RootBody), map[string]modulegraph.Input{"P": workspaceModule("P", "F == <<TRUE,FALSE>>\nG == F!1!Missing")})
	trace = flatProfileTrace{}
	_, _, top, roots, err := analyzeFlatExternalProfile(stdcontext.Background(), prior, 1000, false, nil, &trace)
	if err != nil || top == nil || trace.admission != "root-nested/linear" || trace.exact || trace.firstUnknown != "unproved general identifier selector" {
		t.Fatalf("prior invalidation: err=%v trace=%+v", err, trace)
	}
	if h := &top.owner.history; roots[0].name.text != "P" || h.reuse(roots[0]).uid != 304 || h.reuse(top).known || h.reuse(innerModule(t, top, "A")).known {
		t.Fatal("prefix or sticky invalidation")
	}

	a := acquireOverStandards(t, workspaceModule("Root", "EXTENDS Naturals\n"+h1RootBody), nil)
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	trace = flatProfileTrace{}
	if _, _, root, _, err := analyzeFlatExternalProfile(ctx, a, 1000, false, nil, &trace); err == nil || root != nil || trace.exact {
		t.Fatalf("cancelled run: err=%v trace=%+v", err, trace)
	}
	trace = flatProfileTrace{}
	if _, _, root, _, err := analyzeFlatExternalProfile(stdcontext.Background(), a, 4, false, nil, &trace); err == nil || root != nil || trace.admission != "root-nested/linear" || trace.exact {
		t.Fatalf("depth-limited run: err=%v trace=%+v", err, trace)
	}
	for _, stop := range []int{1, 2} {
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
		if stop == 1 && (err == nil || root != nil || trace.units != 1) {
			t.Fatalf("cancel after 1: err=%v units=%d", err, trace.units)
		}
		if stop == 2 && (err != nil || root == nil) {
			t.Fatalf("cancel after the last unit: err=%v", err)
		}
	}
}

// The predicate itself: cancellation, budget and depth stop the read-only
// inspection, and the ASCII premise.
func TestRootNestedInspectionBounds(t *testing.T) {
	a := acquireOverStandards(t, workspaceModule("Root", "EXTENDS Naturals\n"+h1RootBody), nil)
	order, err := a.Order(stdcontext.Background())
	if err != nil {
		t.Fatal(err)
	}
	docs := map[*modulegraph.Unit]*modulegraph.Document{}
	for _, d := range a.Units {
		docs[d.Unit] = d
	}
	if v := rootNestedRejection(stdcontext.Background(), a, order, docs); v.profile != "linear" || v.why != "" {
		t.Fatalf("verdict %+v", v)
	}
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	if v := rootNestedRejection(ctx, a, order, docs); v.why != rootNestedInspectionStopped {
		t.Fatalf("cancelled verdict %+v", v)
	}
	if _, ok := (&rootNestedInspection{ctx: stdcontext.Background(), maxDepth: 1000, left: 1}).innerModules(a.Root.RootModule); ok {
		t.Fatal("visit budget not enforced")
	}
	if _, ok := (&rootNestedInspection{ctx: stdcontext.Background(), maxDepth: 1, left: 100}).innerModules(a.Root.RootModule); ok {
		t.Fatal("depth limit not enforced")
	}
	if inner, ok := (&rootNestedInspection{ctx: stdcontext.Background(), maxDepth: 2, left: 2}).innerModules(a.Root.RootModule); !ok || len(inner) != 2 || inner[0].Name != "A" || inner[1].Name != "B" {
		t.Fatal("exact budget")
	}
	if v := rootNestedRejection(stdcontext.Background(), a, order[:1], docs); v.why != "dependency order does not cover acquired units" {
		t.Fatalf("coverage verdict %+v", v)
	}
	for s, want := range map[string]bool{"A": true, "Inner_1": true, "": false, "Iné": false, "Α": false} {
		if asciiModuleName(s) != want {
			t.Fatalf("asciiModuleName(%q) != %v", s, want)
		}
	}
	// The inspection left the acquisition untouched.
	if edgeText(a.Root) != "E:Naturals " || len(a.Root.References()) != 1 || len(a.Root.RootModule.Children) != 2 || len(a.Units) != 2 {
		t.Fatal("inspection changed the acquisition")
	}
}

// The test-local replay transcribes SpecObj.java:439-512, 619-701, 725-882
// and 930-983 and ParseUnit.java:377-467, 482-516 over module pointers. It
// takes module names, references and bodies from the parse trees; contexts,
// searches, propagation and vectors are its own.
type replayPointer struct {
	name      string
	unit      *replayUnit
	parent    *replayPointer
	inner     []*replayPointer
	ext, inst []string
	body      []*generated.SyntaxTreeNode
	context   map[string]*replayPointer
}

func (p *replayPointer) path() string {
	if p.parent == nil {
		return p.name
	}
	return p.parent.path() + "!" + p.name
}

type replayUnit struct {
	name                  string
	root                  *replayPointer
	extendees, instancees []*replayUnit
	extLog, instLog       []string
}

type nestedReplay struct {
	t          *testing.T
	docs       map[string]*modulegraph.Document
	units      map[string]*replayUnit
	all        []*replayPointer
	reduced    string // unit loaded as its reduced view
	reverse    bool   // enumerate getModules backwards (L-ID control)
	nextName   string
	nextOwner  *replayPointer
	extFound   bool
	instFound  bool
	selections []string
	acquired   []string
	order      []string
	steps      int
}

func (r *nestedReplay) step() {
	if r.steps++; r.steps > 1<<16 {
		r.t.Fatal("replay: unbounded")
	}
}

func bindIfNotBound(ctx map[string]*replayPointer, n string, p *replayPointer) {
	if ctx[n] == nil {
		ctx[n] = p
	}
}

func (r *nestedReplay) load(name string) *replayUnit {
	d := r.docs[name]
	if d == nil {
		r.t.Fatalf("replay: %s not acquired", name)
	}
	u := &replayUnit{name: name}
	r.units[name] = u
	r.acquired = append(r.acquired, name)
	u.root = r.determine(u, d.RootModule, nil, name == r.reduced)
	return u
}

// determine is ParseUnit.determineModuleRelationships (ParseUnit.java:482-516).
func (r *nestedReplay) determine(u *replayUnit, m *modulegraph.ModuleRef, parent *replayPointer, reduced bool) *replayPointer {
	r.step()
	p := &replayPointer{name: m.Name, unit: u, parent: parent, context: map[string]*replayPointer{}}
	r.all = append(r.all, p)
	if parent != nil {
		parent.inner = append(parent.inner, p)
	}
	for _, ref := range m.References() {
		if ref.Kind == modulegraph.Extends {
			p.ext = append(p.ext, ref.Name)
		}
	}
	r.calculateContext(p)
	body := m.Tree.Heirs()[2].Heirs()
	for _, ref := range m.References() {
		if ref.Kind == modulegraph.Instance && !(reduced && r.internal(body, ref.Name)) {
			p.inst = append(p.inst, ref.Name)
		}
	}
	if reduced {
		return p // V: no inner modules and no internal INSTANCE names
	}
	p.body = body
	for _, def := range body {
		if def.Kind() != generated.N_Module {
			continue
		}
		var child *modulegraph.ModuleRef
		for _, c := range m.Children {
			if c.Tree == def {
				child = c
			}
		}
		if child == nil {
			r.t.Fatal("replay: inner module tree")
		}
		r.determine(u, child, p, false)
	}
	return p
}

// calculateContext is ParseUnit.calculateContextWithinParseUnit (:437-467).
func (r *nestedReplay) calculateContext(p *replayPointer) {
	if p.parent != nil {
		for k, v := range p.parent.context {
			bindIfNotBound(p.context, k, v)
		}
	}
	for a := p; a.parent != nil; a = a.parent {
		for k, v := range a.parent.context {
			bindIfNotBound(p.context, k, v)
		}
		peers := a.parent.inner
		for i := 0; i < len(peers)-1; i++ {
			bindIfNotBound(p.context, peers[i].name, peers[i])
		}
	}
	r.handleExtensions(p, p)
}

// handleExtensions is ParseUnit.java:377-412, recursing inside the loop.
func (r *nestedReplay) handleExtensions(cur, other *replayPointer) {
	r.step()
	if other == nil {
		return
	}
	for _, n := range other.ext {
		if res := cur.context[n]; res != nil {
			for _, im := range res.inner {
				bindIfNotBound(cur.context, im.name, im)
				r.handleExtensions(cur, res)
			}
		}
	}
}

func treeImage(n *generated.SyntaxTreeNode) string {
	if n == nil || n.Image() == nil {
		return ""
	}
	return n.Image().String()
}

// internal is SpecObj.instanceResolvesToInternalModule (:528-600).
func (r *nestedReplay) internal(body []*generated.SyntaxTreeNode, name string) bool {
	seen := map[string]bool{}
	for _, def := range body {
		h := def.Heirs()
		switch def.Kind() {
		case generated.N_Module:
			seen[treeImage(h[0].Heirs()[1])] = true
		case generated.N_Instance, generated.N_ModuleDefinition:
			ix := 0
			if def.Kind() == generated.N_ModuleDefinition {
				ix = 2
			}
			if treeImage(h[0]) == "LOCAL" {
				ix++
			}
			if treeImage(h[ix].Heirs()[1]) == name {
				return seen[name]
			}
		}
	}
	return false
}

func (r *nestedReplay) findExtension(cur *replayPointer, visited map[*replayPointer]bool) bool {
	r.step()
	if cur == nil {
		r.t.Fatal("replay: null module in extension search")
	}
	if visited[cur] {
		r.extFound = false
		return false
	}
	visited[cur] = true
	for _, n := range cur.ext {
		if cur.context[n] == nil {
			r.nextName, r.nextOwner, r.extFound = n, cur, true
			return true
		}
	}
	for _, n := range cur.ext {
		if r.findExtension(cur.context[n], visited) {
			r.extFound = true
			return true
		}
	}
	for _, n := range cur.inst {
		if next := cur.context[n]; next != nil && r.findExtension(next, visited) {
			r.extFound = true
			return true
		}
	}
	for _, c := range cur.inner {
		if r.findExtension(c, visited) {
			r.extFound = true
			return true
		}
	}
	r.extFound = false
	return false
}

func (r *nestedReplay) findInstantiation(cur *replayPointer, visited map[*replayPointer]bool) bool {
	r.step()
	if cur == nil {
		r.t.Fatal("replay: null module in instantiation search")
	}
	if visited[cur] {
		r.instFound = false
		return false
	}
	visited[cur] = true
	for _, n := range cur.inst {
		if cur.context[n] == nil && !r.internal(cur.body, n) {
			r.nextName, r.nextOwner, r.instFound = n, cur, true
			return true
		}
	}
	for _, n := range cur.ext {
		if r.findInstantiation(cur.context[n], visited) {
			r.instFound = true
			return true
		}
	}
	for _, n := range cur.inst {
		if next := cur.context[n]; next != nil && r.findInstantiation(next, visited) {
			r.instFound = true
			return true
		}
	}
	for _, c := range cur.inner {
		if r.findInstantiation(c, visited) {
			r.instFound = true
			return true
		}
	}
	r.instFound = false
	return false
}

func (r *nestedReplay) modules() []*replayPointer {
	out := append([]*replayPointer(nil), r.all...)
	if r.reverse {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out
}

func (r *nestedReplay) directlyExtends(m1, m2 *replayPointer) bool {
	for _, n := range m1.ext {
		if m1.context[n] == m2 {
			return true
		}
	}
	return false
}

// indirectlyExtending is SpecObj.getModulesIndirectlyExtending (:725-769),
// including its in-loop bound updates.
func (r *nestedReplay) indirectlyExtending(module *replayPointer) []*replayPointer {
	extenders := []*replayPointer{module}
	additions, lastStart, lastEnd := true, 0, 1
	for additions {
		additions = false
		for i := lastStart; i < lastEnd; i++ {
			for _, mp := range r.modules() {
				r.step()
				if r.directlyExtends(mp, extenders[i]) {
					if !additions {
						lastStart = lastEnd
					}
					extenders = append(extenders, mp)
					additions = true
				}
			}
			lastStart = lastEnd
			lastEnd = len(extenders)
		}
	}
	return extenders
}

// moduleAndExtension is SpecObj.resolveNamesBetweenModuleAndExtention (:807-882).
func (r *nestedReplay) moduleAndExtension(ext *replayPointer, u *replayUnit) {
	r.step()
	for _, names := range [][]string{ext.ext, ext.inst} {
		for _, n := range names {
			for _, im := range u.root.inner {
				if n == im.name {
					bindIfNotBound(ext.context, n, im)
					break
				}
			}
		}
	}
	for _, c := range ext.inner {
		r.moduleAndExtension(c, u)
	}
}

func (r *nestedReplay) nonCircularity(target, candidate *replayUnit, visited map[*replayUnit]bool) {
	r.step()
	if visited[candidate] {
		return
	}
	visited[candidate] = true
	for _, x := range candidate.instancees {
		found := false
		for _, y := range candidate.extendees {
			found = found || y == x
		}
		if !found {
			candidate.extendees = append(candidate.extendees, x)
		}
	}
	for i := 0; i < len(candidate.extendees); i++ {
		if candidate.extendees[i] == target {
			r.t.Fatalf("replay: circular dependency through %s", target.name)
		}
		r.nonCircularity(target, candidate.extendees[i], visited)
	}
}

func (r *nestedReplay) dependencies(u *replayUnit) {
	r.step()
	for i := 0; i < len(u.extendees); i++ {
		r.dependencies(u.extendees[i])
	}
	for i := 0; i < len(u.instancees); i++ {
		r.dependencies(u.instancees[i])
	}
	for _, n := range r.order {
		if n == u.name {
			return
		}
	}
	r.order = append(r.order, u.name)
}

// replayNested runs SpecObj.loadSpec's loop (:930-983) over G, or over V when
// reduced names the root.
func replayNested(t *testing.T, a *modulegraph.Acquisition, reduced, reverse bool) *nestedReplay {
	t.Helper()
	r := &nestedReplay{t: t, docs: map[string]*modulegraph.Document{}, units: map[string]*replayUnit{}, reverse: reverse}
	if reduced {
		r.reduced = a.Root.Name
	}
	for _, d := range a.Units {
		r.docs[d.Name] = d
	}
	root := r.load(a.Root.Name).root
	for r.findExtension(root, map[*replayPointer]bool{}) || r.findInstantiation(root, map[*replayPointer]bool{}) {
		r.step()
		u := r.units[r.nextName]
		if u == nil {
			u = r.load(r.nextName)
		}
		owner := r.nextOwner
		kind := ""
		if r.extFound {
			kind += "E"
			owner.unit.extendees = append(owner.unit.extendees, u)
			owner.unit.extLog = append(owner.unit.extLog, u.name)
		}
		if r.instFound {
			kind += "I"
			owner.unit.instancees = append(owner.unit.instancees, u)
			owner.unit.instLog = append(owner.unit.instLog, u.name)
		}
		r.selections = append(r.selections, owner.path()+" "+kind+":"+u.name)
		r.nonCircularity(u, u, map[*replayUnit]bool{})
		if r.extFound {
			bindIfNotBound(owner.context, u.name, u.root)
			for _, s := range r.indirectlyExtending(owner) {
				r.moduleAndExtension(s, u)
			}
		}
		if r.instFound {
			bindIfNotBound(owner.context, u.name, u.root)
		}
	}
	r.dependencies(r.units[a.Root.Name])
	return r
}

// contextState renders every pointer's context as sorted name -> target.
func (r *nestedReplay) contextState(rootsOnly bool) map[string]string {
	out := map[string]string{}
	for _, p := range r.all {
		if rootsOnly && p.parent != nil {
			continue
		}
		var parts []string
		for k, v := range p.context {
			parts = append(parts, k+"="+v.unit.name+":"+v.path())
		}
		sort.Strings(parts)
		out[p.unit.name+":"+p.path()] = strings.Join(parts, ",")
	}
	return out
}

// Positive case 4: G against V, Java transcription against Go.
func TestRootNestedReplayGraphEqualsReducedView(t *testing.T) {
	single, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), modulegraph.Input{Path: "Root.tla", Text: []byte(singleNestedSource)}, nil, standardProfileLimits)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		a    *modulegraph.Acquisition
	}{
		{"H1", acquireOverStandards(t, workspaceModule("Root", "EXTENDS Naturals\n"+h1RootBody), nil)},
		{"single unit", single},
		{"flat", acquireOverStandards(t, workspaceModule("Root", "EXTENDS A, B\n"+flatNestedBody), flatNestedStaged())},
		{"standard", acquireOverStandards(t, workspaceModule("Root", "EXTENDS Sequences, FiniteSets\n"+standardNestedBody), nil)},
		{"standard over TLC", acquireOverStandards(t, workspaceModule("Root", "EXTENDS TLC\n---- MODULE Inner ----\nX == 1\n===="), nil)},
	} {
		t.Run(c.name, func(t *testing.T) {
			g, v := replayNested(t, c.a, false, false), replayNested(t, c.a, true, false)
			if !reflect.DeepEqual(g.selections, v.selections) || !reflect.DeepEqual(g.acquired, v.acquired) || !reflect.DeepEqual(g.order, v.order) {
				t.Fatalf("G selections %v acquired %v order %v\nV selections %v acquired %v order %v", g.selections, g.acquired, g.order, v.selections, v.acquired, v.order)
			}
			if gs, vs := g.contextState(true), v.contextState(true); !reflect.DeepEqual(gs, vs) {
				t.Fatalf("unit-root contexts\nG %v\nV %v", gs, vs)
			}
			// Every owner is a unit root, and R's inner contexts name only R's
			// inner modules.
			for _, s := range g.selections {
				if strings.Contains(strings.Fields(s)[0], "!") {
					t.Fatalf("inner owner selected: %s", s)
				}
			}
			for _, p := range g.all {
				if p.parent == nil {
					continue
				}
				for k, target := range p.context {
					if target.parent == nil || target.unit != p.unit {
						t.Fatalf("%s binds %s to %s:%s", p.path(), k, target.unit.name, target.path())
					}
				}
			}
			// L-ID: the reverse getModules enumeration changes nothing.
			back := replayNested(t, c.a, false, true)
			if !reflect.DeepEqual(back.selections, g.selections) || !reflect.DeepEqual(back.contextState(false), g.contextState(false)) || !reflect.DeepEqual(back.order, g.order) {
				t.Fatal("getModules enumeration order changed the replay")
			}
			// Java against Go: acquisition, per-unit vectors and postorder.
			if !reflect.DeepEqual(g.acquired, unitNames(c.a.Units)) {
				t.Fatalf("replayed acquisition %v, Go %v", g.acquired, unitNames(c.a.Units))
			}
			for _, d := range c.a.Units {
				u := g.units[d.Name]
				goExt, goInst := []string{}, []string{}
				for _, e := range d.Edges() {
					if e.Kind == modulegraph.Extends {
						goExt = append(goExt, e.Name)
					} else {
						goInst = append(goInst, e.Name)
					}
				}
				if !reflect.DeepEqual(append([]string{}, u.extLog...), goExt) || !reflect.DeepEqual(append([]string{}, u.instLog...), goInst) {
					t.Fatalf("%s Java vectors %v %v, Go edges %v %v", d.Name, u.extLog, u.instLog, goExt, goInst)
				}
			}
			units, err := c.a.Order(stdcontext.Background())
			if err != nil {
				t.Fatal(err)
			}
			var goOrder []string
			for _, u := range units {
				for _, d := range c.a.Units {
					if d.Unit == u {
						goOrder = append(goOrder, d.Name)
					}
				}
			}
			if !reflect.DeepEqual(g.order, goOrder) {
				t.Fatalf("postorder Java %v, Go %v", g.order, goOrder)
			}
		})
	}
}
