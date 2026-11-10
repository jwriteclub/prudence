// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Package levelcollections models bounded Java HashMap list-bin histories.
package levelcollections

import (
	"errors"
	"math"
)

var (
	ErrLimit    = errors.New("collection resource limit")
	ErrTree     = errors.New("collection tree bin unsupported")
	ErrStopped  = errors.New("collection owner stopped")
	ErrCapacity = errors.New("invalid collection capacity")
)

const maxCapacity = 1 << 30

// Limits bound one owner's entries, table capacity, and cumulative work.
type Limits struct {
	Entries, Capacity, Work uint64
	Check                   func() error
}
type Owner struct {
	limits                  Limits
	entries, work, capacity uint64
	stopped                 error
}

func NewOwner(l Limits) *Owner { return &Owner{limits: l} }
func (o *Owner) step(n uint64) error {
	if o == nil {
		return ErrStopped
	}
	if o.stopped != nil {
		return o.stopped
	}
	if o.limits.Check != nil {
		if err := o.limits.Check(); err != nil {
			o.stopped = err
			return err
		}
	}
	if n > o.limits.Work || o.work > o.limits.Work-n {
		o.stopped = ErrLimit
		return o.stopped
	}
	o.work += n
	return nil
}
func (o *Owner) fail(err error) error {
	if o != nil && o.stopped == nil {
		o.stopped = err
	}
	return err
}

type entry[K any, V any] struct {
	hash  uint32
	key   K
	value V
	next  *entry[K, V]
}
type Map[K any, V any] struct {
	owner           *Owner
	equal           func(K, K) bool
	table           []*entry[K, V]
	threshold, size uint32
	iterating       bool
}

// NewMap creates a lazy default map. Equal keys must have equal supplied hashes.
func NewMap[K any, V any](o *Owner, equal func(K, K) bool) (*Map[K, V], error) {
	return newMap[K, V](o, equal, 0, false)
}

// NewMapCapacity models HashMap(initialCapacity).
func NewMapCapacity[K any, V any](o *Owner, equal func(K, K) bool, capacity int64) (*Map[K, V], error) {
	return newMap[K, V](o, equal, capacity, true)
}
func newMap[K any, V any](o *Owner, equal func(K, K) bool, capacity int64, explicit bool) (*Map[K, V], error) {
	if o == nil {
		return nil, ErrStopped
	}
	if err := o.step(1); err != nil {
		return nil, err
	}
	if equal == nil || capacity < 0 || capacity > math.MaxInt32 {
		return nil, o.fail(ErrCapacity)
	}
	if capacity > maxCapacity {
		capacity = maxCapacity
	}
	m := &Map[K, V]{owner: o, equal: equal}
	if explicit {
		m.threshold = tableSize(uint32(capacity))
	}
	return m, nil
}
func tableSize(n uint32) uint32 {
	if n <= 1 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	if n >= maxCapacity {
		return maxCapacity
	}
	return n + 1
}
func spread(h uint32) uint32 { return h ^ (h >> 16) }
func (m *Map[K, V]) grow() error {
	old := uint32(len(m.table))
	var capacity uint32
	if old == 0 {
		if m.threshold > 0 {
			capacity = m.threshold
		} else {
			capacity = 16
		}
	} else {
		if old >= maxCapacity {
			m.threshold = math.MaxInt32
			return nil
		}
		capacity = old * 2
	}
	if uint64(capacity) > m.owner.limits.Capacity || m.owner.capacity > m.owner.limits.Capacity-uint64(capacity) {
		return m.owner.fail(ErrLimit)
	}
	if err := m.owner.step(uint64(capacity) + uint64(old) + uint64(m.size) + 1); err != nil {
		return err
	}
	t := make([]*entry[K, V], int(capacity))
	tails := make([]*entry[K, V], int(capacity))
	for _, head := range m.table {
		for e := head; e != nil; {
			next := e.next
			e.next = nil
			i := e.hash & (capacity - 1)
			if t[i] == nil {
				t[i] = e
			} else {
				tails[i].next = e
			}
			tails[i] = e
			e = next
		}
	}
	m.table = t
	m.owner.capacity += uint64(capacity) - uint64(old)
	m.threshold = capacity * 3 / 4
	if capacity == maxCapacity {
		m.threshold = math.MaxInt32
	}
	return nil
}

