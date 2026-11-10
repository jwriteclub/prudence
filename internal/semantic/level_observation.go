// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"github.com/jwriteclub/prudence/internal/modulegraph"
)

// levelObservation is a candidate execution, not an enabled-entry identity
// certificate. The source-modeled tickets provide semantic UID hashes when the distinct
// enabled-entry history remains exact. Runtime mapping remains unverified.
type levelObservation struct {
	generation, level                                            []Detail
	attempts                                                     []levelModuleAttempt
	examined, finished, bound, levelComplete, generationComplete bool
	err                                                          error
	cause                                                        levelCause
}

// levelCause is private, bounded, call-owned telemetry about why the flags
// above hold. It is derived from the same execution and never feeds back into
// it. "unknown" means the implementation cannot establish the cause.
type levelCause struct {
	exit            string   // driver step that stopped execution, or "completed"
	driverProfile   string   // "linear", "flat-extends", "standard-local-instance", "root-nested/<sub-profile>", "general-driver", "rejected" or "unknown"
	driverRejection string   // "none" when admitted, rejection text, or "unknown"
	historyState    string   // "exact", "invalidated", "unprofiled" or "unknown"
	historyFirst    string   // first invalidation; "none", "not-applicable" or "unknown"
	historyPhase    string   // "bootstrap", "generation", "level-boundary", "none", "not-applicable" or "unknown"
	historyUnit     int      // 0-based dependency-order unit of historyPhase, -1 if none
	completion      string   // first stage-completeness loss, "none" or "execution unfinished"
	completionUnit  int      // 0-based unit of completion, -1 if none
	boundaries      []string // per attempt: "skipped", "supported" or failed conditions
	// Origin of a final ErrPortIncomplete (see incompleteOrigin): a fixed
	// site, "unknown" when no attributable origin was recorded, or
	// "not-applicable" when the final error is not exactly ErrPortIncomplete.
	incomplete         string
	incompleteCategory string   // derived from incomplete by incompleteCategory
	attemptIncomplete  []string // per attempt, the same for the attempt's error; skipped attempts are "not-applicable"
}

// maxCauseBytes bounds every telemetry string. Reasons are source constants;
// a longer one is reported as unknown rather than truncated.
const maxCauseBytes = 128

func boundedCause(s string) string {
	if s == "" || len(s) > maxCauseBytes {
		return "unknown"
	}
	return s
}

func observeLevelCandidate(ctx stdcontext.Context, acquisition *modulegraph.Acquisition, maxDepth int) (out levelObservation) {
	var trace flatProfileTrace
	var own incompleteOrigin // origins created by this function itself
	var c *levelChecker
	out.cause = levelCause{exit: "invalid request", historyUnit: -1, completionUnit: -1}
	defer func() {
		out.cause.finish(&trace)
		out.cause.attribute(out.err, own, c, &trace)
	}()
	if ctx == nil || maxDepth <= 0 {
		out.err = own.mark("level.precondition.invalid-request")
		return
	}
	var owner *namePool
	// The checker calls identity only from check, after c is assigned. The
	// cases split the original disjunction without changing its result.
	identity := func(n leafNode) (uint64, uint32, error) {
		switch {
		case owner == nil:
			return 0, 0, c.unported("level.mapping.identity-no-published-module")
		case owner.history.profile == 0:
			return 0, 0, c.unported("level.mapping.identity-unprofiled-history")
		case owner.history.profile != levelEnabledProfile:
			return 0, 0, c.unported("level.mapping.identity-profile-not-level-enabled")
		case !owner.history.exact:
			return 0, 0, c.unported("level.mapping.identity-history-inexact")
		}
		t := owner.history.reuse(n)
		if !t.known {
			return 0, 0, c.unported("level.mapping.identity-unknown-ticket")
		}
		if n.header().kind != t.kind {
			return 0, 0, c.unported("level.mapping.identity-kind-mismatch")
		}
		// SemanticNode.hashCode: 31*(31+kind)+myUID (Java int overflow).
		return uint64(t.uid) + 1, uint32(961+31*t.kind) + t.uid, nil
	}
	c, e := newLevelChecker(ctx, identity, maxDepth, 1<<20)
	if e != nil {
		out.cause.exit = "level checker setup"
		out.err = e
		// newLevelChecker's only error is its argument guard.
		own.site = "level.precondition.checker-setup"
		return
	}
	var observed int
	validStages, priorBad := true, false
	details, examined, root, _, err := analyzeFlatExternalProfile(ctx, acquisition, maxDepth, true, func(m *moduleNode, errors *Errors) error {
		all := errors.Messages()
		out.generation = append(out.generation, all[observed:]...)
		observed = len(all)
		unit := len(out.attempts)
		if priorBad {
			validStages = false
			out.cause.lose("prior entered level boundary unsupported", unit)
		}
		owner = m.owner
		if !owner.history.exact || owner.history.profile != levelEnabledProfile {
			validStages = false
			switch {
			case owner.history.profile == 0:
				out.cause.lose("no history profile at publication", unit)
			case !owner.history.exact:
				out.cause.lose("history invalidated before publication", unit)
			default:
				out.cause.lose("history profile not level-enabled at publication", unit)
			}
		}
		out.cause.locate(&owner.history, "generation", unit)
		a := levelModuleAttempt{module: m}
		if !errors.Success() {
			a.skipped = true
			// SANY's Errors gate skips the call entirely. Preserve an exact
			// prefix, but never restore one invalidated by an entered call.
			out.attempts = append(out.attempts, a)
			out.cause.boundaries = append(out.cause.boundaries, "skipped")
			out.cause.attemptIncomplete = append(out.cause.attemptIncomplete, "not-applicable")
			return nil
		}
		a.entered = true
		// The old direct-allocation cursor has no enabled-entry binding.
		m.owner.java.invalidate()
		before := len(all)
		prefix := owner.history.count
		c.errors = errors
		a.correct, a.err = c.check(m, 1)
		a.completed = a.err == nil
		all = errors.Messages()
		// Only a supported successful, error-free boundary may carry the
		// constructor prefix into the next Generator. Never resynchronize.
		if owner.history.count != prefix {
			validStages = false
			out.cause.lose("history events during level check", unit)
		}
		if a.err != nil || !a.correct || len(all) != before || !errors.Success() || owner.history.count != prefix {
			owner.history.invalidate("unsupported or non-successful level boundary")
			priorBad = true
		}
		out.cause.locate(&owner.history, "level-boundary", unit)
		out.cause.boundaries = append(out.cause.boundaries, levelBoundary(a, len(all) != before, errors.Success(), owner.history.count != prefix))
		out.cause.attemptIncomplete = append(out.cause.attemptIncomplete, c.incomplete.attribute(a.err))
		out.level = append(out.level, all[observed:]...)
		observed = len(all)
		out.attempts = append(out.attempts, a)
		return a.err
	}, &trace)
	// Generation can stop before publication; retain accepted records from that
	// partial generation in their original stage.
	if observed <= len(details) {
		out.generation = append(out.generation, details[observed:]...)
	}
	out.examined, out.finished, out.err = examined, err == nil && root != nil, err
	if err == nil && root == nil {
		trace.exit = "completed without root"
	}
	out.bound = out.finished && owner != nil && owner.history.exact && owner.history.profile == levelEnabledProfile
	out.levelComplete = out.finished && validStages
	out.generationComplete = out.finished && validStages
	return
}

