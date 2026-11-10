// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Package parserrt supports experimental generated one-shot recognizers.
package parserrt

import (
	"context"
	"errors"
	"fmt"

	"github.com/jwriteclub/prudence/internal/lexer"
	"github.com/jwriteclub/prudence/internal/recursion"
)

// DefaultMaxDepth is the default active production and scanner call limit.
const DefaultMaxDepth = 12000

// TokenSource supplies ordinary tokens in order. Special tokens remain linked
// to their ordinary token and are never consumed separately by the parser.
type TokenSource interface{ NextToken() (*lexer.Token, error) }

type contextTokenSource interface {
	NextTokenContext(context.Context) (*lexer.Token, error)
}

// InterruptionError distinguishes cooperative cancellation from parse rejection.
type InterruptionError struct{ cause error }

func (e *InterruptionError) Error() string { return "parser interrupted: " + e.cause.Error() }
func (e *InterruptionError) Unwrap() error { return e.cause }

// ParseError reports an ordinary selected-entry recognizer mismatch. Its text
// is not a JavaCC diagnostic compatibility guarantee.
type ParseError struct {
	message    string
	Diagnostic *Diagnostic
}

func (e *ParseError) Error() string { return e.message }

// Event snapshots mutable token fields at the instant a trace event is emitted.
type Event struct {
	Kind, Production, Image string
	Origin                  string
	Depth, TokenKind        int
	ExpectedKind            int
	Begin, End              lexer.Position
}
type Sink func(Event)

type Cursor struct {
	source      TokenSource
	current     *lexer.Token
	sink        Sink
	depth       int
	ctx         context.Context
	interrupted error
	depthLimit  int
	activeCalls int
	depthError  *recursion.DepthError
	diagnostic  *diagnosticState
	sharedScan  *Scan
}

func New(source TokenSource, sink Sink) *Cursor {
	c, _ := NewWithOptions(source, sink, recursion.Options{})
	return c
}

// NewWithOptions copies the per-invocation active production/scanner limit.
func NewWithOptions(source TokenSource, sink Sink, options recursion.Options) (*Cursor, error) {
	if options.MaxDepth == 0 {
		options.MaxDepth = DefaultMaxDepth
	}
	limit, err := options.Limit()
	if err != nil {
		return nil, err
	}
	return &Cursor{source: source, current: &lexer.Token{}, sink: sink, depthLimit: limit}, nil
}

// NewContext creates a cursor for one cooperative, non-resumable invocation.
func NewContext(ctx context.Context, source TokenSource, sink Sink) (*Cursor, error) {
	return NewContextWithOptions(ctx, source, sink, recursion.Options{})
}

// NewContextWithOptions creates a non-resumable invocation with a copied limit.
func NewContextWithOptions(ctx context.Context, source TokenSource, sink Sink, options recursion.Options) (*Cursor, error) {
	if ctx == nil {
		return nil, fmt.Errorf("parser context must not be nil")
	}
	c, err := NewWithOptions(source, sink, options)
	if err != nil {
		return nil, err
	}
	c.ctx = ctx
	return c, nil
}

// Check observes cancellation and makes an observed interruption sticky.
func (c *Cursor) Check() error {
	if c.interrupted != nil {
		return c.interrupted
	}
	if c.depthError != nil {
		return c.depthError
	}
	if c.ctx != nil {
		if err := c.ctx.Err(); err != nil {
			c.interrupted = &InterruptionError{cause: err}
			return c.interrupted
		}
	}
	return nil
}

// EnterDepth bounds active generated production and scanner calls together.
// It never fetches a token to obtain a diagnostic position.
func (c *Cursor) EnterDepth(rule string) error { return c.enterDepth(rule, c.current) }

func (c *Cursor) enterDepth(rule string, at *lexer.Token) error {
	if err := c.Check(); err != nil {
		return err
	}
	if c.activeCalls >= c.depthLimit {
		line, column := 1, 1
		if at != nil && at.Begin.Line > 0 && at.Begin.Column > 0 {
			line, column = at.Begin.Line, at.Begin.Column
		}
		c.depthError = &recursion.DepthError{Stage: "parser", Rule: rule, Offset: -1, Line: line, Column: column, Limit: c.depthLimit}
		return c.depthError
	}
	c.activeCalls++
	return nil
}

// LeaveDepth releases one successfully entered generated call.
func (c *Cursor) LeaveDepth() {
	if c.activeCalls > 0 {
		c.activeCalls--
	}
}
func (c *Cursor) Enter(name string) {
	if c.sink != nil {
		c.sink(Event{Kind: "call", Production: name, Depth: c.depth})
	}
	c.depth += 2
}
func (c *Cursor) Leave(name string) {
	c.depth -= 2
	if c.sink != nil {
		c.sink(Event{Kind: "return", Production: name, Depth: c.depth})
	}
}

