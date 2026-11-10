// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import "testing"

// ASTConstants.java:117-170; Context.java:573-581 initializes the complete
// BuiltInOperators.Properties array before first Generator level entry.
func TestEnabledBootstrapInternsAllASTConstantNames(t *testing.T) {
	ast := []string{"$AngleAct", "$BoundedChoose", "$BoundedExists", "$BoundedForall", "$Case", "$CartesianProd", "$ConjList", "$DisjList", "$Except", "$FcnApply", "$FcnConstructor", "$IfThenElse", "$NonRecursiveFcnSpec", "$Pair", "$RcdConstructor", "$RcdSelect", "$RecursiveFcnSpec", "$Seq", "$SquareAct", "$SetEnumerate", "$SF", "$SetOfAll", "$SetOfRcds", "$SetOfFcns", "$SubsetOf", "$Tuple", "$TemporalExists", "$TemporalForall", "$UnboundedChoose", "$UnboundedExists", "$UnboundedForall", "$WF", "$Nop", "$Qed", "$Pfcase", "$Have", "$Take", "$Pick", "$Witness", "$Suffices", "'"}
	builtins := map[string]bool{}
	for _, d := range builtinDescriptors() {
		builtins[d.text] = true
	}
	if len(ast) != 41 || len(builtinDescriptors()) != 72 {
		t.Fatal("pinned source inventory changed")
	}
	for _, name := range ast {
		if !builtins[name] {
			t.Errorf("ASTConstants name not pre-interned: %q", name)
		}
	}
}
