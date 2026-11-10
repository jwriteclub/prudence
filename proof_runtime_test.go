// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package prudence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	parser "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/parserrt"
)

// Proof-runtime chapter. On these recorded cases stock SANY's proof-step
// handling throws java.lang.NumberFormatException (a step level beyond int
// range) and prints "Fatal errors while parsing"; the oracle is a crash, not
// a verdict. The translated parser returns its errProofRuntime sentinel
// through the callback wrapper. Parse and Check report one CodeIncomplete
// error with no location and CompletionIncomplete, with a nil error.

// proofRuntimeCases are sany-conformance fuzz-jazzer cases, verbatim under
// testdata/proof-runtime (provenance in PROVENANCE); sum is the manifest's
// SHA-256 of Test.tla.
var proofRuntimeCases = []struct{ set, id, sum string }{
	{"deep-cuts/fuzz-jazzer", "D-6a550fecb6ab", "14162b0a29e80345d4e1da6ce3cf4f39ca4265f5b2b13c61adc11292e7f24aae"},
	{"deep-cuts/fuzz-jazzer", "D-cc6f40e2dfb6", "52976242b91a8e189b95751a6973288a2e0de407bfed27fe01050ebdc02a16c5"},
	{"fuzz-jazzer", "D-5ba7194164bc", "54a464cefdeeb0e80e63e31bdcc13b968f9904fa14d9b181a1d990e360201069"},
}

// Authored controls: one level past Java's int range, and the largest level
// Java accepts.
const (
	proofLevelOverflow = "---- MODULE P ----\nTHEOREM TRUE\n<2147483648>1. TRUE\n====\n"
	proofLevelIntMax   = "---- MODULE P ----\nTHEOREM TRUE\n<2147483647>1. TRUE\n====\n"
	proofValid         = "---- MODULE P ----\nTHEOREM TRUE\n<1>1. TRUE\n  OBVIOUS\n<1>2. QED\n  BY <1>1\n====\n"
	proofSyntaxReject  = "---- MODULE P ----\nTHEOREM TRUE\n<1>1. TRUE\n  BY BY\n====\n"
)

func proofRuntimeFixture(t *testing.T, id, sum string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "proof-runtime", id, "Test.tla"))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != sum {
		t.Fatalf("%s is not the recorded case bytes", id)
	}
	return data
}

// requireProofRuntimeIncomplete requires the clean failure: nil error,
// syntax level, incomplete, no tree, and exactly the one location-less
// CodeIncomplete error.
func requireProofRuntimeIncomplete(t *testing.T, label string, r Result, err error, path string) {
	t.Helper()
	if err != nil || r.Completion != CompletionIncomplete || r.Level != LevelSyntax || r.Tree != nil {
		t.Fatalf("%s: %v %+v", label, err, r)
	}
	want := []Diagnostic{{Phase: LevelSyntax, Severity: SeverityError, Code: CodeIncomplete, Message: proofRuntimeIncomplete, Path: path}}
	if !reflect.DeepEqual(r.Diagnostics, want) {
		t.Fatalf("%s: diagnostics %+v", label, r.Diagnostics)
	}
}

// proofRuntimeAll runs Parse and Check at every stage on one source.
func proofRuntimeAll(t *testing.T, path string, src []byte) {
	t.Helper()
	original := append([]byte(nil), src...)
	r, err := Parse(context.Background(), path, src, Options{})
	requireProofRuntimeIncomplete(t, "Parse", r, err, path)
	again, err := Parse(context.Background(), path, src, Options{})
	requireProofRuntimeIncomplete(t, "Parse again", again, err, path)
	if !reflect.DeepEqual(r, again) {
		t.Fatal("repeated Parse differs")
	}
	for _, stage := range []AnalysisLevel{"", LevelSyntax, LevelSemantic, LevelComplete} {
		c, err := Check(context.Background(), Request{Stage: stage, Origin: OriginWorkspace, Path: path, Text: src})
		requireProofRuntimeIncomplete(t, "Check "+string(stage), c, err, path)
	}
	if !bytes.Equal(src, original) {
		t.Fatal("caller source modified")
	}
}

func TestProofRuntimeCorpusCases(t *testing.T) {
	for _, c := range proofRuntimeCases {
		t.Run(c.id, func(t *testing.T) {
			src := proofRuntimeFixture(t, c.id, c.sum)
			raw, err := parser.Parse(context.Background(), src, parser.ParseOptions{})
			var op *parserrt.OperationalError
			if !errors.As(err, &op) || !parser.IsProofRuntimeFailure(err) || err.Error() != "binding callback: sany proof: Java runtime failure" || raw.Accepted {
				t.Fatalf("raw parse changed: %#v %+v", err, raw)
			}
			proofRuntimeAll(t, "Test.tla", src)
		})
	}
}