// next materializes one linked token without moving the consumed cursor.
func (c *Cursor) next(from *lexer.Token) (*lexer.Token, error) {
	if err := c.Check(); err != nil {
		return nil, err
	}
	if from == nil {
		return nil, fmt.Errorf("parser: nil cursor token")
	}
	if from.Next == nil {
		if c.source == nil {
			return nil, fmt.Errorf("parser: nil token source")
		}
		var t *lexer.Token
		var err error
		if contextual, ok := c.source.(contextTokenSource); ok && c.ctx != nil {
			t, err = contextual.NextTokenContext(c.ctx)
		} else {
			t, err = c.source.NextToken()
		}
		if err != nil {
			if c.ctx != nil && c.ctx.Err() != nil && errors.Is(err, c.ctx.Err()) {
				return nil, c.Check()
			}
			return nil, err
		}
		if t == nil {
			return nil, fmt.Errorf("parser: token source returned nil")
		}
		if err := c.Check(); err != nil {
			return nil, err
		}
		from.Next = t
	}
	return from.Next, nil
}

// Peek materializes at most one token and never moves the consumed cursor.
func (c *Cursor) Peek() (*lexer.Token, error) { return c.next(c.current) }

// NextAfter follows or materializes one link from an arbitrary token without
// moving either parser or scan position or spending lookahead budget.
func (c *Cursor) NextAfter(from *lexer.Token) (*lexer.Token, error) { return c.next(from) }

// ScanOutcome distinguishes a successful budget boundary from a mismatch.
type ScanOutcome uint8

const (
	ScanMatched ScanOutcome = iota
	ScanMismatched
	ScanBudgetSatisfied
)

// Scan tracks prediction independently of the parser's consumed position.
// Resetting Position for an alternative does not reset Furthest or Budget.
type Scan struct {
	cursor   *Cursor
	position *lexer.Token
	furthest *lexer.Token
	budget   int
}

// StartScan begins prediction at the current consumed token.
func (c *Cursor) StartScan(budget int) (*Scan, error) {
	if err := c.Check(); err != nil {
		return nil, err
	}
	if budget <= 0 {
		return nil, fmt.Errorf("parser: scan budget must be positive")
	}
	return &Scan{cursor: c, position: c.current, furthest: c.current, budget: budget}, nil
}

// StartGeneratedScan resets JavaCC's shared phase-two scanner globals in place.
// StartScan remains independent for callers using the manual runtime API.
func (c *Cursor) StartGeneratedScan(budget int) (*Scan, error) {
	if err := c.Check(); err != nil {
		return nil, err
	}
	if budget <= 0 {
		return nil, fmt.Errorf("parser: scan budget must be positive")
	}
	if c.sharedScan == nil {
		c.sharedScan = &Scan{cursor: c}
	}
	c.resetGeneratedScan(c.current, budget)
	return c.sharedScan, nil
}

func (c *Cursor) resetGeneratedScan(anchor *lexer.Token, budget int) *Scan {
	if c.sharedScan == nil {
		c.sharedScan = &Scan{cursor: c}
	}
	c.sharedScan.position = anchor
	c.sharedScan.furthest = anchor
	c.sharedScan.budget = budget
	return c.sharedScan
}

// Mark returns the current scan position for a later alternative.
func (s *Scan) Mark() *lexer.Token { return s.position }

// Check observes interruption at scan control-flow boundaries.
func (s *Scan) Check() error { return s.cursor.Check() }

// EnterDepth bounds a generated scanner using its current scan position.
func (s *Scan) EnterDepth(rule string) error { return s.cursor.enterDepth(rule, s.position) }

// LeaveDepth releases one successfully entered generated scanner.
func (s *Scan) LeaveDepth() { s.cursor.LeaveDepth() }

// EnterProduction records a generated lookahead routine's debug entry.
func (s *Scan) EnterProduction(name string) {
	if s.cursor.diagnostic != nil && s.cursor.diagnostic.active {
		return
	}
	if s.cursor.sink != nil {
		s.cursor.sink(Event{Kind: "scan-call", Production: name, Depth: s.cursor.depth})
	}
	s.cursor.depth += 2
}

// LeaveProduction records a normal lookahead return. A satisfied budget
// unwinds generated Java routines without executing their return trace.
func (s *Scan) LeaveProduction(name string, outcome ScanOutcome) {
	if s.cursor.diagnostic != nil && s.cursor.diagnostic.active {
		return
	}
	s.cursor.depth -= 2
	if s.cursor.sink != nil {
		kind := "scan-return-success"
		if outcome == ScanMismatched {
			kind = "scan-return-failure"
		}
		s.cursor.sink(Event{Kind: kind, Production: name, Depth: s.cursor.depth})
	}
}

