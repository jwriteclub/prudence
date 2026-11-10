// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Package lexer runs generated JavaCC-style lexical automata.
package lexer

import (
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Range is an inclusive range of UTF-16 code units.
type Range struct{ Low, High uint16 }
type Edge struct {
	To     int
	Ranges []Range
}
type State struct {
	Epsilon []int
	Edges   []Edge
	// Accept contains rule indexes; ties are resolved by token ordinal.
	Accept []int
}
type Rule struct {
	Ordinal   int
	Kind      string // TOKEN, SKIP, MORE, or SPECIAL_TOKEN
	NextState int    // -1 retains the current lexical state
	Action    string // source-hash binding key, empty when no action
}
type Tables struct {
	States       []State
	Starts       []int
	Rules        []Rule
	DFA          *DFA
	DefaultState int
}
type Position struct{ Line, Column int }

// InputUnits returns the decoded UTF-16 input and corresponding inclusive positions.
func (l *Lexer) InputUnits() ([]uint16, []Position) {
	return append([]uint16(nil), l.input...), append([]Position(nil), l.positions...)
}

// InputByteRanges returns the original byte range for each UTF-16 unit.
func (l *Lexer) InputByteRanges() [][2]int { return append([][2]int(nil), l.byteRanges...) }

// LexicalError reports JavaCC's failure location and encountered UTF-16 unit.
type LexicalError struct {
	Position    Position
	State       int
	EOF         bool
	Encountered uint16
	// StreamStart and PhysicalEnd are input positions before EOF adjustment.
	// PostFailureEnd is the JavaCC character stream position after failure.
	StreamStart, PhysicalEnd, PostFailureEnd Position
	HasStreamSpan                            bool
	after                                    []uint16
}

// AfterUnits returns a defensive copy of JavaCC's error-after image.
// Retaining the error retains the immutable input allocation it refers to.
func (e *LexicalError) AfterUnits() []uint16 { return append([]uint16(nil), e.after...) }

func escapeUnits(units []uint16) string {
	var b strings.Builder
	for _, c := range units {
		switch c {
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
			if c < 0x20 || c > 0x7e {
				fmt.Fprintf(&b, `\u%04x`, c)
			} else {
				b.WriteByte(byte(c))
			}
		}
	}
	return b.String()
}
func (e *LexicalError) Error() string {
	encountered := "<EOF> "
	if !e.EOF {
		encountered = fmt.Sprintf("\"%s\" (%d), ", escapeUnits([]uint16{e.Encountered}), e.Encountered)
	}
	return fmt.Sprintf("Lexical error at line %d, column %d.  Encountered: %safter : \"%s\"", e.Position.Line, e.Position.Column, encountered, escapeUnits(e.after))
}

type Token struct {
	Kind                    int
	Image                   string
	ImageUnits              []uint16
	Begin, End              Position
	StateBefore, StateAfter int
	Next, SpecialToken      *Token
	Range                   SourceRange
	Diagnostic              *RecoveryDiagnostic
}

// SetImage replaces the convenience image and its UTF-16 units. Direct Image
// assignments cannot distinguish an isolated surrogate from U+FFFD when their
// decoded Go string views are equal; use a setter when that distinction matters.
func (t *Token) SetImage(image string) {
	t.Image = image
	t.ImageUnits = utf16.Encode([]rune(image))
}

// SetImageUnits replaces the image with a copy of exact UTF-16 units.
func (t *Token) SetImageUnits(units []uint16) {
	t.ImageUnits = append([]uint16(nil), units...)
	t.Image = string(utf16.Decode(t.ImageUnits))
}

// SourceRange is half-open in UTF-16 units and covers the original input bytes.
// Two tokens splitting a supplementary scalar can have overlapping byte spans.
type SourceRange struct{ UnitStart, UnitEnd, ByteStart, ByteEnd int }

// RecoveryDiagnostic describes why an opt-in kind -1 token was emitted.
type RecoveryDiagnostic struct {
	Reason string
	Error  *LexicalError
}

// Specials returns special tokens in source order, without changing their links.
func (t *Token) Specials() []*Token {
	var rev []*Token
	seen := map[*Token]bool{}
	for p := t.SpecialToken; p != nil; p = p.SpecialToken {
		if seen[p] {
			break
		}
		seen[p] = true
		rev = append(rev, p)
	}
	out := make([]*Token, len(rev))
	for i := range rev {
		out[len(rev)-1-i] = rev[i]
	}
	return out
}

// Action is invoked after each match and before its declared state transition.
type Action func(l *Lexer, rule Rule, token *Token) error
type Lexer struct {
	tables tableView
	State  int
	Action Action
	// ResetAction resets state owned by Action after a successful ReInit.
	ResetAction       func()
	Recover           bool
	UseNFA            bool
	input             []uint16
	byteRanges        [][2]int
	inputBytes        int
	dfaMemo           map[dfaMemoKey]dfaMemoResult
	positions         []Position
	cursor, moreStart int
	more              bool
	lastSpecial       *Token
	initError         error
}

// New creates a lazy scanner with strict lexical errors by default.
func New(tables TableSource, input string) *Lexer {
	view, err := newTableView(tables)
	l := newLexer(view, input)
	l.initError = err
	return l
}

func newLexer(tables tableView, input string) *Lexer {
	runes, spans := decodeUTF8Ranges(input)
	units := utf16.Encode(runes)
	ranges := make([][2]int, 0, len(units))
	for i, r := range runes {
		ranges = append(ranges, spans[i])
		if r > 0xffff {
			ranges = append(ranges, spans[i])
		}
	}
	l := &Lexer{input: units, byteRanges: ranges, inputBytes: len(input), positions: make([]Position, len(units))}
	l.tables = tables
	l.State = tables.defaultState()
	line, column := 1, 0
	prevCR, prevLF := false, false
	for i, c := range units {
		column++
		if prevLF {
			prevLF = false
			line++
			column = 1
		} else if prevCR {
			prevCR = false
			if c == '\n' {
				prevLF = true
			} else {
				line++
				column = 1
			}
		}
		switch c {
		case '\r':
			prevCR = true
		case '\n':
			prevLF = true
		case '\t':
			column--
			column += 8 - (column % 8)
		}
		l.positions[i] = Position{line, column}
	}
	return l
}

// SwitchTo changes the lexical state for subsequent token fetches.
func (l *Lexer) SwitchTo(state int) error {
	if l.initError != nil {
		return l.initError
	}
	if state < 0 || state >= l.tables.startCount() {
		return fmt.Errorf("invalid lexical state %d", state)
	}
	l.State = state
	return nil
}

// SwitchToDefault selects the tables' default lexical state.
func (l *Lexer) SwitchToDefault() error {
	if l.initError != nil {
		return l.initError
	}
	return l.SwitchTo(l.tables.defaultState())
}

// ReInit resets input and lexical state, retains action bindings and options,
// then invokes ResetAction once if present. A stateful Action must be paired
// with its reset hook by its owner.
func (l *Lexer) ReInit(input string) {
	action, resetAction, recoverMode, useNFA := l.Action, l.ResetAction, l.Recover, l.UseNFA
	if l.initError != nil {
		return
	}
	next := newLexer(l.tables, input)
	*l = *next
	l.Action, l.ResetAction, l.Recover = action, resetAction, recoverMode
	l.UseNFA = useNFA
	if l.ResetAction != nil {
		l.ResetAction()
	}
}

// decodeUTF8 follows the replacement grouping of Java's UTF-8 decoder.
func decodeUTF8(s string) []rune {
	out, _ := decodeUTF8Ranges(s)
	return out
}

func decodeUTF8Ranges(s string) ([]rune, [][2]int) {
	var out []rune
	var spans [][2]int
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r != utf8.RuneError || size > 1 {
			out = append(out, r)
			spans = append(spans, [2]int{i, i + size})
			i += size
			continue
		}
		b := s[i]
		need := 0
		switch {
		case b >= 0xc2 && b <= 0xdf:
			need = 2
		case b >= 0xe0 && b <= 0xef:
			need = 3
		case b >= 0xf0 && b <= 0xf4:
			need = 4
		}
		used := 1
		if need > 0 && i+1 < len(s) {
			second := s[i+1]
			validSecond := second >= 0x80 && second <= 0xbf && !(b == 0xe0 && second < 0xa0) && !(b == 0xf0 && second < 0x90) && !(b == 0xf4 && second > 0x8f)
			if validSecond {
				used = 2
				for used < need && i+used < len(s) && s[i+used] >= 0x80 && s[i+used] <= 0xbf {
					used++
				}
			}
		}
		out = append(out, utf8.RuneError)
		spans = append(spans, [2]int{i, i + used})
		i += used
	}
	return out, spans
}