func TestProofRuntimeSyntheticControls(t *testing.T) {
	proofRuntimeAll(t, "P.tla", []byte(proofLevelOverflow))

	r, err := Parse(context.Background(), "P.tla", []byte(proofLevelIntMax), Options{})
	if err != nil || firstDiagnostic(r.Diagnostics, CodeIncomplete) != nil {
		t.Fatalf("int max level: %v %+v", err, r)
	}

	ok, err := Check(context.Background(), Request{Path: "P.tla", Text: []byte(proofValid)})
	requireCompleteCheck(t, ok, err, LevelComplete)
	for _, d := range ok.Diagnostics {
		if d.Severity == SeverityError {
			t.Fatalf("valid proof: %+v", ok.Diagnostics)
		}
	}

	for _, stage := range []AnalysisLevel{"", LevelSyntax} {
		bad, err := Check(context.Background(), Request{Stage: stage, Path: "P.tla", Text: []byte(proofSyntaxReject)})
		if err != nil || bad.Completion != CompletionComplete || bad.Level != LevelSyntax || firstDiagnostic(bad.Diagnostics, CodeSyntax) == nil || firstDiagnostic(bad.Diagnostics, CodeIncomplete) != nil {
			t.Fatalf("syntax rejection %q: %v %+v", stage, err, bad)
		}
	}
}

func TestProofRuntimeDepthUnchanged(t *testing.T) {
	src := proofRuntimeFixture(t, proofRuntimeCases[0].id, proofRuntimeCases[0].sum)
	opts := Options{MaxDepth: 2}
	r, err := Parse(context.Background(), "Test.tla", src, opts)
	if err != nil || r.Completion != CompletionIncomplete || firstDiagnostic(r.Diagnostics, CodeDepthLimit) == nil || firstDiagnostic(r.Diagnostics, CodeIncomplete) != nil {
		t.Fatalf("Parse depth: %v %+v", err, r)
	}
	c, err := Check(context.Background(), Request{Path: "Test.tla", Text: src, Options: opts})
	if err != nil || c.Completion != CompletionIncomplete || firstDiagnostic(c.Diagnostics, CodeDepthLimit) == nil || firstDiagnostic(c.Diagnostics, CodeIncomplete) != nil {
		t.Fatalf("Check depth: %v %+v", err, c)
	}
}

// proofCountdown reports cancellation from its n+1st Err call on.
type proofCountdown struct {
	context.Context
	left atomic.Int64
}

func newProofCountdown(n int64) *proofCountdown {
	c := &proofCountdown{Context: context.Background()}
	c.left.Store(n)
	return c
}
func (c *proofCountdown) Err() error {
	if c.left.Add(-1) >= 0 {
		return nil
	}
	return context.Canceled
}

// TestProofRuntimeCancellation cancels at every context poll in turn. Every
// cancelled run returns context.Canceled and no diagnostic; in particular the
// run cancelled only at the final poll, after the parser already returned the
// proof failure, still reports cancellation.
func TestProofRuntimeCancellation(t *testing.T) {
	src := proofRuntimeFixture(t, proofRuntimeCases[2].id, proofRuntimeCases[2].sum)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Parse(cancelled, "Test.tla", src, Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := Check(cancelled, Request{Path: "Test.tla", Text: src}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Check: %v", err)
	}
	for name, run := range map[string]func(context.Context) (Result, error){
		"Parse": func(ctx context.Context) (Result, error) { return Parse(ctx, "Test.tla", src, Options{}) },
		"Check": func(ctx context.Context) (Result, error) {
			return Check(ctx, Request{Path: "Test.tla", Text: src})
		},
	} {
		for n := int64(0); ; n++ {
			if n > 100000 {
				t.Fatalf("%s never completed", name)
			}
			r, err := run(newProofCountdown(n))
			if err == nil {
				if n == 0 {
					t.Fatalf("%s: no cancelled run", name)
				}
				requireProofRuntimeIncomplete(t, name, r, err, "Test.tla")
				break
			}
			if !errors.Is(err, context.Canceled) || r.Completion != CompletionIncomplete || len(r.Diagnostics) != 0 {
				t.Fatalf("%s n=%d: %v %+v", name, n, err, r)
			}
		}
	}
}
