// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	"slices"
	"unicode/utf16"
)

// labelExtrasStableOrder is the documented fallback order for the residual
// list of a 4332 "Label <name> declares extra parameter(s)  <names>" error
// when the bounded Java cursor (orderedLabelExtras) cannot reproduce Java's
// order.
//
// formalParamsEqual (Generator.java:7092-7097, pinned dc6a944) iterates the
// residual HashSet only to append names to the message; retVal is already
// false, and generateLabel discards the result (6933). No later behavior
// observes the iteration order, so under the approved list-order exception the list
// is compared as a multiset. This order makes no claim about Java's order or
// UID history and touches neither.
//
// It applies only when every residual identity has a distinct name, and
// returns every name once, in UTF-16 code-unit lexical order. Residuals with
// a repeated name among different names return false and stay incomplete.
func labelExtrasStableOrder(remaining map[*formalParamNode]bool) ([]string, bool) {
	names := make([]string, 0, len(remaining))
	seen := make(map[string]bool, len(remaining))
	for p := range remaining {
		if p.name == nil || !p.name.displayExact || seen[p.name.text] {
			return nil, false
		}
		seen[p.name.text] = true
		names = append(names, p.name.text)
	}
	slices.SortFunc(names, compareUTF16)
	return names, true
}

// compareUTF16 orders strings by their UTF-16 code units, as Java's
// String.compareTo does.
func compareUTF16(a, b string) int {
	return slices.Compare(utf16.Encode([]rune(a)), utf16.Encode([]rune(b)))
}
