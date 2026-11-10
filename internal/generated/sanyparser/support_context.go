// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package generated

import "github.com/jwriteclub/prudence/internal/lexer"

type supportError string

func (e supportError) Error() string { return string(e) }

// SupportContext is one SANY identity domain. Do not copy it after use.
// It contains no recognizer, logger, input, token source, or parser class state.
type SupportContext struct {
	Interner
	operators          map[*UniqueString]*Operator
	voidOperator       *Operator
	unknownSource      *UniqueString
	nullLocation       *Location
	nullSyntaxTreeNode *SyntaxTreeNode
	commentSentinel    *lexer.Token
	emptyNodes         []*SyntaxTreeNode
	emptyComments      []*UniqueString
}

// NewSupportContext builds identity-bearing values from private immutable
// definitions. No token fetches or user callbacks occur during initialization.
func NewSupportContext() *SupportContext {
	ctx := &SupportContext{
		operators:       make(map[*UniqueString]*Operator),
		commentSentinel: &lexer.Token{},
		emptyNodes:      []*SyntaxTreeNode{},
		emptyComments:   []*UniqueString{},
	}
	for _, d := range canonicalOperatorData {
		id := ctx.UniqueStringOf(d.name)
		ctx.operators[id] = &Operator{id, d.low, d.high, d.assoc, d.fix}
	}
	for _, group := range operatorSynonyms {
		op := ctx.operators[ctx.UniqueStringOf(group[0])]
		for _, name := range group[1:] {
			ctx.operators[ctx.UniqueStringOf(name)] = op
		}
	}
	ctx.voidOperator = &Operator{ctx.UniqueStringOf("$$_void"), 0, 0, assocNone, infix}
	ctx.unknownSource = ctx.UniqueStringOf("--unknown--")
	ctx.nullLocation = ctx.NewLocation(ctx.unknownSource, 0, 0, 0, 0)
	ctx.nullSyntaxTreeNode = ctx.NewBuiltinSyntaxTreeNode(ctx.UniqueStringOf("***I do not exist***"))
	return ctx
}

func (ctx *SupportContext) VoidOperator() *Operator             { return ctx.voidOperator }
func (ctx *SupportContext) NullLocation() *Location             { return ctx.nullLocation }
func (ctx *SupportContext) NullSyntaxTreeNode() *SyntaxTreeNode { return ctx.nullSyntaxTreeNode }
