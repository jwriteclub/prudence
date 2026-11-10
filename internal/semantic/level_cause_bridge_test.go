//go:build prudence_phase2_adapter

// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestLevelCandidateBridgeCarriesCauseWithoutChangingFlags(t *testing.T) {
	for _, c := range causeControls {
		if c.nilCtx || c.cancelled {
			continue
		}
		o := observeCauseControl(t, c)
		var r LevelCandidate
		if c.nilAcq {
			r = Phase2ObserveLevelCandidate(stdcontext.Background(), nil, c.depth)
		} else {
			r = Phase2ObserveLevelCandidate(stdcontext.Background(), causeAcquire(t, c.root), c.depth)
		}
		if r.Examined != o.examined || r.Finished != o.finished || r.Bound != o.bound || r.LevelComplete != o.levelComplete || r.GenerationComplete != o.generationComplete || len(r.Attempts) != len(o.attempts) || len(r.Generation) != len(o.generation) || len(r.Level) != len(o.level) {
			t.Fatalf("%s: bridge flags %+v", c.name, r)
		}
		k := o.cause
		want := LevelCandidateCause{Exit: k.exit, DriverProfile: k.driverProfile, DriverProfileRejection: k.driverRejection, HistoryState: k.historyState, HistoryFirstInvalidation: k.historyFirst, HistoryInvalidationPhase: k.historyPhase, HistoryInvalidationUnit: k.historyUnit, CompletionLoss: k.completion, CompletionLossUnit: k.completionUnit, AttemptBoundaries: append([]string{}, k.boundaries...), IncompleteOrigin: k.incomplete, IncompleteCategory: k.incompleteCategory, AttemptIncompleteOrigins: append([]string{}, k.attemptIncomplete...)}
		if !reflect.DeepEqual(r.Cause, want) {
			t.Fatalf("%s: bridge cause %+v want %+v", c.name, r.Cause, want)
		}
		// The bridge slice is owned by the result, never aliased to the call.
		if len(r.Cause.AttemptBoundaries) > 0 {
			r.Cause.AttemptBoundaries[0] = "mutated"
			if again := Phase2ObserveLevelCandidate(stdcontext.Background(), causeAcquire(t, c.root), c.depth); again.Cause.AttemptBoundaries[0] == "mutated" {
				t.Fatalf("%s: aliased boundaries", c.name)
			}
			r.Cause.AttemptIncompleteOrigins[0] = "mutated"
			if again := Phase2ObserveLevelCandidate(stdcontext.Background(), causeAcquire(t, c.root), c.depth); again.Cause.AttemptIncompleteOrigins[0] == "mutated" {
				t.Fatalf("%s: aliased incomplete origins", c.name)
			}
		}
		b, err := json.Marshal(r.Cause)
		var keys map[string]any
		if err != nil || json.Unmarshal(b, &keys) != nil || len(keys) != 13 || len(b) > 4096 {
			t.Fatalf("%s: cause JSON %s %v", c.name, b, err)
		}
	}
}
