// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation, MIT license; see NOTICE.
// Source SHA-256: bfc981f31c205333f478b773ab108159b5e2d339b659d33b0f920d56ebf4253d
package generated

import (
	"errors"
	"github.com/jwriteclub/prudence/internal/lexer"
	"github.com/jwriteclub/prudence/internal/parserrt"
	"math"
)

func sanyRegion78776(r *recognizer, f *FrameSomeFixDecl) error {
	f.LocalASTN = nil
	f.Sn = nil
	return r.state.bpa(r, r.state.UniqueStringOf("Op. Symbol Declaration"))
}
func sanyRegion79037(r *recognizer, f *FrameSomeFixDecl) (bool, error) {
	f.Kind = int32(N_PrefixDecl)
	if r.state.lastOp == nil {
		return false, errors.New("sany: nil last operator")
	}
	f.N = r.state.lastOp.Identifier()
	f.Sn = make([]*SyntaxTreeNode, 2)
	if e := sanyNodeSlot(f.Sn, 0)(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("_")
	return false, nil
}
func sanyRegion79179(r *recognizer, f *FrameSomeFixDecl) (bool, error) {
	store := sanyNodeSlot(f.Sn, 1)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion79240(r *recognizer, _ *FrameSomeFixDecl) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("infix or postfix operator")
	return false, nil
}
func sanyRegion79374(r *recognizer, f *FrameSomeFixDecl) (bool, error) {
	f.Kind = int32(N_InfixDecl)
	if r.state.lastOp == nil {
		return false, errors.New("sany: nil last operator")
	}
	f.N = r.state.lastOp.Identifier()
	f.Sn = make([]*SyntaxTreeNode, 3)
	if e := sanyNodeSlot(f.Sn, 1)(f.Tn); e != nil {
		return false, e
	}
	store := sanyNodeSlot(f.Sn, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = store(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("_")
	return false, nil
}
func sanyRegion79570(r *recognizer, f *FrameSomeFixDecl) (bool, error) {
	store := sanyNodeSlot(f.Sn, 2)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion79633(r *recognizer, f *FrameSomeFixDecl) (bool, error) {
	f.Kind = int32(N_PostfixDecl)
	if r.state.lastOp == nil {
		return false, errors.New("sany: nil last operator")
	}
	f.N = r.state.lastOp.Identifier()
	f.Sn = make([]*SyntaxTreeNode, 2)
	if e := sanyNodeSlot(f.Sn, 1)(f.Tn); e != nil {
		return false, e
	}
	store := sanyNodeSlot(f.Sn, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion79813(r *recognizer, f *FrameSomeFixDecl) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, int(f.Kind), f.Sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion79897(r *recognizer, f *FrameInstance) error {
	f.Zn = nil
	if e := r.state.bpa(r, r.state.UniqueStringOf("Instance")); e != nil {
		return e
	}
	r.state.expecting = r.state.UniqueStringOf("LOCAL or instance")
	return nil
}
func sanyRegion80031(r *recognizer, f *FrameInstance) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	f.Zn = n
	return false, nil
}
func sanyRegion80097(r *recognizer, f *FrameInstance) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("COMMA or Module Body")
	return false, nil
}
func sanyRegion80154(r *recognizer, f *FrameInstance) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNodeOptional(r.state.mn, N_Instance, f.Zn, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion80296(r *recognizer, _ *FrameInstantiation) error {
	return r.state.bpa(r, r.state.UniqueStringOf("NonLocalInstance"))
}
func sanyRegion80379(r *recognizer, f *FrameInstantiation) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("Module identifier")
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	f.T = t
	ok, e := IsFieldNameToken(t)
	if e != nil {
		return false, e
	}
	if ok {
		t.Kind = tokenIdentifier
	}
	return false, nil
}
func sanyRegion80543(r *recognizer, f *FrameInstantiation) (bool, error) {
	if f.Tn == nil {
		return false, errors.New("sany: nil dependency node")
	}
	if e := r.state.addDependency(f.Tn.image, r.c.Check); e != nil {
		return false, e
	}
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("WITH or another definition.")
	return false, nil
}
func sanyRegion80648(r *recognizer, f *FrameInstantiation) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.emptyString
	return false, nil
}
func sanyRegion80739(r *recognizer, f *FrameInstantiation) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.emptyString
	return false, nil
}
func sanyRegion80821(r *recognizer, f *FrameInstantiation) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.emptyString
	return false, nil
}
func sanyRegion80914(r *recognizer, f *FrameInstantiation) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.emptyString
	return false, nil
}
func sanyRegion80971(r *recognizer, f *FrameInstantiation) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_NonLocalInstance, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion81887(r *recognizer, f *FrameSubstitution) error {
	f.Zn = make([]*SyntaxTreeNode, 3)
	f.Tn = nil
	return r.state.bpa(r, r.state.UniqueStringOf("Substitution"))
}
func sanyRegion82036(_ *recognizer, f *FrameSubstitution) (bool, error) {
	return false, sanyNodeSlot(f.Zn, 0)(f.Tn)
}
func sanyRegion82370(_ *recognizer, f *FrameSubstitution) (bool, error) {
	return false, sanyNodeSlot(f.Zn, 0)(f.Tn)
}
func sanyRegion82405(_ *recognizer, f *FrameSubstitution) (bool, error) {
	return false, sanyNodeSlot(f.Zn, 0)(f.Tn)
}
func sanyRegion82442(_ *recognizer, f *FrameSubstitution) (bool, error) {
	return false, sanyNodeSlot(f.Zn, 0)(f.Tn)
}
func sanyRegion82462(r *recognizer, _ *FrameSubstitution) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("<-")
	return false, nil
}
func sanyRegion82503(r *recognizer, f *FrameSubstitution) (bool, error) {
	if f.Tn == nil {
		return false, errors.New("sany: nil substitution node")
	}
	f.N = f.Tn.image
	store := sanyNodeSlot(f.Zn, 1)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = store(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("Expression or Op. Symbol")
	return false, nil
}
func sanyRegion82626(r *recognizer, f *FrameSubstitution) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	if e := sanyNodeSlot(f.Zn, 2)(f.Tn); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_Substitution, f.Zn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}

