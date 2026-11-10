// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package semantic

import (
	stdcontext "context"
	"reflect"
	"testing"
)

func TestFunctionConflictingWithBuiltinRetainsIdentity(t *testing.T) {
	for _, source := range []string{
		"STRING[x \\in {1}] == mark(x):: x\nA == TRUE",
		"A == LET STRING[x \\in {1}] == mark(x):: x IN TRUE",
	} {
		t.Run(source, func(t *testing.T) {
			g, _ := newGenerator(stdcontext.Background(), 0)
			builtin := g.pool.builtins.lookup(contextKey{name: g.pool.intern("STRING")}).(*builtinOpDefNode)
			before := *builtin
			m, err := g.module(parsedModule(t, "---- MODULE M ----\n"+source+"\n====\n"))
			if err != nil {
				t.Fatal(err)
			}
			msgs := g.table.errors.Messages()
			if len(msgs) != 1 || msgs[0].Code.Value != 4201 || msgs[0].Format != "Function name `%s' already defined or declared." {
				t.Fatalf("diagnostics: %#v", msgs)
			}
			after := *builtin
			after.labels = before.labels
			if m.definitions[0] != builtin || !reflect.DeepEqual(before, after) || builtin.labels == nil || builtin.labels.get(g.pool.intern("mark")) == nil {
				t.Fatal("builtin identity or state")
			}
			if _, ok := m.definitions[1].(*userOpDefNode); !ok {
				t.Fatal("later definition order")
			}
			for _, cached := range m.opDefs() {
				if cached.name == builtin.name {
					t.Fatal("builtin entered context-derived opDefs cache")
				}
			}
			if len(source) > 0 && source[0] == 'A' {
				let := m.definitions[1].(*userOpDefNode).body.(*letInNode)
				if len(let.definitions) != 1 || let.definitions[0] != builtin {
					t.Fatal("LET definition identity")
				}
			}
			other, _ := newGenerator(stdcontext.Background(), 0)
			otherBuiltin := other.pool.builtins.lookup(contextKey{name: other.pool.intern("STRING")}).(*builtinOpDefNode)
			if otherBuiltin == builtin || otherBuiltin.labels != nil {
				t.Fatal("builtin leaked between requests")
			}
		})
	}
}