// Put retains the original equal key and replaces only its value.
func (m *Map[K, V]) Put(key K, hash uint32, value V) error {
	if m.iterating {
		return m.owner.fail(ErrStopped)
	}
	if err := m.owner.step(1); err != nil {
		return err
	}
	h := spread(hash)
	if len(m.table) == 0 {
		if err := m.grow(); err != nil {
			return err
		}
	}
	i := h & uint32(len(m.table)-1)
	var tail *entry[K, V]
	count := 0
	for e := m.table[i]; e != nil; e = e.next {
		if err := m.owner.step(1); err != nil {
			return err
		}
		if e.hash == h && m.equal(key, e.key) {
			e.value = value
			return nil
		}
		tail = e
		count++
	}
	if m.owner.entries >= m.owner.limits.Entries || m.size == math.MaxInt32 {
		return m.owner.fail(ErrLimit)
	}
	if count >= 8 && len(m.table) >= 64 {
		return m.owner.fail(ErrTree)
	}
	e := &entry[K, V]{hash: h, key: key, value: value}
	if tail == nil {
		m.table[i] = e
	} else {
		tail.next = e
	}
	m.size++
	m.owner.entries++
	if count >= 8 && len(m.table) < 64 {
		if err := m.grow(); err != nil {
			return err
		}
	}
	if m.size > m.threshold {
		return m.grow()
	}
	return nil
}
func (m *Map[K, V]) Get(key K, hash uint32) (V, bool, error) {
	var zero V
	if err := m.owner.step(1); err != nil {
		return zero, false, err
	}
	if len(m.table) == 0 {
		return zero, false, nil
	}
	h := spread(hash)
	for e := m.table[h&uint32(len(m.table)-1)]; e != nil; e = e.next {
		if err := m.owner.step(1); err != nil {
			return zero, false, err
		}
		if e.hash == h && m.equal(key, e.key) {
			return e.value, true, nil
		}
	}
	return zero, false, nil
}
func (m *Map[K, V]) Remove(key K, hash uint32) (bool, error) {
	if m.iterating {
		return false, m.owner.fail(ErrStopped)
	}
	if err := m.owner.step(1); err != nil {
		return false, err
	}
	if len(m.table) == 0 {
		return false, nil
	}
	h := spread(hash)
	i := h & uint32(len(m.table)-1)
	var prev *entry[K, V]
	for e := m.table[i]; e != nil; e = e.next {
		if err := m.owner.step(1); err != nil {
			return false, err
		}
		if e.hash == h && m.equal(key, e.key) {
			if prev == nil {
				m.table[i] = e.next
			} else {
				prev.next = e.next
			}
			m.size--
			m.owner.entries--
			return true, nil
		}
		prev = e
	}
	return false, nil
}

// Each visits bucket and chain order. The callback must not mutate this map.
func (m *Map[K, V]) Each(f func(K, V, uint32) error) error {
	if f == nil || m.iterating {
		return m.owner.fail(ErrStopped)
	}
	m.iterating = true
	defer func() { m.iterating = false }()
	if err := m.owner.step(uint64(len(m.table)) + 1); err != nil {
		return err
	}
	for _, head := range m.table {
		for e := head; e != nil; e = e.next {
			if err := m.owner.step(1); err != nil {
				return err
			}
			if err := f(e.key, e.value, e.hash); err != nil {
				return m.owner.fail(err)
			}
			if err := m.owner.step(0); err != nil {
				return err
			}
		}
	}
	return nil
}

// PutAll models the custom constraint-map loop, without JDK presizing.
func (m *Map[K, V]) PutAll(src *Map[K, V]) error {
	if m == src {
		return m.owner.step(1)
	}
	return src.Each(func(k K, v V, h uint32) error { return m.Put(k, spread(h), v) })
}
func copyCapacity(n uint32) uint32 {
	f := float32(n)/float32(.75) + 1
	if f >= float32(maxCapacity) {
		return maxCapacity
	}
	return uint32(f)
}

// CopyMap models HashMap(Map)'s pre-sizing.
func CopyMap[K any, V any](o *Owner, src *Map[K, V]) (*Map[K, V], error) {
	m, err := NewMap[K, V](o, src.equal)
	if err != nil {
		return nil, err
	}
	if src.size > 0 {
		m.threshold = tableSize(copyCapacity(src.size))
	}
	if err = m.PutAll(src); err != nil {
		return nil, err
	}
	return m, nil
}

// CopySetMap models HashSet(Collection)'s minimum and copy sizing.
func CopySetMap[K any, V any](o *Owner, src *Map[K, V]) (*Map[K, V], error) {
	// Java casts the quotient before adding one, using signed int arithmetic.
	f := float32(src.size) / float32(.75)
	var q int32
	if f >= float32(math.MaxInt32) {
		q = math.MaxInt32
	} else {
		q = int32(f)
	}
	n := int32(uint32(q) + 1)
	capacity := uint32(16)
	if n > 16 {
		capacity = uint32(n)
	}
	if capacity < 16 {
		capacity = 16
	}
	m, err := NewMapCapacity[K, V](o, src.equal, int64(capacity))
	if err != nil {
		return nil, err
	}
	if err = m.PutAll(src); err != nil {
		return nil, err
	}
	return m, nil
}
