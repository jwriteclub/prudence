// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"errors"
	"testing"

	"github.com/jwriteclub/prudence/internal/modulegraph"
)

// These identities model known Java tickets for unit controls only. They do
// not certify the level-enabled driver or the semantic-only history profile.
func testLevelChecker(t *testing.T, nodes ...leafNode) *levelChecker {
	t.Helper()
	ids := make(map[leafNode]uint64)
	pool := newNamePool()
	for i, n := range nodes {
		if n.header().name == nil {
			n.header().name = pool.intern("test")
		}
		ids[n] = uint64(i + 1)
	}
	c, e := newLevelChecker(stdcontext.Background(), func(n leafNode) (uint64, uint32, error) {
		id := ids[n]
		if id == 0 {
			return 0, 0, ErrPortIncomplete
		}
		return id, uint32(961+31*n.header().kind) + uint32(id), nil
	}, 100, 100000)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestLevelDeclaredApplicationAndDefinition(t *testing.T) {
	p := &formalParamNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 11}}}
	f := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 2}}, arity: 1}
	literal := &numeralNode{}
	pUse := &opApplNode{operator: p, operands: []exprOrOpArg{}}
	fUse := &opApplNode{operator: f, operands: []exprOrOpArg{pUse, literal}}
	d := &userOpDefNode{params: []*formalParamNode{p}, body: fUse, maxLevels: []int{3}, weights: []int{0}, isLeibniz: true, isLeibnizArg: []bool{true}}
	c := testLevelChecker(t, p, f)
	ok, e := c.check(d, 1)
	if e != nil || !ok {
		t.Fatalf("check: %v %v", ok, e)
	}
	ds, e := c.data(d)
	if e != nil {
		t.Fatal(e)
	}
	if ds.weights[0] != 1 || ds.level != 0 {
		t.Fatalf("definition: %+v", ds)
	}
	ps, _ := c.sym(p)
	fs, _ := c.sym(f)
	if _, found, e := ds.params.Get(ps, ps.hash); e != nil || found {
		t.Fatalf("formal leaked %v %v", found, e)
	}
	if _, found, e := ds.params.Get(fs, fs.hash); e != nil || !found {
		t.Fatalf("external lost %v %v", found, e)
	}
	key := levelPair{fs, 0}
	if v, found, e := ds.lower.Get(key, fs.hash); e != nil || !found || v != 0 {
		t.Fatalf("explicit zero lower: %d %v %v", v, found, e)
	}
	if _, found, e := ds.triples.Get(levelTriple{fs, 0, ps}, fs.hash+ps.hash); e != nil || !found {
		t.Fatalf("external endpoint triple lost: %v %v", found, e)
	}
	// A normal definition is checked only once, even at a later iteration.
	if ok, e = c.check(d, 2); e != nil || !ok || ds.checked != 1 {
		t.Fatalf("cache %v %v %d", ok, e, ds.checked)
	}
}
func TestLevelConstraintUpdatesAndAlias(t *testing.T) {
	p := &formalParamNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 11}}}
	c := testLevelChecker(t, p)
	ps, e := c.sym(p)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.check(p, 1); e != nil {
		t.Fatal(e)
	}
	a := &opArgNode{operator: p}
	if _, e = c.check(a, 1); e != nil {
		t.Fatal(e)
	}
	source, _ := c.data(p)
	arg, _ := c.data(a)
	if e = c.putUpper(source, ps, 3); e != nil {
		t.Fatal(e)
	}
	if e = c.putUpper(source, ps, 1); e != nil {
		t.Fatal(e)
	}
	if e = c.putUpper(source, ps, 2); e != nil {
		t.Fatal(e)
	}
	if x, ok, e := arg.upper.Get(ps, ps.hash); e != nil || !ok || x != 1 {
		t.Fatalf("aliased upper %d %v %v", x, ok, e)
	}
	key := levelPair{ps, 0}
	for _, n := range []int{0, 2, 1} {
		if e = c.putLower(source, key, n); e != nil {
			t.Fatal(e)
		}
	}
	if x, ok, e := arg.lower.Get(key, ps.hash); e != nil || !ok || x != 2 {
		t.Fatalf("aliased lower %d %v %v", x, ok, e)
	}
	if arg.nonLeibniz == source.nonLeibniz {
		t.Fatal("OpArg incorrectly aliases non-Leibniz parameters")
	}
}
func TestLevelUnknownIdentityAndCancellation(t *testing.T) {
	p := &formalParamNode{}
	c := testLevelChecker(t)
	if _, e := c.check(p, 1); !errors.Is(e, ErrPortIncomplete) {
		t.Fatalf("unknown identity: %v", e)
	}
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	c, e := newLevelChecker(ctx, func(leafNode) (uint64, uint32, error) { return 1, 1, nil }, 10, 100)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.check(&numeralNode{}, 1); !errors.Is(e, stdcontext.Canceled) {
		t.Fatalf("cancellation: %v", e)
	}
}

