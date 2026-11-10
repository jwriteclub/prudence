// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Package generated contains support for the generated SANY parser.
// UniqueString behavior derives from TLA+ util/UniqueString.java,
// Copyright (c) 2003 Compaq Corporation, distributed under the MIT license.
package generated

import (
	"errors"
	"sync"
	"unicode/utf16"
)

// UniqueString is an immutable sequence of Java UTF-16 units, interned within a support context.
type UniqueString struct{ units []uint16 }

// Interner owns one identity domain; its zero value is ready to use.
// Do not copy it after use.
type Interner struct {
	sync.Mutex
	values map[string]*UniqueString
}

func (c *Interner) UniqueStringOf(s string) *UniqueString {
	return c.UniqueStringOfUnits(utf16.Encode([]rune(s)))
}
func (c *Interner) UniqueStringOfUnits(units []uint16) *UniqueString {
	key := make([]byte, 2*len(units))
	for i, u := range units {
		key[2*i], key[2*i+1] = byte(u>>8), byte(u)
	}
	c.Lock()
	defer c.Unlock()
	if u := c.values[string(key)]; u != nil {
		return u
	}
	if c.values == nil {
		c.values = make(map[string]*UniqueString)
	}
	u := &UniqueString{units: append([]uint16(nil), units...)}
	c.values[string(key)] = u
	return u
}

// Units returns a copy of the original Java string units.
func (u *UniqueString) Units() []uint16 {
	if u == nil {
		return nil
	}
	return append([]uint16(nil), u.units...)
}

// String returns the decoded view, replacing unpaired surrogates.
func (u *UniqueString) String() string {
	if u == nil {
		return "<nil>"
	}
	return string(utf16.Decode(u.units))
}

// Concat interns the concatenated original units. Nil operands are invalid.
func (c *Interner) Concat(u, v *UniqueString) (*UniqueString, error) {
	if u == nil || v == nil {
		return nil, errors.New("sanyparser: cannot concatenate nil UniqueString")
	}
	units := make([]uint16, 0, len(u.units)+len(v.units))
	units = append(units, u.units...)
	units = append(units, v.units...)
	return c.UniqueStringOfUnits(units), nil
}