func cloneTables(t *Tables) *Tables {
	out := &Tables{Starts: append([]int(nil), t.Starts...), Rules: append([]Rule(nil), t.Rules...), States: make([]State, len(t.States)), DefaultState: t.DefaultState}
	for i, state := range t.States {
		out.States[i].Epsilon = append([]int(nil), state.Epsilon...)
		out.States[i].Accept = append([]int(nil), state.Accept...)
		for _, edge := range state.Edges {
			out.States[i].Edges = append(out.States[i].Edges, Edge{To: edge.To, Ranges: append([]Range(nil), edge.Ranges...)})
		}
	}
	if t.DFA != nil {
		out.DFA = &DFA{Starts: append([]int(nil), t.DFA.Starts...), States: append([]DFAState(nil), t.DFA.States...)}
		for i, state := range t.DFA.States {
			out.DFA.States[i].Edges = append([]DFAEdge(nil), state.Edges...)
		}
	}
	return out
}

func validate(t *Tables) error {
	if t == nil || len(t.Starts) == 0 {
		return fmt.Errorf("invalid lexer tables: missing lexical states")
	}
	if t.DefaultState < 0 || t.DefaultState >= len(t.Starts) {
		return fmt.Errorf("invalid lexer tables: default lexical state %d", t.DefaultState)
	}
	valid := func(id int) bool { return id >= 0 && id < len(t.States) }
	for _, start := range t.Starts {
		if !valid(start) {
			return fmt.Errorf("invalid lexer state start %d", start)
		}
	}
	for _, state := range t.States {
		for _, id := range state.Epsilon {
			if !valid(id) {
				return fmt.Errorf("invalid epsilon target %d", id)
			}
		}
		for _, edge := range state.Edges {
			if !valid(edge.To) {
				return fmt.Errorf("invalid transition target %d", edge.To)
			}
			for _, r := range edge.Ranges {
				if r.Low > r.High {
					return fmt.Errorf("invalid character range")
				}
			}
		}
		for _, id := range state.Accept {
			if id < 0 || id >= len(t.Rules) {
				return fmt.Errorf("invalid rule index %d", id)
			}
		}
	}
	for _, rule := range t.Rules {
		if rule.Ordinal <= 0 || rule.NextState < -1 || rule.NextState >= len(t.Starts) {
			return fmt.Errorf("invalid lexical rule")
		}
		switch rule.Kind {
		case "TOKEN", "SKIP", "MORE", "SPECIAL_TOKEN":
		default:
			return fmt.Errorf("invalid lexical rule kind %q", rule.Kind)
		}
	}
	if t.DFA != nil {
		d := t.DFA
		if len(d.Starts) != len(t.Starts) {
			return fmt.Errorf("invalid DFA lexical states")
		}
		validDFA := func(id int) bool { return id >= 0 && id < len(d.States) }
		for _, start := range d.Starts {
			if !validDFA(start) {
				return fmt.Errorf("invalid DFA start")
			}
			if d.States[start].Accept >= 0 {
				return fmt.Errorf("invalid lexer tables: zero-width rule")
			}
		}
		for _, state := range d.States {
			if state.Accept < -1 || state.Accept >= len(t.Rules) {
				return fmt.Errorf("invalid DFA rule")
			}
			for _, target := range state.ASCII {
				if target < 0 || target > len(d.States) {
					return fmt.Errorf("invalid DFA transition")
				}
			}
			last := 127
			for _, edge := range state.Edges {
				if int(edge.Low) <= last || edge.Low > edge.High || edge.Target <= 0 || edge.Target > len(d.States) {
					return fmt.Errorf("invalid DFA range")
				}
				last = int(edge.High)
			}
		}
	}
	for _, start := range t.Starts {
		seen := make([]bool, len(t.States))
		queue := []int{start}
		seen[start] = true
		for i := 0; i < len(queue); i++ {
			state := t.States[queue[i]]
			if len(state.Accept) != 0 {
				return fmt.Errorf("invalid lexer tables: zero-width rule")
			}
			for _, id := range state.Epsilon {
				if !seen[id] {
					seen[id] = true
					queue = append(queue, id)
				}
			}
		}
	}
	return nil
}

