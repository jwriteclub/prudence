// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"testing"
)

// selectorSubstitutionSource is shared with the tagged typed-bridge companion test.
const selectorSubstitutionSource = "---- MODULE Root ----\n---- MODULE A ----\nCONSTANT Op(_,_)\n====\nF == \\A y \\in {1}: LET G(x) == <<y,x>> IN G(1)\nINSTANCE A WITH Op <- F!@!G\nI == INSTANCE A WITH Op <- F!@!G\nR == I!Missing\nAfter == TRUE\n====\n"

func TestSelectorSubstitutionAndQualifiedUnknown(t *testing.T) {
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, selectorSubstitutionSource), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &g.pool.history
	msgs := g.table.errors.Messages()
	if !h.exact || len(m.instances) != 2 || len(msgs) != 1 || msgs[0].Code.Value != 4200 || len(msgs[0].Parameters) != 1 || msgs[0].Parameters[0].Kind != ObjectIdentityParameter || msgs[0].Parameters[0].JavaClass != "util.UniqueString" || !msgs[0].Parameters[0].UTF16Present || msgs[0].Parameters[0].Text != "I!Missing" || !h.reuse(m.context.lookup(contextKey{name: g.pool.intern("After")})).known {
		t.Fatalf("exact=%v reason=%q diagnostics=%v", h.exact, h.firstUnknown, msgs)
	}
	for _, inst := range m.instances {
		arg := inst.substs[0].expr.(*opArgNode)
		lam := arg.operator.(*userOpDefNode)
		body := lam.body.(*opApplNode)
		if !h.reuse(arg).known || !h.reuse(lam).known || !h.reuse(body).known || h.reuse(body).uid+1 != h.reuse(lam).uid || h.reuse(lam).uid+1 != h.reuse(arg).uid {
			t.Fatal("untracked wrapper")
		}
	}
}