func TestLevelRecursiveSectionRunsTwoPasses(t *testing.T) {
	p := &formalParamNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 11}}}
	body := &opApplNode{operator: p, operands: []exprOrOpArg{}}
	d := &userOpDefNode{params: []*formalParamNode{p}, body: body, inRecursive: true, inRecursiveSection: true, recursiveSection: 1, maxLevels: []int{3}, weights: []int{0}, isLeibniz: true, isLeibnizArg: []bool{true}}
	c := testLevelChecker(t, p)
	if e := c.checkRecursiveSections([]*userOpDefNode{d}, &Errors{}); e != nil {
		t.Fatal(e)
	}
	s, e := c.data(d)
	if e != nil {
		t.Fatal(e)
	}
	if s.checked != 2 || s.maxLevels[0] != 2 || s.weights[0] != 1 {
		t.Fatalf("recursive state: %d %v %v", s.checked, s.maxLevels, s.weights)
	}
}

func TestLevelFirstOrderDefinitionApplication(t *testing.T) {
	pool := newNamePool()
	p := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 3}}, levels: leafLevel{level: 1}}
	use := &opApplNode{operator: p, operands: []exprOrOpArg{}}
	def := &builtinOpDefNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 7}, name: pool.intern("F")}, arity: 1, levels: leafLevel{checked: 99}, maxLevels: []int{0}, weights: []int{1}, isLeibniz: true, isLeibnizArg: []bool{true}}
	app := &opApplNode{operator: def, operands: []exprOrOpArg{use}}
	c := testLevelChecker(t, p)
	ok, e := c.check(app, 1)
	if e != nil || ok {
		t.Fatalf("expected false level result: %v %v", ok, e)
	}
	records := c.errors.Messages()
	if len(records) != 1 || records[0].Code.Value != 4205 || len(records[0].Parameters) != 2 || records[0].Parameters[1].Integer != 1 {
		t.Fatalf("diagnostic: %+v", records)
	}
	s, e := c.data(app)
	if e != nil || s.level != 1 {
		t.Fatalf("result level %v %v", s, e)
	}
}
func TestLevelUnsupportedStopsCachedReuse(t *testing.T) {
	c := testLevelChecker(t)
	n := &substInNode{}
	if _, e := c.check(n, 1); !errors.Is(e, ErrPortIncomplete) {
		t.Fatalf("unsupported: %v", e)
	}
	if _, e := c.check(n, 1); !errors.Is(e, ErrPortIncomplete) {
		t.Fatalf("cached partial state returned success: %v", e)
	}
}

