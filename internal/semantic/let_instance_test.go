// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	"strings"
	"testing"
)

func letDefinitionName(t *testing.T, n leafNode) string {
	t.Helper()
	switch v := n.(type) {
	case *userOpDefNode:
		return v.name.text
	case *thmOrAssumpDefNode:
		return v.name.text
	}
	t.Fatalf("definition %T", n)
	return ""
}

// TestLetModuleDefinitionVectors follows Generator.processLetIn's
// N_ModuleDefinition branch (Generator.java:2998-3000) into
// processModuleDefinition with the LET's vectors: the OpDef clones in
// instancee context order without LOCAL ones (4876-4950), the
// ThmOrAssumpDef clones (4960-5029), the InstanceNode in insts (5033-5044)
// and the ModuleInstanceKind marker last in defs (5051-5065), then the
// following LET operator. Nothing reaches the module-level lists except the
// LET operator, which processLetIn appends with cm.appendDef (2990-2991).
func TestLetModuleDefinitionVectors(t *testing.T) {
	src := "---- MODULE Inner ----\nCONSTANT c\nA == c\nLOCAL L == c\nTHEOREM T == TRUE\n====\n" +
		"I0 == INSTANCE Inner WITH c <- 0\n" +
		"F(p) == LET J(x) == INSTANCE Inner WITH c <- x\n            K == 1\n        IN J(p)!A\n" +
		"After == F(1)"
	g, m, err := esAnalyze(t, src)
	if err != nil || m == nil {
		t.Fatalf("%v %q", err, g.incomplete.site)
	}
	if got := renderedDiagnostics(t, g); len(got) != 0 {
		t.Fatalf("%q", got)
	}
	var f *userOpDefNode
	var names []string
	for _, d := range m.definitions {
		if u, ok := d.(*userOpDefNode); ok {
			names = append(names, u.name.text)
			if u.name.text == "F" {
				f = u
			}
		}
	}
	if strings.Join(names, ",") != "I0!A,K,F,After" {
		t.Fatalf("module definitions %v", names)
	}
	if len(m.instances) != 1 || m.instances[0].name.text != "I0" {
		t.Fatalf("module instances %+v", m.instances)
	}
	let, ok := f.body.(*letInNode)
	if !ok {
		t.Fatalf("body %T", f.body)
	}
	names = nil
	for _, d := range let.definitions {
		names = append(names, letDefinitionName(t, d))
	}
	if strings.Join(names, ",") != "J!A,J!T,J,K" {
		t.Fatalf("LET definitions %v", names)
	}
	marker := let.definitions[2].(*userOpDefNode)
	clone := let.definitions[0].(*userOpDefNode)
	if marker.kind != 6 || marker.arity != 1 || clone.kind != 5 || len(clone.params) != 1 || clone.params[0] != marker.params[0] || clone.letInLevel != 1 {
		t.Fatalf("marker %+v clone %+v", marker, clone)
	}
	if _, ok := clone.body.(*substInNode); !ok {
		t.Fatalf("clone body %T", clone.body)
	}
	if len(let.instances) != 1 {
		t.Fatalf("instances %+v", let.instances)
	}
	in := let.instances[0]
	if in.name.text != "J" || len(in.params) != 1 || in.params[0] != marker.params[0] || in.module == nil || in.module.name.text != "Inner" || len(in.substs) != 1 || in.local {
		t.Fatalf("instance %+v", in)
	}
	// The LET context owns J and its clones; the module context does not.
	for _, n := range []string{"J", "J!A", "J!T"} {
		key := contextKey{name: g.pool.intern(n)}
		if let.context.lookup(key) == nil || m.context.lookup(key) != nil {
			t.Fatalf("scope of %s", n)
		}
	}
}

// TestLetModuleDefinitionKeepsHistoryConservative: the LET branch reuses the
// module-definition constructor sequence, but no profile has certified it
// under a LET, so the private history stops being exact there; the same
// module without the LET definition stays exact.
func TestLetModuleDefinitionKeepsHistoryConservative(t *testing.T) {
	const prefix = "---- MODULE Inner ----\nA == 1\n====\nI == INSTANCE Inner\n"
	g, _, err := esAnalyze(t, prefix+"F == LET K == 1 IN K")
	if err != nil || !g.pool.history.exact {
		t.Fatalf("control: %v %q", err, g.pool.history.firstUnknown)
	}
	g, _, err = esAnalyze(t, prefix+"F == LET J == INSTANCE Inner IN J!A")
	if err != nil || g.pool.history.exact || g.pool.history.firstUnknown != "unproved LET module definition" {
		t.Fatalf("%v %v %q", err, g.pool.history.exact, g.pool.history.firstUnknown)
	}
}
