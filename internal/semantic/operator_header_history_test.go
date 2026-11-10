// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"fmt"
	"testing"

	"github.com/jwriteclub/prudence/internal/modulegraph"
)

// headerEvent names one expected source-ordered SemanticNode allocation.
type headerEvent struct {
	class string
	kind  int
	node  func(m *moduleNode) any
}

func headerDef(i int) func(*moduleNode) *userOpDefNode {
	return func(m *moduleNode) *userOpDefNode { return m.definitions[i].(*userOpDefNode) }
}
func headerDefEvent(i int) headerEvent {
	return headerEvent{"OpDefNode", 5, func(m *moduleNode) any { return headerDef(i)(m) }}
}
func headerFormalEvent(i, j int) headerEvent {
	return headerEvent{"FormalParamNode", 11, func(m *moduleNode) any { return headerDef(i)(m).params[j] }}
}
func headerApplEvent(path func(m *moduleNode) exprNode) headerEvent {
	return headerEvent{"OpApplNode", 9, func(m *moduleNode) any { return path(m) }}
}
func headerBody(i int) func(*moduleNode) exprNode {
	return func(m *moduleNode) exprNode { return headerDef(i)(m).body }
}
func headerOperand(path func(*moduleNode) exprNode, j int) func(*moduleNode) exprNode {
	return func(m *moduleNode) exprNode { return path(m).(*opApplNode).operands[j].(exprNode) }
}

type headerParam struct {
	text  string
	arity int
}