// lose records the first loss of stage completeness, in callback order.
func (c *levelCause) lose(reason string, unit int) {
	if c.completion == "" {
		c.completion, c.completionUnit = reason, unit
	}
}

// locate records where an admitted history first became inexact. It is called
// after generation publishes a unit and after each entered level boundary.
func (c *levelCause) locate(h *javaHistory, phase string, unit int) {
	if c.historyPhase == "" && h.profile != 0 && !h.exact {
		c.historyPhase, c.historyUnit = phase, unit
	}
}

// levelBoundary names every failed condition of one entered boundary, in the
// fixed order the boundary test evaluates them.
func levelBoundary(a levelModuleAttempt, diagnostics, success, events bool) string {
	s := ""
	add := func(v string) {
		if s != "" {
			s += ","
		}
		s += v
	}
	if a.err != nil {
		add("level check error")
	} else if !a.correct {
		add("level check incorrect")
	}
	if diagnostics {
		add("level diagnostics added")
	}
	if !success {
		add("errors unsuccessful")
	}
	if events {
		add("history events during level check")
	}
	if s == "" {
		return "supported"
	}
	return s
}

// attribute selects the origin record of the step that stopped execution;
// finish has already settled exit. Driver and generation origins travel in the
// trace, level-check origins in the checker, and this function's own in own.
func (c *levelCause) attribute(err error, own incompleteOrigin, checker *levelChecker, t *flatProfileTrace) {
	origin := own
	switch c.exit {
	case "invalid request", "level checker setup":
	case "level check":
		if checker != nil {
			origin = checker.incomplete
		}
	default:
		origin = t.incomplete
	}
	c.incomplete = origin.attribute(err)
	c.incompleteCategory = incompleteCategory(c.incomplete)
}

func (c *levelCause) finish(t *flatProfileTrace) {
	if t.exit != "" {
		c.exit = t.exit
		if c.exit == "publication hook" {
			c.exit = "level check"
		}
	}
	c.driverProfile, c.driverRejection = "unknown", "unknown"
	c.historyState, c.historyFirst = "unknown", "unknown"
	if t.decided {
		c.driverProfile, c.driverRejection = t.admission, "none"
		if t.admission == "rejected" {
			c.driverRejection = "linear: " + boundedCause(t.linearRejection) + "; flat-extends: " + boundedCause(t.flatRejection) + "; " + standardLocalInstanceProfile + ": " + boundedCause(t.standardWhy) + "; " + rootNestedProfile + ": " + rootNestedCause(t.rootNested) + "; " + generalDriverProfile + ": " + boundedCause(t.generalDriver)
		}
		switch {
		case t.profile == 0:
			c.historyState, c.historyFirst = "unprofiled", "not-applicable"
		case t.exact:
			c.historyState, c.historyFirst = "exact", "none"
		default:
			c.historyState, c.historyFirst = "invalidated", boundedCause(t.firstUnknown)
		}
	}
	switch {
	case c.historyPhase != "":
	case c.historyState == "invalidated" && !t.bootstrapExact:
		c.historyPhase, c.historyUnit = "bootstrap", -1
	case c.historyState == "invalidated" && t.units > 0:
		// No callback observed it, so the last started unit's generation did.
		c.historyPhase, c.historyUnit = "generation", t.units-1
	case c.historyState == "invalidated" || c.historyState == "unknown":
		c.historyPhase = "unknown"
	case c.historyState == "unprofiled":
		c.historyPhase = "not-applicable"
	default:
		c.historyPhase = "none"
	}
	if c.completion == "" {
		c.completion = "none"
		if c.exit != "completed" {
			c.completion = "execution unfinished"
		}
	}
}
