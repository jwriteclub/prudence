// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Package sanylex supplies explicit bindings for the pinned SANY lexical actions.
package sanylex

import (
	"fmt"
	"sync"

	"github.com/jwriteclub/prudence/internal/lexer"
)

const Increment = "321689d68525e18e1157304f006ad4850d82a522de15d4cf21ffd24025d25b88"
const Decrement = "32b4e36eeaca3f5e98ba42a78807f52e1a354a52531034651cad332f113ab333"
const Declaration = "b81e5579c1b884d72c90d6cd03c4400f1745cc3752795401f6b2722258e9837d"

// Counter holds the lexical comment nesting state of its owner.
type Counter struct {
	mu    sync.Mutex
	value int
}

func (c *Counter) Value() int { c.mu.Lock(); defer c.mu.Unlock(); return c.value }

// Reset clears the nesting state without replacing the mutex.
func (c *Counter) Reset() { c.mu.Lock(); defer c.mu.Unlock(); c.value = 0 }

// Action applies bound actions using this counter and the generated state map.
func (c *Counter) Action(names []string) lexer.Action {
	comment := -1
	for i, name := range names {
		if name == "IN_COMMENT" {
			comment = i
		}
	}
	return func(l *lexer.Lexer, rule lexer.Rule, token *lexer.Token) error {
		c.mu.Lock()
		defer c.mu.Unlock()
		switch rule.Action {
		case Increment:
			c.value++
		case Decrement:
			c.value--
			if c.value == 0 {
				if comment < 0 {
					return fmt.Errorf("missing IN_COMMENT lexical state")
				}
				l.State = comment
			}
		default:
			return fmt.Errorf("unknown SANY lexical action %s", rule.Action)
		}
		return nil
	}
}
