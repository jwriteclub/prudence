//go:build prudence_phase2_adapter

// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestTypedLevelDiagnosticsRejectsUnknownAndPreservesTypes(t *testing.T) {
	code, _ := CodeFromValue(4206)
	d := Detail{Code: code, Location: Location{Name: "M", NamePresent: true, BeginLine: 1, BeginColumn: 2, EndLine: 1, EndColumn: 3}, Format: "%s %s", Parameters: []Parameter{{Kind: StringParameter, Text: "x"}, {Kind: IntParameter, Integer: 2}}}
	b, e := Phase2TypedLevelDiagnostics([]Detail{d})
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Parameters []struct {
			Type string `json:"type"`
		} `json:"parameters"`
		Rendered string `json:"rendered"`
	}
	if e = json.Unmarshal(b, &rows); e != nil || len(rows) != 1 || rows[0].Parameters[0].Type != "java.lang.String" || rows[0].Parameters[1].Type != "java.lang.Integer" || rows[0].Rendered != "x 2" {
		t.Fatalf("%s %v", b, e)
	}
	d.Parameters[0] = Parameter{Kind: ObjectIdentityParameter, Identity: 1, Text: "x"}
	if _, e = Phase2TypedLevelDiagnostics([]Detail{d}); e != ErrPortIncomplete {
		t.Fatal(e)
	}
	d.Parameters[0] = Parameter{Kind: StringParameter, UTF16Present: true, UTF16Units: []uint16{0xd800}, TextUnrepresentable: true}
	b, e = Phase2TypedLevelDiagnostics([]Detail{d})
	if e != nil || !bytes.Contains(b, []byte(`\ud800`)) {
		t.Fatalf("%s %v", b, e)
	}
}

func TestLevelStageTypeProvenance(t *testing.T) {
	code, _ := CodeFromValue(4245)
	p := newNamePool()
	d := Detail{Code: code, Location: Location{Name: "M", NamePresent: true}, Format: "Level error in instantiating module '%s':\nThe level of the expression or operator substituted for '%s' \nmust be at most %s.", Parameters: []Parameter{nameParam(p.intern("A")), nameParam(p.intern("K")), {Kind: IntParameter, Integer: 1}}}
	b, e := Phase2TypedLevelStageDiagnostics([]Detail{d})
	if e != nil || !bytes.Contains(b, []byte(`util.UniqueString`)) || !bytes.Contains(b, []byte(`java.lang.Integer`)) {
		t.Fatalf("%s %v", b, e)
	}
	d.Parameters[1] = Parameter{Kind: StringParameter, Text: "K"}
	if _, e = Phase2TypedLevelStageDiagnostics([]Detail{d}); e != ErrPortIncomplete {
		t.Fatalf("class mismatch: %v", e)
	}
	d.Parameters[1] = nameParam(p.intern("K"))
	d.Parameters[1].JavaClass = "java.lang.String"
	if _, e = Phase2TypedLevelStageDiagnostics([]Detail{d}); e != ErrPortIncomplete {
		t.Fatalf("contradictory mark: %v", e)
	}
	d.Parameters[1] = nameParam(p.intern("K"))
	d.Format += "!"
	if _, e = Phase2TypedLevelStageDiagnostics([]Detail{d}); e != ErrPortIncomplete {
		t.Fatalf("unknown source format: %v", e)
	}
}

func TestGenerationStageSourceTypesAndOrder(t *testing.T) {
	p := newNamePool()
	code, _ := CodeFromValue(4243)
	d := Detail{Code: code, Location: Location{Name: "M", NamePresent: true}, Format: "An operator must be substituted for symbol '%s', and it must have arity %s.", Parameters: []Parameter{nameParam(p.intern("K")), {Kind: IntParameter, Integer: 2}}}
	b, err := Phase2TypedGenerationStageDiagnostics([]Detail{d})
	if err != nil || !bytes.Contains(b, []byte("util.UniqueString")) || !bytes.Contains(b, []byte("java.lang.Integer")) {
		t.Fatalf("%s %v", b, err)
	}
	d.Parameters[0], d.Parameters[1] = d.Parameters[1], d.Parameters[0]
	if _, err = Phase2TypedGenerationStageDiagnostics([]Detail{d}); err != ErrPortIncomplete {
		t.Fatalf("reordered: %v", err)
	}
	d.Parameters[0], d.Parameters[1] = nameStringParam(p.intern("K")), Parameter{Kind: IntParameter, Integer: 2}
	if _, err = Phase2TypedGenerationStageDiagnostics([]Detail{d}); err != ErrPortIncomplete {
		t.Fatalf("wrong class: %v", err)
	}
	code, _ = CodeFromValue(4201)
	d = Detail{Code: code, Location: d.Location, Format: "Operator %s already defined or declared.", Parameters: []Parameter{strParam("K")}}
	if _, err = Phase2TypedGenerationStageDiagnostics([]Detail{d}); err != ErrPortIncomplete {
		t.Fatalf("unknown text provenance: %v", err)
	}
	d.Parameters[0] = nameStringParam(p.intern("K"))
	if _, err = Phase2TypedGenerationStageDiagnostics([]Detail{d}); err != nil {
		t.Fatal(err)
	}
}
