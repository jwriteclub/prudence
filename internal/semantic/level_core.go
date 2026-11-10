// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from the TLA+ semantic level-checking classes, Copyright 2003
// Compaq Corporation and Microsoft Corporation, under the MIT license.
package semantic

import (
	stdcontext "context"
	"reflect"
	"unicode/utf16"

	"github.com/jwriteclub/prudence/internal/levelcollections"
)

// levelIdentity supplies a proved Java identity and hash. Equal identity values
// must have equal hashes; zero is reserved for an unknown identity. It is not
// derived from Go addresses, graph IDs, or the semantic-only history profile.
type levelIdentity func(leafNode) (identity uint64, hash uint32, err error)

type levelSymbol struct {
	node leafNode
	id   uint64
	hash uint32
}
type levelPair struct {
	symbol levelSymbol
	pos    int
}
type levelTriple struct {
	op    levelSymbol
	pos   int
	param levelSymbol
}
type levelSet = levelcollections.Map[levelSymbol, struct{}]
type levelUpper = levelcollections.Map[levelSymbol, int]
type levelLower = levelcollections.Map[levelPair, int]
type levelTriples = levelcollections.Map[levelTriple, struct{}]

type levelState struct {
	checked, shadow, level  int
	correct                 bool
	params, all, nonLeibniz *levelSet
	upper                   *levelUpper
	lower                   *levelLower
	triples                 *levelTriples
	maxLevels, weights      []int
	leibniz                 []bool
	isLeibniz               bool
	minMax                  [][]int
	conditions              [][][]bool
}

type levelChecker struct {
	ctx               stdcontext.Context
	identity          levelIdentity
	errors            *Errors
	owner             *levelcollections.Owner
	states            map[any]*levelState
	hashes            map[uint64]uint32
	stopped           error
	depth, depthLimit int
	work, workLimit   uint64
	incomplete        incompleteOrigin // private origin telemetry; write-only
}

func newLevelChecker(ctx stdcontext.Context, id levelIdentity, depthLimit int, workLimit uint64) (*levelChecker, error) {
	if ctx == nil || id == nil || depthLimit <= 0 || workLimit == 0 {
		return nil, ErrPortIncomplete
	}
	c := &levelChecker{ctx: ctx, identity: id, depthLimit: depthLimit, workLimit: workLimit, errors: &Errors{}, states: make(map[any]*levelState), hashes: make(map[uint64]uint32)}
	c.owner = levelcollections.NewOwner(levelcollections.Limits{Entries: workLimit, Capacity: workLimit, Work: workLimit, Check: func() error { return c.ctx.Err() }})
	return c, nil
}
func (c *levelChecker) step() error {
	if err := c.ctx.Err(); err != nil {
		return err
	}
	if c.work >= c.workLimit {
		return c.unported("level.resource.work-budget")
	}
	c.work++
	return nil
}
func (c *levelChecker) reserve(n uint64) error {
	if err := c.ctx.Err(); err != nil {
		return err
	}
	if n > c.workLimit-c.work {
		return c.unported("level.resource.work-budget")
	}
	c.work += n
	return nil
}
func (c *levelChecker) sym(n leafNode) (levelSymbol, error) {
	if n == nil || reflect.ValueOf(n).Kind() != reflect.Pointer || reflect.ValueOf(n).IsNil() {
		return levelSymbol{}, c.unported("level.precondition.nil-symbol")
	}
	if e := c.step(); e != nil {
		return levelSymbol{}, e
	}
	id, h, e := c.identity(n)
	if e != nil {
		return levelSymbol{}, e
	}
	if id == 0 {
		return levelSymbol{}, c.unported("level.mapping.zero-identity")
	}
	if old, ok := c.hashes[id]; ok && old != h {
		return levelSymbol{}, c.unported("level.mapping.identity-hash-conflict")
	}
	c.hashes[id] = h
	return levelSymbol{n, id, h}, nil
}
func symbolEq(a, b levelSymbol) bool { return a.id == b.id }
func pairEq(a, b levelPair) bool     { return a.pos == b.pos && symbolEq(a.symbol, b.symbol) }
func tripleEq(a, b levelTriple) bool {
	return a.pos == b.pos && symbolEq(a.op, b.op) && symbolEq(a.param, b.param)
}
func (c *levelChecker) state(n any) (*levelState, error) {
	if n == nil || reflect.ValueOf(n).Kind() != reflect.Pointer || reflect.ValueOf(n).IsNil() {
		return nil, c.unported("level.precondition.nil-node")
	}
	if s := c.states[n]; s != nil {
		return s, nil
	}
	if err := c.step(); err != nil {
		return nil, err
	}
	s := &levelState{correct: true, isLeibniz: true}
	var err error
	if s.params, err = levelcollections.NewMap[levelSymbol, struct{}](c.owner, symbolEq); err != nil {
		return nil, err
	}
	if s.all, err = levelcollections.NewMap[levelSymbol, struct{}](c.owner, symbolEq); err != nil {
		return nil, err
	}
	if s.nonLeibniz, err = levelcollections.NewMap[levelSymbol, struct{}](c.owner, symbolEq); err != nil {
		return nil, err
	}
	if s.upper, err = levelcollections.NewMap[levelSymbol, int](c.owner, symbolEq); err != nil {
		return nil, err
	}
	if s.lower, err = levelcollections.NewMap[levelPair, int](c.owner, pairEq); err != nil {
		return nil, err
	}
	if s.triples, err = levelcollections.NewMap[levelTriple, struct{}](c.owner, tripleEq); err != nil {
		return nil, err
	}
	switch v := n.(type) {
	case *opDeclNode:
		s.level = v.levels.level
		s.checked = 1
		if v.kind == 2 {
			p, e := c.sym(v)
			if e != nil {
				return nil, e
			}
			if e = s.params.Put(p, p.hash, struct{}{}); e != nil {
				return nil, e
			}
			if e = s.all.Put(p, p.hash, struct{}{}); e != nil {
				return nil, e
			}
		}
	case *builtinOpDefNode:
		for _, n := range []int{len(v.maxLevels), len(v.weights), len(v.isLeibnizArg)} {
			if e := c.reserve(uint64(n)); e != nil {
				return nil, e
			}
		}
		if v.levels.checked != 99 {
			return nil, c.unported("level.operation.builtin-levels-unavailable")
		}
		s.level = v.levels.level
		s.checked = 99
		s.maxLevels = append([]int(nil), v.maxLevels...)
		s.weights = append([]int(nil), v.weights...)
		s.leibniz = append([]bool(nil), v.isLeibnizArg...)
		s.isLeibniz = v.isLeibniz
	case *userOpDefNode:
		for _, n := range []int{len(v.maxLevels), len(v.weights), len(v.isLeibnizArg)} {
			if e := c.reserve(uint64(n)); e != nil {
				return nil, e
			}
		}
		s.maxLevels = append([]int(nil), v.maxLevels...)
		s.weights = append([]int(nil), v.weights...)
		s.leibniz = append([]bool(nil), v.isLeibnizArg...)
		s.isLeibniz = v.isLeibniz
	}
	c.states[n] = s
	return s, nil
}
func (c *levelChecker) putUpper(s *levelState, n levelSymbol, v int) error {
	old, ok, e := s.upper.Get(n, n.hash)
	if e != nil {
		return e
	}
	if !ok {
		old = 3
	}
	if old < v {
		v = old
	}
	return s.upper.Put(n, n.hash, v)
}
func (c *levelChecker) putLower(s *levelState, n levelPair, v int) error {
	h := n.symbol.hash + uint32(n.pos)
	old, ok, e := s.lower.Get(n, h)
	if e != nil {
		return e
	}
	if !ok {
		old = 0
	}
	if old > v {
		v = old
	}
	return s.lower.Put(n, h, v)
}
func (c *levelChecker) merge(dst, src *levelState) error {
	if e := dst.params.PutAll(src.params); e != nil {
		return e
	}
	if e := dst.all.PutAll(src.all); e != nil {
		return e
	}
	if e := src.upper.Each(func(k levelSymbol, v int, _ uint32) error { return c.putUpper(dst, k, v) }); e != nil {
		return e
	}
	if e := src.lower.Each(func(k levelPair, v int, _ uint32) error { return c.putLower(dst, k, v) }); e != nil {
		return e
	}
	return dst.triples.PutAll(src.triples)
}