func TestLevelAssumeProveIgnoresProveBooleanAndConstrainsConstants(t *testing.T) {
	pool := newNamePool()
	variable := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 3}}, levels: leafLevel{level: 1}}
	constant := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 2}}}
	varUse := &opApplNode{operator: variable, operands: []exprOrOpArg{}}
	constUse := &opApplNode{operator: constant, operands: []exprOrOpArg{}}
	def := &builtinOpDefNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 7}, name: pool.intern("F")}, arity: 1, levels: leafLevel{checked: 99}, maxLevels: []int{0}, weights: []int{1}, isLeibniz: true, isLeibnizArg: []bool{true}}
	prove := &opApplNode{operator: def, operands: []exprOrOpArg{varUse}}
	ap := &assumeProveNode{assumptions: []levelNode{constUse}, prove: prove}
	c := testLevelChecker(t, variable, constant)
	ok, e := c.check(ap, 1)
	if e != nil || !ok {
		t.Fatalf("AP ignores prove boolean: %v %v", ok, e)
	}
	if len(c.errors.Messages()) != 1 {
		t.Fatalf("prove diagnostic lost: %+v", c.errors.Messages())
	}
	s, e := c.data(ap)
	if e != nil {
		t.Fatal(e)
	}
	sym, e := c.sym(constant)
	if e != nil {
		t.Fatal(e)
	}
	if v, found, e := s.upper.Get(sym, sym.hash); e != nil || !found || v != 2 {
		t.Fatalf("constant ceiling: %d %v %v", v, found, e)
	}
}

func TestLevelNewBoundAliasAndTemporalDiagnostic(t *testing.T) {
	p := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 2}}}
	temporal := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 3}}, levels: leafLevel{level: 3}}
	bound := &opApplNode{operator: temporal, operands: []exprOrOpArg{}}
	n := &newSymbNode{declaration: p, set: bound}
	c := testLevelChecker(t, p, temporal)
	ok, e := c.check(n, 1)
	if e != nil || ok {
		t.Fatalf("temporal NEW: %v %v", ok, e)
	}
	s, _ := c.data(n)
	b, _ := c.data(bound)
	if s.params != b.params || s.upper != b.upper || s.nonLeibniz == b.nonLeibniz {
		t.Fatal("NEW field aliases differ from source")
	}
	records := c.errors.Messages()
	if len(records) != 1 || records[0].Code.Value != 4356 {
		t.Fatalf("NEW diagnostic: %+v", records)
	}
}

func TestLevelTheoremDefinitionWeightsAndLeibnizInitialization(t *testing.T) {
	p := &formalParamNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 11}}}
	q := &formalParamNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 11}}}
	body := &opApplNode{operator: p, operands: []exprOrOpArg{}}
	d := &thmOrAssumpDefNode{params: []*formalParamNode{p, q}, body: body}
	c := testLevelChecker(t, p, q)
	ok, e := c.check(d, 1)
	if e != nil || !ok {
		t.Fatalf("theorem definition: %v %v", ok, e)
	}
	s, e := c.data(d)
	if e != nil {
		t.Fatal(e)
	}
	if s.weights[0] != 0 || s.weights[1] != 0 || len(s.leibniz) != 2 || s.leibniz[0] || !s.leibniz[1] {
		t.Fatalf("source quirks: weights=%v leibniz=%v", s.weights, s.leibniz)
	}
	empty := &thmOrAssumpDefNode{body: &numeralNode{}}
	ok, e = c.check(empty, 1)
	if e != nil || !ok {
		t.Fatalf("zero-param theorem: %v %v", ok, e)
	}
	es, e := c.data(empty)
	if e != nil || es.leibniz != nil || es.isLeibniz {
		t.Fatalf("zero-param fields: %+v %v", es, e)
	}
}

func TestLevelTheoremUsesSeparateShadowCache(t *testing.T) {
	body := &numeralNode{}
	thm := &theoremNode{body: body, proof: &leafProofNode{}}
	c := testLevelChecker(t)
	ok, e := c.check(thm, 1)
	if e != nil || !ok {
		t.Fatalf("theorem: %v %v", ok, e)
	}
	s, e := c.data(thm)
	if e != nil || s.checked != 1 || s.shadow != 1 {
		t.Fatalf("theorem caches: %+v %v", s, e)
	}
	s.correct = false
	ok, e = c.check(thm, 1)
	if e != nil || !ok {
		t.Fatalf("shadow cache must return true: %v %v", ok, e)
	}
}

