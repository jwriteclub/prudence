// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Operator values and tables derive from TLA+ Operator.java and Operators.java,
// Copyright (c) 2003 Compaq Corporation, distributed under the MIT license.
package generated

import "errors"

const (
	assocNone  = 0
	assocLeft  = 1
	assocRight = 2
	nofix      = 0
	prefix     = 1
	postfix    = 2
	infix      = 3
	nfix       = 4
)
const (
	OperatorAssocNone  = assocNone
	OperatorAssocLeft  = assocLeft
	OperatorAssocRight = assocRight
	OperatorNoFix      = nofix
	OperatorPrefix     = prefix
	OperatorPostfix    = postfix
	OperatorInfix      = infix
	OperatorNFix       = nfix
)

// Operator holds immutable operator characteristics. Identity is significant.
type Operator struct {
	id                            *UniqueString
	low, high, associativity, fix int
}

// NewOperator constructs a value with the supplied Java characteristics.
func NewOperator(id *UniqueString, low, high, associativity, fix int) (*Operator, error) {
	if id == nil {
		return nil, errors.New("sanyparser: nil operator identifier")
	}
	return &Operator{id, low, high, associativity, fix}, nil
}
func (o *Operator) Identifier() *UniqueString {
	if o == nil {
		return nil
	}
	return o.id
}
func (o *Operator) Low() int {
	if o == nil {
		return 0
	}
	return o.low
}
func (o *Operator) High() int {
	if o == nil {
		return 0
	}
	return o.high
}
func (o *Operator) Associativity() int {
	if o == nil {
		return 0
	}
	return o.associativity
}
func (o *Operator) Fix() int {
	if o == nil {
		return 0
	}
	return o.fix
}
func (o *Operator) IsPrefix() bool  { return o != nil && o.fix == prefix }
func (o *Operator) IsInfix() bool   { return o != nil && (o.fix == infix || o.fix == nfix) }
func (o *Operator) IsPostfix() bool { return o != nil && o.fix == postfix }
func (o *Operator) IsNfix() bool    { return o != nil && o.fix == nfix }
func (o *Operator) IsPrefixDecl() bool {
	return o.IsPrefix() && len(o.id.units) > 0 && o.id.units[len(o.id.units)-1] == '.'
}
func (o *Operator) AssocLeft() bool  { return o != nil && o.associativity == assocLeft }
func (o *Operator) AssocRight() bool { return o != nil && o.associativity == assocRight }

// StringValue returns Java's description and whether the fix has a description.
func (o *Operator) StringValue() (string, bool) {
	if o == nil || o.id == nil {
		return "", false
	}
	names := [...]string{"nofix", "prefix", "postfix", "infix", "nfix"}
	if o.fix < 0 || o.fix >= len(names) {
		return "", false
	}
	return o.id.String() + ", " + names[o.fix], true
}
func operatorPair(a, b *Operator) error {
	if a == nil || b == nil {
		return errors.New("sanyparser: nil operator")
	}
	return nil
}
func Succ(a, b *Operator) (bool, error) {
	if e := operatorPair(a, b); e != nil {
		return false, e
	}
	return a.low > b.high, nil
}
func Prec(a, b *Operator) (bool, error) {
	if e := operatorPair(a, b); e != nil {
		return false, e
	}
	return a.high < b.low, nil
}
func SamePrec(a, b *Operator) (bool, error) {
	if e := operatorPair(a, b); e != nil {
		return false, e
	}
	return a.low == b.low && a.high == b.high, nil
}

type operatorData struct {
	name                  string
	low, high, assoc, fix int
}

