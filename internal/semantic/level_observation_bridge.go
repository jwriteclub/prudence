//go:build prudence_phase2_adapter

// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"github.com/jwriteclub/prudence/internal/modulegraph"
)

// LevelCandidateAttempt reports actual dispatch, independently of its result.
// Correct is meaningful only when Completed is true.
type LevelCandidateAttempt struct {
	Entered, Completed, Skipped, Correct bool
	ModuleName                           string
	ModuleNameKnown                      bool
	Error                                string
}

// LevelCandidate retains accepted first-winner records in their original stage.
// Finished describes the modeled execution; it is not a parity claim.
type LevelCandidate struct {
	Generation, Level                                            []Detail
	Attempts                                                     []LevelCandidateAttempt
	Examined, Finished, Bound, LevelComplete, GenerationComplete bool
	Error                                                        string
	IdentityBinding                                              string
	Cause                                                        LevelCandidateCause
}

// LevelCandidateCause is private, bounded oracle telemetry owned by one call.
// It explains the flags above and never changes them. Every string is a fixed
// source constant, a recorded first history reason of at most 128 bytes, a
// fixed composition of those, or "unknown" where the implementation cannot
// establish the cause. AttemptBoundaries has one entry per attempt.
type LevelCandidateCause struct {
	Exit                     string   `json:"exit"`
	DriverProfile            string   `json:"driver_profile"`
	DriverProfileRejection   string   `json:"driver_profile_rejection"`
	HistoryState             string   `json:"history_state"`
	HistoryFirstInvalidation string   `json:"history_first_invalidation"`
	HistoryInvalidationPhase string   `json:"history_invalidation_phase"`
	HistoryInvalidationUnit  int      `json:"history_invalidation_unit"`
	CompletionLoss           string   `json:"completion_loss"`
	CompletionLossUnit       int      `json:"completion_loss_unit"`
	AttemptBoundaries        []string `json:"attempt_boundaries"`
	// IncompleteOrigin names where a final "semantic port incomplete" error
	// was created: a fixed "<stage>.<category>.<detail>" site, "unknown" when
	// no attributable origin was recorded, or "not-applicable" when Error is
	// not that exact error. AttemptIncompleteOrigins does the same for each
	// attempt's Error, one entry per attempt.
	IncompleteOrigin         string   `json:"incomplete_origin"`
	IncompleteCategory       string   `json:"incomplete_category"`
	AttemptIncompleteOrigins []string `json:"attempt_incomplete_origins"`
}

func Phase2ObserveLevelCandidate(ctx stdcontext.Context, acquisition *modulegraph.Acquisition, maxDepth int) LevelCandidate {
	o := observeLevelCandidate(ctx, acquisition, maxDepth)
	r := LevelCandidate{Generation: o.generation, Level: o.level, Examined: o.examined, Finished: o.finished, Bound: o.bound, LevelComplete: o.levelComplete, GenerationComplete: o.generationComplete, IdentityBinding: "source-modeled enabled-entry semantic UID hashes; runtime mapping unverified"}
	if o.err != nil {
		r.Error = o.err.Error()
	}
	k := o.cause
	r.Cause = LevelCandidateCause{Exit: k.exit, DriverProfile: k.driverProfile, DriverProfileRejection: k.driverRejection, HistoryState: k.historyState, HistoryFirstInvalidation: k.historyFirst, HistoryInvalidationPhase: k.historyPhase, HistoryInvalidationUnit: k.historyUnit, CompletionLoss: k.completion, CompletionLossUnit: k.completionUnit, AttemptBoundaries: append([]string{}, k.boundaries...), IncompleteOrigin: k.incomplete, IncompleteCategory: k.incompleteCategory, AttemptIncompleteOrigins: append([]string{}, k.attemptIncomplete...)}
	for _, a := range o.attempts {
		v := LevelCandidateAttempt{Entered: a.entered, Completed: a.completed, Skipped: a.skipped, Correct: a.correct}
		if a.module != nil && a.module.name != nil && a.module.name.displayExact {
			v.ModuleName = a.module.name.text
			v.ModuleNameKnown = true
		}
		if a.err != nil {
			v.Error = a.err.Error()
		}
		r.Attempts = append(r.Attempts, v)
	}
	return r
}