// check returns the Java boolean separately from an unsupported/resource stop.
// Cache marks precede recursive descent, matching the source's cycle handling.
func (c *levelChecker) check(n any, iter int) (bool, error) {
	if c.stopped != nil {
		return false, c.stopped
	}
	ok, err := c.checkNode(n, iter)
	if err != nil {
		c.stopped = err
	}
	return ok, err
}
func (c *levelChecker) checkNode(n any, iter int) (bool, error) {
	if err := c.step(); err != nil {
		return false, err
	}
	if iter <= 0 {
		return false, c.unported("level.precondition.invalid-iteration")
	}
	if c.depth >= c.depthLimit {
		return false, c.unported("level.resource.depth-limit")
	}
	c.depth++
	defer func() { c.depth-- }()
	s, err := c.state(n)
	if err != nil {
		return false, err
	}
	switch v := n.(type) {
	case *numeralNode, *decimalNode, *stringNode:
		s.checked = iter
		return true, nil
	case *formalParamNode:
		if s.checked == 0 {
			s.checked = iter
			p, e := c.sym(v)
			if e != nil {
				return false, e
			}
			if e = s.params.Put(p, p.hash, struct{}{}); e != nil {
				return false, e
			}
			if e = s.all.Put(p, p.hash, struct{}{}); e != nil {
				return false, e
			}
		}
		return true, nil
	case *opDeclNode, *builtinOpDefNode:
		return true, nil
	case *opArgNode:
		if s.checked >= iter {
			return s.correct, nil
		}
		s.checked = iter
		ok, e := c.check(v.operator, iter)
		if e != nil {
			return false, e
		}
		src, e := c.data(v.operator)
		if e != nil {
			return false, e
		}
		s.correct = ok
		s.level = src.level
		s.params = src.params
		s.all = src.all
		s.upper = src.upper
		s.lower = src.lower
		s.triples = src.triples
		return ok, nil
	case *labelNode:
		if s.checked >= iter {
			return true, nil
		}
		s.checked = iter
		for _, p := range v.params {
			if p != nil {
				if _, e := c.check(p, iter); e != nil {
					return false, e
				}
			}
		}
		return c.check(v.body, iter)
	case *leafProofNode:
		return c.checkProofSubnodes(s, iter, levelNodesAsAny(v.facts))
	case *useOrHideNode:
		return c.checkProofSubnodes(s, iter, levelNodesAsAny(v.facts))
	case *nonLeafProofNode:
		subs := levelNodesAsAny(v.steps)
		for _, inst := range v.insts {
			subs = append(subs, inst)
		}
		return c.checkProofSubnodes(s, iter, subs)
	case *defStepNode:
		subs := make([]any, 0, len(v.defs))
		for _, def := range v.defs {
			subs = append(subs, def)
		}
		return c.checkProofSubnodes(s, iter, subs)
	case *userOpDefNode:
		if s.checked >= iter || (!v.inRecursiveSection && s.checked > 0) {
			return s.correct, nil
		}
		s.checked = iter
		if v.kind == 37 {
			// OpDefNode marks its own cache before calling levelCheckSubnodes;
			// that helper therefore returns without aggregating the step.
			ok, err := c.check(v.stepNode, iter)
			if err != nil {
				return false, err
			}
			s.correct = ok
			return s.correct, nil
		}
		if v.kind == 6 {
			return false, c.unported("level.operation.instance-marker-definition")
		}
		if v.body == nil {
			return false, c.unported("level.precondition.missing-definition-body")
		}
		ok, e := c.check(v.body, iter)
		if e != nil {
			return false, e
		}
		body, e := c.data(v.body)
		if e != nil {
			return false, e
		}
		s.correct = ok
		if s.level < body.level {
			s.level = body.level
		}
		if len(v.params) != len(s.maxLevels) || len(v.params) != len(s.weights) || len(v.params) != len(s.leibniz) {
			return false, c.unported("level.precondition.definition-signature-shape")
		}
		if e = c.reserve(uint64(len(v.params))); e != nil {
			return false, e
		}
		syms := make([]levelSymbol, len(v.params))
		for i, p := range v.params {
			if p == nil || p.arity < 0 {
				return false, c.unported("level.precondition.formal-parameter-shape")
			}
			syms[i], e = c.sym(p)
			if e != nil {
				return false, e
			}
			sym := syms[i]
			if x, found, e := body.upper.Get(sym, sym.hash); e != nil {
				return false, e
			} else if found && x < s.maxLevels[i] {
				s.maxLevels[i] = x
			}
			if _, found, e := body.params.Get(sym, sym.hash); e != nil {
				return false, e
			} else if found {
				s.weights[i] = 1
			}
		}
		// Count the complete matrix before allocating any row.
		remaining := c.workLimit - c.work
		nparams := uint64(len(v.params))
		if nparams > remaining || (nparams != 0 && nparams > remaining/nparams) {
			return false, c.unported("level.resource.work-budget")
		}
		remaining -= nparams * nparams
		if nparams > remaining/2 {
			return false, c.unported("level.resource.work-budget")
		}
		remaining -= 2 * nparams
		for _, p := range v.params {
			arity := uint64(p.arity)
			if nparams == ^uint64(0) || arity > remaining/(nparams+1) {
				return false, c.unported("level.resource.work-budget")
			}
			cost := arity * (nparams + 1)
			remaining -= cost
		}
		c.work = c.workLimit - remaining
		s.minMax = make([][]int, len(v.params))
		s.conditions = make([][][]bool, len(v.params))
		for i, p := range v.params {
			s.minMax[i] = make([]int, p.arity)
			s.conditions[i] = make([][]bool, len(v.params))
			for k := 0; k < p.arity; k++ {
				key := levelPair{syms[i], k}
				x, found, e := body.lower.Get(key, key.symbol.hash+uint32(k))
				if e != nil {
					return false, e
				}
				if found {
					s.minMax[i][k] = x
				}
			}
			for j := range v.params {
				s.conditions[i][j] = make([]bool, p.arity)
				for k := 0; k < p.arity; k++ {
					t := levelTriple{syms[i], k, syms[j]}
					_, found, e := body.triples.Get(t, t.op.hash+uint32(k)+t.param.hash)
					if e != nil {
						return false, e
					}
					s.conditions[i][j][k] = found
				}
			}
		}
		if e = s.params.PutAll(body.params); e != nil {
			return false, e
		}
		if e = s.all.PutAll(body.all); e != nil {
			return false, e
		}
		if e = s.nonLeibniz.PutAll(body.nonLeibniz); e != nil {
			return false, e
		}
		for i, p := range syms {
			if _, e = s.params.Remove(p, p.hash); e != nil {
				return false, e
			}
			if _, e = s.all.Remove(p, p.hash); e != nil {
				return false, e
			}
			removed, e := s.nonLeibniz.Remove(p, p.hash)
			if e != nil {
				return false, e
			}
			if removed {
				s.leibniz[i] = false
				s.isLeibniz = false
			}
		}
		s.upper, e = levelcollections.CopyMap(c.owner, body.upper)
		if e != nil {
			return false, e
		}
		s.lower, e = levelcollections.CopyMap(c.owner, body.lower)
		if e != nil {
			return false, e
		}
		for i, p := range syms {
			if _, e = s.upper.Remove(p, p.hash); e != nil {
				return false, e
			}
			for k := 0; k < v.params[i].arity; k++ {
				if _, e = s.lower.Remove(levelPair{p, k}, p.hash+uint32(k)); e != nil {
					return false, e
				}
			}
		}
		e = body.triples.Each(func(t levelTriple, _ struct{}, _ uint32) error {
			hasOp, hasParam := false, false
			for _, p := range syms {
				hasOp = hasOp || symbolEq(t.op, p)
				hasParam = hasParam || symbolEq(t.param, p)
			}
			if !hasOp || !hasParam {
				return s.triples.Put(t, t.op.hash+uint32(t.pos)+t.param.hash, struct{}{})
			}
			return nil
		})
		if e != nil {
			return false, e
		}
		return ok, nil
	case *thmOrAssumpDefNode:
		for i := 0; i < 3; i++ {
			if e := c.reserve(uint64(len(v.params))); e != nil {
				return false, e
			}
		}
		if s.checked >= iter {
			return s.correct, nil
		}
		s.checked = iter
		if v.body == nil {
			return false, c.unported("level.precondition.missing-definition-body")
		}
		s.maxLevels = make([]int, len(v.params))
		s.weights = make([]int, len(v.params))
		s.leibniz = nil
		s.isLeibniz = false
		for i := range v.params {
			s.maxLevels[i] = 3
			s.isLeibniz = true
			s.leibniz = make([]bool, len(v.params))
			s.leibniz[i] = true
		}
		ok, e := c.check(v.body, iter)
		if e != nil {
			return false, e
		}
		body, e := c.data(v.body)
		if e != nil {
			return false, e
		}
		s.correct = ok
		s.level = body.level
		if len(v.params) != len(s.maxLevels) || len(v.params) != len(s.weights) || len(v.params) != len(s.leibniz) {
			return false, c.unported("level.precondition.definition-signature-shape")
		}
		if e = c.reserve(uint64(len(v.params))); e != nil {
			return false, e
		}
		syms := make([]levelSymbol, len(v.params))
		for i, p := range v.params {
			if p == nil || p.arity < 0 {
				return false, c.unported("level.precondition.formal-parameter-shape")
			}
			syms[i], e = c.sym(p)
			if e != nil {
				return false, e
			}
			sym := syms[i]
			if x, found, e := body.upper.Get(sym, sym.hash); e != nil {
				return false, e
			} else if found && x < s.maxLevels[i] {
				s.maxLevels[i] = x
			}
			if _, found, e := body.params.Get(sym, sym.hash); e != nil {
				return false, e
			} else if found {
				// The source assigns the existing weight to itself.
			}
		}
		// Count the complete matrix before allocating any row.
		remaining := c.workLimit - c.work
		nparams := uint64(len(v.params))
		if nparams > remaining || (nparams != 0 && nparams > remaining/nparams) {
			return false, c.unported("level.resource.work-budget")
		}
		remaining -= nparams * nparams
		if nparams > remaining/2 {
			return false, c.unported("level.resource.work-budget")
		}
		remaining -= 2 * nparams
		for _, p := range v.params {
			arity := uint64(p.arity)
			if nparams == ^uint64(0) || arity > remaining/(nparams+1) {
				return false, c.unported("level.resource.work-budget")
			}
			cost := arity * (nparams + 1)
			remaining -= cost
		}
		c.work = c.workLimit - remaining
		s.minMax = make([][]int, len(v.params))
		s.conditions = make([][][]bool, len(v.params))
		for i, p := range v.params {
			s.minMax[i] = make([]int, p.arity)
			s.conditions[i] = make([][]bool, len(v.params))
			for k := 0; k < p.arity; k++ {
				key := levelPair{syms[i], k}
				x, found, e := body.lower.Get(key, key.symbol.hash+uint32(k))
				if e != nil {
					return false, e
				}
				if found {
					s.minMax[i][k] = x
				}
			}
			for j := range v.params {
				s.conditions[i][j] = make([]bool, p.arity)
				for k := 0; k < p.arity; k++ {
					t := levelTriple{syms[i], k, syms[j]}
					_, found, e := body.triples.Get(t, t.op.hash+uint32(k)+t.param.hash)
					if e != nil {
						return false, e
					}
					s.conditions[i][j][k] = found
				}
			}
		}
		if e = s.params.PutAll(body.params); e != nil {
			return false, e
		}
		if e = s.all.PutAll(body.all); e != nil {
			return false, e
		}
		if e = s.nonLeibniz.PutAll(body.nonLeibniz); e != nil {
			return false, e
		}
		for i, p := range syms {
			if _, e = s.params.Remove(p, p.hash); e != nil {
				return false, e
			}
			if _, e = s.all.Remove(p, p.hash); e != nil {
				return false, e
			}
			removed, e := s.nonLeibniz.Remove(p, p.hash)
			if e != nil {
				return false, e
			}
			if removed {
				s.leibniz[i] = false
				s.isLeibniz = false
			}
		}
		s.upper, e = levelcollections.CopyMap(c.owner, body.upper)
		if e != nil {
			return false, e
		}
		s.lower, e = levelcollections.CopyMap(c.owner, body.lower)
		if e != nil {
			return false, e
		}
		for i, p := range syms {
			if _, e = s.upper.Remove(p, p.hash); e != nil {
				return false, e
			}
			for k := 0; k < v.params[i].arity; k++ {
				if _, e = s.lower.Remove(levelPair{p, k}, p.hash+uint32(k)); e != nil {
					return false, e
				}
			}
		}
		e = body.triples.Each(func(t levelTriple, _ struct{}, _ uint32) error {
			hasOp, hasParam := false, false
			for _, p := range syms {
				hasOp = hasOp || symbolEq(t.op, p)
				hasParam = hasParam || symbolEq(t.param, p)
			}
			if !hasOp || !hasParam {
				return s.triples.Put(t, t.op.hash+uint32(t.pos)+t.param.hash, struct{}{})
			}
			return nil
		})
		if e != nil {
			return false, e
		}
		return ok, nil
	case *moduleNode:
		return c.checkModule(v, s, iter)
	case *instanceNode:
		return c.checkInstance(v, s, iter)
	case *substInNode:
		return c.checkSubstitution(v.body, v.substs, v.instantiatedModule, s, iter, false)
	case *apSubstInNode:
		return c.checkSubstitution(v.body, v.substs, v.instantiatedModule, s, iter, true)
	case *theoremNode:
		return c.checkTheorem(v, s, iter)
	case *newSymbNode:
		return c.checkNew(v, s, iter)
	case *assumeProveNode:
		return c.checkAssumeProve(v, s, iter)
	case *assumeNode:
		return c.checkAssume(v, s, iter)
	case *opApplNode:
		return c.checkApplication(v, s, iter)
	case *atNode:
		return c.checkAt(v, s, iter)
	case *letInNode:
		if s.checked >= iter {
			return s.correct, nil
		}
		s.checked = iter
		s.correct = true
		for _, d := range v.definitions {
			if d.header().kind != 6 {
				ok, e := c.check(d, iter)
				if e != nil {
					return false, e
				}
				s.correct = s.correct && ok
			}
		}
		ok, e := c.check(v.body, iter)
		if e != nil {
			return false, e
		}
		s.correct = s.correct && ok
		for _, in := range v.instances {
			ok, e = c.check(in, iter)
			if e != nil {
				return false, e
			}
			s.correct = s.correct && ok
		}
		b, e := c.data(v.body)
		if e != nil {
			return false, e
		}
		s.level = b.level
		s.params, e = levelcollections.CopySetMap(c.owner, b.params)
		if e != nil {
			return false, e
		}
		s.all, e = levelcollections.CopySetMap(c.owner, b.all)
		if e != nil {
			return false, e
		}
		if e = c.mergeConstraints(s, b); e != nil {
			return false, e
		}
		for _, d := range v.definitions {
			if d.header().kind == 6 {
				continue
			}
			ds, e := c.data(d)
			if e != nil {
				return false, e
			}
			if e = c.mergeLetDefinition(s, ds, d); e != nil {
				return false, e
			}
		}
		for _, in := range v.instances {
			is, e := c.data(in)
			if e != nil {
				return false, e
			}
			if e = s.triples.PutAll(is.triples); e != nil {
				return false, e
			}
		}
		return s.correct, nil
	default:
		return false, c.unported("level.operation.unsupported-node")
	}
}

