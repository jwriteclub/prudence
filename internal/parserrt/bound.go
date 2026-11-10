// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package parserrt

import (
	"context"
	"errors"
	"fmt"

	"github.com/jwriteclub/prudence/internal/lexer"
	"github.com/jwriteclub/prudence/internal/recursion"
)

// BindingRejection explicitly represents a translated Java ParseException.
// Position is the caller-supplied, already-known location; a zero position
// means that no location was supplied. Constructing it never reads a token.
type BindingRejection struct {
	Message  string
	Position lexer.Position
	Cause    error
}

// NewBindingRejection preserves the supplied message, location and optional cause.
func NewBindingRejection(message string, position lexer.Position, cause error) *BindingRejection {
	return &BindingRejection{Message: message, Position: position, Cause: cause}
}
func (e *BindingRejection) Error() string { return e.Message }
func (e *BindingRejection) Unwrap() error { return e.Cause }

// OperationalError marks an error originating from translated callback code.
type OperationalError struct{ Cause error }

func (e *OperationalError) Error() string {
	if e.Cause == nil {
		return "binding callback failed"
	}
	return "binding callback: " + e.Cause.Error()
}
func (e *OperationalError) Unwrap() error { return e.Cause }

// SourceError marks an error from a bound parser's token source.
type SourceError struct{ Cause error }

func (e *SourceError) Error() string {
	if e.Cause == nil {
		return "bound token source failed"
	}
	return "bound token source: " + e.Cause.Error()
}
func (e *SourceError) Unwrap() error { return e.Cause }

type boundSentinel string

func (e boundSentinel) Error() string { return string(e) }

// ErrNilFrame indicates a missing entry frame.
const ErrNilFrame boundSentinel = "bound parser: nil entry frame"

// ErrNilAssignmentCommit indicates a prepared assignment returned no commit callback.
const ErrNilAssignmentCommit boundSentinel = "bound parser: nil assignment commit"

// BoundCallbackError retains explicit rejection only in the absence of stronger
// operational, interruption, or depth provenance.
func BoundCallbackError(err error) error {
	if err == nil {
		return nil
	}
	var source *SourceError
	var op *OperationalError
	var interrupted *InterruptionError
	var depth *recursion.DepthError
	if errors.As(err, &source) || errors.As(err, &op) || errors.As(err, &interrupted) || errors.As(err, &depth) {
		return err
	}
	var rejection *BindingRejection
	if errors.As(err, &rejection) {
		return err
	}
	return &OperationalError{Cause: err}
}

type boundSource struct{ source TokenSource }

func (s boundSource) NextToken() (*lexer.Token, error) {
	if s.source == nil {
		return nil, &SourceError{Cause: fmt.Errorf("nil token source")}
	}
	t, err := s.source.NextToken()
	if err != nil {
		return nil, &SourceError{Cause: err}
	}
	return t, nil
}

// BoundSource adds source provenance without changing recognizer behavior.
func BoundSource(source TokenSource) TokenSource { return boundSource{source} }

func (s boundSource) NextTokenContext(ctx context.Context) (*lexer.Token, error) {
	if s.source == nil {
		return nil, &SourceError{Cause: fmt.Errorf("nil token source")}
	}
	if source, ok := s.source.(contextTokenSource); ok {
		t, err := source.NextTokenContext(ctx)
		if err != nil {
			return nil, &SourceError{Cause: err}
		}
		return t, nil
	}
	return s.NextToken()
}

// BoundErrorKind classifies the outer provenance of a bound-parser failure.
type BoundErrorKind uint8

const (
	BoundOK BoundErrorKind = iota
	BoundOperational
	BoundInterrupted
	BoundDepth
	BoundRejected
)

// ClassifyBoundError gives operational/source, interruption and depth provenance
// priority over any nested foreign parser errors or binding rejections.
func ClassifyBoundError(err error) BoundErrorKind {
	if err == nil {
		return BoundOK
	}
	var interrupted *InterruptionError
	if errors.As(err, &interrupted) {
		return BoundInterrupted
	}
	var depth *recursion.DepthError
	if errors.As(err, &depth) {
		return BoundDepth
	}
	var source *SourceError
	var op *OperationalError
	if errors.As(err, &source) || errors.As(err, &op) {
		return BoundOperational
	}
	var rejected *BindingRejection
	var mismatch *ParseError
	if errors.As(err, &rejected) || errors.As(err, &mismatch) {
		return BoundRejected
	}
	return BoundOperational
}
