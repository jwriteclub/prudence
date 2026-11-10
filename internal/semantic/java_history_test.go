// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"testing"

	"github.com/jwriteclub/prudence/internal/modulegraph"
)

func TestHistoryBootstrapAndModule(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	if !h.exact || h.count != 305 {
		t.Fatalf("history: exact=%v count=%d reason=%q", h.exact, h.count, h.firstUnknown)
	}
	if got := h.reuse(m); !got.known || got.uid != 304 || got.class != "ModuleNode" || got.kind != 1 {
		t.Fatalf("module ticket: %#v", got)
	}
	if got := h.reuse(g.nullOAN); !got.known || got.uid != 301 || got.class != "OpApplNode" {
		t.Fatalf("recovery ticket: %#v", got)
	}
	for _, c := range []struct {
		name string
		uid  uint32
	}{{"STRING", 150}, {"TRUE", 152}, {"=", 154}} {
		n := g.pool.builtins.lookup(contextKey{name: g.pool.intern(c.name)})
		if got := h.reuse(n); !got.known || got.uid != c.uid {
			t.Fatalf("%s: %#v", c.name, got)
		}
	}
	eq := g.pool.builtins.lookup(contextKey{name: g.pool.intern("=")}).(*builtinOpDefNode)
	if h.reuse(eq.params[0]).uid != 155 || h.reuse(eq.params[1]).uid != 156 {
		t.Fatal("formal schedule")
	}
}
func TestHistorySecondGeneratorAndUnknown(t *testing.T) {
	p := newNamePool()
	p.history = freshJavaHistory(stdcontext.Background())
	g, err := newOwnedGenerator(stdcontext.Background(), 0, p, make(map[*name]leafNode), &Errors{})
	if err != nil {
		t.Fatal(err)
	}
	g2, err := newOwnedGenerator(stdcontext.Background(), 0, p, make(map[*name]leafNode), &Errors{})
	if err != nil {
		t.Fatal(err)
	}
	if p.history.reuse(g.nullOAN).uid != 301 || p.history.reuse(g2.nullOAN).uid != 305 || p.history.count != 308 {
		t.Fatalf("generator schedule: %d", p.history.count)
	}
	p.history.invalidate("unproved path")
	before := p.history.reuse(g.nullOAN)
	if p.history.event("OpDefNode", 5).known || p.history.exact || p.history.firstUnknown != "unproved path" || p.history.reuse(g.nullOAN) != before {
		t.Fatal("invalidation lost prefix or resumed")
	}
}
func TestHistoryUnprofiledAndBudget(t *testing.T) {
	p := newNamePool()
	g, err := newOwnedGenerator(stdcontext.Background(), 0, p, make(map[*name]leafNode), &Errors{})
	if err != nil || p.history.reuse(g.nullOAN).known {
		t.Fatal("unprofiled setup certified")
	}
	h := freshJavaHistory(stdcontext.Background())
	h.workLimit = h.count
	if h.event("ModuleNode", 1).known || h.exact || h.firstUnknown == "" {
		t.Fatal("budget not enforced")
	}
	a, b := freshJavaHistory(stdcontext.Background()), freshJavaHistory(stdcontext.Background())
	a.invalidate("unknown")
	if !b.exact || b.count != 300 {
		t.Fatal("owners shared state")
	}
}
func TestHistoryUnknownBodyRetainsPrefix(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF == <<TRUE,FALSE>>\nG == F!1!Missing\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if g.pool.history.exact || g.pool.history.firstUnknown != "unproved general identifier selector" || g.pool.history.reuse(m).uid != 304 {
		t.Fatal("unknown branch or prefix")
	}
}