func (c *levelChecker) data(n any) (*levelState, error) {
	s, e := c.state(n)
	if e != nil {
		return nil, e
	}
	if s.checked == 0 {
		return nil, c.unported("level.precondition.unchecked-state")
	}
	if _, ok := n.(*moduleNode); ok {
		return nil, c.unported("level.operation.module-state")
	}
	var body any
	switch v := n.(type) {
	case *labelNode:
		body = v.body
	case *assumeNode:
		body = v.expression
	}
	if body != nil {
		b, e := c.data(body)
		if e != nil {
			return nil, e
		}
		result := *b
		result.nonLeibniz = s.nonLeibniz
		return &result, nil
	}
	return s, nil
}
func (c *levelChecker) mergeConstraints(dst, src *levelState) error {
	if e := src.upper.Each(func(k levelSymbol, v int, _ uint32) error { return c.putUpper(dst, k, v) }); e != nil {
		return e
	}
	if e := src.lower.Each(func(k levelPair, v int, _ uint32) error { return c.putLower(dst, k, v) }); e != nil {
		return e
	}
	return dst.triples.PutAll(src.triples)
}

// checkApplication implements the declared/formal-operator branch. Definition
// applications, binders and their temporal diagnostics remain guarded until
// their complete source branches are implemented.
func (c *levelChecker) checkApplication(n *opApplNode, s *levelState, iter int) (bool, error) {
	if s.checked >= iter {
		return s.correct, nil
	}
	s.checked = iter
	s.correct = true
	switch n.operator.(type) {
	case *formalParamNode, *opDeclNode:
	case *userOpDefNode, *builtinOpDefNode, *thmOrAssumpDefNode:
		return c.checkDefinitionApplication(n, s, iter)
	default:
		return false, c.unported("level.operation.unsupported-operator")
	}
	if len(n.ranges) != 0 || len(n.unbounded) != 0 || len(n.bounded) != 0 {
		return false, c.unported("level.operation.parameter-application-bindings")
	}
	for _, arg := range n.operands {
		if arg == nil {
			return false, c.unported("level.precondition.nil-operand")
		}
		ok, e := c.check(arg, iter)
		if e != nil {
			return false, e
		}
		s.correct = s.correct && ok
	}
	if _, e := c.check(n.operator, iter); e != nil {
		return false, e
	}
	op, e := c.data(n.operator)
	if e != nil {
		return false, e
	}
	s.level = op.level
	sym, e := c.sym(n.operator)
	if e != nil {
		return false, e
	}
	// Java replaces the level-parameter set but retains and updates allParams.
	s.params, e = levelcollections.NewMap[levelSymbol, struct{}](c.owner, symbolEq)
	if e != nil {
		return false, e
	}
	if e = s.params.Put(sym, sym.hash, struct{}{}); e != nil {
		return false, e
	}
	if e = s.all.Put(sym, sym.hash, struct{}{}); e != nil {
		return false, e
	}
	for _, arg := range n.operands {
		if _, e = c.check(arg, iter); e != nil {
			return false, e
		}
		a, e := c.data(arg)
		if e != nil {
			return false, e
		}
		if a.level > s.level {
			s.level = a.level
		}
		if e = s.params.PutAll(a.params); e != nil {
			return false, e
		}
		if e = s.all.PutAll(a.all); e != nil {
			return false, e
		}
		if e = s.nonLeibniz.PutAll(a.nonLeibniz); e != nil {
			return false, e
		}
	}
	s.upper, e = levelcollections.NewMap[levelSymbol, int](c.owner, symbolEq)
	if e != nil {
		return false, e
	}
	for _, arg := range n.operands {
		a, e := c.data(arg)
		if e != nil {
			return false, e
		}
		if e = a.upper.Each(func(k levelSymbol, v int, _ uint32) error { return c.putUpper(s, k, v) }); e != nil {
			return false, e
		}
	}
	s.lower, e = levelcollections.NewMap[levelPair, int](c.owner, pairEq)
	if e != nil {
		return false, e
	}
	for i, arg := range n.operands {
		a, e := c.data(arg)
		if e != nil {
			return false, e
		}
		if e = c.putLower(s, levelPair{sym, i}, a.level); e != nil {
			return false, e
		}
		if e = a.lower.Each(func(k levelPair, v int, _ uint32) error { return c.putLower(s, k, v) }); e != nil {
			return false, e
		}
	}
	s.triples, e = levelcollections.NewMap[levelTriple, struct{}](c.owner, tripleEq)
	if e != nil {
		return false, e
	}
	for i, arg := range n.operands {
		a, e := c.data(arg)
		if e != nil {
			return false, e
		}
		if e = a.params.Each(func(p levelSymbol, _ struct{}, _ uint32) error {
			t := levelTriple{sym, i, p}
			return s.triples.Put(t, sym.hash+uint32(i)+p.hash, struct{}{})
		}); e != nil {
			return false, e
		}
		if e = s.triples.PutAll(a.triples); e != nil {
			return false, e
		}
	}
	return c.specialTemporal(n, s)
}

// At checks the current component to terminate EXCEPT cycles, but includes
// only the base and earlier components in its resulting level information.
func (c *levelChecker) checkAt(n *atNode, s *levelState, iter int) (bool, error) {
	if s.checked >= iter {
		return true, nil
	}
	s.checked = iter
	if n.except == nil || n.component == nil || len(n.except.operands) == 0 {
		return false, c.unported("level.precondition.at-shape")
	}
	args := n.except.operands
	for i, arg := range args {
		if arg == nil {
			return false, c.unported("level.precondition.at-shape")
		}
		if _, e := c.check(arg, iter); e != nil {
			return false, e
		}
		if i > 0 && arg == n.component {
			break
		}
	}
	for i, arg := range args {
		if i > 0 && arg == n.component {
			break
		}
		a, e := c.data(arg)
		if e != nil {
			return false, e
		}
		if a.level > s.level {
			s.level = a.level
		}
		if e = c.merge(s, a); e != nil {
			return false, e
		}
	}
	return true, nil
}
func (c *levelChecker) mergeLetDefinition(dst, src *levelState, def leafNode) error {
	if e := src.upper.Each(func(k levelSymbol, v int, _ uint32) error { return c.putUpper(dst, k, v) }); e != nil {
		return e
	}
	if e := src.lower.Each(func(k levelPair, v int, _ uint32) error { return c.putLower(dst, k, v) }); e != nil {
		return e
	}
	var params []*formalParamNode
	if d, ok := def.(*userOpDefNode); ok {
		params = d.params
	}
	syms := make([]levelSymbol, 0, len(params))
	for _, p := range params {
		sym, e := c.sym(p)
		if e != nil {
			return e
		}
		syms = append(syms, sym)
	}
	return src.triples.Each(func(t levelTriple, _ struct{}, _ uint32) error {
		for _, p := range syms {
			if symbolEq(t.op, p) || symbolEq(t.param, p) {
				return nil
			}
		}
		return dst.triples.Put(t, t.op.hash+uint32(t.pos)+t.param.hash, struct{}{})
	})
}

// checkRecursiveSections is ModuleNode's ordered two-pass recursive-section
// phase. It is separate from the still unsupported remainder of module checks.
func (c *levelChecker) checkRecursiveSections(defs []*userOpDefNode, errors *Errors) error {
	if errors == nil {
		return c.unported("level.precondition.checker-errors")
	}
	for first := 0; first < len(defs); {
		if defs[first] == nil {
			return c.unported("level.precondition.nil-recursive-definition")
		}
		end := first + 1
		for end < len(defs) && defs[end] != nil && defs[end].recursiveSection == defs[first].recursiveSection {
			end++
		}
		for _, d := range defs[first:end] {
			s, e := c.state(d)
			if e != nil {
				return e
			}
			if d.inRecursive {
				s.checked = 1
				for i := range s.maxLevels {
					s.maxLevels[i] = 2
					s.weights[i] = 1
				}
			} else {
				s.checked = 0
			}
		}
		lp, e := levelcollections.NewMap[levelSymbol, struct{}](c.owner, symbolEq)
		if e != nil {
			return e
		}
		ap, e := levelcollections.NewMap[levelSymbol, struct{}](c.owner, symbolEq)
		if e != nil {
			return e
		}
		maxLevel := 0
		for _, d := range defs[first:end] {
			s, e := c.state(d)
			if e != nil {
				return e
			}
			if d.inRecursive {
				s.checked = 0
			}
			if _, e = c.check(d, 1); e != nil {
				return e
			}
			if d.inRecursive {
				for i, v := range s.maxLevels {
					if v < 2 {
						loc, e := levelTreeLocation(&d.semanticHeader)
						if e != nil {
							return e
						}
						if d.name == nil {
							return c.unported("level.precondition.unnamed-definition")
						}
						if e = addError(errors, 4290, loc, "Argument %s of recursive operator %s is primed", Parameter{Kind: IntParameter, Integer: int64(i + 1)}, nameParam(d.name)); e != nil {
							return e
						}
					}
				}
				if s.level > maxLevel {
					maxLevel = s.level
				}
				if e = lp.PutAll(s.params); e != nil {
					return e
				}
				if e = ap.PutAll(s.all); e != nil {
					return e
				}
			}
		}
		for _, d := range defs[first:end] {
			s, e := c.state(d)
			if e != nil {
				return e
			}
			if d.inRecursive {
				s.checked = 2
			}
			if s.level < maxLevel {
				s.level = maxLevel
			}
			if e = s.params.PutAll(lp); e != nil {
				return e
			}
			if e = s.all.PutAll(ap); e != nil {
				return e
			}
		}
		for _, d := range defs[first:end] {
			s, e := c.state(d)
			if e != nil {
				return e
			}
			if d.inRecursive {
				s.checked = 1
			}
			if _, e = c.check(d, 2); e != nil {
				return e
			}
		}
		first = end
	}
	return nil
}