func TestLevelSubstitutionFirstMatchAndUnchangedTriple(t *testing.T) {
	pool := newNamePool()
	f := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 2}}}
	p := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 2}}}
	q := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 2}}}
	r := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 2}}}
	variable := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 3}, name: pool.intern("v")}, levels: leafLevel{level: 1}}
	targetContext := newContext(pool)
	targetContext.add(contextKey{name: variable.name}, variable)
	target := &moduleNode{context: targetContext}
	pUse := &opApplNode{operator: p, operands: []exprOrOpArg{}}
	qUse := &opApplNode{operator: q, operands: []exprOrOpArg{}}
	rUse := &opApplNode{operator: r, operands: []exprOrOpArg{}}
	c := testLevelChecker(t, f, p, q, r, variable)
	if _, e := c.check(pUse, 1); e != nil {
		t.Fatal(e)
	}
	body, e := c.data(pUse)
	if e != nil {
		t.Fatal(e)
	}
	fs, _ := c.sym(f)
	ps, _ := c.sym(p)
	qs, _ := c.sym(q)
	rs, _ := c.sym(r)
	triple := levelTriple{fs, 0, ps}
	if e = body.triples.Put(triple, fs.hash+ps.hash, struct{}{}); e != nil {
		t.Fatal(e)
	}
	n := &substInNode{body: pUse, substs: []*subst{{op: p, expr: qUse}, {op: p, expr: rUse}}, instantiatedModule: target}
	ok, e := c.check(n, 1)
	if e != nil || !ok {
		t.Fatalf("substitution: %v %v", ok, e)
	}
	result, e := c.data(n)
	if e != nil {
		t.Fatal(e)
	}
	if _, found, e := result.params.Get(qs, qs.hash); e != nil || !found {
		t.Fatalf("first match missing: %v %v", found, e)
	}
	if _, found, e := result.params.Get(rs, rs.hash); e != nil || found {
		t.Fatalf("second match incorrectly used: %v %v", found, e)
	}
	if _, found, e := result.triples.Get(triple, fs.hash+ps.hash); e != nil || !found {
		t.Fatalf("unchanged operator triple lost: %v %v", found, e)
	}
	if _, found, e := body.all.Get(ps, ps.hash); e != nil || !found {
		t.Fatalf("body copy mutated: %v %v", found, e)
	}
}

func TestLevelModuleNonconstantConstraintsAndReentry(t *testing.T) {
	pool := newNamePool()
	ctx := newContext(pool)
	k := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 2}, name: pool.intern("K")}}
	v := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 3}, name: pool.intern("v")}, levels: leafLevel{level: 1}}
	ctx.add(contextKey{name: k.name}, k)
	ctx.add(contextKey{name: v.name}, v)
	m := &moduleNode{context: ctx}
	c := testLevelChecker(t, k, v)
	ok, e := c.check(m, 1)
	if e != nil || !ok {
		t.Fatalf("module: %v %v", ok, e)
	}
	s := c.states[m]
	sym, _ := c.sym(k)
	if x, found, e := s.upper.Get(sym, sym.hash); e != nil || !found || x != 0 {
		t.Fatalf("nonconstant ceiling: %d %v %v", x, found, e)
	}
	if _, e = c.data(m); !errors.Is(e, ErrPortIncomplete) {
		t.Fatalf("module getLevel must be unavailable: %v", e)
	}
	constant, e := c.moduleConstant(m)
	if e != nil || constant {
		t.Fatalf("variable module constant: %v %v", constant, e)
	}
}

func TestLevelConstantModuleReentry(t *testing.T) {
	pool := newNamePool()
	m := &moduleNode{context: newContext(pool)}
	c := testLevelChecker(t)
	ok, e := c.check(m, 1)
	if e != nil || !ok {
		t.Fatalf("module check: %v %v", ok, e)
	}
	constant, e := c.moduleConstant(m)
	if e != nil || !constant {
		t.Fatalf("module constant: %v %v", constant, e)
	}
}

