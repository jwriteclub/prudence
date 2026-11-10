// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

const historyBoundary = "unsupported or non-successful level boundary"

// Expected private cause telemetry for each source-derived control. The
// strings are the actual recorded first reasons, not names inferred from the
// control's purpose. The three rejected-* graphs without a dependency inner
// module are admitted by the general driver; their names are kept so the
// baselines stay keyed by the same executions.
var causeExpectations = map[string]levelCause{
	// Two distinct EXTENDS defeat only the linear profile.
	"exact-flat-extends":          {exit: "completed", driverProfile: "flat-extends", driverRejection: "none", historyState: "exact", historyFirst: "none", historyPhase: "none", historyUnit: -1, completion: "none", completionUnit: -1, boundaries: []string{"supported", "supported", "supported"}, incomplete: "not-applicable", incompleteCategory: "not-applicable", attemptIncomplete: []string{"not-applicable", "not-applicable", "not-applicable"}},
	"exact-empty":                 {exit: "completed", driverProfile: "linear", driverRejection: "none", historyState: "exact", historyFirst: "none", historyPhase: "none", historyUnit: -1, completion: "none", completionUnit: -1, boundaries: []string{"supported"}, incomplete: "not-applicable", incompleteCategory: "not-applicable", attemptIncomplete: []string{"not-applicable"}},
	"exact-two-units":             {exit: "completed", driverProfile: "linear", driverRejection: "none", historyState: "exact", historyFirst: "none", historyPhase: "none", historyUnit: -1, completion: "none", completionUnit: -1, boundaries: []string{"supported", "supported"}, incomplete: "not-applicable", incompleteCategory: "not-applicable", attemptIncomplete: []string{"not-applicable", "not-applicable"}},
	"exact-generation-diagnostic": {exit: "completed", driverProfile: "linear", driverRejection: "none", historyState: "exact", historyFirst: "none", historyPhase: "none", historyUnit: -1, completion: "none", completionUnit: -1, boundaries: []string{"skipped"}, incomplete: "not-applicable", incompleteCategory: "not-applicable", attemptIncomplete: []string{"not-applicable"}},
	"constructor-selector":        {exit: "completed", driverProfile: "linear", driverRejection: "none", historyState: "invalidated", historyFirst: "unproved general identifier selector", historyPhase: "generation", historyUnit: 0, completion: "history invalidated before publication", completionUnit: 0, boundaries: []string{"skipped"}, incomplete: "not-applicable", incompleteCategory: "not-applicable", attemptIncomplete: []string{"not-applicable"}},
	"constructor-operator":        {exit: "completed", driverProfile: "linear", driverRejection: "none", historyState: "exact", historyFirst: "none", historyPhase: "none", historyUnit: -1, completion: "none", completionUnit: -1, boundaries: []string{"skipped"}, incomplete: "not-applicable", incompleteCategory: "not-applicable", attemptIncomplete: []string{"not-applicable"}},
	"rejected-mixed":              {exit: "completed", driverProfile: generalDriverProfile, driverRejection: "none", historyState: "exact", historyFirst: "none", historyPhase: "none", historyUnit: -1, completion: "none", completionUnit: -1, boundaries: []string{"supported", "supported", "supported"}, incomplete: "not-applicable", incompleteCategory: "not-applicable", attemptIncomplete: []string{"not-applicable", "not-applicable", "not-applicable"}},
	"rejected-nested":             {exit: "completed", driverProfile: generalDriverProfile, driverRejection: "none", historyState: "exact", historyFirst: "none", historyPhase: "none", historyUnit: -1, completion: "none", completionUnit: -1, boundaries: []string{"supported", "supported"}, incomplete: "not-applicable", incompleteCategory: "not-applicable", attemptIncomplete: []string{"not-applicable", "not-applicable"}},
	"rejected-repeated":           {exit: "completed", driverProfile: generalDriverProfile, driverRejection: "none", historyState: "exact", historyFirst: "none", historyPhase: "none", historyUnit: -1, completion: "none", completionUnit: -1, boundaries: []string{"supported", "supported"}, incomplete: "not-applicable", incompleteCategory: "not-applicable", attemptIncomplete: []string{"not-applicable", "not-applicable"}},
	// The fifth clause: a dependency with an inner module stays outside D_G.
	"rejected-dependency-nesting": {exit: "completed", driverProfile: "rejected", driverRejection: "linear: nested module definitions in multi-unit graph; flat-extends: nested module definitions; standard-local-instance: nested module definitions; root-nested: nested module definitions in dependency; general-driver: nested module definitions in dependency", historyState: "unprofiled", historyFirst: "not-applicable", historyPhase: "not-applicable", historyUnit: -1, completion: "no history profile at publication", completionUnit: 0, boundaries: []string{"supported", "supported"}, incomplete: "not-applicable", incompleteCategory: "not-applicable", attemptIncomplete: []string{"not-applicable", "not-applicable"}},
	"level-boundary":              {exit: "completed", driverProfile: "linear", driverRejection: "none", historyState: "invalidated", historyFirst: historyBoundary, historyPhase: "level-boundary", historyUnit: 0, completion: "none", completionUnit: -1, boundaries: []string{"level diagnostics added,errors unsuccessful"}, incomplete: "not-applicable", incompleteCategory: "not-applicable", attemptIncomplete: []string{"not-applicable"}},
	"prior-level-boundary":        {exit: "completed", driverProfile: "linear", driverRejection: "none", historyState: "invalidated", historyFirst: historyBoundary, historyPhase: "level-boundary", historyUnit: 0, completion: "prior entered level boundary unsupported", completionUnit: 1, boundaries: []string{"level diagnostics added,errors unsuccessful", "skipped"}, incomplete: "not-applicable", incompleteCategory: "not-applicable", attemptIncomplete: []string{"not-applicable", "not-applicable"}},
	"depth-generation":            {exit: "generation", driverProfile: "linear", driverRejection: "none", historyState: "invalidated", historyFirst: "depth limit", historyPhase: "generation", historyUnit: 0, completion: "execution unfinished", completionUnit: -1, incomplete: "not-applicable", incompleteCategory: "not-applicable"},
	"depth-level-check":           {exit: "level check", driverProfile: "linear", driverRejection: "none", historyState: "invalidated", historyFirst: historyBoundary, historyPhase: "level-boundary", historyUnit: 0, completion: "execution unfinished", completionUnit: -1, boundaries: []string{"level check error"}, incomplete: "level.resource.depth-limit", incompleteCategory: "resource", attemptIncomplete: []string{"level.resource.depth-limit"}},
	"cancelled-before-start":      {exit: "dependency order", driverProfile: "unknown", driverRejection: "unknown", historyState: "unknown", historyFirst: "unknown", historyPhase: "unknown", historyUnit: -1, completion: "execution unfinished", completionUnit: -1, incomplete: "not-applicable", incompleteCategory: "not-applicable"},
	"nil-acquisition":             {exit: "invalid acquisition", driverProfile: "unknown", driverRejection: "unknown", historyState: "unknown", historyFirst: "unknown", historyPhase: "unknown", historyUnit: -1, completion: "execution unfinished", completionUnit: -1, incomplete: "driver.precondition.no-acquisition", incompleteCategory: "precondition"},
	"invalid-request":             {exit: "invalid request", driverProfile: "unknown", driverRejection: "unknown", historyState: "unknown", historyFirst: "unknown", historyPhase: "unknown", historyUnit: -1, completion: "execution unfinished", completionUnit: -1, incomplete: "level.precondition.invalid-request", incompleteCategory: "precondition"},
}

