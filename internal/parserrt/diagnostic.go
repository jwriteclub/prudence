// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package parserrt

import (
	"fmt"
	"sort"

	"github.com/jwriteclub/prudence/internal/lexer"
)

// Diagnostic contains snapshots taken after diagnostic replay. Token links are
// intentionally omitted: replay can mutate the parser's token chain.
type Diagnostic struct {
	Expected    [][]int
	Consumed    lexer.Token
	ErrorToken  lexer.Token
	ErrorTokens []lexer.Token
}

type savedScan struct {
	generation int32
	first      *lexer.Token
	budget     int
}
type diagnosticState struct {
	images     []string
	generation int32
	gc         int
	marks      []int32
	masks      [][]int
	calls      [][]savedScan
	replay     func(int, *Scan) (ScanOutcome, error)
	active     bool
	invalid    error
	sequences  [][]int
	last       [100]int
	end        int
}

// ConfigureDiagnostics installs the generated grammar's mask table and
// phase-two scanner dispatch before entering a production.
func (c *Cursor) ConfigureDiagnostics(masks [][]int, scanCount int, replay func(int, *Scan) (ScanOutcome, error)) error {
	if scanCount < 0 || (scanCount > 0 && replay == nil) {
		return fmt.Errorf("parser: invalid diagnostic dispatch")
	}
	d := &diagnosticState{masks: masks, marks: make([]int32, len(masks)), calls: make([][]savedScan, scanCount), replay: replay}
	for i := range d.marks {
		d.marks[i] = -1
	}
	c.diagnostic = d
	return nil
}

// ConfigureDiagnosticImages enables JavaCC message formatting for a configured cursor.
func (c *Cursor) ConfigureDiagnosticImages(images []string) error {
	if c.diagnostic == nil || len(images) == 0 {
		return fmt.Errorf("parser: missing diagnostic state or token images")
	}
	c.diagnostic.images = append([]string(nil), images...)
	return nil
}

// MarkDefault records an executed generated switch default.
func (c *Cursor) MarkDefault(index int) {
	if d := c.diagnostic; d != nil {
		if index < 0 || index >= len(d.marks) {
			d.invalid = fmt.Errorf("parser: invalid diagnostic mask index %d", index)
			return
		}
		d.marks[index] = d.generation
	}
}

// SaveScan records phase-two state at scanner return, including abnormal return.
func (c *Cursor) SaveScan(index, requested int, s *Scan) {
	d := c.diagnostic
	if d == nil {
		return
	}
	if index < 0 || index >= len(d.calls) || s == nil {
		d.invalid = fmt.Errorf("parser: invalid saved scan %d", index)
		return
	}
	records := d.calls[index]
	i := 0
	for i < len(records) && records[i].generation > d.generation {
		i++
	}
	record := savedScan{generation: d.generation + int32(requested) - int32(s.budget), first: c.current, budget: requested}
	if i == len(records) {
		records = append(records, record)
	} else {
		records[i] = record
	}
	d.calls[index] = records
}

func (c *Cursor) advanceGeneration(collect bool) {
	d := c.diagnostic
	if d == nil {
		return
	}
	d.generation++
	if collect && len(d.calls) > 0 {
		d.gc++
		if d.gc > 100 {
			d.gc = 0
			for i := range d.calls {
				for j := range d.calls[i] {
					if d.calls[i][j].generation < d.generation {
						d.calls[i][j].first = nil
					}
				}
			}
		}
	}
}

func (d *diagnosticState) add(kind, pos int) {
	if pos >= 100 {
		return
	}
	if pos == d.end+1 {
		d.last[d.end] = kind
		d.end++
		return
	}
	if d.end == 0 {
		return
	}
	sequence := append([]int(nil), d.last[:d.end]...)
	found := false
	for _, old := range d.sequences {
		if len(old) != len(sequence) {
			continue
		}
		same := true
		for i := range old {
			if old[i] != sequence[i] {
				same = false
				break
			}
		}
		if same {
			found = true
			break
		}
	}
	if !found {
		d.sequences = append(d.sequences, sequence)
	}
	if pos != 0 {
		d.end = pos
		d.last[pos-1] = kind
	}
}

func (c *Cursor) diagnosticError(kind int, message string) error {
	d := c.diagnostic
	if d == nil {
		return &ParseError{message: message}
	}
	if d.active {
		return fmt.Errorf("parser: diagnostic reentry")
	}
	if d.invalid != nil {
		return d.invalid
	}
	d.active = true
	defer func() { d.active = false }()
	kinds := map[int]bool{}
	if kind >= 0 {
		kinds[kind] = true
	}
	for i, mark := range d.marks {
		if mark == d.generation {
			for _, k := range d.masks[i] {
				kinds[k] = true
			}
		}
	}
	ordered := make([]int, 0, len(kinds))
	for k := range kinds {
		ordered = append(ordered, k)
	}
	sort.Ints(ordered)
	d.sequences = nil
	d.end = 0
	for _, k := range ordered {
		d.sequences = append(d.sequences, []int{k})
	}
	for i := range d.calls {
		for j := 0; j < len(d.calls[i]); j++ {
			record := d.calls[i][j]
			if record.generation <= d.generation {
				continue
			}
			if record.first == nil {
				return fmt.Errorf("parser: missing saved scan anchor")
			}
			s := c.resetGeneratedScan(record.first, record.budget)
			outcome, err := d.replay(i, s)
			if err != nil {
				return err
			}
			if outcome == ScanBudgetSatisfied {
				break
			}
		}
	}
	d.add(0, 0)
	if c.current == nil || c.current.Next == nil {
		return fmt.Errorf("parser: missing diagnostic error token")
	}
	snapshot := func(t *lexer.Token) lexer.Token {
		value := *t
		value.Next = nil
		value.SpecialToken = nil
		value.ImageUnits = append([]uint16(nil), t.ImageUnits...)
		return value
	}
	consumed := snapshot(c.current)
	errorToken := snapshot(c.current.Next)
	max := 0
	for _, seq := range d.sequences {
		if len(seq) > max {
			max = len(seq)
		}
	}
	tokens := make([]lexer.Token, 0, max)
	tok := c.current.Next
	for i := 0; i < max; i++ {
		if tok == nil {
			return fmt.Errorf("parser: missing linked diagnostic token")
		}
		tokens = append(tokens, snapshot(tok))
		if tok.Kind == 0 {
			break
		}
		tok = tok.Next
	}
	sequences := make([][]int, len(d.sequences))
	for i := range sequences {
		sequences[i] = append([]int(nil), d.sequences[i]...)
	}
	diagnostic := &Diagnostic{Expected: sequences, Consumed: consumed, ErrorToken: errorToken, ErrorTokens: tokens}
	if d.images != nil {
		var err error
		message, err = formatDiagnostic(diagnostic, d.images, c.Check)
		if err != nil {
			return fmt.Errorf("parser: format diagnostic: %w", err)
		}
	}
	return &ParseError{message: message, Diagnostic: diagnostic}
}

// InvalidDiagnosticIndex reports a generated dispatch inconsistency.
func InvalidDiagnosticIndex(index int) error {
	return fmt.Errorf("parser: invalid diagnostic scanner index %d", index)
}
