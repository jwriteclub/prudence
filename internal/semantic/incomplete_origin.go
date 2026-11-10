// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import "strings"

// incompleteOrigin is private, call-owned telemetry naming where the
// ErrPortIncomplete currently propagating was created. mark returns the exact
// singleton, unwrapped, so error identity, text and every decision built on it
// are unchanged; nothing ever reads the site to choose a path.
//
// A site is a fixed source literal "<stage>.<category>.<detail>": stage is
// driver, generator or level; category is construct, operation, mapping,
// precondition or resource (see incompleteCategory). Only true origins mark;
// propagation frames return the error untouched. A site is reported only
// while the final error is exactly ErrPortIncomplete, so cancellation, depth
// errors (DepthLimitError) and other failures are never labeled with it.
type incompleteOrigin struct{ site string }

// mark records the origin and returns the unchanged singleton.
func (o *incompleteOrigin) mark(site string) error {
	o.site = site
	return ErrPortIncomplete
}

// forget drops a recorded origin whose error was consumed by an alternative
// path, so a later unmarked origin is reported as unknown rather than stale.
func (o *incompleteOrigin) forget() { o.site = "" }

// attribute reports the site for err: "not-applicable" unless err is exactly
// ErrPortIncomplete, "unknown" when no attributable origin was recorded.
func (o incompleteOrigin) attribute(err error) string {
	switch {
	case err != ErrPortIncomplete:
		return "not-applicable"
	case o.site == "" || len(o.site) > maxCauseBytes:
		return "unknown"
	default:
		return o.site
	}
}

// incompleteCategory derives the fixed category from a reported site:
// construct (an unported or unsupported Generator construct shape),
// operation (an unported level-checker operation), mapping (a required
// identity or provenance mapping is unavailable), precondition (an internal
// request or node-shape invariant failed) or resource (a work or depth budget
// reported as incomplete). "unknown" and "not-applicable" pass through.
func incompleteCategory(site string) string {
	if site == "unknown" || site == "not-applicable" {
		return site
	}
	parts := strings.SplitN(site, ".", 3)
	if len(parts) == 3 {
		switch parts[1] {
		case "construct", "operation", "mapping", "precondition", "resource":
			return parts[1]
		}
	}
	return "unknown"
}

func (g *generator) unported(site string) error { return g.incomplete.mark(site) }

func (c *levelChecker) unported(site string) error { return c.incomplete.mark(site) }

// unported records a driver origin when the optional trace is present.
func (t *flatProfileTrace) unported(site string) error {
	if t == nil {
		return ErrPortIncomplete
	}
	return t.incomplete.mark(site)
}
