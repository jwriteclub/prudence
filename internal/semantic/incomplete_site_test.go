// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/jwriteclub/prudence/internal/modulegraph"
)

type siteExpectation struct {
	incomplete, category string
	attempts             []string
}

const notApplicable = "not-applicable"

// Expected origins for siteControls. Every site is the recorded origin of
// the execution's final error, not a name inferred from the control's purpose.
// identity-inexact's module-level HIDE is now admitted history
// (resolution_history.go), so its history is exact and its level check
// completes.
var siteExpectations = map[string]siteExpectation{
	"identity-unprofiled":                 {"level.mapping.identity-unprofiled-history", "mapping", []string{notApplicable, notApplicable, "level.mapping.identity-unprofiled-history"}},
	"identity-inexact":                    {notApplicable, notApplicable, []string{notApplicable}},
	"generator-recursive":                 {"generator.construct.end-recursive", "construct", nil},
	"generator-recursive-nested":          {"generator.construct.end-recursive", "construct", nil},
	"generator-qualified-selector-prefix": {"generator.construct.qualified-selector-prefix", "construct", nil},
	// structuralIndex is a free function and stays unattributed. Its
	// propagation frames do not invent an origin, and the consumed
	// structural-LET attempt before it is forgotten rather than reported.
	"unknown-structural-index":     {"unknown", "unknown", nil},
	"unknown-structural-index-let": {"unknown", "unknown", nil},
}

var siteFormat = regexp.MustCompile(`^(driver|generator|level)\.(construct|operation|mapping|precondition|resource)\.[a-z0-9]+(-[a-z0-9]+)*$`)

// checkIncompleteInvariants ties the origin telemetry to the unchanged errors.
func checkIncompleteInvariants(t *testing.T, label string, o levelObservation) {
	t.Helper()
	c := o.cause
	fail := func(format string, args ...any) {
		t.Helper()
		t.Errorf("%s: "+format+" (incomplete %q %q %v err %v)", append(append([]any{label}, args...), c.incomplete, c.incompleteCategory, c.attemptIncomplete, o.err)...)
	}
	valid := func(site string) bool {
		return site == "unknown" || site == notApplicable || (siteFormat.MatchString(site) && len(site) <= maxCauseBytes)
	}
	if !valid(c.incomplete) {
		fail("site format")
	}
	if (c.incomplete == notApplicable) != (o.err != ErrPortIncomplete) {
		fail("site applicability")
	}
	if c.incompleteCategory != incompleteCategory(c.incomplete) {
		fail("category")
	}
	if len(c.attemptIncomplete) != len(o.attempts) {
		fail("attempt sites=%d attempts=%d", len(c.attemptIncomplete), len(o.attempts))
		return
	}
	for i, a := range o.attempts {
		s := c.attemptIncomplete[i]
		if !valid(s) || (s == notApplicable) != (a.err != ErrPortIncomplete) || (a.skipped && s != notApplicable) {
			fail("attempt %d site %q", i, s)
		}
	}
	if c.exit == "level check" && (len(o.attempts) == 0 || c.incomplete != c.attemptIncomplete[len(o.attempts)-1]) {
		fail("level-check exit must report its attempt's origin")
	}
}

func TestIncompleteSiteControls(t *testing.T) {
	for _, c := range siteControls {
		o := observeCauseControl(t, c)
		checkCauseInvariants(t, c.name, o)
		checkIncompleteInvariants(t, c.name, o)
		want, ok := siteExpectations[c.name]
		got := siteExpectation{o.cause.incomplete, o.cause.incompleteCategory, o.cause.attemptIncomplete}
		if !ok || fmt.Sprintf("%+v", got) != fmt.Sprintf("%+v", want) {
			t.Errorf("%s:\n got %+v\nwant %+v", c.name, got, want)
		}
	}
	for _, c := range causeControls {
		checkIncompleteInvariants(t, c.name, observeCauseControl(t, c))
	}
	if len(siteExpectations) != len(siteControls) {
		t.Fatalf("expectations=%d controls=%d", len(siteExpectations), len(siteControls))
	}
}

// Cancellation reaches every driver step; it is never given an origin, even
// where an earlier origin could have been recorded.
func TestIncompleteSiteCancellationIsNotAttributed(t *testing.T) {
	for _, root := range []string{"EXTENDS A\nB == <<X, X>>", "CONSTANT c\nH(p) == p\nHIDE DEF H", "A ==\n  LET RECURSIVE f IN\n  LET f(x) == x IN\n  TRUE"} {
		cancelled, finished := 0, false
		for n := 0; n < 4000 && !finished; n++ {
			left := n
			o := observeLevelCandidate(countdownContext{stdcontext.Background(), &left}, causeAcquire(t, root), 1000)
			label := fmt.Sprintf("%q countdown %d", root, n)
			checkIncompleteInvariants(t, label, o)
			if errors.Is(o.err, stdcontext.Canceled) {
				cancelled++
				if o.cause.incomplete != notApplicable {
					t.Fatalf("%s: cancellation attributed to %q", label, o.cause.incomplete)
				}
			}
			finished = o.err == nil || o.err == ErrPortIncomplete
		}
		if cancelled == 0 || !finished {
			t.Errorf("%q: sweep cancelled=%d finished=%t", root, cancelled, finished)
		}
	}
}