// BEGIN PINNED TABLE
var canonicalOperatorData = []operatorData{
	{"[", 160, 160, assocLeft, postfix},
	{".", 170, 170, assocLeft, infix},
	{"'", 150, 150, assocNone, postfix},
	{"^", 140, 140, assocNone, infix},
	{"/", 130, 130, assocNone, infix},
	{"*", 130, 130, assocLeft, infix},
	{"-", 110, 110, assocLeft, infix},
	{"-.", 120, 120, assocNone, prefix},
	{"+", 100, 100, assocLeft, infix},
	{"=", 50, 50, assocNone, infix},
	{"\\lnot", 40, 40, assocLeft, prefix},
	{"\\land", 30, 30, assocLeft, infix},
	{"\\lor", 30, 30, assocLeft, infix},
	{"~>", 20, 20, assocNone, infix},
	{"=>", 10, 10, assocNone, infix},
	{"[]", 40, 150, assocNone, prefix},
	{"<>", 40, 150, assocNone, prefix},
	{"ENABLED", 40, 150, assocNone, prefix},
	{"UNCHANGED", 40, 150, assocNone, prefix},
	{"SUBSET", 100, 130, assocNone, prefix},
	{"UNION", 100, 130, assocNone, prefix},
	{"DOMAIN", 100, 130, assocNone, prefix},
	{"^+", 150, 150, assocNone, postfix},
	{"^*", 150, 150, assocNone, postfix},
	{"^#", 150, 150, assocNone, postfix},
	{"\\cdot", 50, 140, assocLeft, infix},
	{"\\equiv", 20, 20, assocNone, infix},
	{"-+->", 20, 20, assocNone, infix},
	{"/=", 50, 50, assocNone, infix},
	{"\\subseteq", 50, 50, assocNone, infix},
	{"\\in", 50, 50, assocNone, infix},
	{"\\notin", 50, 50, assocNone, infix},
	{"<", 50, 50, assocNone, infix},
	{"\\leq", 50, 50, assocNone, infix},
	{">", 50, 50, assocNone, infix},
	{"\\geq", 50, 50, assocNone, infix},
	{"\\times", 100, 130, assocLeft, nfix},
	{"\\", 80, 80, assocNone, infix},
	{"\\intersect", 80, 80, assocLeft, infix},
	{"\\union", 80, 80, assocLeft, infix},
	{"...", 90, 90, assocNone, infix},
	{"..", 90, 90, assocNone, infix},
	{"|", 100, 110, assocLeft, infix},
	{"||", 100, 110, assocLeft, infix},
	{"&&", 130, 130, assocLeft, infix},
	{"&", 130, 130, assocLeft, infix},
	{"$$", 90, 130, assocLeft, infix},
	{"$", 90, 130, assocLeft, infix},
	{"??", 90, 130, assocLeft, infix},
	{"%%", 100, 110, assocLeft, infix},
	{"%", 100, 110, assocNone, infix},
	{"##", 90, 130, assocLeft, infix},
	{"++", 100, 100, assocLeft, infix},
	{"--", 110, 110, assocLeft, infix},
	{"**", 130, 130, assocLeft, infix},
	{"//", 130, 130, assocNone, infix},
	{"^^", 140, 140, assocNone, infix},
	{"@@", 60, 60, assocLeft, infix},
	{"!!", 90, 130, assocNone, infix},
	{"|-", 50, 50, assocNone, infix},
	{"|=", 50, 50, assocNone, infix},
	{"-|", 50, 50, assocNone, infix},
	{"=|", 50, 50, assocNone, infix},
	{"<:", 70, 70, assocNone, infix},
	{":>", 70, 70, assocNone, infix},
	{":=", 50, 50, assocNone, infix},
	{"::=", 50, 50, assocNone, infix},
	{"\\oplus", 100, 100, assocLeft, infix},
	{"\\ominus", 110, 110, assocLeft, infix},
	{"\\odot", 130, 130, assocLeft, infix},
	{"\\oslash", 130, 130, assocNone, infix},
	{"\\otimes", 130, 130, assocLeft, infix},
	{"\\uplus", 90, 130, assocLeft, infix},
	{"\\sqcap", 90, 130, assocLeft, infix},
	{"\\sqcup", 90, 130, assocLeft, infix},
	{"\\div", 130, 130, assocNone, infix},
	{"\\wr", 90, 140, assocNone, infix},
	{"\\star", 130, 130, assocLeft, infix},
	{"\\o", 130, 130, assocLeft, infix},
	{"\\bigcirc", 130, 130, assocLeft, infix},
	{"\\bullet", 130, 130, assocLeft, infix},
	{"\\prec", 50, 50, assocNone, infix},
	{"\\succ", 50, 50, assocNone, infix},
	{"\\preceq", 50, 50, assocNone, infix},
	{"\\succeq", 50, 50, assocNone, infix},
	{"\\sim", 50, 50, assocNone, infix},
	{"\\simeq", 50, 50, assocNone, infix},
	{"\\ll", 50, 50, assocNone, infix},
	{"\\gg", 50, 50, assocNone, infix},
	{"\\asymp", 50, 50, assocNone, infix},
	{"\\subset", 50, 50, assocNone, infix},
	{"\\supset", 50, 50, assocNone, infix},
	{"\\supseteq", 50, 50, assocNone, infix},
	{"\\approx", 50, 50, assocNone, infix},
	{"\\cong", 50, 50, assocNone, infix},
	{"\\sqsubset", 50, 50, assocNone, infix},
	{"\\sqsubseteq", 50, 50, assocNone, infix},
	{"\\sqsupset", 50, 50, assocNone, infix},
	{"\\sqsupseteq", 50, 50, assocNone, infix},
	{"\\doteq", 50, 50, assocNone, infix},
	{"\\propto", 50, 50, assocNone, infix},
}

