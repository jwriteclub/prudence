// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
// Source SHA-256: bfc981f31c205333f478b773ab108159b5e2d339b659d33b0f920d56ebf4253d
package generated

import (
	"errors"
	"github.com/jwriteclub/prudence/internal/lexer"
)

// Each no-op declaration contains only the uninitialized Java local Token t.
func sanyRegion59721(*recognizer, *FramePrefixOpToken) error   { return nil }
func sanyRegion60248(*recognizer, *FrameNEPrefixOpToken) error { return nil }
func sanyRegion60531(*recognizer, *FrameInfixOpToken) error    { return nil }
func sanyRegion63880(*recognizer, *FramePostfixOpToken) error  { return nil }
func sanyRegion82732(*recognizer, *FramePrefixOp) error        { return nil }
func sanyRegion82955(*recognizer, *FrameNonExpPrefixOp) error  { return nil }
func sanyRegion83423(*recognizer, *FramePostfixOp) error       { return nil }
func sanyRegion83646(*recognizer, *FrameIdentifier) error      { return nil }

func sanyRegion83179(r *recognizer, _ *FrameInfixOp) error {
	return r.state.bpa(r, r.state.UniqueStringOf("Infix Op"))
}
func sanyRegion59897(_ *recognizer, f *FramePrefixOpToken) (bool, error) {
	f.Result = f.T
	return true, nil
}
func sanyRegion60491(_ *recognizer, f *FrameNEPrefixOpToken) (bool, error) {
	f.Result = f.T
	return true, nil
}
func sanyRegion63838(_ *recognizer, f *FrameInfixOpToken) (bool, error) {
	f.Result = f.T
	return true, nil
}
func sanyRegion63957(_ *recognizer, f *FramePostfixOpToken) (bool, error) {
	f.Result = f.T
	return true, nil
}

// lookupImageOperator evaluates the Java t.image dereference before lookup.
// It deliberately does not change lastOp on error.
func (ctx *SupportContext) lookupImageOperator(t *lexer.Token) (*Operator, error) {
	if t == nil {
		return nil, errors.New("sany: nil operator token")
	}
	return ctx.LookupOperator(ctx.UniqueStringOfUnits(tokenUnits(t)))
}
func sanyRegion82770(r *recognizer, f *FramePrefixOp) (bool, error) {
	op, err := r.state.lookupImageOperator(f.T)
	if err != nil {
		return false, err
	}
	r.state.lastOp = op
	n, err := r.state.NewTokenNodeKind(r.state.mn, N_PrefixOp, f.T)
	if err != nil {
		return false, err
	}
	f.Result = n
	return true, nil
}
func sanyRegion82995(r *recognizer, f *FrameNonExpPrefixOp) (bool, error) {
	op, err := r.state.lookupImageOperator(f.T)
	if err != nil {
		return false, err
	}
	r.state.lastOp = op
	n, err := r.state.NewTokenNodeKind(r.state.mn, N_NonExpPrefixOp, f.T)
	if err != nil {
		return false, err
	}
	f.Result = n
	return true, nil
}
func sanyRegion83234(r *recognizer, f *FrameInfixOp) (bool, error) {
	op, err := r.state.lookupImageOperator(f.T)
	if err != nil {
		return false, err
	}
	r.state.lastOp = op
	if err = r.state.epa(); err != nil {
		return false, err
	}
	n, err := r.state.NewTokenNodeKind(r.state.mn, N_InfixOp, f.T)
	if err != nil {
		return false, err
	}
	f.Result = n
	return true, nil
}
func sanyRegion83462(r *recognizer, f *FramePostfixOp) (bool, error) {
	op, err := r.state.lookupImageOperator(f.T)
	if err != nil {
		return false, err
	}
	r.state.lastOp = op
	n, err := r.state.NewTokenNodeKind(r.state.mn, N_PostfixOp, f.T)
	if err != nil {
		return false, err
	}
	f.Result = n
	return true, nil
}
func sanyRegion83681(r *recognizer, f *FrameIdentifier) (bool, error) {
	n, err := r.state.NewTokenNode(r.state.mn, f.T)
	if err != nil {
		return false, err
	}
	f.Result = n
	return true, nil
}