// checkDefinitionApplication follows the modeled definition-application
// branches. Source null-dereference cases and unrenderable diagnostic
// parameters remain explicit incomplete stops.
func (c *levelChecker) checkDefinitionApplication(n *opApplNode, s *levelState, iter int) (bool, error) {
	for _, arg := range n.operands {
		if arg == nil {
			continue
		}
		ok, e := c.check(arg, iter)
		if e != nil {
			return false, e
		}
		s.correct = s.correct && ok
	}
	for _, r := range n.ranges {
		if r == nil {
			return false, c.unported("level.precondition.nil-range")
		}
		ok, e := c.check(r, iter)
		if e != nil {
			return false, e
		}
		s.correct = s.correct && ok
	}
	opOK, e := c.check(n.operator, iter)
	if e != nil {
		return false, e
	}
	op, e := c.data(n.operator)
	if e != nil {
		return false, e
	}
	arity := nodeArity(n.operator)
	index := func(i int) int {
		if arity == -1 {
			return 0
		}
		return i
	}
	higher := func(i int, operand exprOrOpArg) error {
		arg, ok := operand.(*opArgNode)
		if !ok {
			return nil
		}
		switch arg.operator.(type) {
		case *userOpDefNode, *builtinOpDefNode, *thmOrAssumpDefNode:
			if _, e = c.check(arg.operator, iter); e != nil {
				return e
			}
			ad, e := c.data(arg.operator)
			if e != nil {
				return e
			}
			alen := nodeArity(arg.operator)
			if alen < 0 {
				return c.unported("level.precondition.operator-argument-arity")
			}
			for j := 0; j < alen; j++ {
				min, e := levelMin(op, i, j)
				if e != nil {
					return e
				}
				max, e := levelMax(ad, arg.operator, j)
				if e != nil {
					return e
				}
				if max < min {
					if opOK {
						aok, e := c.check(operand, iter)
						if e != nil {
							return e
						}
						if aok {
							if n.operator.header().name == nil {
								return c.unported("level.precondition.unnamed-operator")
							}
							loc, e := levelTreeLocation(&n.semanticHeader)
							if e != nil {
								return e
							}
							if e = addError(c.errors, 4272, loc, "Level error in applying operator %s:\nThe permitted level of argument %s of the operator argument %s \nmust be at least %s.", nameParam(n.operator.header().name), Parameter{Kind: IntParameter, Integer: int64(j + 1)}, Parameter{Kind: IntParameter, Integer: int64(i + 1)}, Parameter{Kind: IntParameter, Integer: int64(min)}); e != nil {
								return e
							}
						}
					}
					s.correct = false
				}
			}
			for j, other := range n.operands {
				v, e := c.data(other)
				if e != nil {
					return e
				}
				for k := 0; k < alen; k++ {
					cond, e := levelCond(op, i, j, k)
					if e != nil {
						return e
					}
					max, e := levelMax(ad, arg.operator, k)
					if e != nil {
						return e
					}
					if cond && v.level > max {
						left, e := c.check(operand, iter)
						if e != nil {
							return e
						}
						right, e := c.check(other, iter)
						if e != nil {
							return e
						}
						if left && right {
							loc, e := levelTreeLocation(&n.semanticHeader)
							if e != nil {
								return e
							}
							if n.operator.header().name == nil {
								return c.unported("level.precondition.unnamed-operator")
							}
							if e = addError(c.errors, 4273, loc, "Level error in applying operator %s:\nThe level of argument %s exceeds the maximum level allowed by the operator.", nameParam(n.operator.header().name), Parameter{Kind: IntParameter, Integer: int64(j + 1)}); e != nil {
								return e
							}
						}
						s.correct = false
					}
				}
			}
		}
		return nil
	}
	for i, operand := range n.operands {
		if operand == nil {
			continue
		}
		a, e := c.data(operand)
		if e != nil {
			return false, e
		}
		j := index(i)
		if j < 0 || j >= len(op.maxLevels) || j >= len(op.weights) || j >= len(op.leibniz) {
			return false, c.unported("level.precondition.argument-index")
		}
		if a.level > op.maxLevels[j] {
			if opOK {
				ok, e := c.check(operand, iter)
				if e != nil {
					return false, e
				}
				if ok {
					loc, e := levelTreeLocation(&n.semanticHeader)
					if e != nil {
						return false, e
					}
					if n.operator.header().name == nil {
						return false, c.unported("level.precondition.unnamed-operator")
					}
					if e = addError(c.errors, 4205, loc, "Level error in applying operator %s:\nThe level of argument %s exceeds the maximum level allowed by the operator.", nameParam(n.operator.header().name), Parameter{Kind: IntParameter, Integer: int64(i + 1)}); e != nil {
						return false, e
					}
				}
			}
			s.correct = false
		}
		if e = higher(i, operand); e != nil {
			return false, e
		}
	}
	for i, r := range n.ranges {
		ok, e := c.check(r, iter)
		if e != nil {
			return false, e
		}
		v, e := c.data(r)
		if e != nil {
			return false, e
		}
		if v.level > 2 {
			if ok {
				if i >= len(n.bounded) || len(n.bounded[i]) == 0 || n.bounded[i][0] == nil || n.operator.header().name == nil {
					return false, c.unported("level.precondition.bound-parameter-shape")
				}
				p := n.bounded[i][0]
				if p.identity == 0 {
					return false, c.unported("level.precondition.bound-parameter-shape")
				}
				loc, e := levelTreeLocation(&n.semanticHeader)
				if e != nil {
					return false, e
				}
				ploc, e := levelTreeLocation(&p.semanticHeader)
				if e != nil {
					return false, e
				}
				rendered, e := renderLocation(ploc)
				if e != nil {
					return false, e
				}
				param := Parameter{Kind: SemanticNodeParameter, Identity: p.identity, Text: rendered, UTF16Units: utf16.Encode([]rune(rendered)), UTF16Present: true}
				if e = addError(c.errors, 4315, loc, "Level error in applying operator %s:\nThe level of the range for the bounded variable %s \nexceeds the maximum level allowed by the operator.", nameParam(n.operator.header().name), param); e != nil {
					return false, e
				}
			}
			s.correct = false
		}
	}
	bounds, e := levelcollections.NewMap[levelSymbol, struct{}](c.owner, symbolEq)
	if e != nil {
		return false, e
	}
	for _, p := range n.unbounded {
		sym, e := c.sym(p)
		if e != nil {
			return false, e
		}
		if e = bounds.Put(sym, sym.hash, struct{}{}); e != nil {
			return false, e
		}
	}
	for _, group := range n.bounded {
		for _, p := range group {
			sym, e := c.sym(p)
			if e != nil {
				return false, e
			}
			if e = bounds.Put(sym, sym.hash, struct{}{}); e != nil {
				return false, e
			}
		}
	}
	bound := func(k levelSymbol) (bool, error) { _, ok, e := bounds.Get(k, k.hash); return ok, e }
	s.level = op.level
	for i, arg := range n.operands {
		if arg == nil {
			continue
		}
		a, e := c.data(arg)
		if e != nil {
			return false, e
		}
		j := index(i)
		if j < 0 || j >= len(op.maxLevels) || j >= len(op.weights) || j >= len(op.leibniz) {
			return false, c.unported("level.precondition.argument-index")
		}
		if op.weights[j] == 1 && a.level > s.level {
			s.level = a.level
		}
	}
	for _, r := range n.ranges {
		v, e := c.data(r)
		if e != nil {
			return false, e
		}
		if v.level > s.level {
			s.level = v.level
		}
	}
	if e = s.params.PutAll(op.params); e != nil {
		return false, e
	}
	if e = s.all.PutAll(op.all); e != nil {
		return false, e
	}
	if e = s.nonLeibniz.PutAll(op.nonLeibniz); e != nil {
		return false, e
	}
	for i, arg := range n.operands {
		if arg == nil {
			j := index(i)
			if j < 0 || j >= len(op.leibniz) || !op.leibniz[j] {
				return false, c.unported("level.precondition.null-non-leibniz-argument")
			}
			continue
		}
		a, e := c.data(arg)
		if e != nil {
			return false, e
		}
		j := index(i)
		if op.weights[j] == 1 {
			if e = s.params.PutAll(a.params); e != nil {
				return false, e
			}
		}
		if e = s.all.PutAll(a.all); e != nil {
			return false, e
		}
		if e = s.nonLeibniz.PutAll(a.nonLeibniz); e != nil {
			return false, e
		}
		if !op.leibniz[j] {
			if e = s.nonLeibniz.PutAll(a.all); e != nil {
				return false, e
			}
		}
	}
	for _, r := range n.ranges {
		v, e := c.data(r)
		if e != nil {
			return false, e
		}
		if e = s.params.PutAll(v.params); e != nil {
			return false, e
		}
		if e = s.all.PutAll(v.all); e != nil {
			return false, e
		}
		if e = s.nonLeibniz.PutAll(v.nonLeibniz); e != nil {
			return false, e
		}
	}
	if e = bounds.Each(func(k levelSymbol, _ struct{}, _ uint32) error {
		if _, e := s.params.Remove(k, k.hash); e != nil {
			return e
		}
		if _, e := s.all.Remove(k, k.hash); e != nil {
			return e
		}
		_, e := s.nonLeibniz.Remove(k, k.hash)
		return e
	}); e != nil {
		return false, e
	}
	if e = op.upper.Each(func(k levelSymbol, v int, _ uint32) error { return c.putUpper(s, k, v) }); e != nil {
		return false, e
	}
	for _, arg := range n.operands {
		if arg == nil {
			continue
		}
		a, e := c.data(arg)
		if e != nil {
			return false, e
		}
		if e = a.upper.Each(func(k levelSymbol, v int, _ uint32) error {
			yes, e := bound(k)
			if e != nil {
				return e
			}
			if yes {
				return nil
			}
			return c.putUpper(s, k, v)
		}); e != nil {
			return false, e
		}
	}
	for _, r := range n.ranges {
		v, e := c.data(r)
		if e != nil {
			return false, e
		}
		if e = v.upper.Each(func(k levelSymbol, x int, _ uint32) error { return c.putUpper(s, k, x) }); e != nil {
			return false, e
		}
	}
	for i, arg := range n.operands {
		if arg == nil {
			continue
		}
		a, e := c.data(arg)
		if e != nil {
			return false, e
		}
		max := op.maxLevels[index(i)]
		if e = a.params.Each(func(k levelSymbol, _ struct{}, _ uint32) error { return c.putUpper(s, k, max) }); e != nil {
			return false, e
		}
	}
	for i, operand := range n.operands {
		arg, ok := operand.(*opArgNode)
		if !ok {
			continue
		}
		switch arg.operator.(type) {
		case *userOpDefNode, *builtinOpDefNode, *thmOrAssumpDefNode:
			if _, e = c.check(arg.operator, iter); e != nil {
				return false, e
			}
			ad, e := c.data(arg.operator)
			if e != nil {
				return false, e
			}
			alen := nodeArity(arg.operator)
			if alen < 0 {
				return false, c.unported("level.precondition.operator-argument-arity")
			}
			for j, other := range n.operands {
				v, e := c.data(other)
				if e != nil {
					return false, e
				}
				for k := 0; k < alen; k++ {
					cond, e := levelCond(op, i, j, k)
					if e != nil {
						return false, e
					}
					if !cond {
						continue
					}
					max, e := levelMax(ad, arg.operator, k)
					if e != nil {
						return false, e
					}
					if e = v.params.Each(func(p levelSymbol, _ struct{}, _ uint32) error { return c.putUpper(s, p, max) }); e != nil {
						return false, e
					}
					if !ad.isLeibniz {
						if k >= len(ad.leibniz) {
							return false, c.unported("level.precondition.leibniz-index")
						}
						if !ad.leibniz[k] {
							if e = s.nonLeibniz.PutAll(v.all); e != nil {
								return false, e
							}
						}
					}
				}
			}
		}
	}
	if e = op.triples.Each(func(t levelTriple, _ struct{}, _ uint32) error {
		arg, e := c.definitionArgument(n, t.op)
		if e != nil {
			return e
		}
		actual, ok := arg.(*opArgNode)
		if !ok {
			return nil
		}
		switch actual.operator.(type) {
		case *userOpDefNode, *builtinOpDefNode, *thmOrAssumpDefNode:
			if _, e = c.check(actual.operator, iter); e != nil {
				return e
			}
			d, e := c.data(actual.operator)
			if e != nil {
				return e
			}
			max, e := levelMax(d, actual.operator, t.pos)
			if e != nil {
				return e
			}
			return c.putUpper(s, t.param, max)
		}
		return nil
	}); e != nil {
		return false, e
	}
	if e = op.lower.Each(func(k levelPair, v int, _ uint32) error { return c.putLower(s, k, v) }); e != nil {
		return false, e
	}
	for _, arg := range n.operands {
		if arg == nil {
			continue
		}
		a, e := c.data(arg)
		if e != nil {
			return false, e
		}
		if e = a.lower.Each(func(k levelPair, v int, _ uint32) error { return c.putLower(s, k, v) }); e != nil {
			return false, e
		}
	}
	for _, r := range n.ranges {
		v, e := c.data(r)
		if e != nil {
			return false, e
		}
		if e = v.lower.Each(func(k levelPair, x int, _ uint32) error { return c.putLower(s, k, x) }); e != nil {
			return false, e
		}
	}
	for i, operand := range n.operands {
		arg, ok := operand.(*opArgNode)
		if !ok || !levelIsParam(arg.operator) {
			continue
		}
		sym, e := c.sym(arg.operator)
		if e != nil {
			return false, e
		}
		alen := nodeArity(arg.operator)
		if alen < 0 {
			return false, c.unported("level.precondition.operator-argument-arity")
		}
		for j := 0; j < alen; j++ {
			min, e := levelMin(op, i, j)
			if e != nil {
				return false, e
			}
			if e = c.putLower(s, levelPair{sym, j}, min); e != nil {
				return false, e
			}
		}
		for j, other := range n.operands {
			v, e := c.data(other)
			if e != nil {
				return false, e
			}
			for k := 0; k < alen; k++ {
				cond, e := levelCond(op, i, j, k)
				if e != nil {
					return false, e
				}
				if cond {
					if e = c.putLower(s, levelPair{sym, k}, v.level); e != nil {
						return false, e
					}
				}
			}
		}
	}
	if e = op.triples.Each(func(t levelTriple, _ struct{}, _ uint32) error {
		arg, e := c.definitionArgument(n, t.op)
		if e != nil || arg == nil {
			return e
		}
		if _, e = c.check(arg, iter); e != nil {
			return e
		}
		v, e := c.data(arg)
		if e != nil {
			return e
		}
		return c.putLower(s, levelPair{t.op, t.pos}, v.level)
	}); e != nil {
		return false, e
	}
	s.triples, e = levelcollections.NewMap[levelTriple, struct{}](c.owner, tripleEq)
	if e != nil {
		return false, e
	}
	for _, arg := range n.operands {
		if arg == nil {
			continue
		}
		a, e := c.data(arg)
		if e != nil {
			return false, e
		}
		if e = a.triples.Each(func(t levelTriple, _ struct{}, _ uint32) error {
			yes, e := bound(t.param)
			if e != nil {
				return e
			}
			if yes {
				return nil
			}
			return s.triples.Put(t, t.op.hash+uint32(t.pos)+t.param.hash, struct{}{})
		}); e != nil {
			return false, e
		}
	}
	for _, r := range n.ranges {
		v, e := c.data(r)
		if e != nil {
			return false, e
		}
		if e = s.triples.PutAll(v.triples); e != nil {
			return false, e
		}
	}
	if e = op.triples.Each(func(t levelTriple, _ struct{}, _ uint32) error {
		arg, e := c.definitionArgument(n, t.op)
		if e != nil {
			return e
		}
		if arg == nil {
			arg, e = c.definitionArgument(n, t.param)
			if e != nil {
				return e
			}
			if arg == nil {
				return s.triples.Put(t, t.op.hash+uint32(t.pos)+t.param.hash, struct{}{})
			}
			if _, e = c.check(arg, iter); e != nil {
				return e
			}
			v, e := c.data(arg)
			if e != nil {
				return e
			}
			return v.params.Each(func(p levelSymbol, _ struct{}, _ uint32) error {
				z := levelTriple{t.op, t.pos, p}
				return s.triples.Put(z, z.op.hash+uint32(z.pos)+z.param.hash, struct{}{})
			})
		}
		actual, ok := arg.(*opArgNode)
		if !ok || !levelIsParam(actual.operator) {
			return nil
		}
		sym, e := c.sym(actual.operator)
		if e != nil {
			return e
		}
		z := levelTriple{sym, t.pos, t.param}
		return s.triples.Put(z, z.op.hash+uint32(z.pos)+z.param.hash, struct{}{})
	}); e != nil {
		return false, e
	}
	for i, operand := range n.operands {
		arg, ok := operand.(*opArgNode)
		if !ok || !levelIsParam(arg.operator) {
			continue
		}
		sym, e := c.sym(arg.operator)
		if e != nil {
			return false, e
		}
		alen := nodeArity(arg.operator)
		if alen < 0 {
			return false, c.unported("level.precondition.operator-argument-arity")
		}
		for j, other := range n.operands {
			v, e := c.data(other)
			if e != nil {
				return false, e
			}
			for k := 0; k < alen; k++ {
				cond, e := levelCond(op, i, j, k)
				if e != nil {
					return false, e
				}
				if cond {
					if e = v.params.Each(func(p levelSymbol, _ struct{}, _ uint32) error {
						t := levelTriple{sym, k, p}
						return s.triples.Put(t, sym.hash+uint32(k)+p.hash, struct{}{})
					}); e != nil {
						return false, e
					}
				}
			}
		}
	}
	return c.specialTemporal(n, s)
}

