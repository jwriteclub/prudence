//go:build prudence_phase2_adapter

// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	"bytes"
	stdcontext "context"
	"fmt"
	"testing"

	"github.com/jwriteclub/prudence/internal/modulegraph"
)

func acquireAbortGraph(t *testing.T, ctx stdcontext.Context, root, lib string) *modulegraph.Acquisition {
	t.Helper()
	a, err := modulegraph.AcquireFlatExternal(ctx, modulegraph.Input{Path: "Root", Text: []byte(root)}, func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		return modulegraph.Input{Path: n, Text: []byte(lib)}, nil
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// The flat driver returns the Generator.java:443 abort unwrapped; its log is
// the observed log. Complete stays false (the old adapter gate), Aborted is
// the source-modeled SANY.java:503 boundary, and the graph stays unavailable.
func TestPhase2FlatAbortObservation(t *testing.T) {
	ctx := stdcontext.Background()
	a := acquireAbortGraph(t, ctx, "---- MODULE Root ----\nEXTENDS Lib\nUSE Expr!⟩\nB == Missing\n====\n", "---- MODULE Lib ----\nOp == Missing\n====\n")
	got := Phase2ObserveFlat(ctx, a, 1000)
	abort, ok := got.Err.(*Abort)
	if got.Complete || !got.Aborted || !ok || len(got.Details) != 2 || !sameDetail(got.Details[1], abort.Detail) || got.Details[0].Location.Name != "Lib" {
		t.Fatalf("%+v", got)
	}
	var out bytes.Buffer
	if err := got.WriteXML(ctx, &out, 10000, t.TempDir(), 1<<20); err != got.Err || out.Len() != 0 {
		t.Fatalf("graph after abort: %v %d", err, out.Len())
	}
}

// Only the unwrapped abort whose log equals the observed log is admitted.
func TestSourceAbortRejectsOtherFailures(t *testing.T) {
	code, ok := CodeFromValue(4003)
	if !ok {
		t.Fatal("code")
	}
	loc := Location{Name: "M", NamePresent: true, BeginLine: 1, BeginColumn: 1, EndLine: 1, EndColumn: 1}
	log := &Errors{}
	abort, err := log.Add(code, &loc, "Unexpected token found.")
	if err != nil {
		t.Fatal(err)
	}
	details := log.Messages()
	if !sourceAbort(abort, details) {
		t.Fatal("modeled abort rejected")
	}
	other := &Errors{}
	foreign, _ := other.Add(code, &loc, "Was expecting a GeneralId.")
	duplicate, _ := log.Add(code, &loc, "Unexpected token found.")
	for name, c := range map[string]struct {
		err     error
		details []Detail
	}{
		"port incomplete":   {ErrPortIncomplete, details},
		"cancelled":         {stdcontext.Canceled, details},
		"wrapped":           {fmt.Errorf("x: %w", abort), details},
		"nil abort":         {(*Abort)(nil), details},
		"foreign log":       {foreign, details},
		"truncated log":     {abort, nil},
		"record not in log": {&Abort{Detail: foreign.Detail, Source: log}, details},
	} {
		if sourceAbort(c.err, c.details) {
			t.Fatalf("%s admitted", name)
		}
	}
	// Errors.addMessage returns a fresh abort for an already-logged record.
	if !sourceAbort(duplicate, log.Messages()) {
		t.Fatal("duplicate abort rejected")
	}
	cancelled, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	if got := Phase2ObserveFlat(cancelled, acquireAbortGraph(t, stdcontext.Background(), "---- MODULE Root ----\nUSE Expr!⟩\n====\n", ""), 1000); got.Aborted || got.Complete {
		t.Fatalf("cancelled run admitted: %+v", got)
	}
}
