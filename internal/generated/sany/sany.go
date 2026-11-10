// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Package generated provides the generated SANY lexical tables and bindings.
package generated

import (
	"context"

	"github.com/jwriteclub/prudence/internal/internal/sanylex"
	"github.com/jwriteclub/prudence/internal/lexer"
)

// New creates a manager with fresh, instance-owned lexical action state.
func New(input string) *lexer.Lexer {
	counter := &sanylex.Counter{}
	l := lexer.New(Tables(), input)
	l.Action = counter.Action(StateNames())
	l.ResetAction = counter.Reset
	return l
}

// Editor owns a recovering lexer for one full-buffer scan at a time.
type Editor struct{ lexer *lexer.Lexer }

// NewEditor creates an opt-in recovering lexer with private action state.
func NewEditor(input string) *Editor {
	l := New(input)
	l.Recover = true
	return &Editor{lexer: l}
}

// NextToken returns the next regular, recovery, or EOF token.
func (e *Editor) NextToken() (*lexer.Token, error) { return e.lexer.NextToken() }

// NextTokenContext cooperatively interrupts a token request without emitting
// a recovery token or consuming the unfinished match.
func (e *Editor) NextTokenContext(ctx context.Context) (*lexer.Token, error) {
	return e.lexer.NextTokenContext(ctx)
}

// ReInit starts a fresh full-buffer scan without affecting any other manager.
func (e *Editor) ReInit(input string) {
	e.lexer.ReInit(input)
}

// InputCoordinates snapshots the lexer's UTF-16 positions and byte ranges.
func (e *Editor) InputCoordinates() ([]lexer.Position, [][2]int) {
	_, positions := e.lexer.InputUnits()
	return positions, e.lexer.InputByteRanges()
}
