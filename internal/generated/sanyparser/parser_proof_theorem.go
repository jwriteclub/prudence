// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
package generated

func sanyRegion94001(r *recognizer, _ *FrameTheorem) error {
	if e := r.state.bpa(r, r.state.UniqueStringOf("Theorem")); e != nil {
		return e
	}
	r.state.expecting = r.state.UniqueStringOf("THEOREM, PROPOSITION")
	return nil
}
func sanyRegion94225(r *recognizer, f *FrameTheorem) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("Identifier, Assume-Prove or Expression")
	return false, nil
}
func sanyRegion94360(r *recognizer, f *FrameTheorem) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("==")
	return false, nil
}
func sanyRegion94418(r *recognizer, f *FrameTheorem) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion94645(r *recognizer, _ *FrameTheorem) (bool, error) {
	return false, BelchDEF(r, r.c.Check)
}
func sanyRegion94695(r *recognizer) (bool, error) {
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
func sanyRegion94815(r *recognizer, f *FrameTheorem) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion95083(r *recognizer) (bool, error) {
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	return r.state.BeginsProof(t)
}
func sanyRegion95136(r *recognizer, f *FrameTheorem) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion95163(r *recognizer, f *FrameTheorem) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_Theorem, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
