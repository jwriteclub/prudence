// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"errors"
	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"testing"
)

func TestDeclaredSymbolSubexpressionErrors(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nCONSTANT C\nSTRING[x \\in {1}] == mark(x):: x\nA == STRING!mark\nB == C!missing\nF(x) == x!missing\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 4 || msgs[0].Code.Value != 4201 {
		t.Fatalf("diagnostics: %#v", msgs)
	}
	for i, name := range []string{"STRING", "C", "x"} {
		msg := msgs[i+1]
		if msg.Code.Value != 4005 || msg.Format != "Cannot take subexpression of `%s'." || len(msg.Parameters) != 1 || msg.Parameters[0].Text != name {
			t.Fatalf("diagnostic %d: %#v", i, msg)
		}
	}
}

func TestDeclaredSymbolSubexpressionAsOperatorArgument(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nApply(P(_)) == P(1)\nA == Apply(STRING!mark)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4005 || msgs[0].Format != "Cannot take subexpression of `%s'." || msgs[1].Code.Value != 4271 {
		t.Fatalf("diagnostics: %#v", msgs)
	}
}

func TestRecursivePlaceholderSubexpressionErrors(t *testing.T) {
	for _, tc := range []struct {
		source string
		codes  []int
	}{
		{"RECURSIVE F(_)\nF(x) == F(x)!missing", []int{4005}},
		{"RECURSIVE F\nApply(P(_)) == P(1)\nF == Apply(F!missing)", []int{4005, 4271}},
	} {
		t.Run(tc.source, func(t *testing.T) {
			g, _ := newGenerator(stdcontext.Background(), 0)
			_, e := g.module(parsedModule(t, "---- MODULE M ----\n"+tc.source+"\n====\n"))
			if e != nil {
				t.Fatal(e)
			}
			msgs := g.table.errors.Messages()
			if len(msgs) != len(tc.codes) {
				t.Fatalf("diagnostics: %#v", msgs)
			}
			for i, code := range tc.codes {
				if msgs[i].Code.Value != code {
					t.Fatalf("diagnostics: %#v", msgs)
				}
			}
			if msgs[0].Format != "Subexpression of  `%s' used inside the operator's definition." || msgs[0].Parameters[0].Text != "F" {
				t.Fatalf("placeholder diagnostic: %#v", msgs[0])
			}
		})
	}
}

func TestUnresolvedSelectorCompoundNameAndSkippedArguments(t *testing.T) {
	for _, tc := range []struct {
		selection, name string
		end             int
	}{
		{"Nope!Missing", "Nope!Missing", 17},
		{"Nope(Unknown)!Missing", "Nope!Missing", 26},
		{"Nope!1!Missing", "Nope!Missing", 19},
	} {
		t.Run(tc.selection, func(t *testing.T) {
			g, _ := newGenerator(stdcontext.Background(), 0)
			_, e := g.module(parsedModule(t, "---- MODULE M ----\nA == "+tc.selection+"\n====\n"))
			if e != nil {
				t.Fatal(e)
			}
			msgs := g.table.errors.Messages()
			if len(msgs) != 1 || msgs[0].Code.Value != 4200 || msgs[0].Parameters[0].Text != tc.name || msgs[0].Location.BeginColumn != 6 || msgs[0].Location.EndColumn != tc.end {
				t.Fatalf("diagnostics: %#v", msgs)
			}
		})
	}
}

func TestUnresolvedSelectorDoesNotRejectExistingCompoundSymbol(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	name := g.pool.intern("Nope!Missing")
	builtin := g.pool.builtins.lookup(contextKey{name: g.pool.intern("TRUE")})
	g.table.top.add(contextKey{name: name}, builtin)
	_, e := g.module(parsedModule(t, "---- MODULE M ----\nA == Nope!Missing\n====\n"))
	if e != ErrPortIncomplete || len(g.table.errors.Messages()) != 0 {
		t.Fatalf("compound namespace boundary: %v %#v", e, g.table.errors.Messages())
	}
}

func TestUnresolvedSelectorHonorsDepthAndCancellation(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 1)
	if _, err := g.unresolvedSelector([]*generated.SyntaxTreeNode{nil}, nil, 1); !errors.Is(err, ErrPortIncomplete) {
		t.Fatalf("depth: %v", err)
	}
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	g, _ = newGenerator(ctx, 0)
	if _, err := g.unresolvedSelector([]*generated.SyntaxTreeNode{nil}, nil, 0); err != stdcontext.Canceled {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestUnresolvedSymbolicSelectorOperatorArgument(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nApply(P(_,_)) == P(1, 2)\nA == Apply(Missing!+)\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, e := g.module(tree)
	if e != nil {
		t.Fatal(e)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 || msgs[0].Code.Value != 4200 || msgs[0].Parameters[0].Text != "Missing!+" || msgs[1].Code.Value != 4271 {
		t.Fatalf("diagnostics: %#v", msgs)
	}
}
