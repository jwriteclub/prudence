// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ semantic error observations; see LICENSE.
package semantic

import "testing"

func TestObservedDedupSeverityAndAbort(t *testing.T) {
	e := new(Errors)
	err, _ := CodeFromValue(4200)
	warn, _ := CodeFromValue(4802)
	a, _ := e.Add(err, nil, "missing %s", Parameter{Kind: StringParameter, Text: "x"})
	b, _ := e.Add(err, nil, "missing %s", Parameter{Kind: StringParameter, Text: "x"})
	loc := Location{Name: "M", NamePresent: true, BeginLine: 1, BeginColumn: 2, EndLine: 1, EndColumn: 3}
	e.Add(err, &loc, "missing %s", Parameter{Kind: StringParameter, Text: "y"})
	e.Add(warn, nil, "field %s %s", Parameter{Kind: StringParameter, Text: "a"}, Parameter{Kind: StringParameter, Text: "b"})
	if a == b || a.Source != e || len(e.Messages()) != 3 || len(e.OfLevel(Error)) != 2 || len(e.OfLevel(Warning)) != 1 || e.Success() {
		t.Fatal("dedup/order/severity/abort mismatch")
	}
}
func TestIdentityIncludesTypedParamsAndFormat(t *testing.T) {
	e := new(Errors)
	c, _ := CodeFromValue(4200)
	for _, p := range []Parameter{{Kind: IntParameter, Integer: 1}, {Kind: LongParameter, Integer: 1}, {Kind: ObjectIdentityParameter, Identity: 1}, {Kind: ObjectIdentityParameter, Identity: 2}} {
		e.Add(c, nil, "%s", p)
	}
	e.Add(c, nil, "%s!", Parameter{Kind: IntParameter, Integer: 1})
	if len(e.Messages()) != 5 {
		t.Fatal("structured identities collapsed")
	}
}
func TestMessagesAreCopied(t *testing.T) {
	e := new(Errors)
	c, _ := CodeFromValue(4200)
	p := []Parameter{{Kind: StringParameter, Text: "x"}}
	e.Add(c, nil, "%s", p...)
	p[0].Text = "changed"
	got := e.Messages()
	got[0].Parameters[0].Text = "changed"
	if e.Messages()[0].Parameters[0].Text != "x" {
		t.Fatal("state escaped request")
	}
}
func TestBoundedMessageRendering(t *testing.T) {
	d := Detail{Format: "%s at %s", Parameters: []Parameter{{Kind: ObjectIdentityParameter, Identity: 1, Text: "op"}, {Kind: LocationParameter, Location: Location{Name: "M", NamePresent: true, BeginLine: 1, BeginColumn: 2, EndLine: 1, EndColumn: 3}}}}
	got, err := d.Render()
	if err != nil || got != "op at line 1, col 2 to line 1, col 3 of module M" {
		t.Fatalf("render: %q %v", got, err)
	}
	d = Detail{Format: "100% literal"}
	got, err = d.Render()
	if err != nil || got != d.Format {
		t.Fatal("zero-param format must be verbatim")
	}
	d = Detail{Format: "%s %s %s", Parameters: []Parameter{{Kind: IntParameter, Integer: -4}, {Kind: LongParameter, Integer: 2147483648}, {Kind: BoolParameter, Boolean: true}}}
	got, err = d.Render()
	if err != nil || got != "-4 2147483648 true" {
		t.Fatal("typed formatting", got, err)
	}
	d = Detail{Format: "%d", Parameters: []Parameter{{Kind: IntParameter, Integer: 4}}}
	if _, err = d.Render(); err != ErrPortIncomplete {
		t.Fatal("unsupported conversion must remain incomplete")
	}
}
func TestObjectIdentityAndLocationValue(t *testing.T) {
	e := new(Errors)
	c, _ := CodeFromValue(4200)
	l := Location{Name: "M", NamePresent: true, BeginLine: 1}
	e.Add(c, nil, "%s", Parameter{Kind: ObjectIdentityParameter, Identity: 1, Text: "x"})
	e.Add(c, nil, "%s", Parameter{Kind: ObjectIdentityParameter, Identity: 2, Text: "x"})
	e.Add(c, nil, "%s", Parameter{Kind: LocationParameter, Location: l})
	e.Add(c, nil, "%s", Parameter{Kind: LocationParameter, Location: l})
	if len(e.Messages()) != 3 {
		t.Fatal("identity/value comparison")
	}
}
func TestMalformedNullLocationAndInternConflict(t *testing.T) {
	e := new(Errors)
	c, _ := CodeFromValue(4200)
	bad := Location{Null: true, Name: "M", NamePresent: true, BeginLine: 1}
	if _, err := e.Add(c, &bad, "bad"); err != ErrPortIncomplete {
		t.Fatal("malformed nullLoc accepted")
	}
	if _, err := e.Add(c, nil, "%s", Parameter{Kind: LocationParameter, Location: bad}); err != ErrPortIncomplete {
		t.Fatal("malformed parameter location accepted")
	}
	if _, err := e.Add(c, nil, "%s", Parameter{Kind: ObjectIdentityParameter, Identity: 1, Text: "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Add(c, nil, "%s", Parameter{Kind: ObjectIdentityParameter, Identity: 1, Text: "y"}); err != ErrPortIncomplete {
		t.Fatal("conflicting intern text accepted")
	}
	if len(e.Messages()) != 1 {
		t.Fatal("invalid input changed messages")
	}
}

func TestNameParameterPreservesUnpairedUTF16(t *testing.T) {
	p := newNamePool()
	n := p.internUnits([]uint16{0xd800, 'x'})
	param := nameParam(n)
	e := new(Errors)
	if err := addError(e, 4200, Location{Null: true, Name: "--unknown--", NamePresent: true}, "Unknown operator: `%s'.", param); err != nil {
		t.Fatal(err)
	}
	param.UTF16Units[0] = 0
	message := e.Messages()[0]
	if !message.Parameters[0].UTF16Present || message.Parameters[0].UTF16Units[0] != 0xd800 || !message.Parameters[0].TextUnrepresentable {
		t.Fatal("raw name units lost")
	}
	message.Parameters[0].UTF16Units[0] = 0
	if e.Messages()[0].Parameters[0].UTF16Units[0] != 0xd800 {
		t.Fatal("diagnostic aliases caller")
	}
	if _, err := e.Messages()[0].Render(); err != ErrPortIncomplete {
		t.Fatal("lossy rendering", err)
	}
}

func TestExactStringParameterDistinguishesUnpairedUnits(t *testing.T) {
	p := newNamePool()
	a := nameStringParam(p.internUnits([]uint16{0xd800}))
	b := nameStringParam(p.internUnits([]uint16{0xd801}))
	if !a.UTF16Present || !a.TextUnrepresentable || sameParameter(a, b) {
		t.Fatal("lossy string comparison")
	}
	if !sameParameter(exactStringParam("plain"), strParam("plain")) {
		t.Fatal("equal valid Java strings")
	}
	if asciiStringParam("\u03bb").UTF16Present {
		t.Fatal("unproved non-ASCII text")
	}
}