// Depth stops are either DepthLimitError (not attributed, although it
// satisfies errors.Is(err, ErrPortIncomplete)) or a level-checker resource
// site; they are never reported as an unported construct or mapping.
func TestIncompleteSiteDepthIsResourceOrNotApplicable(t *testing.T) {
	for _, c := range []struct{ root, unlimited string }{
		{"EXTENDS A\nB == <<X, X>>", notApplicable},
		{"CONSTANT c\nH(p) == p\nHIDE DEF H", notApplicable},
	} {
		var generation, level bool
		for depth := 1; depth <= 64; depth++ {
			o := observeLevelCandidate(stdcontext.Background(), causeAcquire(t, c.root), depth)
			label := fmt.Sprintf("%q depth %d", c.root, depth)
			checkIncompleteInvariants(t, label, o)
			var limit *DepthLimitError
			switch {
			case errors.As(o.err, &limit):
				generation = true
				if !errors.Is(o.err, ErrPortIncomplete) || o.cause.incomplete != notApplicable {
					t.Fatalf("%s: depth error attributed to %q", label, o.cause.incomplete)
				}
			case o.cause.incompleteCategory == "resource":
				level = true
				if o.cause.incomplete != "level.resource.depth-limit" {
					t.Fatalf("%s: resource site %q", label, o.cause.incomplete)
				}
			case o.cause.incomplete != c.unlimited:
				t.Fatalf("%s: site %q, unlimited %q", label, o.cause.incomplete, c.unlimited)
			}
		}
		if !generation || !level {
			t.Errorf("%q: depth sweep reached generation=%t level=%t", c.root, generation, level)
		}
	}
}

// Direct level-checker controls: resource, unported operation, cancellation
// and an identity helper that cannot be attributed.
func TestLevelCheckerIncompleteOrigins(t *testing.T) {
	plain := func(leafNode) (uint64, uint32, error) { return 1, 1, nil }
	cases := []struct {
		name     string
		ctx      stdcontext.Context
		identity levelIdentity
		work     uint64
		node     any
		want     string
	}{
		{"work budget", stdcontext.Background(), plain, 1, &numeralNode{}, "level.resource.work-budget"},
		{"unsupported node", stdcontext.Background(), plain, 1 << 20, new(int), "level.operation.unsupported-node"},
		{"identity helper without origin", stdcontext.Background(), func(leafNode) (uint64, uint32, error) { return 0, 0, ErrPortIncomplete }, 1 << 20, &formalParamNode{}, "unknown"},
		{"identity helper other error", stdcontext.Background(), func(leafNode) (uint64, uint32, error) { return 0, 0, stdcontext.DeadlineExceeded }, 1 << 20, &formalParamNode{}, notApplicable},
		{"cancelled", cancelledContext(), plain, 1 << 20, &numeralNode{}, notApplicable},
	}
	for _, tc := range cases {
		c, err := newLevelChecker(tc.ctx, tc.identity, 100, tc.work)
		if err != nil {
			t.Fatal(err)
		}
		_, err = c.check(tc.node, 1)
		if got := c.incomplete.attribute(err); err == nil || got != tc.want {
			t.Errorf("%s: err %v site %q want %q", tc.name, err, got, tc.want)
		}
		// The checker's stop is sticky; so is the origin it reports.
		_, again := c.check(&numeralNode{}, 1)
		if again != err || c.incomplete.attribute(again) != tc.want {
			t.Errorf("%s: repeated stop %v %q", tc.name, again, c.incomplete.attribute(again))
		}
	}
}

func cancelledContext() stdcontext.Context {
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	return ctx
}

