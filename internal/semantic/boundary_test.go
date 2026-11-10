// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"errors"
	"github.com/jwriteclub/prudence/internal/recursion"
	"strings"
	"testing"
)

func TestGeneratorDepthBudget(t *testing.T) {
	g, err := newGenerator(stdcontext.Background(), 0)
	if err != nil || g.maxDepth != recursion.DefaultMaxDepth {
		t.Fatalf("generator=%v err=%v", g, err)
	}
	if err = g.check(513); err != nil {
		t.Fatal(err)
	}
	g, err = newGenerator(stdcontext.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	err = g.check(11)
	var depth *DepthLimitError
	if !errors.As(err, &depth) || depth.Limit != 10 || !errors.Is(err, ErrPortIncomplete) {
		t.Fatalf("depth=%v err=%v", depth, err)
	}
}

func TestGeneratorNestedExpressionBudget(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nX == "+strings.Repeat("~ ", 600)+"TRUE\n====\n")
	_, examined, err := Analyze(stdcontext.Background(), tree, 2000)
	if err != nil || !examined {
		t.Fatalf("within budget: examined=%v err=%v", examined, err)
	}
	_, _, err = Analyze(stdcontext.Background(), tree, 100)
	var depth *DepthLimitError
	if !errors.As(err, &depth) {
		t.Fatalf("lower budget: %v", err)
	}
}
