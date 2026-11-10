// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
package generated

func sanyRegion100918(r *recognizer, f *FrameUseOrHideOrBy) error {
	f.Kind = int32(N_UseOrHide)
	return r.state.bpa(r, r.state.UniqueStringOf("UseOrHideOrBy"))
}
func sanyRegion101791(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion101848(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	f.Kind = int32(N_TerminalProof)
	return false, nil
}
func sanyRegion101881(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion101937(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion102001(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion102057(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion102123(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion102170(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	r.state.expecting = r.state.UniqueStringOf("an expression, `MODULE' or `DEF'")
	return false, nil
}
func sanyRegion102248(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
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
func sanyRegion102372(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion102416(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion102457(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("MODULE or expression")
	return false, nil
}
func sanyRegion102585(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
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
func sanyRegion102713(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion102759(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion102790(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	if f.Kind == int32(N_TerminalProof) {
		r.state.expecting = r.state.UniqueStringOf("comma, DEF, or [.]")
	} else {
		r.state.expecting = r.state.UniqueStringOf("comma, DEF, or proof step")
	}
	return false, nil
}
func sanyRegion102954(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("MODULE or expression")
	return false, nil
}
func sanyRegion103075(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
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
func sanyRegion103202(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion103246(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion103287(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = r.state.addHeir(n); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("MODULE or expression")
	return false, nil
}
func sanyRegion103417(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
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
func sanyRegion103545(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion103591(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	return false, nil
}
func sanyRegion103623(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	if f.Kind == int32(N_TerminalProof) {
		r.state.expecting = r.state.UniqueStringOf("comma or [.]")
	} else {
		r.state.expecting = r.state.UniqueStringOf("comma or proof step")
	}
	return false, nil
}
func sanyRegion103765(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
	if f.Kind == int32(N_TerminalProof) {
		r.state.expecting = r.state.UniqueStringOf("[.]")
	}
	return false, nil
}
func sanyRegion103964(r *recognizer, f *FrameUseOrHideOrBy) (bool, error) {
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
