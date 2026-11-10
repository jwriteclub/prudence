// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package modulegraph

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPCalValidationFails(t *testing.T) {
	const tr = "\\* BEGIN TRANSLATION\n\\* END TRANSLATION\n"
	for _, tc := range []struct {
		name, before, body string
		want               bool
	}{
		{"no algorithm word", "", "x == \"open\n" + tr, false},
		{"unterminated string after marker", "", "x == \"--algorithm foo\"\n" + tr, true},
		{"no translation markers", "", "x == \"--algorithm foo\"\n", false},
		{"only end marker", "", "x == \"--algorithm foo\"\n\\* END TRANSLATION\n", true},
		{"terminated string", "", "(* --algorithm a \"done\" *)\n" + tr, false},
		{"escaped quote", "", "(* --algorithm a \"x\\\"y\" *)\n" + tr, false},
		{"backslash ends line in string", "", "(* --algorithm a \"x\\\n*)\n" + tr, true},
		{"end-of-line comment", "", "(* --algorithm a \\* \"x\n*)\n" + tr, false},
		{"nested comment", "", "(* --algorithm a (* \"x *)\n*)\n" + tr, false},
		{"stops at unmatched close", "", "(* --algorithm a *) \"x\n" + tr, false},
		{"fair algorithm", "", "(* --fair algorithm a \"x\n*)\n" + tr, true},
		// Validator.java:99 requires "algorithm" after column 0 of some line.
		{"algorithm only at column 0", "", "algorithm == 1\n(* --fair x \"y\n*)\n" + tr, false},
		{"algorithm after column 0", "", " algorithm == 1\n(* --fair x \"y\n*)\n" + tr, true},
		// Lines outside the root module's location are never read.
		{"marker before module", "x == \"--algorithm foo\n" + tr, "y == 1\n", false},
		{"string closed on next line only", "", "(* --algorithm a \"x\n\" *)\n" + tr, true},
		// Validator.java:126 detabs before findTokenPair (Validator.java:201,
		// 207), so tabs between the marker words count as spaces.
		{"tab markers", "", "x == \"--algorithm foo\"\n\\* BEGIN\tTRANSLATION\n\\* END\tTRANSLATION\n", true},
		{"mixed tab and space markers", "", "x == \"--algorithm foo\"\n\\* BEGIN \t TRANSLATION\n\\* END\t\t  TRANSLATION\n", true},
		{"tab end marker only", "", "x == \"--algorithm foo\"\n\\* END\tTRANSLATION\n", true},
		{"tab then other text", "", "x == \"--algorithm foo\"\n\\* BEGIN\tX TRANSLATION\n\\* END\tX TRANSLATION\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := tc.before + "---- MODULE M ----\n" + tc.body + "====\n"
			begin := strings.Count(tc.before, "\n") + 1
			end := strings.Count(text, "\n")
			if got := pcalValidationFails([]byte(text), begin, end); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			crlf := strings.ReplaceAll(text, "\n", "\r\n")
			if got := pcalValidationFails([]byte(crlf), begin, end); got != tc.want {
				t.Fatalf("CRLF: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestAcquisitionFileIdentityAndValidation(t *testing.T) {
	lim := AcquisitionLimits{MaxDepth: 1000, MaxVisits: 100000, MaxUnits: 100, MaxEdges: 1000, MaxNodes: 100000, MaxSourceBytes: 1 << 20, MaxTotalBytes: 8 << 20}
	none := func(context.Context, string) (Input, error) { return Input{}, ErrResolverUnavailable }
	// Without a file identity the root module name is not compared.
	if _, err := AcquireFlatExternal(context.Background(), Input{Path: "x", Text: []byte("---- MODULE A ----\n====\n")}, none, lim); err != nil {
		t.Fatal(err)
	}
	_, err := AcquireFlatExternal(context.Background(), Input{Path: "x", FileName: "B", Text: []byte("---- MODULE A ----\n====\n")}, none, lim)
	var mismatch *FileNameMismatchError
	if !errors.As(err, &mismatch) || mismatch.FileName != "B" || mismatch.ModuleName != "A" || mismatch.Path != "x" {
		t.Fatalf("root: %v", err)
	}
	// A dependency's identity is the requested name; the root's FileName is
	// not inherited and the resolver's label is only reported.
	resolve := func(_ context.Context, name string) (Input, error) {
		return Input{Path: "label", FileName: name, Text: []byte("---- MODULE Other ----\n====\n")}, nil
	}
	acq, err := AcquireFlatExternal(context.Background(), Input{Path: "R", FileName: "R", Text: []byte("---- MODULE R ----\nEXTENDS Dep\n====\n")}, resolve, lim)
	if !errors.As(err, &mismatch) || mismatch.FileName != "Dep" || mismatch.ModuleName != "Other" || mismatch.Path != "label" || acq == nil || acq.Request == nil || acq.Request.Name != "Dep" {
		t.Fatalf("dependency: %v %+v", err, acq)
	}
	// Validation covers the root and EXTENDS targets, once each, in order;
	// an INSTANCE target is not validated.
	// The marker sits in a TLA+ string; uncomment then sees an open one.
	body := "x == \"--algorithm foo\"\n\\* BEGIN TRANSLATION\n\\* END TRANSLATION\n====\n"
	files := map[string]string{"E": "---- MODULE E ----\n" + body, "I": "---- MODULE I ----\n" + body}
	resolve = func(_ context.Context, name string) (Input, error) {
		return Input{Path: name, Text: []byte(files[name])}, nil
	}
	root := "---- MODULE Root ----\nEXTENDS E\nJ == INSTANCE I\n" + body
	acq, err = AcquireFlatExternal(context.Background(), Input{Path: "Root", Text: []byte(root)}, resolve, lim)
	if err != nil || len(acq.Validation) != 2 || acq.Validation[0].Name != "Root" || acq.Validation[1].Name != "E" {
		t.Fatalf("%v %+v", err, acq)
	}
}
