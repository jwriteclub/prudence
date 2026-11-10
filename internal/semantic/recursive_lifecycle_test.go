// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package semantic

import (
	stdcontext "context"
	"testing"
)

// Java scans the module's accumulated declarations at a reused LET depth.
// Its error accumulator deduplicates the earlier declaration's diagnostic.
func TestRecursiveSiblingUndefinedDiagnosticsRemainDistinct(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nA == LET RECURSIVE F IN 1\nB == LET RECURSIVE G IN 2\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4291 || msgs[1].Code.Value != 4291 {
		t.Fatalf("sibling diagnostics: %#v", msgs)
	}
	if len(m.recursiveDecls) != 2 || m.recursiveDecls[0].name.text != "F" || m.recursiveDecls[1].name.text != "G" || g.unresolvedSum != 0 {
		t.Fatal("sibling declaration bookkeeping")
	}
}

// The local table is popped before the sibling LET is entered; equal names
// in separate LETs must resolve to separate declared objects.
func TestRecursiveSiblingDefinedNamesHaveSeparateIdentity(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nA == LET RECURSIVE F F == 1 IN F\nB == LET RECURSIVE F F == 2 IN F\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.table.errors.Messages()) != 0 || len(m.recursiveDecls) != 2 || m.recursiveDecls[0] == m.recursiveDecls[1] || !m.recursiveDecls[0].defined || !m.recursiveDecls[1].defined || g.unresolvedSum != 0 {
		t.Fatal("sibling scopes or recursive identity")
	}
}

// Java's endOpDefNode signals WrongInvocationException when its global count
// goes negative. This port must stop incomplete, not issue a SANY rejection.
func TestEndRecursiveUnderflowIsIncomplete(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	n := &userOpDefNode{inRecursive: true}
	if err := g.endRecursive(n, nil, nil); err != ErrPortIncomplete {
		t.Fatalf("underflow: %v", err)
	}
	if !n.defined || g.unresolvedSum != -1 || len(g.table.errors.Messages()) != 0 {
		t.Fatal("underflow state or invented diagnostic")
	}
}