// Restore changes only the scan position; the furthest position and budget persist.
func (s *Scan) Restore(mark *lexer.Token) error {
	if err := s.Check(); err != nil {
		return err
	}
	if mark == nil {
		return fmt.Errorf("parser: nil scan mark")
	}
	s.position = mark
	return nil
}

// Token advances a scan once, observing a mismatch before budget success.
func (s *Scan) Token(kind int) (ScanOutcome, error) {
	if err := s.Check(); err != nil {
		return ScanMismatched, err
	}
	extending := s.position == s.furthest
	if extending {
		if s != s.cursor.sharedScan && s.budget <= 0 {
			return ScanMismatched, fmt.Errorf("parser: exhausted scan budget was not propagated")
		}
		if s == s.cursor.sharedScan {
			s.budget = int(int32(s.budget) - 1)
		} else {
			s.budget--
		}
	}
	t, err := s.cursor.next(s.position)
	if err != nil {
		return ScanMismatched, err
	}
	s.position = t
	if extending {
		s.furthest = t
	}
	outcome, err := s.finishToken(t, kind)
	if err != nil {
		return ScanMismatched, err
	}
	if err := s.Check(); err != nil {
		return ScanMismatched, err
	}
	return outcome, nil
}

func (s *Scan) finishToken(t *lexer.Token, kind int) (ScanOutcome, error) {
	if d := s.cursor.diagnostic; d != nil && d.active {
		pos := 0
		for tok := s.cursor.current; tok != nil; tok = tok.Next {
			if err := s.Check(); err != nil {
				return ScanMismatched, err
			}
			if tok == t {
				d.add(kind, pos)
				break
			}
			pos++
		}
	} else if s.cursor.sink != nil {
		s.cursor.sink(Event{Kind: "scan", Depth: s.cursor.depth, TokenKind: t.Kind, ExpectedKind: kind, Image: t.Image, Begin: t.Begin, End: t.End})
	}
	if t.Kind != kind {
		return ScanMismatched, nil
	}
	if s.budget == 0 && s.position == s.furthest {
		return ScanBudgetSatisfied, nil
	}
	return ScanMatched, nil
}

// Consume commits only a matching token; a mismatch retains the fetched link.
func (c *Cursor) Consume(kind int) error {
	_, err := c.ConsumeToken(kind)
	return err
}

// ConsumeToken consumes a matching token and returns its original identity.
// On mismatch or any error it returns no token.
func (c *Cursor) ConsumeToken(kind int) (*lexer.Token, error) {
	t, err := c.Peek()
	if err != nil {
		return nil, err
	}
	if t.Kind != kind {
		return nil, c.diagnosticError(kind, fmt.Sprintf("parser: expected token kind %d, got %d at %d:%d", kind, t.Kind, t.Begin.Line, t.Begin.Column))
	}
	c.current = t
	c.advanceGeneration(true)
	if c.sink != nil {
		c.sink(Event{Kind: "consume", Depth: c.depth, TokenKind: t.Kind, Image: t.Image, Begin: t.Begin, End: t.End})
	}
	if err := c.Check(); err != nil {
		return nil, err
	}
	return t, nil
}

// NextToken advances the ordinary consumed position without checking a kind,
// including when called during a scan predicate.
func (c *Cursor) NextToken() (*lexer.Token, error) {
	t, err := c.Peek()
	if err != nil {
		return nil, err
	}
	c.current = t
	c.advanceGeneration(false)
	if c.sink != nil {
		c.sink(Event{Kind: "consume", Origin: "getNextToken", Depth: c.depth, TokenKind: t.Kind, Image: t.Image, Begin: t.Begin, End: t.End})
	}
	if err := c.Check(); err != nil {
		return nil, err
	}
	return t, nil
}
func (c *Cursor) Unexpected() error {
	t, err := c.Peek()
	if err != nil {
		return err
	}
	return c.diagnosticError(-1, fmt.Sprintf("parser: unexpected token kind %d at %d:%d", t.Kind, t.Begin.Line, t.Begin.Column))
}

// TokenAt follows the lazy token chain from the current consumed token, without
// moving the cursor. Nonpositive indices return the current token as JavaCC does.
func (c *Cursor) TokenAt(index int) (*lexer.Token, error) { return c.tokenAt(c.current, index) }

// TokenAt follows the lazy token chain from the scan cursor without moving it
// or spending lookahead budget.
func (s *Scan) TokenAt(index int) (*lexer.Token, error) { return s.cursor.tokenAt(s.position, index) }
func (c *Cursor) tokenAt(t *lexer.Token, index int) (*lexer.Token, error) {
	if err := c.Check(); err != nil {
		return nil, err
	}
	for i := 0; i < index; i++ {
		var err error
		t, err = c.next(t)
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}
