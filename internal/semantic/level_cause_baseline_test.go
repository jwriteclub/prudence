// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"fmt"
	"strings"
	"testing"

	"github.com/jwriteclub/prudence/internal/modulegraph"
)

// causeControl is one source-derived level-candidate execution. The same
// table drives the unchanged-output baseline below and the cause telemetry
// expectations, so both describe exactly the same executions.
type causeControl struct {
	name, root string
	depth      int
	nilCtx     bool // invalid request
	nilAcq     bool // acquisition absent before profile selection
	cancelled  bool // context cancelled before the driver starts
}

var causeControls = []causeControl{
	{name: "exact-empty", root: "A == 1", depth: 1000},
	{name: "exact-two-units", root: "EXTENDS A\nB == <<X, X>>", depth: 1000},
	{name: "exact-flat-extends", root: "EXTENDS A, B\nC == <<X, V>>", depth: 1000},
	{name: "exact-generation-diagnostic", root: "THEOREM TRUE\n<1>1. USE MODULE M\n<1>2. <1>1(123)\n<1> QED OBVIOUS\nAfter == TRUE", depth: 1000},
	{name: "constructor-selector", root: "F == <<TRUE,FALSE>>\nG == F!1!Missing", depth: 1000},
	{name: "constructor-operator", root: "F(x) == x + 1\nG == F!:", depth: 1000},
	{name: "rejected-mixed", root: "EXTENDS B\nI == INSTANCE A", depth: 1000},
	{name: "rejected-nested", root: "---- MODULE Inner ----\nEXTENDS A\n====", depth: 1000},
	{name: "rejected-repeated", root: "EXTENDS A, A\nB == X", depth: 1000},
	// A dependency with an inner module defeats all five profiles, the
	// general driver at G2.
	{name: "rejected-dependency-nesting", root: "EXTENDS N\nB == TRUE", depth: 1000},
	{name: "level-boundary", root: "VARIABLE x\nASSUME x", depth: 1000},
	{name: "prior-level-boundary", root: "EXTENDS L\nB == TRUE", depth: 1000},
	{name: "depth-generation", root: "A == 1", depth: 1},
	{name: "depth-level-check", root: "A == 1", depth: 2},
	{name: "cancelled-before-start", root: "A == 1", depth: 1000, cancelled: true},
	{name: "nil-acquisition", depth: 1000, nilAcq: true},
	{name: "invalid-request", root: "A == 1", depth: 1000, nilCtx: true},
}

