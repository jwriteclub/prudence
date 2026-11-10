// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/modulegraph"
)

// analyzeOwned is shared by the public-facing bridge and differential tests.
// A non-nil module records partial generation; it is not a completion claim.
func analyzeOwned(ctx stdcontext.Context, tree *generated.SyntaxTreeNode, maxDepth int) (*generator, *moduleNode, error) {
	pool := newNamePool()
	pool.java = freshJavaAlloc()
	pool.history = freshJavaHistory(ctx)
	g, e := newOwnedGenerator(ctx, maxDepth, pool, make(map[*name]leafNode), &Errors{})
	if e != nil {
		return nil, nil, e
	}
	m, e := g.module(tree)
	return g, m, e
}

// Analyze examines a parser tree using request-owned internal state. A true
// examined flag means partial semantic generation began, not that it finished.
// ErrPortIncomplete is distinct from a semantic diagnostic or rejection.
func Analyze(ctx stdcontext.Context, tree *generated.SyntaxTreeNode, maxDepth int) (details []Detail, examined bool, err error) {
	g, m, err := analyzeOwned(ctx, tree, maxDepth)
	if g != nil {
		details = g.table.errors.Messages()
	}
	return details, m != nil, err
}

// AnalyzeFlatExternal generates supported acquired units in dependency order.
// A partial module is never published. Ordinary diagnostics do not stop later units.
func AnalyzeFlatExternal(ctx stdcontext.Context, acquisition *modulegraph.Acquisition, maxDepth int) (details []Detail, examined bool, err error) {
	details, examined, _, err = analyzeFlatExternalOwned(ctx, acquisition, maxDepth)
	return
}

func analyzeFlatExternalOwned(ctx stdcontext.Context, acquisition *modulegraph.Acquisition, maxDepth int) (details []Detail, examined bool, root *moduleNode, err error) {
	details, examined, root, _, err = analyzeFlatExternalRoots(ctx, acquisition, maxDepth)
	return
}

func analyzeFlatExternalRoots(ctx stdcontext.Context, acquisition *modulegraph.Acquisition, maxDepth int) (details []Detail, examined bool, root *moduleNode, roots []*moduleNode, err error) {
	return analyzeFlatExternalWithHook(ctx, acquisition, maxDepth, nil)
}

func analyzeFlatExternalWithHook(ctx stdcontext.Context, acquisition *modulegraph.Acquisition, maxDepth int, afterPublish func(*moduleNode, *Errors) error) (details []Detail, examined bool, root *moduleNode, roots []*moduleNode, err error) {
	return analyzeFlatExternalProfile(ctx, acquisition, maxDepth, false, afterPublish, nil)
}

// flatProfileTrace is optional, call-owned private telemetry. The driver only
// writes it; no decision reads it, so execution is identical with or without
// one. All strings are fixed source constants or the history's first reason.
type flatProfileTrace struct {
	decided         bool   // driver-profile selection was reached
	admission       string // "linear", "flat-extends", "standard-local-instance", "root-nested/<sub-profile>", "general-driver" or "rejected"
	linearRejection string // first condition that defeated the linear profile
	flatRejection   string // first condition that defeated the flat-EXTENDS profile
	standardWhy     string // first condition that defeated the standard LOCAL INSTANCE profile
	bootstrapExact  bool   // history exactness immediately after profile bootstrap
	units           int    // generation units started, in dependency order
	exit            string // last driver step reached when err != nil, else "completed"
	profile         javaHistoryProfile
	exact           bool
	firstUnknown    string
	incomplete      incompleteOrigin // driver or generator origin of a terminal error

	rootNested    rootNestedVerdict // first condition that defeated root-local nesting, when rejected
	generalDriver string            // first guard that defeated the general driver, when rejected
}

func firstCause(dst *string, reason string) {
	if *dst == "" {
		*dst = reason
	}
}