var operatorSynonyms = [][]string{
	{"\\lnot", "~", "\\neg", "¬"},
	{"[]", "□"},
	{"<>", "◇"},
	{"\\land", "/\\", "∧"},
	{"\\lor", "\\/", "∨"},
	{"\\equiv", "<=>", "≡", "⇔"},
	{"/=", "#", "≠"},
	{"\\leq", "<=", "=<", "≤"},
	{"\\geq", ">=", "≥"},
	{"\\times", "\\X", "×"},
	{"\\intersect", "\\cap", "∩"},
	{"\\union", "\\cup", "∪"},
	{"\\o", "\\circ", "∘"},
	{"\\oplus", "(+)", "⊕"},
	{"\\ominus", "(-)", "⊖"},
	{"\\odot", "(.)", "⊙"},
	{"\\oslash", "(/)", "⊘"},
	{"\\otimes", "(\\X)", "⊗"},
	{"\\approx", "≈"},
	{":=", "≔"},
	{"\\asymp", "≍"},
	{"\\bigcirc", "◯"},
	{"::=", "⩴"},
	{"\\bullet", "●"},
	{"\\cdot", "⋅"},
	{"\\cong", "≅"},
	{"\\div", "÷"},
	{"\\doteq", "≐"},
	{"..", "‥"},
	{"...", "…"},
	{"!!", "‼"},
	{"\\gg", "≫"},
	{"=>", "⇒"},
	{"\\in", "∈"},
	{"=|", "⫤"},
	{"~>", "↝"},
	{"\\ll", "≪"},
	{"-|", "⊣"},
	{"\\notin", "∉"},
	{"-+->", "⇸"},
	{"\\prec", "≺"},
	{"\\preceq", "⪯"},
	{"\\propto", "∝"},
	{"??", "⁇"},
	{"|=", "⊨"},
	{"|-", "⊢"},
	{"\\sim", "∼"},
	{"\\simeq", "≃"},
	{"\\sqcap", "⊓"},
	{"\\sqcup", "⊔"},
	{"\\sqsubset", "⊏"},
	{"\\sqsubseteq", "⊑"},
	{"\\sqsupset", "⊐"},
	{"\\sqsupseteq", "⊒"},
	{"\\star", "⋆"},
	{"\\subset", "⊂"},
	{"\\subseteq", "⊆"},
	{"\\succ", "≻"},
	{"\\succeq", "⪰"},
	{"\\supset", "⊃"},
	{"\\supseteq", "⊇"},
	{"\\uplus", "⊎"},
	{"||", "‖"},
	{"\\wr", "≀"},
	{"^+", "⁺"},
}

// END PINNED TABLE

// LookupOperator returns nil for an unknown name and an error for nil.
func (ctx *SupportContext) LookupOperator(name *UniqueString) (*Operator, error) {
	if name == nil {
		return nil, errors.New("sanyparser: nil operator name")
	}
	return ctx.operators[name], nil
}
func (ctx *SupportContext) ExistsOperator(name *UniqueString) (bool, error) {
	o, e := ctx.LookupOperator(name)
	return o != nil, e
}
func (ctx *SupportContext) ResolveSynonym(name *UniqueString) (*UniqueString, error) {
	o, e := ctx.LookupOperator(name)
	if e != nil {
		return nil, e
	}
	if o == nil {
		return name, nil
	}
	return o.id, nil
}

// GetMixfix returns a prefix unchanged, or the lookup of its identifier plus a dot.
func (ctx *SupportContext) GetMixfix(op *Operator) (*Operator, error) {
	if op == nil || op.id == nil {
		return nil, errors.New("sanyparser: nil operator")
	}
	if op.IsPrefix() {
		return op, nil
	}
	name, e := ctx.Concat(op.id, ctx.UniqueStringOf("."))
	if e != nil {
		return nil, e
	}
	return ctx.LookupOperator(name)
}
