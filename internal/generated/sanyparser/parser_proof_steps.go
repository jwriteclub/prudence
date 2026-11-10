// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
package generated

import "github.com/jwriteclub/prudence/internal/lexer"

func sanyCorrectProofStep(r *recognizer, t *lexer.Token, n *SyntaxTreeNode, message string) error {
	ok, e := r.state.ProofState.CorrectLevel(t)
	if e != nil {
		return e
	}
	if !ok {
		return reject(n.Location().String() + message)
	}
	if t.Kind == sanyTok_ProofImplicitStepLexeme || t.Kind == tokenProofStepLexeme || t.Kind == tokenProofStepDotLexeme {
		n.originalImage = n.image
		u, e := r.state.ProofState.CorrectedStepNum(t)
		if e != nil {
			return e
		}
		n.image = u
	}
	return nil
}
func sanyRegion104315(r *recognizer, f *FrameQEDStep) error {
	f.Level = -1
	if e := r.state.bpa(r, r.state.UniqueStringOf("QED step")); e != nil {
		return e
	}
	r.state.expecting = r.state.UniqueStringOf("Step number")
	return nil
}
func sanyRegion104696(r *recognizer, f *FrameQEDStep) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	f.Tn = n
	if e = sanyCorrectProofStep(r, f.T, f.Tn, ": QED step's number has bad level."); e != nil {
		return false, e
	}
	if e = r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("QED")
	return false, nil
}
func sanyRegion105197(r *recognizer, f *FrameQEDStep) (bool, error) {
	f.Sn = make([]*SyntaxTreeNode, 1)
	store := sanyNodeSlot(f.Sn, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = store(n); e != nil {
		return false, e
	}
	n, e = r.state.NewNode(r.state.mn, N_QEDStep, f.Sn)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion105353(r *recognizer) (bool, error) {
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	return r.state.ProofState.BeginsProof(t)
}
func sanyRegion105406(r *recognizer, f *FrameQEDStep) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion105433(r *recognizer, f *FrameQEDStep) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	f.Sn = sn
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_ProofStep, f.Sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion105554(r *recognizer, f *FrameStep) error {
	f.T = nil
	f.Tn = nil
	f.MayHaveProof = false
	if e := r.state.bpa(r, r.state.UniqueStringOf("Step")); e != nil {
		return e
	}
	r.state.expecting = r.state.UniqueStringOf("Step number")
	return nil
}
func sanyRegion106937(r *recognizer, f *FrameStep) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	f.Tn = n
	if e = sanyCorrectProofStep(r, f.T, f.Tn, ": step's number has bad level."); e != nil {
		return false, e
	}
	if e = r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("proof step")
	return false, nil
}
func sanyRegion107432(r *recognizer) (bool, error) {
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errProofBindingToken
	}
	if t.Kind == sanyTok_USE {
		return true, nil
	}
	t, e = r.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errProofBindingToken
	}
	return t.Kind == sanyTok_HIDE, nil
}
func sanyRegion107521(r *recognizer, f *FrameStep) (bool, error) { return false, r.state.addHeir(f.Tn) }
func sanyRegion107557(r *recognizer) (bool, error) {
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errProofBindingToken
	}
	return t.Kind == sanyTok_INSTANCE, nil
}
func sanyRegion107617(r *recognizer, f *FrameStep) (bool, error) { return false, r.state.addHeir(f.Tn) }
func sanyRegion107653(r *recognizer) (bool, error) {
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errProofBindingToken
	}
	if t.Kind == tokenDEFBREAK {
		return true, nil
	}
	t, e = r.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errProofBindingToken
	}
	return t.Kind == sanyTok_DEFINE, nil
}
func sanyRegion107771(r *recognizer, f *FrameStep) (bool, error) { return false, r.state.addHeir(f.Tn) }
func sanyRegion107815(r *recognizer, f *FrameStep) (bool, error) { return false, r.state.addHeir(f.Tn) }
func sanyRegion107858(r *recognizer, f *FrameStep) (bool, error) { return false, r.state.addHeir(f.Tn) }
func sanyRegion107901(r *recognizer, f *FrameStep) (bool, error) { return false, r.state.addHeir(f.Tn) }
func sanyRegion107944(r *recognizer, f *FrameStep) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	f.MayHaveProof = true
	return false, nil
}
func sanyRegion108010(r *recognizer, f *FrameStep) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	f.MayHaveProof = true
	return false, nil
}
func sanyRegion108781(r *recognizer, f *FrameStep) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	f.MayHaveProof = true
	return false, nil
}
func sanyRegion108841(r *recognizer) (bool, error) {
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	return r.state.ProofState.BeginsProof(t)
}
func sanyRegion108874(_ *recognizer, f *FrameStep) (bool, error) {
	if !f.MayHaveProof {
		if f.Tn == nil {
			return false, errProofBindingToken
		}
		return false, reject(f.Tn.Location().String() + ": proof of step that does not take a proof.")
	}
	return false, nil
}
func sanyRegion109062(r *recognizer, f *FrameStep) (bool, error) { return false, r.state.addHeir(f.Tn) }
func sanyRegion109088(r *recognizer, f *FrameStep) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_ProofStep, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
