// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ SymbolTable.java; see LICENSE.
package semantic

import "testing"

func TestInternalModuleCopiesStackButSharesContexts(t *testing.T) {
	p := newNamePool()
	parentErr := new(Errors)
	parent := table(t, p, parentErr)
	outer := newContext(p)
	if err := parent.push(outer); err != nil {
		t.Fatal(err)
	}
	module, err := newModule(p, p.intern("M"), nil, outer)
	if err != nil {
		t.Fatal(err)
	}
	if err = parent.setModuleNode(module); err != nil {
		t.Fatal(err)
	}
	childErr := new(Errors)
	external := map[*name]leafNode{}
	child, err := newInternalTable(external, childErr, parent)
	if err != nil {
		t.Fatal(err)
	}
	if child.getModuleNode() != module || child.base != parent.stack[0] || child.top != nil || len(child.stack) != 2 || child.stack[1] != outer || child.errors != childErr {
		t.Fatal("constructor state")
	}
	x := decl(t, p, "shared", 2)
	outer.add(contextKey{name: x.name}, x)
	if got, err := child.resolve(x.name, false); err != nil || got != x {
		t.Fatal("shared context lookup")
	}
	// Replace an already occupied parent index, before any append could
	// allocate a different backing array. The child's slot must stay shared
	// with the original Context object, not the parent's replacement.
	if err = parent.pop(); err != nil {
		t.Fatal(err)
	}
	replacement := newContext(p)
	if err = parent.push(replacement); err != nil {
		t.Fatal(err)
	}
	if parent.stack[1] != replacement || child.stack[1] != outer {
		t.Fatal("stack backing array shared")
	}
	if err = parent.pop(); err != nil {
		t.Fatal(err)
	}
	if err = parent.push(outer); err != nil {
		t.Fatal(err)
	}
	fresh := newContext(p)
	if err = parent.push(fresh); err != nil {
		t.Fatal(err)
	}
	if len(child.stack) != 2 || child.top != nil {
		t.Fatal("shared stack container")
	}
	inner := newContext(p)
	if err = child.push(inner); err != nil {
		t.Fatal(err)
	}
	if child.top != inner || len(parent.stack) != 3 || parent.top != fresh {
		t.Fatal("child push affected parent")
	}
	if err = child.pop(); err != nil || child.top != outer {
		t.Fatal("pop top")
	}
}
func TestInternalModuleBoundaryAndExternalMap(t *testing.T) {
	p := newNamePool()
	parent := table(t, p, new(Errors))
	module, err := newModule(p, p.intern("X"), nil, newContext(p))
	if err != nil {
		t.Fatal(err)
	}
	child, err := newInternalTable(map[*name]leafNode{module.name: module}, new(Errors), parent)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := child.resolve(module.name, true); err != nil || got != module {
		t.Fatal("new external table")
	}
	if _, err := newInternalTable(nil, new(Errors), parent); err != ErrPortIncomplete {
		t.Fatal("nil external")
	}
	if _, err := newInternalTable(map[*name]leafNode{}, nil, parent); err != ErrPortIncomplete {
		t.Fatal("nil errors")
	}
	if _, err := newInternalTable(map[*name]leafNode{}, new(Errors), nil); err != ErrPortIncomplete {
		t.Fatal("nil parent")
	}
	if _, err := child.addSymbol(module.name, module); err != ErrPortIncomplete {
		t.Fatal("nil top boundary")
	}
	foreign := newNamePool()
	fm, err := newModule(foreign, foreign.intern("Y"), nil, newContext(foreign))
	if err != nil {
		t.Fatal(err)
	}
	if err = child.setModuleNode(fm); err != ErrPortIncomplete {
		t.Fatal("foreign module")
	}
	if err = child.setModuleNode(module); err != nil || child.getModuleNode() != module {
		t.Fatal("set module")
	}
	if err = child.setModuleNode(nil); err != nil || child.getModuleNode() != nil {
		t.Fatal("clear module")
	}
	if _, err := newInternalTable(map[*name]leafNode{fm.name: fm}, new(Errors), parent); err != ErrPortIncomplete {
		t.Fatal("foreign external")
	}
}

func TestInternalModuleEmptyParentStack(t *testing.T) {
	p := newNamePool()
	parent := table(t, p, new(Errors))
	if err := parent.pop(); err != nil {
		t.Fatal(err)
	}
	child, err := newInternalTable(map[*name]leafNode{}, new(Errors), parent)
	if err != nil {
		t.Fatal(err)
	}
	if child.base != nil || child.top != nil || len(child.stack) != 0 {
		t.Fatal("empty copied stack")
	}
	c := newContext(p)
	if err := child.push(c); err != nil || child.top != c {
		t.Fatal("first push")
	}
}