// Java evaluates the two getToken calls separately and in this order.
func sanyBulletValues(r *recognizer) (int32, int32, error) {
	a, e := r.Token(1)
	if e != nil {
		return 0, 0, e
	}
	if a == nil {
		return 0, 0, errors.New("sany: nil junction token")
	}
	c := a.Begin.Column
	if int64(c) < math.MinInt32 || int64(c) > math.MaxInt32 {
		return 0, 0, errors.New("sany: junction column outside Java int")
	}
	b, e := r.Token(1)
	if e != nil {
		return 0, 0, e
	}
	if b == nil {
		return 0, 0, errors.New("sany: nil junction token")
	}
	if int64(b.Kind) < math.MinInt32 || int64(b.Kind) > math.MaxInt32 {
		return 0, 0, errors.New("sany: junction kind outside Java int")
	}
	return int32(c), int32(b.Kind), nil
}
func sanyRegion147857(r *recognizer, _ *FrameJunctions) error {
	c, k, e := sanyBulletValues(r)
	if e != nil {
		return e
	}
	if e = r.state.junctionListCtx.Start(c, k); e != nil {
		return e
	}
	return r.state.bpa(r, r.state.UniqueStringOf("AND-OR Junction"))
}
func sanyRegion148334(_ *recognizer, f *FrameJunctions) (bool, error) {
	f.Kind = int32(N_DisjList)
	return false, nil
}
func sanyRegion148406(_ *recognizer, f *FrameJunctions) (bool, error) {
	f.Kind = int32(N_ConjList)
	return false, nil
}
func sanyRegion148445(r *recognizer, f *FrameJunctions) (bool, error) {
	if e := r.state.junctionListCtx.Terminate(); e != nil {
		return false, e
	}
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, int(f.Kind), sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion148614(*recognizer, *FrameDisjList) error { return nil }
func sanyRegion148670(r *recognizer, f *FrameDisjList) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion148706(r *recognizer) (bool, error) {
	c, k, e := sanyBulletValues(r)
	if e != nil {
		return false, e
	}
	return r.state.junctionListCtx.IsNewBullet(c, k), nil
}
func sanyRegion148815(r *recognizer, f *FrameDisjList) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion148858(*recognizer, *FrameConjList) error { return nil }
func sanyRegion148914(r *recognizer, f *FrameConjList) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion148951(r *recognizer) (bool, error) {
	c, k, e := sanyBulletValues(r)
	if e != nil {
		return false, e
	}
	return r.state.junctionListCtx.IsNewBullet(c, k), nil
}
func sanyRegion149060(r *recognizer, f *FrameConjList) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion149130(r *recognizer, f *FrameJuncItem) error {
	f.Zn = make([]*SyntaxTreeNode, 2)
	return r.state.bpa(r, r.state.UniqueStringOf("Junction Item"))
}
func sanyRegion149306(r *recognizer, f *FrameJuncItem) (bool, error) {
	store := sanyNodeSlot(f.Zn, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion149351(_ *recognizer, f *FrameJuncItem) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 1), nil
}
func sanyRegion149374(r *recognizer, f *FrameJuncItem) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, int(f.ItemKind), f.Zn)
	if e != nil {
		return false, e
	}
	f.Tn = n
	children := n.Heirs()
	for i := 1; i < len(children); i++ {
		if e = r.c.Check(); e != nil {
			return false, e
		}
		if e = r.state.checkIndentation(children[i], n, r.c.Check); e != nil {
			return false, e
		}
	}
	f.Result = f.Tn
	return true, nil
}
func sanyRegion149879(r *recognizer, f *FrameUnboundOrBoundChoose) error {
	f.Zn = make([]*SyntaxTreeNode, 5)
	return r.state.bpa(r, r.state.UniqueStringOf("(Un)Bounded Choose"))
}
func sanyRegion150008(r *recognizer, f *FrameUnboundOrBoundChoose) (bool, error) {
	store := sanyNodeSlot(f.Zn, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion150051(_ *recognizer, f *FrameUnboundOrBoundChoose) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 1), nil
}
func sanyRegion150076(_ *recognizer, f *FrameUnboundOrBoundChoose) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 1), nil
}
func sanyRegion150106(_ *recognizer, f *FrameUnboundOrBoundChoose) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 2), nil
}
func sanyRegion150141(r *recognizer, f *FrameUnboundOrBoundChoose) (bool, error) {
	store := sanyNodeSlot(f.Zn, 3)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion150182(_ *recognizer, f *FrameUnboundOrBoundChoose) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 4), nil
}
func sanyRegion150205(r *recognizer, f *FrameUnboundOrBoundChoose) (bool, error) {
	_, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_UnboundOrBoundChoose, f.Zn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion150964(r *recognizer, _ *FrameLambda) error {
	return r.state.bpa(r, r.state.UniqueStringOf("Lambda"))
}
func sanyRegion151032(r *recognizer, f *FrameLambda) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("Identifier")
	return false, nil
}
func sanyRegion151140(r *recognizer, f *FrameLambda) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("`,' or `:'")
	return false, nil
}
func sanyRegion151253(r *recognizer, f *FrameLambda) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("Identifier")
	return false, nil
}
func sanyRegion151372(r *recognizer, f *FrameLambda) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("`,' or `:'")
	return false, nil
}
func sanyRegion151470(r *recognizer, f *FrameLambda) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("Expression")
	return false, nil
}
func sanyRegion151578(r *recognizer, f *FrameLambda) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_Lambda, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion169303(*recognizer, *FrameOpenStart) error { return nil }

