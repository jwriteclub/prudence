// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	"errors"
	"fmt"
	"testing"
)

// siteControls end in, or pass through, representative incomplete origins.
// They use the causeControl acquisition (dependencies A, B and L).
var siteControls = []causeControl{
	// Rejected driver profile: a later unit's formal parameter needs an
	// enabled-entry identity that the unprofiled history cannot supply. The
	// dependency N has an inner module, so even the general driver rejects
	// it (G2); the earlier mixed root "EXTENDS B\nI == INSTANCE A" is now
	// admitted by the general driver.
	{name: "identity-unprofiled", root: "EXTENDS B, N\nF(p) == p", depth: 1000},
	// HIDE DEF invalidates the history without a diagnostic, so the level
	// check is entered and the formal parameter's identity is unavailable.
	{name: "identity-inexact", root: "CONSTANT c\nH(p) == p\nHIDE DEF H", depth: 1000},
	// A LET RECURSIVE declaration whose definition is in a nested LET.
	{name: "generator-recursive", root: "A ==\n  LET RECURSIVE f IN\n  LET f(x) == x IN\n  TRUE", depth: 1000},
	// The same origin below further LET and definition frames.
	{name: "generator-recursive-nested", root: "A == LET B == LET RECURSIVE f IN LET f(x) == x IN TRUE IN B\nC == A", depth: 1000},
	// An instance selector whose name resumes after a structural selector
	// (I!@!Foo). I!@ itself now reports the unknown compound I
	// (Generator.java:732-813; root clean_rejection_test.go).
	{name: "generator-qualified-selector-prefix", root: "---- MODULE In ----\nFoo == 1\n====\nI == INSTANCE In\nX == I!@!Foo", depth: 1000},
	// The structural LET attempt is consumed, then the numeric selector's
	// free-function index parser stops on an out-of-range literal.
	{name: "unknown-structural-index", root: "F == <<1>>\nG == F!99999999999", depth: 1000},
	{name: "unknown-structural-index-let", root: "F == LET a == 1 IN a\nG == F!99999999999", depth: 1000},
}

// siteErrorIdentity prints the error relations callers may depend on.
func siteErrorIdentity(err error) string {
	var depth *DepthLimitError
	return fmt.Sprintf("nil=%t same=%t is=%t depthAs=%t", err == nil, err == ErrPortIncomplete, errors.Is(err, ErrPortIncomplete), errors.As(err, &depth))
}

func siteOutputSummary(o levelObservation) string {
	s := levelOutputSummary(o) + " error{" + siteErrorIdentity(o.err) + "} attemptErrors{"
	for _, a := range o.attempts {
		s += "[" + siteErrorIdentity(a.err) + "]"
	}
	return s + "}"
}

