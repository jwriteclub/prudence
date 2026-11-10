// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package prudence

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMonolithCheckAndResolverPrecedence(t *testing.T) {
	root := []byte("---- MODULE Root ----\nEXTENDS Wanted\n====\n---- MODULE Wanted ----\nBad == MissingName\n====\n")
	calls := 0
	r, e := Check(context.Background(), Request{Path: "Root.tla", Text: root, Resolve: func(context.Context, string) (Source, error) { calls++; return Source{}, ErrModuleNotFound }})
	if e != nil || calls != 1 {
		t.Fatalf("%v %d", e, calls)
	}
	found := false
	for _, d := range r.Diagnostics {
		if strings.Contains(d.Message, "MissingName") {
			found = true
			a, b := d.Span.Start.ByteOffset, d.Span.End.ByteOffset
			if d.Path != "Root.tla" || a < 0 || b > len(root) || !strings.Contains(string(root[a:b]), "MissingName") {
				t.Fatalf("ownership %+v", d)
			}
		}
	}
	if !found {
		t.Fatalf("missing semantic error: %+v", r.Diagnostics)
	}
	r, e = Check(context.Background(), Request{Path: "Root.tla", Text: root, Resolve: func(context.Context, string) (Source, error) {
		return Source{Path: "external.tla", Text: []byte("---- MODULE Wanted ----\nGood == 1\n====")}, nil
	}})
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range r.Diagnostics {
		if strings.Contains(d.Message, "MissingName") {
			t.Fatalf("fallback overrode resolver %+v", d)
		}
	}
	failure := errors.New("resolver broke")
	_, e = Check(context.Background(), Request{Text: root, Resolve: func(context.Context, string) (Source, error) { return Source{}, failure }})
	if !errors.Is(e, failure) {
		t.Fatalf("lost resolver error %v", e)
	}
}
func TestMonolithFailedParseOwnsRoot(t *testing.T) {
	root := []byte("---- MODULE Root ----\r\nEXTENDS Wanted\r\n====\r\n---- MODULE Wanted ----\r\nBad == )\r\n====")
	r, e := Check(context.Background(), Request{Path: "Root.tla", Text: root, Resolve: func(context.Context, string) (Source, error) { return Source{}, ErrModuleNotFound }})
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range r.Diagnostics {
		if d.Code == CodeSyntax {
			if d.Path != "Root.tla" || d.Span.Start.ByteOffset < strings.Index(string(root), "---- MODULE Wanted") {
				t.Fatalf("bad parse ownership %+v", d)
			}
			return
		}
	}
	t.Fatalf("missing parse diagnostic %+v", r.Diagnostics)
}

func TestMonolithRootOnlyAndCache(t *testing.T) {
	root := []byte("---- MODULE Root ----\nEXTENDS A, B\n====\n---- MODULE Shared ----\nValue == 1\n====")
	calls := map[string]int{}
	resolve := func(_ context.Context, name string) (Source, error) {
		calls[name]++
		switch name {
		case "A", "B":
			return Source{Path: name + ".tla", Text: []byte("---- MODULE " + name + " ----\nEXTENDS Shared\n====")}, nil
		default:
			return Source{}, ErrModuleNotFound
		}
	}
	r, err := Check(context.Background(), Request{Path: "Root.tla", Text: root, Resolve: resolve})
	if err != nil {
		t.Fatal(err)
	}
	if calls["Shared"] != 1 {
		t.Fatalf("cache calls: %v", calls)
	}
	for _, d := range r.Diagnostics {
		if strings.Contains(d.Message, "not found") {
			t.Fatalf("fallback: %+v", d)
		}
	}
	root = []byte("---- MODULE Root ----\nEXTENDS A\n====")
	resolve = func(_ context.Context, name string) (Source, error) {
		if name == "A" {
			return Source{Text: []byte("---- MODULE A ----\nEXTENDS Hidden\n====\n---- MODULE Hidden ----\nX == 1\n====")}, nil
		}
		return Source{}, ErrModuleNotFound
	}
	r, err = Check(context.Background(), Request{Text: root, Resolve: resolve})
	if err != nil {
		t.Fatal(err)
	}
	// Monolithic fallback reads only the root source; SpecObj then aborts.
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "semantic/4220" || r.Diagnostics[0].Message != "Cannot find source file for module Hidden imported in module A." || r.Completion != CompletionComplete {
		t.Fatalf("scanned dependency: %+v", r)
	}
}

// A dependency cycle through a monolith-extracted unit names the unit's
// temporary file, new File(name).getName()+".tla"
// (MonolithSpecExtractor.java:77-82,107; FileUtil.java:600-612), because
// SpecObj.pathToString prints ParseUnit.getFileName (SpecObj.java:341-349,
// ParseUnit.java:111). The diagnostic itself stays owned by the root path.
func TestMonolithCycleNamesExtractedFile(t *testing.T) {
	cases := []struct{ id, path, text, want string }{
		// deep-cuts/inspection/U-89e00eeb3046, recorded SANY: A.tla --> A.tla.
		{"U-89e00eeb3046", "ExtSelfInner.tla", "---- MODULE ExtSelfInner ----\n---- MODULE A ----\nEXTENDS A\n====\n====\n", "A.tla --> A.tla"},
		// model-pit-survivors/E-bc225ab8a548, recorded SANY: Inner.tla --> Inner.tla.
		{"E-bc225ab8a548", "Target.tla", "\\* Copyright 2026 Christopher O'Connell\n\\* All rights reserved\n\\* Aim: tla2sany/modanalyzer/ParseUnit.java:457:ConditionalsBoundaryMutator (model @cf/zai-org/glm-5.3-flash)\n---- MODULE Target ----\n---- MODULE Inner ----\nCONSTANT c\nf == INSTANCE Inner\n====\ng == 1\n====\n", "Inner.tla --> Inner.tla"},
		// Authored: the label is the module's file name, never the root's
		// directory or file.
		{"authored", "dir/Root.tla", "---- MODULE Root ----\nEXTENDS A\n====\n---- MODULE A ----\nEXTENDS B\n====\n---- MODULE B ----\nEXTENDS A\n====\n", "A.tla --> B.tla --> A.tla"},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			r, e := Check(context.Background(), Request{Path: c.path, Origin: OriginWorkspace, Text: []byte(c.text), Resolve: func(context.Context, string) (Source, error) { return Source{}, ErrModuleNotFound }})
			if e != nil || r.Completion != CompletionComplete || len(r.Diagnostics) != 1 {
				t.Fatalf("%v %+v", e, r)
			}
			d := r.Diagnostics[0]
			if d.Code != "semantic/4222" || d.Severity != SeverityError || d.Path != c.path || d.Message != "Circular dependency among .tla files; dependency cycle is:\n\n  "+c.want {
				t.Fatalf("%+v", d)
			}
		})
	}
}