func (l *Lexer) image(start, end int) string { return string(utf16.Decode(l.input[start:end])) }
func (l *Lexer) sourceRange(start, end int) SourceRange {
	result := SourceRange{UnitStart: start, UnitEnd: end, ByteStart: l.inputBytes, ByteEnd: l.inputBytes}
	if start < len(l.byteRanges) {
		result.ByteStart = l.byteRanges[start][0]
	}
	if end > start {
		result.ByteEnd = l.byteRanges[end-1][1]
	} else {
		result.ByteEnd = result.ByteStart
	}
	return result
}
func (l *Lexer) pos(i int) Position {
	if i >= 0 && i < len(l.positions) {
		return l.positions[i]
	}
	if len(l.positions) > 0 {
		return l.positions[len(l.positions)-1]
	}
	return Position{0, 0}
}
func (l *Lexer) failure(start, at, tokenStart int) *LexicalError {
	pos := l.pos(at)
	eof := at+1 >= len(l.input)
	if eof {
		if l.input[at] == '\n' || l.input[at] == '\r' {
			pos.Line++
			pos.Column = 0
		} else {
			pos.Column++
		}
	}
	var after []uint16
	if at-start+1 > 1 {
		end := at
		if eof {
			end = at + 1
		}
		after = l.input[tokenStart:end]
	}
	physicalEnd := l.pos(at)
	postEnd := physicalEnd
	if !eof {
		postEnd = Position{}
		if at > 0 {
			postEnd = l.pos(at - 1)
		}
	}
	return &LexicalError{Position: pos, State: l.State, EOF: eof, Encountered: l.input[at],
		StreamStart: l.pos(tokenStart), PhysicalEnd: physicalEnd, PostFailureEnd: postEnd, HasStreamSpan: true, after: after}
}