func levelTreeLocation(h *semanticHeader) (Location, error) {
	if h == nil {
		return Location{}, ErrPortIncomplete
	}
	if h.tree == nil {
		return Location{Null: true, Name: "--unknown--", NamePresent: true}, nil
	}
	l := h.tree.Location()
	if l == nil || h.owner == nil {
		return Location{}, ErrPortIncomplete
	}
	out := Location{SourceTree: h.tree, BeginLine: int(l.BeginLine()), BeginColumn: int(l.BeginColumn()), EndLine: int(l.EndLine()), EndColumn: int(l.EndColumn())}
	if src := l.Source(); src != nil {
		units := src.Units()
		out.Name = string(utf16.Decode(units))
		round := utf16.Encode([]rune(out.Name))
		if len(round) != len(units) {
			return Location{}, ErrPortIncomplete
		}
		for i := range units {
			if units[i] != round[i] {
				return Location{}, ErrPortIncomplete
			}
		}
		out.NamePresent = true
	}
	return out, nil
}

func (c *levelChecker) temporalConstants(s *levelState) error {
	return s.params.Each(func(k levelSymbol, _ struct{}, _ uint32) error {
		if k.node.header().kind == 2 {
			return c.putUpper(s, k, 2)
		}
		return nil
	})
}
func (c *levelChecker) checkNew(n *newSymbNode, s *levelState, iter int) (bool, error) {
	if s.checked >= iter {
		return s.correct, nil
	}
	s.checked = iter
	if n.declaration == nil {
		return false, c.unported("level.precondition.new-symbol-declaration")
	}
	declOK, e := c.check(n.declaration, iter)
	if e != nil {
		return false, e
	}
	d, e := c.data(n.declaration)
	if e != nil {
		return false, e
	}
	s.level = d.level
	if n.set != nil {
		ok, e := c.check(n.set, iter)
		if e != nil {
			return false, e
		}
		s.correct = ok
		b, e := c.data(n.set)
		if e != nil {
			return false, e
		}
		if b.level > s.level {
			s.level = b.level
		}
		if s.level == 3 {
			s.correct = false
			loc, e := levelTreeLocation(&n.semanticHeader)
			if e != nil {
				return false, e
			}
			if e = addError(c.errors, 4356, loc, "Level error:\nTemporal formula used as set."); e != nil {
				return false, e
			}
		}
		s.params = b.params
		s.all = b.all
		s.upper = b.upper
		s.lower = b.lower
		s.triples = b.triples
	}
	s.correct = s.correct && declOK
	return s.correct, nil
}
func (c *levelChecker) checkAssumeProve(n *assumeProveNode, s *levelState, iter int) (bool, error) {
	if s.checked >= iter {
		return s.correct, nil
	}
	s.checked = iter
	s.correct = true
	for _, a := range n.assumptions {
		if a != nil {
			ok, e := c.check(a, iter)
			if e != nil {
				return false, e
			}
			s.correct = s.correct && ok
		}
	}
	if n.prove == nil {
		return false, c.unported("level.precondition.assume-prove-shape")
	}
	if _, e := c.check(n.prove, iter); e != nil {
		return false, e
	}
	p, e := c.data(n.prove)
	if e != nil {
		return false, e
	}
	s.level = p.level
	for _, a := range n.assumptions {
		if a == nil {
			return false, c.unported("level.precondition.assume-prove-shape")
		}
		if _, e = c.check(a, iter); e != nil {
			return false, e
		}
		as, e := c.data(a)
		if e != nil {
			return false, e
		}
		if as.level > s.level {
			s.level = as.level
		}
	}
	if e = c.merge(s, p); e != nil {
		return false, e
	}
	for _, a := range n.assumptions {
		as, e := c.data(a)
		if e != nil {
			return false, e
		}
		if e = c.merge(s, as); e != nil {
			return false, e
		}
	}
	if s.correct {
		if e = c.temporalConstants(s); e != nil {
			return false, e
		}
	}
	return s.correct, nil
}
func (c *levelChecker) checkAssume(n *assumeNode, s *levelState, iter int) (bool, error) {
	if s.checked >= iter {
		return true, nil
	}
	s.checked = iter
	if n.expression == nil {
		return false, c.unported("level.precondition.assume-expression")
	}
	ok, e := c.check(n.expression, iter)
	if e != nil {
		return false, e
	}
	if n.definition != nil {
		d, e := c.check(n.definition, iter)
		if e != nil {
			return false, e
		}
		ok = d && ok
	}
	body, e := c.data(n.expression)
	if e != nil {
		return false, e
	}
	if body.level != 0 {
		loc, e := levelTreeLocation(&n.semanticHeader)
		if e != nil {
			return false, e
		}
		if e = addError(c.errors, 4206, loc, "Level error: assumptions must be level 0 (Constant), \nbut this one has level %s.", Parameter{Kind: IntParameter, Integer: int64(body.level)}); e != nil {
			return false, e
		}
	}
	if ok {
		if e = c.temporalConstants(s); e != nil {
			return false, e
		}
	}
	return ok, nil
}

