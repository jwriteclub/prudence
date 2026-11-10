//go:build prudence_phase2_adapter

// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	"bytes"
	stdcontext "context"
	"strconv"
	"strings"
	"testing"
)

func TestFunctionBuiltinConflictLETXMLIdentity(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(parsedModule(t, "---- MODULE M ----\nA == LET STRING[x \\in {1}] == mark(x):: x IN TRUE\n====\n"))
	if err != nil {
		t.Fatal(err)
	}
	builtin := m.definitions[0].(*builtinOpDefNode)
	var out bytes.Buffer
	if err = phase2XML(stdcontext.Background(), &out, m, 1000, t.TempDir(), 1<<20); err != nil {
		t.Fatal(err)
	}
	ref := "<BuiltInKindRef><UID>" + strconv.FormatUint(builtin.identity, 10) + "</UID></BuiltInKindRef>"
	if !strings.Contains(out.String(), "<opDefs>"+ref+"</opDefs>") || !strings.Contains(out.String(), "<BuiltInKind>") {
		t.Fatal("missing actual builtin identity in LET export")
	}
}
