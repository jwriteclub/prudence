// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package levelcollections

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

type key struct {
	id   int
	name string
}

func owner() *Owner     { return NewOwner(Limits{Entries: 1000, Capacity: 10000, Work: 100000}) }
func eq(a, b *key) bool { return a.id == b.id }
func order(t *testing.T, m *Map[*key, int]) []int {
	t.Helper()
	var got []int
	if err := m.Each(func(k *key, _ int, _ uint32) error { got = append(got, k.id); return nil }); err != nil {
		t.Fatal(err)
	}
	return got
}
func TestHashAndOriginalKey(t *testing.T) {
	o := owner()
	m, _ := NewMap[*key, int](o, eq)
	a := &key{1, "original"}
	b := &key{1, "wrapper"}
	for _, x := range []struct {
		k *key
		h uint32
		v int
	}{{a, 0xffffffff, 1}, {&key{2, ""}, 0, 2}, {&key{3, ""}, 0x10000, 3}, {b, 0xffffffff, 4}} {
		if err := m.Put(x.k, x.h, x.v); err != nil {
			t.Fatal(err)
		}
	}
	var keys []*key
	m.Each(func(k *key, v int, _ uint32) error {
		keys = append(keys, k)
		if k.id == 1 && v != 4 {
			t.Fatal(v)
		}
		return nil
	})
	if !reflect.DeepEqual(keys, []*key{a, keys[1], keys[2]}) || !reflect.DeepEqual(order(t, m), []int{1, 2, 3}) {
		t.Fatalf("keys: %#v", keys)
	}
}
func TestResizeSplitRemoveReinsert(t *testing.T) {
	m, _ := NewMap[*key, int](owner(), eq)
	hashes := []uint32{0, 16, 32, 48, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	for i, h := range hashes {
		if err := m.Put(&key{i, ""}, h, i); err != nil {
			t.Fatal(err)
		}
		if i == 11 && (len(m.table) != 16 || m.threshold != 12) {
			t.Fatal(len(m.table), m.threshold)
		}
	}
	if len(m.table) != 32 {
		t.Fatal(len(m.table))
	}
	if got := order(t, m); !reflect.DeepEqual(got, []int{0, 2, 4, 5, 6, 7, 8, 9, 10, 11, 12, 1, 3}) {
		t.Fatal(got)
	}
	m.Remove(&key{2, ""}, 32)
	m.Put(&key{2, "new"}, 32, 99)
	if got := order(t, m); got[0] != 0 || got[1] != 2 {
		t.Fatal(got)
	}
}
func TestCopyAndPutAll(t *testing.T) {
	o := owner()
	src, _ := NewMap[*key, int](o, eq)
	for i := 0; i < 12; i++ {
		src.Put(&key{i, ""}, uint32(i), i)
	}
	cp, err := CopyMap(o, src)
	if err != nil {
		t.Fatal(err)
	}
	set, err := CopySetMap(o, src)
	if err != nil {
		t.Fatal(err)
	}
	custom, _ := NewMap[*key, int](o, eq)
	if err = custom.PutAll(src); err != nil {
		t.Fatal(err)
	}
	if len(cp.table) != 32 || len(set.table) != 32 || len(custom.table) != 16 {
		t.Fatal(len(cp.table), len(set.table), len(custom.table))
	}
	alias := src
	alias.Remove(&key{0, ""}, 0)
	if src.size != 11 || cp.size != 12 {
		t.Fatal(src.size, cp.size)
	}
	empty, _ := NewMap[*key, int](o, eq)
	mc, _ := CopyMap(o, empty)
	sc, _ := CopySetMap(o, empty)
	mc.Put(&key{99, ""}, 99, 1)
	sc.Put(&key{99, ""}, 99, 1)
	if len(mc.table) != 16 || len(sc.table) != 16 {
		t.Fatal(len(mc.table), len(sc.table))
	}
	single, _ := NewMap[*key, int](o, eq)
	single.Put(&key{1, ""}, 1, 1)
	mc, _ = CopyMap(o, single)
	sc, _ = CopySetMap(o, single)
	if len(mc.table) != 2 || len(sc.table) != 16 {
		t.Fatal(len(mc.table), len(sc.table))
	}
}
func TestTreeStop(t *testing.T) {
	o := owner()
	m, _ := NewMap[*key, int](o, eq)
	for i := 0; i < 10; i++ {
		if err := m.Put(&key{i, ""}, 5, i); err != nil {
			t.Fatal(i, err)
		}
		if i == 8 && len(m.table) != 32 {
			t.Fatal(len(m.table))
		}
		if i == 9 && len(m.table) != 64 {
			t.Fatal(len(m.table))
		}
	}
	if err := m.Put(&key{10, ""}, 5, 10); !errors.Is(err, ErrTree) {
		t.Fatal(err)
	}
	if _, _, err := m.Get(&key{0, ""}, 5); !errors.Is(err, ErrTree) {
		t.Fatal(err)
	}
	fresh, _ := NewMap[*key, int](owner(), eq)
	if err := fresh.Put(&key{0, ""}, 5, 0); err != nil {
		t.Fatal(err)
	}
}
func TestLimits(t *testing.T) {
	for _, cap := range []int64{-1, int64(math.MaxInt32) + 1} {
		o := owner()
		if _, err := NewMapCapacity[*key, int](o, eq, cap); !errors.Is(err, ErrCapacity) {
			t.Fatal(err)
		}
	}
	large, err := NewMapCapacity[*key, int](owner(), eq, math.MaxInt32)
	if err != nil || large.threshold != maxCapacity {
		t.Fatal(large, err)
	}
	if tableSize(0) != 1 || tableSize(17) != 32 || tableSize(maxCapacity) != maxCapacity {
		t.Fatal("rounding")
	}
	o := NewOwner(Limits{Entries: 1, Capacity: 100, Work: 1000})
	m, _ := NewMap[*key, int](o, eq)
	m.Put(&key{1, ""}, 1, 1)
	if err := m.Put(&key{2, ""}, 2, 2); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	o = NewOwner(Limits{Entries: 2, Capacity: 15, Work: 1000})
	m, _ = NewMap[*key, int](o, eq)
	if err := m.Put(&key{1, ""}, 1, 1); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	o = NewOwner(Limits{Entries: 2, Capacity: 100, Work: 1})
	m, _ = NewMap[*key, int](o, eq)
	if err := m.Put(&key{1, ""}, 1, 1); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	cancel := errors.New("cancel")
	o = NewOwner(Limits{Entries: 2, Capacity: 100, Work: 1000, Check: func() error { return cancel }})
	if _, err := NewMap[*key, int](o, eq); !errors.Is(err, cancel) {
		t.Fatal(err)
	}
}

func TestMutationDuringIterationStopsOwner(t *testing.T) {
	m, _ := NewMap[*key, int](owner(), eq)
	m.Put(&key{1, ""}, 1, 1)
	err := m.Each(func(_ *key, _ int, _ uint32) error { _ = m.Put(&key{2, ""}, 2, 2); return nil })
	if !errors.Is(err, ErrStopped) {
		t.Fatal(err)
	}
}
