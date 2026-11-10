// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license; see NOTICE.
// Source SHA-256: bfc981f31c205333f478b773ab108159b5e2d339b659d33b0f920d56ebf4253d
package generated

import (
	"errors"
	"fmt"
	"github.com/jwriteclub/prudence/internal/parserrt"
)

const errExpressionNilToken supportError = "sany expression: nil token"

func sanyRegion116512(*recognizer, *FrameParenthesesExpression) error { return nil }
func sanyRegion116656(_ *recognizer, f *FrameParenthesesExpression) (bool, error) {
	f.Result = f.Tn
	return true, nil
}
func sanyRegion116709(*recognizer, *FrameOpenExpression) error { return nil }
func sanyRegion116862(_ *recognizer, f *FrameOpenExpression) (bool, error) {
	f.Result = f.Tn
	return true, nil
}

func sanyRegion116906(r *recognizer, _ *FrameString) error {
	return r.state.bpa(r, r.state.UniqueStringOf("String"))
}
func sanyRegion116984(r *recognizer, f *FrameString) (bool, error) {
	n, e := r.state.NewTokenNodeKind(r.state.mn, N_String, f.T)
	if e != nil {
		return false, e
	}
	f.Tn = n
	image, e := r.state.ReduceString(f.Tn.image, r.c.Check)
	if e != nil {
		return false, e
	}
	f.Tn.image = image
	if e = r.state.epa(); e != nil {
		return false, e
	}
	f.Result = f.Tn
	return true, nil
}
func sanyRegion117144(_ *recognizer, f *FrameNumber) error { f.Sn = nil; f.Kind = N_Number; return nil }
func sanyRegion117276(r *recognizer, f *FrameNumber) (bool, error) {
	f.Kind = N_Real
	r.state.decimalFlag = true
	f.Sn = make([]*SyntaxTreeNode, 3)
	store := sanyNodeSlot(f.Sn, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T1)
	if e != nil {
		return false, e
	}
	if e = store(n); e != nil {
		return false, e
	}
	store = sanyNodeSlot(f.Sn, 1)
	n, e = r.state.NewTokenNode(r.state.mn, f.T2)
	if e != nil {
		return false, e
	}
	if e = store(n); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion117466(r *recognizer, f *FrameNumber) (bool, error) {
	store := sanyNodeSlot(f.Sn, 2)
	n, e := r.state.NewTokenNode(r.state.mn, f.T1)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion117514(r *recognizer, f *FrameNumber) (bool, error) {
	if f.Sn == nil {
		r.state.numberFlag = true
		f.Sn = make([]*SyntaxTreeNode, 1)
		store := sanyNodeSlot(f.Sn, 0)
		n, e := r.state.NewTokenNode(r.state.mn, f.T1)
		if e != nil {
			return false, e
		}
		if e = store(n); e != nil {
			return false, e
		}
		f.Kind = N_Number
	}
	n, e := r.state.NewNode(r.state.mn, int(f.Kind), f.Sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion117741(r *recognizer, _ *FrameOpArgs) error {
	return r.state.bpa(r, r.state.UniqueStringOf("Optional Arguments"))
}
func sanyRegion117867(r *recognizer, f *FrameOpArgs) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion117938(r *recognizer, f *FrameOpArgs) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion118013(r *recognizer, f *FrameOpArgs) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion118057(r *recognizer, f *FrameOpArgs) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_OpArgs, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion126609(*recognizer, *FrameOpSuite) error { return nil }
func sanyRegion126650(r *recognizer, f *FrameOpSuite) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion126708(_ *recognizer, f *FrameParenExpr) error {
	f.Zn = make([]*SyntaxTreeNode, 3)
	return nil
}
func sanyRegion126806(r *recognizer, f *FrameParenExpr) (bool, error) {
	store := sanyNodeSlot(f.Zn, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion126847(_ *recognizer, f *FrameParenExpr) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 1), nil
}
func sanyRegion126881(r *recognizer, f *FrameParenExpr) (bool, error) {
	store := sanyNodeSlot(f.Zn, 2)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion126922(r *recognizer, f *FrameParenExpr) (bool, error) {
	n, e := r.state.NewNode(r.state.mn, N_ParenExpr, f.Zn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}

func sanyRegion137435(r *recognizer, f *FrameFieldVal) error {
	f.Zn = make([]*SyntaxTreeNode, 3)
	return r.state.bpa(r, r.state.UniqueStringOf("Field Value"))
}
func sanyRegion137523(_ *recognizer, f *FrameFieldVal) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 0), nil
}
func sanyRegion137599(_ *recognizer, f *FrameFieldVal) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 2), nil
}
func sanyRegion137558(r *recognizer, f *FrameFieldVal) (bool, error) {
	store := sanyNodeSlot(f.Zn, 1)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion137622(r *recognizer, f *FrameFieldVal) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_FieldVal, f.Zn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion137712(r *recognizer, f *FrameFieldSet) error {
	f.Zn = make([]*SyntaxTreeNode, 3)
	return r.state.bpa(r, r.state.UniqueStringOf("Field Set"))
}
func sanyRegion137819(_ *recognizer, f *FrameFieldSet) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 0), nil
}
func sanyRegion137895(_ *recognizer, f *FrameFieldSet) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 2), nil
}
func sanyRegion137854(r *recognizer, f *FrameFieldSet) (bool, error) {
	store := sanyNodeSlot(f.Zn, 1)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion137918(r *recognizer, f *FrameFieldSet) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_FieldSet, f.Zn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion145040(r *recognizer, f *FrameIfThenElse) error {
	f.Zn = make([]*SyntaxTreeNode, 6)
	return r.state.bpa(r, r.state.UniqueStringOf("IF THEN ELSE"))
}
func sanyRegion145194(_ *recognizer, f *FrameIfThenElse) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 1), nil
}
func sanyRegion145283(_ *recognizer, f *FrameIfThenElse) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 3), nil
}
func sanyRegion145372(_ *recognizer, f *FrameIfThenElse) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 5), nil
}
func sanyRegion145141(r *recognizer, f *FrameIfThenElse) (bool, error) {
	store := sanyNodeSlot(f.Zn, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion145229(r *recognizer, f *FrameIfThenElse) (bool, error) {
	store := sanyNodeSlot(f.Zn, 2)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion145318(r *recognizer, f *FrameIfThenElse) (bool, error) {
	store := sanyNodeSlot(f.Zn, 4)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion145396(r *recognizer, f *FrameIfThenElse) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_IfThenElse, f.Zn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion146061(r *recognizer, f *FrameCaseArm) error {
	f.Zn = make([]*SyntaxTreeNode, 3)
	return r.state.bpa(r, r.state.UniqueStringOf("Case Arm"))
}
func sanyRegion146146(_ *recognizer, f *FrameCaseArm) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 0), nil
}
func sanyRegion146222(_ *recognizer, f *FrameCaseArm) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 2), nil
}
func sanyRegion146181(r *recognizer, f *FrameCaseArm) (bool, error) {
	store := sanyNodeSlot(f.Zn, 1)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion146245(r *recognizer, f *FrameCaseArm) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_CaseArm, f.Zn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion146335(r *recognizer, f *FrameOtherArm) error {
	f.Zn = make([]*SyntaxTreeNode, 3)
	return r.state.bpa(r, r.state.UniqueStringOf("Case Other Arm"))
}
func sanyRegion146532(_ *recognizer, f *FrameOtherArm) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 2), nil
}
func sanyRegion146438(r *recognizer, f *FrameOtherArm) (bool, error) {
	store := sanyNodeSlot(f.Zn, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion146491(r *recognizer, f *FrameOtherArm) (bool, error) {
	store := sanyNodeSlot(f.Zn, 1)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion146555(r *recognizer, f *FrameOtherArm) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_OtherArm, f.Zn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion147182(r *recognizer, f *FrameLetIn) error {
	f.Zn = make([]*SyntaxTreeNode, 4)
	return r.state.bpa(r, r.state.UniqueStringOf("Case Other Arm"))
}
func sanyRegion147345(_ *recognizer, f *FrameLetIn) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 1), nil
}
func sanyRegion147425(_ *recognizer, f *FrameLetIn) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 3), nil
}
func sanyRegion147304(r *recognizer, f *FrameLetIn) (bool, error) {
	store := sanyNodeSlot(f.Zn, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion147384(r *recognizer, f *FrameLetIn) (bool, error) {
	store := sanyNodeSlot(f.Zn, 2)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion147448(r *recognizer, f *FrameLetIn) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_LetIn, f.Zn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion141440(r *recognizer, _ *FrameReducedExpression) error {
	return r.state.bpa(r, r.state.UniqueStringOf("restricted form of expression"))
}
func sanyRegion141866(r *recognizer, f *FrameReducedExpression) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	f.Result = f.Expr
	return true, nil
}
func sanyRegion147542(r *recognizer, _ *FrameLetDefinitions) error {
	return r.state.bpa(r, r.state.UniqueStringOf("Let Definitions"))
}
func sanyRegion147648(r *recognizer, f *FrameLetDefinitions) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion147691(r *recognizer, f *FrameLetDefinitions) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion147718(r *recognizer, f *FrameLetDefinitions) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_LetDefinitions, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}

func sanyRegion127054(r *recognizer, f *FrameSomeQuant) error {
	return r.state.bpa(r, r.state.UniqueStringOf("Quantified form"))
}
func sanyRegion127396(r *recognizer, f *FrameSomeQuant) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion127654(r *recognizer, f *FrameSomeQuant) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion127849(r *recognizer, f *FrameSomeQuant) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion127957(r *recognizer, f *FrameSomeQuant) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion127722(r *recognizer, f *FrameSomeQuant) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion127915(r *recognizer, f *FrameSomeQuant) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion128019(r *recognizer, f *FrameSomeQuant) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion128198(r *recognizer, f *FrameSomeTQuant) error {
	return r.state.bpa(r, r.state.UniqueStringOf("Bound Quantified Expression"))
}
func sanyRegion128312(r *recognizer, f *FrameSomeTQuant) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion128413(r *recognizer, f *FrameSomeTQuant) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion128520(r *recognizer, f *FrameSomeTQuant) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion128374(r *recognizer, f *FrameSomeTQuant) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion128477(r *recognizer, f *FrameSomeTQuant) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion128738(r *recognizer, f *FrameQuantBound) error {
	return r.state.bpa(r, r.state.UniqueStringOf("Quant Bound"))
}
func sanyRegion128925(r *recognizer, f *FrameQuantBound) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion128825(r *recognizer, f *FrameQuantBound) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion128872(r *recognizer, f *FrameQuantBound) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion138011(r *recognizer, f *FrameExceptSpec) error {
	return r.state.bpa(r, r.state.UniqueStringOf("Except Spec"))
}
func sanyRegion138084(r *recognizer, f *FrameExceptSpec) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion138324(r *recognizer, f *FrameExceptSpec) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion139615(r *recognizer, f *FrameTupleOrAction) error {
	f.Kind = 0
	return r.state.bpa(r, r.state.UniqueStringOf("Some << -- >> or >>_ Form"))
}
func sanyRegion139717(r *recognizer, f *FrameTupleOrAction) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion139818(r *recognizer, f *FrameTupleOrAction) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion139781(r *recognizer, f *FrameTupleOrAction) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion139886(r *recognizer, f *FrameTupleOrAction) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion140101(r *recognizer, f *FrameTupleOrAction) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion145483(r *recognizer, f *FrameCase) error {
	return r.state.bpa(r, r.state.UniqueStringOf("CASE Expression"))
}
func sanyRegion145560(r *recognizer, f *FrameCase) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion145718(r *recognizer, f *FrameCase) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion145850(r *recognizer, f *FrameCase) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion145619(r *recognizer, f *FrameCase) (bool, error) { return false, r.state.addHeir(f.Tn) }
func sanyRegion145779(r *recognizer, f *FrameCase) (bool, error) { return false, r.state.addHeir(f.Tn) }
func sanyRegion145912(r *recognizer, f *FrameCase) (bool, error) { return false, r.state.addHeir(f.Tn) }
func sanyRegion127580(r *recognizer, f *FrameSomeQuant) (bool, error) {
	f.Kind = N_UnboundQuant
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion127780(r *recognizer, f *FrameSomeQuant) (bool, error) {
	f.Kind = N_BoundQuant
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion139925(r *recognizer, f *FrameTupleOrAction) (bool, error) {
	f.Kind = N_Tuple
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion140003(r *recognizer, f *FrameTupleOrAction) (bool, error) {
	f.Kind = N_ActionExpr
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion128040(r *recognizer, f *FrameSomeQuant) (bool, error) {
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
func sanyRegion128582(r *recognizer, f *FrameSomeTQuant) (bool, error) {
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
	n, e := r.state.NewNode(r.state.mn, N_UnboundQuant, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion129181(r *recognizer, f *FrameQuantBound) (bool, error) {
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
	n, e := r.state.NewNode(r.state.mn, N_QuantBound, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion138345(r *recognizer, f *FrameExceptSpec) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_ExceptSpec, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion140126(r *recognizer, f *FrameTupleOrAction) (bool, error) {
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
func sanyRegion145935(r *recognizer, f *FrameCase) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_Case, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion128993(r *recognizer, f *FrameQuantBound) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf(", or \\in")
	return false, nil
}
func sanyRegion129081(r *recognizer, f *FrameQuantBound) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	f.Tn = n
	f.Tn.SetKind(T_IN)
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion138221(r *recognizer, f *FrameExceptSpec) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	f.Tn = n
	f.Tn.SetKind(T_EQUAL)
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion138152(r *recognizer, f *FrameExceptSpec) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("= or ,")
	return false, nil
}
func sanyRegion145653(r *recognizer) (bool, error) {
	ok, e := r.state.caseSep(r)
	if e != nil || !ok {
		return false, e
	}
	t, e := r.Token(2)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errExpressionNilToken
	}
	return t.Kind != sanyTok_OTHER, nil
}
func sanyRegion145816(r *recognizer) (bool, error) { return r.state.caseSep(r) }

func sanyRegion124346(*recognizer, *FrameOpOrExpr) error { return nil }
func sanyRegion125064(r *recognizer) (bool, error) {
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errExpressionNilToken
	}
	return r.state.junctionListCtx.IsAboveCurrent(int32(t.Begin.Column)), nil
}
func sanyRegion125181(_ *recognizer, f *FrameOpOrExpr) (bool, error) {
	f.Kind = N_GenNonExpPrefixOp
	return false, nil
}
func sanyRegion125248(r *recognizer, f *FrameOpOrExpr) (bool, error) {
	f.Kind = N_GenInfixOp
	if f.Tn == nil {
		return false, errors.New("sany expression: nil infix node")
	}
	if f.Tn.image == r.state.UniqueStringOf("\\X") || f.Tn.image == r.state.UniqueStringOf("\\times") {
		return false, parserrt.NewBindingRejection(f.Tn.Location().String()+": \\X may not be used as an infix operator.", nodePosition(f.Tn), nil)
	}
	return false, nil
}
func sanyRegion125623(_ *recognizer, f *FrameOpOrExpr) (bool, error) {
	f.Kind = N_GenPostfixOp
	return false, nil
}
func sanyRegion125654(r *recognizer, f *FrameOpOrExpr) (bool, error) {
	heirs := make([]*SyntaxTreeNode, 2)
	n, e := r.state.NewNode(r.state.mn, N_IdPrefix, nil)
	if e != nil {
		return false, e
	}
	heirs[0] = n
	heirs[1] = f.Tn
	n, e = r.state.NewNode(r.state.mn, int(f.Kind), heirs)
	if e != nil {
		return false, e
	}
	f.Tn = n
	return false, nil
}
func sanyRegion125875(r *recognizer) (bool, error) {
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errExpressionNilToken
	}
	return r.state.junctionListCtx.IsAboveCurrent(int32(t.Begin.Column)), nil
}
func sanyRegion126517(_ *recognizer, f *FrameOpOrExpr) (bool, error) {
	f.Result = f.Tn
	return true, nil
}
func sanyRegion140301(_ *recognizer, f *FrameNoOpExtension) error {
	f.Last = nil
	f.Top = nil
	f.T = nil
	return nil
}
func sanyRegion140512(r *recognizer, f *FrameNoOpExtension) (bool, error) {
	var heirs []*SyntaxTreeNode
	if f.Top == nil {
		heirs = make([]*SyntaxTreeNode, 2)
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		heirs[1] = n
	} else {
		heirs = make([]*SyntaxTreeNode, 3)
		heirs[1] = f.Top
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		heirs[2] = n
	}
	heirs[0] = f.Tid
	n, e := r.state.NewNode(r.state.mn, N_IdPrefixElement, heirs)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion140990(r *recognizer, f *FrameNoOpExtension) (bool, error) {
	if f.Last == nil {
		if f.Top != nil {
			r.state.FairnessHook = f.Top
		} else {
			r.state.FairnessHook = nil
		}
		zn := make([]*SyntaxTreeNode, 2)
		heirs, e := r.state.getLastHeirs(r.c.Check)
		if e != nil {
			return false, e
		}
		n, e := r.state.NewNode(r.state.mn, N_IdPrefix, heirs)
		if e != nil {
			return false, e
		}
		zn[0] = n
		zn[1] = f.Tid
		n, e = r.state.NewNode(r.state.mn, N_GeneralId, zn)
		if e != nil {
			return false, e
		}
		f.Last = n
	}
	return false, nil
}
func sanyRegion141383(_ *recognizer, f *FrameNoOpExtension) (bool, error) {
	f.Result = f.Last
	return true, nil
}
func sanyRegion138488(r *recognizer, _ *FrameExceptComponent) error {
	return r.state.bpa(r, r.state.UniqueStringOf("Except Component"))
}
func sanyRegion138567(r *recognizer, f *FrameExceptComponent) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	next, e := r.Token(1)
	if e != nil {
		return false, e
	}
	ok, e := IsFieldNameToken(next)
	if e != nil {
		return false, e
	}
	if ok {
		next.Kind = tokenIdentifier
	}
	return false, nil
}
func sanyRegion139071(r *recognizer, f *FrameExceptComponent) (bool, error) {
	if f.Tn == nil || f.Tn.image == nil {
		return false, errors.New("sany expression: null identifier image")
	}
	if f.Tn.image == r.state.At {
		r.state.PErrors.Push(positionedError(r.state.NewSimpleParseError(r.state.UniqueStringOf("@ used in !.@")), f.Tn))
	}
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion139219(r *recognizer, f *FrameExceptComponent) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion139283(r *recognizer, f *FrameExceptComponent) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion139320(r *recognizer, f *FrameExceptComponent) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion139386(r *recognizer, f *FrameExceptComponent) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion139422(r *recognizer, f *FrameExceptComponent) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion139470(r *recognizer, f *FrameExceptComponent) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_ExceptComponent, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion142425(r *recognizer, f *FrameFairnessExpr) error {
	f.Zn = make([]*SyntaxTreeNode, 5)
	r.state.FairnessHook = nil
	return r.state.bpa(r, r.state.UniqueStringOf("Fairness Expression"))
}
func sanyRegion142593(r *recognizer, f *FrameFairnessExpr) (bool, error) {
	store := sanyNodeSlot(f.Zn, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion142694(r *recognizer, f *FrameFairnessExpr) (bool, error) {
	if e := sanyNodeSlot(f.Zn, 1)(f.Expr); e != nil {
		return false, e
	}
	f.Expr = nil
	store := sanyNodeSlot(f.Zn, 2)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion142821(_ *recognizer, f *FrameFairnessExpr) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 3), nil
}
func sanyRegion142856(r *recognizer, f *FrameFairnessExpr) (bool, error) {
	store := sanyNodeSlot(f.Zn, 4)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, store(n)
}
func sanyRegion142901(r *recognizer, f *FrameFairnessExpr) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	if f.Expr != nil {
		if r.state.FairnessHook == nil {
			if len(f.Zn) < 1 || f.Zn[0] == nil {
				return false, errors.New("sany fairness: missing first node")
			}
			loc := f.Zn[0].location
			return false, parserrt.NewBindingRejection(fmt.Sprintf("Ill-structured fairness expression at line %d, column %d", loc[0], loc[1]), nodePosition(f.Zn[0]), nil)
		}
		parameters := r.state.FairnessHook.Heirs()
		if parameters != nil && len(parameters) == 3 {
			if e := sanyNodeSlot(f.Zn, 1)(f.Expr); e != nil {
				return false, e
			}
			for i := 0; i < 3; i++ {
				if e := sanyNodeSlot(f.Zn, i+2)(parameters[i]); e != nil {
					return false, e
				}
			}
		} else {
			if len(f.Zn) < 1 || f.Zn[0] == nil {
				return false, errors.New("sany fairness: missing first node")
			}
			loc := f.Zn[0].location
			r.state.PErrors.Push(positionedError(NewParseError(r.state.UniqueStringOf(fmt.Sprintf("Error in fairness expression at %d: %d\n", loc[0], loc[1])), r.state.UniqueStringOf("-- --")), f.Zn[0]))
			f.Result = f.Expr
			return true, nil
		}
	} else {
		if len(f.Zn) < 2 || f.Zn[1] == nil {
			return false, errors.New("sany fairness: missing expression")
		}
		if f.Zn[1].IsKind(N_GeneralId) && r.state.FairnessHook != nil {
			ozn := make([]*SyntaxTreeNode, 2)
			ozn[0] = f.Zn[1]
			ozn[1] = r.state.FairnessHook
			store := sanyNodeSlot(f.Zn, 1)
			n, e := r.state.NewNode(r.state.mn, N_OpApplication, ozn)
			if e != nil {
				return false, e
			}
			if e = store(n); e != nil {
				return false, e
			}
		} else if !(f.Zn[1].IsKind(N_Tuple) || f.Zn[1].IsKind(N_ParenExpr) || f.Zn[1].IsKind(N_SetEnumerate) || f.Zn[1].IsKind(N_SubsetOf) || f.Zn[1].IsKind(N_SetOfAll) || f.Zn[1].IsKind(N_SetOfFcns) || f.Zn[1].IsKind(N_RcdConstructor) || f.Zn[1].IsKind(N_SetOfRcds) || f.Zn[1].IsKind(N_Except) || f.Zn[1].IsKind(N_FcnConst) || f.Zn[1].IsKind(N_ActionExpr)) {
			if len(f.Zn) < 1 || f.Zn[0] == nil {
				return false, errors.New("sany fairness: missing first node")
			}
			loc := f.Zn[0].location
			r.state.PErrors.Push(positionedError(NewParseError(r.state.UniqueStringOf(fmt.Sprintf("Error in fairness expression at %d: %d: could not link arguments\n", loc[0], loc[1])), r.state.UniqueStringOf("-- --")), f.Zn[0]))
			f.Result = f.Zn[1]
			return true, nil
		}
	}
	n, e := r.state.NewNode(r.state.mn, N_FairnessExpr, f.Zn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}

func sanyRegion134672(r *recognizer, f *FrameSBracketCases) error {
	f.Kind = 0
	return r.state.bpa(r, r.state.UniqueStringOf("Some [] Form"))
}
func sanyRegion134818(r *recognizer) (bool, error) { return matchFcnConst(r, r.c.Check) }
func sanyRegion134762(r *recognizer, f *FrameSBracketCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion135019(r *recognizer, f *FrameSBracketCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion135123(r *recognizer, f *FrameSBracketCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion135220(r *recognizer, f *FrameSBracketCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion135415(r *recognizer, f *FrameSBracketCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion135515(r *recognizer, f *FrameSBracketCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion135798(r *recognizer, f *FrameSBracketCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion135898(r *recognizer, f *FrameSBracketCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion136088(r *recognizer, f *FrameSBracketCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion136188(r *recognizer, f *FrameSBracketCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion136291(r *recognizer, f *FrameSBracketCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion136682(r *recognizer, f *FrameSBracketCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion136913(r *recognizer, f *FrameSBracketCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion137019(r *recognizer, f *FrameSBracketCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion135085(r *recognizer, f *FrameSBracketCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion135187(r *recognizer, f *FrameSBracketCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion135479(r *recognizer, f *FrameSBracketCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion135862(r *recognizer, f *FrameSBracketCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion136152(r *recognizer, f *FrameSBracketCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion136252(r *recognizer, f *FrameSBracketCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion136359(r *recognizer, f *FrameSBracketCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion136647(r *recognizer, f *FrameSBracketCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion136874(r *recognizer, f *FrameSBracketCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion136981(r *recognizer, f *FrameSBracketCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion137187(r *recognizer, f *FrameSBracketCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion134951(r *recognizer, f *FrameSBracketCases) (bool, error) {
	f.Kind = N_FcnConst
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion135342(r *recognizer, f *FrameSBracketCases) (bool, error) {
	f.Kind = N_RcdConstructor
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion135725(r *recognizer, f *FrameSBracketCases) (bool, error) {
	f.Kind = N_RcdConstructor
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion136019(r *recognizer, f *FrameSBracketCases) (bool, error) {
	f.Kind = N_SetOfRcds
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion136510(r *recognizer, f *FrameSBracketCases) (bool, error) {
	f.Kind = N_SetOfFcns
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion136743(r *recognizer, f *FrameSBracketCases) (bool, error) {
	f.Kind = N_Except
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion137077(r *recognizer, f *FrameSBracketCases) (bool, error) {
	f.Kind = N_ActionExpr
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion136404(r *recognizer, f *FrameSBracketCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	f.Kind = N_FcnAppl
	r.state.lastOp = r.state.FcnOp
	return false, nil
}
func sanyRegion135572(r *recognizer) (bool, error) {
	t, e := r.Token(2)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errExpressionNilToken
	}
	if t.Kind != sanyTok_MAPTO {
		return false, nil
	}
	t, e = r.Token(1)
	if e != nil {
		return false, e
	}
	return IsFieldNameToken(t)
}
func sanyRegion135670(r *recognizer, _ *FrameSBracketCases) (bool, error) {
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errExpressionNilToken
	}
	t.Kind = tokenIdentifier
	return false, nil
}
func sanyRegion137218(r *recognizer, f *FrameSBracketCases) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if f.Kind == 0 {
		return false, errors.New("sany: assertion failed in SBracketCases")
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

func sanyRegion129375(r *recognizer, f *FrameBraceCases) error {
	f.Kind = N_SetEnumerate
	f.Htn = nil
	f.Te = false
	return r.state.bpa(r, r.state.UniqueStringOf("Some { } form"))
}
func sanyRegion129914(r *recognizer) (bool, error) { return matchFcnConst(r, r.c.Check) }
func sanyRegion129858(r *recognizer, f *FrameBraceCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion132131(r *recognizer, f *FrameBraceCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion132388(r *recognizer, f *FrameBraceCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion132488(r *recognizer, f *FrameBraceCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion133892(r *recognizer, f *FrameBraceCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion131751(r *recognizer, f *FrameBraceCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion131942(r *recognizer, f *FrameBraceCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion132199(r *recognizer, f *FrameBraceCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion132346(r *recognizer, f *FrameBraceCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion132452(r *recognizer, f *FrameBraceCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion132554(r *recognizer, f *FrameBraceCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion132598(r *recognizer, f *FrameBraceCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion133854(r *recognizer, f *FrameBraceCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion133958(r *recognizer, f *FrameBraceCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion134099(r *recognizer, f *FrameBraceCases) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion129987(r *recognizer, f *FrameBraceCases) (bool, error) {
	f.Tn_0 = f.Tn
	r.state.expecting = r.state.UniqueStringOf("\\in")
	return false, nil
}
func sanyRegion130175(r *recognizer, f *FrameBraceCases) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	f.Tn_1 = n
	f.Tn_1.SetKind(T_IN)
	return false, nil
}
func sanyRegion130758(r *recognizer, f *FrameBraceCases) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("':', ',' or '}'")
	zn := make([]*SyntaxTreeNode, 3)
	wn := make([]*SyntaxTreeNode, 2)
	n, e := r.state.NewNode(r.state.mn, N_IdPrefix, make([]*SyntaxTreeNode, 0))
	if e != nil {
		return false, e
	}
	wn[0] = n
	wn[1] = f.Tn_0
	n, e = r.state.NewNode(r.state.mn, N_GeneralId, wn)
	if e != nil {
		return false, e
	}
	zn[0] = n
	wn = make([]*SyntaxTreeNode, 2)
	n, e = r.state.NewNode(r.state.mn, N_IdPrefix, make([]*SyntaxTreeNode, 0))
	if e != nil {
		return false, e
	}
	wn[0] = n
	wn[1] = f.Tn_1
	n, e = r.state.NewNode(r.state.mn, N_GenInfixOp, wn)
	if e != nil {
		return false, e
	}
	zn[1] = n
	zn[2] = f.Tn_2
	n, e = r.state.NewNode(r.state.mn, N_InfixExpr, zn)
	if e != nil {
		return false, e
	}
	f.Htn = n
	return false, nil
}
func sanyRegion131500(r *recognizer, f *FrameBraceCases) (bool, error) {
	f.Htn = nil
	if e := r.state.addHeir(f.Tn_0); e != nil {
		return false, e
	}
	if e := r.state.addHeir(f.Tn_1); e != nil {
		return false, e
	}
	if e := r.state.addHeir(f.Tn_2); e != nil {
		return false, e
	}
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	f.Kind = N_SubsetOf
	return false, nil
}
func sanyRegion131792(r *recognizer, f *FrameBraceCases) (bool, error) {
	if f.Htn != nil {
		if e := r.state.addHeir(f.Htn); e != nil {
			return false, e
		}
		f.Htn = nil
	}
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion132059(r *recognizer, f *FrameBraceCases) (bool, error) {
	f.Kind = N_SetEnumerate
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion132290(_ *recognizer, f *FrameBraceCases) (bool, error) {
	f.Te = true
	f.Kind = N_SetOfAll
	return false, nil
}
func sanyRegion132636(r *recognizer, f *FrameBraceCases) (bool, error) {
	f.Te = true
	f.Kind = N_SetOfAll
	if f.Tn == nil {
		return false, errors.New("sany brace: nil expression")
	}
	hone := f.Tn.Heirs()
	if len(hone) > 1 {
		if hone[1] == nil {
			return false, errors.New("sany brace: nil second heir")
		}
		hone = hone[1].Heirs()
		if f.Tn.IsKind(N_InfixExpr) {
			if len(hone) < 2 || hone[1] == nil || hone[1].image == nil {
				return false, errors.New("sany brace: missing infix image")
			}
			if hone[1].image == r.state.UniqueStringOf("\\in") {
				if f.T == nil {
					return false, errExpressionNilToken
				}
				return false, parserrt.NewBindingRejection(fmt.Sprintf("Form {a \\in b : c \\in d }, at line %d, is not allowed", f.T.Begin.Line), f.T.Begin, nil)
			}
		}
	}
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion134000(r *recognizer, f *FrameBraceCases) (bool, error) {
	f.Kind = N_SetEnumerate
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion134144(r *recognizer, f *FrameBraceCases) (bool, error) {
	if f.Htn != nil {
		if e := r.state.addHeir(f.Htn); e != nil {
			return false, e
		}
	}
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e = r.state.NewNode(r.state.mn, int(f.Kind), sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
