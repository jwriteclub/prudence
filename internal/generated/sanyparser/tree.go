// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Derived from TLA+ SyntaxTreeNode.java, Copyright (c) 2003 Compaq
// Corporation, distributed under the MIT license. See NOTICE.
package generated

import (
	"errors"
	"math"
	"unicode/utf16"

	"github.com/jwriteclub/prudence/internal/lexer"
)

// SyntaxTreeNode holds parser-visible tree data. Child partitions retain the
// supplied slices, as the Java constructors retain their arrays.
type SyntaxTreeNode struct {
	context              *SupportContext
	zero, one            []*SyntaxTreeNode
	kind                 int
	image, originalImage *UniqueString
	location             [4]int32
	fileName             *UniqueString
	preComment           []*UniqueString
	proofLevel           int32
}

func (ctx *SupportContext) NewEmptySyntaxTreeNode() *SyntaxTreeNode {
	return &SyntaxTreeNode{context: ctx, zero: ctx.emptyNodes, one: ctx.emptyNodes, preComment: ctx.emptyComments, proofLevel: -1}
}
func (ctx *SupportContext) NewBuiltinSyntaxTreeNode(image *UniqueString) *SyntaxTreeNode {
	n := ctx.NewEmptySyntaxTreeNode()
	n.image = image
	n.fileName = ctx.UniqueStringOf("--TLA+ BUILTINS--")
	return n
}

// tokenUnits follows the runtime's image convention: exact units win when
// their decoded view matches Image; otherwise Image was rewritten directly.
func tokenUnits(t *lexer.Token) []uint16 {
	if string(utf16.Decode(t.ImageUnits)) == t.Image {
		return t.ImageUnits
	}
	return utf16.Encode([]rune(t.Image))
}
func tokenLocation(t *lexer.Token) ([4]int32, error) {
	values := [4]int{t.Begin.Line, t.Begin.Column, t.End.Line, t.End.Column}
	var out [4]int32
	for i, x := range values {
		if int64(x) < math.MinInt32 || int64(x) > math.MaxInt32 {
			return out, errors.New("sanyparser: token coordinate outside Java int range")
		}
		out[i] = int32(x)
	}
	return out, nil
}

