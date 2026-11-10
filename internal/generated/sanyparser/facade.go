// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
package generated

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sany "github.com/jwriteclub/prudence/internal/generated/sany"
	"github.com/jwriteclub/prudence/internal/lexer"
	"github.com/jwriteclub/prudence/internal/parserrt"
	"github.com/jwriteclub/prudence/internal/recursion"
)

// ParseOptions configures one invocation. A zero recursion limit uses
// parserrt.DefaultMaxDepth; positive values override it.
type ParseOptions struct {
	Logger    SANYLogger
	Sink      parserrt.Sink
	Recursion recursion.Options
}
type ParseResult struct {
	Accepted     bool
	Tree         *SyntaxTreeNode
	ModuleName   *UniqueString
	Dependencies []*UniqueString
	Errors       []*ParseError
	Failure      error // Original structured rejection, if any.
	PartialBody  []*SyntaxTreeNode
}

// The embedded concrete manager also exposes SwitchToDefault to callbacks.
type sanyOwnedLexer struct {
	*lexer.Lexer
	failures map[*lexer.LexicalError]struct{}
}

func (s *sanyOwnedLexer) record(err error) {
	if e, ok := err.(*lexer.LexicalError); ok {
		s.failures[e] = struct{}{}
	}
}
func (s *sanyOwnedLexer) NextToken() (*lexer.Token, error) {
	t, e := s.Lexer.NextToken()
	s.record(e)
	return t, e
}
func (s *sanyOwnedLexer) NextTokenContext(ctx context.Context) (*lexer.Token, error) {
	t, e := s.Lexer.NextTokenContext(ctx)
	s.record(e)
	return t, e
}

// Only the two known runtime wrappers may be stripped, and SourceError is
// mandatory. Pointer identity must match an error emitted by this manager.
func (s *sanyOwnedLexer) lexicalCause(err error) *lexer.LexicalError {
	source := false
	for err != nil {
		switch e := err.(type) {
		case *parserrt.SourceError:
			source = true
			err = e.Cause
		case *parserrt.OperationalError:
			err = e.Cause
		case *lexer.LexicalError:
			if source {
				if _, ok := s.failures[e]; ok {
					return e
				}
			}
			return nil
		default:
			return nil
		}
	}
	return nil
}
func (ctx *SupportContext) sanyLexicalMessage(e *lexer.LexicalError) (*UniqueString, error) {
	msg := e.Error()
	if strings.Contains(msg, "EOF") {
		if !e.HasStreamSpan {
			return nil, errors.New("sany: missing lexical stream span")
		}
		if e.StreamStart.Line != e.PostFailureEnd.Line {
			return ctx.UniqueStringOf(fmt.Sprintf("Lexical {error: EOF reached, possibly open comment starting around line %d", e.StreamStart.Line)), nil
		}
	}
	return ctx.UniqueStringOf(msg), nil
}
func sanyParseResult(s *SANYState) ParseResult {
	deps := make([]*UniqueString, len(s.dependencyList))
	copy(deps, s.dependencyList)
	return ParseResult{Tree: s.ParseTree, ModuleName: s.mn, Dependencies: deps, Errors: s.PErrors.Errors(), PartialBody: append([]*SyntaxTreeNode(nil), s.partialBody...)}
}

// Parse is a one-shot wrapper for CompilationUnit. Operational failures are
// returned separately; grammar and owned lexical failures are recorded.
func Parse(ctx context.Context, input []byte, options ParseOptions) (ParseResult, error) {
	if ctx == nil {
		return ParseResult{}, errors.New("sany: nil context")
	}
	source := &sanyOwnedLexer{Lexer: sany.New(string(input)), failures: make(map[*lexer.LexicalError]struct{})}
	var logger SANYLogger
	if options.Logger != nil {
		logger = func(level SANYLogLevel, message *UniqueString) error {
			if err := options.Logger(level, message); err != nil {
				return &parserrt.OperationalError{Cause: err}
			}
			return nil
		}
	}
	frame := &FrameCompilationUnit{Logger: logger}
	err := ParseBoundContext(ctx, source, options.Sink, frame, options.Recursion)
	state := frame.State
	if state == nil {
		if err != nil {
			return ParseResult{}, err
		}
		return ParseResult{}, errors.New("sany: missing parser state witness")
	}
	if err == nil {
		state.ParseTree = frame.Result
	} else {
		kind := parserrt.ClassifyBoundError(err)
		if kind == parserrt.BoundInterrupted || kind == parserrt.BoundDepth {
			return sanyParseResult(state), err
		}
		var message *UniqueString
		if lex := source.lexicalCause(err); lex != nil {
			var e error
			message, e = state.sanyLexicalMessage(lex)
			if e != nil {
				return sanyParseResult(state), e
			}
		} else if kind == parserrt.BoundRejected {
			var short []uint16
			var mismatch *parserrt.ParseError
			var rejection *parserrt.BindingRejection
			switch {
			case errors.As(err, &rejection):
				short = sanyAppendText(nil, rejection.Message)
			case errors.As(err, &mismatch):
				if mismatch.Diagnostic == nil {
					return sanyParseResult(state), errors.New("sany: missing mismatch diagnostic")
				}
				var e error
				short, e = sanyShortDiagnostic(mismatch.Diagnostic, ctx.Err)
				if e != nil {
					return sanyParseResult(state), e
				}
			default:
				return sanyParseResult(state), err
			}
			var e error
			message, e = state.sanyMessage(short, ctx.Err)
			if e != nil {
				return sanyParseResult(state), e
			}
		} else {
			return sanyParseResult(state), err
		}
		state.PErrors.Push(state.NewSimpleParseError(message))
	}
	result, finishErr := sanyFinish(ctx, state)
	result.Failure = err
	return result, finishErr
}

func sanyFinish(ctx context.Context, state *SANYState) (ParseResult, error) {
	if state.PErrors.Empty() {
		if state.heirsIndex != 0 {
			return sanyParseResult(state), errors.New("sany: parser heir-stack sanity assertion failed")
		}
	} else {
		for _, entry := range state.PErrors.Errors() {
			if e := ctx.Err(); e != nil {
				return sanyParseResult(state), e
			}
			if entry == nil {
				return sanyParseResult(state), errors.New("sany: nil parse error")
			}
			if state.out != nil {
				if e := state.out(SANYError, entry.ReportedError()); e != nil {
					return sanyParseResult(state), e
				}
			}
		}
	}
	result := sanyParseResult(state)
	result.Accepted = state.PErrors.Empty()
	return result, nil
}
