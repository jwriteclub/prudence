// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
package generated

func sanyRegion98756(r *recognizer, f *FrameProof) error {
	f.T = nil
	f.T0 = nil
	if e := r.state.ProofState.Push(); e != nil {
		return e
	}
	return r.state.bpa(r, r.state.UniqueStringOf("Proof"))
}
func sanyRegion99950(r *recognizer) (bool, error) {
	t, e := r.Token(1)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errProofBindingToken
	}
	if t.Kind == tokenBy {
		return true, nil
	}
	t, e = r.Token(2)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errProofBindingToken
	}
	return t.Kind == tokenBy, nil
}
func sanyRegion100107(r *recognizer, f *FrameProof) (bool, error) {
	var sn []*SyntaxTreeNode
	if f.T0 != nil {
		sn = make([]*SyntaxTreeNode, 2)
		n, e := r.state.NewTokenNode(r.state.mn, f.T0)
		if e != nil {
			return false, e
		}
		sn[0] = n
		n, e = r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		sn[1] = n
	} else {
		sn = make([]*SyntaxTreeNode, 1)
		n, e := r.state.NewTokenNode(r.state.mn, f.T)
		if e != nil {
			return false, e
		}
		sn[0] = n
	}
	n, e := r.state.NewNode(r.state.mn, N_TerminalProof, sn)
	if e != nil {
		return false, e
	}
	f.Tn = n
	return false, nil
}
func sanyRegion100474(r *recognizer, f *FrameProof) (bool, error) {
	n, e := r.state.NewTokenNode(r.state.mn, f.T)
	if e != nil {
		return false, e
	}
	return false, r.state.addHeir(n)
}
func sanyRegion100532(r *recognizer) (bool, error) {
	t, e := r.Token(2)
	if e != nil {
		return false, e
	}
	if t == nil {
		return false, errProofBindingToken
	}
	return t.Kind != sanyTok_QED, nil
}
func sanyRegion100580(r *recognizer, f *FrameProof) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	r.state.expecting = r.state.UniqueStringOf("a proof step")
	return false, nil
}
func sanyRegion100671(r *recognizer, f *FrameProof) (bool, error) {
	if e := r.state.addHeir(f.Tn); e != nil {
		return false, e
	}
	sn, e := r.state.getLastHeirs(r.c.Check)
	if e != nil {
		return false, e
	}
	n, e := r.state.NewNode(r.state.mn, N_Proof, sn)
	if e != nil {
		return false, e
	}
	f.Tn = n
	level, e := r.state.ProofState.Get()
	if e != nil {
		return false, e
	}
	f.Tn.SetProofLevel(level)
	return false, nil
}
func sanyRegion100830(r *recognizer, f *FrameProof) (bool, error) {
	if e := r.state.epa(); e != nil {
		return false, e
	}
	if e := r.state.ProofState.Pop(); e != nil {
		return false, e
	}
	f.Result = f.Tn
	return true, nil
}
