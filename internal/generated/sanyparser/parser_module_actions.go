// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
// Source SHA-256: bfc981f31c205333f478b773ab108159b5e2d339b659d33b0f920d56ebf4253d
package generated

import "errors"

// Capture the array and index before evaluating a child production. Java checks
// the array store only after the child returns successfully.
func sanyNodeSlot(a []*SyntaxTreeNode, i int) func(*SyntaxTreeNode) error {
	return func(n *SyntaxTreeNode) error {
		if i < 0 || i >= len(a) {
			return errors.New("sany: node array store out of bounds")
		}
		a[i] = n
		return nil
	}
}
func sanyRegion64009(r *recognizer, f *FrameCompilationUnit) error {
	f.State = r.state
	r.state.out = f.Logger
	return BelchDEF(r, r.c.Check)
}
func sanyRegion64093(r *recognizer, _ *FrameCompilationUnit) (bool, error) {
	sw, ok := r.Source().(interface{ SwitchToDefault() error })
	if !ok {
		return false, errors.New("sany: token source lacks default lexical-state switch")
	}
	return false, sw.SwitchToDefault()
}
func sanyRegion64227(_ *recognizer, f *FrameCompilationUnit) (bool, error) {
	f.Result = f.TempASTN
	return true, nil
}
func sanyRegion64267(*recognizer, *FramePrelude) error { return nil }
func sanyRegion64348(r *recognizer, f *FrameModule) error {
	f.LSTN = make([]*SyntaxTreeNode, 4)
	if e := r.state.bpa(r, r.state.UniqueStringOf("Module definition")); e != nil {
		return e
	}
	f.StackLevel = int32(len(r.state.internals))
	f.Pop = nil
	return nil
}
func sanyRegion64510(r *recognizer, _ *FrameModule) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("---- MODULE")
	return false, nil
}
func sanyRegion64543(_ *recognizer, f *FrameModule) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.LSTN, 0), nil
}
func sanyRegion64568(r *recognizer, _ *FrameModule) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("EXTENDS clause or module body")
	return false, nil
}
func sanyRegion64619(_ *recognizer, f *FrameModule) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.LSTN, 1), nil
}
func sanyRegion64640(r *recognizer, _ *FrameModule) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("Module body")
	return false, nil
}
func sanyRegion64673(_ *recognizer, f *FrameModule) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.LSTN, 2), nil
}
func sanyRegion64691(r *recognizer, _ *FrameModule) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("==== or more Module body")
	return false, nil
}
func sanyRegion64737(_ *recognizer, f *FrameModule) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.LSTN, 3), nil
}
func sanyRegion64760(r *recognizer, f *FrameModule) (bool, error) {
	for int32(len(r.state.internals)) > f.StackLevel {
		if e := r.c.Check(); e != nil {
			return false, e
		}
		r.state.internals = r.state.internals[1:]
	}
	if len(f.LSTN) == 0 || f.LSTN[0] == nil {
		return false, errors.New("sany: nil module header")
	}
	z := f.LSTN[0].zero
	if len(z) <= 1 || z[1] == nil {
		return false, errors.New("sany: missing module name")
	}
	u := z[1].image
	if u == nil {
		return false, errors.New("sany: null module name in internals")
	}
	r.state.internals = append([]*UniqueString{u}, r.state.internals...)
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_Module, f.LSTN)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion64968(r *recognizer, f *FrameBeginModule) error {
	f.LSTN = make([]*SyntaxTreeNode, 3)
	return r.state.bpa(r, r.state.UniqueStringOf("Begin module"))
}
func sanyRegion65057(r *recognizer, _ *FrameBeginModule) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("---- MODULE (beginning of module)")
	return false, nil
}
func sanyRegion65153(r *recognizer, f *FrameBeginModule) (bool, error) {
	store := sanyNodeSlot(f.LSTN, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = store(n); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion65195(r *recognizer, f *FrameBeginModule) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("Identifier")
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
func sanyRegion65296(_ *recognizer, f *FrameBeginModule) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.LSTN, 1), nil
}
func sanyRegion65319(r *recognizer, f *FrameBeginModule) (bool, error) {
	if r.state.mn == nil {
		if len(f.LSTN) <= 1 || f.LSTN[1] == nil {
			return false, errors.New("sany: missing module name node")
		}
		r.state.mn = f.LSTN[1].image
	}
	r.state.expecting = r.state.UniqueStringOf("----")
	return false, nil
}
func sanyRegion65410(r *recognizer, f *FrameBeginModule) (bool, error) {
	store := sanyNodeSlot(f.LSTN, 2)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = store(n); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion65455(r *recognizer, f *FrameBeginModule) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_BeginModule, f.LSTN)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion65552(_ *recognizer, f *FrameEndModule) error {
	f.LSTN = make([]*SyntaxTreeNode, 1)
	return nil
}
func sanyRegion65636(r *recognizer, f *FrameEndModule) (bool, error) {
	store := sanyNodeSlot(f.LSTN, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = store(n); e != nil {
		return false, e
	}
	n, e = r.state.NewNode(r.state.mn, N_EndModule, f.LSTN)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}

func sanyRegion65766(r *recognizer, _ *FrameExtends) error {
	return r.state.bpa(r, r.state.UniqueStringOf("Extends"))
}
func sanyRegion65846(r *recognizer, f *FrameExtends) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion65887(r *recognizer, f *FrameExtends) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("Identifier")
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
func sanyRegion66006(r *recognizer, f *FrameExtends) (bool, error) {
	if f.Tn == nil {
		return false, errors.New("sany: nil dependency node")
	}
	if e := r.state.addDependency(f.Tn.image, r.c.Check); e != nil {
		return false, e
	}
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion66052(r *recognizer, _ *FrameExtends) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("comma or module body")
	return false, nil
}
func sanyRegion66113(r *recognizer, f *FrameExtends) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion66155(r *recognizer, f *FrameExtends) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("Identifier")
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
func sanyRegion66275(r *recognizer, f *FrameExtends) (bool, error) {
	if f.Tn == nil {
		return false, errors.New("sany: nil dependency node")
	}
	if e := r.state.addDependency(f.Tn.image, r.c.Check); e != nil {
		return false, e
	}
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion66330(r *recognizer, f *FrameExtends) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_Extends, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}

func sanyRegion67968(r *recognizer, _ *FrameParamDeclaration) error {
	if e := r.state.bpa(r, r.state.UniqueStringOf("Parameter declaration")); e != nil {
		return e
	}
	r.state.expecting = r.state.UniqueStringOf("CONSTANT")
	return nil
}
func sanyRegion68113(r *recognizer, f *FrameParamDeclaration) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion68130(r *recognizer, _ *FrameParamDeclaration) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("Identifier, operator or _")
	return false, nil
}
func sanyRegion68207(r *recognizer, f *FrameParamDeclaration) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion68224(r *recognizer, _ *FrameParamDeclaration) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf(",")
	return false, nil
}
func sanyRegion68264(r *recognizer, f *FrameParamDeclaration) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion68307(r *recognizer, _ *FrameParamDeclaration) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("Identifier, operator or _")
	return false, nil
}
func sanyRegion68386(r *recognizer, f *FrameParamDeclaration) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion68410(r *recognizer, f *FrameParamDeclaration) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	f.Sn = sn
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_ParamDeclaration, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}

