// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Derived from tla+.jj and ParseExceptionExtended.java, Copyright (c) 2003
// Compaq Corporation, distributed under the MIT license. See NOTICE.
package generated

import (
	"fmt"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/jwriteclub/prudence/internal/lexer"

	"github.com/jwriteclub/prudence/internal/parserrt"
)

func sanyAppendText(dst []uint16, s string) []uint16 { return append(dst, utf16.Encode([]rune(s))...) }
func sanyAppendRaw(dst []uint16, u *UniqueString) []uint16 {
	if u == nil {
		return sanyAppendText(dst, "null")
	}
	return append(dst, u.units...)
}
func sanyEscapeJava(units []uint16, check func() error) ([]uint16, error) {
	var out []uint16
	for i, u := range units {
		if i%256 == 0 && check != nil {
			if e := check(); e != nil {
				return nil, e
			}
		}
		switch u {
		case 0:
			continue
		case '\b':
			out = sanyAppendText(out, `\b`)
		case '\t':
			out = sanyAppendText(out, `\t`)
		case '\n':
			out = sanyAppendText(out, `\n`)
		case '\f':
			out = sanyAppendText(out, `\f`)
		case '\r':
			out = sanyAppendText(out, `\r`)
		case '"':
			out = sanyAppendText(out, `\"`)
		case '\'':
			out = sanyAppendText(out, `\'`)
		case '\\':
			out = sanyAppendText(out, `\\`)
		default:
			if u < 0x20 || u > 0x7e {
				out = sanyAppendText(out, fmt.Sprintf(`\u%04x`, u))
			} else {
				out = append(out, u)
			}
		}
	}
	return out, nil
}

// sanyDiagnosticUnits follows the mutable-token image convention while
// checking cancellation during long image comparisons and conversions.
func sanyDiagnosticUnits(t *lexer.Token, check func() error) ([]uint16, error) {
	pos := 0
	matches := true
	for i := 0; i < len(t.ImageUnits); i++ {
		if i%256 == 0 && check != nil {
			if e := check(); e != nil {
				return nil, e
			}
		}
		r := rune(t.ImageUnits[i])
		if r >= 0xd800 && r <= 0xdbff && i+1 < len(t.ImageUnits) && t.ImageUnits[i+1] >= 0xdc00 && t.ImageUnits[i+1] <= 0xdfff {
			r = utf16.DecodeRune(r, rune(t.ImageUnits[i+1]))
			i++
		} else if r >= 0xd800 && r <= 0xdfff {
			r = utf8.RuneError
		}
		got, size := utf8.DecodeRuneInString(t.Image[pos:])
		if size == 0 || got != r {
			matches = false
			break
		}
		pos += size
	}
	if matches && pos == len(t.Image) {
		return t.ImageUnits, nil
	}
	var out []uint16
	count := 0
	for _, r := range t.Image {
		if count%256 == 0 && check != nil {
			if e := check(); e != nil {
				return nil, e
			}
		}
		count++
		if r <= 0xffff {
			out = append(out, uint16(r))
		} else {
			a, b := utf16.EncodeRune(r)
			out = append(out, uint16(a), uint16(b))
		}
	}
	return out, nil
}

// sanyShortDiagnostic corresponds to ParseExceptionExtended's special case.
func sanyShortDiagnostic(d *parserrt.Diagnostic, check func() error) ([]uint16, error) {
	if d == nil {
		return nil, fmt.Errorf("sany: missing special parse diagnostic")
	}
	max := 0
	for _, seq := range d.Expected {
		if check != nil {
			if e := check(); e != nil {
				return nil, e
			}
		}
		if len(seq) > max {
			max = len(seq)
		}
	}
	out := sanyAppendText(nil, "Encountered \"")
	for i := 0; i < max; i++ {
		if check != nil {
			if e := check(); e != nil {
				return nil, e
			}
		}
		if i >= len(d.ErrorTokens) {
			return nil, fmt.Errorf("sany: missing diagnostic token")
		}
		t := &d.ErrorTokens[i]
		if i != 0 {
			out = append(out, ' ')
		}
		if t.Kind == 0 {
			out = sanyAppendText(out, "<EOF>")
			break
		}
		units, e := sanyDiagnosticUnits(t, check)
		if e != nil {
			return nil, e
		}
		out = append(out, units...)
	}
	out = sanyAppendText(out, fmt.Sprintf("\" at line %d, column %d and token \"", d.ErrorToken.Begin.Line, d.ErrorToken.Begin.Column))
	consumed, e := sanyDiagnosticUnits(&d.Consumed, check)
	if e != nil {
		return nil, e
	}
	escaped, e := sanyEscapeJava(consumed, check)
	if e != nil {
		return nil, e
	}
	out = append(out, escaped...)
	return sanyAppendText(out, "\" "), nil
}
func (s *SANYState) sanyMessage(short []uint16, check func() error) (*UniqueString, error) {
	out := sanyAppendText(nil, "***Parse Error***\n")
	if s.expecting == nil || len(s.expecting.units) != 0 {
		out = sanyAppendText(out, "Was expecting \"")
		out = sanyAppendRaw(out, s.expecting)
		out = sanyAppendText(out, "\"\n")
	}
	out = append(out, short...)
	out = sanyAppendText(out, "\n\nResidual stack trace follows:\n")
	last := s.msgStackCurrentSize - 10
	if last < 0 {
		last = 0
	}
	for i := s.msgStackCurrentSize; i > last; i -= 2 {
		if check != nil {
			if e := check(); e != nil {
				return nil, e
			}
		}
		if i < 2 || int64(i) > int64(len(s.msgStack)) {
			return nil, fmt.Errorf("sany: invalid message stack index")
		}
		out = sanyAppendRaw(out, s.msgStack[i-2].text)
		out = sanyAppendText(out, " starting at line ")
		t := s.msgStack[i-1].token
		if t == nil {
			return nil, fmt.Errorf("sany: nil message stack token")
		}
		out = sanyAppendText(out, fmt.Sprintf("%d, column %d.\n", t.Begin.Line, t.Begin.Column))
	}
	return s.UniqueStringOfUnits(out), nil
}
