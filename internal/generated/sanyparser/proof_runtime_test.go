// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package generated

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/jwriteclub/prudence/internal/lexer"
	"github.com/jwriteclub/prudence/internal/parserrt"
	"github.com/jwriteclub/prudence/internal/recursion"
)

// proofOverflow's step level is one past Java's int range, so the translated
// Integer.parseInt returns errProofRuntime; proofIntMax is the last level
// Java accepts.
const (
	proofOverflow = "---- MODULE P ----\nTHEOREM TRUE\n<2147483648>1. TRUE\n====\n"
	proofIntMax   = "---- MODULE P ----\nTHEOREM TRUE\n<2147483647>1. TRUE\n====\n"
)

// sameTextError has errProofRuntime's text under another type.
type sameTextError string

func (e sameTextError) Error() string { return string(e) }

func TestIsProofRuntimeFailureShape(t *testing.T) {
	exact := &parserrt.OperationalError{Cause: errProofRuntime}
	if !IsProofRuntimeFailure(exact) {
		t.Fatal("exact callback shape not recognized")
	}
	if !IsProofRuntimeFailure(parserrt.BoundCallbackError(errProofRuntime)) {
		t.Fatal("BoundCallbackError wrapping not recognized")
	}
	if got := parserrt.ClassifyBoundError(exact); got != parserrt.BoundOperational {
		t.Fatalf("classification changed: %v", got)
	}
	text := errProofRuntime.Error()
	var nilOp *parserrt.OperationalError
	for name, err := range map[string]error{
		"nil":                       nil,
		"typed nil wrapper":         nilOp,
		"empty wrapper":             &parserrt.OperationalError{},
		"bare sentinel":             errProofRuntime,
		"same text errors.New":      &parserrt.OperationalError{Cause: errors.New(text)},
		"same text other type":      &parserrt.OperationalError{Cause: sameTextError(text)},
		"same full text":            errors.New(exact.Error()),
		"other support sentinel":    &parserrt.OperationalError{Cause: errReduceStringRuntime},
		"wrapped cause":             &parserrt.OperationalError{Cause: fmt.Errorf("x: %w", errProofRuntime)},
		"double wrapper":            &parserrt.OperationalError{Cause: exact},
		"outer wrap":                fmt.Errorf("x: %w", exact),
		"source wrapper":            &parserrt.SourceError{Cause: errProofRuntime},
		"source over callback":      &parserrt.SourceError{Cause: exact},
		"rejection cause":           parserrt.NewBindingRejection(text, lexer.Position{}, errProofRuntime),
		"joined with depth":         errors.Join(exact, &recursion.DepthError{Stage: "parser", Limit: 1}),
		"depth over callback cause": &parserrt.OperationalError{Cause: &recursion.DepthError{Stage: "parser", Limit: 1}},
	} {
		if IsProofRuntimeFailure(err) {
			t.Errorf("%s: recognized %v", name, err)
		}
	}
}

func TestProofRuntimeRawParse(t *testing.T) {
	result, err := Parse(context.Background(), []byte(proofOverflow), ParseOptions{})
	if err == nil || !IsProofRuntimeFailure(err) || err.Error() != "binding callback: sany proof: Java runtime failure" {
		t.Fatalf("raw parse error changed: %#v", err)
	}
	if result.Accepted || result.Tree != nil || len(result.Errors) != 0 || result.Failure != nil {
		t.Fatalf("raw parse result changed: %+v", result)
	}
	_, err = Parse(context.Background(), []byte(proofIntMax), ParseOptions{})
	if IsProofRuntimeFailure(err) {
		t.Fatalf("int max level recognized: %v", err)
	}
}

func TestProofRuntimeDepthPrecedes(t *testing.T) {
	_, err := Parse(context.Background(), []byte(proofOverflow), ParseOptions{Recursion: recursion.Options{MaxDepth: 2}})
	var depth *recursion.DepthError
	if !errors.As(err, &depth) || IsProofRuntimeFailure(err) {
		t.Fatalf("depth provenance changed: %#v", err)
	}
}

// countdownContext reports cancellation from its n+1st Err call on.
type countdownContext struct {
	context.Context
	left atomic.Int64
}

func newCountdown(n int64) *countdownContext {
	c := &countdownContext{Context: context.Background()}
	c.left.Store(n)
	return c
}
func (c *countdownContext) Err() error {
	if c.left.Add(-1) >= 0 {
		return nil
	}
	return context.Canceled
}

func TestProofRuntimeInterruptionNotRecognized(t *testing.T) {
	interrupted := 0
	for n := int64(0); ; n++ {
		if n > 100000 {
			t.Fatal("parse never completed")
		}
		_, err := Parse(newCountdown(n), []byte(proofOverflow), ParseOptions{})
		if errors.Is(err, context.Canceled) {
			if IsProofRuntimeFailure(err) {
				t.Fatalf("n=%d: interruption recognized: %v", n, err)
			}
			interrupted++
			continue
		}
		if !IsProofRuntimeFailure(err) {
			t.Fatalf("n=%d: %#v", n, err)
		}
		break
	}
	if interrupted == 0 {
		t.Fatal("no interrupted parse observed")
	}
}