// olderProfileRejections returns the first condition that defeats the linear
// and the flat-EXTENDS profile over view, or "" for a profile that admits it.
// Over actualView it is the driver's original inline check, unchanged.
func olderProfileRejections(order []*modulegraph.Unit, units []*modulegraph.Document, docs map[*modulegraph.Unit]*modulegraph.Document, view referenceView) (linearWhy, flatWhy string) {
	if len(order) != len(docs) {
		linearWhy, flatWhy = "dependency order does not cover acquired units", "dependency order does not cover acquired units"
	}
	for _, d := range units {
		refs := view.references(d)
		if len(refs) > 1 {
			firstCause(&linearWhy, "unit with multiple module references")
		}
		if d.RootModule == nil || view.nested(d) {
			structure := "nested module definitions"
			if d.RootModule == nil {
				structure = "module structure unavailable"
			}
			firstCause(&flatWhy, structure)
			if len(order) > 1 {
				firstCause(&linearWhy, structure+" in multi-unit graph")
			}
		}
		seen := map[string]bool{}
		for _, r := range refs {
			if r.Kind != modulegraph.Extends {
				firstCause(&flatWhy, "non-EXTENDS module reference")
			} else if seen[r.Name] {
				firstCause(&flatWhy, "repeated EXTENDS reference")
			}
			seen[r.Name] = true
		}
	}
	return linearWhy, flatWhy
}

// levelEnabled selects the separate source-modeled driver profile; it does not
// widen the legacy acquisition or constructor-history eligibility checks.
func analyzeFlatExternalProfile(ctx stdcontext.Context, acquisition *modulegraph.Acquisition, maxDepth int, levelEnabled bool, afterPublish func(*moduleNode, *Errors) error, trace *flatProfileTrace) (details []Detail, examined bool, root *moduleNode, roots []*moduleNode, err error) {
	step := "invalid acquisition"
	if trace != nil {
		defer func() {
			if err == nil {
				trace.exit = "completed"
			} else {
				trace.exit = step
			}
		}()
	}
	if acquisition == nil || acquisition.Root == nil {
		return nil, false, nil, nil, trace.unported("driver.precondition.no-acquisition")
	}
	step = "dependency order"
	order, err := acquisition.Order(ctx)
	if err != nil {
		return nil, false, nil, nil, err
	}
	step = "invalid acquisition"
	docs := make(map[*modulegraph.Unit]*modulegraph.Document, len(acquisition.Units))
	for _, d := range acquisition.Units {
		if d == nil || d.Unit == nil || docs[d.Unit] != nil {
			return nil, false, nil, nil, trace.unported("driver.precondition.invalid-unit")
		}
		docs[d.Unit] = d
	}
	pool := newNamePool()
	if len(order) == 1 && len(acquisition.Root.References()) == 0 {
		pool.java = freshJavaAlloc()
	}
	// A single chain has unique postorder. For flat EXTENDS-only graphs,
	// unresolved extendees are selected in source order before recursion and
	// both drivers then traverse their recorded vectors in that order.
	linearWhy, flatWhy := olderProfileRejections(order, acquisition.Units, docs, actualView())
	linear, flatExtends := linearWhy == "", flatWhy == ""
	// The standard-library LOCAL INSTANCE profile is consulted only when both
	// older profiles reject, so it can never displace one of them.
	standardLocal := false
	var standardWhy string
	if !linear && !flatExtends {
		standardWhy = standardLocalInstanceRejection(acquisition, order, docs, actualView())
		standardLocal = standardWhy == ""
	}
	// Root-local nesting is consulted only after all three reject, so no
	// existing admission moves. It evaluates a read-only reduced view.
	var nested rootNestedVerdict
	if !linear && !flatExtends && !standardLocal {
		nested = rootNestedRejection(ctx, acquisition, order, docs)
	}
	rootNested := nested.profile != ""
	// The general driver is consulted only after all four reject, so no
	// existing admission or post-guard moves, and it is never retried after a
	// post-guard invalidates an earlier admission.
	var generalWhy string
	if !linear && !flatExtends && !standardLocal && !rootNested {
		generalWhy = generalDriverRejection(ctx, acquisition, order, docs)
	}
	generalDriver := !linear && !flatExtends && !standardLocal && !rootNested && generalWhy == ""
	// The standard post-generation guard covers every unit of either standard
	// admission, including the reduced view's standard sub-profile.
	standardGuard := standardLocal || nested.profile == standardLocalInstanceProfile
	if linear || flatExtends || standardLocal || rootNested || generalDriver {
		if levelEnabled {
			pool.history = freshLevelEnabledHistory(ctx)
		} else {
			pool.history = freshJavaHistory(ctx)
		}
	}
	if trace != nil {
		trace.decided, trace.bootstrapExact = true, pool.history.exact
		switch {
		case linear:
			trace.admission = "linear"
		case flatExtends:
			trace.admission = "flat-extends"
		case standardLocal:
			trace.admission = standardLocalInstanceProfile
		case rootNested:
			trace.admission = rootNestedProfile + "/" + nested.profile
			trace.linearRejection, trace.flatRejection, trace.standardWhy = linearWhy, flatWhy, standardWhy
		case generalDriver:
			trace.admission = generalDriverProfile
			trace.linearRejection, trace.flatRejection, trace.standardWhy = linearWhy, flatWhy, standardWhy
			trace.rootNested = nested
		default:
			trace.admission, trace.linearRejection, trace.flatRejection, trace.standardWhy = "rejected", linearWhy, flatWhy, standardWhy
			trace.rootNested, trace.generalDriver = nested, generalWhy
		}
		defer func() {
			trace.profile, trace.exact, trace.firstUnknown = pool.history.profile, pool.history.exact, pool.history.firstUnknown
		}()
	}
	external := make(map[*name]leafNode)
	errors := &Errors{}
	defer func() { details = errors.Messages() }()
	for _, unit := range order {
		step = "cancelled before unit"
		if err = ctx.Err(); err != nil {
			return
		}
		step = "unit unavailable"
		d := docs[unit]
		if d == nil || d.Parse.Tree == nil {
			err = trace.unported("driver.precondition.unit-unavailable")
			return
		}
		if trace != nil {
			trace.units++
		}
		step = "generator setup"
		var g *generator
		g, err = newOwnedGenerator(ctx, maxDepth, pool, external, errors)
		if err != nil {
			return
		}
		step = "generation"
		var m *moduleNode
		m, err = g.moduleExtending(d.Parse.Tree, nil)
		examined = examined || m != nil
		if err != nil {
			if trace != nil {
				trace.incomplete = g.incomplete
			}
			return
		}
		if standardGuard && !standardInstancesProved(m) {
			pool.history.invalidate("unproved standard LOCAL INSTANCE target")
		}
		step = "origin"
		standard, originErr := standardOrigin(d.Input.Origin)
		if originErr != nil {
			err = originErr
			if originErr == ErrPortIncomplete && trace != nil {
				// standardOrigin's only incomplete origin is an unmapped value.
				trace.incomplete.site = "driver.mapping.module-origin"
			}
			return
		}
		m.standard = standard
		external[m.name] = m
		roots = append(roots, m)
		step = "publication hook"
		if afterPublish != nil {
			if err = afterPublish(m, errors); err != nil {
				return
			}
		}
		if d == acquisition.Root {
			root = m
		}
	}
	return
}

