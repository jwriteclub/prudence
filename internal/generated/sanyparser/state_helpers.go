// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
// Source SHA-256: bfc981f31c205333f478b773ab108159b5e2d339b659d33b0f920d56ebf4253d
package generated

import (
	"errors"
	"fmt"
	"math"
	"unicode/utf16"

	"github.com/jwriteclub/prudence/internal/lexer"
	"github.com/jwriteclub/prudence/internal/parserrt"
)

type SANYLogLevel int

const (
	SANYTrace SANYLogLevel = iota
	SANYError
)

type SANYLogger func(SANYLogLevel, *UniqueString) error

// Each slot represents one Java Object array element, including nil token slots.
type messageSlot struct {
	text  *UniqueString
	token *lexer.Token
}
type SANYState struct {
	partialBodyDepth int
	partialBody      []*SyntaxTreeNode
	*SupportContext
	out                                  SANYLogger
	ParseTree                            *SyntaxTreeNode
	dependencyList                       []*UniqueString
	mn                                   *UniqueString
	numberFlag, decimalFlag              bool
	FcnOp                                *Operator
	FairnessHook                         *SyntaxTreeNode
	At                                   *UniqueString
	PErrors                              *ParseErrors
	OperatorStack                        *OperatorStack
	junctionListCtx                      *JunctionListContext
	lastOp                               *Operator
	msgStack                             []messageSlot
	msgStackMaxSize, msgStackCurrentSize int32
	expecting                            *UniqueString
	heirsTable                           []*SyntaxTreeNode
	heirsSize, heirsIndex                int32
	emptyString                          *UniqueString
	internals                            []*UniqueString
	*ProofState
}

