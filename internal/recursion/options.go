// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Package recursion defines the per-invocation recursion policy shared by the
// reader, generators and experimental generated parser runtime.
package recursion

import "fmt"

// DefaultMaxDepth is the default active-recursion limit.
const DefaultMaxDepth = 2000

// Options is copied for each invocation. Zero MaxDepth selects the consuming
// subsystem's default; direct Limit calls use DefaultMaxDepth. A positive
// value selects a different limit for that invocation.
type Options struct{ MaxDepth int }

// Limit validates the options and returns the effective depth limit.
func (o Options) Limit() (int, error) {
	if o.MaxDepth < 0 {
		return 0, fmt.Errorf("recursion depth must not be negative")
	}
	if o.MaxDepth == 0 {
		return DefaultMaxDepth, nil
	}
	return o.MaxDepth, nil
}

// DepthError identifies a recursion limit reached during processing. Offset is
// a byte offset when the source is available; line and column are one-based.
type DepthError struct {
	Stage, Rule                 string
	Offset, Line, Column, Limit int
}

func (e *DepthError) Error() string {
	if e.Line > 0 && e.Column > 0 {
		return fmt.Sprintf("%s:%d:%d: recursion depth exceeds %d", e.Stage, e.Line, e.Column, e.Limit)
	}
	return fmt.Sprintf("%s: recursion depth exceeds %d", e.Stage, e.Limit)
}