// standardOrigin converts the acquisition transport provenance explicitly.
func standardOrigin(origin uint8) (bool, error) {
	switch origin {
	case modulegraph.OriginUnspecified, modulegraph.OriginWorkspace:
		return false, nil
	case modulegraph.OriginStandard:
		return true, nil
	default:
		return false, ErrPortIncomplete
	}
}

// levelModuleAttempt records entry separately from correctness and completion.
// The private entry remains incomplete until the enabled-profile allocation
// composition and all reachable module branches are established.
type levelModuleAttempt struct {
	module                      *moduleNode
	entered, completed, skipped bool
	correct                     bool
	err                         error
}

func analyzeFlatExternalLevelAttempt(ctx stdcontext.Context, acquisition *modulegraph.Acquisition, maxDepth int, c *levelChecker) (details []Detail, examined bool, root *moduleNode, attempts []levelModuleAttempt, err error) {
	if c == nil || ctx == nil || len(c.states) != 0 || c.stopped != nil {
		return nil, false, nil, nil, ErrPortIncomplete
	}
	c.ctx = ctx
	details, examined, root, _, err = analyzeFlatExternalWithHook(ctx, acquisition, maxDepth, func(m *moduleNode, errors *Errors) error {
		a := levelModuleAttempt{module: m}
		if !errors.Success() {
			a.skipped = true
			attempts = append(attempts, a)
			return nil
		}
		a.entered = true
		c.errors = errors
		// Semantic-only tickets do not establish the enabled-entry profile. Even
		// after partial checks succeed, this attempt must remain incomplete.
		m.owner.history.invalidate("level-enabled entry profile unproved")
		m.owner.java.invalidate()
		a.correct, a.err = c.check(m, 1)
		a.completed = a.err == nil
		attempts = append(attempts, a)
		return a.err
	})
	if err == nil {
		err = ErrPortIncomplete
	}
	return
}
