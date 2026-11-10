// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package modulegraph

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// The read-only accessors return copies, read only the acquired state, own
// their budget and stop on cancellation.
func TestModuleRefInspectionAccessors(t *testing.T) {
	root := source("Root", "EXTENDS A\n---- MODULE I1 ----\nX == 1\n====\n---- MODULE I2 ----\nEXTENDS I1\nINSTANCE Late\n====\nINSTANCE I1\nJ == INSTANCE I2\n---- MODULE Late ----\nY == 1\n====", 1)
	a, err := AcquireFlatExternal(context.Background(), root, func(_ context.Context, n string) (Input, error) {
		if n == "A" {
			return source("A", "Z == 1", 1), nil
		}
		return Input{}, errors.New("unexpected " + n)
	}, limits())
	if err == nil {
		t.Fatal("I2's INSTANCE Late follows no inner module of I2 and must be acquired")
	}
	root = source("Root", "EXTENDS A\n---- MODULE I1 ----\nX == 1\n====\n---- MODULE I2 ----\nEXTENDS I1\n====\nINSTANCE I1\nJ == INSTANCE I2", 1)
	a, err = AcquireFlatExternal(context.Background(), root, func(_ context.Context, n string) (Input, error) {
		return source("A", "Z == 1", 1), nil
	}, limits())
	if err != nil {
		t.Fatal(err)
	}
	r := a.Root.RootModule
	if len(r.Children) != 2 {
		t.Fatalf("children %d", len(r.Children))
	}
	i1, i2 := r.Children[0], r.Children[1]
	refs := r.References()
	if len(refs) != 3 || refs[0] != (Reference{Name: "A", Tree: refs[0].Tree, Kind: Extends}) || refs[1].Name != "I1" || refs[2].Name != "I2" || refs[1].Kind != Instance {
		t.Fatalf("root references %+v", refs)
	}
	refs[0].Name = "changed"
	if r.References()[0].Name != "A" || !reflect.DeepEqual(a.Root.References(), r.References()) {
		t.Fatal("References is not a copy of the root module's references")
	}
	if r.Bound("A") != a.Units[1].RootModule || r.Bound("A").Parent != nil || r.Bound("I1") != nil {
		t.Fatal("root bindings")
	}
	if i2.Bound("I1") != i1 || i1.Parent != r || len(i2.References()) != 1 {
		t.Fatal("inner binding at discovery")
	}
	for _, c := range []struct {
		name string
		want bool
	}{{"I1", true}, {"I2", true}, {"A", false}, {"Missing", false}} {
		got, err := r.ResolvesInternally(context.Background(), c.name, 100)
		if err != nil || got != c.want {
			t.Fatalf("%s internal %v err %v", c.name, got, err)
		}
	}
	if _, err := r.ResolvesInternally(context.Background(), "I2", 2); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("budget err %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.ResolvesInternally(ctx, "I1", 100); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err %v", err)
	}
	if _, err := r.ResolvesInternally(nil, "I1", 100); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("nil context err %v", err)
	}
	if _, err := (*ModuleRef)(nil).ResolvesInternally(context.Background(), "I1", 100); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("nil module err %v", err)
	}
	if _, err := r.ResolvesInternally(context.Background(), "I1", 0); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("zero budget err %v", err)
	}
	if d, v := a.InspectionLimits(); d != 1000 || v != 10000 {
		t.Fatalf("limits %d %d", d, v)
	}
	if d, v := (*Acquisition)(nil).InspectionLimits(); d != 0 || v != 0 {
		t.Fatal("nil acquisition limits")
	}
	// Nothing above changed the acquisition.
	if len(a.Units) != 2 || len(a.Root.Edges()) != 1 || a.Root.Linked("A") != a.Units[1] || r.Bound("I1") != nil {
		t.Fatal("inspection changed the acquisition")
	}
}