func causeInput(name, body string) modulegraph.Input {
	return modulegraph.Input{Path: name + ".tla", Origin: modulegraph.OriginWorkspace, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
}

func causeAcquire(t *testing.T, root string) *modulegraph.Acquisition {
	t.Helper()
	deps := map[string]string{"A": "X == 1", "B": "V == 2", "L": "VARIABLE y\nASSUME y", "N": "---- MODULE NI ----\nW == 3\n====\nU == 4"}
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), causeInput("M", root), func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		if b, ok := deps[n]; ok {
			return causeInput(n, b), nil
		}
		return modulegraph.Input{}, ErrPortIncomplete
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func observeCauseControl(t *testing.T, c causeControl) levelObservation {
	t.Helper()
	var a *modulegraph.Acquisition
	if !c.nilAcq {
		a = causeAcquire(t, c.root)
	}
	ctx := stdcontext.Background()
	if c.cancelled {
		cancelled, cancel := stdcontext.WithCancel(ctx)
		cancel()
		ctx = cancelled
	}
	if c.nilCtx {
		ctx = nil
	}
	return observeLevelCandidate(ctx, a, c.depth)
}

// levelOutputSummary prints only outputs that existed before cause telemetry.
func levelOutputSummary(o levelObservation) string {
	var b strings.Builder
	err := ""
	if o.err != nil {
		err = o.err.Error()
	}
	fmt.Fprintf(&b, "err=%q examined=%t finished=%t bound=%t level=%t generation=%t attempts=", err, o.examined, o.finished, o.bound, o.levelComplete, o.generationComplete)
	for _, a := range o.attempts {
		flag := func(v bool, c byte) byte {
			if v {
				return c
			}
			return '-'
		}
		ae := ""
		if a.err != nil {
			ae = a.err.Error()
		}
		fmt.Fprintf(&b, "[%c%c%c%c %q]", flag(a.entered, 'E'), flag(a.completed, 'C'), flag(a.skipped, 'S'), flag(a.correct, 'K'), ae)
	}
	codes := func(ds []Detail) []int {
		out := []int{}
		for _, d := range ds {
			out = append(out, d.Code.Value)
		}
		return out
	}
	fmt.Fprintf(&b, " generationCodes=%v levelCodes=%v", codes(o.generation), codes(o.level))
	return b.String()
}

// The expectations were recorded by running this file unchanged against base
// c4e7819, before cause telemetry existed. They must not change.
// constructor-operator binds since its unresolved infix body was admitted to
// history (resolution_history.go); its generation codes are unchanged.
// rejected-mixed, rejected-nested and rejected-repeated were re-recorded when
// the general-driver profile admitted them (general_driver_profile.go): each
// now binds with an exact history and the same attempts and codes.
var causeBaseline = map[string]string{
	"exact-flat-extends":          `err="" examined=true finished=true bound=true level=true generation=true attempts=[EC-K ""][EC-K ""][EC-K ""] generationCodes=[] levelCodes=[]`,
	"exact-empty":                 `err="" examined=true finished=true bound=true level=true generation=true attempts=[EC-K ""] generationCodes=[] levelCodes=[]`,
	"exact-two-units":             `err="" examined=true finished=true bound=true level=true generation=true attempts=[EC-K ""][EC-K ""] generationCodes=[] levelCodes=[]`,
	"exact-generation-diagnostic": `err="" examined=true finished=true bound=true level=true generation=true attempts=[--S- ""] generationCodes=[4204] levelCodes=[]`,
	"constructor-selector":        `err="" examined=true finished=true bound=false level=false generation=false attempts=[--S- ""] generationCodes=[4005] levelCodes=[]`,
	"constructor-operator":        `err="" examined=true finished=true bound=true level=true generation=true attempts=[--S- ""] generationCodes=[4004 4004 4204] levelCodes=[]`,
	"rejected-mixed":              `err="" examined=true finished=true bound=true level=true generation=true attempts=[EC-K ""][EC-K ""][EC-K ""] generationCodes=[] levelCodes=[]`,
	"rejected-nested":             `err="" examined=true finished=true bound=true level=true generation=true attempts=[EC-K ""][EC-K ""] generationCodes=[] levelCodes=[]`,
	"rejected-repeated":           `err="" examined=true finished=true bound=true level=true generation=true attempts=[EC-K ""][EC-K ""] generationCodes=[] levelCodes=[]`,
	"rejected-dependency-nesting": `err="" examined=true finished=true bound=false level=false generation=false attempts=[EC-K ""][EC-K ""] generationCodes=[] levelCodes=[]`,
	"level-boundary":              `err="" examined=true finished=true bound=false level=true generation=true attempts=[EC-K ""] generationCodes=[] levelCodes=[4206]`,
	"prior-level-boundary":        `err="" examined=true finished=true bound=false level=false generation=false attempts=[EC-K ""][--S- ""] generationCodes=[] levelCodes=[4206]`,
	"depth-generation":            `err="semantic recursion depth limit exceeded" examined=true finished=false bound=false level=false generation=false attempts= generationCodes=[] levelCodes=[]`,
	"depth-level-check":           `err="semantic port incomplete" examined=true finished=false bound=false level=false generation=false attempts=[E--- "semantic port incomplete"] generationCodes=[] levelCodes=[]`,
	"cancelled-before-start":      `err="context canceled" examined=false finished=false bound=false level=false generation=false attempts= generationCodes=[] levelCodes=[]`,
	"nil-acquisition":             `err="semantic port incomplete" examined=false finished=false bound=false level=false generation=false attempts= generationCodes=[] levelCodes=[]`,
	"invalid-request":             `err="semantic port incomplete" examined=false finished=false bound=false level=false generation=false attempts= generationCodes=[] levelCodes=[]`,
}

func TestLevelCauseControlsKeepBaselineOutputs(t *testing.T) {
	for _, c := range causeControls {
		got := levelOutputSummary(observeCauseControl(t, c))
		if want, ok := causeBaseline[c.name]; !ok || got != want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, want)
		}
	}
}
