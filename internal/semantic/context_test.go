// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from source-bound TLA+ semantic control cases; see LICENSE.
package semantic

import (
	"testing"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/lexer"
)

func testTree(t *testing.T, p *namePool) *generated.SyntaxTreeNode {
	t.Helper()
	n, err := p.syntax.NewTokenNode(p.syntax.UniqueStringOf("M"), &lexer.Token{Image: "x", Begin: lexer.Position{Line: 1, Column: 2}, End: lexer.Position{Line: 1, Column: 3}})
	if err != nil {
		t.Fatal(err)
	}
	return n
}
func decl(t *testing.T, p *namePool, name string, kind int) *opDeclNode {
	t.Helper()
	v, e := newOpDecl(p, p.intern(name), kind, 0, 0, testTree(t, p), nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func table(t *testing.T, p *namePool, e *Errors) *symbolTable {
	t.Helper()
	v, err := newTable(p, map[*name]leafNode{}, e)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func TestBuiltinsRequestIdentityAndInitialization(t *testing.T) {
	p := newNamePool()
	e := new(Errors)
	a := table(t, p, e)
	b := table(t, p, e)
	x, _ := a.resolve(p.intern("="), false)
	y, _ := b.resolve(p.intern("="), false)
	if x != y {
		t.Fatal("request builtin identity")
	}
	eq := x.(*builtinOpDefNode)
	if len(eq.params) != 2 || eq.params[0].tree != nil || eq.params[0].name != p.intern("Formal_0") {
		t.Fatal("formal identity")
	}
	if ok, err := a.base.mergeExtend(b.base, e); !ok || err != nil || len(e.Messages()) != 0 {
		t.Fatal("builtin merge")
	}
	dot, _ := a.resolve(p.intern("."), false)
	truth, _ := a.resolve(p.intern("TRUE"), false)
	if dot.(*builtinOpDefNode).maxLevels != nil || truth.(*builtinOpDefNode).maxLevels == nil || truth.(*builtinOpDefNode).params == nil {
		t.Fatal("null/empty")
	}
	q := newNamePool()
	z, _ := table(t, q, new(Errors)).resolve(q.intern("="), false)
	if z == x {
		t.Fatal("cross-request identity")
	}
}
func TestContextOverwriteAndScopes(t *testing.T) {
	p := newNamePool()
	a := decl(t, p, "x", 2)
	b := decl(t, p, "x", 2)
	c := newContext(p)
	k := contextKey{name: a.name}
	c.add(k, a)
	c.add(k, b)
	if c.lookup(k) != b || c.last.prior.value != a || c.duplicate().lookup(k) != a {
		t.Fatal("pair order")
	}
	tab := table(t, p, new(Errors))
	if ok, err := tab.addSymbol(a.name, a); !ok || err != nil {
		t.Fatal(ok, err)
	}
	tab.push(newContext(p))
	if got, _ := tab.resolve(a.name, false); got != a {
		t.Fatal("scope")
	}
	tab.pop()
}
func TestConstructorIdentityAndSideEffects(t *testing.T) {
	p := newNamePool()
	e := new(Errors)
	tab := table(t, p, e)
	tree := testTree(t, p)
	f, err := newFormalParam(p, p.intern("fp"), 0, tree, tab, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := tab.resolve(f.name, false)
	if got != f || f.tree != tree || f.levels.checked != 0 || len(f.levels.levelParams) != 0 {
		t.Fatal("formal identity")
	}
	f.levelCheck(0)
	if f.levels.checked != 0 || len(f.levels.levelParams) != 1 || len(f.levels.allParams) != 1 {
		t.Fatal("level zero")
	}
	f.levelCheck(2)
	f.levelCheck(3)
	if _, ok := f.levels.levelParams[f]; !ok || f.levels.checked != 2 || len(f.levels.levelParams) != 1 || len(f.levels.allParams) != 1 {
		t.Fatal("formal level")
	}
	c, err := newOpDecl(p, p.intern("c"), 2, 0, 0, tree, tab, nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err := newOpDecl(p, p.intern("v"), 3, 1, 0, tree, tab, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.levels.levelParams[c]; !ok || c.levels.checked != 1 || len(v.levels.levelParams) != 0 || v.levels.checked != 1 {
		t.Fatal("declaration levels")
	}
	duplicate, err := newFormalParam(p, p.intern("fp"), 0, tree, tab, nil)
	if err != nil || duplicate == nil || len(e.OfLevel(Error)) != 1 {
		t.Fatal("false add must preserve constructed object and diagnostic")
	}
	got, _ = tab.resolve(f.name, false)
	if got != f {
		t.Fatal("duplicate replaced original")
	}
}
func TestModuleCacheAndNamespace(t *testing.T) {
	p := newNamePool()
	c := newContext(p)
	m, err := newModule(p, p.intern("m"), testTree(t, p), c)
	if err != nil {
		t.Fatal(err)
	}
	if !m.parameterFree() || m.constantCache == nil || m.variableCache == nil {
		t.Fatal("empty initialized declaration arrays")
	}
	x := decl(t, p, "x", 2)
	c.add(contextKey{name: x.name}, x)
	if !m.parameterFree() {
		t.Fatal("constant snapshot")
	}
	c2 := newContext(p)
	c2.add(contextKey{name: x.name}, x)
	m2, _ := newModule(p, p.intern("m2"), nil, c2)
	if m2.parameterFree() || m2.variablesCached {
		t.Fatal("short circuit")
	}
	v := decl(t, p, "v", 3)
	c2.add(contextKey{name: v.name}, v)
	if !m2.hasDeclarations(3) {
		t.Fatal("lazy variable snapshot")
	}
	member, err := newOpDecl(p, p.intern("member"), 2, 0, 0, testTree(t, p), nil, m2)
	if err != nil || member.module != m2 {
		t.Fatal("actual originating module identity")
	}
	tab := table(t, p, new(Errors))
	if ok, err := tab.addModule(m.name, m); !ok || err != nil {
		t.Fatal(ok, err)
	}
	same := decl(t, p, "m", 2)
	if ok, err := tab.addSymbol(same.name, same); !ok || err != nil {
		t.Fatal(ok, err)
	}
	a, _ := tab.resolve(m.name, true)
	b, _ := tab.resolve(m.name, false)
	if a != m || b != same {
		t.Fatal("separate namespace")
	}
}
func TestNamePoolPreservesUnpairedUTF16(t *testing.T) {
	p := newNamePool()
	a := p.internUnits([]uint16{0xd800})
	b := p.internUnits([]uint16{0xd801})
	if a == b || a.displayExact || b.displayExact || p.internUnits([]uint16{0xd800}) != a {
		t.Fatal("UTF-16 identity")
	}
	if p.intern("a") != p.internUnits([]uint16{'a'}) {
		t.Fatal("interner identity")
	}
}

func TestBuiltinVectorsAndAlias(t *testing.T) {
	p := newNamePool()
	null, err := newBuiltin(p, &builtinDescriptor{text: "probe-null", arity: 0, level: 2})
	if err != nil {
		t.Fatal(err)
	}
	if null.levels.checked != 0 || null.isLeibniz || null.maxLevels != nil || null.isLeibnizArg != nil {
		t.Fatal("null initialization")
	}
	max := []int{1, 2}
	weights := []int{1, 0}
	fixed, err := newBuiltin(p, &builtinDescriptor{text: "probe-fixed", arity: 2, level: 2, max: max, weights: weights})
	if err != nil {
		t.Fatal(err)
	}
	if fixed.levels.checked != 99 || fixed.isLeibniz || len(fixed.isLeibnizArg) != 2 || !fixed.isLeibnizArg[0] || fixed.isLeibnizArg[1] {
		t.Fatal("fixed initialization")
	}
	max[0] = 3
	weights[0] = 2
	if fixed.maxLevels[0] != 3 || fixed.weights[0] != 2 {
		t.Fatal("source array alias")
	}
	variadic, err := newBuiltin(p, &builtinDescriptor{text: "probe-var", arity: -1, level: 2, max: max, weights: weights})
	if err != nil {
		t.Fatal(err)
	}
	if variadic.params != nil || len(variadic.maxLevels) != 1 || variadic.maxLevels[0] != 3 || len(variadic.isLeibnizArg) != 2 {
		t.Fatal("variadic initialization")
	}
	max[0] = 4
	if variadic.maxLevels[0] != 3 {
		t.Fatal("variadic vectors copied")
	}
}
func TestUnpairedNameDiagnosticIncomplete(t *testing.T) {
	p := newNamePool()
	e := new(Errors)
	tab := table(t, p, e)
	name := p.internUnits([]uint16{0xd800})
	a, err := newOpDecl(p, name, 2, 0, 0, testTree(t, p), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := newOpDecl(p, name, 3, 0, 0, testTree(t, p), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := tab.addSymbol(name, a); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if _, err = tab.addSymbol(name, b); err != ErrPortIncomplete || len(e.Messages()) != 0 {
		t.Fatal("lossy diagnostic must remain incomplete")
	}
}

func TestMergeDiagnosticsAndLocalSkip(t *testing.T) {
	p := newNamePool()
	e := new(Errors)
	dst := newContext(p)
	src := newContext(p)
	a := decl(t, p, "x", 2)
	b := decl(t, p, "x", 2)
	f, err := newFormalParam(p, p.intern("local"), 0, testTree(t, p), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dst.add(contextKey{name: a.name}, a)
	src.add(contextKey{name: b.name}, b)
	src.add(contextKey{name: f.name}, f)
	ok, err := dst.mergeExtend(src, e)
	if !ok || err != nil || len(e.OfLevel(Warning)) != 1 || dst.lookup(contextKey{name: f.name}) != nil || dst.lookup(contextKey{name: a.name}) != a {
		t.Fatal("merge warning/local")
	}
}
func TestBuiltinRedefinitionIsDefinitiveDiagnostic(t *testing.T) {
	p := newNamePool()
	e := new(Errors)
	tab := table(t, p, e)
	v := decl(t, p, "TRUE", 2)
	ok, err := tab.addSymbol(v.name, v)
	if ok || err != nil || len(e.OfLevel(Error)) != 1 || e.OfLevel(Error)[0].Code.Value != 4202 {
		t.Fatal("builtin diagnostic")
	}
}

// The expected diagnostic text and order come from the retained Context probe.
func TestProbeDiagnostics(t *testing.T) {
	p := newNamePool()
	e := new(Errors)
	tab := table(t, p, e)
	a := decl(t, p, "probe_x", 2)
	b := decl(t, p, "probe_x", 2)
	wrong := decl(t, p, "probe_x", 3)
	if ok, err := tab.addSymbol(a.name, a); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if ok, err := tab.addSymbol(b.name, b); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if ok, err := tab.addSymbol(wrong.name, wrong); ok || err != nil {
		t.Fatal(ok, err)
	}
	got := e.Messages()
	if len(got) != 2 || got[0].Code.Value != 4801 || got[1].Code.Value != 4201 {
		t.Fatal("diagnostic order")
	}
	expected := []string{"Multiple declarations or definitions for symbol probe_x.  \nThis duplicates the one at line 1, col 2 to line 1, col 3 of module M.", "Multiply-defined symbol 'probe_x': this definition or declaration conflicts \nwith the one at line 1, col 2 to line 1, col 3 of module M."}
	for i, d := range got {
		m, err := d.Render()
		if err != nil || m != expected[i] {
			t.Fatalf("message %d: %q %v", i, m, err)
		}
	}
}

func TestMergeDifferentConcreteClasses(t *testing.T) {
	p := newNamePool()
	e := new(Errors)
	tab := table(t, p, e)
	builtin, _ := tab.resolve(p.intern("TRUE"), false)
	dst, src := newContext(p), newContext(p)
	d := decl(t, p, "TRUE", 2)
	dst.add(contextKey{name: d.name}, d)
	src.add(contextKey{name: d.name}, builtin)
	ok, err := dst.mergeExtend(src, e)
	if ok || err != nil || len(e.OfLevel(Error)) != 1 || e.OfLevel(Error)[0].Code.Value != 4224 {
		t.Fatal("class conflict")
	}
}

type unknownLeaf struct{ nodeHeader }

func (n *unknownLeaf) header() *nodeHeader { return &n.nodeHeader }

func TestBoundaryRefusalsAndExternalModule(t *testing.T) {
	p := newNamePool()
	externalName := p.intern("external")
	m, err := newModule(p, externalName, nil, newContext(p))
	if err != nil {
		t.Fatal(err)
	}
	tab, err := newTable(p, map[*name]leafNode{externalName: m}, new(Errors))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := tab.resolve(externalName, true); err != nil || got != m {
		t.Fatal("external module map")
	}
	if ok, err := tab.addModule(externalName, m); !ok || err != nil {
		t.Fatal("same external module reinsertion")
	}
	if _, err := newTable(p, nil, new(Errors)); err != ErrPortIncomplete {
		t.Fatal("nil map")
	}
	prior := len(tab.errors.Messages())
	if _, err := newOpDecl(p, externalName, 99, 0, 0, nil, tab, nil); err != ErrPortIncomplete {
		t.Fatal("unknown declaration kind")
	}
	if len(tab.errors.Messages()) != prior || tab.top.lookup(contextKey{name: externalName}) != nil {
		t.Fatal("invalid kind side effect")
	}
	fake := &unknownLeaf{nodeHeader: nodeHeader{semanticHeader: semanticHeader{owner: p}, name: externalName}}
	if _, err := tab.addSymbol(externalName, fake); err != ErrPortIncomplete {
		t.Fatal("unknown node class")
	}
	foreign := newNamePool().intern("foreign")
	if _, err := newFormalParam(p, foreign, 0, nil, nil, nil); err != ErrPortIncomplete {
		t.Fatal("foreign name")
	}
	other := newNamePool()
	foreignNode, err := newOpDecl(other, other.intern("external"), 2, 0, 0, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tab.addSymbol(externalName, foreignNode); err != ErrPortIncomplete {
		t.Fatal("foreign node")
	}
}
func TestInternUnitsCopiesInput(t *testing.T) {
	p := newNamePool()
	units := []uint16{'a', 0xd800}
	n := p.internUnits(units)
	units[0] = 'b'
	if n.units[0] != 'a' || p.internUnits([]uint16{'a', 0xd800}) != n {
		t.Fatal("caller changed interned units")
	}
}

func TestContextEnumerationAndDeclarationSnapshots(t *testing.T) {
	p := newNamePool()
	c := newContext(p)
	for _, text := range []string{"Aa", "BB", "C"} {
		d := decl(t, p, text, 2)
		c.add(contextKey{name: d.name}, d)
	}
	replacement := decl(t, p, "Aa", 2)
	c.add(contextKey{name: replacement.name}, replacement)
	values := c.orderedValues()
	if len(values) != 3 || values[0].header().name.text != "C" || values[1].header().name.text != "BB" || values[2] != replacement {
		t.Fatal("collision and replacement order", values)
	}
	m, _ := newModule(p, p.intern("M"), nil, c)
	if !m.hasDeclarations(2) || m.constantCache[0] != replacement || m.constantCache[1].header().name.text != "BB" || m.constantCache[2].header().name.text != "C" {
		t.Fatal("reverse declaration cache", m.constantCache)
	}
	newDecl := decl(t, p, "D", 2)
	c.add(contextKey{name: newDecl.name}, newDecl)
	if len(m.constantCache) != 3 {
		t.Fatal("cache changed")
	}
	duplicate := c.duplicate()
	if duplicate.lookup(contextKey{name: replacement.name}) == replacement {
		t.Fatal("duplicate must replay pair history")
	}
}

func TestContextEnumerationGrowth(t *testing.T) {
	p := newNamePool()
	c := newContext(p)
	for _, r := range "ABCDEFGHI" {
		d := decl(t, p, string(r), 2)
		c.add(contextKey{name: d.name}, d)
	}
	got := ""
	for _, value := range c.orderedValues() {
		got += value.header().name.text
	}
	if got != "DCBAIHGFE" || len(c.order) != 23 {
		t.Fatal("growth enumeration", got)
	}
}

func TestContextOrderNamespacesAndUTF16(t *testing.T) {
	p := newNamePool()
	c := newContext(p)
	name := p.internUnits([]uint16{0xd800, 0x61})
	a, _ := newOpDecl(p, name, 2, 0, 0, testTree(t, p), nil, nil)
	b, _ := newModule(p, name, nil, newContext(p))
	c.add(contextKey{name: name}, a)
	c.add(contextKey{name: name, module: true}, b)
	var h uint32
	for _, u := range name.units {
		h = 31*h + uint32(u)
	}
	index := int(h&0x7fffffff) % 11
	if len(c.order[index]) != 2 || c.order[index][0].module || !c.order[index][1].module || c.orderedValues()[0] != b {
		t.Fatal("UTF-16 hash or shared namespace collision")
	}
}

func TestLabelTableEnumeration(t *testing.T) {
	p := newNamePool()
	labels := newLabelSet(p)
	for _, text := range []string{"Aa", "BB", "C"} {
		name := p.intern(text)
		labels.put(name, &labelNode{name: name})
	}
	ordered := labels.ordered()
	if len(ordered) != 3 || ordered[0].name.text != "C" || ordered[1].name.text != "BB" || ordered[2].name.text != "Aa" {
		t.Fatal("label table enumeration", ordered)
	}
}

func TestModuleOpDefPairOrderAndSnapshot(t *testing.T) {
	p := newNamePool()
	c := newContext(p)
	m, _ := newModule(p, p.intern("M"), nil, c)
	a, _ := newUserOpDef(p, p.intern("F"), nil, false, nil, m, nil, nil)
	b, _ := newUserOpDef(p, p.intern("F"), nil, false, nil, m, nil, nil)
	c.add(contextKey{name: a.name}, a)
	c.add(contextKey{name: b.name}, b)
	defs := m.opDefs()
	if len(defs) != 2 || defs[0] != a || defs[1] != b {
		t.Fatal("pair order", defs)
	}
	z, _ := newUserOpDef(p, p.intern("Z"), nil, false, nil, m, nil, nil)
	c.add(contextKey{name: z.name}, z)
	if len(m.opDefs()) != 2 {
		t.Fatal("operator cache changed")
	}
}