// checkCauseInvariants ties the telemetry to the unchanged flags it explains.
func checkCauseInvariants(t *testing.T, label string, o levelObservation) {
	t.Helper()
	c := o.cause
	fail := func(format string, args ...any) {
		t.Helper()
		t.Errorf("%s: "+format+" (cause %+v)", append(append([]any{label}, args...), c)...)
	}
	for _, s := range []string{c.exit, c.driverProfile, c.historyState, c.historyFirst, c.historyPhase, c.completion} {
		if s == "" || len(s) > maxCauseBytes {
			fail("unbounded or empty telemetry %q", s)
		}
	}
	// Five clauses, each a bounded cause or the composed reduced-view clause.
	if c.driverRejection == "" || len(c.driverRejection) > 8*maxCauseBytes+128 {
		fail("unbounded rejection")
	}
	if o.finished != (c.exit == "completed") {
		fail("finished=%t", o.finished)
	}
	if o.bound != (o.finished && c.historyState == "exact") {
		fail("bound=%t", o.bound)
	}
	if o.levelComplete != (c.completion == "none") || o.generationComplete != o.levelComplete {
		fail("level=%t generation=%t", o.levelComplete, o.generationComplete)
	}
	if o.finished && c.completion == "execution unfinished" {
		fail("finished execution reported unfinished")
	}
	if (c.completionUnit < 0) != (c.completion == "none" || c.completion == "execution unfinished") || c.completionUnit >= len(o.attempts)+1 {
		fail("completion unit")
	}
	exact := c.historyState == "exact"
	if exact != (c.historyFirst == "none") || exact != (c.historyPhase == "none") {
		fail("exact history must have no invalidation")
	}
	if (c.historyState == "unprofiled") != (c.driverProfile == "rejected") || (c.historyState == "unknown") != (c.driverProfile == "unknown") {
		fail("profile/history state mismatch")
	}
	if (c.driverRejection == "none") != (c.driverProfile == "linear" || c.driverProfile == "flat-extends" || c.driverProfile == standardLocalInstanceProfile || strings.HasPrefix(c.driverProfile, rootNestedProfile+"/") || c.driverProfile == generalDriverProfile) {
		fail("rejection text")
	}
	located := c.historyPhase == "generation" || c.historyPhase == "level-boundary"
	if located != (c.historyUnit >= 0) || c.historyUnit > len(o.attempts) {
		fail("history unit")
	}
	if len(c.boundaries) != len(o.attempts) {
		fail("boundaries=%d attempts=%d", len(c.boundaries), len(o.attempts))
		return
	}
	for i, a := range o.attempts {
		b := c.boundaries[i]
		if (b == "skipped") != a.skipped || (b == "supported" && !(a.entered && a.completed && a.correct)) || (a.err != nil) != strings.Contains(b, "level check error") || len(b) > maxCauseBytes {
			fail("attempt %d boundary %q", i, b)
		}
	}
}

