// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package prudence

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/jwriteclub/prudence/internal/standard"
)

func TestStandardProviderResourcesOwned(t *testing.T) {
	a := standard.Lookup("Naturals")
	if len(a) == 0 || standard.Lookup("../Naturals") != nil || standard.Lookup("Unknown") != nil {
		t.Fatal("lookup")
	}
	a[0] ^= 1
	if reflect.DeepEqual(a, standard.Lookup("Naturals")) {
		t.Fatal("resource shared between requests")
	}
}

func TestCheckStandardProviderComposition(t *testing.T) {
	root := []byte("---- MODULE Root ----\nEXTENDS Integers\nR == 1 + 2\n====\n")
	var calls []string
	resolver := func(_ context.Context, name string) (Source, error) {
		calls = append(calls, name)
		return Source{}, fmt.Errorf("missing: %w", ErrModuleNotFound)
	}
	r, err := Check(context.Background(), Request{Path: "Root.tla", Text: root, Resolve: resolver})
	if err != nil || r.Level != LevelComplete || r.Completion != CompletionComplete || len(r.Diagnostics) != 0 {
		t.Fatalf("%+v %v", r, err)
	}
	if !reflect.DeepEqual(calls, []string{"Integers", "Naturals"}) {
		t.Fatal(calls)
	}
	r, err = Check(context.Background(), Request{Text: root})
	if err != nil || r.Level != LevelComplete || r.Completion != CompletionComplete || len(r.Diagnostics) != 0 {
		t.Fatalf("%+v %v", r, err)
	}

	override := []byte("---- MODULE Naturals ----\nX == MissingOverride\n====\n")
	r, err = Check(context.Background(), Request{Text: []byte("---- MODULE Root ----\nEXTENDS Naturals\n====\n"), Resolve: func(_ context.Context, name string) (Source, error) {
		return Source{Path: "workspace", Text: override, Origin: OriginWorkspace}, nil
	}})
	if err != nil || r.Level != LevelSemantic {
		t.Fatalf("%+v %v", r, err)
	}
	found := false
	for _, d := range r.Diagnostics {
		if d.Code == "semantic/4200" && d.Path == "workspace" {
			found = true
		}
	}
	if !found {
		t.Fatal(r.Diagnostics)
	}

	failure := errors.New("operational")
	_, err = Check(context.Background(), Request{Text: root, Resolve: func(context.Context, string) (Source, error) { return Source{}, failure }})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err = Check(ctx, Request{Text: root, Resolve: func(context.Context, string) (Source, error) { cancel(); return Source{}, ErrModuleNotFound }})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	r, err = Check(context.Background(), Request{Text: []byte("---- MODULE Root ----\nEXTENDS Missing\n====\n"), Resolve: resolver})
	if err != nil || r.Completion != CompletionComplete || len(r.Diagnostics) != 1 || r.Diagnostics[0].Message != "Cannot find source file for module Missing imported in module Root." {
		t.Fatal(r, err)
	}
}

func TestNestedDiagnosticUsesOriginalBytes(t *testing.T) {
	text := []byte("---- MODULE Root ----\r\n\\* é😀\r\n---- MODULE Inner ----\r\n\tBad == Missing\r\n====\r\n====\r\n")
	r, err := Check(context.Background(), Request{Path: "same-label", Text: text})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range r.Diagnostics {
		if d.Code == "semantic/4200" {
			if d.Path != "same-label" || string(text[d.Span.Start.ByteOffset:d.Span.End.ByteOffset]) != "Missing" {
				t.Fatalf("bad diagnostic %+v", d)
			}
			return
		}
	}
	t.Fatal(r.Diagnostics)
}

func TestDiagnosticSourceIdentityWithEqualPathLabels(t *testing.T) {
	root := []byte("---- MODULE Root ----\nEXTENDS Dep\nR == MissingRoot\n====\n")
	dep := []byte("---- MODULE Dep ----\n\\* different bytes\nD == MissingDep\n====\n")
	r, err := Check(context.Background(), Request{Path: "same", Text: root, Resolve: func(context.Context, string) (Source, error) { return Source{Path: "same", Text: dep}, nil }})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, d := range r.Diagnostics {
		if d.Code != "semantic/4200" {
			continue
		}
		for _, tc := range []struct {
			text []byte
			word string
		}{{root, "MissingRoot"}, {dep, "MissingDep"}} {
			if d.Span.End.ByteOffset <= len(tc.text) && string(tc.text[d.Span.Start.ByteOffset:d.Span.End.ByteOffset]) == tc.word {
				seen[tc.word] = true
			}
		}
	}
	if !seen["MissingRoot"] || !seen["MissingDep"] {
		t.Fatal(r.Diagnostics)
	}
}
