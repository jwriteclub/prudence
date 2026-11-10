// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package modulegraph

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func requireBudget(t *testing.T, e error, want BudgetError) {
	t.Helper()
	var got *BudgetError
	if !errors.Is(e, ErrIncomplete) || !errors.As(e, &got) || *got != want {
		t.Fatalf("want %+v, got %v", want, e)
	}
}

// siblings returns a root body of k empty inner modules. Discovery binds each
// inner module's earlier peers, so its visits grow quadratically while the
// syntax-node walk grows linearly.
func siblings(k int) string {
	var b strings.Builder
	for i := 0; i < k; i++ {
		fmt.Fprintf(&b, "---- MODULE M%d ----\n====\n", i)
	}
	return b.String()
}

func TestBudgetDiscoveryPass(t *testing.T) {
	root := source("Root", siblings(15), 1)
	l := limits()
	if _, e := AcquireFlatExternal(context.Background(), root, nil, l); e != nil {
		t.Fatal(e)
	}
	l.MaxVisits = 50
	_, e := AcquireFlatExternal(context.Background(), root, nil, l)
	requireBudget(t, e, BudgetError{Pass: PassDiscovery, Limit: 50, Observed: 51})
	// The syntax-node walk keeps its own counter: with visits ample and
	// nodes short, the node pass stops first.
	l = limits()
	l.MaxNodes = 10
	_, e = AcquireFlatExternal(context.Background(), root, nil, l)
	requireBudget(t, e, BudgetError{Pass: PassSyntaxNodes, Limit: 10, Observed: 11})
}

func TestBudgetResolutionPass(t *testing.T) {
	var refs []string
	files := map[string]Input{}
	for i := 0; i < 20; i++ {
		n := fmt.Sprintf("A%d", i)
		refs = append(refs, n)
		files[n] = source(n, "", 1)
	}
	root := source("Root", "EXTENDS "+strings.Join(refs, ", "), 1)
	lookup := func(_ context.Context, n string) (Input, error) { return files[n], nil }
	l := limits()
	if _, e := AcquireFlatExternal(context.Background(), root, lookup, l); e != nil {
		t.Fatal(e)
	}
	// Discovery counts per unit and stays small; resolution counts across
	// the acquisition, so some budget stops it while discovery fits.
	seen := map[BudgetPass]bool{}
	for v := 1; v <= 2000; v++ {
		l.MaxVisits = v
		_, e := AcquireFlatExternal(context.Background(), root, lookup, l)
		if e == nil {
			break
		}
		var b *BudgetError
		if !errors.As(e, &b) || b.Limit != v || b.Observed != v+1 {
			t.Fatalf("visits %d: %v", v, e)
		}
		seen[b.Pass] = true
	}
	if !seen[PassResolution] {
		t.Fatal(seen)
	}
}

func TestBudgetDepthKeepsCategory(t *testing.T) {
	root := source("Root", siblings(3), 1)
	l := limits()
	l.MaxDepth = 4
	_, e := AcquireFlatExternal(context.Background(), root, nil, l)
	var b *BudgetError
	if e == nil || errors.As(e, &b) {
		t.Fatalf("depth stop must not be a budget error: %v", e)
	}
}

func TestBudgetCycleAndOrderPasses(t *testing.T) {
	g, e := New(100, 2, 100, 100)
	if e != nil {
		t.Fatal(e)
	}
	u := map[string]*Unit{}
	for _, n := range []string{"A", "B", "C", "D"} {
		if u[n], e = g.Unit(n, n+".tla"); e != nil {
			t.Fatal(e)
		}
	}
	ctx := context.Background()
	if e = g.Link(ctx, u["B"], u["C"], Extends); e != nil {
		t.Fatal(e)
	}
	if e = g.Link(ctx, u["A"], u["B"], Extends); e != nil {
		t.Fatal(e)
	}
	// Order visits A, B and C: three visits over a budget of two.
	_, e = g.Order(ctx, u["A"])
	requireBudget(t, e, BudgetError{Pass: PassOrder, Limit: 2, Observed: 3})
	// Linking D to A searches A, B and C for a cycle.
	e = g.Link(ctx, u["D"], u["A"], Extends)
	requireBudget(t, e, BudgetError{Pass: PassCycle, Limit: 2, Observed: 3})
}
