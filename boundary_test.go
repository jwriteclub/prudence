// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package prudence

import (
	"context"
	"strings"
	"testing"
)

func TestCheckBoundaryArgumentsAndSourceLimit(t *testing.T) {
	ctx := context.Background()
	if _, err := Check(ctx, Request{Origin: SourceOrigin(99)}); err == nil {
		t.Fatal("invalid root origin accepted")
	}
	root := []byte("---- MODULE R ----\nEXTENDS D\n====\n")
	if _, err := Check(ctx, Request{Text: root, Resolve: func(context.Context, string) (Source, error) { return Source{Origin: SourceOrigin(99)}, nil }}); err == nil {
		t.Fatal("invalid resolver origin accepted")
	}
	// Over-limit input must be rejected before scanning or copying, including syntax-only requests.
	text := make([]byte, (32<<20)+1)
	r, err := Check(ctx, Request{Text: text, Stage: LevelSyntax})
	if err != nil || r.Completion != CompletionIncomplete || len(r.Diagnostics) != 1 || !strings.Contains(r.Diagnostics[0].Message, "size limit") {
		t.Fatalf("result=%+v err=%v", r, err)
	}
}

func TestDependencyDepthDiagnostic(t *testing.T) {
	source := "---- MODULE D ----\nX == " + strings.Repeat("(", 80) + "1" + strings.Repeat(")", 80) + "\n====\n"
	opts := Options{MaxDepth: 30}
	root, err := Check(context.Background(), Request{Path: "direct.tla", Text: []byte(source), Options: opts})
	if err != nil {
		t.Fatal(err)
	}
	dep, err := Check(context.Background(), Request{Path: "R.tla", Text: []byte("---- MODULE R ----\nEXTENDS D\n====\n"), Options: opts, Resolve: func(context.Context, string) (Source, error) {
		return Source{Path: "dep.tla", Text: []byte(source)}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	var rd, dd *Diagnostic
	for i := range root.Diagnostics {
		if root.Diagnostics[i].Code == CodeDepthLimit {
			rd = &root.Diagnostics[i]
		}
	}
	for i := range dep.Diagnostics {
		if dep.Diagnostics[i].Code == CodeDepthLimit {
			dd = &dep.Diagnostics[i]
		}
	}
	if rd == nil || dd == nil || dd.Path != "dep.tla" || rd.Message != dd.Message || rd.Span != dd.Span || dep.Completion != CompletionIncomplete {
		t.Fatalf("root=%+v dependency=%+v", root, dep)
	}
}
