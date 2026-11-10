// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package lexer

import "fmt"

// TableSource is a sealed source of lexer tables.
type TableSource interface{ tableSource() }

func (*Tables) tableSource()     {}
func (ConstTables) tableSource() {}

// ConstTables holds a versioned immutable table encoding. Its strings may be
// copied freely. New checks the layout, and checks each reached value on use.
type ConstTables struct{ Layout, Indices, Units, Text string }

const constSections = 12
const (
	secStarts = iota
	secStates
	secEpsilon
	secEdges
	secAccept
	secDFAStarts
	secDFAStates
	secASCII
	secDFAEdges
	secRules
	secStateNames
	secTokenNames
)

type constSection struct{ off, count int }
type tableView struct {
	legacy   *Tables
	c        ConstTables
	width    int
	sections [constSections]constSection
	def      int
}
type constTableError struct{}

func (*constTableError) Error() string { return "invalid constant lexer tables" }
func invalid() error                   { return &constTableError{} }
func u32(s string, p int) (uint32, error) {
	if p < 0 || p > len(s)-4 {
		return 0, invalid()
	}
	return uint32(s[p]) | uint32(s[p+1])<<8 | uint32(s[p+2])<<16 | uint32(s[p+3])<<24, nil
}
func newTableView(src TableSource) (tableView, error) {
	var v tableView
	switch t := src.(type) {
	case *Tables:
		if t == nil {
			return v, fmt.Errorf("nil lexer tables")
		}
		if err := validate(t); err != nil {
			return v, err
		}
		v.legacy = cloneTables(t)
		return v, nil
	case ConstTables:
		v.c = t
	case *ConstTables:
		if t == nil {
			return v, invalid()
		}
		v.c = *t
	default:
		return v, invalid()
	}
	s := v.c.Layout
	if len(s) != 12+constSections*8 || s[:4] != "JLC1" || (s[4] != 2 && s[4] != 4) || s[5] != 0 || s[6] != 0 || s[7] != 0 {
		return v, invalid()
	}
	v.width = int(s[4])
	d, _ := u32(s, 8)
	if uint64(d) > uint64(^uint(0)>>1) {
		return v, invalid()
	}
	v.def = int(d)
	strides := [constSections]int{1, 9, 1, 4, 1, 1, 5, 1, 2, 5, 2, 2}
	end := uint64(0)
	for i := range v.sections {
		o, _ := u32(s, 12+8*i)
		n, _ := u32(s, 16+8*i)
		if uint64(o) > uint64(^uint(0)>>1) || uint64(n) > uint64(^uint(0)>>1) {
			return v, invalid()
		}
		off, count := int(o), int(n)
		if uint64(off) != end {
			return v, invalid()
		}
		end = uint64(off) + uint64(count)*uint64(strides[i])
		if end > uint64(len(v.c.Indices)/v.width) {
			return v, invalid()
		}
		v.sections[i] = constSection{off, count}
	}
	if end != uint64(len(v.c.Indices)/v.width) || v.sections[secDFAStarts].count != v.sections[secStarts].count || uint64(v.sections[secDFAStates].count)*128 != uint64(v.sections[secASCII].count) {
		return v, invalid()
	}
	if len(v.c.Indices)%v.width != 0 || len(v.c.Units)%4 != 0 || v.def >= v.startCount() {
		return tableView{}, invalid()
	}
	return v, nil
}
func (v *tableView) number(sec, row, col, stride int) (int, error) {
	x := v.sections[sec]
	if row < 0 || row >= x.count || col < 0 || col >= stride {
		return 0, invalid()
	}
	p := (x.off + row*stride + col) * v.width
	var n uint32
	if v.width == 2 {
		n = uint32(v.c.Indices[p]) | uint32(v.c.Indices[p+1])<<8
	} else {
		n, _ = u32(v.c.Indices, p)
	}
	if uint64(n) > uint64(^uint(0)>>1) {
		return 0, invalid()
	}
	return int(n), nil
}
func (v *tableView) stateCount() int {
	if v.legacy != nil {
		return len(v.legacy.States)
	}
	return v.sections[secStates].count
}
func (v *tableView) ruleCount() int {
	if v.legacy != nil {
		return len(v.legacy.Rules)
	}
	return v.sections[secRules].count
}
func (v *tableView) startCount() int {
	if v.legacy != nil {
		return len(v.legacy.Starts)
	}
	return v.sections[secStarts].count
}
func (v *tableView) defaultState() int {
	if v.legacy != nil {
		return v.legacy.DefaultState
	}
	return v.def
}
func (v *tableView) hasDFA() bool {
	if v.legacy != nil {
		return v.legacy.DFA != nil
	}
	return v.width != 0
}
func (v *tableView) dfaStateCount() int {
	if v.legacy != nil {
		if v.legacy.DFA == nil {
			return 0
		}
		return len(v.legacy.DFA.States)
	}
	return v.sections[secDFAStates].count
}
func (v *tableView) stateID(i int) error {
	if i < 0 || i >= v.stateCount() {
		return invalid()
	}
	return nil
}
func (v *tableView) start(i int) (int, error) {
	if i < 0 || i >= v.startCount() {
		return 0, invalid()
	}
	if v.legacy != nil {
		return v.legacy.Starts[i], nil
	}
	n, e := v.number(secStarts, i, 0, 1)
	if e == nil {
		e = v.stateID(n)
	}
	return n, e
}

