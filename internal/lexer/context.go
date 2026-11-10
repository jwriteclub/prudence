// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package lexer

import (
	"context"
	"fmt"
)

// InterruptionError reports a canceled token request. It unwraps to the context
// error and is separate from lexical and recovery errors.
type InterruptionError struct{ cause error }

func (e *InterruptionError) Error() string { return "lexer interrupted: " + e.cause.Error() }
func (e *InterruptionError) Unwrap() error { return e.cause }

const cancellationWorkInterval = 256

type cancelProbe struct {
	ctx  context.Context
	work int
}

func (p *cancelProbe) check() error {
	if err := p.ctx.Err(); err != nil {
		return &InterruptionError{cause: err}
	}
	return nil
}
func (p *cancelProbe) step() error {
	p.work++
	if p.work >= cancellationWorkInterval {
		p.work = 0
		return p.check()
	}
	return nil
}

// NextTokenContext cooperatively interrupts matching. Once a rule starts
// committing, its action and state transition finish before cancellation is
// observed at the next safe boundary. A completed regular token can therefore
// win a race with cancellation. A fresh context can resume after interruption.
func (l *Lexer) NextTokenContext(ctx context.Context) (*Token, error) {
	if ctx == nil {
		return nil, fmt.Errorf("lexer context must not be nil")
	}
	probe := &cancelProbe{ctx: ctx}
	if err := probe.check(); err != nil {
		return nil, err
	}
	return l.nextToken(probe)
}

// stepIfActive preserves the cancellation accounting sites for context calls.
func (p *cancelProbe) stepIfActive() error {
	if p == nil {
		return nil
	}
	return p.step()
}

func (l *Lexer) closure(seed []int, p *cancelProbe) ([]int, error) {
	seen := make([]bool, l.tables.stateCount())
	out := append([]int(nil), seed...)
	for _, n := range seed {
		if err := p.stepIfActive(); err != nil {
			return nil, err
		}
		if err := l.tables.stateID(n); err != nil {
			return nil, err
		}
		seen[n] = true
	}
	for i := 0; i < len(out); i++ {
		if err := p.stepIfActive(); err != nil {
			return nil, err
		}
		list, err := l.tables.epsilon(out[i])
		if err != nil {
			return nil, err
		}
		for j := 0; j < list.Len(); j++ {
			if err := p.stepIfActive(); err != nil {
				return nil, err
			}
			n, err := list.At(j)
			if err != nil {
				return nil, err
			}
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out, nil
}

func (l *Lexer) matchNFA(p *cancelProbe) (int, int, int, error) {
	start, err := l.tables.start(l.State)
	if err != nil {
		return 0, 0, 0, err
	}
	active, err := l.closure([]int{start}, p)
	if err != nil {
		return 0, 0, 0, err
	}
	bestRule, bestEnd, bestOrdinal := -1, l.cursor, 0
	fail := l.cursor
	for i := l.cursor; i < len(l.input); i++ {
		if err := p.stepIfActive(); err != nil {
			return 0, 0, 0, err
		}
		fail = i
		seen := make([]bool, l.tables.stateCount())
		var next []int
		for _, id := range active {
			if err := p.stepIfActive(); err != nil {
				return 0, 0, 0, err
			}
			edges, err := l.tables.edges(id)
			if err != nil {
				return 0, 0, 0, err
			}
			for j := 0; j < edges.Len(); j++ {
				if err := p.stepIfActive(); err != nil {
					return 0, 0, 0, err
				}
				e, err := edges.At(j)
				if err != nil {
					return 0, 0, 0, err
				}
				for k := 0; k < e.Ranges.Len(); k++ {
					if err := p.stepIfActive(); err != nil {
						return 0, 0, 0, err
					}
					r, err := e.Ranges.At(k)
					if err != nil {
						return 0, 0, 0, err
					}
					if l.input[i] >= r.Low && l.input[i] <= r.High {
						if !seen[e.To] {
							seen[e.To] = true
							next = append(next, e.To)
						}
						break
					}
				}
			}
		}
		if len(next) == 0 {
			break
		}
		active, err = l.closure(next, p)
		if err != nil {
			return 0, 0, 0, err
		}
		for _, id := range active {
			if err := p.stepIfActive(); err != nil {
				return 0, 0, 0, err
			}
			accepts, err := l.tables.accepts(id)
			if err != nil {
				return 0, 0, 0, err
			}
			for j := 0; j < accepts.Len(); j++ {
				if err := p.stepIfActive(); err != nil {
					return 0, 0, 0, err
				}
				ruleID, err := accepts.At(j)
				if err != nil {
					return 0, 0, 0, err
				}
				rule, err := l.tables.rule(ruleID)
				if err != nil {
					return 0, 0, 0, err
				}
				if bestEnd != i+1 || bestRule < 0 || rule.Ordinal < bestOrdinal {
					bestRule, bestEnd, bestOrdinal = ruleID, i+1, rule.Ordinal
				}
			}
		}
	}
	return bestRule, bestEnd, fail, nil
}
