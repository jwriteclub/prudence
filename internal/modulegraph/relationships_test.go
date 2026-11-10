// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package modulegraph

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func fixture(t *testing.T, names ...string) (*Graph, map[string]*Unit) {
	t.Helper()
	g, e := New(100, 10000, 100, 100)
	if e != nil {
		t.Fatal(e)
	}
	m := map[string]*Unit{}
	for _, n := range names {
		u, e := g.Unit(n, n+".tla")
		if e != nil {
			t.Fatal(e)
		}
		m[n] = u
	}
	return g, m
}
func connect(t *testing.T, g *Graph, a, b *Unit, k EdgeKind) {
	t.Helper()
	if e := g.Link(context.Background(), a, b, k); e != nil {
		t.Fatal(e)
	}
}
func names(units []*Unit) []string {
	v := make([]string, len(units))
	for i, u := range units {
		v[i] = u.nameKey
	}
	return v
}
func TestOrderedDependenciesAndDiamond(t *testing.T) {
	g, m := fixture(t, "Root", "A", "B", "C", "D")
	connect(t, g, m["A"], m["D"], Extends)
	connect(t, g, m["B"], m["D"], Extends)
	connect(t, g, m["Root"], m["A"], Extends)
	connect(t, g, m["Root"], m["B"], Extends)
	connect(t, g, m["Root"], m["C"], Instance)
	got, e := g.Order(context.Background(), m["Root"])
	if e != nil || !reflect.DeepEqual(names(got), []string{"D", "A", "B", "C", "Root"}) {
		t.Fatal(names(got), e)
	}
	again, e := g.Unit("A", "other path is not identity")
	if e != nil || again != m["A"] {
		t.Fatal("parse-unit name cache", e)
	}
}
func TestCycleCheckMutatesExtendeesBeforeLaterExtension(t *testing.T) {
	g, m := fixture(t, "Root", "A", "B", "C")
	connect(t, g, m["A"], m["B"], Instance)
	// Cycle checking A as target appends B to A's actual extendees vector.
	connect(t, g, m["Root"], m["A"], Extends)
	connect(t, g, m["A"], m["C"], Extends)
	got, e := g.Order(context.Background(), m["Root"])
	if e != nil || !reflect.DeepEqual(names(got), []string{"B", "C", "A", "Root"}) {
		t.Fatal(names(got), e)
	}
}
func TestCyclePathAndIdentity(t *testing.T) {
	g, m := fixture(t, "A", "B", "C")
	connect(t, g, m["A"], m["B"], Extends)
	connect(t, g, m["B"], m["C"], Instance)
	e := g.Link(context.Background(), m["C"], m["A"], Extends)
	var cycle *CycleError
	if !errors.As(e, &cycle) || !reflect.DeepEqual(cycle.Path, []string{"A.tla", "B.tla", "C.tla", "A.tla"}) {
		t.Fatal(e)
	}
}
func TestCancellationDepthAndForeignUnit(t *testing.T) {
	g, m := fixture(t, "A", "B")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(g.Link(ctx, m["A"], m["B"], Extends), context.Canceled) {
		t.Fatal("cancellation")
	}
	h, n := fixture(t, "A")
	_ = h
	if !errors.Is(g.Link(context.Background(), m["A"], n["A"], Extends), ErrIncomplete) {
		t.Fatal("foreign graph accepted")
	}
	shallow, _ := New(1, 10000, 100, 100)
	a, _ := shallow.Unit("A", "a")
	b, _ := shallow.Unit("B", "b")
	if e := shallow.Link(context.Background(), a, b, Extends); e != nil {
		t.Fatal(e)
	}
	if _, e := shallow.Order(context.Background(), a); !errors.Is(e, ErrIncomplete) {
		t.Fatal("depth", e)
	}
}

func TestDistinctNamesWithSameSourceLabelAndLimits(t *testing.T) {
	g, e := New(10, 100, 2, 1)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := g.Unit("outer", "same.tla")
	b, _ := g.Unit("inner-parse-unit", "same.tla")
	if a == b {
		t.Fatal("source label collapsed distinct parse units")
	}
	if _, e = g.Unit("third", "third.tla"); !errors.Is(e, ErrIncomplete) {
		t.Fatal("unit cap", e)
	}
	if e = g.Link(context.Background(), a, b, Extends); e != nil {
		t.Fatal(e)
	}
	if e = g.Link(context.Background(), a, b, Instance); !errors.Is(e, ErrIncomplete) {
		t.Fatal("edge cap", e)
	}
}
