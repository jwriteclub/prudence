// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package lexer

// DFAEdge is an inclusive UTF-16 range whose target is encoded as state+1.
type DFAEdge struct {
	Low, High uint16
	Target    int
}

// DFAState has direct ASCII dispatch and sorted, non-overlapping non-ASCII ranges.
// Zero denotes no transition; Accept is a rule index or -1.
type DFAState struct {
	ASCII  [128]int
	Edges  []DFAEdge
	Accept int
}

// DFA contains epsilon-closed subsets of a generated NFA.
type DFA struct {
	Starts []int
	States []DFAState
}

type dfaMemoKey struct{ state, offset int }
type dfaMemoResult struct{ rule, end, failed int }
type dfaStep struct {
	key      dfaMemoKey
	accepted int
}

// dfaTarget checks each accessed transition while retaining the binary search.
func (l *Lexer) dfaTarget(state int, c uint16, p *cancelProbe) (int, error) {
	if c < 128 {
		return l.tables.dfaASCII(state, int(c))
	}
	edges, err := l.tables.dfaEdges(state)
	if err != nil {
		return 0, err
	}
	lo, hi := 0, edges.Len()
	for lo < hi {
		if err := p.stepIfActive(); err != nil {
			return 0, err
		}
		mid := lo + (hi-lo)/2
		edge, err := edges.At(mid)
		if err != nil {
			return 0, err
		}
		if edge.High < c {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < edges.Len() {
		edge, err := edges.At(lo)
		if err != nil {
			return 0, err
		}
		if edge.Low <= c {
			return edge.Target, nil
		}
	}
	return 0, nil
}

// matchDFA memoizes only long pure automaton walks.
func (l *Lexer) matchDFA(p *cancelProbe) (int, int, int, error) {
	if l.dfaMemo != nil {
		return l.matchDFAMemo(p)
	}
	state, err := l.tables.dfaStart(l.State)
	if err != nil {
		return 0, 0, 0, err
	}
	bestRule, bestEnd, failed := -1, l.cursor, l.cursor
	lastValidated := -1
	for i := l.cursor; i < len(l.input); i++ {
		if err := p.stepIfActive(); err != nil {
			return 0, 0, 0, err
		}
		failed = i
		target, err := l.dfaTarget(state, l.input[i], p)
		if err != nil {
			return 0, 0, 0, err
		}
		if target == 0 {
			break
		}
		state = target - 1
		accepted, err := l.tables.dfaAccept(state)
		if err != nil {
			return 0, 0, 0, err
		}
		if accepted >= 0 {
			if accepted != lastValidated {
				if _, err := l.tables.rule(accepted); err != nil {
					return 0, 0, 0, err
				}
				lastValidated = accepted
			}
			bestRule, bestEnd = accepted, i+1
		}
	}
	if failed+1-bestEnd > 64 {
		l.dfaMemo = make(map[dfaMemoKey]dfaMemoResult)
		return l.matchDFAMemo(p)
	}
	return bestRule, bestEnd, failed, nil
}

func (l *Lexer) matchDFAMemo(p *cancelProbe) (int, int, int, error) {
	state, err := l.tables.dfaStart(l.State)
	if err != nil {
		return 0, 0, 0, err
	}
	best := dfaMemoResult{rule: -1, end: l.cursor, failed: l.cursor}
	lastValidated := -1
	var local [64]dfaStep
	path := local[:0]
	finish := func(tail dfaMemoResult) (int, int, int, error) {
		if l.dfaMemo != nil {
			for i := len(path) - 1; i >= 0; i-- {
				if err := p.stepIfActive(); err != nil {
					return 0, 0, 0, err
				}
				step := path[i]
				if step.accepted >= 0 && (tail.rule < 0 || tail.end <= step.key.offset+1) {
					tail.rule, tail.end = step.accepted, step.key.offset+1
				}
				l.dfaMemo[step.key] = tail
			}
			if tail.rule < 0 {
				tail.end = l.cursor
			}
			return tail.rule, tail.end, tail.failed, nil
		}
		return best.rule, best.end, best.failed, nil
	}
	for i := l.cursor; i < len(l.input); i++ {
		if err := p.stepIfActive(); err != nil {
			return 0, 0, 0, err
		}
		key := dfaMemoKey{state, i}
		if l.dfaMemo != nil {
			if suffix, ok := l.dfaMemo[key]; ok {
				return finish(suffix)
			}
		}
		best.failed = i
		target, err := l.dfaTarget(state, l.input[i], p)
		if err != nil {
			return 0, 0, 0, err
		}
		if target == 0 {
			tail := dfaMemoResult{rule: -1, failed: i}
			if l.dfaMemo != nil {
				l.dfaMemo[key] = tail
			}
			return finish(tail)
		}
		state = target - 1
		accepted, err := l.tables.dfaAccept(state)
		if err != nil {
			return 0, 0, 0, err
		}
		if accepted >= 0 {
			if accepted != lastValidated {
				if _, err := l.tables.rule(accepted); err != nil {
					return 0, 0, 0, err
				}
				lastValidated = accepted
			}
			best.rule, best.end = accepted, i+1
		}
		if len(path) == len(local) && l.dfaMemo == nil {
			l.dfaMemo = make(map[dfaMemoKey]dfaMemoResult)
		}
		path = append(path, dfaStep{key: key, accepted: accepted})
	}
	return finish(dfaMemoResult{rule: -1, failed: len(l.input) - 1})
}
