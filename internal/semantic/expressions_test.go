// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import "testing"

func TestLiteralConstructors(t *testing.T) {
	p := newNamePool()
	for _, tc := range []struct {
		image string
		small bool
		value int32
		big   string
	}{
		{"2147483647", true, 2147483647, ""}, {"2147483648", false, 0, "2147483648"}, {"\\HFFFFFFFF", false, 0, "4294967295"}, {"\\o17", true, 15, ""},
	} {
		n, e := newNumeral(p, tc.image, nil)
		if e != nil || n.image != tc.image || (n.bigValue == nil) != tc.small || n.value != tc.value {
			t.Fatalf("numeral %q: %#v %v", tc.image, n, e)
		}
		if !tc.small && n.bigValue.String() != tc.big {
			t.Fatal(n.bigValue)
		}
		n.levelCheck(2)
		n.levelCheck(1)
		if n.levels.checked != 1 {
			t.Fatal(n.levels.checked)
		}
	}
	d, e := newDecimal(p, "1", "200", nil)
	if e != nil || d.mantissa != 1200 || d.exponent != -3 || d.bigValue != nil {
		t.Fatal(d, e)
	}
	d, e = newDecimal(p, "9223372036854775808", "00", nil)
	if e != nil || d.bigValue.String() != "922337203685477580800" || d.scale != 2 || d.exponent != 0 {
		t.Fatal(d, e)
	}
}
func TestUserDefinitionConstructor(t *testing.T) {
	p := newNamePool()
	c := newContext(p)
	m, e := newModule(p, p.intern("M"), nil, c)
	if e != nil {
		t.Fatal(e)
	}
	body, e := newNumeral(p, "1", nil)
	if e != nil {
		t.Fatal(e)
	}
	param, e := newFormalParam(p, p.intern("x"), 0, nil, nil, m)
	if e != nil {
		t.Fatal(e)
	}
	params := []*formalParamNode{param}
	n, e := newUserOpDef(p, p.intern("F"), params, true, body, m, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	if n.body != body || n.module != m || !n.local || !n.defined || n.arity != 1 || n.maxLevels[0] != 3 || n.weights[0] != 0 || !n.isLeibniz || !n.isLeibnizArg[0] || n.levels.checked != 0 || nodeClassOf(n) != classOpDef {
		t.Fatal(n)
	}
	params[0] = nil
	if n.params[0] != nil {
		t.Fatal("parameter array was copied")
	}
	empty, e := newUserOpDef(p, p.intern("N"), nil, false, body, m, nil, nil)
	if e != nil || empty.params == nil || len(empty.params) != 0 {
		t.Fatal(empty, e)
	}
	if !sameOriginallyDefined(n, n) || sameOriginallyDefined(n, empty) {
		t.Fatal("source identity")
	}
}
