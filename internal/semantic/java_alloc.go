// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

// javaAlloc models only the fresh Phase2Probe prefix through a first, direct
// label check. It is separate from graph identities and becomes permanently
// inexact when an unmodeled event may have occurred.
type javaAlloc struct {
	exact                     bool
	count                     uint64
	tickets                   map[*formalParamNode]javaTicket
	atCheck                   uint64
	generatorUsed, moduleUsed bool
}
type javaClass uint8

const classFormalParam javaClass = 1

type javaTicket struct {
	uid   uint32
	kind  int
	class javaClass
	known bool
}

func (a *javaAlloc) event() javaTicket {
	if !a.exact || a.count >= (uint64(1)<<32)-1 {
		a.exact = false
		return javaTicket{}
	}
	t := javaTicket{uid: uint32(a.count), known: true}
	a.count++
	return t
}
func freshJavaAlloc() javaAlloc {
	a := javaAlloc{exact: true, tickets: make(map[*formalParamNode]javaTicket)}
	a.event() // nullSN
	descriptors, slots := 0, 0
	for _, d := range builtinDescriptors() {
		descriptors++
		if d.arity > 0 {
			slots += d.arity
		}
	}
	if descriptors != 72 || slots != 77 {
		a.invalidate()
		return a
	}
	for pass := 0; pass < 2; pass++ {
		for _, d := range builtinDescriptors() {
			a.event()
			for i := 0; i < d.arity; i++ {
				a.event()
			}
		}
	}
	a.event() // static dummy
	return a
}
func (a *javaAlloc) generator() {
	if a.generatorUsed {
		a.invalidate()
		return
	}
	a.generatorUsed = true
	for i := 0; i < 4; i++ {
		a.event()
	} // recovery nodes
}
func (a *javaAlloc) module() {
	if a.moduleUsed {
		a.invalidate()
		return
	}
	a.moduleUsed = true
	a.event()
}
func (a *javaAlloc) formal(p *formalParamNode) {
	t := a.event()
	t.kind = 11
	t.class = classFormalParam
	if a.tickets != nil {
		a.tickets[p] = t
	}
}
func (a *javaAlloc) invalidate() { a.exact = false }

// orderedLabelExtras implements the bounded fresh default-set contract. The
// insertion sequence includes entries subsequently removed and duplicate adds.
func (a *javaAlloc) orderedLabelExtras(params []*formalParamNode, remaining map[*formalParamNode]bool) ([]*formalParamNode, bool) {
	if !a.exact || a.count >= uint64(1)<<32 {
		return nil, false
	}
	seen := make(map[*formalParamNode]bool)
	var buckets [16][]*formalParamNode
	for _, p := range params {
		if seen[p] {
			continue
		}
		seen[p] = true
		if len(seen) > 7 {
			return nil, false
		}
		t, ok := a.tickets[p]
		if !ok || !t.known || t.class != classFormalParam || t.kind != 11 {
			return nil, false
		}
		h := uint32(1302) + t.uid
		h ^= h >> 16
		if remaining[p] {
			buckets[h&15] = append(buckets[h&15], p)
		}
	}
	var out []*formalParamNode
	for _, b := range buckets {
		out = append(out, b...)
	}
	return out, len(out) == len(remaining)
}