// levelCheckSubnodes skips modules and module-instance markers, but checks
// ordinary instances after proof steps. Definitions on BY/USE are not inputs.
func levelNodesAsAny(nodes []levelNode) []any {
	out := make([]any, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n)
	}
	return out
}

func (c *levelChecker) checkProofSubnodes(s *levelState, iter int, subs []any) (bool, error) {
	if s.checked >= iter {
		return s.correct, nil
	}
	s.checked = iter
	for _, sub := range subs {
		if _, ok := sub.(*moduleNode); ok {
			continue
		}
		if def, ok := sub.(*userOpDefNode); ok && def != nil && def.kind == 6 {
			continue
		}
		ok, err := c.check(sub, iter)
		if err != nil {
			return false, err
		}
		s.correct = ok && s.correct
		child, err := c.data(sub)
		if err != nil {
			return false, err
		}
		if child.level > s.level {
			s.level = child.level
		}
		if err = c.merge(s, child); err != nil {
			return false, err
		}
		if err = s.nonLeibniz.PutAll(child.nonLeibniz); err != nil {
			return false, err
		}
	}
	return s.correct, nil
}

// temporalProof follows only CASE and QED subproofs, as in TheoremNode.
func (c *levelChecker) temporalProof(proof levelNode) error {
	if err := c.step(); err != nil {
		return err
	}
	if c.depth >= c.depthLimit {
		return c.unported("level.resource.depth-limit")
	}
	c.depth++
	defer func() { c.depth-- }()
	p, ok := proof.(*nonLeafProofNode)
	if !ok {
		return nil
	}
	if p == nil {
		return c.unported("level.precondition.proof-shape")
	}
	for _, step := range p.steps {
		if err := c.step(); err != nil {
			return err
		}
		t, ok := step.(*theoremNode)
		if !ok {
			continue
		}
		if t == nil {
			return c.unported("level.precondition.proof-shape")
		}
		app, ok := t.body.(*opApplNode)
		if !ok {
			continue
		}
		if app == nil || app.operator == nil || app.operator.header().name == nil {
			return c.unported("level.precondition.proof-shape")
		}
		name := app.operator.header().name.text
		if name != "$Take" && name != "$Witness" && name != "$Have" && name != "$Pfcase" && name != "$Qed" {
			continue
		}
		if name != "$Qed" {
			state, err := c.data(app)
			if err != nil {
				return err
			}
			if state.level != 0 {
				loc, err := levelTreeLocation(&app.semanticHeader)
				if err != nil {
					return err
				}
				code, message := 4352, "Non-constant TAKE, WITNESS, or HAVE for temporal goal."
				if name == "$Pfcase" {
					code, message = 4353, "Non-constant CASE for temporal goal."
				}
				if err = addError(c.errors, code, loc, message); err != nil {
					return err
				}
			}
		}
		if name == "$Pfcase" || name == "$Qed" {
			if err := c.temporalProof(t.proof); err != nil {
				return err
			}
		}
	}
	return nil
}

// TheoremNode has a shadow checked counter in addition to the LevelNode
// aggregation cache.
func (c *levelChecker) checkTheorem(n *theoremNode, s *levelState, iter int) (bool, error) {
	if s.shadow >= iter {
		return true, nil
	}
	s.shadow = iter
	if s.checked >= iter {
		return s.correct, nil
	}
	s.checked = iter
	var subs []any
	if n.definition != nil {
		subs = append(subs, n.definition)
	} else {
		if n.body == nil {
			return false, c.unported("level.precondition.theorem-body")
		}
		subs = append(subs, n.body)
	}
	if n.proof != nil {
		subs = append(subs, n.proof)
	}
	for _, sub := range subs {
		ok, e := c.check(sub, iter)
		if e != nil {
			return false, e
		}
		s.correct = ok && s.correct
		child, e := c.data(sub)
		if e != nil {
			return false, e
		}
		if child.level > s.level {
			s.level = child.level
		}
		if e = c.merge(s, child); e != nil {
			return false, e
		}
		if e = s.nonLeibniz.PutAll(child.nonLeibniz); e != nil {
			return false, e
		}
	}
	// PICK uses the body's raw level field and emits diagnostics without
	// changing the return value.
	if app, ok := n.body.(*opApplNode); ok && app.operator != nil && app.operator.header().name != nil && app.operator.header().name.text == "$Pick" {
		raw := c.states[app]
		if raw == nil {
			return false, c.unported("level.precondition.pick-state")
		}
		if raw.level == 3 {
			for _, r := range app.ranges {
				v, e := c.data(r)
				if e != nil {
					return false, e
				}
				if v.level != 0 {
					loc, e := levelTreeLocation(r.levelHeader())
					if e != nil {
						return false, e
					}
					if e = addError(c.errors, 4354, loc, "Non-constant bound of temporal PICK."); e != nil {
						return false, e
					}
				}
			}
		}
	}
	if n.body != nil {
		raw := c.states[n.body]
		if raw != nil && raw.level == 3 {
			if e := c.temporalProof(n.proof); e != nil {
				return false, e
			}
		}
	}
	return s.correct, nil
}

// subFor uses Java reference identity and the first matching substitution.
func (c *levelChecker) subFor(symbol levelSymbol, subs []*subst) (exprOrOpArg, error) {
	for _, sub := range subs {
		if sub == nil || sub.op == nil || sub.expr == nil {
			return nil, c.unported("level.precondition.substitution-shape")
		}
		k, e := c.sym(sub.op)
		if e != nil {
			return nil, e
		}
		if symbolEq(k, symbol) {
			return sub.expr, nil
		}
	}
	return nil, nil
}
func (c *levelChecker) substitutedParams(symbol levelSymbol, subs []*subst, all bool) (*levelSet, error) {
	sub, e := c.subFor(symbol, subs)
	if e != nil {
		return nil, e
	}
	if sub != nil {
		s, e := c.data(sub)
		if e != nil {
			return nil, e
		}
		if all {
			return s.all, nil
		}
		return s.params, nil
	}
	set, e := levelcollections.NewMap[levelSymbol, struct{}](c.owner, symbolEq)
	if e != nil {
		return nil, e
	}
	if e = set.Put(symbol, symbol.hash, struct{}{}); e != nil {
		return nil, e
	}
	return set, nil
}
func levelIsParam(n leafNode) bool {
	switch n.(type) {
	case *formalParamNode, *opDeclNode:
		return true
	}
	return false
}
func (c *levelChecker) substituteConstraints(dst, body *levelState, subs []*subst, isConstant bool, iter int) error {
	var e error
	dst.upper, e = levelcollections.NewMap[levelSymbol, int](c.owner, symbolEq)
	if e != nil {
		return e
	}
	if e = body.upper.Each(func(p levelSymbol, v int, _ uint32) error {
		if !isConstant {
			if p.node.header().kind == 2 {
				v = 0
			} else if p.node.header().kind == 3 {
				v = 1
			}
		}
		values, e := c.substitutedParams(p, subs, false)
		if e != nil {
			return e
		}
		return values.Each(func(q levelSymbol, _ struct{}, _ uint32) error { return c.putUpper(dst, q, v) })
	}); e != nil {
		return e
	}
	if e = body.triples.Each(func(t levelTriple, _ struct{}, _ uint32) error {
		x, e := c.subFor(t.op, subs)
		if e != nil || x == nil {
			return e
		}
		arg, ok := x.(*opArgNode)
		if !ok {
			return c.unported("level.precondition.substitution-op-arg")
		}
		switch arg.operator.(type) {
		case *userOpDefNode, *builtinOpDefNode:
		default:
			return nil
		}
		if _, e = c.check(arg.operator, iter); e != nil {
			return e
		}
		op, e := c.data(arg.operator)
		if e != nil {
			return e
		}
		i := t.pos
		if nodeArity(arg.operator) == -1 {
			i = 0
		}
		if i < 0 || i >= len(op.maxLevels) {
			return c.unported("level.precondition.substitution-index")
		}
		values, e := c.substitutedParams(t.param, subs, false)
		if e != nil {
			return e
		}
		return values.Each(func(q levelSymbol, _ struct{}, _ uint32) error { return c.putUpper(dst, q, op.maxLevels[i]) })
	}); e != nil {
		return e
	}
	dst.lower, e = levelcollections.NewMap[levelPair, int](c.owner, pairEq)
	if e != nil {
		return e
	}
	if e = body.lower.Each(func(p levelPair, v int, _ uint32) error {
		x, e := c.subFor(p.symbol, subs)
		if e != nil {
			return e
		}
		if x == nil {
			return c.putLower(dst, p, v)
		}
		arg, ok := x.(*opArgNode)
		if !ok {
			return c.unported("level.precondition.substitution-op-arg")
		}
		if !levelIsParam(arg.operator) {
			return nil
		}
		sym, e := c.sym(arg.operator)
		if e != nil {
			return e
		}
		return c.putLower(dst, levelPair{sym, p.pos}, v)
	}); e != nil {
		return e
	}
	if e = body.triples.Each(func(t levelTriple, _ struct{}, _ uint32) error {
		param, e := c.subFor(t.param, subs)
		if e != nil || param == nil {
			return e
		}
		opSub, e := c.subFor(t.op, subs)
		if e != nil {
			return e
		}
		op := t.op
		if opSub != nil {
			arg, ok := opSub.(*opArgNode)
			if !ok {
				return c.unported("level.precondition.substitution-op-arg")
			}
			op, e = c.sym(arg.operator)
			if e != nil {
				return e
			}
		}
		if !levelIsParam(op.node) {
			return nil
		}
		if _, e = c.check(param, iter); e != nil {
			return e
		}
		p, e := c.data(param)
		if e != nil {
			return e
		}
		return c.putLower(dst, levelPair{op, t.pos}, p.level)
	}); e != nil {
		return e
	}
	dst.triples, e = levelcollections.NewMap[levelTriple, struct{}](c.owner, tripleEq)
	if e != nil {
		return e
	}
	return body.triples.Each(func(t levelTriple, _ struct{}, _ uint32) error {
		x, e := c.subFor(t.op, subs)
		if e != nil {
			return e
		}
		if x == nil {
			return dst.triples.Put(t, t.op.hash+uint32(t.pos)+t.param.hash, struct{}{})
		}
		arg, ok := x.(*opArgNode)
		if !ok {
			return c.unported("level.precondition.substitution-op-arg")
		}
		if !levelIsParam(arg.operator) {
			return nil
		}
		op, e := c.sym(arg.operator)
		if e != nil {
			return e
		}
		values, e := c.substitutedParams(t.param, subs, false)
		if e != nil {
			return e
		}
		return values.Each(func(p levelSymbol, _ struct{}, _ uint32) error {
			z := levelTriple{op, t.pos, p}
			return dst.triples.Put(z, op.hash+uint32(t.pos)+p.hash, struct{}{})
		})
	})
}