type indexList struct {
	legacy     []int
	start, end int
	view       *tableView
	sec        int
	state      bool
}

func (x indexList) Len() int {
	if x.view == nil {
		return len(x.legacy)
	}
	return x.end - x.start
}
func (x indexList) At(i int) (int, error) {
	if i < 0 || i >= x.Len() {
		return 0, invalid()
	}
	if x.view == nil {
		return x.legacy[i], nil
	}
	n, e := x.view.number(x.sec, x.start+i, 0, 1)
	if e != nil {
		return 0, e
	}
	if x.state {
		e = x.view.stateID(n)
	} else if n >= x.view.ruleCount() {
		e = invalid()
	}
	return n, e
}

type edgeList struct {
	legacy     []Edge
	start, end int
	view       *tableView
}
type rangeList struct {
	legacy     []Range
	start, end int
	view       *tableView
}
type edgeView struct {
	To     int
	Ranges rangeList
}

func (x edgeList) Len() int {
	if x.view == nil {
		return len(x.legacy)
	}
	return x.end - x.start
}
func (x edgeList) At(i int) (edgeView, error) {
	if i < 0 || i >= x.Len() {
		return edgeView{}, invalid()
	}
	if x.view == nil {
		e := x.legacy[i]
		return edgeView{e.To, rangeList{legacy: e.Ranges}}, nil
	}
	v := x.view
	row := x.start + i
	to, e := v.number(secEdges, row, 0, 4)
	if e != nil {
		return edgeView{}, e
	}
	if e = v.stateID(to); e != nil {
		return edgeView{}, e
	}
	a, b, e := v.span(secEdges, row, 1, 4, len(v.c.Units)/4)
	return edgeView{to, rangeList{start: a, end: b, view: v}}, e
}
func (x rangeList) Len() int {
	if x.view == nil {
		return len(x.legacy)
	}
	return x.end - x.start
}
func (x rangeList) At(i int) (Range, error) {
	if i < 0 || i >= x.Len() {
		return Range{}, invalid()
	}
	if x.view == nil {
		return x.legacy[i], nil
	}
	p := (x.start + i) * 4
	s := x.view.c.Units
	if p < 0 || p > len(s)-4 {
		return Range{}, invalid()
	}
	r := Range{uint16(s[p]) | uint16(s[p+1])<<8, uint16(s[p+2]) | uint16(s[p+3])<<8}
	if r.Low > r.High {
		return Range{}, invalid()
	}
	return r, nil
}
func (v *tableView) span(sec, row, col, stride, limit int) (int, int, error) {
	a, e := v.number(sec, row, col, stride)
	if e != nil {
		return 0, 0, e
	}
	b, e := v.number(sec, row, col+1, stride)
	if e != nil {
		return 0, 0, e
	}
	p, e := v.number(sec, row, col+2, stride)
	if e != nil || p > 1 || a > b || b > limit || p == 0 && a != b {
		return 0, 0, invalid()
	}
	return a, b, nil
}
func (v *tableView) epsilon(i int) (indexList, error) {
	if e := v.stateID(i); e != nil {
		return indexList{}, e
	}
	if v.legacy != nil {
		return indexList{legacy: v.legacy.States[i].Epsilon}, nil
	}
	a, b, e := v.span(secStates, i, 0, 9, v.sections[secEpsilon].count)
	return indexList{start: a, end: b, view: v, sec: secEpsilon, state: true}, e
}
func (v *tableView) edges(i int) (edgeList, error) {
	if e := v.stateID(i); e != nil {
		return edgeList{}, e
	}
	if v.legacy != nil {
		return edgeList{legacy: v.legacy.States[i].Edges}, nil
	}
	a, b, e := v.span(secStates, i, 3, 9, v.sections[secEdges].count)
	return edgeList{start: a, end: b, view: v}, e
}
func (v *tableView) accepts(i int) (indexList, error) {
	if e := v.stateID(i); e != nil {
		return indexList{}, e
	}
	if v.legacy != nil {
		return indexList{legacy: v.legacy.States[i].Accept}, nil
	}
	a, b, e := v.span(secStates, i, 6, 9, v.sections[secAccept].count)
	return indexList{start: a, end: b, view: v, sec: secAccept}, e
}
func (v *tableView) dfaStart(i int) (int, error) {
	if i < 0 || i >= v.startCount() {
		return 0, invalid()
	}
	if v.legacy != nil {
		if v.legacy.DFA == nil || i >= len(v.legacy.DFA.Starts) {
			return 0, invalid()
		}
		return v.legacy.DFA.Starts[i], nil
	}
	n, e := v.number(secDFAStarts, i, 0, 1)
	if e == nil && (n < 0 || n >= v.dfaStateCount()) {
		e = invalid()
	}
	return n, e
}
func (v *tableView) dfaAccept(i int) (int, error) {
	if i < 0 || i >= v.dfaStateCount() {
		return 0, invalid()
	}
	if v.legacy != nil {
		return v.legacy.DFA.States[i].Accept, nil
	}
	n, e := v.number(secDFAStates, i, 0, 5)
	if e != nil {
		return 0, e
	}
	n--
	if n >= v.ruleCount() {
		return 0, invalid()
	}
	return n, nil
}
func (v *tableView) dfaASCII(i, unit int) (int, error) {
	if i < 0 || i >= v.dfaStateCount() || unit < 0 || unit >= 128 {
		return 0, invalid()
	}
	if v.legacy != nil {
		return v.legacy.DFA.States[i].ASCII[unit], nil
	}
	a, e := v.number(secDFAStates, i, 1, 5)
	if e != nil || a > v.sections[secASCII].count || unit >= v.sections[secASCII].count-a {
		return 0, invalid()
	}
	n, e := v.number(secASCII, a+unit, 0, 1)
	if e == nil && n > v.dfaStateCount() {
		e = invalid()
	}
	return n, e
}

