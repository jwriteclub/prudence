// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Derived from TLA+ OperatorStack.java and OSelement.java, Copyright (c) 2003
// Compaq Corporation, distributed under the MIT license. See NOTICE.
package generated

import (
	"errors"
	"fmt"
	"unicode/utf16"

	"github.com/jwriteclub/prudence/internal/lexer"
	"github.com/jwriteclub/prudence/internal/parserrt"
)

type OSelement struct {
	node *SyntaxTreeNode
	op   *Operator
}

func (e *OSelement) Node() *SyntaxTreeNode { return e.node }
func (e *OSelement) Operator() *Operator   { return e.op }
func (e *OSelement) IsOperator() bool      { return e.op != nil }

type OperatorStack struct {
	context *SupportContext
	stacks  [][]*OSelement
	errors  *ParseErrors
	Check   func() error
}

func (ctx *SupportContext) NewOperatorStack(e *ParseErrors) *OperatorStack {
	return &OperatorStack{context: ctx, errors: e}
}
func (s *OperatorStack) NewStack() { s.stacks = append(s.stacks, []*OSelement{}) }
func (s *OperatorStack) current() ([]*OSelement, error) {
	if len(s.stacks) == 0 {
		return nil, errors.New("sanyparser: no current operator stack")
	}
	return s.stacks[len(s.stacks)-1], nil
}
func (s *OperatorStack) set(v []*OSelement) { s.stacks[len(s.stacks)-1] = v }
func (s *OperatorStack) PopStack() error {
	if len(s.stacks) == 0 {
		return errors.New("sanyparser: no current operator stack")
	}
	s.stacks = s.stacks[:len(s.stacks)-1]
	return nil
}
func (s *OperatorStack) Size() (int, error)   { v, e := s.current(); return len(v), e }
func (s *OperatorStack) Empty() (bool, error) { n, e := s.Size(); return n == 0, e }
func (s *OperatorStack) PreInEmptyTop() bool {
	if len(s.stacks) == 0 {
		return true
	}
	v := s.stacks[len(s.stacks)-1]
	if len(v) == 0 {
		return true
	}
	o := v[len(v)-1].op
	return o != nil && (o.IsPrefix() || o.IsInfix())
}
func (s *OperatorStack) PushOnStack(n *SyntaxTreeNode, o *Operator) error {
	v, e := s.current()
	if e != nil {
		return e
	}
	s.set(append(v, &OSelement{n, o}))
	return nil
}
func (s *OperatorStack) TopOfStack() *OSelement {
	if len(s.stacks) == 0 {
		return nil
	}
	v := s.stacks[len(s.stacks)-1]
	if len(v) == 0 {
		return nil
	}
	return v[len(v)-1]
}
func (s *OperatorStack) PopCurrentTop() error {
	v, e := s.current()
	if e != nil {
		return e
	}
	if len(v) == 0 {
		return errors.New("sanyparser: empty operator stack")
	}
	s.set(v[:len(v)-1])
	return nil
}
func (s *OperatorStack) BottomOfStack() (*SyntaxTreeNode, error) {
	v, e := s.current()
	if e != nil {
		return nil, e
	}
	if len(v) == 0 {
		return nil, errors.New("sanyparser: empty operator stack")
	}
	return v[0].node, nil
}
func (s *OperatorStack) IsWellReduced() (bool, error) { n, e := s.Size(); return n == 2, e }

func reject(msg string) error { return parserrt.NewBindingRejection(msg, lexer.Position{}, nil) }

