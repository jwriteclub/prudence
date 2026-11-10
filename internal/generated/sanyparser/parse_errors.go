// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Derived from TLA+ ParseError.java and ParseErrors.java, Copyright (c) 2003
// Compaq Corporation, distributed under the MIT license. See NOTICE.
package generated

import "github.com/jwriteclub/prudence/internal/lexer"

type ParseError struct {
	custom, backup *UniqueString
	position       lexer.Position
}

func (e *ParseError) Position() lexer.Position { return e.position }
func positionedError(e *ParseError, n *SyntaxTreeNode) *ParseError {
	e.position = nodePosition(n)
	return e
}

func NewParseError(custom, backup *UniqueString) *ParseError {
	return &ParseError{custom: custom, backup: backup}
}
func (ctx *SupportContext) NewSimpleParseError(custom *UniqueString) *ParseError {
	return NewParseError(custom, ctx.UniqueStringOf(""))
}
func (e *ParseError) ReportedError() *UniqueString { return e.custom }
func (e *ParseError) DefaultError() *UniqueString  { return e.backup }

type ParseErrors struct{ entries []*ParseError }

func NewParseErrors() *ParseErrors        { return &ParseErrors{} }
func (e *ParseErrors) Empty() bool        { return len(e.entries) == 0 }
func (e *ParseErrors) Push(p *ParseError) { e.entries = append(e.entries, p) }
func (e *ParseErrors) Errors() []*ParseError {
	r := make([]*ParseError, len(e.entries))
	copy(r, e.entries)
	return r
}