func TestHistoryDefinitionEvaluationOrder(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF(a,b) == L(b,a)::TRUE\nG == 42\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	if !h.exact || h.count != 312 {
		t.Fatalf("exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
	defs := testUserDefinitions(t, m.definitions)
	f, gdef := defs[0], defs[1]
	if h.reuse(f.params[0]).uid != 305 || h.reuse(f.params[1]).uid != 306 || h.reuse(f.body).uid != 308 || h.reuse(f).uid != 309 || h.reuse(gdef.body).uid != 310 || h.reuse(gdef).uid != 311 {
		t.Fatal("source evaluation order")
	}
	label := f.body.(*labelNode)
	if h.reuse(label.body).uid != 307 {
		t.Fatal("body precedes label")
	}
}

func TestHistoryArgumentsBeforeApplication(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF == (TRUE /\\ FALSE)\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	f := testUserDefinitions(t, m.definitions)[0]
	app := f.body.(*opApplNode)
	if !h.exact || h.count != 309 || h.reuse(app.operands[0]).uid != 305 || h.reuse(app.operands[1]).uid != 306 || h.reuse(app).uid != 307 || h.reuse(f).uid != 308 {
		t.Fatalf("order count=%d exact=%v reason=%s", h.count, h.exact, h.firstUnknown)
	}
}

func TestHistoryDeclarationsAndNestedGeneratorReuse(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE Root ----\nCONSTANT A, B\nVARIABLE v\n---- MODULE Inner ----\nX == TRUE\n====\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	for _, c := range []struct {
		name string
		uid  uint32
		kind int
	}{{"A", 305, 2}, {"B", 306, 2}, {"v", 307, 3}} {
		n := m.context.lookup(contextKey{name: g.pool.intern(c.name)})
		got := h.reuse(n)
		if !got.known || got.uid != c.uid || got.kind != c.kind {
			t.Fatalf("%s: %#v", c.name, got)
		}
	}
	inner := m.bodyDefinitions[0].(*moduleNode)
	x := testUserDefinitions(t, inner.definitions)[0]
	if !h.exact || h.reuse(inner).uid != 308 || h.reuse(x.body).uid != 309 || h.reuse(x).uid != 310 || h.count != 311 {
		t.Fatalf("nested trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryRecursiveDiscardedAndReused(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nRECURSIVE F(_)\nF(x) == x\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	f := testUserDefinitions(t, m.definitions)[0]
	if !h.exact || h.count != 310 || h.reuse(f).uid != 307 || h.reuse(f.params[0]).uid != 308 || h.reuse(f.body).uid != 309 {
		t.Fatalf("recursive trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryFunctionPlaceholdersBeforeBody(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF[x \\in {1}] == x\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	f := testUserDefinitions(t, m.definitions)[0]
	spec := f.body.(*opApplNode)
	bound := spec.ranges[0].(*opApplNode)
	if !h.exact || h.count != 312 || h.reuse(bound.operands[0]).uid != 305 || h.reuse(bound).uid != 306 || h.reuse(spec.bounded[0][0]).uid != 307 || h.reuse(spec).uid != 309 || h.reuse(f).uid != 310 || h.reuse(spec.operands[0]).uid != 311 {
		t.Fatalf("function trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryOperatorApplicationArguments(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nId(x) == x\nUse == Id(TRUE)\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	defs := testUserDefinitions(t, m.definitions)
	app := defs[1].body.(*opApplNode)
	if !h.exact || h.count != 311 || h.reuse(defs[0].params[0]).uid != 305 || h.reuse(defs[0].body).uid != 306 || h.reuse(defs[0]).uid != 307 || h.reuse(app.operands[0]).uid != 308 || h.reuse(app).uid != 309 || h.reuse(defs[1]).uid != 310 {
		t.Fatalf("application trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryIllegalLabelArgumentsAllocateSeparateDummies(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF == L(x,x)::TRUE\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	f := testUserDefinitions(t, m.definitions)[0]
	label := f.body.(*labelNode)
	if !h.exact || h.count != 310 || h.reuse(label.body).uid != 305 || h.reuse(label.params[0]).uid != 306 || h.reuse(label.params[1]).uid != 307 || h.reuse(label).uid != 308 || h.reuse(f).uid != 309 {
		t.Fatalf("dummy trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryStatementConstructionOrder(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nASSUME A == TRUE\nTHEOREM T == FALSE OBVIOUS\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	a := m.assumptions[0]
	th := m.theorems[0]
	if !h.exact || h.count != 312 || h.reuse(a.expression).uid != 305 || h.reuse(a.definition).uid != 306 || h.reuse(a).uid != 307 || h.reuse(th.definition).uid != 308 || h.reuse(th.body).uid != 309 || h.reuse(th.proof).uid != 310 || h.reuse(th).uid != 311 {
		t.Fatalf("statement trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryAssumeProveNewAndStaticReuse(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nTHEOREM T == ASSUME NEW x \\in {1} PROVE x = x\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	th := m.theorems[0]
	ap := th.body.(*assumeProveNode)
	ns := ap.assumptions[0].(*newSymbNode)
	bound := ns.set.(*opApplNode)
	eq := ap.prove.(*opApplNode)
	if !h.exact || h.count != 315 || h.reuse(th.definition).uid != 305 || h.reuse(ap).uid != 306 || h.reuse(bound.operands[0]).uid != 307 || h.reuse(bound).uid != 308 || h.reuse(ns.declaration).uid != 309 || h.reuse(ns).uid != 310 || h.reuse(eq.operands[0]).uid != 311 || h.reuse(eq.operands[1]).uid != 312 || h.reuse(eq).uid != 313 || h.reuse(th).uid != 314 || h.reuse(g.inAssume).uid != 299 {
		t.Fatalf("AP trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryParameterFreeInstances(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE Root ----\n---- MODULE Inner ----\nX == TRUE\n====\nINSTANCE Inner\nI == INSTANCE Inner\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	inner := m.bodyDefinitions[0].(*moduleNode)
	original := testUserDefinitions(t, inner.definitions)[0]
	clone := m.context.lookup(contextKey{name: g.pool.intern("I!X")})
	marker := m.context.lookup(contextKey{name: g.pool.intern("I")})
	if !h.exact || h.count != 314 || h.reuse(inner).uid != 305 || h.reuse(original.body).uid != 306 || h.reuse(original).uid != 307 || h.reuse(m.instances[0]).uid != 309 || h.reuse(clone).uid != 311 || h.reuse(m.instances[1]).uid != 312 || h.reuse(marker).uid != 313 {
		t.Fatalf("instance trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
	if m.context.lookup(contextKey{name: g.pool.intern("X")}) != original {
		t.Fatal("unnamed instance did not share original")
	}
}

func TestHistoryExplicitInstanceSubstitution(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE Root ----\n---- MODULE Inner ----\nCONSTANT C\nX == C\n====\nINSTANCE Inner WITH C <- 42\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	inner := m.bodyDefinitions[0].(*moduleNode)
	original := testUserDefinitions(t, inner.definitions)[0]
	clone := m.context.lookup(contextKey{name: g.pool.intern("X")}).(*userOpDefNode)
	wrap := clone.body.(*substInNode)
	if !h.exact || h.count != 314 || h.reuse(inner).uid != 305 || h.reuse(inner.context.lookup(contextKey{name: g.pool.intern("C")})).uid != 306 || h.reuse(original.body).uid != 307 || h.reuse(original).uid != 308 || h.reuse(wrap.substs[0].expr).uid != 310 || h.reuse(wrap).uid != 311 || h.reuse(clone).uid != 312 || h.reuse(m.instances[0]).uid != 313 {
		t.Fatalf("substitution trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryRecordSyntheticOrdering(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF == [a |-> TRUE]\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	f := testUserDefinitions(t, m.definitions)[0]
	rec := f.body.(*opApplNode)
	pair := rec.operands[0].(*opApplNode)
	if !h.exact || h.count != 310 || h.reuse(pair.operands[0]).uid != 305 || h.reuse(pair.operands[1]).uid != 306 || h.reuse(pair).uid != 307 || h.reuse(rec).uid != 308 || h.reuse(f).uid != 309 {
		t.Fatalf("record trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryChoiceRangeBeforeBoundAndBody(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF == CHOOSE x \\in {1}: x = 1\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	f := testUserDefinitions(t, m.definitions)[0]
	choose := f.body.(*opApplNode)
	rangeSet := choose.ranges[0].(*opApplNode)
	body := choose.operands[0].(*opApplNode)
	if !h.exact || h.count != 313 || h.reuse(rangeSet.operands[0]).uid != 305 || h.reuse(rangeSet).uid != 306 || h.reuse(choose.bounded[0][0]).uid != 307 || h.reuse(body.operands[0]).uid != 308 || h.reuse(body.operands[1]).uid != 309 || h.reuse(body).uid != 310 || h.reuse(choose).uid != 311 || h.reuse(f).uid != 312 {
		t.Fatalf("choice trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryExceptBeforeAt(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF == [[a |-> 1] EXCEPT !.a = @]\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	f := testUserDefinitions(t, m.definitions)[0]
	outer := f.body.(*opApplNode)
	rec := outer.operands[0].(*opApplNode)
	pair := outer.operands[1].(*opApplNode)
	seq := pair.operands[0].(*opApplNode)
	if !h.exact || h.count != 315 || h.reuse(rec).uid != 308 || h.reuse(outer).uid != 309 || h.reuse(seq.operands[0]).uid != 310 || h.reuse(seq).uid != 311 || h.reuse(pair).uid != 312 || h.reuse(pair.operands[1]).uid != 313 || h.reuse(f).uid != 314 {
		t.Fatalf("except trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryExternalChainPublicationOrder(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("Root", "EXTENDS A\nG == F"), func(_ stdcontext.Context, n string) (modulegraph.Input, error) { return input("A", "F == TRUE"), nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	_, _, root, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil {
		t.Fatal(err)
	}
	h := &root.owner.history
	dep := root.extendees[0]
	f := testUserDefinitions(t, dep.definitions)[0]
	defs := testUserDefinitions(t, root.definitions)
	g := defs[len(defs)-1]
	if !h.exact || h.count != 314 || h.reuse(dep).uid != 304 || h.reuse(f.body).uid != 305 || h.reuse(f).uid != 306 || h.reuse(root).uid != 311 || h.reuse(g.body).uid != 312 || h.reuse(g).uid != 313 {
		t.Fatalf("external trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryBoundaryAndTicketBudget(t *testing.T) {
	h := freshJavaHistory(stdcontext.Background())
	h.workLimit = uint64(1) << 32
	h.count = (uint64(1) << 32) - 1
	if h.event("OpDefNode", 5).known || h.exact || h.count != (uint64(1)<<32)-1 {
		t.Fatal("UID boundary")
	}
	h = freshJavaHistory(stdcontext.Background())
	h.ticketLimit = 1
	first := new(formalParamNode)
	second := new(formalParamNode)
	t1 := h.event("FormalParamNode", 11)
	h.attach(first, t1)
	h.attach(second, h.event("FormalParamNode", 11))
	if h.exact || h.firstUnknown != "ticket budget" || h.reuse(first) != t1 || h.reuse(second).known {
		t.Fatal("ticket budget")
	}
	c, cancel := stdcontext.WithCancel(stdcontext.Background())
	h = freshJavaHistory(c)
	cancel()
	if h.event("OpDefNode", 5).known || h.firstUnknown != "cancelled" {
		t.Fatal("cancellation")
	}
}

func TestHistoryHigherOrderAndLambdaArguments(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nUse(Op(_)) == Op(TRUE)\nF(x) == x\nR == Use(F)\nS == Use(LAMBDA y: y)\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	defs := testUserDefinitions(t, m.definitions)
	r := defs[2].body.(*opApplNode)
	app := defs[3].body.(*opApplNode)
	opArg := app.operands[0].(*opArgNode)
	lambda := opArg.operator.(*userOpDefNode)
	if !h.exact || h.count != 321 || h.reuse(defs[0].params[0]).uid != 305 || h.reuse(r.operands[0]).uid != 312 || h.reuse(r).uid != 313 || h.reuse(defs[2]).uid != 314 || h.reuse(lambda.params[0]).uid != 315 || h.reuse(lambda.body).uid != 316 || h.reuse(lambda).uid != 317 {
		t.Fatalf("higher order trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryConstructorDiagnosticAndRecoveryReuse(t *testing.T) {
	source := "---- MODULE M ----\nUse(Op(_)) == Op(TRUE)\nHigher(F(_)) == TRUE\nR == Use(Higher)\nBad == @\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	defs := testUserDefinitions(t, m.definitions)
	r := defs[2].body.(*opApplNode)
	if !h.exact || len(g.table.errors.Messages()) < 2 || h.count != 318 || h.reuse(r.operands[0]).uid != 312 || h.reuse(r).uid != 313 || h.reuse(defs[2]).uid != 314 || defs[3].body != g.nullOAN || h.reuse(defs[3]).uid != 315 || h.reuse(defs[4].body).uid != 316 || h.reuse(defs[4]).uid != 317 {
		t.Fatalf("constructor/recovery trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryNamedParameterizedInstance(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE Root ----\n---- MODULE Inner ----\nCONSTANT C\nX == C\n====\nI(y) == INSTANCE Inner WITH C <- y\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	instance := m.instances[0]
	clone := m.context.lookup(contextKey{name: g.pool.intern("I!X")}).(*userOpDefNode)
	marker := m.context.lookup(contextKey{name: g.pool.intern("I")})
	wrap := clone.body.(*substInNode)
	if !h.exact || h.count != 316 || h.reuse(instance.params[0]).uid != 309 || h.reuse(wrap.substs[0].expr).uid != 311 || h.reuse(wrap).uid != 312 || h.reuse(clone).uid != 313 || h.reuse(instance).uid != 314 || h.reuse(marker).uid != 315 || clone.params[0] != instance.params[0] {
		t.Fatalf("named substitution exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryZeroArgumentInstanceSelector(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE Root ----\n---- MODULE Inner ----\nX == TRUE\n====\nI == INSTANCE Inner\nR == I!X\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	r := testUserDefinitions(t, m.definitions)[len(testUserDefinitions(t, m.definitions))-1]
	if !h.exact || h.count != 314 || h.reuse(r.body).uid != 312 || h.reuse(r).uid != 313 {
		t.Fatalf("selector trace exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryDirectLabelSelectorWrapper(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF == L::TRUE\nR == F!L\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	defs := testUserDefinitions(t, m.definitions)
	label := defs[0].body.(*labelNode)
	wrapper := defs[1].body.(*opApplNode)
	if !h.exact || h.count != 310 || h.reuse(label.body).uid != 305 || h.reuse(label).uid != 306 || h.reuse(defs[0]).uid != 307 || h.reuse(wrapper).uid != 308 || h.reuse(defs[1]).uid != 309 || wrapper.operands[0] != label {
		t.Fatalf("label selector exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryParameterizedLabelSelectorWrappers(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF == L(x)::TRUE\nR == F!L(1)\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	defs := testUserDefinitions(t, m.definitions)
	label := defs[0].body.(*labelNode)
	app := defs[1].body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	if !h.exact || h.count != 313 || h.reuse(label.body).uid != 305 || h.reuse(label.params[0]).uid != 306 || h.reuse(label).uid != 307 || h.reuse(defs[0]).uid != 308 || h.reuse(app.operands[0]).uid != 309 || h.reuse(lambda).uid != 310 || h.reuse(app).uid != 311 || h.reuse(defs[1]).uid != 312 || lambda.params[0] != label.params[0] {
		t.Fatalf("parameterized label selector exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryInvalidAttachmentDoesNotPanic(t *testing.T) {
	h := freshJavaHistory(stdcontext.Background())
	ticket := h.event("FormalParamNode", 11)
	h.attach([]int{1}, ticket)
	if h.exact || h.firstUnknown != "invalid attachment" || h.reuse([]int{1}).known {
		t.Fatal("invalid attachment accepted")
	}
}

func TestHistoryFlatExtendsSiblingOrder(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	files := map[string]modulegraph.Input{"A": input("A", "X == TRUE"), "B": input("B", "Y == FALSE")}
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("Root", "EXTENDS A, B\nG == TRUE"), func(_ stdcontext.Context, n string) (modulegraph.Input, error) { return files[n], nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	_, _, root, roots, err := analyzeFlatExternalRoots(stdcontext.Background(), a, 1000)
	if err != nil {
		t.Fatal(err)
	}
	h := &root.owner.history
	if !h.exact || h.count != 321 || len(roots) != 3 || h.reuse(roots[0]).uid != 304 || h.reuse(roots[1]).uid != 311 || h.reuse(root).uid != 318 {
		t.Fatalf("sibling order exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

// The general driver admits an inner module's EXTENDS of a unit (GD-H5).
func TestHistoryExternalInnerReferencesProfiled(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("Root", "---- MODULE Inner ----\nEXTENDS A\n===="), func(_ stdcontext.Context, n string) (modulegraph.Input, error) { return input("A", "X == TRUE"), nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	_, _, root, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if !root.owner.history.exact || root.owner.history.profile != freshPhase2Profile {
		t.Fatalf("inner dependency history profile=%d exact=%v first=%q", root.owner.history.profile, root.owner.history.exact, root.owner.history.firstUnknown)
	}
}

func TestHistoryNestedLabelsAfterBodies(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF == Outer::Inner::(TRUE /\\ FALSE)\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	f := testUserDefinitions(t, m.definitions)[0]
	outer := f.body.(*labelNode)
	inner := outer.body.(*labelNode)
	body := inner.body.(*opApplNode)
	if !h.exact || h.count != 311 || h.reuse(body.operands[0]).uid != 305 || h.reuse(body.operands[1]).uid != 306 || h.reuse(body).uid != 307 || h.reuse(inner).uid != 308 || h.reuse(outer).uid != 309 || h.reuse(f).uid != 310 {
		t.Fatalf("nested labels exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryRecursiveFunctionReusesPlaceholder(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nRECURSIVE F\nF[x \\in {1}] == x\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	f := testUserDefinitions(t, m.definitions)[0]
	spec := f.body.(*opApplNode)
	bound := spec.ranges[0].(*opApplNode)
	if !h.exact || h.count != 313 || h.reuse(f).uid != 306 || h.reuse(bound.operands[0]).uid != 307 || h.reuse(bound).uid != 308 || h.reuse(spec.bounded[0][0]).uid != 309 || h.reuse(spec).uid != 311 || h.reuse(spec.operands[0]).uid != 312 {
		t.Fatalf("recursive function exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistorySelectorArgumentsAndSharedFormals(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF(a) == L(a)::TRUE\nR == F(1)!L(2)\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	defs := testUserDefinitions(t, m.definitions)
	label := defs[0].body.(*labelNode)
	app := defs[1].body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	if !h.exact || h.count != 314 || h.reuse(defs[0].params[0]).uid != 305 || h.reuse(label.body).uid != 306 || h.reuse(label).uid != 307 || h.reuse(defs[0]).uid != 308 || h.reuse(app.operands[0]).uid != 309 || h.reuse(app.operands[1]).uid != 310 || h.reuse(lambda).uid != 311 || h.reuse(app).uid != 312 || h.reuse(defs[1]).uid != 313 || lambda.params[0] != lambda.params[1] {
		t.Fatalf("selector args exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryInstanceEnumerationAndStatementClones(t *testing.T) {
	src := "---- MODULE Root ----\n---- MODULE Inner ----\nCONSTANT C, D\nX == C\nY == D\nTHEOREM T == C = C\n====\nINSTANCE Inner WITH C <- 1, D <- 2\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, src), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	x := m.context.lookup(contextKey{name: g.pool.intern("X")}).(*userOpDefNode)
	y := m.context.lookup(contextKey{name: g.pool.intern("Y")}).(*userOpDefNode)
	th := m.context.lookup(contextKey{name: g.pool.intern("T")}).(*thmOrAssumpDefNode)
	inst := m.instances[0]
	if !h.exact || h.count != 327 || h.reuse(inst.substs[0].expr).uid != 318 || h.reuse(inst.substs[1].expr).uid != 319 || h.reuse(y.body).uid != 320 || h.reuse(y).uid != 321 || h.reuse(x.body).uid != 322 || h.reuse(x).uid != 323 || h.reuse(th.body).uid != 324 || h.reuse(th).uid != 325 || h.reuse(inst).uid != 326 {
		t.Fatalf("instance enumeration exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryHigherOrderInstanceSubstitution(t *testing.T) {
	src := "---- MODULE Root ----\nF(x) == x\n---- MODULE Inner ----\nCONSTANT Op(_)\nX == Op(TRUE)\n====\nI == INSTANCE Inner WITH Op <- F\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, src), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	clone := m.context.lookup(contextKey{name: g.pool.intern("I!X")}).(*userOpDefNode)
	wrap := clone.body.(*substInNode)
	inst := m.instances[0]
	marker := m.context.lookup(contextKey{name: g.pool.intern("I")})
	if !h.exact || h.count != 319 || h.reuse(inst.substs[0].expr).uid != 314 || h.reuse(wrap).uid != 315 || h.reuse(clone).uid != 316 || h.reuse(inst).uid != 317 || h.reuse(marker).uid != 318 {
		t.Fatalf("operator substitution exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryDirectNumericSelectorWrapper(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF == TRUE /\\ FALSE\nR == F!1\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	defs := testUserDefinitions(t, m.definitions)
	app := defs[1].body.(*opApplNode)
	if !h.exact || h.count != 311 || h.reuse(defs[0]).uid != 308 || h.reuse(app).uid != 309 || h.reuse(defs[1]).uid != 310 {
		t.Fatalf("numeric selector exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
}

// Generator.selectorToNode checks expected arity before constructing an OpArgNode;
// generateOpArg returns its existing nullOpArg on the error path.
func TestHistorySubstitutionArityRecoveryReusesNull(t *testing.T) {
	prefix := "---- MODULE Root ----\nF(x) == x\n---- MODULE Inner ----\nCONSTANT Op(_,_)\nX == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, prefix+"I == INSTANCE Inner WITH Op <- F\nAfter == TRUE\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	defs := testUserDefinitions(t, m.definitions)
	after := defs[len(defs)-1]
	if !h.exact || h.count != 319 || h.reuse(after.body).uid != 317 || h.reuse(after).uid != 318 || len(g.table.errors.Messages()) == 0 {
		t.Fatalf("recovery exact=%v count=%d reason=%s diagnostics=%v", h.exact, h.count, h.firstUnknown, g.table.errors.Messages())
	}
	if len(m.instances) != 1 || len(m.instances[0].substs) != 1 || m.instances[0].substs[0].expr != g.nullOpArg {
		t.Fatal("arity recovery did not reuse the generator null operator argument")
	}
}

// Generator.selectorToNode resolves the fully qualified operator before the
// arity check; successful selection constructs one OpArg, mismatch none.
func TestHistoryQualifiedSubstitutionOperator(t *testing.T) {
	for _, tc := range []struct {
		name, decl string
		mismatch   bool
	}{
		{"matched", "Op(_,_)", false}, {"mismatch", "Op(_)", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE Root ----\n---- MODULE A ----\nCONSTANT X\nF(y) == <<X,y>>\n====\n---- MODULE B ----\nCONSTANT " + tc.decl + "\nG == TRUE\n====\nI(p) == INSTANCE A WITH X <- p\nINSTANCE B WITH Op <- I!F\nAfter == TRUE\n====\n"
			g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
			if err != nil {
				t.Fatal(err)
			}
			h := &g.pool.history
			if !h.exact || len(m.instances) != 2 {
				t.Fatalf("exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
			}
			rhs := m.instances[1].substs[0].expr
			if tc.mismatch {
				if rhs != g.nullOpArg || len(g.table.errors.Messages()) == 0 {
					t.Fatal("mismatch did not reuse null argument")
				}
			} else {
				arg, ok := rhs.(*opArgNode)
				if !ok || arg.operator != m.context.lookup(contextKey{name: g.pool.intern("I!F")}) || !h.reuse(arg).known {
					t.Fatal("qualified selection not attached")
				}
			}
			after := m.context.lookup(contextKey{name: g.pool.intern("After")}).(*userOpDefNode)
			want := uint32(328)
			if tc.mismatch {
				want = 327
			}
			if h.reuse(after.body).uid != want || h.reuse(after).uid != want+1 || h.count != uint64(want)+2 {
				t.Fatal("qualified constructor count")
			}
		})
	}
}

// selectorToNode clones the selected LET operator's formals, applies it,
// recreates substitution prefixes, then constructs a lambda and OpArg.
func TestHistorySubstitutionLetSelectorClonesAndWraps(t *testing.T) {
	src := "---- MODULE Root ----\n---- MODULE A ----\nCONSTANT X\nF == LET G(y) == <<X,y>> IN G(1)\n====\n---- MODULE B ----\nCONSTANT Op(_)\nH == TRUE\n====\nINSTANCE A WITH X <- 1\nINSTANCE B WITH Op <- F!:!G\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, src), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	if !h.exact || len(m.instances) != 2 {
		t.Fatalf("exact=%v count=%d reason=%s", h.exact, h.count, h.firstUnknown)
	}
	f := m.context.lookup(contextKey{name: g.pool.intern("F")}).(*userOpDefNode)
	original := f.body.(*substInNode)
	arg := m.instances[1].substs[0].expr.(*opArgNode)
	lambda := arg.operator.(*userOpDefNode)
	wrapper := lambda.body.(*substInNode)
	app := wrapper.body.(*opApplNode)
	cloned := lambda.params[0]
	operand := app.operands[0].(*opApplNode)
	if wrapper.substs[0] != original.substs[0] || operand.operator != cloned || !h.reuse(cloned).known || !h.reuse(operand).known || !h.reuse(app).known || !h.reuse(wrapper).known || !h.reuse(lambda).known || !h.reuse(arg).known {
		t.Fatal("clones, wrappers or shared substitutions not retained")
	}
	if h.reuse(cloned).uid != 326 || h.reuse(operand).uid != 327 || h.reuse(app).uid != 328 || h.reuse(wrapper).uid != 329 || h.reuse(lambda).uid != 330 || h.reuse(arg).uid != 331 || h.count != 337 {
		t.Fatal("selector constructor order")
	}
}

// processFunction allocates a self formal and function application even for
// existing user definitions; erroneous recursive arity does not allocate a
// replacement definition and its body still runs.
func TestHistoryExistingFunctionErrorEvents(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"recursive arity", "RECURSIVE F(_)\nF[x \\in {1}] == TRUE"},
		{"redefinition", "F == TRUE\nF[x \\in {1}] == TRUE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\n"+tc.body+"\nAfter == TRUE\n====\n"), 0)
			if err != nil {
				t.Fatal(err)
			}
			h := &g.pool.history
			if !h.exact || len(g.table.errors.Messages()) == 0 {
				t.Fatalf("exact=%v reason=%s", h.exact, h.firstUnknown)
			}
			after := m.context.lookup(contextKey{name: g.pool.intern("After")}).(*userOpDefNode)
			want := uint32(314)
			if tc.name == "recursive arity" {
				want = 315
			}
			if h.reuse(after).uid != want || h.count != uint64(want)+1 {
				t.Fatal("discarded function constructor order")
			}
		})
	}
}

func TestHistorySubstitutionLetSelectorOperatorFormal(t *testing.T) {
	src := "---- MODULE Root ----\n---- MODULE A ----\nCONSTANT X\nF == LET G(P(_)) == TRUE IN TRUE\n====\n---- MODULE B ----\nCONSTANT Op(_)\nH == TRUE\n====\nINSTANCE A WITH X <- 1\nINSTANCE B WITH Op <- F!:!G\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, src), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	if !h.exact {
		t.Fatalf("reason=%s", h.firstUnknown)
	}
	arg := m.instances[1].substs[0].expr.(*opArgNode)
	lambda := arg.operator.(*userOpDefNode)
	wrapper := lambda.body.(*substInNode)
	app := wrapper.body.(*opApplNode)
	clone := lambda.params[0]
	operand := app.operands[0].(*opArgNode)
	if clone.arity != 1 || operand.operator != clone {
		t.Fatal("operator clone identity")
	}
	c := h.reuse(clone).uid
	if !h.reuse(clone).known || h.reuse(operand).uid != c+1 || h.reuse(app).uid != c+2 || h.reuse(wrapper).uid != c+3 || h.reuse(lambda).uid != c+4 || h.reuse(arg).uid != c+5 {
		t.Fatal("operator clone construction order")
	}
}

// Generator generates a BY UseOrHideNode before its LeafProofNode, even
// though only the latter survives in the theorem graph.
func TestHistoryProofLeafAndDiscardedBY(t *testing.T) {
	for _, tc := range []struct {
		proof                string
		leaf, theorem, count uint64
	}{
		{"OBVIOUS", 306, 307, 308},
		{"BY MODULE M", 307, 308, 309},
	} {
		g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE "+tc.proof+"\n====\n"), 0)
		if err != nil {
			t.Fatal(err)
		}
		h := &g.pool.history
		th := m.theorems[0]
		if !h.exact || uint64(h.reuse(th.proof).uid) != tc.leaf || uint64(h.reuse(th).uid) != tc.theorem || h.count != tc.count {
			t.Fatalf("%s: exact=%v reason=%s count=%d leaf=%#v theorem=%#v", tc.proof, h.exact, h.firstUnknown, h.count, h.reuse(th.proof), h.reuse(th))
		}
	}
}

func TestHistoryStructuredProofDiscardedPlaceholder(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. USE MODULE M\n<1> QED OBVIOUS\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	pf := m.theorems[0].proof.(*nonLeafProofNode)
	use := pf.steps[0].(*useOrHideNode)
	alias := pf.context.lookup(contextKey{name: g.pool.intern("<1>1")})
	if !h.exact || h.reuse(use).uid != 307 || h.reuse(alias).uid != 308 || h.reuse(pf).uid <= h.reuse(alias).uid {
		t.Fatalf("exact=%v reason=%s use=%#v alias=%#v proof=%#v", h.exact, h.firstUnknown, h.reuse(use), h.reuse(alias), h.reuse(pf))
	}
}

func TestHistoryProofSelectorInvalidatesOnlyItsOwner(t *testing.T) {
	source := "---- MODULE M ----\nF == <<TRUE, FALSE>>\nTHEOREM TRUE BY F!1!Missing\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	old := h.reuse(m)
	if h.exact || h.firstUnknown != "unproved compound fact selector history" || !old.known || h.reuse(m) != old || h.event("TheoremNode", 12).known {
		t.Fatalf("history resumed: exact=%v reason=%q ticket=%#v", h.exact, h.firstUnknown, old)
	}
	other, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE N ----\nTHEOREM TRUE OBVIOUS\n====\n"), 0)
	if err != nil || !other.pool.history.exact {
		t.Fatal("fresh owner inherited invalidation", err)
	}
}

func TestHistoryDirectProofFactAndDef(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF == TRUE\nTHEOREM TRUE BY F DEF F\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	leaf := m.theorems[0].proof.(*leafProofNode)
	if !h.exact || h.reuse(leaf.facts[0]).uid != 308 || h.reuse(leaf.defs[0]).uid != 306 || h.reuse(leaf).uid != 310 || h.reuse(m.theorems[0]).uid != 311 || h.count != 312 {
		t.Fatalf("exact=%v reason=%s fact=%#v def=%#v leaf=%#v theorem=%#v count=%d", h.exact, h.firstUnknown, h.reuse(leaf.facts[0]), h.reuse(leaf.defs[0]), h.reuse(leaf), h.reuse(m.theorems[0]), h.count)
	}
}

// Selector-to-node returns the existing recovery object for a numbered
// non-fact alias and creates no semantic node before later proof constructors.
func TestHistoryProofAliasRecoveryContinues(t *testing.T) {
	source := "---- MODULE M ----\nTHEOREM TRUE\n<1>1. USE MODULE M\n<1>2. TRUE BY <1>1\n<1> QED OBVIOUS\nF == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	pf := m.theorems[0].proof.(*nonLeafProofNode)
	step := pf.steps[1].(*theoremNode)
	leaf := step.proof.(*leafProofNode)
	if !h.exact || leaf.facts[0] != g.nullOAN || h.reuse(leaf).uid != h.reuse(step).uid-1 || !h.reuse(testUserDefinitions(t, m.definitions)[0]).known {
		t.Fatalf("exact=%v reason=%q leaf=%#v step=%#v", h.exact, h.firstUnknown, h.reuse(leaf), h.reuse(step))
	}
}

// A DEF selector with arguments reports the source errors and reuses the
// recovery application; the following definition still receives its UID.
func TestHistoryMalformedProofDefContinues(t *testing.T) {
	source := "---- MODULE M ----\nF(x) == x\nTHEOREM TRUE BY DEF F(1)\nG == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	if !h.exact || len(g.table.errors.Messages()) != 4 || h.reuse(m.theorems[0].proof).uid+1 != h.reuse(m.theorems[0]).uid || !h.reuse(testUserDefinitions(t, m.definitions)[1]).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, g.table.errors.Messages())
	}
}

func TestHistoryProofTheoremAPFactSelection(t *testing.T) {
	source := "---- MODULE M ----\nTHEOREM T == ASSUME NEW x PROVE TRUE\nTHEOREM U == ASSUME TRUE PROVE TRUE\nTHEOREM TRUE BY T!1, U!2\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	ap := m.theorems[0].body.(*assumeProveNode)
	leaf := m.theorems[2].proof.(*leafProofNode)
	if !h.exact || leaf.facts[0] != ap.assumptions[0] || h.reuse(leaf.facts[0]) != h.reuse(ap.assumptions[0]) || h.reuse(leaf.facts[1]).class != "OpApplNode" || h.reuse(leaf.facts[1]).uid+2 != h.reuse(leaf).uid {
		t.Fatalf("exact=%v reason=%q facts=%#v/%#v leaf=%#v", h.exact, h.firstUnknown, h.reuse(leaf.facts[0]), h.reuse(leaf.facts[1]), h.reuse(leaf))
	}
}

func TestHistoryProofImportedAPSubstitution(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name + ".tla", Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	for _, tc := range []struct{ name, instance, fact string }{
		{"named", "I == INSTANCE A WITH c <- TRUE", "I!T!1"},
		{"unnamed", "INSTANCE A WITH c <- TRUE", "T!1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rootInput := input("Root", tc.instance+"\nTHEOREM TRUE BY "+tc.fact+" DEF Missing\nAfter == TRUE")
			acquisition, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), rootInput, func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
				if n != "A" {
					t.Fatalf("unexpected dependency %s", n)
				}
				return input("A", "CONSTANT c\nTHEOREM T == ASSUME c PROVE TRUE"), nil
			}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
			if err != nil {
				t.Fatal(err)
			}
			details, _, root, _, err := analyzeFlatExternalRoots(stdcontext.Background(), acquisition, 1000)
			if err != nil {
				t.Fatal(err)
			}
			h := &root.owner.history
			leaf := root.theorems[0].proof.(*leafProofNode)
			app := leaf.facts[0].(*opApplNode)
			wrapper, ok := app.operands[0].(*substInNode)
			if !ok || !h.exact || !h.reuse(wrapper).known || h.reuse(wrapper).uid+1 != h.reuse(app).uid || h.reuse(app).uid+2 != h.reuse(leaf).uid || len(details) != 2 || details[0].Code.Value != 4200 || details[1].Code.Value != 4200 || !h.reuse(root.context.lookup(contextKey{name: root.owner.intern("After")})).known {
				t.Fatalf("exact=%v reason=%q diagnostics=%v wrapper=%#v app=%#v leaf=%#v", h.exact, h.firstUnknown, details, h.reuse(wrapper), h.reuse(app), h.reuse(leaf))
			}
		})
	}
}

func TestHistoryParameterizedProofInstanceFactAndDef(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name + ".tla", Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	rootInput := input("Root", "THEOREM TRUE\n<1>1. DEFINE I(y) == INSTANCE A WITH c <- y\n<1>2. TRUE BY I(1)!F DEF I!F\n<1> QED OBVIOUS\nAfter == TRUE")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), rootInput, func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		return input("A", "CONSTANT c\nF == c"), nil
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, _, root, _, err := analyzeFlatExternalRoots(stdcontext.Background(), a, 1000)
	if err != nil {
		t.Fatal(err)
	}
	h := &root.owner.history
	pf := root.theorems[0].proof.(*nonLeafProofNode)
	leaf := pf.steps[1].(*theoremNode).proof.(*leafProofNode)
	fact := leaf.facts[0].(*opApplNode)
	if !h.exact || len(details) != 0 || len(pf.insts) != 1 || leaf.defs[0] != fact.operator || h.reuse(fact.operands[0]).uid+1 != h.reuse(fact).uid || h.reuse(fact).uid+2 != h.reuse(leaf).uid || !h.reuse(root.context.lookup(contextKey{name: root.owner.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q details=%v fact=%#v leaf=%#v", h.exact, h.firstUnknown, details, h.reuse(fact), h.reuse(leaf))
	}
}

func TestHistoryProofAliasArgumentsRejectBeforeEvaluation(t *testing.T) {
	source := "---- MODULE M ----\nTHEOREM TRUE\n<1>1. USE MODULE M\n<1>2. <1>1(123)\n<1> QED OBVIOUS\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	pf := m.theorems[0].proof.(*nonLeafProofNode)
	step := pf.steps[1].(*theoremNode)
	msgs := g.table.errors.Messages()
	if !g.pool.history.exact || step.body != g.nullOAN || len(msgs) != 1 || msgs[0].Code.Value != 4204 || !g.pool.history.reuse(testUserDefinitions(t, m.definitions)[0]).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", g.pool.history.exact, g.pool.history.firstUnknown, msgs)
	}
}

func TestHistoryProofLabelAndOperandFacts(t *testing.T) {
	source := "---- MODULE M ----\nF == L::TRUE\nG == TRUE /\\ FALSE\nTHEOREM TRUE BY F!L, G!1\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	leaf := m.theorems[0].proof.(*leafProofNode)
	if !h.exact || len(g.table.errors.Messages()) != 0 || h.reuse(leaf.facts[0]).class != "OpApplNode" || h.reuse(leaf.facts[1]).uid != h.reuse(leaf.facts[0]).uid+1 || h.reuse(leaf).uid != h.reuse(leaf.facts[1]).uid+2 {
		t.Fatalf("exact=%v reason=%q facts=%#v/%#v leaf=%#v", h.exact, h.firstUnknown, h.reuse(leaf.facts[0]), h.reuse(leaf.facts[1]), h.reuse(leaf))
	}
}

func TestHistoryLogicalTheoremFactSelection(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nTHEOREM T == TRUE /\\ FALSE\nTHEOREM TRUE BY T!1\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	leaf := m.theorems[1].proof.(*leafProofNode)
	if !h.exact || len(g.table.errors.Messages()) != 0 || h.reuse(leaf.facts[0]).class != "OpApplNode" || h.reuse(leaf.facts[0]).uid+2 != h.reuse(leaf).uid {
		t.Fatalf("exact=%v reason=%q fact=%#v leaf=%#v", h.exact, h.firstUnknown, h.reuse(leaf.facts[0]), h.reuse(leaf))
	}
}

func TestHistoryCompoundProofDefRecoveryAndContinuation(t *testing.T) {
	for _, tc := range []struct {
		name, setup, entry string
		codes              []int
	}{
		{"local", "G == LET F == TRUE IN F", "G!:!F", nil},
		{"label", "G == L:: TRUE", "G!L", []int{4200, 4003}},
		{"theorem-new", "THEOREM T == ASSUME NEW x PROVE TRUE", "T!1", []int{4005, 4200, 4003}},
		{"unresolved", "G == TRUE", "G!Missing", []int{4004, 4200, 4003}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\n" + tc.setup + "\nTHEOREM TRUE BY DEF " + tc.entry + "\nAfter == TRUE\n====\n"
			g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
			if err != nil {
				t.Fatal(err)
			}
			msgs := g.table.errors.Messages()
			if !g.pool.history.exact || len(msgs) != len(tc.codes) || !g.pool.history.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
				t.Fatalf("exact=%v reason=%q diagnostics=%v", g.pool.history.exact, g.pool.history.firstUnknown, msgs)
			}
			for i, code := range tc.codes {
				if msgs[i].Code.Value != code {
					t.Fatalf("diagnostic %d: %v", i, msgs)
				}
			}
		})
	}
}

func TestHistoryQualifiedLabelFactSubstitution(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name + ".tla", Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("Root", "I == INSTANCE A WITH c <- TRUE\nTHEOREM TRUE BY I!F!L"), func(_ stdcontext.Context, _ string) (modulegraph.Input, error) {
		return input("A", "CONSTANT c\nF == L:: c"), nil
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, _, root, _, err := analyzeFlatExternalRoots(stdcontext.Background(), a, 1000)
	if err != nil {
		t.Fatal(err)
	}
	h := &root.owner.history
	leaf := root.theorems[0].proof.(*leafProofNode)
	app := leaf.facts[0].(*opApplNode)
	wrap, ok := app.operands[0].(*substInNode)
	if !ok || !h.exact || len(details) != 0 || h.reuse(wrap).uid+1 != h.reuse(app).uid || h.reuse(app).uid+2 != h.reuse(leaf).uid {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, details)
	}
}

func TestHistoryTheoremLabelFactSelection(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nTHEOREM T == L:: TRUE\nTHEOREM TRUE BY T!L\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	leaf := m.theorems[1].proof.(*leafProofNode)
	if !h.exact || len(g.table.errors.Messages()) != 0 || h.reuse(leaf.facts[0]).class != "OpApplNode" || h.reuse(leaf.facts[0]).uid+2 != h.reuse(leaf).uid {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, g.table.errors.Messages())
	}
}

// Ordinary compound operand selectors reuse both selected operands and create
// one final wrapper before the proof and following definition.
func TestHistoryCompoundOperandFact(t *testing.T) {
	source := "---- MODULE M ----\nF == <<<<TRUE, FALSE>>, TRUE>>\nTHEOREM TRUE BY F!1!2\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	leaf := m.theorems[0].proof.(*leafProofNode)
	selected := m.context.lookup(contextKey{name: g.pool.intern("F")}).(*userOpDefNode).body.(*opApplNode).operands[0].(*opApplNode).operands[1]
	fact := leaf.facts[0].(*opApplNode)
	after := m.context.lookup(contextKey{name: g.pool.intern("After")}).(*userOpDefNode)
	if !h.exact || len(g.table.errors.Messages()) != 0 || fact.operands[0] != selected || h.reuse(fact).uid+2 != h.reuse(leaf).uid || !h.reuse(after).known {
		t.Fatalf("exact=%v reason=%q fact=%#v leaf=%#v", h.exact, h.firstUnknown, h.reuse(fact), h.reuse(leaf))
	}
}

func TestHistoryParameterizedCompoundOperandFact(t *testing.T) {
	source := "---- MODULE M ----\nF(x) == <<<<x, TRUE>>, FALSE>>\nTHEOREM TRUE BY F(42)!1!1\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	leaf := m.theorems[0].proof.(*leafProofNode)
	fact := leaf.facts[0].(*opApplNode)
	lambda := fact.operator.(*userOpDefNode)
	root := m.context.lookup(contextKey{name: g.pool.intern("F")}).(*userOpDefNode)
	selected := root.body.(*opApplNode).operands[0].(*opApplNode).operands[0]
	after := m.context.lookup(contextKey{name: g.pool.intern("After")}).(*userOpDefNode)
	if !h.exact || len(g.table.errors.Messages()) != 0 || lambda.body != selected || lambda.params[0] != root.params[0] || h.reuse(fact.operands[0]).uid+1 != h.reuse(lambda).uid || h.reuse(lambda).uid+1 != h.reuse(fact).uid || !h.reuse(after).known {
		t.Fatalf("exact=%v reason=%q fact=%#v lambda=%#v", h.exact, h.firstUnknown, h.reuse(fact), h.reuse(lambda))
	}
}

func TestHistoryCompoundTheoremAPOperandFact(t *testing.T) {
	source := "---- MODULE M ----\nTHEOREM T == ASSUME TRUE PROVE <<TRUE, FALSE>>\nTHEOREM TRUE BY T!2!1\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	leaf := m.theorems[1].proof.(*leafProofNode)
	fact := leaf.facts[0].(*opApplNode)
	ap := m.theorems[0].body.(*assumeProveNode)
	selected := ap.prove.(*opApplNode).operands[0]
	after := m.context.lookup(contextKey{name: g.pool.intern("After")}).(*userOpDefNode)
	if !h.exact || len(g.table.errors.Messages()) != 0 || fact.operands[0] != selected || h.reuse(fact).uid+2 != h.reuse(leaf).uid || !h.reuse(after).known {
		t.Fatalf("exact=%v reason=%q fact=%#v leaf=%#v", h.exact, h.firstUnknown, h.reuse(fact), h.reuse(leaf))
	}
}

func TestHistoryCompoundLetAndUserOperandFact(t *testing.T) {
	source := "---- MODULE M ----\nG(x) == x\nF == LET H == TRUE IN G(<<TRUE, FALSE>>)\nTHEOREM TRUE BY F!1!1!2\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	leaf := m.theorems[0].proof.(*leafProofNode)
	fact := leaf.facts[0].(*opApplNode)
	root := m.context.lookup(contextKey{name: g.pool.intern("F")}).(*userOpDefNode)
	selected := root.body.(*letInNode).body.(*opApplNode).operands[0].(*opApplNode).operands[1]
	after := m.context.lookup(contextKey{name: g.pool.intern("After")}).(*userOpDefNode)
	if !h.exact || len(g.table.errors.Messages()) != 0 || fact.operands[0] != selected || h.reuse(fact).uid+2 != h.reuse(leaf).uid || !h.reuse(after).known {
		t.Fatalf("exact=%v reason=%q fact=%#v leaf=%#v", h.exact, h.firstUnknown, h.reuse(fact), h.reuse(leaf))
	}
}

func TestHistoryNestedParameterizedLabelSelectors(t *testing.T) {
	source := "---- MODULE M ----\nF(a,b) == L(a):: M(b):: <<a,b>>\nR == F(1,2)!L(3)!M(4)\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	f := m.context.lookup(contextKey{name: g.pool.intern("F")}).(*userOpDefNode)
	outer := f.body.(*labelNode)
	inner := outer.body.(*labelNode)
	app := m.context.lookup(contextKey{name: g.pool.intern("R")}).(*userOpDefNode).body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	if !h.exact || lambda.body != inner || len(lambda.params) != 4 || lambda.params[0] != lambda.params[2] || lambda.params[1] != lambda.params[3] || len(app.operands) != 4 || h.reuse(app.operands[3]).uid+1 != h.reuse(lambda).uid || h.reuse(lambda).uid+1 != h.reuse(app).uid || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q params=%d diagnostics=%v", h.exact, h.firstUnknown, len(lambda.params), g.table.errors.Messages())
	}
}

func TestHistoryParameterizedLetFactClones(t *testing.T) {
	source := "---- MODULE M ----\nF(x) == LET G(y) == <<x,y>> IN G(x)\nTHEOREM TRUE BY F(1)!:!G(2)\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	fact := m.theorems[0].proof.(*leafProofNode).facts[0].(*opApplNode)
	lambda := fact.operator.(*userOpDefNode)
	body := lambda.body.(*opApplNode)
	clone := lambda.params[1]
	operand := body.operands[0].(*opApplNode)
	if !h.exact || operand.operator != clone || h.reuse(clone).uid+1 != h.reuse(operand).uid || h.reuse(operand).uid+1 != h.reuse(body).uid || h.reuse(body).uid+1 != h.reuse(lambda).uid || h.reuse(lambda).uid+1 != h.reuse(fact).uid || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, g.table.errors.Messages())
	}
}

func TestHistoryQualifiedParameterizedLabelAndTheoremFacts(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name + ".tla", Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	rootInput := input("Root", "I(p) == INSTANCE A WITH c <- p\nR == I(1)!F(2)!L(3)\nTHEOREM TRUE BY I(4)!T!2!1\nAfter == TRUE")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), rootInput, func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		return input("A", "CONSTANT c\nF(y) == L(y):: <<c,y>>\nTHEOREM T == ASSUME c PROVE <<TRUE,FALSE>>"), nil
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, _, root, _, err := analyzeFlatExternalRoots(stdcontext.Background(), a, 1000)
	if err != nil {
		t.Fatal(err)
	}
	h := &root.owner.history
	r := root.context.lookup(contextKey{name: root.owner.intern("R")}).(*userOpDefNode).body.(*opApplNode)
	lambda := r.operator.(*userOpDefNode)
	wrapper := lambda.body.(*substInNode)
	fact := root.theorems[0].proof.(*leafProofNode).facts[0].(*opApplNode)
	thLambda := fact.operator.(*userOpDefNode)
	thWrapper := thLambda.body.(*substInNode)
	if !h.exact || len(details) != 1 || details[0].Code.Value != 4332 || len(r.operands) != 3 || len(lambda.params) != 3 || len(fact.operands) != 1 || len(thLambda.params) != 1 || !h.reuse(wrapper).known || !h.reuse(thWrapper).known || h.reuse(thWrapper).uid+1 != h.reuse(thLambda).uid || h.reuse(thLambda).uid+1 != h.reuse(fact).uid || !h.reuse(root.context.lookup(contextKey{name: root.owner.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, details)
	}
}

func TestHistoryQuantifiedOperandFactArguments(t *testing.T) {
	source := "---- MODULE M ----\nF == \\A x \\in {1}: <<x, TRUE>>\nTHEOREM TRUE BY F!(2)!1 DEF Missing\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	root := m.context.lookup(contextKey{name: g.pool.intern("F")}).(*userOpDefNode)
	quant := root.body.(*opApplNode)
	fact := m.theorems[0].proof.(*leafProofNode).facts[0].(*opApplNode)
	lambda := fact.operator.(*userOpDefNode)
	selected := quant.operands[0].(*opApplNode).operands[0]
	msgs := g.table.errors.Messages()
	if !h.exact || lambda.body != selected || lambda.params[0] != quant.bounded[0][0] || h.reuse(fact.operands[0]).uid+1 != h.reuse(lambda).uid || h.reuse(lambda).uid+1 != h.reuse(fact).uid || len(msgs) != 2 || msgs[0].Code.Value != 4200 || msgs[1].Code.Value != 4200 || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, msgs)
	}
}

func TestHistoryQuantifiedLetFactClonesAndContinuation(t *testing.T) {
	source := "---- MODULE M ----\nF == \\A x \\in {1}: \\A y \\in {2}: LET G(z) == <<x,y,z>> IN G(1)\nTHEOREM TRUE BY F!(4)!(5)!G(6) DEF Missing\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	root := m.context.lookup(contextKey{name: g.pool.intern("F")}).(*userOpDefNode)
	first := root.body.(*opApplNode)
	second := first.operands[0].(*opApplNode)
	fact := m.theorems[0].proof.(*leafProofNode).facts[0].(*opApplNode)
	lambda := fact.operator.(*userOpDefNode)
	body := lambda.body.(*opApplNode)
	clone := lambda.params[2]
	operand := body.operands[0].(*opApplNode)
	msgs := g.table.errors.Messages()
	if !h.exact || len(lambda.params) != 3 || lambda.params[0] != first.bounded[0][0] || lambda.params[1] != second.bounded[0][0] || operand.operator != clone || h.reuse(clone).uid+1 != h.reuse(operand).uid || h.reuse(operand).uid+1 != h.reuse(body).uid || h.reuse(body).uid+1 != h.reuse(lambda).uid || h.reuse(lambda).uid+1 != h.reuse(fact).uid || len(msgs) != 2 || msgs[0].Code.Value != 4200 || msgs[1].Code.Value != 4200 || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, msgs)
	}
}

func TestHistoryOperatorArgumentQuantifiedLetClones(t *testing.T) {
	source := "---- MODULE M ----\nF == \\A y \\in {1}: LET G(x) == <<y,x>> IN G(1)\nApply(Op(_,_)) == Op(1,2)\nA == Apply(F!@!G)\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	root := m.context.lookup(contextKey{name: g.pool.intern("F")}).(*userOpDefNode)
	quant := root.body.(*opApplNode)
	app := m.context.lookup(contextKey{name: g.pool.intern("A")}).(*userOpDefNode).body.(*opApplNode)
	arg := app.operands[0].(*opArgNode)
	lambda := arg.operator.(*userOpDefNode)
	selected := lambda.body.(*opApplNode)
	clone := lambda.params[1]
	operand := selected.operands[0].(*opApplNode)
	if !h.exact || len(g.table.errors.Messages()) != 0 || lambda.params[0] != quant.bounded[0][0] || operand.operator != clone || h.reuse(clone).uid+1 != h.reuse(operand).uid || h.reuse(operand).uid+1 != h.reuse(selected).uid || h.reuse(selected).uid+1 != h.reuse(lambda).uid || h.reuse(lambda).uid+1 != h.reuse(arg).uid || h.reuse(arg).uid+1 != h.reuse(app).uid || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, g.table.errors.Messages())
	}
}

func TestHistorySelectedOperatorArgumentSyntheticFormals(t *testing.T) {
	source := "---- MODULE M ----\nApply(Op(_)) == Op(1)\nH(y) == y\nF(x) == Apply(H)\nUse(Op(_,_)) == Op(1,2)\nA == Use(F!1)\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	f := m.context.lookup(contextKey{name: g.pool.intern("F")}).(*userOpDefNode)
	app := m.context.lookup(contextKey{name: g.pool.intern("A")}).(*userOpDefNode).body.(*opApplNode)
	arg := app.operands[0].(*opArgNode)
	lambda := arg.operator.(*userOpDefNode)
	body := lambda.body.(*opApplNode)
	synthetic := lambda.params[1]
	msgs := g.table.errors.Messages()
	if !h.exact || len(msgs) != 1 || msgs[0].Code.Value != 4004 || lambda.params[0] != f.params[0] || h.reuse(synthetic).uid+1 != h.reuse(body).uid || h.reuse(body).uid+1 != h.reuse(lambda).uid || h.reuse(lambda).uid+1 != h.reuse(arg).uid || h.reuse(arg).uid+1 != h.reuse(app).uid || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, msgs)
	}
}

func TestHistoryMalformedMultiPrefixDefRecovery(t *testing.T) {
	for _, entry := range []string{"F!L!M(1)", "F!L!M!N(1)"} {
		t.Run(entry, func(t *testing.T) {
			source := "---- MODULE M ----\nF == TRUE\nTHEOREM TRUE BY DEF " + entry + "\nAfter == TRUE\n====\n"
			g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
			if err != nil {
				t.Fatal(err)
			}
			msgs := g.table.errors.Messages()
			want := []int{4003, 4003, 4005, 4200, 4003}
			if !g.pool.history.exact || len(msgs) != len(want) {
				t.Fatalf("exact=%v reason=%q diagnostics=%v", g.pool.history.exact, g.pool.history.firstUnknown, msgs)
			}
			for i, code := range want {
				if msgs[i].Code.Value != code {
					t.Fatalf("diagnostics %v", msgs)
				}
			}
			if !g.pool.history.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
				t.Fatal("later definition lost")
			}
		})
	}
}

func TestHistoryLabelOperatorArgument(t *testing.T) {
	source := "---- MODULE M ----\nF(a) == L(a)::TRUE\nUse(Op(_,_)) == Op(1,2)\nA == Use(F!L)\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	f := m.context.lookup(contextKey{name: g.pool.intern("F")}).(*userOpDefNode)
	label := f.body.(*labelNode)
	app := m.context.lookup(contextKey{name: g.pool.intern("A")}).(*userOpDefNode).body.(*opApplNode)
	arg := app.operands[0].(*opArgNode)
	lambda := arg.operator.(*userOpDefNode)
	if !h.exact || lambda.body != label || len(lambda.params) != 2 || lambda.params[0] != f.params[0] || lambda.params[1] != label.params[0] || h.reuse(lambda).uid+1 != h.reuse(arg).uid || h.reuse(arg).uid+1 != h.reuse(app).uid || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, g.table.errors.Messages())
	}
}

func TestHistoryTheoremNumericExpressionContinuation(t *testing.T) {
	source := "---- MODULE M ----\nTHEOREM T == ASSUME TRUE PROVE <<TRUE,FALSE>>\nR == T!2!1\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	ap := m.theorems[0].body.(*assumeProveNode)
	selected := ap.prove.(*opApplNode).operands[0]
	app := m.context.lookup(contextKey{name: g.pool.intern("R")}).(*userOpDefNode).body.(*opApplNode)
	if !h.exact || app.operands[0] != selected || !h.reuse(app).known || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, g.table.errors.Messages())
	}
}

func TestHistoryTheoremQuantifiedLetExpression(t *testing.T) {
	source := "---- MODULE M ----\nTHEOREM T == ASSUME TRUE PROVE \\A y \\in {1}: LET G(x) == <<y,x>> IN G(1)\nR == T!2!(2)!G(3)\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	ap := m.theorems[0].body.(*assumeProveNode)
	quant := ap.prove.(*opApplNode)
	app := m.context.lookup(contextKey{name: g.pool.intern("R")}).(*userOpDefNode).body.(*opApplNode)
	lambda := app.operator.(*userOpDefNode)
	selected := lambda.body.(*opApplNode)
	clone := lambda.params[1]
	operand := selected.operands[0].(*opApplNode)
	if !h.exact || len(g.table.errors.Messages()) != 0 || lambda.params[0] != quant.bounded[0][0] || operand.operator != clone || h.reuse(clone).uid+1 != h.reuse(operand).uid || h.reuse(operand).uid+1 != h.reuse(selected).uid || h.reuse(selected).uid+1 != h.reuse(lambda).uid || h.reuse(lambda).uid+1 != h.reuse(app).uid || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, g.table.errors.Messages())
	}
}

func TestHistoryLabelArgumentRecoveryBeforeEvaluation(t *testing.T) {
	source := "---- MODULE M ----\nF(x) == L(x)::TRUE\nR == F(1)!L(Missing, Other)\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	r := m.context.lookup(contextKey{name: g.pool.intern("R")}).(*userOpDefNode)
	if !g.pool.history.exact || len(msgs) != 2 || msgs[0].Code.Value != 4332 || msgs[1].Code.Value != 4337 || r.body != g.nullOAN || !g.pool.history.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", g.pool.history.exact, g.pool.history.firstUnknown, msgs)
	}
}

func TestHistoryTheoremAPLabelLetSelector(t *testing.T) {
	source := "---- MODULE M ----\nTHEOREM T == ASSUME L:: ASSUME LET G(x) == x IN G(1) PROVE TRUE PROVE TRUE\nA == T!L!1!G(2)\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	app := m.context.lookup(contextKey{name: g.pool.intern("A")}).(*userOpDefNode).body.(*opApplNode)
	if !h.exact || len(g.table.errors.Messages()) != 0 || !h.reuse(app).known || !h.reuse(app.operands[0]).known || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, g.table.errors.Messages())
	}
}

func TestHistoryLambdaOperandSelectorAndRecovery(t *testing.T) {
	source := "---- MODULE M ----\nApply(Op(_)) == Op(1)\nF == Apply(LAMBDA x: <<x, TRUE>>)\nA == F!1!(2)!1\nB == F!1!(3,4)\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	f := m.context.lookup(contextKey{name: g.pool.intern("F")}).(*userOpDefNode)
	original := f.body.(*opApplNode).operands[0].(*opArgNode).operator.(*userOpDefNode)
	app := m.context.lookup(contextKey{name: g.pool.intern("A")}).(*userOpDefNode).body.(*opApplNode)
	wrapper := app.operator.(*userOpDefNode)
	b := m.context.lookup(contextKey{name: g.pool.intern("B")}).(*userOpDefNode)
	msgs := g.table.errors.Messages()
	if !h.exact || wrapper.params[0] != original.params[0] || wrapper.body != original.body.(*opApplNode).operands[0] || h.reuse(app.operands[0]).uid+1 != h.reuse(wrapper).uid || h.reuse(wrapper).uid+1 != h.reuse(app).uid || b.body != g.nullOAN || len(msgs) != 1 || msgs[0].Code.Value != 4005 || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, msgs)
	}
}

func TestHistoryOperatorArgumentRecoveryAndContinuation(t *testing.T) {
	source := "---- MODULE M ----\nApply(Op(_)) == Op(1)\nH(x,y) == x\nA == Apply(H)\nB == Apply(<<1,2>>)\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	a := m.context.lookup(contextKey{name: g.pool.intern("A")}).(*userOpDefNode).body.(*opApplNode)
	b := m.context.lookup(contextKey{name: g.pool.intern("B")}).(*userOpDefNode).body.(*opApplNode)
	msgs := g.table.errors.Messages()
	if !h.exact || a.operands[0] != g.nullOAN || b.operands[0] != g.nullOAN || len(msgs) != 4 || msgs[0].Code.Value != 4271 || msgs[1].Code.Value != 4271 || msgs[2].Code.Value != 4270 || msgs[3].Code.Value != 4271 || h.reuse(a).uid+2 != h.reuse(b).uid || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, msgs)
	}
}

func TestHistoryCaseRecordAndColonSelectors(t *testing.T) {
	source := "---- MODULE M ----\nR == [a |-> 1, b |-> 2]\nC == CASE TRUE -> 1 [] OTHER -> 2\nF(x) == x\nUse(Op(_)) == Op(1)\nA == R!2\nB == C!2!2\nBad == C!1\nX == F(3)!:\nY == Use(F!:)\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	a := m.context.lookup(contextKey{name: g.pool.intern("A")}).(*userOpDefNode).body.(*opApplNode)
	b := m.context.lookup(contextKey{name: g.pool.intern("B")}).(*userOpDefNode).body.(*opApplNode)
	bad := m.context.lookup(contextKey{name: g.pool.intern("Bad")}).(*userOpDefNode)
	x := m.context.lookup(contextKey{name: g.pool.intern("X")}).(*userOpDefNode).body.(*opApplNode)
	y := m.context.lookup(contextKey{name: g.pool.intern("Y")}).(*userOpDefNode).body.(*opApplNode)
	arg := y.operands[0].(*opArgNode)
	f := m.context.lookup(contextKey{name: g.pool.intern("F")}).(*userOpDefNode)
	msgs := g.table.errors.Messages()
	if !h.exact || len(msgs) != 1 || msgs[0].Code.Value != 4005 || bad.body != g.nullOAN || !h.reuse(a).known || !h.reuse(b).known || x.operator.(*userOpDefNode).body != f.body || arg.operator.(*userOpDefNode).body != f.body || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, msgs)
	}
}

func TestHistoryQualifiedApplicationAndArityRecovery(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name + ".tla", Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	rootInput := input("Root", "I(p) == INSTANCE A WITH c <- p\nR == I(1)!F(2)\nS == I(3)!F\nAfter == TRUE")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), rootInput, func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		return input("A", "CONSTANT c\nF(y) == <<c,y>>"), nil
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	details, _, root, _, err := analyzeFlatExternalRoots(stdcontext.Background(), a, 1000)
	if err != nil {
		t.Fatal(err)
	}
	h := &root.owner.history
	r := root.context.lookup(contextKey{name: root.owner.intern("R")}).(*userOpDefNode).body.(*opApplNode)
	s := root.context.lookup(contextKey{name: root.owner.intern("S")}).(*userOpDefNode)
	if !h.exact || len(details) != 1 || details[0].Code.Value != 4204 || len(r.operands) != 2 || h.reuse(r.operands[0]).uid+1 != h.reuse(r.operands[1]).uid || h.reuse(r.operands[1]).uid+1 != h.reuse(r).uid || !h.reuse(s).known || !h.reuse(root.context.lookup(contextKey{name: root.owner.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, details)
	}
}

func TestHistoryNumericRecoveryContinues(t *testing.T) {
	source := "---- MODULE M ----\nF == <<TRUE,FALSE>>\nTHEOREM TRUE BY F!9, F!1!2\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	leaf := m.theorems[0].proof.(*leafProofNode)
	msgs := g.table.errors.Messages()
	if !h.exact || len(msgs) != 2 || msgs[0].Code.Value != 4005 || msgs[1].Code.Value != 4005 || leaf.facts[0] != g.nullOAN || leaf.facts[1] != g.nullOAN || !h.reuse(leaf).known || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, msgs)
	}
}

func TestHistoryMissingLabelAndUnknownSelectorRecovery(t *testing.T) {
	source := "---- MODULE M ----\nF(x) == x\nA == F(1)!Missing\nB == Unknown(2)!Other\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	a := m.context.lookup(contextKey{name: g.pool.intern("A")}).(*userOpDefNode)
	b := m.context.lookup(contextKey{name: g.pool.intern("B")}).(*userOpDefNode)
	msgs := g.table.errors.Messages()
	if !h.exact || a.body != g.nullOAN || b.body != g.nullOAN || len(msgs) != 2 || msgs[0].Code.Value != 4004 || msgs[1].Code.Value != 4200 || h.reuse(a).uid+1 != h.reuse(b).uid || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, msgs)
	}
}

func TestHistorySufficesAPSelectionAndRecovery(t *testing.T) {
	source := "---- MODULE M ----\nTHEOREM TRUE\n<1>1. SUFFICES ASSUME TRUE PROVE TRUE\n<1>2. TRUE BY <1>1!1!2, <1>1!@\n<1> QED OBVIOUS\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	proof := m.theorems[0].proof.(*nonLeafProofNode)
	leaf := proof.steps[1].(*theoremNode).proof.(*leafProofNode)
	msgs := g.table.errors.Messages()
	if !h.exact || len(msgs) != 1 || msgs[0].Code.Value != 4005 || leaf.facts[1] != g.nullOAN || !h.reuse(leaf.facts[0]).known || !h.reuse(leaf).known || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, msgs)
	}
}

func TestHistoryDirectArityAndMissingRecovery(t *testing.T) {
	source := "---- MODULE M ----\nF(x) == x\nA == F(Missing, Other)\nB == Absent\nC == F\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	msgs := g.table.errors.Messages()
	a := m.context.lookup(contextKey{name: g.pool.intern("A")}).(*userOpDefNode)
	b := m.context.lookup(contextKey{name: g.pool.intern("B")}).(*userOpDefNode)
	c := m.context.lookup(contextKey{name: g.pool.intern("C")}).(*userOpDefNode)
	if !h.exact || len(msgs) != 3 || msgs[0].Code.Value != 4204 || msgs[1].Code.Value != 4200 || msgs[2].Code.Value != 4204 || a.body != g.nullOAN || b.body != g.nullOAN || c.body != g.nullOAN || h.reuse(a).uid+1 != h.reuse(b).uid || h.reuse(b).uid+1 != h.reuse(c).uid || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, msgs)
	}
}

// selectorUnknownSource is shared with the tagged typed-bridge companion test.
const selectorUnknownSource = "---- MODULE M ----\nG == LET X == TRUE IN X\nA == G!:!Missing\nB == Unknown!Other\nAfter == TRUE\n====\n"

func TestSelectorUnknownDiagnosticsUseUniqueString(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, selectorUnknownSource), 0)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if !g.pool.history.exact || len(msgs) != 2 || msgs[0].Code.Value != 4200 || msgs[1].Code.Value != 4200 {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", g.pool.history.exact, g.pool.history.firstUnknown, msgs)
	}
	for i, want := range []string{"Missing", "Unknown!Other"} {
		if len(msgs[i].Parameters) != 1 {
			t.Fatal(msgs[i])
		}
		p := msgs[i].Parameters[0]
		if p.Kind != ObjectIdentityParameter || p.JavaClass != "util.UniqueString" || p.Text != want || !p.UTF16Present || p.Identity == 0 {
			t.Fatalf("parameter %d: %#v", i, p)
		}
	}
	if !g.pool.history.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatal("lost continuation")
	}
}

func TestHistoryMissingOperatorSelectorRecovery(t *testing.T) {
	source := "---- MODULE M ----\nF(x) == x\nUse(Op(_)) == Op(1)\nA == Use(F!Missing)\nB == Use(F(999)!Missing)\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	a := m.context.lookup(contextKey{name: g.pool.intern("A")}).(*userOpDefNode).body.(*opApplNode)
	b := m.context.lookup(contextKey{name: g.pool.intern("B")}).(*userOpDefNode).body.(*opApplNode)
	msgs := g.table.errors.Messages()
	if !h.exact || len(msgs) != 4 || msgs[0].Code.Value != 4004 || msgs[1].Code.Value != 4271 || msgs[2].Code.Value != 4005 || msgs[3].Code.Value != 4271 || a.operands[0] != g.nullOAN || b.operands[0] != g.nullOAN || h.reuse(a).uid+2 != h.reuse(b).uid || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, msgs)
	}
}

func TestHistoryQualifiedMultiInstanceProofSelectors(t *testing.T) {
	source := "---- MODULE Root ----\n---- MODULE A ----\nCONSTANT X\nF(z) == L:: <<X,z>>\n====\n---- MODULE B ----\nCONSTANT Y\nI(p) == INSTANCE A WITH X <- Y\n====\nJ(q) == INSTANCE B WITH Y <- q\nTHEOREM TRUE BY J(1)!I(2)!F(3)!L!1 DEF J!I!F\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	leaf := m.theorems[0].proof.(*leafProofNode)
	app := leaf.facts[0].(*opApplNode)
	selected := leaf.defs[0].(*userOpDefNode)
	originalOuter := selected.body.(*substInNode)
	originalInner := originalOuter.body.(*substInNode)
	lambda := app.operator.(*userOpDefNode)
	outer := lambda.body.(*substInNode)
	inner := outer.body.(*substInNode)
	if !h.exact || len(g.table.errors.Messages()) != 0 || len(app.operands) != 3 || selected != m.context.lookup(contextKey{name: g.pool.intern("J!I!F")}) || outer == originalOuter || inner == originalInner || outer.substs[0] != originalOuter.substs[0] || inner.substs[0] != originalInner.substs[0] || h.reuse(inner).uid+1 != h.reuse(outer).uid || h.reuse(outer).uid+1 != h.reuse(lambda).uid || h.reuse(lambda).uid+1 != h.reuse(app).uid || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, g.table.errors.Messages())
	}
}

