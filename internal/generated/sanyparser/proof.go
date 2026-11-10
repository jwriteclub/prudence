// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
// Source SHA-256: bfc981f31c205333f478b773ab108159b5e2d339b659d33b0f920d56ebf4253d
package generated

import (
	"strconv"
	"unicode/utf16"

	"github.com/jwriteclub/prudence/internal/lexer"
	"github.com/jwriteclub/prudence/internal/parserrt"
)

const errProofRuntime supportError = "sany proof: Java runtime failure"

type ProofState struct {
	context         *SupportContext
	proofDepth      int32
	proofLevelStack [100]int32
}

func (ctx *SupportContext) NewProofState() *ProofState {
	return &ProofState{context: ctx, proofDepth: -1}
}
func (p *ProofState) at(i int32) (int32, error) {
	if i < 0 || i >= int32(len(p.proofLevelStack)) {
		return 0, errProofRuntime
	}
	return p.proofLevelStack[i], nil
}
func (p *ProofState) set(i, v int32) error {
	if i < 0 || i >= int32(len(p.proofLevelStack)) {
		return errProofRuntime
	}
	p.proofLevelStack[i] = v
	return nil
}
func (p *ProofState) Push() error {
	p.proofDepth++
	if p.proofDepth >= 100 {
		return parserrt.NewBindingRejection("Proofs nested more than 100deep.", lexer.Position{}, nil)
	}
	return p.set(p.proofDepth, -1)
}
func (p *ProofState) Pop() error {
	if p.proofDepth < 0 {
		return parserrt.NewBindingRejection("Parser bug: an extra QED step found.", lexer.Position{}, nil)
	}
	p.proofDepth--
	return nil
}
func (p *ProofState) Get() (int32, error) {
	if p.proofDepth < 0 {
		return p.proofDepth, nil
	}
	return p.at(p.proofDepth)
}
func proofUnits(t *lexer.Token) ([]uint16, error) {
	if t == nil {
		return nil, errProofRuntime
	}
	return tokenUnits(t), nil
}
func proofSubstring(s []uint16, a, b int) ([]uint16, error) {
	if a < 0 || b < a || b > len(s) {
		return nil, errProofRuntime
	}
	return s[a:b], nil
}
func proofGreaterIndex(s []uint16) int {
	for i, u := range s {
		if u == '>' {
			return i
		}
	}
	return -1
}

// Java Character.digit for UTF-16 char values accepted by Integer.parseInt.
func proofDigit(u uint16) int64 {
	for _, start := range javaDigitStarts {
		if u >= start && uint32(u) < uint32(start)+10 {
			return int64(u - start)
		}
	}
	return -1
}
func proofParseInt(s []uint16) (int32, error) {
	if len(s) == 0 {
		return 0, errProofRuntime
	}
	neg, i := false, 0
	if s[0] == '-' || s[0] == '+' {
		neg = s[0] == '-'
		i++
		if i == len(s) {
			return 0, errProofRuntime
		}
	}
	var n int64
	limit := int64(2147483647)
	if neg {
		limit++
	}
	for ; i < len(s); i++ {
		d := proofDigit(s[i])
		if d < 0 || n > (limit-d)/10 {
			return 0, errProofRuntime
		}
		n = n*10 + d
	}
	if neg {
		n = -n
	}
	return int32(n), nil
}
func LevelOfProofStepLexeme(t *lexer.Token) (int32, error) {
	s, err := proofUnits(t)
	if err != nil {
		return 0, err
	}
	c, err := proofSubstring(s, 1, 2)
	if err != nil {
		return 0, err
	}
	if c[0] == '*' {
		return -1, nil
	}
	if c[0] == '+' {
		return -2, nil
	}
	n, err := proofSubstring(s, 1, proofGreaterIndex(s))
	if err != nil {
		return 0, err
	}
	return proofParseInt(n)
}
func (p *ProofState) CorrectedStepNum(t *lexer.Token) (*UniqueString, error) {
	s, err := proofUnits(t)
	if err != nil {
		return nil, err
	}
	c, err := proofSubstring(s, 1, 2)
	if err != nil {
		return nil, err
	}
	var level int32
	var suffix []uint16
	if c[0] == '*' || c[0] == '+' {
		level, err = p.Get()
		if err != nil {
			return nil, err
		}
		if level < 0 && p.proofDepth > 0 {
			level, err = p.at(p.proofDepth - 1)
			if err != nil {
				return nil, err
			}
		}
		suffix, err = proofSubstring(s, 2, len(s))
	} else {
		level, err = LevelOfProofStepLexeme(t)
		if err != nil {
			return nil, err
		}
		suffix, err = proofSubstring(s, proofGreaterIndex(s), len(s))
	}
	if err != nil {
		return nil, err
	}
	out := utf16.Encode([]rune("<" + strconv.FormatInt(int64(level), 10)))
	if p.context == nil {
		p.context = NewSupportContext()
	}
	return p.context.UniqueStringOfUnits(append(out, suffix...)), nil
}
func (p *ProofState) BeginsProof(t *lexer.Token) (bool, error) {
	s, err := proofUnits(t)
	if err != nil {
		return false, err
	}
	if len(s) < 2 {
		return false, nil
	}
	if s[1] == '*' {
		return p.proofDepth < 0, nil
	}
	if s[1] == '+' {
		return true, nil
	}
	switch t.Kind {
	case tokenProofStepLexeme, tokenBareLevelLexeme, tokenUnnumberedStepLexeme, tokenProofStepDotLexeme:
		if p.proofDepth < 0 {
			return true, nil
		}
		l, err := LevelOfProofStepLexeme(t)
		if err != nil {
			return false, err
		}
		current, err := p.at(p.proofDepth)
		if err != nil {
			return false, err
		}
		return current >= 0 && l > current, nil
	case tokenBy, tokenProof, tokenObvious, tokenOmitted:
		return true, nil
	}
	return false, nil
}
func (p *ProofState) CorrectLevel(t *lexer.Token) (bool, error) {
	level, err := LevelOfProofStepLexeme(t)
	if err != nil {
		return false, err
	}
	last := int32(-1)
	if p.proofDepth > 0 {
		last, err = p.at(p.proofDepth - 1)
		if err != nil {
			return false, err
		}
	}
	current, err := p.at(p.proofDepth)
	if err != nil {
		return false, err
	}
	switch level {
	case -1:
		if current < 0 {
			if err = p.set(p.proofDepth, last+1); err != nil {
				return false, err
			}
		}
		return true, nil
	case -2:
		if current < 0 {
			if err = p.set(p.proofDepth, last+1); err != nil {
				return false, err
			}
			return true, nil
		}
		return false, nil
	default:
		if current < 0 {
			if err = p.set(p.proofDepth, level); err != nil {
				return false, err
			}
			return level > last, nil
		}
		return current == level, nil
	}
}