func TestLevelCauseSourceControls(t *testing.T) {
	for _, c := range causeControls {
		o := observeCauseControl(t, c)
		checkCauseInvariants(t, c.name, o)
		want, ok := causeExpectations[c.name]
		if got := fmt.Sprintf("%+v", o.cause); !ok || got != fmt.Sprintf("%+v", want) {
			t.Errorf("%s:\n got %s\nwant %+v", c.name, got, want)
		}
	}
	if len(causeExpectations) != len(causeControls) {
		t.Fatalf("expectations=%d controls=%d", len(causeExpectations), len(causeControls))
	}
}

// countdownContext reports cancellation after a fixed number of Err calls, so
// cancellation reaches each driver step deterministically without timing.
type countdownContext struct {
	stdcontext.Context
	left *int
}

func (c countdownContext) Err() error {
	if *c.left <= 0 {
		return stdcontext.Canceled
	}
	*c.left--
	return nil
}

func TestLevelCauseCancellationChronology(t *testing.T) {
	// Values mark chronologies the sweep must reach; false ones are allowed.
	seen := map[string]bool{}
	allowed := map[string]bool{
		"dependency order|unknown|unknown":          true,
		"cancelled before unit|bootstrap|cancelled": true,
		// An early exit can retain an exact history; exit distinguishes it.
		"cancelled before unit|none|none": true,
		// A cancelled history event does not itself stop generation; the
		// later level check then stops on the same cancellation.
		"level check|generation|cancelled":              false,
		"generation|generation|cancelled":               true,
		"level check|level-boundary|" + historyBoundary: true,
		"completed|none|none":                           true,
	}
	for n := 0; n < 4000; n++ {
		left := n
		o := observeLevelCandidate(countdownContext{stdcontext.Background(), &left}, causeAcquire(t, "EXTENDS A\nB == <<X, X>>"), 1000)
		label := fmt.Sprintf("countdown %d", n)
		checkCauseInvariants(t, label, o)
		key := o.cause.exit + "|" + o.cause.historyPhase + "|" + o.cause.historyFirst
		if _, ok := allowed[key]; !ok {
			t.Fatalf("%s: unexpected chronology %s", label, key)
		}
		seen[key] = true
		if o.finished {
			if levelOutputSummary(o) != causeBaseline["exact-two-units"] {
				t.Fatalf("%s: finished output %s", label, levelOutputSummary(o))
			}
			break
		}
	}
	for key, want := range allowed {
		if want && !seen[key] {
			t.Errorf("cancellation never reached %s", key)
		}
	}
}