// The schedules below follow Generator.java:2494-2704 (processOperator):
// all FormalParamNodes in header order, then the body, then one OpDefNode.
// Expression schedules are the already-admitted producers.
var headerHistoryCases = []struct {
	label, body string
	events      []headerEvent
	defs        []string // canonical definition names
	local       []bool
	params      [][]headerParam
}{
	{
		label: "infix-lhs-application",
		body:  "a ++ b == a\nG == TRUE ++ FALSE",
		events: []headerEvent{
			headerFormalEvent(0, 0), headerFormalEvent(0, 1), headerApplEvent(headerBody(0)), headerDefEvent(0),
			headerApplEvent(headerOperand(headerBody(1), 0)), headerApplEvent(headerOperand(headerBody(1), 1)), headerApplEvent(headerBody(1)), headerDefEvent(1),
		},
		defs: []string{"++", "G"}, local: []bool{false, false},
		params: [][]headerParam{{{"a", 0}, {"b", 0}}, {}},
	},
	{
		label: "prefix-lhs-application",
		body:  "-. a == a\nG == -TRUE",
		events: []headerEvent{
			headerFormalEvent(0, 0), headerApplEvent(headerBody(0)), headerDefEvent(0),
			headerApplEvent(headerOperand(headerBody(1), 0)), headerApplEvent(headerBody(1)), headerDefEvent(1),
		},
		defs: []string{"-.", "G"}, local: []bool{false, false},
		params: [][]headerParam{{{"a", 0}}, {}},
	},
	{
		label: "postfix-lhs-application",
		body:  "a ^+ == a\nG == TRUE^+",
		events: []headerEvent{
			headerFormalEvent(0, 0), headerApplEvent(headerBody(0)), headerDefEvent(0),
			headerApplEvent(headerOperand(headerBody(1), 0)), headerApplEvent(headerBody(1)), headerDefEvent(1),
		},
		defs: []string{"^+", "G"}, local: []bool{false, false},
		params: [][]headerParam{{{"a", 0}}, {}},
	},
	{
		// Operators.resolveSynonym canonicalizes the defined name; the
		// canonical identifier of (+) is \oplus.
		label: "canonical-synonym-lhs",
		body:  "a (+) b == b\nG == TRUE \\oplus FALSE",
		events: []headerEvent{
			headerFormalEvent(0, 0), headerFormalEvent(0, 1), headerApplEvent(headerBody(0)), headerDefEvent(0),
			headerApplEvent(headerOperand(headerBody(1), 0)), headerApplEvent(headerOperand(headerBody(1), 1)), headerApplEvent(headerBody(1)), headerDefEvent(1),
		},
		defs: []string{"\\oplus", "G"}, local: []bool{false, false},
		params: [][]headerParam{{{"a", 0}, {"b", 0}}, {}},
	},
	{
		label: "local-ident-and-infix",
		body:  "LOCAL F(a) == a\nLOCAL a ** b == b\nG == F(TRUE)",
		events: []headerEvent{
			headerFormalEvent(0, 0), headerApplEvent(headerBody(0)), headerDefEvent(0),
			headerFormalEvent(1, 0), headerFormalEvent(1, 1), headerApplEvent(headerBody(1)), headerDefEvent(1),
			headerApplEvent(headerOperand(headerBody(2), 0)), headerApplEvent(headerBody(2)), headerDefEvent(2),
		},
		defs: []string{"F", "**", "G"}, local: []bool{true, true, false},
		params: [][]headerParam{{{"a", 0}}, {{"a", 0}, {"b", 0}}, {}},
	},
	{
		// IdentLHS formals: PrefixDecl, PostfixDecl, InfixDecl (canonical), and
		// a higher-order IdentDecl, each one FormalParamNode in header order.
		label: "symbolic-and-higher-order-formals",
		body:  "F(-. _, _ ^+, _ + _, _ (+) _, H(_, _), x) == x",
		events: []headerEvent{
			headerFormalEvent(0, 0), headerFormalEvent(0, 1), headerFormalEvent(0, 2), headerFormalEvent(0, 3), headerFormalEvent(0, 4), headerFormalEvent(0, 5),
			headerApplEvent(headerBody(0)), headerDefEvent(0),
		},
		defs: []string{"F"}, local: []bool{false},
		params: [][]headerParam{{{"-.", 1}, {"^+", 1}, {"+", 2}, {"\\oplus", 2}, {"H", 2}, {"x", 0}}},
	},
	{
		// Applications of symbolic formals reach the existing fixity producer.
		label: "symbolic-formal-applications",
		body:  "F(-. _, _ + _, x) == (-x) + x",
		events: []headerEvent{
			headerFormalEvent(0, 0), headerFormalEvent(0, 1), headerFormalEvent(0, 2),
			headerApplEvent(headerOperand(headerOperand(headerBody(0), 0), 0)), headerApplEvent(headerOperand(headerBody(0), 0)),
			headerApplEvent(headerOperand(headerBody(0), 1)), headerApplEvent(headerBody(0)), headerDefEvent(0),
		},
		defs: []string{"F"}, local: []bool{false},
		params: [][]headerParam{{{"-.", 1}, {"+", 2}, {"x", 0}}},
	},
	{
		// SymbolTable.addSymbol rejects the second binding after its
		// FormalParamNode constructor has already taken a UID.
		label: "duplicate-formals",
		body:  "F(a, a) == a",
		events: []headerEvent{
			headerFormalEvent(0, 0), headerFormalEvent(0, 1), headerApplEvent(headerBody(0)), headerDefEvent(0),
		},
		defs: []string{"F"}, local: []bool{false},
		params: [][]headerParam{{{"a", 0}, {"a", 0}}},
	},
}

func TestHistoryOperatorHeaderSchedules(t *testing.T) {
	for _, c := range headerHistoryCases {
		t.Run(c.label, func(t *testing.T) {
			g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\n"+c.body+"\n====\n"), 0)
			if err != nil {
				t.Fatal(err)
			}
			h := &g.pool.history
			if !h.exact || h.count != uint64(305+len(c.events)) {
				t.Fatalf("history exact=%v count=%d first=%q", h.exact, h.count, h.firstUnknown)
			}
			if len(m.definitions) != len(c.defs) {
				t.Fatalf("definitions: %d", len(m.definitions))
			}
			for i, want := range c.defs {
				d := headerDef(i)(m)
				if d.name.text != want || d.local != c.local[i] || len(d.params) != len(c.params[i]) {
					t.Fatalf("definition %d: name=%q local=%v params=%d", i, d.name.text, d.local, len(d.params))
				}
				for j, p := range c.params[i] {
					if d.params[j].name.text != p.text || d.params[j].arity != p.arity {
						t.Fatalf("definition %d param %d: %q/%d", i, j, d.params[j].name.text, d.params[j].arity)
					}
				}
			}
			for i, e := range c.events {
				node := e.node(m)
				got := h.reuse(node)
				if !got.known || got.uid != uint32(305+i) || got.class != e.class || got.kind != e.kind {
					t.Fatalf("event %d (%s): %#v", i, e.class, got)
				}
			}
		})
	}
}

