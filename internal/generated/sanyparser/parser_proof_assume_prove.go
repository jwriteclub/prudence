// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
package generated

func sanyRegion85003(r *recognizer, _ *FrameAssumeProve) error {
	return r.state.bpa(r, r.state.UniqueStringOf("Assume-Prove"))
}
func sanyRegion87112(r *recognizer, f *FrameAssumeProve) (bool, error) {
	prefix, e := r.state.NewNode(r.state.mn, N_IdPrefix, make([]*SyntaxTreeNode, 0))
	if e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_GeneralId, []*SyntaxTreeNode{prefix, f.Tn})
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion87890(r *recognizer, f *FrameAssumeProve) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion87953(r *recognizer, f *FrameAssumeProve) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion87972(r *recognizer, f *FrameAssumeProve) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	f.Sn = sn
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_Label, f.Sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
func sanyRegion88109(r *recognizer, f *FrameAssumeProve) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("Expression, Declaration, or AssumeProve")
	return false, nil
}
func sanyAssumeProveAhead(r *recognizer) (bool, error) {
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
	if t.Kind == sanyTok_BOXASSUME {
		return true, nil
	}
	t, e = r.Token(2)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errProofBindingToken
	}
	if t.Kind != sanyTok_COLONCOLON {
		return false, nil
	}
	t, e = r.Token(3)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errProofBindingToken
	}
	if t.Kind == sanyTok_ASSUME {
		return true, nil
	}
	t, e = r.Token(3)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errProofBindingToken
	}
	return t.Kind == sanyTok_BOXASSUME, nil
}
func sanyRegion88223(r *recognizer) (bool, error)                  { return sanyAssumeProveAhead(r) }
func sanyRegion88736(*recognizer, *FrameAssumeProve) (bool, error) { return false, nil }
func sanyRegion88744(r *recognizer, f *FrameAssumeProve) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("PROVE or `,'")
	return false, nil
}
func sanyRegion88806(r *recognizer, f *FrameAssumeProve) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("Expression, Declaration, or AssumeProve")
	return false, nil
}
func sanyRegion88920(r *recognizer) (bool, error) { return sanyAssumeProveAhead(r) }
func sanyRegion89455(r *recognizer, f *FrameAssumeProve) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("PROVE or `,'")
	return false, nil
}
func sanyRegion89539(r *recognizer, f *FrameAssumeProve) (bool, error) {
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
func sanyRegion89627(r *recognizer, f *FrameAssumeProve) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion89648(r *recognizer, f *FrameAssumeProve) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	f.Sn = sn
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_AssumeProve, f.Sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