func sanyRegion66461(r *recognizer, _ *FrameBody) error {
	r.state.partialBodyDepth++
	if e := r.state.bpa(r, r.state.UniqueStringOf("Module body")); e != nil {
		return e
	}
	r.state.expecting = r.state.UniqueStringOf("LOCAL, INSTANCE, PROOF, ASSUMPTION, THEOREM, RECURSIVE, declaration, or definition")
	return nil
}
func sanyRegion66668(r *recognizer, f *FrameBody) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	f.Tn = n
	return false, nil
}
func sanyRegion67064(r *recognizer, _ *FrameBody) (bool, error) { return false, BelchDEF(r, r.c.Check) }
func sanyRegion67098(r *recognizer) (bool, error) {
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errors.New("sany: nil predicate token")
	}
	if t.Kind == tokenUse {
		u, e := r.Token(2)
		if e != nil {
			return false, e
		}
		if u == nil {
			return false, errors.New("sany: nil predicate token")
		}
		if u.Kind != tokenOnly {
			return true, nil
		}
	}
	t, e = r.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errors.New("sany: nil predicate token")
	}
	return t.Kind == tokenHide, nil
}
func sanyRegion67242(r *recognizer, f *FrameBody) (bool, error) {
	if err := r.state.addHeir(f.Tn); err != nil {
		return false, err
	}
	if r.state.partialBodyDepth == 1 && f.Tn != nil {
		r.state.partialBody = append(r.state.partialBody, f.Tn)
	}
	return false, nil
}
func sanyRegion67264(r *recognizer, f *FrameBody) (bool, error) {
	r.state.partialBodyDepth--
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	f.Sn = sn
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_Body, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion67388(r *recognizer, f *FrameVariableDeclaration) error {
	f.LSTN = make([]*SyntaxTreeNode, 1)
	return r.state.bpa(r, r.state.UniqueStringOf("variable declaration"))
}
func sanyRegion67534(r *recognizer, f *FrameVariableDeclaration) (bool, error) {
	store := sanyNodeSlot(f.LSTN, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion67576(r *recognizer, _ *FrameVariableDeclaration) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("Identifier")
	return false, nil
}
func sanyRegion67626(r *recognizer, f *FrameVariableDeclaration) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion67645(r *recognizer, _ *FrameVariableDeclaration) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("comma or module body")
	return false, nil
}
func sanyRegion67706(r *recognizer, f *FrameVariableDeclaration) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion67750(r *recognizer, _ *FrameVariableDeclaration) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("Identifier")
	return false, nil
}
func sanyRegion67802(r *recognizer, f *FrameVariableDeclaration) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion67828(r *recognizer, f *FrameVariableDeclaration) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	f.Sn = sn
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNodeParts(r.state.mn, N_VariableDeclaration, f.LSTN, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion68769(r *recognizer, _ *FrameParamSubDecl) error {
	return r.state.bpa(r, r.state.UniqueStringOf("Parameter declaration item"))
}
func sanyRegion68873(r *recognizer, f *FrameParamSubDecl) (bool, error) {
	f.Sn = make([]*SyntaxTreeNode, 1)
	store := sanyNodeSlot(f.Sn, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = store(n); e != nil {
		return false, e
	}
	n, e = r.state.NewNode(r.state.mn, N_ConsDecl, f.Sn)
	if e != nil {
		return false, e
	}
	f.Tn = n
	return false, nil
}
func sanyRegion68998(r *recognizer, f *FrameParamSubDecl) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	f.Result = f.Tn
	return true, nil
}
