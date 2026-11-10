// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package modulegraph

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestMonolithPhysicalLinesAndMapping(t *testing.T) {
	root := Input{Path: "root.tla", Text: []byte("before\r\n---- MODULE Wanted ----\rA == \t\"λ😀\"\n==== trailing\r---- MODULE Wanted ----\nB == 1\n====")}
	budget := 1 << 20
	out, ok, err := ExtractMonolith(context.Background(), root, "Wanted", &budget)
	if err != nil || !ok {
		t.Fatalf("extract: %v %v", ok, err)
	}
	want := "---- MODULE Wanted ----\nA == \t\"λ😀\"\n==== trailing\n"
	if string(out.Text) != want {
		t.Fatalf("got %q", out.Text)
	}
	for _, sub := range []string{"Wanted", "λ😀", "==== trailing"} {
		a := strings.Index(string(out.Text), sub)
		b := a + len(sub)
		x, y, good := out.View.MapSpan(a, b)
		if !good || string(root.Text[x:y]) != sub {
			t.Fatalf("%q -> %d:%d %v", sub, x, y, good)
		}
	}
	a := strings.Index(string(out.Text), "\n")
	x, y, good := out.View.MapSpan(a, a+1)
	if !good || string(root.Text[x:y]) != "\r" {
		t.Fatalf("newline: %q %v", root.Text[x:y], good)
	}
}
func TestMonolithExactMatchAndBounds(t *testing.T) {
	for _, tc := range []struct {
		source string
		found  bool
	}{
		{" ---- MODULE X ----\n====", false}, {"---- MODULE X ---- \\* comment\n====", false},
		{"--- MODULE X ----\n====", false}, {"---- MODULE X ---\n====", true},
		{"---- MODULE X ----\nno terminator", true}, {"(*\n---- MODULE X ----\n====\n*)", true},
	} {
		budget := 1024
		out, ok, err := ExtractMonolith(context.Background(), Input{Text: []byte(tc.source)}, "X", &budget)
		if err != nil || ok != tc.found {
			t.Fatalf("%q: %v %v", tc.source, ok, err)
		}
		if ok && !strings.HasSuffix(string(out.Text), "\n") {
			t.Fatal("missing println newline")
		}
	}
	budget := 1
	_, _, err := ExtractMonolith(context.Background(), Input{Text: []byte("---- MODULE X ----")}, "X", &budget)
	if !errors.Is(err, ErrIncomplete) {
		t.Fatal(err)
	}
	budget = 100
	_, ok, err := ExtractMonolith(context.Background(), Input{Text: []byte{0xff}}, "X", &budget)
	if err != nil || ok {
		t.Fatalf("malformed root without the module: %v %v", ok, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = ExtractMonolith(ctx, Input{}, "X", &budget)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

// Malformed bytes are copied unchanged; a truncated group never absorbs a
// CR or LF, and Java's header and end-marker patterns never match U+FFFD.
func TestMonolithMalformedBytes(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n", "\r"} {
		lines := []string{
			"\\* \xff", "---- MODULE X \xff----", "---- MODULE X ----", "A == \"\xe2\x82\"",
			"\\* \xf0\x9f", "====\x85", "B == 1", "====\xc2\x85", "C == \xed\xa0\x80", "====", "tail \xc0",
		}
		root := Input{Path: "root.tla", Text: []byte(strings.Join(lines, nl))}
		budget := 1 << 20
		out, ok, err := ExtractMonolith(context.Background(), root, "X", &budget)
		if err != nil || !ok {
			t.Fatalf("%q: %v %v", nl, ok, err)
		}
		// The FF header is skipped; the lone 85 byte is not U+0085, so that
		// line ends the module.
		want := strings.Join(lines[2:6], "\n") + "\n"
		if string(out.Text) != want {
			t.Fatalf("%q: got %q, want %q", nl, out.Text, want)
		}
		for _, sub := range []string{"\xe2\x82", "\xf0\x9f", "====\x85"} {
			a := strings.Index(string(out.Text), sub)
			x, y, good := out.View.MapSpan(a, a+len(sub))
			if !good || string(root.Text[x:y]) != sub {
				t.Fatalf("%q: %q -> %d:%d %v", nl, sub, x, y, good)
			}
		}
		// The terminator after the truncated F0 9F maps to the original one.
		a := strings.Index(string(out.Text), "\xf0\x9f") + 2
		x, y, good := out.View.MapSpan(a, a+1)
		if !good || string(root.Text[x:y]) != nl {
			t.Fatalf("%q: terminator %q %v", nl, root.Text[x:y], good)
		}
		// Valid U+0085 is a Java line terminator, so that line is no end.
		budget = 1 << 20
		lines[5] = "B0 == 0"
		root.Text = []byte(strings.Join(lines, nl))
		out, ok, err = ExtractMonolith(context.Background(), root, "X", &budget)
		want = strings.Join(lines[2:10], "\n") + "\n"
		if err != nil || !ok || string(out.Text) != want {
			t.Fatalf("%q: got %q %v %v, want %q", nl, out.Text, ok, err, want)
		}
	}
}

func TestMonolithNestedModuleIsExternalParseUnit(t *testing.T) {
	root := Input{Path: "root.tla", Text: []byte("---- MODULE Root ----\nINSTANCE Inner\n---- MODULE Inner ----\nX == 1\n====\n====\n")}
	budget := 1 << 20
	a, err := AcquireFlatExternal(context.Background(), root, func(ctx context.Context, name string) (Input, error) {
		out, found, e := ExtractMonolith(ctx, root, name, &budget)
		if e != nil {
			return Input{}, e
		}
		if !found {
			return Input{}, errors.New("missing")
		}
		return out, nil
	}, limits())
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Units) != 2 || a.Units[1].Name != "Inner" || a.Units[1].Parse.Tree == a.Root.RootModule.Children[0].Tree || a.Units[1].Input.View == nil {
		t.Fatalf("external identity: %+v", a)
	}
}

func TestMonolithSpanBoundaries(t *testing.T) {
	for _, ending := range []string{"\n", "\r\n", "\r", ""} {
		t.Run(strings.ReplaceAll(strings.ReplaceAll(ending, "\r", "CR"), "\n", "LF"), func(t *testing.T) {
			// A following module verifies EOF maps after the extracted terminator,
			// rather than after the rest of the owner.
			suffix := ""
			if ending != "" {
				suffix = "---- MODULE Later ----\n====\n"
			}
			source := "prefix\n---- MODULE X ----\r\nvalue\r====" + ending + suffix
			budget := 1 << 20
			in, ok, err := ExtractMonolith(context.Background(), Input{Text: []byte(source)}, "X", &budget)
			if err != nil || !ok {
				t.Fatalf("extract: %v %v", ok, err)
			}
			parsed := string(in.Text)
			newline := strings.IndexByte(parsed, '\n')
			next := newline + 1
			value := strings.Index(parsed, "value")
			eof := len(parsed)
			ownerStart := strings.Index(source, "---- MODULE X")
			delimiterEnd := strings.Index(source, "====") + 4 + len(ending)
			cases := []struct{ off, want int }{
				{0, ownerStart}, {1, ownerStart + 1},
				{newline, strings.Index(source, "\r\n")},
				{next, strings.Index(source, "value")},
				{value + len("value"), strings.Index(source, "value") + len("value")},
				{eof, delimiterEnd},
			}
			for _, tc := range cases {
				a, b, good := in.View.MapSpan(tc.off, tc.off)
				if !good || a != tc.want || b != tc.want {
					t.Fatalf("empty %d => %d:%d, want %d (ok=%v)", tc.off, a, b, tc.want, good)
				}
			}
			a, b, good := in.View.MapSpan(newline, newline+1)
			if !good || source[a:b] != "\r\n" {
				t.Fatalf("newline %q %v", source[a:b], good)
			}
			a, b, good = in.View.MapSpan(value, value+len("value\n====\n"))
			if !good || source[a:b] != "value\r===="+ending {
				t.Fatalf("across lines %q %v", source[a:b], good)
			}
		})
	}
}

func TestMonolithExternalDependencyCycle(t *testing.T) {
	root := Input{Path: "root.tla", Text: []byte("---- MODULE Root ----\nEXTENDS A\n====\n---- MODULE A ----\nEXTENDS B\n====\n---- MODULE B ----\nEXTENDS A\n====\n")}
	budget := 1 << 20
	var calls []string
	views := map[string]*SourceView{}
	_, err := AcquireFlatExternal(context.Background(), root, func(ctx context.Context, name string) (Input, error) {
		calls = append(calls, name)
		out, found, e := ExtractMonolith(ctx, root, name, &budget)
		if e != nil {
			return Input{}, e
		}
		if !found {
			return Input{}, errors.New("missing")
		}
		views[name] = out.View
		return out, nil
	}, limits())
	var cycle *CycleError
	if !errors.As(err, &cycle) {
		t.Fatalf("expected external cycle: %v", err)
	}
	if strings.Join(calls, ",") != "A,B" {
		t.Fatalf("cache or ordering: %v", calls)
	}
	// Extracted units are labeled by their temporary file, not the root
	// (MonolithSpecExtractor.java:77-82,107; SpecObj.java:341-349).
	if len(cycle.Path) != 3 || strings.Join(cycle.Path, ",") != "A.tla,B.tla,A.tla" {
		t.Fatalf("cycle path %v", cycle.Path)
	}
	if views["A"] == nil || views["B"] == nil || views["A"] == views["B"] || &views["A"].Owner[0] != &views["B"].Owner[0] {
		t.Fatal("views do not have distinct identities and a shared source owner")
	}
}