func sanyAtToken(r *recognizer) (bool, error) {
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errors.New("sany: nil token")
	}
	u := tokenUnits(t)
	return len(u) == 1 && u[0] == '@', nil
}
func sanyRegion166905(r *recognizer, _ *FrameBangExt) error {
	return r.state.bpa(r, r.state.UniqueStringOf("Bang Extension"))
}
func sanyRegion167673(r *recognizer, f *FrameBangExt) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion167748(r *recognizer) (bool, error) { at, e := sanyAtToken(r); return !at, e }
func sanyRegion168289(r *recognizer, f *FrameBangExt) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion168345(r *recognizer, f *FrameBangExt) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion168390(r *recognizer, f *FrameBangExt) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion168427(r *recognizer, f *FrameBangExt) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion168449(r *recognizer, f *FrameBangExt) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_IdPrefixElement, sn)
	if e != nil {
		return false, e
	}
	f.Tn = n
	if e = r.state.epa(); e != nil {
		return false, e
	}
	f.Result = f.Tn
	return true, nil
}
func sanyRegion168585(r *recognizer, f *FrameStructOp) error {
	f.Tn = nil
	f.T = nil
	if e := r.state.bpa(r, r.state.UniqueStringOf("StructOp")); e != nil {
		return e
	}
	r.state.expecting = r.state.UniqueStringOf("`<<' , `>>' , `:' , `@' , or number")
	return nil
}
func sanyRegion168780(r *recognizer) (bool, error) { return sanyAtToken(r) }
func sanyRegion168836(r *recognizer, f *FrameStructOp) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	if f.Tn == nil {
		n, e := r.state.NewTokenNodeKind(r.state.mn, N_StructOp, f.T)
		if e != nil {
			return false, e
		}
		f.Result = n
		return true, nil
	}
	if f.Tn.kind == N_Real {
		return false, parserrt.NewBindingRejection("Illegal structural term at "+f.Tn.Location().String(), nodePosition(f.Tn), nil)
	}
	heirs := make([]*SyntaxTreeNode, 1)
	heirs[0] = f.Tn
	n, e := r.state.NewNode(r.state.mn, N_StructOp, heirs)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}

