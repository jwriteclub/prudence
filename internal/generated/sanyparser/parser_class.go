// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license; see NOTICE.
package generated

func initSANYClass(r *recognizer) error {
	state, err := NewSANYState(nil)
	if err != nil {
		return err
	}
	state.OperatorStack.Check = r.c.Check
	r.state = state
	return nil
}
