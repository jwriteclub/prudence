// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
package generated

const errProofBindingToken supportError = "sany: nil proof predicate token"

func sanyRegion104093(r *recognizer, f *FrameStepStartToken) error { return nil }
func sanyRegion104270(r *recognizer, f *FrameStepStartToken) (bool, error) {
	f.Result = f.T
	return true, nil
}
func sanyRegion109229(r *recognizer, f *FrameDefStep) error {
	f.T = nil
	f.Tn = nil
	return r.state.bpa(r, r.state.UniqueStringOf("DefStep"))
}
func sanyRegion109696(r *recognizer, f *FrameDefStep) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion109799(r *recognizer, f *FrameDefStep) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion109826(r *recognizer, f *FrameDefStep) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_DefStep, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion109966(r *recognizer, f *FrameHaveStep) error {
	f.T = nil
	f.Tn = nil
	return r.state.bpa(r, r.state.UniqueStringOf("HaveStep"))
}
func sanyRegion110354(r *recognizer, f *FrameHaveStep) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("expression")
	return false, nil
}
func sanyRegion110465(r *recognizer, f *FrameHaveStep) (bool, error) {
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
	n, e := r.state.NewNode(r.state.mn, N_HaveStep, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion110633(r *recognizer, f *FrameTakeStep) error {
	f.T = nil
	f.Tn = nil
	return r.state.bpa(r, r.state.UniqueStringOf("TakeStep"))
}
func sanyRegion111097(r *recognizer, f *FrameTakeStep) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("identifier")
	return false, nil
}
func sanyRegion111279(r *recognizer, f *FrameTakeStep) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("comma or step")
	return false, nil
}
func sanyRegion111371(r *recognizer, f *FrameTakeStep) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("identifier or tuple of identifiers")
	return false, nil
}
func sanyRegion111509(r *recognizer, f *FrameTakeStep) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("comma or proof step")
	return false, nil
}
func sanyRegion111628(r *recognizer, f *FrameTakeStep) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("comma or proof step")
	return false, nil
}
func sanyRegion111725(r *recognizer, f *FrameTakeStep) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("identifier")
	return false, nil
}
func sanyRegion111839(r *recognizer, f *FrameTakeStep) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("comma or proof step")
	return false, nil
}
func sanyRegion111937(r *recognizer, f *FrameTakeStep) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_TakeStep, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion112084(r *recognizer, f *FrameWitnessStep) error {
	f.T = nil
	f.Tn = nil
	return r.state.bpa(r, r.state.UniqueStringOf("WitnessStep"))
}
func sanyRegion112632(r *recognizer, f *FrameWitnessStep) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("expression")
	return false, nil
}
func sanyRegion112739(r *recognizer, f *FrameWitnessStep) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion113085(r *recognizer, f *FrameWitnessStep) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("expression")
	return false, nil
}
func sanyRegion113203(r *recognizer, f *FrameWitnessStep) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("comma or colon")
	return false, nil
}
func sanyRegion113293(r *recognizer, f *FrameWitnessStep) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_WitnessStep, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion113440(r *recognizer, f *FramePickStep) error {
	f.T = nil
	f.Tn = nil
	return r.state.bpa(r, r.state.UniqueStringOf("PickStep"))
}
func sanyRegion113828(r *recognizer, f *FramePickStep) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("identifier")
	return false, nil
}
func sanyRegion114056(r *recognizer, f *FramePickStep) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("comma, or colon")
	return false, nil
}
func sanyRegion114151(r *recognizer, f *FramePickStep) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("identifier")
	return false, nil
}
func sanyRegion114267(r *recognizer, f *FramePickStep) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("comma or colon")
	return false, nil
}
func sanyRegion114385(r *recognizer, f *FramePickStep) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("comma or colon")
	return false, nil
}
func sanyRegion114480(r *recognizer, f *FramePickStep) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("identifier or tuple of identifiers")
	return false, nil
}
func sanyRegion114620(r *recognizer, f *FramePickStep) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("comma or colon")
	return false, nil
}
func sanyRegion114728(r *recognizer, f *FramePickStep) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("expression")
	return false, nil
}
func sanyRegion114837(r *recognizer, f *FramePickStep) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion114859(r *recognizer, f *FramePickStep) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_PickStep, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion115003(r *recognizer, f *FrameCaseStep) error {
	f.T = nil
	f.Tn = nil
	return r.state.bpa(r, r.state.UniqueStringOf("CaseStep"))
}
func sanyRegion115390(r *recognizer, f *FrameCaseStep) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("expression")
	return false, nil
}
func sanyRegion115503(r *recognizer, f *FrameCaseStep) (bool, error) {
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
	n, e := r.state.NewNode(r.state.mn, N_CaseStep, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion115673(r *recognizer, f *FrameAssertStep) error {
	f.T = nil
	f.Tn = nil
	return r.state.bpa(r, r.state.UniqueStringOf("AssertStep"))
}
func sanyRegion116067(r *recognizer, f *FrameAssertStep) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("expression or ASSUME/PROVE")
	return false, nil
}
func sanyRegion116229(r *recognizer) (bool, error) {
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errProofBindingToken
	}
	if t.Kind == sanyTok_ASSUME {
		return true, nil
	}
	t, e = r.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errProofBindingToken
	}
	return t.Kind == sanyTok_BOXASSUME, nil
}
func sanyRegion116329(r *recognizer, f *FrameAssertStep) (bool, error) {
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
	n, e := r.state.NewNode(r.state.mn, N_AssertStep, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