// rejectAt reports msg at the first block its text cites; SANY's
// ParseException has no position of its own, so editors use the cited location.
func rejectAt(at *SyntaxTreeNode, msg string) error {
	return parserrt.NewBindingRejection(msg, nodePosition(at), nil)
}
func loc(n *SyntaxTreeNode) string { return n.Location().String() }
func id(o *Operator) string        { return o.Identifier().String() }
func (s *OperatorStack) reduceInfix(o *Operator) error {
	v, _ := s.current()
	n := len(v) - 1
	if n < 3 {
		return nil
	}
	a, b, c := v[n-3].node, v[n-2].node, v[n-1].node
	if a == nil || b == nil || c == nil {
		return errors.New("sanyparser: nil reduction node")
	}
	var x *SyntaxTreeNode
	var e error
	if o.IsNfix() && a.IsKind(N_Times) {
		ch := append(a.Heirs(), b, c)
		x, e = s.context.NewNodeInherit(N_Times, ch)
	} else if o.IsNfix() {
		x, e = s.context.NewNodeTriple(N_Times, a, b, c)
	} else {
		x, e = s.context.NewNodeTriple(N_InfixExpr, a, b, c)
	}
	if e != nil {
		return e
	}
	v[n-3] = &OSelement{node: x}
	s.set(append(v[:n-2], v[n:]...))
	return nil
}
func (s *OperatorStack) reducePrefix() error {
	v, _ := s.current()
	n := len(v) - 1
	if n < 2 {
		return nil
	}
	x, e := s.context.NewNodePair(N_PrefixExpr, v[n-2].node, v[n-1].node)
	if e != nil {
		return e
	}
	v[n-2] = &OSelement{node: x}
	s.set(append(v[:n-1], v[n:]...))
	return nil
}
func (s *OperatorStack) reducePostfix() error {
	v, _ := s.current()
	n := len(v) - 1
	if n < 2 {
		return nil
	}
	a, b := v[n-2].node, v[n-1].node
	if a == nil || b == nil {
		return errors.New("sanyparser: nil reduction node")
	}
	f, _ := s.context.LookupOperator(s.context.UniqueStringOf("["))
	var x *SyntaxTreeNode
	var e error
	if v[n-1].op == f {
		x, e = s.context.NewNodeOptional(a.Filename(), N_FcnAppl, a, b.Heirs())
	} else {
		x, e = s.context.NewNodePair(N_PostfixExpr, a, b)
	}
	if e != nil {
		return e
	}
	v[n-2] = &OSelement{node: x}
	s.set(append(v[:n-1], v[n:]...))
	return nil
}
func (s *OperatorStack) ReduceStack() error {
	for {
		if s.Check != nil {
			if e := s.Check(); e != nil {
				return e
			}
		}
		v, e := s.current()
		if e != nil {
			return e
		}
		n := len(v) - 1
		if n < 0 || v[n] == nil {
			return errors.New("sanyparser: invalid reduction stack")
		}
		r := v[n]
		if !r.IsOperator() {
			return nil
		}
		or := r.op
		before := len(v)
		if or.IsPostfix() {
			if n == 0 {
				if r.node == nil {
					return errors.New("sanyparser: nil location node")
				}
				return rejectAt(r.node, "\n  Encountered postfix op "+id(or)+" in block "+loc(r.node)+" on empty stack")
			}
			a := v[n-1]
			if a.IsOperator() {
				ol := a.op
				if ol.IsInfix() || ol.IsPrefix() {
					if r.node == nil {
						return errors.New("sanyparser: nil location node")
					}
					return rejectAt(r.node, "\n  Encountered postfix op "+id(or)+" in block "+loc(r.node)+" following prefix or infix op "+id(ol)+".")
				}
				e = s.reducePostfix()
			} else if n > 1 {
				b := v[n-2]
				if b.IsOperator() {
					ol := b.op
					succ, _ := Succ(ol, or)
					prec, _ := Prec(ol, or)
					if succ {
						if ol.IsInfix() {
							e = s.reduceInfix(ol)
						} else {
							e = s.reducePrefix()
						}
					} else if !prec {
						if r.node == nil {
							return errors.New("sanyparser: nil location node")
						}
						return rejectAt(r.node, "Precedence conflict between ops "+id(ol)+" in block "+loc(r.node)+" and "+id(or)+".")
					} else {
						return nil
					}
				} else {
					if a.node == nil || b.node == nil {
						return errors.New("sanyparser: nil location node")
					}
					return rejectAt(b.node, "Expression at location "+loc(b.node)+" and expression at location "+loc(a.node)+" follow each other without any intervening operator.")
				}
			} else {
				return nil
			}
		} else if or.IsPrefix() {
			if n == 0 {
				return nil
			}
			a := v[n-1]
			if a.IsOperator() {
				if a.op.IsPostfix() {
					if r.node == nil {
						return errors.New("sanyparser: nil location node")
					}
					return rejectAt(r.node, "\n  Encountered prefix op "+id(or)+" in block "+loc(r.node)+" following postfix op "+id(a.op)+".")
				}
				return nil
			}
			if r.node == nil {
				return errors.New("sanyparser: nil location node")
			}
			return rejectAt(r.node, "\n  Encountered prefix op "+id(or)+" in block "+loc(r.node)+" following an expression.")
		} else {
			if n == 0 {
				mix, _ := s.context.GetMixfix(or)
				if mix == nil {
					if r.node == nil {
						return errors.New("sanyparser: nil location node")
					}
					return rejectAt(r.node, "\n  Encountered infix op "+id(or)+" in block "+loc(r.node)+" on empty stack.")
				}
				return nil
			}
			a := v[n-1]
			if a.IsOperator() {
				ol := a.op
				if ol.IsInfix() || ol.IsPrefix() {
					mix, _ := s.context.GetMixfix(or)
					if mix == nil {
						if or == s.context.VoidOperator() {
							if a.node == nil {
								return errors.New("sanyparser: nil location node")
							}
							return rejectAt(a.node, "\n  Missing expression in block "+loc(a.node)+" following prefix or infix op "+id(ol)+".")
						}
						if a.node == nil {
							return errors.New("sanyparser: nil location node")
						}
						return rejectAt(a.node, "\n  Encountered infix op "+id(or)+" in block "+loc(a.node)+" following prefix or infix op "+id(ol)+".")
					}
					succ, _ := Succ(ol, mix)
					if succ || (ol == mix && ol.AssocLeft()) {
						if a.node == nil {
							return errors.New("sanyparser: nil location node")
						}
						return rejectAt(a.node, "\n  Precedence conflict between ops "+id(ol)+" in block "+loc(a.node)+" and "+id(mix)+".")
					}
					return nil
				}
				e = s.reducePostfix()
			} else if n > 1 {
				b := v[n-2]
				if !b.IsOperator() {
					if a.node == nil || b.node == nil {
						return errors.New("sanyparser: nil location node")
					}
					return rejectAt(b.node, "Expression at location "+loc(b.node)+" and expression at location "+loc(a.node)+" follow each other without any intervening operator.")
				}
				ol := b.op
				mix, _ := s.context.GetMixfix(ol)
				if mix != nil && (n == 2 || v[n-3].IsOperator()) {
					ol = mix
				}
				succ, _ := Succ(ol, or)
				prec, _ := Prec(ol, or)
				if succ || (ol == or && ol.AssocLeft()) {
					if ol.IsInfix() {
						e = s.reduceInfix(ol)
					} else if ol.IsPrefix() {
						e = s.reducePrefix()
					} else {
						if b.node == nil || a.node == nil {
							return errors.New("sanyparser: nil location node")
						}
						if b.node.Location().BeginLine() < a.node.Location().BeginLine() && or.Identifier() == s.context.UniqueStringOf("=") {
							if b.node == nil {
								return errors.New("sanyparser: nil location node")
							}
							return rejectAt(b.node, "\n  *** Hint *** You may have mistyped ==\n  Illegal combination of operators "+id(ol)+" in block "+loc(b.node)+" and "+id(or)+".")
						} else if or == s.context.VoidOperator() {
							if b.node == nil {
								return errors.New("sanyparser: nil location node")
							}
							return rejectAt(b.node, "\n Error following expression at  "+loc(b.node)+", missing operator or separator.")
						} else {
							if b.node == nil {
								return errors.New("sanyparser: nil location node")
							}
							return rejectAt(b.node, "\n  Illegal combination of operators "+id(ol)+" in block "+loc(b.node)+" and "+id(or)+".")
						}
					}
				} else if !(prec || (ol == or && ol.AssocRight())) {
					if b.node == nil {
						return errors.New("sanyparser: nil location node")
					}
					return rejectAt(b.node, "\n  Precedence conflict between ops "+id(ol)+" in block "+loc(b.node)+" and "+id(or)+".")
				} else {
					return nil
				}
			} else {
				return nil
			}
		}
		if e != nil {
			return e
		}
		v, _ = s.current()
		if len(v) == before {
			return nil
		}
	}
}
func (s *OperatorStack) FinalReduce() (*SyntaxTreeNode, error) {
	if e := s.PushOnStack(nil, s.context.VoidOperator()); e != nil {
		return nil, e
	}
	if e := s.ReduceStack(); e != nil {
		return nil, e
	}
	v, _ := s.current()
	if len(v) == 2 {
		return v[0].node, nil
	}
	if s.errors == nil {
		return nil, errors.New("sanyparser: nil parse errors")
	}
	if len(v) == 1 {
		return nil, errors.New("sanyparser: nil incomplete expression")
	}
	units := utf16.Encode([]rune("Couldn't properly parse expression"))
	for _, x := range v[:len(v)-1] {
		if x == nil || x.node == nil {
			return nil, errors.New("sanyparser: nil incomplete expression")
		}
		l := x.node.Location()
		prefix := fmt.Sprintf("line %d, col %d to line %d, col %d of module ", l.bl, l.bc, l.el, l.ec)
		if l.source != nil && l.source != s.context.unknownSource && l.bl == 0 && l.bc == 0 && l.el == 0 && l.ec == 0 {
			prefix = "In module "
		}
		units = append(units, utf16.Encode([]rune("-- incomplete expression at "+prefix))...)
		if l.source == nil {
			units = append(units, utf16.Encode([]rune("null"))...)
		} else {
			units = append(units, l.source.units...)
		}
		units = append(units, utf16.Encode([]rune(".\n"))...)
	}
	s.errors.Push(NewParseError(s.context.UniqueStringOfUnits(units), s.context.UniqueStringOf("-- --")))
	return nil, nil
}
func (s *OperatorStack) ReduceRecord(middle, right *SyntaxTreeNode) error {
	v, e := s.current()
	if e != nil {
		return e
	}
	n := len(v) - 1
	if n < 0 {
		if middle == nil {
			return errors.New("sanyparser: nil record node")
		}
		return rejectAt(middle, "\n    ``.'' has no left hand side at "+loc(middle)+".")
	}
	x := v[n]
	if x == nil {
		return errors.New("sanyparser: invalid record element")
	}
	if x.IsOperator() {
		if n == 0 {
			return errors.New("sanyparser: record operator without predecessor")
		}
		if v[n-1] == nil {
			return errors.New("sanyparser: invalid record predecessor")
		}
		if x.op.IsPostfix() && !v[n-1].IsOperator() {
			s.set(append(v, nil))
			if e = s.reducePostfix(); e != nil {
				s.set(v)
				return e
			}
			v, _ = s.current()
			v = v[:len(v)-1]
			s.set(v)
			n = len(v) - 1
		} else {
			if x.node == nil {
				return errors.New("sanyparser: nil record node")
			}
			return rejectAt(x.node, "\n    ``.'' follows operator "+loc(x.node)+".")
		}
	}
	r, e := s.context.NewNodeTriple(N_RecordComponent, v[n].node, middle, right)
	if e != nil {
		return e
	}
	v[n] = &OSelement{node: r}
	return nil
}