func (c *levelChecker) checkSubstitution(body any, subs []*subst, target *moduleNode, s *levelState, iter int, ap bool) (bool, error) {
	if s.checked >= iter {
		return s.correct, nil
	}
	s.checked = iter
	s.correct = true
	ok, e := c.check(body, iter)
	if e != nil {
		return false, e
	}
	s.correct = ok
	for _, sub := range subs {
		if sub == nil || sub.op == nil || sub.expr == nil {
			return false, c.unported("level.precondition.substitution-shape")
		}
		ok, e = c.check(sub.expr, iter)
		if e != nil {
			return false, e
		}
		s.correct = s.correct && ok
	}
	b, e := c.data(body)
	if e != nil {
		return false, e
	}
	s.level = b.level
	for _, sub := range subs {
		sym, e := c.sym(sub.op)
		if e != nil {
			return false, e
		}
		_, present, e := b.params.Get(sym, sym.hash)
		if e != nil {
			return false, e
		}
		if present {
			x, e := c.data(sub.expr)
			if e != nil {
				return false, e
			}
			if x.level > s.level {
				s.level = x.level
			}
		}
	}
	if e = b.params.Each(func(p levelSymbol, _ struct{}, _ uint32) error {
		v, e := c.substitutedParams(p, subs, false)
		if e != nil {
			return e
		}
		return s.params.PutAll(v)
	}); e != nil {
		return false, e
	}
	if ap {
		if e = b.all.Each(func(p levelSymbol, _ struct{}, _ uint32) error {
			v, e := c.substitutedParams(p, subs, true)
			if e != nil {
				return e
			}
			return s.all.PutAll(v)
		}); e != nil {
			return false, e
		}
	} else {
		s.all, e = levelcollections.CopySetMap(c.owner, b.all)
		if e != nil {
			return false, e
		}
		s.nonLeibniz, e = levelcollections.CopySetMap(c.owner, b.nonLeibniz)
		if e != nil {
			return false, e
		}
		for _, sub := range subs {
			sym, e := c.sym(sub.op)
			if e != nil {
				return false, e
			}
			_, found, e := s.all.Get(sym, sym.hash)
			if e != nil {
				return false, e
			}
			if found {
				if _, e = s.all.Remove(sym, sym.hash); e != nil {
					return false, e
				}
				v, e := c.data(sub.expr)
				if e != nil {
					return false, e
				}
				if e = s.all.PutAll(v.all); e != nil {
					return false, e
				}
				if e = s.nonLeibniz.PutAll(v.nonLeibniz); e != nil {
					return false, e
				}
				removed, e := s.nonLeibniz.Remove(sym, sym.hash)
				if e != nil {
					return false, e
				}
				if removed {
					if e = s.nonLeibniz.PutAll(v.all); e != nil {
						return false, e
					}
				}
			}
		}
	}
	constant, e := c.moduleConstant(target)
	if e != nil {
		return false, e
	}
	if e = c.substituteConstraints(s, b, subs, constant, iter); e != nil {
		return false, e
	}
	return s.correct, nil
}

func (c *levelChecker) moduleConstant(m *moduleNode) (bool, error) {
	if m == nil || m.context == nil {
		return false, c.unported("level.precondition.module-shape")
	}
	if m.hasDeclarations(3) {
		return false, nil
	}
	if _, e := c.check(m, 1); e != nil {
		return false, e
	}
	for _, d := range m.opDefs() {
		if d.kind != 6 {
			if d.body == nil {
				return false, c.unported("level.precondition.module-shape")
			}
			b, e := c.data(d.body)
			if e != nil {
				return false, e
			}
			if b.level != 0 {
				return false, nil
			}
		}
	}
	for _, t := range m.theorems {
		v, e := c.data(t)
		if e != nil {
			return false, e
		}
		if v.level != 0 {
			return false, nil
		}
	}
	return true, nil
}
func (c *levelChecker) checkModule(m *moduleNode, s *levelState, iter int) (bool, error) {
	if s.checked >= iter {
		return s.correct, nil
	}
	s.checked = iter
	if m.context == nil {
		return false, c.unported("level.precondition.module-shape")
	}
	if e := c.checkRecursiveSections(m.recursiveSectionDefs, c.errors); e != nil {
		return false, e
	}
	s.correct = true
	for _, n := range m.context.orderedValues() {
		if inner, ok := n.(*moduleNode); ok {
			v, e := c.check(inner, 1)
			if e != nil {
				return false, e
			}
			s.correct = s.correct && v
		}
	}
	defs := m.opDefs()
	for _, d := range defs {
		v, e := c.check(d, 1)
		if e != nil {
			return false, e
		}
		s.correct = s.correct && v
	}
	var thms []*thmOrAssumpDefNode
	for p := m.context.last; p != nil; p = p.prior {
		if d, ok := p.value.(*thmOrAssumpDefNode); ok {
			thms = append(thms, d)
		}
	}
	for i := len(thms) - 1; i >= 0; i-- {
		v, e := c.check(thms[i], 1)
		if e != nil {
			return false, e
		}
		s.correct = s.correct && v
	}
	for _, n := range m.topLevel {
		if n == nil {
			return false, c.unported("level.precondition.module-shape")
		}
		v, e := c.check(n, 1)
		if e != nil {
			return false, e
		}
		s.correct = s.correct && v
	}
	m.hasDeclarations(2)
	for _, n := range m.constantCache {
		sym, e := c.sym(n)
		if e != nil {
			return false, e
		}
		if e = s.params.Put(sym, sym.hash, struct{}{}); e != nil {
			return false, e
		}
		if e = s.all.Put(sym, sym.hash, struct{}{}); e != nil {
			return false, e
		}
	}
	constant, e := c.moduleConstant(m)
	if e != nil {
		return false, e
	}
	if !constant {
		for _, n := range m.constantCache {
			sym, e := c.sym(n)
			if e != nil {
				return false, e
			}
			if e = c.putUpper(s, sym, 0); e != nil {
				return false, e
			}
		}
	}
	for _, d := range defs {
		v, e := c.data(d)
		if e != nil {
			return false, e
		}
		if e = c.mergeLetDefinition(s, v, d); e != nil {
			return false, e
		}
	}
	for _, n := range m.topLevel {
		v, e := c.data(n)
		if e != nil {
			return false, e
		}
		if e = c.mergeConstraints(s, v); e != nil {
			return false, e
		}
	}
	return s.correct, nil
}