func TestLevelInstanceDeduplicatesRestrictions(t *testing.T) {
	pool := newNamePool()
	ctx := newContext(pool)
	k := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 2}, name: pool.intern("K")}}
	v := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 3}, name: pool.intern("v")}, levels: leafLevel{level: 1}}
	ctx.add(contextKey{name: k.name}, k)
	ctx.add(contextKey{name: v.name}, v)
	mod := &moduleNode{nodeHeader: nodeHeader{name: pool.intern("M")}, context: ctx}
	rhs := &opApplNode{operator: v, operands: []exprOrOpArg{}}
	in := &instanceNode{module: mod, substs: []*subst{{op: k, expr: rhs}}}
	c := testLevelChecker(t, k, v)
	ok, e := c.check(in, 1)
	if e != nil || ok {
		t.Fatalf("instance: %v %v", ok, e)
	}
	records := c.errors.Messages()
	if len(records) != 1 || records[0].Code.Value != 4245 || records[0].Parameters[2].Integer != 0 {
		t.Fatalf("deduplicated restriction: %+v", records)
	}
}

func TestLevelTemporalApplicationDiagnostics(t *testing.T) {
	pool := newNamePool()
	action := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 3}, name: pool.intern("A")}, levels: leafLevel{level: 2}}
	temporal := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 3}, name: pool.intern("T")}, levels: leafLevel{level: 3}}
	a := &opApplNode{operator: action, operands: []exprOrOpArg{}}
	b := &opApplNode{operator: temporal, operands: []exprOrOpArg{}}
	always := &builtinOpDefNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 7}, name: pool.intern("[]")}, arity: 1, levels: leafLevel{checked: 99, level: 3}, maxLevels: []int{3}, weights: []int{1}, isLeibnizArg: []bool{true}, isLeibniz: true}
	logic := &builtinOpDefNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 7}, name: pool.intern("\\land")}, arity: 2, levels: leafLevel{checked: 99}, maxLevels: []int{3, 3}, weights: []int{1, 1}, isLeibnizArg: []bool{true, true}, isLeibniz: true}
	c := testLevelChecker(t, action, temporal)
	box := &opApplNode{operator: always, operands: []exprOrOpArg{a}}
	ok, e := c.check(box, 1)
	if e != nil || ok {
		t.Fatalf("box action: %v %v", ok, e)
	}
	mixed := &opApplNode{operator: logic, operands: []exprOrOpArg{a, b}}
	ok, e = c.check(mixed, 1)
	if e != nil || ok {
		t.Fatalf("mixed logical: %v %v", ok, e)
	}
	records := c.errors.Messages()
	if len(records) != 2 || records[0].Code.Value != 4310 || records[1].Code.Value != 4313 {
		t.Fatalf("temporal diagnostics: %+v", records)
	}
}

func TestLevelBoundFilteringAndTemporalRange(t *testing.T) {
	pool := newNamePool()
	p := &formalParamNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 11}, name: pool.intern("p")}}
	action := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 3}, name: pool.intern("A")}, levels: leafLevel{level: 2}}
	operand := &opApplNode{operator: p, operands: []exprOrOpArg{}}
	bound := &opApplNode{operator: action, operands: []exprOrOpArg{}}
	op := &builtinOpDefNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 7}, name: pool.intern("$BoundedForall")}, arity: 1, levels: leafLevel{checked: 99, level: 3}, maxLevels: []int{3}, weights: []int{1}, isLeibnizArg: []bool{true}, isLeibniz: true}
	app := &opApplNode{operator: op, operands: []exprOrOpArg{operand}, ranges: []exprNode{bound}, bounded: [][]*formalParamNode{{p}}}
	c := testLevelChecker(t, p, action)
	ok, e := c.check(app, 1)
	if e != nil || ok {
		t.Fatalf("temporal bound: %v %v", ok, e)
	}
	s, e := c.data(app)
	if e != nil {
		t.Fatal(e)
	}
	ps, _ := c.sym(p)
	if _, found, e := s.params.Get(ps, ps.hash); e != nil || found {
		t.Fatalf("bound parameter retained: %v %v", found, e)
	}
	if v, found, e := s.upper.Get(ps, ps.hash); e != nil || !found || v != 3 {
		t.Fatalf("later constraint insertion: %d %v %v", v, found, e)
	}
	records := c.errors.Messages()
	if len(records) != 1 || records[0].Code.Value != 4314 {
		t.Fatalf("range diagnostic: %+v", records)
	}
}