func sanyAboveCurrent(r *recognizer) (bool, error) {
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errors.New("sany: nil token")
	}
	c := t.Begin.Column
	if int64(c) < math.MinInt32 || int64(c) > math.MaxInt32 {
		return false, errors.New("sany: column outside Java int")
	}
	return r.state.junctionListCtx.IsAboveCurrent(int32(c)), nil
}

// The stack check is invocation-scoped; never retain another recognizer's check.
func sanyReduceStack(r *recognizer) error {
	s := r.state.OperatorStack
	old := s.Check
	s.Check = r.c.Check
	defer func() { s.Check = old }()
	return s.ReduceStack()
}
func sanyFinalReduce(r *recognizer) (*SyntaxTreeNode, error) {
	s := r.state.OperatorStack
	old := s.Check
	s.Check = r.c.Check
	defer func() { s.Check = old }()
	return s.FinalReduce()
}
func sanyReduceRecord(r *recognizer, a, b *SyntaxTreeNode) error {
	s := r.state.OperatorStack
	old := s.Check
	s.Check = r.c.Check
	defer func() { s.Check = old }()
	return s.ReduceRecord(a, b)
}
func sanyPushGeneral(r *recognizer, kind int, heirs []*SyntaxTreeNode) error {
	n, e := r.state.NewNode(r.state.mn, kind, heirs)
	if e != nil {
		return e
	}
	s := r.state.OperatorStack
	if e = s.PushOnStack(n, r.state.lastOp); e != nil {
		return e
	}
	size, e := s.Size()
	if e != nil {
		return e
	}
	if size != 1 {
		return sanyReduceStack(r)
	}
	return nil
}
func sanyRegion151791(r *recognizer, _ *FrameExpression) error {
	if e := r.state.bpa(r, r.state.UniqueStringOf("Expression")); e != nil {
		return e
	}
	r.state.OperatorStack.NewStack()
	return nil
}
func sanyRegion151969(r *recognizer) (bool, error) { return sanyAboveCurrent(r) }
func sanyRegion152050(_ *recognizer, f *FrameExpression) (bool, error) {
	f.Kind = int32(N_GenPrefixOp)
	return false, nil
}
func sanyRegion152095(_ *recognizer, f *FrameExpression) (bool, error) {
	f.Kind = int32(N_GenInfixOp)
	return false, nil
}
func sanyRegion152437(r *recognizer, f *FrameExpression) (bool, error) {
	heirs := make([]*SyntaxTreeNode, 2)
	n, e := r.state.NewNode(r.state.mn, N_IdPrefix, nil)
	if e != nil {
		return false, e
	}
	heirs[0] = n
	heirs[1] = f.Tn
	return false, sanyPushGeneral(r, int(f.Kind), heirs)
}
func sanyRegion153081(r *recognizer) (bool, error) { return sanyAboveCurrent(r) }
func sanyRegion153165(r *recognizer, f *FrameExpression) (bool, error) {
	return false, r.state.OperatorStack.PushOnStack(f.Tn, nil)
}
func sanyRegion153221(r *recognizer) (bool, error) { return sanyAboveCurrent(r) }
func sanyRegion153578(r *recognizer, f *FrameExpression) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := sanyFinalReduce(r)
	if e != nil {
		return false, e
	}
	f.Tn = n
	if n == nil {
		return false, parserrt.NewBindingRejection(" Couldn't reduce expression stack.", lexer.Position{}, nil)
	}
	if e = r.state.OperatorStack.PopStack(); e != nil {
		return false, e
	}
	f.Result = f.Tn
	return true, nil
}