func TestHistoryLabelSpecialOperandLetSelection(t *testing.T) {
	for _, body := range []string{
		"[a |-> LET G(x) == x IN G(1)]",
		"[a : LET G(x) == x IN {1}]",
		"[(LET G(x) == x IN [a |-> 1]) EXCEPT !.a = 2]",
	} {
		t.Run(body, func(t *testing.T) {
			source := "---- MODULE M ----\nF == mark:: " + body + "\nApply(Op(_)) == Op(1)\nA == F!mark!1!G(2)\nB == Apply(F!mark!1!G)\nAfter == TRUE\n====\n"
			g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
			if err != nil {
				t.Fatal(err)
			}
			h := &g.pool.history
			if !h.exact || len(g.table.errors.Messages()) != 0 {
				t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, g.table.errors.Messages())
			}
			a := m.context.lookup(contextKey{name: g.pool.intern("A")}).(*userOpDefNode)
			b := m.context.lookup(contextKey{name: g.pool.intern("B")}).(*userOpDefNode)
			app := a.body.(*opApplNode)
			arg := b.body.(*opApplNode).operands[0].(*opArgNode)
			if len(app.operands) != 1 || arg.arity != 1 || !h.reuse(app).known || !h.reuse(arg).known || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
				t.Fatal("selection or constructor history")
			}
		})
	}
}

