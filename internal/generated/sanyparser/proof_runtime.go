// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Prudence-owned extension; not copied from jigo (see PROVENANCE).
package generated

import "github.com/jwriteclub/prudence/internal/parserrt"

// IsProofRuntimeFailure reports whether err is exactly the bound parser's
// callback wrapper around proof.go's errProofRuntime: a
// *parserrt.OperationalError whose direct Cause is that typed sentinel. Proof
// support returns the sentinel where Java SANY's proof-step handling throws a
// runtime exception (for example Integer.parseInt overflow on a step level),
// so the parse stopped without a source verdict.
//
// Only that two-level shape matches. Any other wrapper (source,
// interruption, depth or an extra layer), another cause, or an error that
// merely carries the same text is not recognized.
func IsProofRuntimeFailure(err error) bool {
	op, ok := err.(*parserrt.OperationalError)
	if !ok || op == nil {
		return false
	}
	cause, ok := op.Cause.(supportError)
	return ok && cause == errProofRuntime
}
