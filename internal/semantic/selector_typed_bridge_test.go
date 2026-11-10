//go:build prudence_phase2_adapter

// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"testing"
)

// These typed-bridge conversions were split from their untagged history tests,
// which keep every non-bridge assertion in the ordinary build.
func TestSelectorUnknownDiagnosticsTypedBridge(t *testing.T) {
	g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, selectorUnknownSource), 0)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 2 {
		t.Fatalf("diagnostics=%v", msgs)
	}
	if _, err := Phase2TypedGenerationStageDiagnostics(msgs); err != nil {
		t.Fatal(err)
	}
}

func TestSelectorSubstitutionTypedBridge(t *testing.T) {
	g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, selectorSubstitutionSource), 0)
	if err != nil {
		t.Fatal(err)
	}
	msgs := g.table.errors.Messages()
	if len(msgs) != 1 {
		t.Fatalf("diagnostics=%v", msgs)
	}
	if _, err := Phase2TypedGenerationStageDiagnostics(msgs); err != nil {
		t.Fatal(err)
	}
}