func TestHistoryLabelExceptUnsupportedOperandRemainsIncomplete(t *testing.T) {
	source := "---- MODULE M ----\nF == mark:: [[a |-> 1] EXCEPT !.a = 2]\nA == F!mark!2\nAfter == TRUE\n====\n"
	g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if g.pool.history.exact || len(msgs) != 1 || msgs[0].Code.Value != 4005 || msgs[0].Format != "Selecting subexpression of an EXCEPT not yet implemented." {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", g.pool.history.exact, g.pool.history.firstUnknown, msgs)
	}
}

func TestHistoryLabelCaseOperandSelection(t *testing.T) {
	for _, selector := range []string{"!1!2", "!2!2"} {
		t.Run(selector, func(t *testing.T) {
			source := "---- MODULE M ----\nF == mark:: CASE TRUE -> LET G(x) == x IN G(1) [] OTHER -> LET G(x) == x IN G(2)\nApply(Op(_)) == Op(1)\nA == F!mark" + selector + "!G(2)\nB == Apply(F!mark" + selector + "!G)\nAfter == TRUE\n====\n"
			g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
			if err != nil {
				t.Fatal(err)
			}
			h := &g.pool.history
			if !h.exact || len(g.table.errors.Messages()) != 0 {
				t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, g.table.errors.Messages())
			}
			a := m.context.lookup(contextKey{name: g.pool.intern("A")}).(*userOpDefNode)
			b := m.context.lookup(contextKey{name: g.pool.intern("B")}).(*userOpDefNode)
			app := a.body.(*opApplNode)
			arg := b.body.(*opApplNode).operands[0].(*opArgNode)
			if len(app.operands) != 1 || arg.arity != 1 || !h.reuse(app).known || !h.reuse(arg).known || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
				t.Fatal("selection history")
			}
		})
	}
}