// Recorded by running this file unchanged against an unmodified export of
// base 56f4121, before origin telemetry existed. They must not change,
// except constructor-operator and identity-inexact, re-recorded when their
// unresolved-operator body and module-level HIDE were admitted to history
// (resolution_history.go): both now bind. rejected-mixed, rejected-nested and
// rejected-repeated were re-recorded when the general-driver profile admitted
// them (general_driver_profile.go): each now binds. identity-unprofiled keeps
// its recorded output on a root the general driver still rejects.
var siteBaseline = map[string]string{
	"exact-empty":                         `err="" examined=true finished=true bound=true level=true generation=true attempts=[EC-K ""] generationCodes=[] levelCodes=[] error{nil=true same=false is=false depthAs=false} attemptErrors{[nil=true same=false is=false depthAs=false]}`,
	"exact-two-units":                     `err="" examined=true finished=true bound=true level=true generation=true attempts=[EC-K ""][EC-K ""] generationCodes=[] levelCodes=[] error{nil=true same=false is=false depthAs=false} attemptErrors{[nil=true same=false is=false depthAs=false][nil=true same=false is=false depthAs=false]}`,
	"exact-flat-extends":                  `err="" examined=true finished=true bound=true level=true generation=true attempts=[EC-K ""][EC-K ""][EC-K ""] generationCodes=[] levelCodes=[] error{nil=true same=false is=false depthAs=false} attemptErrors{[nil=true same=false is=false depthAs=false][nil=true same=false is=false depthAs=false][nil=true same=false is=false depthAs=false]}`,
	"exact-generation-diagnostic":         `err="" examined=true finished=true bound=true level=true generation=true attempts=[--S- ""] generationCodes=[4204] levelCodes=[] error{nil=true same=false is=false depthAs=false} attemptErrors{[nil=true same=false is=false depthAs=false]}`,
	"constructor-selector":                `err="" examined=true finished=true bound=false level=false generation=false attempts=[--S- ""] generationCodes=[4005] levelCodes=[] error{nil=true same=false is=false depthAs=false} attemptErrors{[nil=true same=false is=false depthAs=false]}`,
	"constructor-operator":                `err="" examined=true finished=true bound=true level=true generation=true attempts=[--S- ""] generationCodes=[4004 4004 4204] levelCodes=[] error{nil=true same=false is=false depthAs=false} attemptErrors{[nil=true same=false is=false depthAs=false]}`,
	"rejected-mixed":                      `err="" examined=true finished=true bound=true level=true generation=true attempts=[EC-K ""][EC-K ""][EC-K ""] generationCodes=[] levelCodes=[] error{nil=true same=false is=false depthAs=false} attemptErrors{[nil=true same=false is=false depthAs=false][nil=true same=false is=false depthAs=false][nil=true same=false is=false depthAs=false]}`,
	"rejected-nested":                     `err="" examined=true finished=true bound=true level=true generation=true attempts=[EC-K ""][EC-K ""] generationCodes=[] levelCodes=[] error{nil=true same=false is=false depthAs=false} attemptErrors{[nil=true same=false is=false depthAs=false][nil=true same=false is=false depthAs=false]}`,
	"rejected-repeated":                   `err="" examined=true finished=true bound=true level=true generation=true attempts=[EC-K ""][EC-K ""] generationCodes=[] levelCodes=[] error{nil=true same=false is=false depthAs=false} attemptErrors{[nil=true same=false is=false depthAs=false][nil=true same=false is=false depthAs=false]}`,
	"rejected-dependency-nesting":         `err="" examined=true finished=true bound=false level=false generation=false attempts=[EC-K ""][EC-K ""] generationCodes=[] levelCodes=[] error{nil=true same=false is=false depthAs=false} attemptErrors{[nil=true same=false is=false depthAs=false][nil=true same=false is=false depthAs=false]}`,
	"level-boundary":                      `err="" examined=true finished=true bound=false level=true generation=true attempts=[EC-K ""] generationCodes=[] levelCodes=[4206] error{nil=true same=false is=false depthAs=false} attemptErrors{[nil=true same=false is=false depthAs=false]}`,
	"prior-level-boundary":                `err="" examined=true finished=true bound=false level=false generation=false attempts=[EC-K ""][--S- ""] generationCodes=[] levelCodes=[4206] error{nil=true same=false is=false depthAs=false} attemptErrors{[nil=true same=false is=false depthAs=false][nil=true same=false is=false depthAs=false]}`,
	"depth-generation":                    `err="semantic recursion depth limit exceeded" examined=true finished=false bound=false level=false generation=false attempts= generationCodes=[] levelCodes=[] error{nil=false same=false is=true depthAs=true} attemptErrors{}`,
	"depth-level-check":                   `err="semantic port incomplete" examined=true finished=false bound=false level=false generation=false attempts=[E--- "semantic port incomplete"] generationCodes=[] levelCodes=[] error{nil=false same=true is=true depthAs=false} attemptErrors{[nil=false same=true is=true depthAs=false]}`,
	"cancelled-before-start":              `err="context canceled" examined=false finished=false bound=false level=false generation=false attempts= generationCodes=[] levelCodes=[] error{nil=false same=false is=false depthAs=false} attemptErrors{}`,
	"nil-acquisition":                     `err="semantic port incomplete" examined=false finished=false bound=false level=false generation=false attempts= generationCodes=[] levelCodes=[] error{nil=false same=true is=true depthAs=false} attemptErrors{}`,
	"invalid-request":                     `err="semantic port incomplete" examined=false finished=false bound=false level=false generation=false attempts= generationCodes=[] levelCodes=[] error{nil=false same=true is=true depthAs=false} attemptErrors{}`,
	"identity-unprofiled":                 `err="semantic port incomplete" examined=true finished=false bound=false level=false generation=false attempts=[EC-K ""][EC-K ""][E--- "semantic port incomplete"] generationCodes=[] levelCodes=[] error{nil=false same=true is=true depthAs=false} attemptErrors{[nil=true same=false is=false depthAs=false][nil=true same=false is=false depthAs=false][nil=false same=true is=true depthAs=false]}`,
	"identity-inexact":                    `err="" examined=true finished=true bound=true level=true generation=true attempts=[EC-K ""] generationCodes=[] levelCodes=[] error{nil=true same=false is=false depthAs=false} attemptErrors{[nil=true same=false is=false depthAs=false]}`,
	"generator-recursive":                 `err="semantic port incomplete" examined=true finished=false bound=false level=false generation=false attempts= generationCodes=[4291 4292] levelCodes=[] error{nil=false same=true is=true depthAs=false} attemptErrors{}`,
	"generator-recursive-nested":          `err="semantic port incomplete" examined=true finished=false bound=false level=false generation=false attempts= generationCodes=[4291 4292] levelCodes=[] error{nil=false same=true is=true depthAs=false} attemptErrors{}`,
	"generator-qualified-selector-prefix": `err="semantic port incomplete" examined=true finished=false bound=false level=false generation=false attempts= generationCodes=[] levelCodes=[] error{nil=false same=true is=true depthAs=false} attemptErrors{}`,
	"unknown-structural-index":            `err="semantic port incomplete" examined=true finished=false bound=false level=false generation=false attempts= generationCodes=[] levelCodes=[] error{nil=false same=true is=true depthAs=false} attemptErrors{}`,
	"unknown-structural-index-let":        `err="semantic port incomplete" examined=true finished=false bound=false level=false generation=false attempts= generationCodes=[] levelCodes=[] error{nil=false same=true is=true depthAs=false} attemptErrors{}`,
}

func TestIncompleteSiteControlsKeepBaselineOutputs(t *testing.T) {
	for _, c := range append(append([]causeControl{}, causeControls...), siteControls...) {
		got := siteOutputSummary(observeCauseControl(t, c))
		if want, ok := siteBaseline[c.name]; !ok || got != want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, want)
		}
	}
	if len(siteBaseline) != len(causeControls)+len(siteControls) {
		t.Fatalf("baseline=%d controls=%d", len(siteBaseline), len(causeControls)+len(siteControls))
	}
}