// NextToken returns one regular token or EOF, retaining preceding specials.
func (l *Lexer) NextToken() (*Token, error) {
	return l.nextToken(nil)
}

func (l *Lexer) nextToken(probe *cancelProbe) (*Token, error) {
	if l.initError != nil {
		return nil, l.initError
	}
	for {
		if probe != nil {
			if err := probe.check(); err != nil {
				return nil, err
			}
		}
		if l.State < 0 || l.State >= l.tables.startCount() {
			return nil, fmt.Errorf("invalid lexical state %d", l.State)
		}
		if l.cursor == len(l.input) {
			if l.more {
				if l.Recover {
					start := l.moreStart
					l.more = false
					t := &Token{Kind: -1, Image: l.image(start, l.cursor), ImageUnits: append([]uint16(nil), l.input[start:l.cursor]...), Begin: l.pos(start), End: l.pos(l.cursor - 1), SpecialToken: l.lastSpecial, StateBefore: l.State, StateAfter: l.State, Range: l.sourceRange(start, l.cursor), Diagnostic: &RecoveryDiagnostic{Reason: "unterminated_more", Error: l.failure(l.cursor-1, l.cursor-1, start)}}
					l.lastSpecial = nil
					return t, nil
				}
				failure := l.failure(l.cursor-1, l.cursor-1, l.moreStart)
				l.more = false
				l.lastSpecial = nil
				return nil, failure
			}
			t := &Token{Kind: 0, Begin: l.pos(l.cursor), End: l.pos(l.cursor), SpecialToken: l.lastSpecial, StateBefore: l.State, StateAfter: l.State, Range: l.sourceRange(l.cursor, l.cursor)}
			l.lastSpecial = nil
			return t, nil
		}
		start := l.cursor
		var ruleIndex, end, failed int
		var err error
		if l.tables.hasDFA() && !l.UseNFA {
			ruleIndex, end, failed, err = l.matchDFA(probe)
		} else {
			ruleIndex, end, failed, err = l.matchNFA(probe)
		}
		if err != nil {
			if _, interrupted := err.(*InterruptionError); !interrupted {
				l.initError = err
			}
			return nil, err
		}
		if ruleIndex < 0 {
			tokenStart := start
			if l.more {
				tokenStart = l.moreStart
			}
			failure := l.failure(start, failed, tokenStart)
			if !l.Recover {
				if failure.EOF {
					l.cursor = len(l.input)
				} else {
					l.cursor = failed
				}
				l.more = false
				l.lastSpecial = nil
				return nil, failure
			}
			if l.more {
				start = l.moreStart
			}
			l.cursor++
			t := &Token{Kind: -1, Image: l.image(start, l.cursor), ImageUnits: append([]uint16(nil), l.input[start:l.cursor]...), Begin: l.pos(start), End: l.pos(l.cursor - 1), SpecialToken: l.lastSpecial, StateBefore: l.State, StateAfter: l.State, Range: l.sourceRange(start, l.cursor), Diagnostic: &RecoveryDiagnostic{Reason: "unexpected_input", Error: failure}}
			l.lastSpecial = nil
			l.more = false
			return t, nil
		}
		rule, err := l.tables.rule(ruleIndex)
		if err != nil {
			l.initError = err
			return nil, err
		}
		l.cursor = end
		if !l.more {
			l.moreStart = start
		}
		// JavaCC accumulates MORE without materializing an image until a
		// lexical action or completed token needs it. This also avoids
		// repeatedly copying long comments one character at a time.
		if rule.Action == "" && (rule.Kind == "MORE" || rule.Kind == "SKIP") {
			if rule.NextState >= 0 {
				l.State = rule.NextState
			}
			l.more = rule.Kind == "MORE"
			continue
		}
		t := &Token{Kind: rule.Ordinal, Image: l.image(l.moreStart, end), ImageUnits: append([]uint16(nil), l.input[l.moreStart:end]...), Begin: l.pos(l.moreStart), End: l.pos(end - 1), StateBefore: l.State, Range: l.sourceRange(l.moreStart, end)}
		if rule.Action != "" {
			if l.Action == nil {
				return nil, fmt.Errorf("unbound lexical action %s", rule.Action)
			}
			if err := l.Action(l, rule, t); err != nil {
				return nil, err
			}
		}
		if rule.NextState >= 0 {
			l.State = rule.NextState
		}
		t.StateAfter = l.State
		switch rule.Kind {
		case "MORE":
			l.more = true
		case "SKIP":
			l.more = false
		case "SPECIAL_TOKEN":
			l.more = false
			t.SpecialToken = l.lastSpecial
			if l.lastSpecial != nil {
				l.lastSpecial.Next = t
			}
			l.lastSpecial = t
		case "TOKEN":
			l.more = false
			t.SpecialToken = l.lastSpecial
			l.lastSpecial = nil
			return t, nil
		default:
			return nil, fmt.Errorf("invalid lexical rule kind %q", rule.Kind)
		}
	}
}