func TestHistoryLabelCaseRecovery(t *testing.T) {
	for _, tc := range []struct{ selector, message string }{
		{"!1", "Subexpression of CASE must have form !i!j."},
		{"!1!3", "Second selector for CASE subexpression must specify  one of two operands."},
		{"!2!1", "Selecting OTHER in a CASE statement."},
	} {
		t.Run(tc.selector, func(t *testing.T) {
			source := "---- MODULE M ----\nF == mark:: CASE TRUE -> 1 [] OTHER -> 2\nA == F!mark" + tc.selector + "\nAfter == TRUE\n====\n"
			g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
			if err != nil {
				t.Fatal(err)
			}
			msgs := g.table.errors.Messages()
			if !g.pool.history.exact || len(msgs) != 1 || msgs[0].Code.Value != 4005 || msgs[0].Format != tc.message || !g.pool.history.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
				t.Fatalf("exact=%v reason=%q diagnostics=%v", g.pool.history.exact, g.pool.history.firstUnknown, msgs)
			}
		})
	}
}

func TestHistoryTheoremCaseLabelLetSelector(t *testing.T) {
	source := "---- MODULE M ----\nTHEOREM T == ASSUME L:: ASSUME CASE TRUE -> LET G(x) == x IN G(1) [] OTHER -> 2 PROVE TRUE PROVE TRUE\nA == T!L!1!1!2!G(2)\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	if !h.exact || len(g.table.errors.Messages()) != 0 || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, g.table.errors.Messages())
	}
}

func TestHistoryLabelCaseTerminalAndOperatorRecovery(t *testing.T) {
	source := "---- MODULE M ----\nF == mark:: CASE TRUE -> 1 [] OTHER -> 2\nApply(Op(_)) == Op(1)\nA == F!mark!1!2\nB == Apply(F!mark!2!1)\nAfter == TRUE\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	msgs := g.table.errors.Messages()
	if !h.exact || len(msgs) != 2 || msgs[0].Code.Value != 4005 || msgs[0].Format != "Selecting OTHER in a CASE statement." || msgs[1].Code.Value != 4271 || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, msgs)
	}
}
