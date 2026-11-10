// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package prudence

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/jwriteclub/prudence/internal/semantic"
)

// sanyEntry is one entry of SANY's recorded error list. A zero position is
// SANY's never-set location (the SyntaxTreeNode location array and Location.java),
// printed as "line 2147483647, col 2147483647 to line -2147483648, col
// -2147483648".
type sanyEntry struct {
	l1, c1, l2, c2 int
	message        string
}

type sentinelCase struct {
	id, root string
	entries  []sanyEntry
}

// Recorded stock SANY output (tlaplus dc6a944) for the deep-cuts/fuzz-jazzer
// cases; sources are copied under testdata/sentinel.
var sentinelCases = []sentinelCase{
	{id: "D-072e65b52eb4", root: "Fairness_INSTANCE_test.tla", entries: []sanyEntry{
		{l1: 6, c1: 13, l2: 6, c2: 16, message: "Unknown operator: `vars'."},
		{l1: 6, c1: 18, l2: 6, c2: 21, message: "Unknown operator: `Next'."},
		{l1: 7, c1: 13, l2: 7, c2: 16, message: "Unknown operator: `vars'."},
		{l1: 7, c1: 18, l2: 7, c2: 21, message: "Unknown operator: `Next'."},
		{l1: 11, c1: 17, l2: 11, c2: 20, message: "Unknown operator: `Next'."},
		{l1: 11, c1: 24, l2: 11, c2: 27, message: "Unknown operator: `vars'."},
		{l1: 12, c1: 13, l2: 12, c2: 16, message: "Unknown operator: `vars'."},
		{l1: 12, c1: 18, l2: 12, c2: 21, message: "Unknown operator: `Next'."},
		{l1: 13, c1: 13, l2: 13, c2: 16, message: "Unknown operator: `vars'."},
		{l1: 13, c1: 18, l2: 13, c2: 21, message: "Unknown operator: `Next'."},
		{l1: 10, c1: 1, l2: 14, c2: 22, message: "Multiply-defined module 'Inner': this definition or declaration conflicts \nwith the one at line 4, col 1 to line 8, col 5 of module Fairness_INSTANCE_test."},
		{l1: 18, c1: 3, l2: 18, c2: 9, message: "Unknown operator: `I!WFair'."},
		{l1: 18, c1: 20, l2: 18, c2: 23, message: "Unknown operator: `I!En'."},
		{l1: 18, c1: 34, l2: 18, c2: 39, message: "Unknown operator: `I!Next'."},
		{l1: 18, c1: 44, l2: 18, c2: 49, message: "Unknown operator: `I!vars'."},
		{l1: 19, c1: 12, l2: 19, c2: 18, message: "Unknown operator: `I!WFair'."},
		{l1: 19, c1: 12, l2: 19, c2: 18, message: "DEF clause entry should describe a defined operator."},
		{message: "Unexpected token found."},
		{l1: 19, c1: 24, l2: 19, c2: 24, message: "Unexpected token found."},
		{message: "Need name or step number here, not `N_IdPrefix'."},
		{l1: 19, c1: 21, l2: 19, c2: 25, message: "DEF clause entry should describe a defined operator."},
		{l1: 21, c1: 9, l2: 21, c2: 22, message: "Unknown operator: `InstantiatedSs'."},
	}},
	{id: "D-5a22a0a14135", root: "SubrefLL_test.tla", entries: []sanyEntry{
		{l1: 12, c1: 23, l2: 12, c2: 25, message: "Unknown operator: `lab'."},
		{l1: 14, c1: 12, l2: 14, c2: 12, message: "The operator % requires 2 arguments."},
		{l1: 15, c1: 1, l2: 15, c2: 37, message: "Operator Foo already defined or declared."},
		{l1: 15, c1: 27, l2: 15, c2: 27, message: "Illegal parameter @ of label `lab'."},
		{l1: 15, c1: 23, l2: 15, c2: 37, message: "Label lab must contain formal parameter `x'."},
		{l1: 15, c1: 23, l2: 15, c2: 37, message: "Label lab declares extra parameter(s)  @  "},
		{l1: 15, c1: 1, l2: 15, c2: 37, message: "Multiply-defined symbol 'Foo': this definition or declaration conflicts \nwith the one at line 7, col 1 to line 7, col 13 of module SubrefLL_test."},
		{l1: 17, c1: 8, l2: 17, c2: 14, message: "The operator Foo requires 1 arguments."},
		{message: "Unexpected token found."},
		{l1: 19, c1: 12, l2: 19, c2: 12, message: "Unexpected token found."},
		{l1: 19, c1: 16, l2: 19, c2: 16, message: "Unexpected token found."},
		{message: "Need name or step number here, not `N_IdPrefix'."},
		{l1: 19, c1: 9, l2: 19, c2: 25, message: "DEF clause entry should describe a defined operator."},
		{l1: 19, c1: 1, l2: 19, c2: 25, message: "Empty USE or HIDE statement."},
		{l1: 21, c1: 9, l2: 21, c2: 11, message: "ASSUME/PROVE used where an expression is required."},
		{l1: 21, c1: 15, l2: 21, c2: 21, message: "The operator Foo requires 1 arguments."},
	}},
	{id: "D-a9c6a1692280", root: "def_nondef_test.tla", entries: []sanyEntry{
		{l1: 9, c1: 13, l2: 9, c2: 13, message: "The operator \\times requires 2 arguments."},
		{l1: 9, c1: 18, l2: 9, c2: 18, message: "Unknown operator: `v'."},
		{l1: 9, c1: 18, l2: 9, c2: 18, message: "DEF clause entry should describe a defined operator."},
		{l1: 10, c1: 3, l2: 10, c2: 7, message: "Multiply-defined symbol '<1>2': this definition or declaration conflicts \nwith the one at line 9, col 3 to line 9, col 18 of module def_nondef_test."},
		{l1: 13, c1: 10, l2: 13, c2: 94, message: "Was expecting a GeneralId."},
		{message: "Unexpected token found."},
		{l1: 13, c1: 90, l2: 13, c2: 90, message: "Unexpected token found."},
		{message: "Need name or step number here, not `N_IdPrefix'."},
		{l1: 13, c1: 10, l2: 13, c2: 94, message: "DEF clause entry should describe a defined operator."},
		{l1: 13, c1: 3, l2: 13, c2: 94, message: "Empty BY"},
	}},
	{id: "D-c412e23757be", root: "Test.tla", entries: []sanyEntry{
		{l1: 4, c1: 22, l2: 4, c2: 27, message: "Unknown operator: `3HHIDE'."},
		{l1: 4, c1: 34, l2: 4, c2: 39, message: "Unexpected token found."},
		{message: "Unexpected token found."},
		{l1: 4, c1: 52, l2: 4, c2: 52, message: "Unexpected token found."},
		{l1: 5, c1: 8, l2: 5, c2: 9, message: "Unexpected token found."},
		{l1: 4, c1: 34, l2: 4, c2: 39, message: "Selector `N_InfixExpr' should not have argument(s)."},
		{l1: 4, c1: 34, l2: 5, c2: 28, message: "DEF clause entry should describe a defined operator."},
	}},
}