type dfaEdgeList struct {
	legacy     []DFAEdge
	start, end int
	view       *tableView
}

func (x dfaEdgeList) Len() int {
	if x.view == nil {
		return len(x.legacy)
	}
	return x.end - x.start
}
func (x dfaEdgeList) At(i int) (DFAEdge, error) {
	if i < 0 || i >= x.Len() {
		return DFAEdge{}, invalid()
	}
	if x.view == nil {
		return x.legacy[i], nil
	}
	v := x.view
	row := x.start + i
	r, e := v.number(secDFAEdges, row, 0, 2)
	if e != nil {
		return DFAEdge{}, e
	}
	if r >= len(v.c.Units)/4 {
		return DFAEdge{}, invalid()
	}
	ranges := rangeList{start: r, end: r + 1, view: v}
	rr, e := ranges.At(0)
	if e != nil {
		return DFAEdge{}, e
	}
	target, e := v.number(secDFAEdges, row, 1, 2)
	if e != nil || target == 0 || target > v.dfaStateCount() {
		return DFAEdge{}, invalid()
	}
	return DFAEdge{Low: rr.Low, High: rr.High, Target: target}, nil
}
func (v *tableView) dfaEdges(i int) (dfaEdgeList, error) {
	if i < 0 || i >= v.dfaStateCount() {
		return dfaEdgeList{}, invalid()
	}
	if v.legacy != nil {
		return dfaEdgeList{legacy: v.legacy.DFA.States[i].Edges}, nil
	}
	a, b, e := v.span(secDFAStates, i, 2, 5, v.sections[secDFAEdges].count)
	return dfaEdgeList{start: a, end: b, view: v}, e
}
func (v *tableView) text(a, b int) (string, error) {
	if a < 0 || a > b || b > len(v.c.Text) {
		return "", invalid()
	}
	return v.c.Text[a:b], nil
}
func (v *tableView) rule(i int) (Rule, error) {
	if i < 0 || i >= v.ruleCount() {
		return Rule{}, invalid()
	}
	if v.legacy != nil {
		return v.legacy.Rules[i], nil
	}
	var z [5]int
	for j := range z {
		n, e := v.number(secRules, i, j, 5)
		if e != nil {
			return Rule{}, e
		}
		z[j] = n
	}
	kinds := [4]string{"TOKEN", "SKIP", "MORE", "SPECIAL_TOKEN"}
	if z[0] == 0 || z[1] >= len(kinds) || z[2] > v.startCount() {
		return Rule{}, invalid()
	}
	action, e := v.text(z[3], z[4])
	if e != nil {
		return Rule{}, e
	}
	return Rule{Ordinal: z[0], Kind: kinds[z[1]], NextState: z[2] - 1, Action: action}, nil
}
func (v *tableView) names(sec int) ([]string, error) {
	out := make([]string, v.sections[sec].count)
	for i := range out {
		a, e := v.number(sec, i, 0, 2)
		if e != nil {
			return nil, e
		}
		b, e := v.number(sec, i, 1, 2)
		if e != nil {
			return nil, e
		}
		out[i], e = v.text(a, b)
		if e != nil {
			return nil, e
		}
	}
	return out, nil
}

// StateNames returns a fresh snapshot of lexical-state names.
func (c ConstTables) StateNames() ([]string, error) {
	v, e := newTableView(c)
	if e != nil {
		return nil, e
	}
	return v.names(secStateNames)
}

// TokenNames returns a fresh snapshot of token names.
func (c ConstTables) TokenNames() ([]string, error) {
	v, e := newTableView(c)
	if e != nil {
		return nil, e
	}
	return v.names(secTokenNames)
}
