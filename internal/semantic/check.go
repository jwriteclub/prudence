// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"

	"github.com/jwriteclub/prudence/internal/modulegraph"
)

// CheckStage names the SANY stage that recorded a CheckOutcome detail.
type CheckStage uint8

const (
	StageGeneration CheckStage = iota + 1
	StageLevel
)

// CheckOutcome is one request-owned front-end run over an acquisition.
// Details and Stages are parallel, in SANY Errors order. Err is nil exactly
// when the requested analysis reached SANY's own stopping point: every unit
// generated (and, when levels were requested, level-checked or skipped by the
// shared Errors gate), or generation stopped at a modeled AbortException.
// A non-nil Err (ErrPortIncomplete, *DepthLimitError, a context error or an
// acquisition-order failure) means the run is incomplete; Details then hold
// only what was accepted before the stop.
type CheckOutcome struct {
	Details  []Detail
	Stages   []CheckStage
	Examined bool // generation began
	Levels   bool // at least one module entered level checking
	Aborted  bool // generation stopped at a modeled AbortException
	Err      error
	Site     string // fixed origin site of an ErrPortIncomplete, or "unknown"/"not-applicable"
}

// checkWorkLimit bounds level-checker steps and collection entries per call.
const checkWorkLimit = 1 << 24

// CheckFlatExternal runs SANY's frontEndSemanticAnalysis loop: one shared
// Errors, one name pool and module table across units in dependency order,
// and, when levels is true, ModuleNode.levelCheck after each published unit
// while the shared Errors is still successful (SANY.java:463-466).
//
// Level-checker symbol identity is per call. A node whose constructor ticket
// was recorded while the level-enabled Java history was exact uses that
// source UID and SemanticNode.hashCode; any other node uses its call-owned
// node sequence number in a disjoint range. The first identity chosen for a
// node is kept for the whole call. This is a runtime identity only: it does
// not certify a Java UID mapping, and private conformance observations keep
// their stricter identity.
func CheckFlatExternal(ctx stdcontext.Context, acquisition *modulegraph.Acquisition, maxDepth int, levels bool) (out CheckOutcome) {
	out.Site = "not-applicable"
	if ctx == nil || maxDepth <= 0 {
		out.Err = ErrPortIncomplete
		out.Site = "level.precondition.invalid-request"
		return
	}
	var trace flatProfileTrace
	var checker *levelChecker
	var hook func(*moduleNode, *Errors) error
	observed := 0
	record := func(all []Detail, stage CheckStage) {
		for _, d := range all[observed:] {
			out.Details = append(out.Details, d)
			out.Stages = append(out.Stages, stage)
		}
		observed = len(all)
	}
	if levels {
		ids := make(map[leafNode]levelSymbol)
		identity := func(n leafNode) (uint64, uint32, error) {
			if s, ok := ids[n]; ok {
				return s.id, s.hash, nil
			}
			h := n.header()
			if h == nil || h.owner == nil || h.identity == 0 || h.identity >= 1<<62 {
				return 0, 0, checker.unported("level.precondition.runtime-identity")
			}
			s := levelSymbol{node: n, id: runtimeIdentityBase + h.identity, hash: uint32(961+31*h.kind) + uint32(h.identity)}
			if hist := &h.owner.history; hist.profile == levelEnabledProfile {
				if t := hist.reuse(n); t.known && t.kind == h.kind {
					// SemanticNode.hashCode: 31*(31+kind)+myUID (Java int overflow).
					s.id, s.hash = uint64(t.uid)+1, uint32(961+31*t.kind)+t.uid
				}
			}
			ids[n] = s
			return s.id, s.hash, nil
		}
		var err error
		checker, err = newLevelChecker(ctx, identity, maxDepth, checkWorkLimit)
		if err != nil {
			out.Err = err
			out.Site = "level.precondition.checker-setup"
			return
		}
		hook = func(m *moduleNode, errors *Errors) error {
			record(errors.Messages(), StageGeneration)
			if !errors.Success() {
				return nil
			}
			out.Levels = true
			checker.errors = errors
			_, err := checker.check(m, 1)
			record(errors.Messages(), StageLevel)
			return err
		}
	}
	details, examined, root, _, err := analyzeFlatExternalProfile(ctx, acquisition, maxDepth, levels, hook, &trace)
	if observed <= len(details) {
		record(details, StageGeneration)
	}
	out.Examined = examined
	switch {
	case err == nil && root == nil:
		out.Err = ErrPortIncomplete
		out.Site = "driver.precondition.no-root"
	case err != nil && sourceAbort(err, details):
		out.Aborted = true
	case err != nil:
		out.Err = err
		origin := trace.incomplete
		if trace.exit == "publication hook" && checker != nil {
			origin = checker.incomplete
		}
		out.Site = origin.attribute(err)
	}
	return
}

// runtimeIdentityBase keeps call-owned node sequence identities disjoint from
// source UID identities, which are at most 1<<32.
const runtimeIdentityBase = uint64(1) << 40

// sourceAbort admits only the unwrapped *Abort a modeled `throw
// errors.addMessage` returns. Generator.java has no catch or finally, so the
// log at the throw is final; the abort's own log must equal the observed one
// record for record and contain the thrown record (Errors.java:159-166 appends
// it unless already present). Every other error stays unfinished generation.
func sourceAbort(err error, details []Detail) bool {
	abort, ok := err.(*Abort)
	if !ok || abort == nil || abort.Source == nil || len(abort.Source.messages) != len(details) {
		return false
	}
	thrown := false
	for i, d := range details {
		if !sameDetail(d, abort.Source.messages[i]) {
			return false
		}
		thrown = thrown || sameDetail(d, abort.Detail)
	}
	return thrown
}