func TestLevelHigherOrderCoparameterConstraint(t *testing.T) {
	pool := newNamePool()
	f := &formalParamNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 11}, name: pool.intern("F")}, arity: 1}
	p := &formalParamNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 11}, name: pool.intern("p")}}
	varDecl := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 3}, name: pool.intern("v")}, levels: leafLevel{level: 1}}
	pUse := &opApplNode{operator: p, operands: []exprOrOpArg{}}
	body := &opApplNode{operator: f, operands: []exprOrOpArg{pUse}}
	op := &userOpDefNode{nodeHeader: nodeHeader{name: pool.intern("Apply")}, arity: 2, params: []*formalParamNode{f, p}, body: body, maxLevels: []int{3, 3}, weights: []int{0, 0}, isLeibniz: true, isLeibnizArg: []bool{true, true}}
	actual := &builtinOpDefNode{nodeHeader: nodeHeader{name: pool.intern("ConstOnly")}, arity: 1, levels: leafLevel{checked: 99}, maxLevels: []int{0}, weights: []int{1}, isLeibniz: true, isLeibnizArg: []bool{true}}
	arg := &opArgNode{operator: actual}
	v := &opApplNode{operator: varDecl, operands: []exprOrOpArg{}}
	app := &opApplNode{operator: op, operands: []exprOrOpArg{arg, v}}
	c := testLevelChecker(t, f, p, varDecl)
	ok, e := c.check(app, 1)
	if e != nil || ok {
		t.Fatalf("higher-order constraint: %v %v", ok, e)
	}
	records := c.errors.Messages()
	if len(records) != 1 || records[0].Code.Value != 4273 || records[0].Parameters[1].Integer != 2 {
		t.Fatalf("higher-order diagnostic: %+v", records)
	}
}

func TestLevelHigherOrderExternalTripleSubstitution(t *testing.T) {
	pool := newNamePool()
	f := &formalParamNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 11}, name: pool.intern("F")}, arity: 1}
	g := &formalParamNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 11}, name: pool.intern("G")}, arity: 1}
	x := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 2}, name: pool.intern("X")}}
	xUse := &opApplNode{operator: x, operands: []exprOrOpArg{}}
	body := &opApplNode{operator: f, operands: []exprOrOpArg{xUse}}
	d := &userOpDefNode{nodeHeader: nodeHeader{name: pool.intern("D")}, arity: 1, params: []*formalParamNode{f}, body: body, maxLevels: []int{3}, weights: []int{0}, isLeibniz: true, isLeibnizArg: []bool{true}}
	app := &opApplNode{operator: d, operands: []exprOrOpArg{&opArgNode{operator: g}}}
	c := testLevelChecker(t, f, g, x)
	ok, e := c.check(app, 1)
	if e != nil || !ok {
		t.Fatalf("triple application: %v %v", ok, e)
	}
	s, e := c.data(app)
	if e != nil {
		t.Fatal(e)
	}
	gs, _ := c.sym(g)
	xs, _ := c.sym(x)
	fs, _ := c.sym(f)
	tkey := levelTriple{gs, 0, xs}
	if _, found, e := s.triples.Get(tkey, gs.hash+xs.hash); e != nil || !found {
		t.Fatalf("substituted triple: %v %v", found, e)
	}
	key := levelPair{fs, 0}
	if _, found, e := s.lower.Get(key, fs.hash); e != nil || !found {
		t.Fatalf("source alp.op lower constraint: %v %v", found, e)
	}
}