// The canonical synonym of an infix formal is the bound name: the body's
// \oplus resolves to the formal declared as (+).
func TestHistoryInfixDeclCanonicalFormalBinding(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF(_ (+) _, x) == x \\oplus x\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	f := testUserDefinitions(t, m.definitions)[0]
	app := f.body.(*opApplNode)
	if !h.exact || h.count != 311 || app.operator != f.params[0] || h.reuse(f.params[0]).uid != 305 || h.reuse(f.params[1]).uid != 306 || h.reuse(app).uid != 309 || h.reuse(f).uid != 310 {
		t.Fatalf("exact=%v count=%d first=%q", h.exact, h.count, h.firstUnknown)
	}
}

func TestHistoryDuplicateFormalDiagnosticAndBinding(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF(a, a) == a\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	f := testUserDefinitions(t, m.definitions)[0]
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4201 || msgs[0].Format != "Multiply-defined symbol '%s': this definition or declaration conflicts \nwith the one at %s." {
		t.Fatalf("diagnostics: %+v", msgs)
	}
	if f.body.(*opApplNode).operator != f.params[0] || !g.pool.history.exact {
		t.Fatal("duplicate formal binding")
	}
}

// A RECURSIVE symbolic declaration is completed by an InfixLHS definition:
// the header's formals follow the declaration's placeholders, and
// endOpDefNode allocates no second OpDefNode (Generator.java:2598-2650,
// 2665-2666).
func TestHistoryRecursiveInfixHeader(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nRECURSIVE _ ++ _\na ++ b == a ++ b\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	f := testUserDefinitions(t, m.definitions)[0]
	app := f.body.(*opApplNode)
	if !h.exact || h.count != 314 || f.name.text != "++" || len(g.table.errors.Messages()) != 0 {
		t.Fatalf("exact=%v count=%d first=%q", h.exact, h.count, h.firstUnknown)
	}
	if h.reuse(f).uid != 308 || h.reuse(f.params[0]).uid != 309 || h.reuse(f.params[1]).uid != 310 || h.reuse(app.operands[0].(exprNode)).uid != 311 || h.reuse(app.operands[1].(exprNode)).uid != 312 || h.reuse(app).uid != 313 || app.operator != f {
		t.Fatal("recursive infix schedule")
	}
}

// LET definitions use the same processOperator header schedule, and are
// also appended to the module's definitions (Generator.java:2991).
func TestHistoryLetSymbolicHeader(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nG == LET a ++ b == a\n         -. c == c\n     IN -(TRUE ++ FALSE)\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	defs := testUserDefinitions(t, m.definitions)
	let := defs[len(defs)-1].body.(*letInNode)
	if !h.exact || len(let.definitions) != 2 {
		t.Fatalf("exact=%v count=%d first=%q", h.exact, h.count, h.firstUnknown)
	}
	plus, minus := let.definitions[0].(*userOpDefNode), let.definitions[1].(*userOpDefNode)
	if plus.name.text != "++" || minus.name.text != "-." || h.reuse(plus.params[0]).uid != 305 || h.reuse(plus.params[1]).uid != 306 || h.reuse(plus.body).uid != 307 || h.reuse(plus).uid != 308 || h.reuse(minus.params[0]).uid != 309 || h.reuse(minus.body).uid != 310 || h.reuse(minus).uid != 311 {
		t.Fatal("LET header schedule")
	}
	outer := let.body.(*opApplNode)
	if outer.operator != minus || outer.operands[0].(*opApplNode).operator != plus || h.reuse(outer).uid <= h.reuse(outer.operands[0].(exprNode)).uid {
		t.Fatal("LET symbolic applications")
	}
}