func TestPublicCheckNeverSetLocationDiagnostics(t *testing.T) {
	library, err := os.ReadFile(filepath.Join("testdata", "sentinel", "TLAPS.tla"))
	if err != nil {
		t.Fatal(err)
	}
	resolve := func(_ context.Context, name string) (Source, error) {
		if name == "TLAPS" {
			return Source{Path: "TLAPS.tla", Text: library, Origin: OriginWorkspace}, nil
		}
		return Source{}, ErrModuleNotFound
	}
	for _, c := range sentinelCases {
		t.Run(c.id, func(t *testing.T) {
			text, err := os.ReadFile(filepath.Join("testdata", "sentinel", c.id, c.root))
			if err != nil {
				t.Fatal(err)
			}
			r := checkText(t, "", c.root, string(text), resolve)
			var errs []Diagnostic
			for _, d := range r.Diagnostics {
				if d.Severity == SeverityError {
					errs = append(errs, d)
				}
			}
			if r.Completion != CompletionComplete || len(errs) != len(c.entries) {
				t.Fatalf("completion %v, %d errors, want %d: %+v", r.Completion, len(errs), len(c.entries), r.Diagnostics)
			}
			for _, d := range r.Diagnostics {
				if d.Code == CodeIncomplete {
					t.Fatalf("incomplete diagnostic: %+v", d)
				}
			}
			sentinels := 0
			for i, want := range c.entries {
				d := errs[i]
				if d.Message != want.message || d.Path != c.root {
					t.Fatalf("entry %d: %+v, want %q", i, d, want.message)
				}
				if want.l1 == 0 {
					sentinels++
					if d.Span != (Span{}) {
						t.Fatalf("entry %d: sentinel span %+v", i, d.Span)
					}
					continue
				}
				if d.Span.Start.Line+1 != want.l1 || d.Span.Start.Column+1 != want.c1 || d.Span.End.Line+1 != want.l2 || d.Span.End.Column != want.c2 {
					t.Fatalf("entry %d: span %+v, SANY line %d, col %d to line %d, col %d", i, d.Span, want.l1, want.c1, want.l2, want.c2)
				}
			}
			if sentinels == 0 {
				t.Fatal("no sentinel entries in the recorded case")
			}
		})
	}
}

func TestNeverSetLocationIsExact(t *testing.T) {
	sentinel := semantic.Location{NamePresent: true, Name: "M", BeginLine: math.MaxInt32, BeginColumn: math.MaxInt32, EndLine: math.MinInt32, EndColumn: math.MinInt32}
	if !neverSetLocation(sentinel) {
		t.Fatal("exact sentinel not recognized")
	}
	for name, mutate := range map[string]func(*semantic.Location){
		"null":         func(l *semantic.Location) { l.Null = true },
		"no name":      func(l *semantic.Location) { l.NamePresent = false },
		"begin line":   func(l *semantic.Location) { l.BeginLine = 7 },
		"begin column": func(l *semantic.Location) { l.BeginColumn = 7 },
		"end line":     func(l *semantic.Location) { l.EndLine = -7 },
		"end column":   func(l *semantic.Location) { l.EndColumn = -7 },
		"zero range":   func(l *semantic.Location) { *l = semantic.Location{NamePresent: true, Name: "M"} },
		"swapped":      func(l *semantic.Location) { l.BeginLine, l.EndLine = l.EndLine, l.BeginLine },
	} {
		l := sentinel
		mutate(&l)
		if neverSetLocation(l) {
			t.Errorf("%s: accepted", name)
		}
	}
}
