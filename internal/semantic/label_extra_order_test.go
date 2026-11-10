// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"slices"
	"testing"
)

// labelExtraRun analyzes one module body and returns its rendered errors in
// emission order.
func labelExtraRun(t *testing.T, body string) (*generator, []string, error) {
	t.Helper()
	g, _, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\n"+body+"\n====\n"), 0)
	var out []string
	for _, d := range g.table.errors.Messages() {
		text, e := d.Render()
		if e != nil {
			t.Fatal(e)
		}
		out = append(out, text)
	}
	return g, out, err
}

// Outside the bounded cursor, distinct-name residuals print every name once
// in UTF-16 lexical order (the approved fallback; Generator.java:7092-7097 only appends
// names). Every other error keeps its source text and position in the
// sequence: illegal parameters are dummy formals (6900-6920), body errors
// come first, a repeated formal is 4330 and the label is added afterwards
// (6933-6945).
func TestLabelExtrasFallbackOrder(t *testing.T) {
	illegal := func(p string) string { return "Illegal parameter " + p + " of label `L'." }
	tests := []struct {
		name, body string
		want       []string
	}{
		{"full list in UTF-16 order", "F == L(zz, a, Mm) :: TRUE", []string{
			illegal("zz"), illegal("a"), illegal("Mm"),
			"Label L declares extra parameter(s)  Mm  a  zz  ",
		}},
		{"five names each once", "F == L(e, d, c, b, a) :: TRUE", []string{
			illegal("e"), illegal("d"), illegal("c"), illegal("b"), illegal("a"),
			"Label L declares extra parameter(s)  a  b  c  d  e  ",
		}},
		{"bound parameters removed first", "F == \\A x, y : L(x, q, y, p) :: TRUE", []string{
			illegal("q"), illegal("p"),
			"Label L declares extra parameter(s)  p  q  ",
		}},
		{"header formals beyond the cursor", "F(_ + _, b, c, d, e, f, g, h, x, y) == L(y, x) :: TRUE", []string{
			"Label L declares extra parameter(s)  x  y  ",
		}},
		{"repeated formal keeps 4330", "F(_ + _, a, b) == L(b, a, b) :: TRUE", []string{
			"Repeated formal parameter b \nin label `L'.",
			"Label L declares extra parameter(s)  a  b  ",
		}},
		{"errors continue after the list", "F == /\\ L(b, a) :: b\n     /\\ L :: FALSE\nG == Undef", []string{
			"Unknown operator: `b'.", illegal("b"), illegal("a"),
			"Label L declares extra parameter(s)  a  b  ",
			"Duplicate label `L'.",
			"Unknown operator: `Undef'.",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, got, err := labelExtraRun(t, tt.body)
			if err != nil {
				t.Fatalf("%v (site %q)", err, g.incomplete.site)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("errors:\n%q\nwant\n%q", got, tt.want)
			}
			if g.labelOrderFallbacks != 1 {
				t.Fatalf("%d fallback lists", g.labelOrderFallbacks)
			}
		})
	}
}

// The bounded cursor's exact Java order takes precedence over the fallback.
func TestLabelExtrasCursorPrecedence(t *testing.T) {
	body := "F(a,b,c,d,e,f,g,h,i,j) == L(a,j)::FALSE"
	g, got, err := labelExtraRun(t, body)
	if err != nil || g.labelOrderFallbacks != 0 {
		t.Fatalf("%v, fallbacks %d", err, g.labelOrderFallbacks)
	}
	if want := []string{"Label L declares extra parameter(s)  j  a  "}; !slices.Equal(got, want) {
		t.Fatalf("errors %q, want %q", got, want)
	}
	_, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, "---- MODULE M ----\n"+body+"\n====\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	label := testUserDefinitions(t, m.definitions)[0].body.(*labelNode)
	names, ok := labelExtrasStableOrder(map[*formalParamNode]bool{label.params[0]: true, label.params[1]: true})
	if !ok || !slices.Equal(names, []string{"a", "j"}) {
		t.Fatalf("fallback %q %v", names, ok)
	}
}

// Residuals mixing a repeated name with another name stay incomplete; the
// exception covers only distinct names. Same-name residuals are unchanged
// (TestGeneratorSameNameLabelResiduals).
func TestLabelExtrasMixedNamesStayIncomplete(t *testing.T) {
	for _, body := range []string{"F == L(x, x, y) :: TRUE", "F == L(y, x, y, z) :: TRUE"} {
		g, _, err := labelExtraRun(t, body)
		if err != ErrPortIncomplete || g.incomplete.site != "generator.construct.generate-label" {
			t.Fatalf("%s: err=%v site=%q", body, err, g.incomplete.site)
		}
	}
}

func TestLabelExtrasStableOrderUnits(t *testing.T) {
	pool := &namePool{}
	param := func(text string, exact bool) *formalParamNode {
		p := new(formalParamNode)
		p.name = &name{text: text, displayExact: exact, owner: pool}
		return p
	}
	// U+1F600 encodes as a surrogate pair (0xD83D ...), below U+FFFD in
	// UTF-16 although above it in UTF-8 byte order.
	high, low := string(rune(0x1F600)), string(rune(0xFFFD))
	if compareUTF16(high, low) >= 0 || high < low {
		t.Fatal("UTF-16 order not distinguished from byte order")
	}
	names, ok := labelExtrasStableOrder(map[*formalParamNode]bool{param(low, true): true, param(high, true): true, param("b", true): true})
	if !ok || !slices.Equal(names, []string{"b", high, low}) {
		t.Fatalf("order %q %v", names, ok)
	}
	if _, ok = labelExtrasStableOrder(map[*formalParamNode]bool{param("x", true): true, param("x", true): true, param("y", true): true}); ok {
		t.Fatal("repeated name admitted")
	}
	if _, ok = labelExtrasStableOrder(map[*formalParamNode]bool{param("x", false): true, param("y", true): true}); ok {
		t.Fatal("inexact display name admitted")
	}
}
