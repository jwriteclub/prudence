// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
// Source SHA-256: bfc981f31c205333f478b773ab108159b5e2d339b659d33b0f920d56ebf4253d
package generated

import (
	"github.com/jwriteclub/prudence/internal/lexer"
)

type localTokenView interface {
	Token(int) (*lexer.Token, error)
	NextAfter(*lexer.Token) (*lexer.Token, error)
}

const errNilPreviewToken supportError = "sany preview: nil token"

func InitPreviewToken(v localTokenView) (*lexer.Token, error) {
	t, err := v.Token(0)
	if err == nil && t == nil {
		err = errNilPreviewToken
	}
	return t, err
}

func NextPreviewToken(v localTokenView, t *lexer.Token) (*lexer.Token, error) {
	if t == nil {
		return nil, errNilPreviewToken
	}
	n, err := v.NextAfter(t)
	if err == nil && n == nil {
		err = errNilPreviewToken
	}
	return n, err
}

func isOpKind(kind int) bool        { return kind >= tokenOp57 && kind <= tokenOp119 }
func isInfixOpKind(kind int) bool   { return kind >= tokenOp1 && kind <= tokenOp119 }
func isPostfixOpKind(kind int) bool { return kind >= tokenOp57 && kind <= tokenOp70 }

func IsOp(t *lexer.Token) (bool, error) {
	if t == nil {
		return false, errNilPreviewToken
	}
	return isOpKind(t.Kind), nil
}
func IsInfixOp(t *lexer.Token) (bool, error) {
	if t == nil {
		return false, errNilPreviewToken
	}
	return isInfixOpKind(t.Kind), nil
}
func IsPostfixOp(t *lexer.Token) (bool, error) {
	if t == nil {
		return false, errNilPreviewToken
	}
	return isPostfixOpKind(t.Kind), nil
}

// BelchDEF inserts the definition marker while restoring all temporarily
// reversed links on both successful and unsuccessful returns. After an error,
// callers must use a fresh token source.
func BelchDEF(v localTokenView, check func() error) (err error) {
	step := func() error {
		if check != nil {
			return check()
		}
		return nil
	}
	if err = step(); err != nil {
		return err
	}
	anchor, err := InitPreviewToken(v)
	if err != nil {
		return err
	}
	current, err := NextPreviewToken(v, anchor)
	if err != nil {
		return err
	}
	// head is the reversed prefix; tail is the retained forward suffix.
	head, tail := anchor, current
	restore := func() {
		for head != nil {
			next := head.Next
			head.Next = tail
			tail = head
			head = next
		}
	}
	defer restore()
	anchor.Next = nil
	next, err := NextPreviewToken(v, current)
	if err != nil {
		return err
	}
	current.Next = head
	head, tail = current, next
	previous := anchor
	for next.Kind != tokenEOF && next.Kind != tokenTheorem && next.Kind != tokenProposition && next.Kind != tokenAssumption && next.Kind != tokenEndModule && current.Kind != tokenTheorem && current.Kind != tokenProposition {
		if err = step(); err != nil {
			return err
		}
		if current.Kind == tokenDEF {
			t := previous
			if t == nil {
				return errNilPreviewToken
			}
			if t.Kind == tokenRBR || t.Kind == tokenRSB {
				depth := 1
				for t.Next != nil {
					if err = step(); err != nil {
						return err
					}
					t = t.Next
					if (t.Kind == tokenLBR || t.Kind == tokenLSB) && depth == 1 {
						break
					}
					if (t.Kind == tokenLBR || t.Kind == tokenLSB) && depth > 1 {
						depth--
					}
					if t.Kind == tokenRBR || t.Kind == tokenRSB {
						depth++
					}
				}
				if t.Next == nil {
					break
				}
				if t.Next.Kind == tokenIdentifier {
					t = t.Next
				}
			} else if t.Kind == tokenIdentifier {
				identifier := t
				if t.Next == nil {
					return errNilPreviewToken
				}
				if isOpKind(t.Next.Kind) && !isPostfixOpKind(t.Next.Kind) && (!isInfixOpKind(t.Next.Kind) || (t.Next.Next != nil && t.Next.Next.Kind == tokenIdentifier)) {
					t = t.Next
					if t.Next == nil {
						return errNilPreviewToken
					}
					if t.Next.Kind == tokenSubstitute {
						t = identifier
					} else if isInfixOpKind(t.Kind) {
						t = t.Next
					}
				}
			} else if isOpKind(t.Kind) {
				t = t.Next
			}
			if t == nil {
				return errNilPreviewToken
			}
			if t.Next == nil || t.Next.Kind != tokenDEFBREAK {
				marker := &lexer.Token{Kind: tokenDEFBREAK, Begin: t.Begin, End: t.End, Range: t.Range, Next: t.Next}
				marker.SetImage("Beginning of definition")
				t.Next = marker
			}
			break
		}
		previous = current
		current = next
		next, err = NextPreviewToken(v, current)
		if err != nil {
			return err
		}
		current.Next = head
		head, tail = current, next
	}
	restore()
	return step()
}
