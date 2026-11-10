// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package semantic

import (
	stdcontext "context"
	"testing"
)

func TestUnresolvedSelectorNormalizesOperatorSynonyms(t *testing.T) {
	source := "---- MODULE Test ----\nop == {\n  A!B!\\cap(x, y),\n  A!B!~(x),\n  A!B!□(x)\n}\n====\n"
	g, err := newGenerator(stdcontext.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.module(parsedModule(t, source)); err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	names := []string{"A!B!\\intersect", "A!B!\\lnot", "A!B![]"}
	ends := []int{10, 7, 7}
	if len(msgs) != len(names) {
		t.Fatalf("diagnostics: %#v", msgs)
	}
	for i, msg := range msgs {
		if msg.Code.Value != 4200 || msg.Format != "Unknown operator: `%s'." || len(msg.Parameters) != 1 || msg.Parameters[0].Text != names[i] || msg.Location.BeginLine != i+3 || msg.Location.EndLine != i+3 || msg.Location.BeginColumn != 3 || msg.Location.EndColumn != ends[i] {
			t.Fatalf("diagnostic %d: %#v", i, msg)
		}
	}
}

// Java selectorToNode resolves the final operator synonym before lookup.
func TestInstanceSelectorResolvesOperatorSynonyms(t *testing.T) {
	source := "---- MODULE Root ----\n---- MODULE A ----\nx \\o y == x\nx \\geq y == x\n====\nI == INSTANCE A\nF == I!\\circ(1, 2)\nG == I!>=(1, 2)\nBad == I!>=(1)\n====\n"
	g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4204 {
		t.Fatalf("diagnostics: %#v", msgs)
	}
}

func TestUnqualifiedOperatorApplicationResolvesSynonym(t *testing.T) {
	source := "---- MODULE Root ----\nF == ⇸(1)\n====\n"
	g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 || msgs[0].Code.Value != 4204 || msgs[0].Parameters[0].Text != "-+->" {
		t.Fatalf("diagnostics: %#v", msgs)
	}
}
