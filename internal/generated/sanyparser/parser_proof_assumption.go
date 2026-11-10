// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
package generated

func sanyRegion84060(r *recognizer, f *FrameAssumption) error {
	f.Zn = nil
	if e := r.state.bpa(r, r.state.UniqueStringOf("Assumption")); e != nil {
		return e
	}
	r.state.expecting = r.state.UniqueStringOf("ASSUM...")
	return nil
}
func sanyRegion84300(r *recognizer, f *FrameAssumption) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion84491(r *recognizer, f *FrameAssumption) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("==")
	return false, nil
}
func sanyRegion84549(r *recognizer, f *FrameAssumption) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion84776(r *recognizer, _ *FrameAssumption) (bool, error) {
	if e := BelchDEF(r, r.c.Check); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("Expression")
	return false, nil
}
func sanyRegion84842(r *recognizer, f *FrameAssumption) (bool, error) {
	return false, r.state.addHeir(f.Tn)
}
func sanyRegion84861(r *recognizer, f *FrameAssumption) (bool, error) {
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	if e = r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNodeOptional(r.state.mn, N_Assumption, f.Zn, sn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
