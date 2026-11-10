// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"fmt"
	"testing"
)

func TestFreshLabelAllocation(t *testing.T) {
	cases := []struct {
		definition, want string
		count            uint64
	}{
		{"F(a,b) == L(b,a)::TRUE", "Label L declares extra parameter(s)  a  b  ", 309},
		{"F(a,b,c,d,e,f,g,h,i,j) == L(a,j)::FALSE", "Label L declares extra parameter(s)  j  a  ", 317},
	}
	for _, tt := range cases {
		g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\n"+tt.definition+"\n====\n"), 0)
		if err != nil {
			t.Fatalf("%s: %v, count=%d", tt.definition, err, g.pool.java.count)
		}
		msgs := g.table.errors.Messages()
		if len(msgs) != 1 || msgs[0].Parameters[0].Text != tt.want || g.pool.java.atCheck != tt.count {
			t.Fatalf("%s: %#v count=%d", tt.definition, msgs, g.pool.java.atCheck)
		}
	}
}

func TestBoundedLabelSetContract(t *testing.T) {
	a := freshJavaAlloc()
	if a.count != 300 {
		t.Fatalf("bootstrap count %d", a.count)
	}
	a.generator()
	a.module() // first module
	p, q, r := new(formalParamNode), new(formalParamNode), new(formalParamNode)
	a.tickets[p] = javaTicket{uid: 305, kind: 11, class: classFormalParam, known: true}
	a.tickets[q] = javaTicket{uid: 321, kind: 11, class: classFormalParam, known: true}
	a.tickets[r] = javaTicket{uid: 306, kind: 11, class: classFormalParam, known: true}
	got, ok := a.orderedLabelExtras([]*formalParamNode{q, p, q, r}, map[*formalParamNode]bool{q: true, p: true})
	if !ok || len(got) != 2 || got[0] != q || got[1] != p {
		t.Fatalf("same bucket and removal: %v %v", got, ok)
	}
	all := make([]*formalParamNode, 8)
	rest := map[*formalParamNode]bool{}
	for i := range all {
		all[i] = new(formalParamNode)
		a.tickets[all[i]] = javaTicket{uid: uint32(305 + i), kind: 11, class: classFormalParam, known: true}
	}
	rest[all[0]] = true
	rest[all[1]] = true
	if _, ok = a.orderedLabelExtras(all, rest); ok {
		t.Fatal("eight inserted identities admitted")
	}
	for stage := 0; stage < 3; stage++ {
		b := freshJavaAlloc()
		b.generator()
		b.module()
		b.tickets[p] = a.tickets[p]
		b.tickets[q] = a.tickets[q]
		if stage == 0 {
			b.invalidate()
		}
		b.event() // leaf
		if stage == 1 {
			b.invalidate()
		}
		b.event() // label
		if stage == 2 {
			b.invalidate()
		}
		if _, ok = b.orderedLabelExtras([]*formalParamNode{p, q}, map[*formalParamNode]bool{p: true, q: true}); ok {
			t.Fatalf("invalidation stage %d", stage)
		}
		if !b.tickets[p].known {
			t.Fatal("lost historical ticket")
		}
	}

	b := freshJavaAlloc()
	b.count = (uint64(1) << 32) - 1
	if b.event().known || b.exact {
		t.Fatal("counter boundary admitted")
	}
}

func TestFreshProfileBoundaries(t *testing.T) {
	cases := []string{
		"F(a,b,c,d,e,f,g,h) == L(a,b,c,d,e,f,g,h)::TRUE",
		"X == TRUE\nF(a,b) == L(a,b)::TRUE",
		"F(a,b) == L(a,b)::(TRUE /\\ TRUE)",
		"F(a,b) == outer::L(a,b)::TRUE",
		"F(a,b) == L(a,b)::1",
		"F(a,b) == L(a,b,x)::TRUE",
	}
	// Outside the profile the cursor refuses; the distinct names then print
	// in the stable fallback order (label_extra_order.go).
	for _, src := range cases {
		g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\n"+src+"\n====\n"), 0)
		if err != nil || g.labelOrderFallbacks != 1 {
			t.Errorf("%q: %v, fallbacks %d", src, err, g.labelOrderFallbacks)
		}
	}
}

func TestAllocationRequestOwnership(t *testing.T) {
	const source = "---- MODULE M ----\nF(a,b) == L(b,a)::TRUE\n====\n"
	tree := parsedModule(t, source)
	done := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			g, _, err := analyzeOwned(stdcontext.Background(), tree, 0)
			if err == nil && g.pool.java.atCheck != 309 {
				err = fmt.Errorf("count %d", g.pool.java.atCheck)
			}
			done <- err
		}()
	}
	for i := 0; i < 8; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func TestJavaTicketsDoNotChangeGraphIdentity(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nF(a,b) == L(a,a)::TRUE\n====\n")
	plain, _ := newGenerator(stdcontext.Background(), 0)
	m1, e := plain.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	tracked, m2, e := analyzeOwned(stdcontext.Background(), tree, 0)
	if e != nil {
		t.Fatal(e)
	}
	d1, d2 := testUserDefinitions(t, m1.definitions)[0], testUserDefinitions(t, m2.definitions)[0]
	if m1.identity != m2.identity || d1.identity != d2.identity || d1.params[0].identity != d2.params[0].identity || plain.pool.nextNode != tracked.pool.nextNode {
		t.Fatal("graph identities changed")
	}
	if plain.pool.java.exact || len(plain.pool.java.tickets) != 0 {
		t.Fatal("direct constructor certified")
	}
}

func TestBoundedLabelDuplicateSequence(t *testing.T) {
	g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\nF(a,b) == L(b,a,b)::TRUE\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4330 || msgs[1].Code.Value != 4332 || msgs[1].Parameters[0].Text != "Label L declares extra parameter(s)  a  b  " {
		t.Fatalf("%#v", msgs)
	}
}
func TestRepeatedGeneratorAndModuleInvalidate(t *testing.T) {
	a := freshJavaAlloc()
	a.generator()
	a.generator()
	if a.exact {
		t.Fatal("second generator admitted")
	}
	b := freshJavaAlloc()
	b.generator()
	b.module()
	b.module()
	if b.exact {
		t.Fatal("second module admitted")
	}
}

// Dependencies stay incomplete. A nested module leaves the cursor, so the
// label extras print in the stable fallback order.
func TestDependencyAndNestedModuleRemainIncomplete(t *testing.T) {
	for _, src := range []string{
		"---- MODULE M ----\nEXTENDS Naturals\nF(a,b) == L(a,b)::TRUE\n====\n",
		"---- MODULE M ----\nINSTANCE Other\nF(a,b) == L(a,b)::TRUE\n====\n",
	} {
		_, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, src), 0)
		if err != ErrPortIncomplete {
			t.Errorf("%q: %v", src, err)
		}
	}
	src := "---- MODULE M ----\n---- MODULE Inner ----\nX == TRUE\n====\nF(a,b) == L(a,b)::TRUE\n====\n"
	g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, src), 0)
	if err != nil || g.labelOrderFallbacks != 1 {
		t.Errorf("%q: %v, fallbacks %d", src, err, g.labelOrderFallbacks)
	}
}