func TestLevelDriverInterleavesAndSkipsAfterError(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name + ".tla", Origin: modulegraph.OriginWorkspace, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	dep := input("A", "VARIABLE x\nASSUME x")
	root := input("Root", "EXTENDS A")
	ctx := stdcontext.Background()
	a, e := modulegraph.AcquireFlatExternal(ctx, root, func(stdcontext.Context, string) (modulegraph.Input, error) { return dep, nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if e != nil {
		t.Fatal(e)
	}
	ids := map[leafNode]uint64{}
	var next uint64
	c, e := newLevelChecker(ctx, func(n leafNode) (uint64, uint32, error) {
		if ids[n] == 0 {
			next++
			ids[n] = next
		}
		return ids[n], uint32(961+31*n.header().kind) + uint32(ids[n]), nil
	}, 1000, 100000)
	if e != nil {
		t.Fatal(e)
	}
	details, examined, result, attempts, e := analyzeFlatExternalLevelAttempt(ctx, a, 1000, c)
	if !errors.Is(e, ErrPortIncomplete) || !examined || result == nil || len(attempts) != 2 {
		t.Fatalf("driver: examined=%v root=%v attempts=%+v err=%v", examined, result, attempts, e)
	}
	if !attempts[0].entered || !attempts[0].completed || !attempts[1].skipped || attempts[1].entered {
		t.Fatalf("driver attempts: %+v", attempts)
	}
	if len(details) != 1 || details[0].Code.Value != 4206 {
		t.Fatalf("driver diagnostic: %+v", details)
	}
}

func TestLevelTemporalRangeDiagnosticUsesSymbolParameter(t *testing.T) {
	pool := newNamePool()
	p := &formalParamNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 11, identity: 42}, name: pool.intern("p")}}
	temporal := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 3}, name: pool.intern("T")}, levels: leafLevel{level: 3}}
	r := &opApplNode{operator: temporal, operands: []exprOrOpArg{}}
	op := &builtinOpDefNode{nodeHeader: nodeHeader{name: pool.intern("Quant")}, arity: 1, levels: leafLevel{checked: 99}, maxLevels: []int{3}, weights: []int{1}, isLeibnizArg: []bool{true}, isLeibniz: true}
	app := &opApplNode{operator: op, operands: []exprOrOpArg{&numeralNode{}}, ranges: []exprNode{r}, bounded: [][]*formalParamNode{{p}}}
	c := testLevelChecker(t, p, temporal)
	ok, e := c.check(app, 1)
	if e != nil || ok {
		t.Fatalf("range: %v %v", ok, e)
	}
	records := c.errors.Messages()
	if len(records) != 1 || records[0].Code.Value != 4315 || records[0].Parameters[1].Kind != SemanticNodeParameter || records[0].Parameters[1].Identity != 42 {
		t.Fatalf("range diagnostic: %+v", records)
	}
}

func TestLevelCaseOtherNullOperand(t *testing.T) {
	pool := newNamePool()
	pair := &builtinOpDefNode{nodeHeader: nodeHeader{name: pool.intern("$Pair")}, arity: 2, levels: leafLevel{checked: 99}, maxLevels: []int{3, 3}, weights: []int{1, 1}, isLeibnizArg: []bool{true, true}, isLeibniz: true}
	app := &opApplNode{operator: pair, operands: []exprOrOpArg{nil, &numeralNode{}}}
	c := testLevelChecker(t)
	if ok, e := c.check(app, 1); e != nil || !ok {
		t.Fatalf("CASE OTHER pair: %v %v", ok, e)
	}
}

func TestLevelTemporalPickReportsWithoutChangingBoolean(t *testing.T) {
	pool := newNamePool()
	v := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 3}, name: pool.intern("v")}, levels: leafLevel{level: 1}}
	rng := &opApplNode{operator: v, operands: []exprOrOpArg{}}
	pick := &builtinOpDefNode{nodeHeader: nodeHeader{name: pool.intern("$Pick")}, arity: 1, levels: leafLevel{checked: 99, level: 3}, maxLevels: []int{3}, weights: []int{1}, isLeibniz: true, isLeibnizArg: []bool{true}}
	body := &opApplNode{operator: pick, operands: []exprOrOpArg{&numeralNode{}}, ranges: []exprNode{rng}}
	thm := &theoremNode{body: body, proof: &leafProofNode{}}
	c := testLevelChecker(t, v)
	ok, e := c.check(thm, 1)
	if e != nil || !ok {
		t.Fatalf("temporal PICK boolean: %v %v", ok, e)
	}
	records := c.errors.Messages()
	if len(records) != 1 || records[0].Code.Value != 4354 {
		t.Fatalf("PICK diagnostic: %+v", records)
	}
}
