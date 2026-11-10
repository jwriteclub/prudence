// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package semantic

import (
	stdcontext "context"
	"os"
	"testing"

	"github.com/jwriteclub/prudence/internal/modulegraph"
)

// The null body and non-GeneralId operator paths follow Generator.java and
// SubstInNode.java. The fixture combines both recovery paths in source order.
func TestInstanceDiagnosticRecovery(t *testing.T) {
	inner, err := os.ReadFile("testdata/instance_diagnostics/Inner.tla")
	if err != nil {
		t.Fatal(err)
	}
	target, err := os.ReadFile("testdata/instance_diagnostics/Target.tla")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, source string
		end          int
	}{
		{"named", string(target), 39},
		{"unnamed", "---- MODULE Target ----\nINSTANCE Inner WITH F <- -, k <- 1\n====\n", 34},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), modulegraph.Input{Path: "Target", Text: []byte(tc.source)}, func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
				return modulegraph.Input{Path: n, Text: inner}, nil
			}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
			if err != nil {
				t.Fatal(err)
			}
			msgs, examined, _, err := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
			if err != nil || !examined {
				t.Fatalf("examined %v error %v", examined, err)
			}
			if len(msgs) != 5 {
				t.Fatalf("messages: %#v", msgs)
			}
			if msgs[0].Code.Value != 4004 || msgs[1].Code.Value != 4004 || msgs[2].Code.Value != 4004 || msgs[2].Format != "Could not find declaration or definition of symbol '%s'." || msgs[2].Parameters[0].Text != "-" || msgs[3].Code.Value != 4243 || msgs[4].Code.Value != 4003 || msgs[4].Format != "Substitution error, probably due to error \nin module being instantiated." || msgs[4].Location.BeginLine != 2 || msgs[4].Location.BeginColumn != 1 || msgs[4].Location.EndColumn != tc.end {
				t.Fatalf("messages: %#v", msgs)
			}
		})
	}
}