func (c *levelChecker) instanceError(n *instanceNode, code int, format string, params ...Parameter) error {
	loc, e := levelTreeLocation(&n.semanticHeader)
	if e != nil {
		return e
	}
	return addError(c.errors, code, loc, format, params...)
}
func (c *levelChecker) checkInstance(n *instanceNode, s *levelState, iter int) (bool, error) {
	if s.checked >= iter {
		return s.correct, nil
	}
	s.checked = iter
	var e error
	s.params, e = levelcollections.NewMap[levelSymbol, struct{}](c.owner, symbolEq)
	if e != nil {
		return false, e
	}
	s.correct = true
	if n.module == nil || n.module.name == nil {
		return false, c.unported("level.precondition.instance-shape")
	}
	ok, e := c.check(n.module, iter)
	if e != nil {
		return false, e
	}
	s.correct = ok
	for _, sub := range n.substs {
		if sub == nil || sub.op == nil || sub.op.name == nil || sub.expr == nil {
			return false, c.unported("level.precondition.instance-shape")
		}
		ok, e = c.check(sub.expr, iter)
		if e != nil {
			return false, e
		}
		s.correct = s.correct && ok
	}
	const levelFmt = "Level error in instantiating module '%s':\nThe level of the expression or operator substituted for '%s' \nmust be at most %s."
	for _, sub := range n.substs {
		exprOK, e := c.check(sub.expr, iter)
		if e != nil {
			return false, e
		}
		declOK, e := c.check(sub.op, iter)
		if e != nil {
			return false, e
		}
		expr, e := c.data(sub.expr)
		if e != nil {
			return false, e
		}
		decl, e := c.data(sub.op)
		if e != nil {
			return false, e
		}
		constant, e := c.moduleConstant(n.module)
		if e != nil {
			return false, e
		}
		if !constant && expr.level > decl.level {
			if exprOK && declOK {
				if e = c.instanceError(n, 4245, levelFmt, nameParam(n.module.name), nameParam(sub.op.name), Parameter{Kind: IntParameter, Integer: int64(decl.level)}); e != nil {
					return false, e
				}
			}
			s.correct = false
		}
		if arg, ok := sub.expr.(*opArgNode); ok {
			switch arg.operator.(type) {
			case *userOpDefNode, *builtinOpDefNode:
				op, e := c.data(arg.operator)
				if e != nil {
					return false, e
				}
				if !op.isLeibniz {
					if e = c.instanceError(n, 4244, "Error in instantiating module '%s':\n A non-Leibniz operator substituted for '%s'.", nameParam(n.module.name), nameParam(sub.op.name)); e != nil {
						return false, e
					}
				}
			}
		}
	}
	mod := c.states[n.module]
	if mod == nil {
		return false, c.unported("level.precondition.instance-module-state")
	}
	for _, sub := range n.substs {
		sym, e := c.sym(sub.op)
		if e != nil {
			return false, e
		}
		expr, e := c.data(sub.expr)
		if e != nil {
			return false, e
		}
		upper, found, e := mod.upper.Get(sym, sym.hash)
		if e != nil {
			return false, e
		}
		if found && expr.level > upper {
			ok, e := c.check(sub.expr, iter)
			if e != nil {
				return false, e
			}
			if ok {
				if e = c.instanceError(n, 4245, levelFmt, nameParam(n.module.name), nameParam(sub.op.name), Parameter{Kind: IntParameter, Integer: int64(upper)}); e != nil {
					return false, e
				}
			}
			s.correct = false
		}
		if sub.op.arity > 0 {
			arg, ok := sub.expr.(*opArgNode)
			if !ok {
				return false, c.unported("level.precondition.substitution-op-arg")
			}
			switch arg.operator.(type) {
			case *userOpDefNode, *builtinOpDefNode:
				for j := 0; j < sub.op.arity; j++ {
					key := levelPair{sym, j}
					required, has, e := mod.lower.Get(key, sym.hash+uint32(j))
					if e != nil {
						return false, e
					}
					opOK, e := c.check(arg.operator, iter)
					if e != nil {
						return false, e
					}
					op, e := c.data(arg.operator)
					if e != nil {
						return false, e
					}
					idx := j
					if nodeArity(arg.operator) == -1 {
						idx = 0
					}
					if idx >= len(op.maxLevels) {
						return false, c.unported("level.precondition.substitution-index")
					}
					if has && op.maxLevels[idx] < required {
						if opOK {
							if arg.operator.header().name == nil {
								return false, c.unported("level.precondition.unnamed-operator")
							}
							if e = c.instanceError(n, 4246, "Level error in instantiating module '%s':\nThe level of the argument %s of the operator %s \nmust be at least %s.", nameParam(n.module.name), Parameter{Kind: IntParameter, Integer: int64(j + 1)}, nameParam(arg.operator.header().name), Parameter{Kind: IntParameter, Integer: int64(required)}); e != nil {
								return false, e
							}
						}
						s.correct = false
					}
				}
			}
		}
	}
	if e = mod.triples.Each(func(t levelTriple, _ struct{}, _ uint32) error {
		for _, left := range n.substs {
			ls, e := c.sym(left.op)
			if e != nil {
				return e
			}
			for _, right := range n.substs {
				rs, e := c.sym(right.op)
				if e != nil {
					return e
				}
				if !symbolEq(t.op, ls) || !symbolEq(t.param, rs) {
					continue
				}
				arg, ok := left.expr.(*opArgNode)
				if !ok {
					return c.unported("level.precondition.substitution-op-arg")
				}
				opOK, e := c.check(arg.operator, iter)
				if e != nil {
					return e
				}
				switch arg.operator.(type) {
				case *userOpDefNode, *builtinOpDefNode:
					op, e := c.data(arg.operator)
					if e != nil {
						return e
					}
					i := t.pos
					if nodeArity(arg.operator) == -1 {
						i = 0
					}
					if i < 0 || i >= len(op.maxLevels) {
						return c.unported("level.precondition.substitution-index")
					}
					value, e := c.data(right.expr)
					if e != nil {
						return e
					}
					if value.level > op.maxLevels[i] {
						rightOK, e := c.check(right.expr, iter)
						if e != nil {
							return e
						}
						if opOK && rightOK {
							if e = c.instanceError(n, 4247, "Level error when instantiating module '%s':\nThe level of the argument %s of the operator %s' \nmust be at most %s.", nameParam(n.module.name), Parameter{Kind: IntParameter, Integer: int64(t.pos)}, nameParam(left.op.name), Parameter{Kind: IntParameter, Integer: int64(op.maxLevels[i])}); e != nil {
								return e
							}
						}
						s.correct = false
					}
				}
			}
		}
		return nil
	}); e != nil {
		return false, e
	}
	constant, e := c.moduleConstant(n.module)
	if e != nil {
		return false, e
	}
	temp := &levelState{}
	if e = c.substituteConstraints(temp, mod, n.substs, constant, iter); e != nil {
		return false, e
	}
	formals := make([]levelSymbol, 0, len(n.params))
	for _, p := range n.params {
		sym, e := c.sym(p)
		if e != nil {
			return false, e
		}
		formals = append(formals, sym)
	}
	contains := func(x levelSymbol) bool {
		for _, p := range formals {
			if symbolEq(p, x) {
				return true
			}
		}
		return false
	}
	upper := func(src *levelState) error {
		return src.upper.Each(func(k levelSymbol, v int, _ uint32) error {
			if contains(k) {
				return nil
			}
			return c.putUpper(s, k, v)
		})
	}
	lower := func(src *levelState) error {
		return src.lower.Each(func(k levelPair, v int, _ uint32) error {
			if contains(k.symbol) {
				return nil
			}
			return c.putLower(s, k, v)
		})
	}
	triples := func(src *levelState) error {
		return src.triples.Each(func(t levelTriple, _ struct{}, _ uint32) error {
			if contains(t.op) || contains(t.param) {
				return nil
			}
			return s.triples.Put(t, t.op.hash+uint32(t.pos)+t.param.hash, struct{}{})
		})
	}
	if e = upper(temp); e != nil {
		return false, e
	}
	for _, sub := range n.substs {
		x, e := c.data(sub.expr)
		if e != nil {
			return false, e
		}
		if e = upper(x); e != nil {
			return false, e
		}
	}
	if e = lower(temp); e != nil {
		return false, e
	}
	for _, sub := range n.substs {
		x, e := c.data(sub.expr)
		if e != nil {
			return false, e
		}
		if e = lower(x); e != nil {
			return false, e
		}
	}
	if e = triples(temp); e != nil {
		return false, e
	}
	for _, sub := range n.substs {
		x, e := c.data(sub.expr)
		if e != nil {
			return false, e
		}
		if e = triples(x); e != nil {
			return false, e
		}
	}
	return s.correct, nil
}

func (c *levelChecker) specialTemporal(n *opApplNode, s *levelState) (bool, error) {
	if n.operator == nil || n.operator.header().name == nil {
		return false, c.unported("level.precondition.unnamed-operator")
	}
	name := n.operator.header().name.text
	value := func(i int) (*levelState, error) {
		if i < 0 || i >= len(n.operands) || n.operands[i] == nil {
			return nil, c.unported("level.precondition.temporal-operand")
		}
		return c.data(n.operands[i])
	}
	issue := func(code int, msg string, params ...Parameter) error {
		loc, e := levelTreeLocation(&n.semanticHeader)
		if e != nil {
			return e
		}
		s.correct = false
		return addError(c.errors, code, loc, msg, params...)
	}
	if name == "[]" || name == "<>" {
		a, e := value(0)
		if e != nil {
			return false, e
		}
		if a.level == 2 {
			if app, ok := n.operands[0].(*opApplNode); ok {
				if app.operator == nil || app.operator.header().name == nil {
					return false, c.unported("level.precondition.unnamed-operator")
				}
				expected := "$SquareAct"
				code := 4310
				message := "[] followed by action not of form [A]_v."
				if name == "<>" {
					expected = "$AngleAct"
					code = 4311
					message = "<> followed by action not of form <<A>>_v."
				}
				if app.operator.header().name.text != expected {
					if e = issue(code, message); e != nil {
						return false, e
					}
				}
			}
		}
	}
	if name == "~>" || name == "-+->" {
		a, e := value(0)
		if e != nil {
			return false, e
		}
		b, e := value(1)
		if e != nil {
			return false, e
		}
		if a.level == 2 || b.level == 2 {
			if e = issue(4312, "Action used where only temporal formula or state predicate allowed."); e != nil {
				return false, e
			}
		}
	}
	switch name {
	case "\\land", "\\lor", "=>", "\\equiv", "$ConjList", "$DisjList":
		action, temporal := false, false
		for i := range n.operands {
			v, e := value(i)
			if e != nil {
				return false, e
			}
			action = action || v.level == 2
			temporal = temporal || v.level == 3
		}
		if action && temporal {
			pop := name
			if pop == "$ConjList" {
				pop = "Conjunction list"
			}
			if pop == "$DisjList" {
				pop = "Disjunction list"
			}
			if e := issue(4313, "%s has both temporal formula and action as arguments.", strParam(pop)); e != nil {
				return false, e
			}
		}
	}
	if s.level == 3 && (name == "$BoundedExists" || name == "$BoundedForall") {
		for _, r := range n.ranges {
			v, e := c.data(r)
			if e != nil {
				return false, e
			}
			if v.level == 2 {
				loc, e := levelTreeLocation(r.levelHeader())
				if e != nil {
					return false, e
				}
				if e = addError(c.errors, 4314, loc, "Action-level bound of quantified temporal formula."); e != nil {
					return false, e
				}
				s.correct = false
			}
		}
	}
	return s.correct, nil
}

func levelMin(s *levelState, i, j int) (int, error) {
	if s.minMax == nil {
		return 0, nil
	}
	if i < 0 || i >= len(s.minMax) || j < 0 || j >= len(s.minMax[i]) {
		return 0, ErrPortIncomplete
	}
	return s.minMax[i][j], nil
}
func levelCond(s *levelState, i, j, k int) (bool, error) {
	if s.conditions == nil {
		return false, nil
	}
	if i < 0 || i >= len(s.conditions) || j < 0 || j >= len(s.conditions[i]) || k < 0 || k >= len(s.conditions[i][j]) {
		return false, ErrPortIncomplete
	}
	return s.conditions[i][j][k], nil
}
func levelMax(s *levelState, n leafNode, i int) (int, error) {
	if nodeArity(n) == -1 {
		i = 0
	}
	if i < 0 || i >= len(s.maxLevels) {
		return 0, ErrPortIncomplete
	}
	return s.maxLevels[i], nil
}

func (c *levelChecker) definitionArgument(n *opApplNode, target levelSymbol) (exprOrOpArg, error) {
	var params []*formalParamNode
	switch d := n.operator.(type) {
	case *userOpDefNode:
		params = d.params
	case *builtinOpDefNode:
		params = d.params
	case *thmOrAssumpDefNode:
		params = d.params
	default:
		return nil, c.unported("level.operation.definition-argument-operator")
	}
	for i, arg := range n.operands {
		if i >= len(params) || params[i] == nil {
			return nil, c.unported("level.precondition.definition-argument-index")
		}
		sym, e := c.sym(params[i])
		if e != nil {
			return nil, e
		}
		if symbolEq(sym, target) {
			return arg, nil
		}
	}
	return nil, nil
}