func sanyRegion153841(r *recognizer, _ *FrameExtendableExpr) error {
	return r.state.bpa(r, r.state.UniqueStringOf("ExtendableExpr"))
}
func sanyRegion154262(r *recognizer) (bool, error) { return r.state.OperatorStack.PreInEmptyTop(), nil }
func sanyRegion154320(r *recognizer, f *FrameExtendableExpr) (bool, error) {
	return false, r.state.OperatorStack.PushOnStack(f.Tn, nil)
}
func sanyRegion154630(r *recognizer, f *FrameExtendableExpr) (bool, error) {
	return false, r.state.OperatorStack.PushOnStack(f.Tn, nil)
}
func sanyRegion154929(r *recognizer, f *FrameExtendableExpr) (bool, error) {
	return false, r.state.OperatorStack.PushOnStack(f.Tn, nil)
}
func sanyRegion155001(r *recognizer) (bool, error) { return sanyAboveCurrent(r) }
func sanyExtendGeneral(r *recognizer, f *FrameExtendableExpr, kind int) error {
	f.Heirs = make([]*SyntaxTreeNode, 2)
	n, e := r.state.NewNode(r.state.mn, N_IdPrefix, nil)
	if e != nil {
		return e
	}
	f.Heirs[0] = n
	f.Heirs[1] = f.Tn
	return sanyPushGeneral(r, kind, f.Heirs)
}
func sanyRegion155087(r *recognizer, f *FrameExtendableExpr) (bool, error) {
	return false, sanyExtendGeneral(r, f, N_GenPostfixOp)
}
func sanyRegion155452(r *recognizer) (bool, error) { return sanyAboveCurrent(r) }
func sanyRegion155531(r *recognizer, _ *FrameExtendableExpr) (bool, error) {
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	ok, e := IsFieldNameToken(t)
	if e != nil {
		return false, e
	}
	if ok {
		t.Kind = tokenIdentifier
	}
	return false, nil
}
func sanyRegion155640(r *recognizer) (bool, error) { return sanyAboveCurrent(r) }
func sanyRegion155728(r *recognizer, f *FrameExtendableExpr) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, sanyReduceRecord(r, n, f.Tn)
}
func sanyRegion155815(r *recognizer) (bool, error) { return sanyAboveCurrent(r) }
func sanyRegion156109(r *recognizer, _ *FrameExtendableExpr) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("function argument")
	return false, nil
}
func sanyRegion156172(r *recognizer, f *FrameExtendableExpr) (bool, error) {
	ok, e := IsGenOp(f.Tn)
	if e != nil {
		return false, e
	}
	if !ok {
		return false, r.state.OperatorStack.PushOnStack(f.Tn, nil)
	}
	if e = r.state.OperatorStack.PushOnStack(f.Tn, r.state.lastOp); e != nil {
		return false, e
	}
	n, e := r.state.OperatorStack.Size()
	if e != nil {
		return false, e
	}
	if n != 1 {
		return false, sanyReduceStack(r)
	}
	return false, nil
}
func sanyRegion157035(r *recognizer) (bool, error) { return sanyAboveCurrent(r) }
func sanyRegion157117(r *recognizer, f *FrameExtendableExpr) (bool, error) {
	return false, sanyExtendGeneral(r, f, N_GenInfixOp)
}
func sanyRegion157506(r *recognizer) (bool, error) { return sanyAboveCurrent(r) }
func sanyRegion157589(_ *recognizer, f *FrameExtendableExpr) (bool, error) {
	f.Kind = int32(N_GenPrefixOp)
	return false, nil
}
func sanyRegion157636(_ *recognizer, f *FrameExtendableExpr) (bool, error) {
	f.Kind = int32(N_GenInfixOp)
	return false, nil
}
func sanyRegion157989(r *recognizer, f *FrameExtendableExpr) (bool, error) {
	return false, sanyExtendGeneral(r, f, int(f.Kind))
}
func sanyRegion158625(r *recognizer) (bool, error) { return sanyAboveCurrent(r) }
func sanyRegion158711(r *recognizer, f *FrameExtendableExpr) (bool, error) {
	return false, r.state.OperatorStack.PushOnStack(f.Tn, nil)
}
func sanyRegion158769(r *recognizer) (bool, error) { return sanyAboveCurrent(r) }
func sanyRegion159162(r *recognizer) (bool, error) { return sanyAboveCurrent(r) }
func sanyRegion159245(r *recognizer, f *FrameExtendableExpr) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	f.Tn1 = n
	top := r.state.OperatorStack.TopOfStack()
	if top == nil {
		return false, errors.New("sany: nil operator stack top")
	}
	f.Tn0 = top.Node()
	ok, e := r.state.isLabel(f.Tn0, r.c.Check)
	if e != nil {
		return false, e
	}
	if !ok {
		return false, parserrt.NewBindingRejection("`::' at "+f.Tn1.Location().String()+" does not follow a label.", nodePosition(f.Tn1), nil)
	}
	return false, r.state.OperatorStack.PopCurrentTop()
}
func sanyRegion159595(r *recognizer, f *FrameExtendableExpr) (bool, error) {
	ok, e := r.state.labelDoesNotChangeParse(f.Tn2, r.state.lastOp)
	if e != nil {
		return false, e
	}
	if !ok {
		if f.Tn0 == nil {
			return false, errors.New("sany: nil label node")
		}
		return false, parserrt.NewBindingRejection("Removing label at "+f.Tn0.Location().String()+" would change expression parsing.", nodePosition(f.Tn0), nil)
	}
	heirs := []*SyntaxTreeNode{f.Tn0, f.Tn1, f.Tn2}
	n, e := r.state.NewNodeInherit(N_Label, heirs)
	if e != nil {
		return false, e
	}
	f.Tn = n
	return false, r.state.OperatorStack.PushOnStack(f.Tn, nil)
}
func sanyRegion160265(r *recognizer, _ *FrameExtendableExpr) (bool, error) {
	return false, r.state.epa()
}