// NewTokenNode uses the token's kind; NewTokenNodeKind uses an explicit kind.
func (ctx *SupportContext) NewTokenNode(fn *UniqueString, t *lexer.Token) (*SyntaxTreeNode, error) {
	if t == nil {
		return nil, errors.New("sanyparser: nil token")
	}
	return ctx.NewTokenNodeKind(fn, t.Kind, t)
}
func (ctx *SupportContext) NewTokenNodeKind(fn *UniqueString, kind int, t *lexer.Token) (*SyntaxTreeNode, error) {
	if t == nil {
		return nil, errors.New("sanyparser: nil token")
	}
	loc, err := tokenLocation(t)
	if err != nil {
		return nil, err
	}
	// Validate the entire backwards chain before changing any Next links.
	seen := make(map[*lexer.Token]bool)
	var chain []*lexer.Token
	for p := t.SpecialToken; p != nil; p = p.SpecialToken {
		if seen[p] {
			return nil, errors.New("sanyparser: cyclic special-token chain")
		}
		seen[p] = true
		chain = append(chain, p)
	}
	n := ctx.NewEmptySyntaxTreeNode()
	n.kind = kind
	n.image = ctx.UniqueStringOfUnits(tokenUnits(t))
	n.location = loc
	n.fileName = fn
	if len(chain) != 0 {
		n.preComment = make([]*UniqueString, len(chain))
		next := ctx.commentSentinel
		for i, p := range chain {
			p.Next = next
			next = p
			n.preComment[len(chain)-1-i] = ctx.UniqueStringOfUnits(tokenUnits(p))
		}
	}
	return n, nil
}
func (ctx *SupportContext) newNode(fn *UniqueString, kind int, zero, one []*SyntaxTreeNode) (*SyntaxTreeNode, error) {
	image, err := ctx.SyntaxNodeImage(kind)
	if err != nil {
		return nil, err
	}
	n := &SyntaxTreeNode{kind: kind, image: image, zero: zero, one: one, fileName: fn, context: ctx, preComment: ctx.emptyComments, proofLevel: -1, location: [4]int32{math.MaxInt32, math.MaxInt32, math.MinInt32, math.MinInt32}}
	for _, group := range [][]*SyntaxTreeNode{zero, one} {
		for _, c := range group {
			if c == nil {
				return nil, errors.New("sanyparser: nil child")
			}
			l := c.location
			if l[0] < n.location[0] || (l[0] == n.location[0] && l[1] < n.location[1]) {
				n.location[0], n.location[1] = l[0], l[1]
			}
			if l[2] > n.location[2] || (l[2] == n.location[2] && l[3] > n.location[3]) {
				n.location[2], n.location[3] = l[2], l[3]
			}
		}
	}
	return n, nil
}
func (ctx *SupportContext) NewNode(fn *UniqueString, kind int, children []*SyntaxTreeNode) (*SyntaxTreeNode, error) {
	return ctx.newNode(fn, kind, children, nil)
}
func (ctx *SupportContext) NewNodeInherit(kind int, children []*SyntaxTreeNode) (*SyntaxTreeNode, error) {
	if len(children) == 0 || children[0] == nil {
		return nil, errors.New("sanyparser: inherited filename requires first child")
	}
	return ctx.newNode(children[0].fileName, kind, children, nil)
}
func (ctx *SupportContext) NewNodeOptional(fn *UniqueString, kind int, optional *SyntaxTreeNode, children []*SyntaxTreeNode) (*SyntaxTreeNode, error) {
	var zero []*SyntaxTreeNode
	if optional != nil {
		zero = []*SyntaxTreeNode{optional}
	}
	return ctx.newNode(fn, kind, zero, children)
}
func (ctx *SupportContext) NewNodeParts(fn *UniqueString, kind int, zero, one []*SyntaxTreeNode) (*SyntaxTreeNode, error) {
	return ctx.newNode(fn, kind, zero, one)
}
func (ctx *SupportContext) NewNodePair(kind int, a, b *SyntaxTreeNode) (*SyntaxTreeNode, error) {
	if a == nil {
		return nil, errors.New("sanyparser: inherited filename requires first child")
	}
	return ctx.newNode(a.fileName, kind, []*SyntaxTreeNode{a, b}, nil)
}
func (ctx *SupportContext) NewNodeTriple(kind int, a, b, c *SyntaxTreeNode) (*SyntaxTreeNode, error) {
	if a == nil {
		return nil, errors.New("sanyparser: inherited filename requires first child")
	}
	return ctx.newNode(a.fileName, kind, []*SyntaxTreeNode{a, b, c}, nil)
}
func (n *SyntaxTreeNode) Kind() int                            { return n.kind }
func (n *SyntaxTreeNode) SetKind(kind int)                     { n.kind = kind }
func (n *SyntaxTreeNode) IsKind(kind int) bool                 { return n.kind == kind }
func (n *SyntaxTreeNode) Image() *UniqueString                 { return n.image }
func (n *SyntaxTreeNode) OriginalImage() *UniqueString         { return n.originalImage }
func (n *SyntaxTreeNode) SetOriginalImage(image *UniqueString) { n.originalImage = image }
func (n *SyntaxTreeNode) Filename() *UniqueString              { return n.fileName }
func (n *SyntaxTreeNode) Location() *Location {
	if n.context == nil {
		return &Location{source: n.fileName, bl: n.location[0], bc: n.location[1], el: n.location[2], ec: n.location[3]}
	}
	return n.context.NewLocation(n.fileName, n.location[0], n.location[1], n.location[2], n.location[3])
}
func (n *SyntaxTreeNode) ProofLevel() int32         { return n.proofLevel }
func (n *SyntaxTreeNode) SetProofLevel(level int32) { n.proofLevel = level }
func copyNodes(v []*SyntaxTreeNode) []*SyntaxTreeNode {
	if v == nil {
		return nil
	}
	result := make([]*SyntaxTreeNode, len(v))
	copy(result, v)
	return result
}
func (n *SyntaxTreeNode) Zero() []*SyntaxTreeNode { return copyNodes(n.zero) }
func (n *SyntaxTreeNode) One() []*SyntaxTreeNode  { return copyNodes(n.one) }
func (n *SyntaxTreeNode) Heirs() []*SyntaxTreeNode {
	result := make([]*SyntaxTreeNode, 0, len(n.zero)+len(n.one))
	result = append(result, n.zero...)
	return append(result, n.one...)
}
func (n *SyntaxTreeNode) PreComments() []*UniqueString {
	result := make([]*UniqueString, len(n.preComment))
	copy(result, n.preComment)
	return result
}

// nodePosition carries the same source position used by SANY messages.
func nodePosition(n *SyntaxTreeNode) lexer.Position {
	if n == nil {
		return lexer.Position{}
	}
	return lexer.Position{Line: int(n.location[0]), Column: int(n.location[1])}
}
