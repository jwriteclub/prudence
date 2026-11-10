// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from MonolithSpecExtractor.java, Copyright 2020 Microsoft Research,
// MIT; see LICENSE.
package modulegraph

import (
	"context"
	"regexp"
	"sort"
)

// Segment maps a printed physical line, including its normalized newline, to
// the original root bytes. End excludes the original line terminator.
type Segment struct {
	ParsedStart, ParsedEnd, SourceStart, SourceEnd int
	SourceNext                                     int
}
type SourceView struct {
	Owner    []byte
	Path     string
	Segments []Segment
}

// MapSpan maps a half-open parser-byte span to the immutable source owner.
func (v *SourceView) MapSpan(start, end int) (int, int, bool) {
	if v == nil || start < 0 || end < start {
		return 0, 0, false
	}
	point := func(off int) (int, bool) {
		i := sort.Search(len(v.Segments), func(i int) bool { return v.Segments[i].ParsedEnd >= off })
		if i == len(v.Segments) {
			return 0, false
		}
		s := v.Segments[i]
		if off == s.ParsedEnd {
			return s.SourceNext, true
		}
		if off < s.ParsedStart {
			return 0, false
		}
		contentEnd := s.ParsedEnd - 1
		if off <= contentEnd {
			return s.SourceStart + off - s.ParsedStart, true
		}
		return 0, false
	}
	a, ok := point(start)
	if !ok {
		return 0, 0, false
	}
	b, ok := point(end)
	return a, b, ok && b >= a
}

// ExtractMonolith implements the pinned FileReader/readLine/println path on a
// platform whose default charset is UTF-8, with LF line separators. The work
// budget counts bytes traversed line by line across requests, including
// skipped lines; exhausting it is explicitly incomplete.
//
// Malformed UTF-8 is copied unchanged. Java decodes each malformed group to
// one U+FFFD, writes it as EF BF BD and decodes that again, so the extracted
// text it parses is the decoding of the raw line bytes plus LF:
//   - a malformed group holds only a lead byte and continuation bytes, so it
//     never contains CR or LF and splitting raw bytes on CR/LF finds Java's
//     readLine lines;
//   - a group truncated by a line end gets the same length before LF as
//     before CR or at EOF, so the lexer's decoder sees the same groups;
//   - module names produced by the SANY grammar contain no U+FFFD. Neither
//     replacement can satisfy a name or an ASCII header literal/class. The
//     end-marker's dot-star accepts both replacements, since neither is a
//     Java line terminator. The number of replacements cannot affect either
//     pattern's match.
func ExtractMonolith(ctx context.Context, root Input, name string, budget *int) (Input, bool, error) {
	if err := ctx.Err(); err != nil {
		return Input{}, false, err
	}
	if budget == nil {
		return Input{}, false, ErrIncomplete
	}
	// Java's default regex \s is ASCII; Java dot excludes these line terminators.
	header, err := regexp.Compile(`^-{4,}[\t\x0B\f ]*MODULE[\t\x0B\f ]+` + regexp.QuoteMeta(name) + `[\t\x0B\f ]*-{3,}$`)
	if err != nil {
		return Input{}, false, ErrIncomplete
	}
	out := Input{Path: root.Path, Origin: OriginWorkspace}
	view := &SourceView{Owner: root.Text, Path: root.Path}
	active := false
	for pos := 0; pos < len(root.Text); {
		if err := ctx.Err(); err != nil {
			return Input{}, false, err
		}
		start := pos
		for pos < len(root.Text) && root.Text[pos] != '\r' && root.Text[pos] != '\n' {
			if pos&4095 == 0 {
				if err := ctx.Err(); err != nil {
					return Input{}, false, err
				}
			}
			pos++
		}
		end := pos
		if pos < len(root.Text) {
			pos++
			if root.Text[end] == '\r' && pos < len(root.Text) && root.Text[pos] == '\n' {
				pos++
			}
		}
		cost := pos - start
		if cost > *budget {
			return Input{}, false, ErrIncomplete
		}
		*budget -= cost
		line := root.Text[start:end]
		if !active {
			if containsJavaLineTerminator(line) || !header.Match(line) {
				continue
			}
			active = true
		}
		ps := len(out.Text)
		out.Text = append(out.Text, line...)
		out.Text = append(out.Text, '\n')
		view.Segments = append(view.Segments, Segment{ps, len(out.Text), start, end, pos})
		if len(view.Segments) > (128<<20)/40 || len(out.Text) > 32<<20 {
			return Input{}, false, ErrIncomplete
		}
		if len(line) >= 4 && string(line[:4]) == "====" && !containsJavaLineTerminator(line) {
			break
		}
	}
	if !active {
		return Input{}, false, nil
	}
	out.View = view
	return out, true, nil
}
func containsJavaLineTerminator(b []byte) bool {
	for _, r := range string(b) {
		if r == 0x85 || r == 0x2028 || r == 0x2029 {
			return true
		}
	}
	return false
}
