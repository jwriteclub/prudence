// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package parserrt

import (
	"fmt"
	"runtime"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// FormatDiagnostic formats an immutable diagnostic snapshot using JavaCC 4's
// special ParseException constructor. It never reads the token source.
func FormatDiagnostic(d *Diagnostic, images []string) (string, error) {
	return formatDiagnostic(d, images, nil)
}

func formatDiagnostic(d *Diagnostic, images []string, check func() error) (string, error) {
	if d == nil || len(images) == 0 {
		return "", fmt.Errorf("parser: missing diagnostic or token images")
	}
	eol := "\n"
	if runtime.GOOS == "windows" {
		eol = "\r\n"
	}
	var expected strings.Builder
	max := 0
	for _, seq := range d.Expected {
		if check != nil {
			if err := check(); err != nil {
				return "", err
			}
		}
		if len(seq) == 0 {
			return "", fmt.Errorf("parser: empty diagnostic expectation")
		}
		if len(seq) > max {
			max = len(seq)
		}
		for j, k := range seq {
			if j%256 == 0 && check != nil {
				if err := check(); err != nil {
					return "", err
				}
			}
			if k < 0 || k >= len(images) {
				return "", fmt.Errorf("parser: invalid diagnostic token kind %d", k)
			}
			expected.WriteString(images[k])
			expected.WriteByte(' ')
		}
		if seq[len(seq)-1] != 0 {
			expected.WriteString("...")
		}
		expected.WriteString(eol)
		expected.WriteString("    ")
	}
	var b strings.Builder
	b.WriteString("Encountered \"")
	for i := 0; i < max; i++ {
		if i >= len(d.ErrorTokens) {
			return "", fmt.Errorf("parser: missing linked diagnostic token")
		}
		t := d.ErrorTokens[i]
		if i != 0 {
			b.WriteByte(' ')
		}
		if t.Kind == 0 {
			b.WriteString(images[0])
			break
		}
		units, err := messageUnits(t.Image, t.ImageUnits, check)
		if err != nil {
			return "", err
		}
		escaped, err := escapeDiagnosticUnits(units, check)
		if err != nil {
			return "", err
		}
		b.WriteString(escaped)
	}
	fmt.Fprintf(&b, "\" at line %d, column %d.%s", d.ErrorToken.Begin.Line, d.ErrorToken.Begin.Column, eol)
	if len(d.Expected) == 1 {
		b.WriteString("Was expecting:")
	} else {
		b.WriteString("Was expecting one of:")
	}
	b.WriteString(eol)
	b.WriteString("    ")
	b.WriteString(expected.String())
	return b.String(), nil
}

func escapeDiagnosticUnits(units []uint16, check func() error) (string, error) {
	var b strings.Builder
	for i, u := range units {
		if i%256 == 0 && check != nil {
			if err := check(); err != nil {
				return "", err
			}
		}
		switch u {
		case 0:
			continue
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		case '"':
			b.WriteString(`\"`)
		case '\'':
			b.WriteString(`\'`)
		case '\\':
			b.WriteString(`\\`)
		default:
			if u < 0x20 || u > 0x7e {
				fmt.Fprintf(&b, `\u%04x`, u)
			} else {
				b.WriteByte(byte(u))
			}
		}
	}
	return b.String(), nil
}

// messageUnits compares decoded UTF-16 incrementally so cancellation remains
// observable even when a token image is very large.
func messageUnits(image string, units []uint16, check func() error) ([]uint16, error) {
	pos := 0
	matches := true
	operations := 0
	for i := 0; i < len(units); i++ {
		if operations%256 == 0 && check != nil {
			if err := check(); err != nil {
				return nil, err
			}
		}
		operations++
		r := rune(units[i])
		if r >= 0xd800 && r <= 0xdbff && i+1 < len(units) && units[i+1] >= 0xdc00 && units[i+1] <= 0xdfff {
			r = utf16.DecodeRune(r, rune(units[i+1]))
			i++
		} else if r >= 0xd800 && r <= 0xdfff {
			r = utf8.RuneError
		}
		got, size := utf8.DecodeRuneInString(image[pos:])
		if size == 0 || got != r {
			matches = false
			break
		}
		pos += size
	}
	if matches && pos == len(image) {
		return units, nil
	}
	result := make([]uint16, 0, len(image))
	operations = 0
	for _, r := range image {
		if operations%256 == 0 && check != nil {
			if err := check(); err != nil {
				return nil, err
			}
		}
		operations++
		if r <= 0xffff {
			result = append(result, uint16(r))
		} else {
			a, b := utf16.EncodeRune(r)
			result = append(result, uint16(a), uint16(b))
		}
	}
	return result, nil
}
