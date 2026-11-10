// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
package generated

const errProofBindingNode supportError = "sany: nil proof binding node"

func sanyRegion89766(r *recognizer, _ *FrameNewSymb) error {
	if e := r.state.bpa(r, r.state.UniqueStringOf("NEW symbol declaration")); e != nil {
		return e
	}
	r.state.expecting = r.state.UniqueStringOf("NEW, CONSTANT, VARIABLE, STATE, ACTION, or TEMPORAL")
	return nil
}
func sanyRegion90992(r *recognizer, f *FrameNewSymb) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion91059(r *recognizer, f *FrameNewSymb) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion91126(r *recognizer, f *FrameNewSymb) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion91193(r *recognizer, f *FrameNewSymb) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion91246(r *recognizer, _ *FrameNewSymb) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("Constant declaration")
	return false, nil
}
func sanyRegion91331(r *recognizer, f *FrameNewSymb) (bool, error) {
	if f.Tn == nil {
		return false, errProofBindingNode
	}
	f.HasArgs = len(f.Tn.Heirs()) > 1
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion91431(r *recognizer, f *FrameNewSymb) (bool, error) {
	if f.HasArgs {
		if f.Tn == nil {
			return false, errProofBindingNode
		}
		return false, reject("declared symbol with arguments before \\in at " + f.Tn.Location().String())
	}
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
func sanyRegion91798(r *recognizer, f *FrameNewSymb) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion91864(r *recognizer, f *FrameNewSymb) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion91933(r *recognizer, f *FrameNewSymb) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion91994(r *recognizer, f *FrameNewSymb) (bool, error) {
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
func sanyRegion92106(r *recognizer, f *FrameNewSymb) (bool, error) {
	sn := make([]*SyntaxTreeNode, 1)
	sn[0] = f.Tn
	n, e := r.state.NewNode(r.state.mn, N_IdentDecl, sn)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion92694(r *recognizer, f *FrameNewSymb) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion92795(r *recognizer, f *FrameNewSymb) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("Declaration")
	return false, nil
}
func sanyRegion92897(r *recognizer, f *FrameNewSymb) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion92940(r *recognizer, f *FrameNewSymb) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion92966(r *recognizer, f *FrameNewSymb) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_NewSymb, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
