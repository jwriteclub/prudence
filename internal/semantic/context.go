// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ Context.java, SymbolTable.java and SymbolNode.java,
// Copyright 2003 Compaq Corporation, 2003 Microsoft Corporation, and
// 2025 Linux Foundation, under the MIT license; see LICENSE.
package semantic

import (
	"unicode/utf16"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

// This internal foundation represents only the explicitly modeled node classes.
// It is not a Generator, a complete AST, or semantic-check completion.
type name struct {
	text         string
	units        []uint16
	displayExact bool
	identity     uint64
	owner        *namePool
}
type namePool struct {
	java     javaAlloc
	history  javaHistory
	names    map[string]*name
	next     uint64
	nextNode uint64
	syntax   *generated.SupportContext
	builtins *context
}

func newNamePool() *namePool {
	return &namePool{names: make(map[string]*name), syntax: generated.NewSupportContext()}
}
func (p *namePool) intern(s string) *name { return p.internUnits(utf16.Encode([]rune(s))) }

// lookupUnits examines an existing interned name without allocating a new identity.
func (p *namePool) lookupUnits(units []uint16) *name {
	key := make([]byte, 2*len(units))
	for i, u := range units {
		key[2*i] = byte(u >> 8)
		key[2*i+1] = byte(u)
	}
	return p.names[string(key)]
}

// internUnits keys the raw Java UTF-16 units; decoding alone is lossy for an
// unpaired surrogate. Diagnostic rendering is separately guarded below.
func (p *namePool) internUnits(units []uint16) *name {
	key := make([]byte, 2*len(units))
	for i, u := range units {
		key[2*i] = byte(u >> 8)
		key[2*i+1] = byte(u)
	}
	if n := p.names[string(key)]; n != nil {
		return n
	}
	p.next++
	text := string(utf16.Decode(units))
	round := utf16.Encode([]rune(text))
	exact := len(round) == len(units)
	if exact {
		for i := range units {
			if units[i] != round[i] {
				exact = false
				break
			}
		}
	}
	n := &name{text: text, units: append([]uint16(nil), units...), displayExact: exact, identity: p.next, owner: p}
	p.names[string(key)] = n
	return n
}

type nodeClass uint8

const (
	classOpDef nodeClass = iota + 1
	classFormal
	classOpDecl
	classModule
	classTheorem
)
const (
	kindModule  = 1
	kindBound   = 4
	kindBuiltIn = 7
	kindFormal  = 11
)

type builtinDescriptor struct {
	text         string
	arity, level int
	max, weights []int
}

func nodeClassOf(n leafNode) nodeClass {
	switch v := n.(type) {
	case *userOpDefNode:
		if v != nil {
			return classOpDef
		}
	case *builtinOpDefNode:
		if v != nil {
			return classOpDef
		}
	case *thmOrAssumpDefNode:
		if v != nil {
			return classTheorem
		}
	case *formalParamNode:
		if v != nil {
			return classFormal
		}
	case *opDeclNode:
		if v != nil {
			return classOpDecl
		}
	case *moduleNode:
		if v != nil {
			return classModule
		}
	}
	return 0
}
func nodeArity(n leafNode) int {
	switch v := n.(type) {
	case *userOpDefNode:
		return v.arity
	case *builtinOpDefNode:
		return v.arity
	case *thmOrAssumpDefNode:
		return len(v.params)
	case *formalParamNode:
		return v.arity
	case *opDeclNode:
		return v.arity
	case *moduleNode:
		return -2
	}
	return 0
}
func nodeLocal(n leafNode) bool {
	if v, ok := n.(*thmOrAssumpDefNode); ok {
		return v.local
	}
	if v, ok := n.(*userOpDefNode); ok {
		return v.local
	}
	_, ok := n.(*formalParamNode)
	return ok
}

func sameOriginallyDefined(a, b leafNode) bool {
	if x, ok := a.(*thmOrAssumpDefNode); ok {
		y, ok := b.(*thmOrAssumpDefNode)
		if !ok {
			return false
		}
		if x.source != nil {
			x = x.source
		}
		if y.source != nil {
			y = y.source
		}
		return x == y && (x.module == nil || x.module.parameterFree())
	}
	if a == b {
		switch n := a.(type) {
		case *builtinOpDefNode:
			return true
		case *userOpDefNode:
			return n.module == nil || n.module.parameterFree()
		}
		return false
	}
	x, ok := a.(*userOpDefNode)
	if !ok {
		return false
	}
	y, ok := b.(*userOpDefNode)
	if !ok {
		return false
	}
	if x.source != nil {
		x = x.source
	}
	if y.source != nil {
		y = y.source
	}
	return x == y && (x.module == nil || x.module.parameterFree())
}

type contextKey struct {
	name   *name
	module bool
}
type pair struct {
	value leafNode
	prior *pair
}
type context struct {
	owner      *namePool
	table      map[contextKey]*pair
	last       *pair
	order      [][]contextKey
	orderCount int
}

func newContext(owner *namePool) *context {
	return &context{owner: owner, table: make(map[contextKey]*pair), order: make([][]contextKey, 11)}
}

// add follows table.put plus linked insertion even when a key was present.
func (c *context) add(k contextKey, s leafNode) {
	if _, exists := c.table[k]; !exists {
		c.recordKey(k)
	}
	v := &pair{value: s, prior: c.last}
	c.last = v
	c.table[k] = v
}
func (c *context) lookup(k contextKey) leafNode {
	if v := c.table[k]; v != nil {
		return v.value
	}
	return nil
}

// duplicate follows Context.duplicate's reverse walk and rekeys by symbol name.
func (c *context) duplicate() *context {
	d := newContext(c.owner)
	var tail *pair
	for p := c.last; p != nil; p = p.prior {
		n := &pair{value: p.value}
		if tail == nil {
			d.last = n
		} else {
			tail.prior = n
		}
		tail = n
		key := contextKey{name: p.value.header().name}
		if _, exists := d.table[key]; !exists {
			d.recordKey(key)
		}
		d.table[key] = n
	}
	return d
}

// recordKey tracks ordering events independently of the symbol-value map.
func (c *context) recordKey(key contextKey) {
	if c.orderCount >= len(c.order)*3/4 {
		old := c.order
		c.order = make([][]contextKey, len(old)*2+1)
		for i := len(old) - 1; i >= 0; i-- {
			for j := len(old[i]) - 1; j >= 0; j-- {
				c.prependKey(old[i][j])
			}
		}
	}
	c.prependKey(key)
	c.orderCount++
}

// Chains store oldest keys first so adding a new logical head is amortized constant time.
func (c *context) prependKey(key contextKey) {
	var hash uint32
	for _, unit := range key.name.units {
		hash = hash*31 + uint32(unit)
	}
	i := int(hash&0x7fffffff) % len(c.order)
	c.order[i] = append(c.order[i], key)
}
func (c *context) orderedValues() []leafNode {
	values := make([]leafNode, 0, c.orderCount)
	for i := len(c.order) - 1; i >= 0; i-- {
		for j := len(c.order[i]) - 1; j >= 0; j-- {
			values = append(values, c.table[c.order[i][j]].value)
		}
	}
	return values
}

func displayKind(s leafNode) string {
	if nodeClassOf(s) == classOpDef || nodeClassOf(s) == classFormal {
		return "definition"
	}
	return "declaration"
}
func strParam(x string) Parameter {
	return Parameter{Kind: StringParameter, JavaClass: "java.lang.String", Text: x}
}

// exactStringParam is reserved for source-proved text: fixed ASCII or a
// value assembled from text already checked for lossless representation.
func exactStringParam(x string) Parameter {
	return Parameter{Kind: StringParameter, JavaClass: "java.lang.String", Text: x, UTF16Units: utf16.Encode([]rune(x)), UTF16Present: true}
}
func nameStringParam(n *name) Parameter {
	return Parameter{Kind: StringParameter, JavaClass: "java.lang.String", Text: n.text, UTF16Units: append([]uint16{}, n.units...), UTF16Present: true, TextUnrepresentable: !n.displayExact}
}
func asciiStringParam(x string) Parameter {
	for i := range x {
		if x[i] >= 0x80 {
			return strParam(x)
		}
	}
	return exactStringParam(x)
}
func nameParam(n *name) Parameter {
	return Parameter{Kind: ObjectIdentityParameter, JavaClass: "util.UniqueString", Identity: n.identity, Text: n.text, UTF16Units: append([]uint16{}, n.units...), UTF16Present: true, TextUnrepresentable: !n.displayExact}
}
func locParam(l Location) Parameter { return Parameter{Kind: LocationParameter, Location: l} }
func addError(e *Errors, code int, loc Location, format string, params ...Parameter) error {
	c, ok := CodeFromValue(code)
	if !ok {
		return ErrPortIncomplete
	}
	_, err := e.Add(c, &loc, format, params...)
	return err
}
func (c *context) mergeExtend(src *context, e *Errors) (bool, error) {
	if src == nil || e == nil || c.owner == nil || src.owner != c.owner {
		return false, ErrPortIncomplete
	}
	var reverse []leafNode
	for p := src.last; p != nil; p = p.prior {
		reverse = append(reverse, p.value)
	}
	ok := true
	for i := len(reverse) - 1; i >= 0; i-- {
		s := reverse[i]
		if nodeLocal(s) {
			continue
		}
		name := s.header().name
		key := contextKey{name: name, module: nodeClassOf(s) == classModule}
		old := c.lookup(key)
		if old == nil {
			c.add(key, s)
			continue
		}
		if old == s {
			continue
		}
		if nodeClassOf(old) == nodeClassOf(s) && sameOriginallyDefined(old, s) {
			continue
		}
		if !name.displayExact {
			return false, ErrPortIncomplete
		}
		sl, err := nodeLocation(s)
		if err != nil {
			return false, err
		}
		ol, err := nodeLocation(old)
		if err != nil {
			return false, err
		}
		if nodeClassOf(old) == nodeClassOf(s) {
			if err = addError(e, 4800, sl, "Warning: the %s of '%s' conflicts with \nits %s at %s.", exactStringParam(displayKind(old)), nameStringParam(name), exactStringParam(displayKind(old)), locParam(ol)); err != nil {
				return false, err
			}
		} else {
			if err = addError(e, 4224, sl, "The %s of '%s' conflicts with \nits %s at %s.", exactStringParam(displayKind(s)), nameStringParam(name), exactStringParam(displayKind(old)), locParam(ol)); err != nil {
				return false, err
			}
			ok = false
		}
	}
	return ok, nil
}

type symbolTable struct {
	owner     *namePool
	stack     []*context
	base, top *context
	external  map[*name]leafNode
	errors    *Errors
	module    *moduleNode
}

func newTable(pool *namePool, external map[*name]leafNode, e *Errors) (*symbolTable, error) {
	if pool == nil || e == nil || external == nil {
		return nil, ErrPortIncomplete
	}
	for n, s := range external {
		if n == nil || n.owner != pool || s == nil || nodeClassOf(s) != classModule || s.header().owner != pool {
			return nil, ErrPortIncomplete
		}
	}
	if pool.builtins == nil {
		c := newContext(pool)
		for i, d := range builtinDescriptors() {
			b := d
			s, err := newBuiltin(pool, &b)
			if err != nil {
				return nil, err
			}
			if i < len(pool.history.builtins) {
				bt := pool.history.builtins[i]
				pool.history.attach(s, bt.def)
				for j, param := range s.params {
					if j < len(bt.params) {
						pool.history.attach(param, bt.params[j])
					}
				}
			}
			if c.lookup(contextKey{name: s.name}) != nil {
				return nil, ErrPortIncomplete
			}
			c.add(contextKey{name: s.name}, s)
		}
		pool.builtins = c
	}
	base := pool.builtins.duplicate()
	return &symbolTable{owner: pool, stack: []*context{base}, base: base, top: base, external: external, errors: e}, nil
}

// newInternalTable copies the stack container but shares its Context objects.
// Like SANY's internal-module constructor, top remains nil until push.
func newInternalTable(external map[*name]leafNode, e *Errors, parent *symbolTable) (*symbolTable, error) {
	if parent == nil || parent.owner == nil || e == nil || external == nil {
		return nil, ErrPortIncomplete
	}
	owner := parent.owner
	if parent.module != nil && parent.module.owner != owner {
		return nil, ErrPortIncomplete
	}
	for n, s := range external {
		if n == nil || n.owner != owner || nodeClassOf(s) != classModule || s.header().owner != owner {
			return nil, ErrPortIncomplete
		}
	}
	stack := append([]*context(nil), parent.stack...)
	for _, c := range stack {
		if c == nil || c.owner != owner {
			return nil, ErrPortIncomplete
		}
	}
	var base *context
	if len(stack) > 0 {
		base = stack[0]
	}
	return &symbolTable{owner: owner, stack: stack, base: base, top: nil, external: external, errors: e, module: parent.module}, nil
}
func (t *symbolTable) setModuleNode(module *moduleNode) error {
	if t == nil || t.owner == nil || (module != nil && module.owner != t.owner) {
		return ErrPortIncomplete
	}
	t.module = module
	return nil
}
func (t *symbolTable) getModuleNode() *moduleNode { return t.module }

func (t *symbolTable) push(c *context) error {
	if c == nil || c.owner != t.owner {
		return ErrPortIncomplete
	}
	t.stack = append(t.stack, c)
	t.top = c
	return nil
}
func (t *symbolTable) pop() error {
	if len(t.stack) == 0 {
		return ErrPortIncomplete
	}
	t.stack = t.stack[:len(t.stack)-1]
	t.top = nil
	if len(t.stack) > 0 {
		t.top = t.stack[len(t.stack)-1]
	}
	return nil
}
func (t *symbolTable) resolve(n *name, module bool) (leafNode, error) {
	if n == nil || n.owner != t.owner {
		return nil, ErrPortIncomplete
	}
	for i := len(t.stack) - 1; i >= 0; i-- {
		if s := t.stack[i].lookup(contextKey{name: n, module: module}); s != nil {
			return s, nil
		}
	}
	if module && t.external != nil {
		s := t.external[n]
		if s != nil && (nodeClassOf(s) != classModule || s.header().owner != t.owner) {
			return nil, ErrPortIncomplete
		}
		return s, nil
	}
	return nil, nil
}
func (t *symbolTable) addSymbol(n *name, s leafNode) (bool, error) {
	if n == nil || nodeClassOf(s) == 0 || t.top == nil || n.owner != t.owner || s.header().owner != t.owner {
		return false, ErrPortIncomplete
	}
	old, err := t.resolve(n, false)
	if err != nil {
		return false, err
	}
	if old == s {
		return true, nil
	}
	if old == nil {
		t.top.add(contextKey{name: n}, s)
		return true, nil
	}
	if !n.displayExact {
		return false, ErrPortIncomplete
	}
	sl, err := nodeLocation(s)
	if err != nil {
		return false, err
	}
	ol, err := nodeLocation(old)
	if err != nil {
		return false, err
	}
	if ol.NamePresent && ol.Name == "--TLA+ BUILTINS--" {
		return false, addError(t.errors, 4202, sl, "Symbol %s is a built-in operator, and cannot be redefined.", nameParam(n))
	}
	loc, err := renderLocation(ol)
	if err != nil {
		return false, err
	}
	if s.header().kind == kindFormal || s.header().kind == kindBound || old.header().kind != s.header().kind || nodeArity(old) != nodeArity(s) {
		return false, addError(t.errors, 4201, sl, "Multiply-defined symbol '%s': this definition or declaration conflicts \nwith the one at %s.", nameParam(n), exactStringParam(loc))
	}
	if sameOriginallyDefined(s, old) {
		return true, nil
	}
	return true, addError(t.errors, 4801, sl, "Multiple declarations or definitions for symbol %s.  \nThis duplicates the one at %s.", nameParam(n), exactStringParam(loc))
}
func (t *symbolTable) addModule(n *name, s leafNode) (bool, error) {
	if n == nil || s == nil || nodeClassOf(s) != classModule || t.top == nil || n.owner != t.owner || s.header().owner != t.owner {
		return false, ErrPortIncomplete
	}
	old, err := t.resolve(n, true)
	if err != nil {
		return false, err
	}
	if old == s {
		return true, nil
	}
	if old == nil {
		t.top.add(contextKey{name: n, module: true}, s)
		return true, nil
	}
	if !n.displayExact {
		return false, ErrPortIncomplete
	}
	sl, err := nodeLocation(s)
	if err != nil {
		return false, err
	}
	ol, err := nodeLocation(old)
	if err != nil {
		return false, err
	}
	loc, err := renderLocation(ol)
	if err != nil {
		return false, err
	}
	return false, addError(t.errors, 4223, sl, "Multiply-defined module '%s': this definition or declaration conflicts \nwith the one at %s.", nameParam(n), exactStringParam(loc))
}
