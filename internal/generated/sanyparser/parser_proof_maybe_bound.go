// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
package generated

func sanyRegion93100(r *recognizer, f *FrameMaybeBound) error {
	f.Zn = nil
	return r.state.bpa(r, r.state.UniqueStringOf("Domain binding"))
}
func sanyRegion93205(r *recognizer, f *FrameMaybeBound) (bool, error) {
	f.Zn = make([]*SyntaxTreeNode, 2)
	store := sanyNodeSlot(f.Zn, 0)
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	if e = store(n); e != nil {
		return false, e
	}
	f.Zn[0].SetKind(T_IN)
	r.state.expecting = r.state.UniqueStringOf("Expression")
	return false, nil
}
func sanyRegion93345(_ *recognizer, f *FrameMaybeBound) (func(*SyntaxTreeNode) error, error) {
	return sanyNodeSlot(f.Zn, 1), nil
}
func sanyRegion93371(r *recognizer, f *FrameMaybeBound) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_MaybeBound, f.Zn)
	if e != nil {
		return false, e
	}
	f.Result = n
	return true, nil
}