func NewSANYState(log SANYLogger) (*SANYState, error) {
	s := &SANYState{SupportContext: NewSupportContext(), out: log}
	s.dependencyList = make([]*UniqueString, 0, 20)
	var err error
	s.FcnOp, err = s.LookupOperator(s.UniqueStringOf("["))
	if err != nil {
		return nil, err
	}
	s.At = s.UniqueStringOf("@")
	s.PErrors = NewParseErrors()
	s.OperatorStack = s.NewOperatorStack(s.PErrors)
	s.junctionListCtx = NewJunctionListContext()
	s.msgStack = make([]messageSlot, 512)
	s.msgStackMaxSize = 512
	s.expecting = s.UniqueStringOf("nothing")
	s.heirsTable = make([]*SyntaxTreeNode, 512)
	s.heirsSize = 512
	s.emptyString = s.UniqueStringOf("")
	s.internals = make([]*UniqueString, 0)
	s.ProofState = s.NewProofState()
	return s, nil
}
func stateCheck(check func() error) error {
	if check != nil {
		return check()
	}
	return nil
}
func (s *SANYState) log(level SANYLogLevel, prefix string, value *UniqueString) error {
	if s.out == nil {
		return nil
	}
	u := utf16.Encode([]rune(prefix))
	if value == nil {
		u = append(u, utf16.Encode([]rune("null"))...)
	} else {
		u = append(u, value.units...)
	}
	return s.out(level, s.UniqueStringOfUnits(u))
}
func (s *SANYState) addHeir(n *SyntaxTreeNode) error {
	if s.heirsIndex == s.heirsSize {
		if s.heirsSize > math.MaxInt32-512 {
			return errors.New("sany heirs: array size overflow")
		}
		ntable := make([]*SyntaxTreeNode, int(s.heirsSize+512))
		copy(ntable, s.heirsTable)
		s.heirsSize += 512
		s.heirsTable = ntable
	}
	if s.heirsIndex < 0 || int64(s.heirsIndex) >= int64(len(s.heirsTable)) {
		return errors.New("sany heirs: index out of bounds")
	}
	s.heirsTable[s.heirsIndex] = n
	s.heirsIndex++
	return nil
}
func (s *SANYState) getLastHeirs(check func() error) ([]*SyntaxTreeNode, error) {
	i := s.heirsIndex - 1
	for {
		if err := stateCheck(check); err != nil {
			return nil, err
		}
		if i < 0 || int64(i) >= int64(len(s.heirsTable)) {
			return nil, errors.New("sany heirs: index out of bounds")
		}
		if s.heirsTable[i] == nil {
			break
		}
		i--
	}
	count := s.heirsIndex - i - 1
	if count == 0 {
		return nil, nil
	}
	result := make([]*SyntaxTreeNode, int(count))
	copy(result, s.heirsTable[i+1:s.heirsIndex])
	s.heirsIndex = i + 1
	return result, nil
}
func (s *SANYState) popHeir() (bool, error) {
	s.heirsIndex--
	if s.heirsIndex < 0 || int64(s.heirsIndex) >= int64(len(s.heirsTable)) {
		return false, errors.New("sany heirs: index out of bounds")
	}
	return s.heirsTable[s.heirsIndex] == nil, nil
}
func (s *SANYState) pushMsg(text *UniqueString, tok *lexer.Token) error {
	if s.msgStackCurrentSize == s.msgStackMaxSize {
		size := s.msgStackMaxSize * 2
		if size < 0 {
			return errors.New("sany messages: array size overflow")
		}
		neo := make([]messageSlot, int(size))
		copy(neo, s.msgStack)
		s.msgStack = neo
		s.msgStackMaxSize = size
	}
	i := s.msgStackCurrentSize
	if i < 0 || int64(i) >= int64(len(s.msgStack)) {
		return errors.New("sany messages: index out of bounds")
	}
	s.msgStack[i] = messageSlot{text: text}
	if int64(i)+1 >= int64(len(s.msgStack)) {
		return errors.New("sany messages: index out of bounds")
	}
	s.msgStack[i+1] = messageSlot{token: tok}
	s.msgStackCurrentSize += 2
	return nil
}
func (s *SANYState) popMsg() { s.msgStackCurrentSize -= 2 }
func (s *SANYState) bpa(v localTokenView, name *UniqueString) error {
	if err := s.addHeir(nil); err != nil {
		return err
	}
	if err := s.log(SANYTrace, "Beginning ", name); err != nil {
		return err
	}
	t, err := v.Token(1)
	if err != nil {
		return err
	}
	if err = s.pushMsg(name, t); err != nil {
		return err
	}
	s.expecting = s.emptyString
	return nil
}
func (s *SANYState) epa() error {
	s.popMsg()
	i := s.msgStackCurrentSize
	if i < 0 || int64(i) >= int64(len(s.msgStack)) {
		return errors.New("sany messages: index out of bounds")
	}
	if err := s.log(SANYTrace, "Ending ", s.msgStack[i].text); err != nil {
		return err
	}
	ok, err := s.popHeir()
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("sany parser: assertion failed in epa")
	}
	s.expecting = s.emptyString
	return nil
}
func (s *SANYState) addDependency(u *UniqueString, check func() error) error {
	for _, v := range s.internals {
		if err := stateCheck(check); err != nil {
			return err
		}
		if u == v {
			return nil
		}
	}
	s.dependencyList = append(s.dependencyList, u)
	return nil
}
func IsPrefixOp(t *lexer.Token) (bool, error) {
	if t == nil {
		return false, errNilPreviewToken
	}
	return t.Kind >= tokenOp26 && t.Kind <= tokenOp116, nil
}
func IsFieldNameToken(t *lexer.Token) (bool, error) {
	if t == nil {
		return false, errNilPreviewToken
	}
	k := t.Kind
	return k >= tokenAction && k <= tokenExcept || k == tokenExtends || k >= tokenIf && k <= tokenSF || k == tokenState || k >= tokenThen && k <= tokenWith || k == tokenUS || k >= tokenOp112 && k <= tokenOp116, nil
}
func IsGenOp(n *SyntaxTreeNode) (bool, error) {
	if n == nil {
		return false, errors.New("sany: nil general operator node")
	}
	k := n.kind
	return k == N_GenPrefixOp || k == N_GenNonExpPrefixOp || k == N_GenInfixOp || k == N_GenPostfixOp || k == N_FcnAppl, nil
}
func childAt(n *SyntaxTreeNode, i int) (*SyntaxTreeNode, error) {
	if n == nil {
		return nil, errors.New("sany: nil tree node")
	}
	a := n.Heirs()
	if i < 0 || i >= len(a) {
		return nil, errors.New("sany: child index out of bounds")
	}
	return a[i], nil
}
func (s *SANYState) isLabel(n *SyntaxTreeNode, check func() error) (bool, error) {
	if n == nil {
		return false, nil
	}
	if n.IsKind(N_GeneralId) {
		c, e := childAt(n, 0)
		if e != nil {
			return false, e
		}
		if c == nil {
			return false, errors.New("sany: nil label child")
		}
		return len(c.Heirs()) == 0, nil
	}
	if !n.IsKind(N_OpApplication) {
		return false, nil
	}
	args, e := childAt(n, 1)
	if e != nil {
		return false, e
	}
	if args == nil {
		return false, errors.New("sany: nil op args")
	}
	if args.kind != N_OpArgs {
		if e = s.log(SANYError, "Bug: not N_OpArgs node", s.UniqueStringOf("")); e != nil {
			return false, e
		}
	}
	for i := 1; i < len(args.Heirs()); i += 2 {
		if e = stateCheck(check); e != nil {
			return false, e
		}
		g, e := childAt(args, i)
		if e != nil {
			return false, e
		}
		if g == nil {
			return false, errors.New("sany: nil label argument")
		}
		if g.kind != N_GeneralId {
			return false, nil
		}
		c, e := childAt(g, 0)
		if e != nil {
			return false, e
		}
		if c == nil {
			return false, errors.New("sany: nil label prefix")
		}
		if len(c.Heirs()) != 0 {
			return false, nil
		}
	}
	return true, nil
}
func (s *SANYState) labelDoesNotChangeParse(n *SyntaxTreeNode, op *Operator) (bool, error) {
	if n == nil {
		return false, errors.New("sany: nil labeled expression")
	}
	if !n.IsKind(N_InfixExpr) && !n.IsKind(N_PostfixExpr) {
		return true, nil
	}
	top := s.OperatorStack.TopOfStack()
	if top == nil {
		return true, nil
	}
	stackOp := top.Operator()
	if stackOp == nil {
		return true, nil
	}
	return Prec(stackOp, op)
}
func (s *SANYState) checkIndentation(n, junct *SyntaxTreeNode, check func() error) error {
	if n == nil {
		return errors.New("sany: nil indentation node")
	}
	type frame struct {
		children []*SyntaxTreeNode
		index    int
	}
	stack := []frame{{children: n.Heirs()}}
	for len(stack) > 0 {
		f := &stack[len(stack)-1]
		if f.index == len(f.children) {
			stack = stack[:len(stack)-1]
			continue
		}
		if err := stateCheck(check); err != nil {
			return err
		}
		child := f.children[f.index]
		f.index++
		if child == nil {
			return errors.New("sany: nil indentation child")
		}
		if child.IsKind(N_ConjList) || child.IsKind(N_DisjList) {
			continue
		}
		if !s.junctionListCtx.IsAboveCurrent(child.location[1]) {
			if junct == nil {
				return errors.New("sany: nil junction node")
			}
			msg := fmt.Sprintf("Item at %s is not properly indented inside conjunction or  disjunction list item at %s", child.Location(), junct.Location())
			return parserrt.NewBindingRejection(msg, nodePosition(child), nil)
		}
		stack = append(stack, frame{children: child.Heirs()})
	}
	return nil
}
func (s *SANYState) caseSep(v localTokenView) (bool, error) {
	t, e := v.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errNilPreviewToken
	}
	if t.Kind != tokenCaseSep {
		return false, nil
	}
	if int64(t.Begin.Column) < math.MinInt32 || int64(t.Begin.Column) > math.MaxInt32 {
		return false, errors.New("sany: column outside Java int range")
	}
	return s.junctionListCtx.IsAboveCurrent(int32(t.Begin.Column)), nil
}
func matchFcnConst(v localTokenView, check func() error) (bool, error) {
	t, e := InitPreviewToken(v)
	if e != nil {
		return false, e
	}
	t, e = NextPreviewToken(v, t)
	if e != nil {
		return false, e
	}
	if t.Kind == tokenLAB {
		count := int32(1)
		for count != 0 {
			if e = stateCheck(check); e != nil {
				return false, e
			}
			t, e = NextPreviewToken(v, t)
			if e != nil {
				return false, e
			}
			if t.Kind == tokenLAB {
				count++
			}
			if t.Kind == tokenRAB {
				count--
			}
			if t.Kind == tokenEOF {
				return false, nil
			}
		}
		t, e = NextPreviewToken(v, t)
		if e != nil {
			return false, e
		}
		return t.Kind == tokenIN, nil
	}
	if t.Kind == tokenIdentifier {
		t, e = NextPreviewToken(v, t)
		if e != nil {
			return false, e
		}
		for t.Kind == tokenComma {
			if e = stateCheck(check); e != nil {
				return false, e
			}
			t, e = NextPreviewToken(v, t)
			if e != nil {
				return false, e
			}
			if t.Kind != tokenIdentifier {
				return false, nil
			}
			t, e = NextPreviewToken(v, t)
			if e != nil {
				return false, e
			}
		}
		return t.Kind == tokenIN, nil
	}
	return false, nil
}