// mark returns the exact singleton; check forgets a consumed origin; depth
// and cancellation errors are never attributed.
func TestGeneratorIncompleteOriginLifecycle(t *testing.T) {
	g, err := newGenerator(stdcontext.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	site := "generator.construct.expression-kind"
	err = g.unported(site)
	if err != ErrPortIncomplete || err.Error() != "semantic port incomplete" || g.incomplete.attribute(err) != site {
		t.Fatalf("mark: %v %q", err, g.incomplete.attribute(err))
	}
	if g.check(1) != nil || g.incomplete.attribute(ErrPortIncomplete) != "unknown" {
		t.Fatalf("check kept consumed origin %q", g.incomplete.site)
	}
	_ = g.unported(site)
	g.incomplete.forget()
	if g.incomplete.attribute(ErrPortIncomplete) != "unknown" {
		t.Fatal("forget")
	}
	_ = g.unported(site)
	depth := g.check(11)
	if !errors.Is(depth, ErrPortIncomplete) || depth == ErrPortIncomplete || g.incomplete.attribute(depth) != notApplicable {
		t.Fatalf("depth: %v %q", depth, g.incomplete.attribute(depth))
	}
	c, err := newGenerator(cancelledContext(), 10)
	if err != nil {
		t.Fatal(err)
	}
	_ = c.unported(site)
	if e := c.check(1); e == nil || c.incomplete.attribute(e) != notApplicable {
		t.Fatalf("cancelled: %v", e)
	}
}

func TestIncompleteSiteHelpers(t *testing.T) {
	for _, tc := range []struct{ site, category string }{
		{"level.resource.work-budget", "resource"},
		{"generator.construct.operator", "construct"},
		{"level.mapping.identity-history-inexact", "mapping"},
		{"level.operation.unsupported-node", "operation"},
		{"driver.precondition.no-acquisition", "precondition"},
		{"unknown", "unknown"},
		{notApplicable, notApplicable},
		{"level.other.x", "unknown"},
		{"level", "unknown"},
	} {
		if got := incompleteCategory(tc.site); got != tc.category {
			t.Errorf("category(%q)=%q", tc.site, got)
		}
	}
	long := incompleteOrigin{site: "level.precondition." + strings.Repeat("x", maxCauseBytes)}
	if long.attribute(ErrPortIncomplete) != "unknown" || (incompleteOrigin{}).attribute(ErrPortIncomplete) != "unknown" || (incompleteOrigin{site: "level.resource.depth-limit"}).attribute(nil) != notApplicable {
		t.Fatal("attribute bounds")
	}
	var trace *flatProfileTrace
	if trace.unported("driver.precondition.no-acquisition") != ErrPortIncomplete {
		t.Fatal("nil trace")
	}
}

// Every origin is a fixed, bounded source literal; no site is computed.
func TestIncompleteSitesAreFixedLiterals(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	call := regexp.MustCompile(`\.(unported|mark)\(([^)]*)\)|\.site = ([^\n]*)`)
	sites := map[string]int{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") || e.Name() == "incomplete_origin.go" {
			continue
		}
		b, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range call.FindAllSubmatch(b, -1) {
			arg := string(m[2]) + string(m[3])
			if len(arg) < 2 || arg[0] != '"' || arg[len(arg)-1] != '"' || !siteFormat.MatchString(arg[1:len(arg)-1]) || len(arg)-2 > maxCauseBytes {
				t.Errorf("%s: non-literal or malformed site %s", e.Name(), m[0])
				continue
			}
			sites[arg[1:len(arg)-1]]++
		}
	}
	stages := map[string]bool{}
	for s := range sites {
		stages[strings.SplitN(s, ".", 2)[0]] = true
	}
	if len(sites) < 100 || !stages["driver"] || !stages["generator"] || !stages["level"] {
		t.Fatalf("found %d distinct sites in stages %v", len(sites), stages)
	}
}

// Telemetry is owned by the call: interleaved, repeated and concurrent calls
// report independent origins, and a caller's mutation never leaks back.
func TestIncompleteSiteIsCallOwned(t *testing.T) {
	controls := append(append([]causeControl{}, siteControls...), causeControls[12], causeControls[0])
	sequential := make([]string, len(controls))
	for i, c := range controls {
		o := observeCauseControl(t, c)
		sequential[i] = fmt.Sprintf("%q %q %v", o.cause.incomplete, o.cause.incompleteCategory, o.cause.attemptIncomplete)
		if len(o.cause.attemptIncomplete) > 0 {
			o.cause.attemptIncomplete[0] = "mutated"
		}
	}
	for i := len(controls) - 1; i >= 0; i-- {
		o := observeCauseControl(t, controls[i])
		if got := fmt.Sprintf("%q %q %v", o.cause.incomplete, o.cause.incompleteCategory, o.cause.attemptIncomplete); got != sequential[i] {
			t.Errorf("%s: repeated %s want %s", controls[i].name, got, sequential[i])
		}
	}
	got := make([]string, len(controls))
	for round := 0; round < 4; round++ {
		// Acquire on the test goroutine; observations then run concurrently.
		acquisitions := make([]*modulegraph.Acquisition, len(controls))
		for i, c := range controls {
			acquisitions[i] = causeAcquire(t, c.root)
		}
		var wg sync.WaitGroup
		for i := range controls {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				o := observeLevelCandidate(stdcontext.Background(), acquisitions[i], controls[i].depth)
				got[i] = fmt.Sprintf("%q %q %v", o.cause.incomplete, o.cause.incompleteCategory, o.cause.attemptIncomplete)
			}(i)
		}
		wg.Wait()
		for i := range controls {
			if got[i] != sequential[i] {
				t.Errorf("round %d %s: concurrent %s want %s", round, controls[i].name, got[i], sequential[i])
			}
		}
	}
}
