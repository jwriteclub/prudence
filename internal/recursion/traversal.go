// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package recursion

// Location identifies a node in the source being traversed.
type Location struct{ Offset, Line, Column int }

// Traversal tracks one recursive traversal. Keep the same instance while
// crossing both child edges and named references, and leave only successful
// entries. An observed limit remains sticky for that traversal.
type Traversal struct {
	limit      int
	stage      string
	kinds      [28]int
	references int
	err        error
}

// NewTraversal copies and resolves the caller's per-invocation policy.
func NewTraversal(options Options, stage string) (*Traversal, error) {
	limit, err := options.Limit()
	if err != nil {
		return nil, err
	}
	return &Traversal{limit: limit, stage: stage}, nil
}

// bucket is finite even for direct callers supplying arbitrary node kinds.
// All unrecognized kinds share bucket zero.
func bucket(kind string) int {
	switch kind {
	case "action":
		return 1
	case "bnf":
		return 2
	case "call":
		return 3
	case "catch":
		return 4
	case "character":
		return 5
	case "character_list":
		return 6
	case "character_range":
		return 7
	case "choice":
		return 8
	case "eof":
		return 9
	case "finally":
		return 10
	case "group":
		return 11
	case "javacode":
		return 12
	case "literal":
		return 13
	case "lookahead":
		return 14
	case "lookahead_amount":
		return 15
	case "optional":
		return 16
	case "predicate":
		return 17
	case "regex_definition":
		return 18
	case "regex_group":
		return 19
	case "regex_inline":
		return 20
	case "regex_reference":
		return 21
	case "regex_sequence":
		return 22
	case "regex_spec":
		return 23
	case "sequence":
		return 24
	case "token_manager_declarations":
		return 25
	case "try":
		return 26
	case "TOKEN", "SKIP", "MORE", "SPECIAL_TOKEN":
		return 27
	default:
		return 0
	}
}

func (t *Traversal) fail(rule string, at Location) error {
	if t.err == nil {
		t.err = &DepthError{Stage: t.stage, Rule: rule, Offset: at.Offset, Line: at.Line, Column: at.Column, Limit: t.limit}
	}
	return t.err
}

// EnterNode counts active occurrences in a finite canonical kind bucket.
func (t *Traversal) EnterNode(kind string, at Location) error {
	if t.err != nil {
		return t.err
	}
	b := bucket(kind)
	if t.kinds[b] >= t.limit {
		if b == 0 {
			kind = "unknown"
		}
		return t.fail(kind, at)
	}
	t.kinds[b]++
	return nil
}

// LeaveNode releases a previously successful EnterNode.
func (t *Traversal) LeaveNode(kind string) {
	b := bucket(kind)
	if t.kinds[b] > 0 {
		t.kinds[b]--
	}
}

// EnterReference bounds active named-reference crossings, including chains
// whose names are all different.
func (t *Traversal) EnterReference(at Location) error {
	if t.err != nil {
		return t.err
	}
	if t.references >= t.limit {
		return t.fail("reference", at)
	}
	t.references++
	return nil
}

// LeaveReference releases a previously successful EnterReference.
func (t *Traversal) LeaveReference() {
	if t.references > 0 {
		t.references--
	}
}

// Err returns the first observed limit error, if any.
func (t *Traversal) Err() error { return t.err }
