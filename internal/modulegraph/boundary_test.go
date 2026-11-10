// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package modulegraph

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestOrderReusesCompletedUnits(t *testing.T) {
	g, _ := New(100, 100, 100, 200)
	root, _ := g.Unit("root", "root")
	prev := []*Unit{root}
	for i := 0; i < 22; i++ {
		a, _ := g.Unit(fmt.Sprintf("a%d", i), "")
		b, _ := g.Unit(fmt.Sprintf("b%d", i), "")
		for _, u := range prev {
			u.extendees = []*Unit{a, b}
		}
		prev = []*Unit{a, b}
	}
	order, err := g.Order(context.Background(), root)
	if err != nil || len(order) != 45 || order[len(order)-1] != root {
		t.Fatalf("len=%d err=%v", len(order), err)
	}
	g.maxVisits = 10
	if _, err = g.Order(context.Background(), root); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("budget err=%v", err)
	}
	g.maxVisits = 100
	prev[0].extendees = []*Unit{root}
	if _, err = g.Order(context.Background(), root); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("cycle err=%v", err)
	}
}

func TestOrderUsesUnitIdentityForTraversal(t *testing.T) {
	g, _ := New(20, 20, 20, 20)
	root, _ := g.Unit("root", "")
	first, _ := g.Unit("same", "")
	child, _ := g.Unit("child", "")
	second := &Unit{nameKey: "same", owner: g, extendees: []*Unit{child}}
	root.extendees = []*Unit{first, second}
	order, err := g.Order(context.Background(), root)
	if err != nil || len(order) != 3 || order[0] != first || order[1] != child || order[2] != root {
		t.Fatalf("order=%v err=%v", order, err)
	}
}