// Controls: the legacy direct-allocation helper keeps its own guards for
// every newly admitted header form. An InfixDecl formal used to bypass both
// cursors; with Java's FormalParamNode at 305, x and y take 313 and 314,
// whose default-set buckets (15, 0) order the extras "y x". The helper now
// refuses the case instead of rendering "x y" from shifted UIDs; the list
// then prints in the stable fallback order "x y", which matches Java's
// only order-normalised and certifies nothing. The allocation history is
// left as modeled: the residual iteration allocates nothing.
func TestHistoryOperatorHeaderLegacyJavaGuards(t *testing.T) {
	for _, c := range headerHistoryCases {
		g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\n"+c.body+"\n====\n"), 0)
		if err != nil {
			t.Fatal(err)
		}
		if g.pool.java.exact {
			t.Errorf("%s: legacy helper certified", c.label)
		}
	}
	g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF(_ + _, b, c, d, e, f, g, h, x, y) == L(x, y)::TRUE\n====\n"), 0)
	if msgs := g.table.errors.Messages(); err != nil || g.labelOrderFallbacks != 1 || g.pool.java.exact || len(msgs) != 1 || msgs[0].Parameters[0].Text != "Label L declares extra parameter(s)  x  y  " {
		t.Fatalf("infix formal label extras: err=%v fallbacks=%d cursor=%v history=%v msgs=%+v", err, g.labelOrderFallbacks, g.pool.java.exact, g.pool.history.exact, msgs)
	}
	g, _, err = analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF(a, b, c, d, e, f, g, h, x, y) == L(x, y)::TRUE\n====\n"), 0)
	msgs := g.table.errors.Messages()
	if err != nil || g.pool.java.atCheck != 317 || len(msgs) != 1 || msgs[0].Parameters[0].Text != "Label L declares extra parameter(s)  y  x  " {
		t.Fatalf("identifier control: err=%v atCheck=%d msgs=%+v", err, g.pool.java.atCheck, msgs)
	}
}

// Definitions that collide with an existing symbol are admitted by
// existingDefinitionHistoryAdmitted (resolution_history.go), with unchanged
// diagnostics.
func TestHistoryOperatorHeaderCollisionsAdmitted(t *testing.T) {
	for _, c := range []struct {
		body string
		code []int
	}{
		{"a ++ b == a\na ++ b == b", []int{4201, 4801}},
		{"a \\cup b == a", []int{4201, 4202}},
		{"LOCAL F(a) == a\nF(b) == b", []int{4201, 4801}},
	} {
		g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\n"+c.body+"\n====\n"), 0)
		if err != nil {
			t.Fatal(err)
		}
		h := &g.pool.history
		if !h.exact {
			t.Errorf("%q: exact=%v first=%q", c.body, h.exact, h.firstUnknown)
		}
		msgs := g.table.errors.Messages()
		var codes []int
		for _, d := range msgs {
			codes = append(codes, d.Code.Value)
		}
		if fmt.Sprint(codes) != fmt.Sprint(c.code) {
			t.Errorf("%q: codes %v", c.body, codes)
		}
	}
}

// Controls: the header predicate admits only the four parser LHS kinds and
// a nil or single-LOCAL-token zero, the shapes SANY's parser constructs.
func TestOperatorHeaderHistoryPredicate(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nLOCAL F == TRUE\nG == TRUE\n====\n")
	defs := tree.Heirs()[2].Heirs()
	local, plain := defs[0], defs[1]
	if len(local.Zero()) != 1 || local.Zero()[0].Kind() != 54 || plain.Zero() != nil {
		t.Fatalf("parser zero shapes: %d %v", len(local.Zero()), plain.Zero())
	}
	lhs := plain.One()[0]
	for _, c := range []struct {
		kind int
		zero bool
		want bool
	}{
		{366, false, true}, {400, false, true}, {372, false, true}, {396, false, true},
		{366, true, true}, {372, true, true},
		{363, false, false}, {370, false, false}, {0, false, false},
	} {
		zero := plain.Zero()
		if c.zero {
			zero = local.Zero()
		}
		if got := operatorHeaderHistoryAdmitted(c.kind, zero); got != c.want {
			t.Errorf("kind %d zero %v: %v", c.kind, c.zero, got)
		}
	}
	if operatorHeaderHistoryAdmitted(int(lhs.Kind()), local.One()[:1]) || operatorHeaderHistoryAdmitted(int(lhs.Kind()), append(local.Zero(), local.Zero()...)) || operatorHeaderHistoryAdmitted(int(lhs.Kind()), local.Zero()[:0]) {
		t.Error("non-LOCAL, repeated or empty non-nil zero admitted")
	}
}