// The trace is write-only: identical driver results with or without it.
func TestFlatProfileTraceDoesNotChangeExecution(t *testing.T) {
	summary := func(details []Detail, examined bool, root *moduleNode, roots []*moduleNode, err error) string {
		s := fmt.Sprintf("examined=%t roots=%d err=%v codes=", examined, len(roots), err)
		for _, d := range details {
			s += fmt.Sprintf("%d,", d.Code.Value)
		}
		if root != nil {
			h := root.owner.history
			s += fmt.Sprintf(" root=%s profile=%d exact=%t count=%d first=%q", root.name.text, h.profile, h.exact, h.count, h.firstUnknown)
		}
		return s
	}
	for _, c := range causeControls {
		if c.nilAcq || c.nilCtx || c.cancelled {
			continue
		}
		for _, enabled := range []bool{false, true} {
			var trace flatProfileTrace
			plain := summary(analyzeFlatExternalProfile(stdcontext.Background(), causeAcquire(t, c.root), c.depth, enabled, nil, nil))
			traced := summary(analyzeFlatExternalProfile(stdcontext.Background(), causeAcquire(t, c.root), c.depth, enabled, nil, &trace))
			if plain != traced || !trace.decided || trace.exit == "" {
				t.Errorf("%s enabled=%t:\n plain %s\ntraced %s trace=%+v", c.name, enabled, plain, traced, trace)
			}
		}
	}
}

func TestLevelCauseIsBoundedAndCallOwned(t *testing.T) {
	if boundedCause(strings.Repeat("x", maxCauseBytes+1)) != "unknown" || boundedCause("") != "unknown" || boundedCause("depth limit") != "depth limit" {
		t.Fatal("boundedCause")
	}
	// Every history reason is a short source literal, so reported first
	// invalidations are bounded constants rather than source-derived text.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	literal := regexp.MustCompile(`history\.invalidate\(("[^"]*")\)|h\.invalidate\(("[^"]*")\)`)
	dynamic := regexp.MustCompile(`(history|h)\.invalidate\([^"]`)
	count := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if dynamic.Match(b) {
			t.Errorf("%s: non-literal history invalidation reason", e.Name())
		}
		for _, m := range literal.FindAllSubmatch(b, -1) {
			count++
			if r := string(m[1]) + string(m[2]); len(r)-2 > maxCauseBytes || len(r) <= 2 {
				t.Errorf("%s: reason %s exceeds bound", e.Name(), r)
			}
		}
	}
	if count < 40 {
		t.Fatalf("found only %d history reasons", count)
	}
	// Interleaved calls own independent telemetry.
	first := observeCauseControl(t, causeControls[3])
	second := observeCauseControl(t, causeControls[0])
	first.cause.boundaries[0] = "mutated"
	again := observeCauseControl(t, causeControls[0])
	if fmt.Sprintf("%+v", second.cause) != fmt.Sprintf("%+v", again.cause) || fmt.Sprintf("%+v", again.cause) != fmt.Sprintf("%+v", causeExpectations["exact-empty"]) {
		t.Fatalf("shared telemetry: %+v %+v", second.cause, again.cause)
	}
}
