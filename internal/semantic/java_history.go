// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"reflect"
)

// javaHistory is independent of graph IDs and of the bounded label cursor.
// Its profile is the fresh Phase2Probe entry described by UID-BOOTSTRAP-EVIDENCE.
// A zero value is unprofiled and cannot certify Java identities.
type javaHistory struct {
	profile                javaHistoryProfile
	ctx                    stdcontext.Context
	exact                  bool
	count                  uint64
	firstUnknown           string
	tickets                map[any]javaHistoryTicket
	builtins               []javaBuiltinTickets
	staticDummy            javaHistoryTicket
	workLimit, ticketLimit uint64
}
type javaHistoryProfile uint8

const (
	freshPhase2Profile  javaHistoryProfile = 1
	levelEnabledProfile javaHistoryProfile = 2
)

type javaHistoryTicket struct {
	class string
	kind  int
	uid   uint32
	known bool
}
type javaBuiltinTickets struct {
	def    javaHistoryTicket
	params []javaHistoryTicket
}

func (h *javaHistory) invalidate(reason string) {
	if h.exact {
		h.exact = false
		h.firstUnknown = reason
	}
}
func (h *javaHistory) event(class string, kind int) javaHistoryTicket {
	if !h.exact {
		return javaHistoryTicket{}
	}
	if h.ctx == nil || h.ctx.Err() != nil {
		h.invalidate("cancelled")
		return javaHistoryTicket{}
	}
	if h.count >= h.workLimit || h.count >= (uint64(1)<<32)-1 {
		h.invalidate("event budget or UID boundary")
		return javaHistoryTicket{}
	}
	t := javaHistoryTicket{class: class, kind: kind, uid: uint32(h.count), known: true}
	h.count++
	return t
}

// attach never creates an event. It attaches a reserved ticket to its Go
// representative, and leaves existing tickets immutable.
func (h *javaHistory) attach(node any, t javaHistoryTicket) {
	if node == nil || !t.known {
		return
	}
	if h.tickets == nil || reflect.TypeOf(node).Kind() != reflect.Pointer || reflect.ValueOf(node).IsNil() {
		h.invalidate("invalid attachment")
		return
	}
	if old, ok := h.tickets[node]; ok {
		if old != t {
			h.invalidate("conflicting attachment")
		}
		return
	}
	if uint64(len(h.tickets)) >= h.ticketLimit {
		h.invalidate("ticket budget")
		return
	}
	h.tickets[node] = t
}
func (h *javaHistory) reuse(node any) javaHistoryTicket {
	if node == nil || reflect.TypeOf(node).Kind() != reflect.Pointer || reflect.ValueOf(node).IsNil() {
		return javaHistoryTicket{}
	}
	return h.tickets[node]
}
func freshJavaHistory(ctx stdcontext.Context) javaHistory {
	h := javaHistory{profile: freshPhase2Profile, ctx: ctx, exact: true, tickets: make(map[any]javaHistoryTicket), workLimit: 1 << 20, ticketLimit: 1 << 18}
	h.event("NullSemanticNode", -1<<31)
	ds := builtinDescriptors()
	slots := 0
	for _, d := range ds {
		if d.arity > 0 {
			slots += d.arity
		}
	}
	if len(ds) != 72 || slots != 77 {
		h.invalidate("builtin descriptor schedule")
		return h
	}
	for pass := 0; pass < 2; pass++ {
		for _, d := range ds {
			b := javaBuiltinTickets{def: h.event("OpDefNode", 7)}
			for i := 0; i < d.arity; i++ {
				b.params = append(b.params, h.event("FormalParamNode", 11))
			}
			if pass == 1 {
				h.builtins = append(h.builtins, b)
			}
		}
	}
	h.staticDummy = h.event("OpDeclNode", 0) // Generator's static InAssumeDummyNode
	return h
}

// freshLevelEnabledHistory binds the separately selected SANY level-enabled
// driver entry to its fixed bootstrap schedule. The same initialized classes
// precede generation under this entry; completed closed-world level calls do
// not allocate SemanticNodes. Unknown paths still invalidate permanently.
func freshLevelEnabledHistory(ctx stdcontext.Context) javaHistory {
	h := javaHistory{profile: levelEnabledProfile, ctx: ctx, exact: true, tickets: make(map[any]javaHistoryTicket), workLimit: 1 << 20, ticketLimit: 1 << 18}
	// SANY's enabled driver reaches the same SemanticNode/Context bootstrap
	// before its first Generator. This schedule is owned by this profile.
	h.event("NullSemanticNode", -1<<31)
	ds := builtinDescriptors()
	slots := 0
	for _, d := range ds {
		if d.arity > 0 {
			slots += d.arity
		}
	}
	if len(ds) != 72 || slots != 77 {
		h.invalidate("enabled bootstrap descriptor schedule")
		return h
	}
	for pass := 0; pass < 2; pass++ {
		for _, d := range ds {
			b := javaBuiltinTickets{def: h.event("OpDefNode", 7)}
			for i := 0; i < d.arity; i++ {
				b.params = append(b.params, h.event("FormalParamNode", 11))
			}
			if pass == 1 {
				h.builtins = append(h.builtins, b)
			}
		}
	}
	h.staticDummy = h.event("OpDeclNode", 0)
	return h
}