// Cancellation at any point stops certification permanently and never
// publishes a ticket beyond the recorded count.
func TestHistoryOperatorHeaderCancellation(t *testing.T) {
	src := parsedModule(t, "---- MODULE M ----\nLOCAL a ++ b == a\nF(-. _, _ + _, x) == (-x) + x\nG == TRUE ++ FALSE\n====\n")
	finished := false
	for n := 0; n < 400 && !finished; n++ {
		left := n
		ctx := countdownContext{stdcontext.Background(), &left}
		g, m, err := analyzeOwned(ctx, src, 0)
		if g == nil {
			continue
		}
		h := &g.pool.history
		if err == nil && m != nil && !h.exact {
			// A cancellation seen only by a history event stops certification
			// while generation completes.
			if h.firstUnknown != "cancelled" {
				t.Fatalf("countdown %d: completed run first=%q", n, h.firstUnknown)
			}
			continue
		}
		if err == nil && m != nil {
			if !h.exact || h.count != 305+16 {
				t.Fatalf("countdown %d finished: exact=%v count=%d first=%q", n, h.exact, h.count, h.firstUnknown)
			}
			finished = true
			continue
		}
		if h.exact {
			t.Fatalf("countdown %d: aborted run still exact", n)
		}
		for node, ticket := range h.tickets {
			if uint64(ticket.uid) >= h.count {
				t.Fatalf("countdown %d: ticket %d beyond count %d for %T", n, ticket.uid, h.count, node)
			}
		}
	}
	if !finished {
		t.Fatal("countdown never finished")
	}
}

// Level observation over the real entry binds the admitted headers.
func TestLevelObservationAdmitsOperatorHeaders(t *testing.T) {
	ctx := stdcontext.Background()
	limits := modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400}
	acquire := func(root, dep string) *modulegraph.Acquisition {
		input := func(name, body string) modulegraph.Input {
			return modulegraph.Input{Path: name + ".tla", Origin: modulegraph.OriginWorkspace, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
		}
		a, e := modulegraph.AcquireFlatExternal(ctx, input("Root", root), func(stdcontext.Context, string) (modulegraph.Input, error) { return input("A", dep), nil }, limits)
		if e != nil {
			t.Fatal(e)
		}
		return a
	}
	for _, c := range []struct {
		label, root, dep string
		units            int
		first            string
	}{
		{"single", "LOCAL a ++ b == a\nF(-. _, _ \\oplus _, x) == (-x) (+) x\nG == TRUE ++ FALSE", "", 1, "none"},
		{"extends", "EXTENDS A\nG == TRUE ++ FALSE", "LOCAL L(x) == x\na ++ b == L(a)\nb ^+ == b", 2, "none"},
		{"collision", "a ++ b == a\na ++ b == b", "", 1, "none"},
	} {
		o := observeLevelCandidate(ctx, acquire(c.root, c.dep), 1000)
		checkCauseInvariants(t, c.label, o)
		if o.err != nil || !o.finished || len(o.attempts) != c.units || o.cause.historyFirst != c.first {
			t.Errorf("%s: err=%v finished=%v attempts=%d cause=%+v", c.label, o.err, o.finished, len(o.attempts), o.cause)
			continue
		}
		if c.first == "none" && (!o.bound || !o.levelComplete) {
			t.Errorf("%s: bound=%v level=%v cause=%+v", c.label, o.bound, o.levelComplete, o.cause)
		}
	}
}
