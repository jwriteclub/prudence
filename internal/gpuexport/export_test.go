// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package gpuexport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/jwriteclub/prudence/internal/modulegraph"
	"github.com/jwriteclub/prudence/internal/semantic"
)

func TestResolvedBindingsAndInstances(t *testing.T) {
	dep := `---- MODULE Dep ----
CONSTANT K
W == K
====
`
	src := `---- MODULE Root ----
CONSTANT C
A == INSTANCE Dep WITH K <- 1
B == INSTANCE Dep WITH K <- 2
Apply(Op(_), v) == Op(v)
Choose(v) == \E <<x,y>> \in {<<1,2>>}: LET F(x1) == x1 = x IN Apply(F,y)
Both == A!W = B!W
====
`
	input := modulegraph.Input{Path: "Root.tla", FileName: "Root", Text: []byte(src)}
	resolve := func(_ context.Context, name string) (modulegraph.Input, error) {
		if name != "Dep" {
			return modulegraph.Input{}, fs.ErrNotExist
		}
		return modulegraph.Input{Path: "Dep.tla", Text: []byte(dep)}, nil
	}
	cfg := []byte("CONSTANT C = {m, \"m\", 1, TRUE}\nINVARIANTS Both\nCHECK_DEADLOCK FALSE")
	run := func() *Document {
		d, e := Export(context.Background(), input, resolve, "X.cfg", cfg)
		if e != nil {
			t.Fatal(e)
		}
		return d
	}
	d := run()
	if d.Version != 1 || d.ConfigSHA256 != digest(cfg) || d.RootSHA256 != digest(input.Text) || len(d.Modules) != 2 || len(d.Sources) != 2 {
		t.Fatal("header", d.Sources)
	}
	nodes := make(map[uint64]semantic.ResolvedNode)
	for _, n := range d.Nodes {
		if _, ok := nodes[n.ID]; n.ID == 0 || ok {
			t.Fatal("duplicate/zero ID", n.ID)
		}
		nodes[n.ID] = n
	}
	node := func(id uint64) semantic.ResolvedNode {
		n, ok := nodes[id]
		if !ok {
			t.Fatalf("unknown node %d", id)
		}
		return n
	}
	root := node(d.Root)
	def := func(name string) semantic.ResolvedNode { return node(root.Symbols[name]) }
	choose := def("Choose")
	q := node(choose.Body)
	if q.Kind != "apply" || node(q.Operator).Name != "$BoundedExists" || len(q.Binders) != 1 || !q.Binders[0].Tuple || len(q.Binders[0].Params) != 2 || q.Binders[0].Domain == 0 {
		t.Fatal("quantifier", q)
	}
	x, y := q.Binders[0].Params[0], q.Binders[0].Params[1]
	if node(x).Name != "x" || node(y).Name != "y" || node(x).Span.Source != "Root" || q.Span.Start[0] != 6 {
		t.Fatal("bound parameter or source", node(x), q)
	}
	let := node(q.Args[0])
	f := node(let.Definitions[0])
	eq := node(f.Body)
	if f.Name != "F" || node(eq.Args[0]).Operator != f.Params[0] || node(eq.Args[1]).Operator != x {
		t.Fatal("LET formal and captured binder", f, eq)
	}
	apply := def("Apply")
	call := node(let.Body)
	if call.Operator != apply.ID || node(call.Args[0]).Kind != "opArg" || node(call.Args[0]).Operator != f.ID || node(call.Args[1]).Operator != y || node(apply.Body).Operator != apply.Params[0] {
		t.Fatal("higher-order reference", call)
	}
	both := node(def("Both").Body)
	a := def("A!W")
	b := def("B!W")
	if a.ID == b.ID || node(both.Args[0]).Operator != a.ID || node(both.Args[1]).Operator != b.ID {
		t.Fatal("INSTANCE binding", both)
	}
	for i, d := range []semantic.ResolvedNode{a, b} {
		sub := node(d.Body)
		if sub.Kind != "substIn" || len(sub.Substitutions) != 1 || sub.FromModule != root.ID || node(sub.ToModule).Name != "Dep" {
			t.Fatal("INSTANCE body", sub)
		}
		s := sub.Substitutions[0]
		value := node(s.Value)
		if node(s.Target).Name != "K" || node(sub.Body).Operator != s.Target || value.Value == nil || *value.Value != []string{"1", "2"}[i] {
			t.Fatal("substitution", s)
		}
	}
	check, _ := json.Marshal(d)
	again, _ := json.Marshal(run())
	if !bytes.Equal(check, again) {
		t.Fatal("same input generated different graph bytes")
	}
}
func TestConfig(t *testing.T) {
	cfg := "(* heading *) CONSTANTS S = {id, \"id\", TRUE, 02, {}} \\* comment\nIS = FALSE\nSPECIFICATION Run\nINVARIANTS First Second\nCONSTRAINT Guard\nCHECK_DEADLOCK FALSE"
	ds, err := parseConfig(context.Background(), "model.cfg", []byte(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 7 || ds[0].Name != "S" || ds[1].Name != "IS" || ds[2].Kind != "SPECIFICATION" || ds[3].Name != "First" || ds[4].Name != "Second" || ds[5].Kind != "CONSTRAINT" || ds[6].Value.Text != "FALSE" {
		t.Fatal(ds)
	}
	for i, kind := range []string{"model", "string", "boolean", "integer", "set"} {
		if ds[0].Value.Items[i].Kind != kind {
			t.Fatal(ds[0])
		}
	}
	if ds[0].Span.Start[0] != 1 || ds[6].Span.End[0] != 6 {
		t.Fatal("cfg coordinates", ds)
	}
	for _, cfg := range []string{"CONSTANT K <- N", "CONSTANT K = {a,}", "CONSTANT K = {a b}", "CONSTANT K = \"a\\nb\"", "CONSTANT K = 999999999999", "NEXT", "NEXT A NEXT B", "INVARIANT I ALIAS X", "INIT /\\", "CHECK_DEADLOCK yes", "INVARIANTS I \x00 NEXT N"} {
		if _, err := parseConfig(context.Background(), "control.cfg", []byte(cfg)); err == nil || !strings.Contains(err.Error(), "control.cfg") {
			t.Errorf("invalid %q: %v", cfg, err)
		}
	}
}
func TestRejectedGraph(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	src := modulegraph.Input{Path: "Root.tla", Text: []byte("---- MODULE Root ----\nF == Missing\n====\n")}
	if d, e := Export(ctx, src, nil, "", nil); d != nil || !errors.Is(e, context.Canceled) {
		t.Fatal(d, e)
	}
	if d, e := Export(context.Background(), src, nil, "", nil); d != nil || e == nil || !strings.Contains(e.Error(), "Missing") {
		t.Fatal(d, e)
	}
}