func sanyChild(n *SyntaxTreeNode, i int) (*SyntaxTreeNode, error) {
	if n == nil {
		return nil, errors.New("sany: nil tree node")
	}
	a := n.Heirs()
	if i < 0 || i >= len(a) {
		return nil, errors.New("sany: tree child index out of bounds")
	}
	return a[i], nil
}
func sanyRegion160330(r *recognizer, f *FramePrimitiveExp) error {
	f.TnOpArgs = nil
	f.TnBangs = nil
	return r.state.bpa(r, r.state.UniqueStringOf("Primitive expression"))
}
func sanyRegion160537(r *recognizer) (bool, error) { return sanyAboveCurrent(r) }
func sanyRegion160663(r *recognizer) (bool, error) { return sanyAboveCurrent(r) }
func sanyRegion160852(r *recognizer, f *FramePrimitiveExp) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	f.Tn0 = n
	level, e := r.state.ProofState.Get()
	if e != nil {
		return false, e
	}
	if level < 0 && r.state.proofDepth <= 0 {
		return false, parserrt.NewBindingRejection(f.Tn0.Location().String()+": Step number used outside proof.", nodePosition(f.Tn0), nil)
	}
	if f.T == nil {
		return false, errors.New("sany: nil proof token")
	}
	u := tokenUnits(f.T)
	if len(u) < 2 {
		return false, errors.New("sany: proof token substring out of bounds")
	}
	if u[1] == '+' {
		return false, parserrt.NewBindingRejection(f.Tn0.Location().String()+": <+> step number used in an expression.", nodePosition(f.Tn0), nil)
	}
	if f.T.Kind == sanyTok_ProofImplicitStepLexeme {
		target := f.Tn0
		target.originalImage = target.image
		im, e := r.state.ProofState.CorrectedStepNum(f.T)
		if e != nil {
			return false, e
		}
		target.image = im
	}
	return false, nil
}
func sanyRegion162026(r *recognizer, f *FramePrimitiveExp) (bool, error) {
	return false, r.state.addHeir(f.Tn1)
}
func sanyRegion162060(r *recognizer, f *FramePrimitiveExp) (bool, error) {
	bangs, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	f.TnBangs = bangs
	if bangs == nil {
		f.Heirs = make([]*SyntaxTreeNode, 2)
		n, e := r.state.NewNode(r.state.mn, N_IdPrefix, make([]*SyntaxTreeNode, 0))
		if e != nil {
			return false, e
		}
		f.Heirs[0] = n
		f.Heirs[1] = f.Tn0
		n, e = r.state.NewNode(r.state.mn, N_GeneralId, f.Heirs)
		if e != nil {
			return false, e
		}
		f.Tn1 = n
		if f.TnOpArgs == nil {
			f.Tn = f.Tn1
		} else {
			f.Heirs = make([]*SyntaxTreeNode, 2)
			f.Heirs[0] = f.Tn1
			f.Heirs[1] = f.TnOpArgs
			n, e = r.state.NewNode(r.state.mn, N_OpApplication, f.Heirs)
			if e != nil {
				return false, e
			}
			f.Tn = n
		}
		return false, nil
	}
	f.Heirs = make([]*SyntaxTreeNode, len(bangs))
	var elt []*SyntaxTreeNode
	if f.TnOpArgs == nil {
		elt = make([]*SyntaxTreeNode, 2)
		elt[0] = f.Tn0
		v, e := sanyChild(bangs[0], 0)
		if e != nil {
			return false, e
		}
		elt[1] = v
	} else {
		elt = make([]*SyntaxTreeNode, 3)
		elt[0] = f.Tn0
		elt[1] = f.TnOpArgs
		v, e := sanyChild(bangs[0], 0)
		if e != nil {
			return false, e
		}
		elt[2] = v
	}
	n, e := r.state.NewNode(r.state.mn, N_IdPrefixElement, elt)
	if e != nil {
		return false, e
	}
	f.Heirs[0] = n
	for i := 0; i < len(bangs)-1; i++ {
		if e = r.c.Check(); e != nil {
			return false, e
		}
		if bangs[i] == nil {
			return false, errors.New("sany: nil bang node")
		}
		elt = make([]*SyntaxTreeNode, len(bangs[i].Heirs()))
		for j := 0; j < len(elt)-1; j++ {
			if e = r.c.Check(); e != nil {
				return false, e
			}
			v, e := sanyChild(bangs[i], j+1)
			if e != nil {
				return false, e
			}
			elt[j] = v
		}
		store := sanyNodeSlot(elt, len(elt)-1)
		v, e := sanyChild(bangs[i+1], 0)
		if e != nil {
			return false, e
		}
		if e = store(v); e != nil {
			return false, e
		}
		n, e = r.state.NewNode(r.state.mn, N_IdPrefixElement, elt)
		if e != nil {
			return false, e
		}
		f.Heirs[i+1] = n
	}
	if bangs[len(bangs)-1] == nil {
		return false, errors.New("sany: nil last bang")
	}
	last := bangs[len(bangs)-1].Heirs()
	gen := make([]*SyntaxTreeNode, 2)
	n, e = r.state.NewNode(r.state.mn, N_IdPrefix, f.Heirs)
	if e != nil {
		return false, e
	}
	gen[0] = n
	if len(last) <= 1 {
		return false, errors.New("sany: last bang child index out of bounds")
	}
	gen[1] = last[1]
	n, e = r.state.NewNode(r.state.mn, N_GeneralId, gen)
	if e != nil {
		return false, e
	}
	if len(last) == 2 {
		f.Tn = n
	} else {
		f.Heirs = make([]*SyntaxTreeNode, 2)
		f.Heirs[0] = n
		if len(last) <= 2 {
			return false, errors.New("sany: last bang child index out of bounds")
		}
		f.Heirs[1] = last[2]
		n, e = r.state.NewNode(r.state.mn, N_OpApplication, f.Heirs)
		if e != nil {
			return false, e
		}
		f.Tn = n
	}
	return false, nil
}
func sanyRegion166829(r *recognizer, f *FramePrimitiveExp) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	f.Result = f.Tn
	return true, nil
}
