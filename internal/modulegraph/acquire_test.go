// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package modulegraph

import (
	"context"
	"errors"
	"reflect"
	"testing"

	parser "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

func limits() AcquisitionLimits {
	return AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400}
}
func source(name, body string, origin uint8) Input {
	return Input{Path: "label/" + name, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n"), Origin: origin}
}
func TestFlatExtendsOrderDiamondAndProvenance(t *testing.T) {
	input := source("Root", "EXTENDS A, B\nX == 1", 1)
	files := map[string]Input{"A": source("A", "EXTENDS C\nX == 1", 1), "B": source("B", "EXTENDS C\nY == 1", 2), "C": source("C", "Z == 1", 2)}
	var calls []string
	lookup := func(_ context.Context, n string) (Input, error) { calls = append(calls, n); return files[n], nil }
	a, e := AcquireFlatExternal(context.Background(), input, lookup, limits())
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(calls, []string{"A", "B", "C"}) {
		t.Fatal(calls)
	}
	units, e := a.Order(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	var got []string
	for _, u := range units {
		got = append(got, u.Filename())
	}
	if !reflect.DeepEqual(got, []string{"label/C", "label/A", "label/B", "label/Root"}) {
		t.Fatal(got)
	}
	if refs := a.Root.References(); len(refs) != 2 || refs[0].Name != "A" || refs[0].Tree == nil || refs[1].Name != "B" {
		t.Fatal("reference order or location")
	}
	if len(a.Units) != 4 || a.Units[2].Input.Origin != 2 {
		t.Fatal("origin and identities")
	}
	original := a.Root.Input.Text[0]
	input.Text[0] = 'X'
	if a.Root.Input.Text[0] != original {
		t.Fatal("root bytes aliased")
	}
}
func TestFlatExtendsMissingNilAndCancellation(t *testing.T) {
	root := source("Root", "EXTENDS Missing", 1)
	if _, e := AcquireFlatExternal(context.Background(), root, nil, limits()); !errors.Is(e, ErrResolverUnavailable) {
		t.Fatal(e)
	}
	missing := errors.New("resolver-specific missing")
	if _, e := AcquireFlatExternal(context.Background(), root, func(context.Context, string) (Input, error) { return Input{}, missing }, limits()); !errors.Is(e, missing) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := AcquireFlatExternal(ctx, root, nil, limits()); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestFlatExtendsHeaderMismatch(t *testing.T) {
	root := source("Root", "EXTENDS A", 1)
	_, e := AcquireFlatExternal(context.Background(), root, func(context.Context, string) (Input, error) { return source("Wrong", "X == 1", 0), nil }, limits())
	// SANY reads module A from A.tla; a different module there is its
	// MODULE_NAME_DIFFERENT_FROM_FILE_NAME abort.
	var mismatch *FileNameMismatchError
	if !errors.As(e, &mismatch) || mismatch.FileName != "A" || mismatch.ModuleName != "Wrong" {
		t.Fatal(e)
	}
}
func TestFlatExtendsCyclesAndLimits(t *testing.T) {
	root := source("Root", "EXTENDS A", 1)
	_, e := AcquireFlatExternal(context.Background(), root, func(context.Context, string) (Input, error) { return source("A", "EXTENDS Root", 1), nil }, limits())
	var cycle *CycleError
	if !errors.As(e, &cycle) {
		t.Fatal(e)
	}
	l := limits()
	l.MaxTotalBytes = 1
	if _, e = AcquireFlatExternal(context.Background(), root, nil, l); !errors.Is(e, ErrIncomplete) {
		t.Fatal(e)
	}
}

func TestFlatExtendsRetainsFailedParse(t *testing.T) {
	root := source("Root", "EXTENDS A", 1)
	broken := source("A", "X ==", 2)
	a, err := AcquireFlatExternal(context.Background(), root, func(context.Context, string) (Input, error) { return broken, nil }, limits())
	if !errors.Is(err, ErrParseIncomplete) || a == nil || a.Failed == nil || a.Failed.Name != "A" || a.Failed.Input.Path != broken.Path {
		t.Fatalf("%#v %v", a, err)
	}
	broken.Text[0] = 'X'
	if a.Failed.Input.Text[0] == 'X' {
		t.Fatal("failed input aliased")
	}
}

func TestParsedRootMustMatchHeaderAndSource(t *testing.T) {
	a := source("A", "X == 1", 0)
	b := source("B", "X == 1", 0)
	parsed, err := parser.Parse(context.Background(), a.Text, parser.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wrong, err := parser.Parse(context.Background(), b.Text, parser.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	parsed.ModuleName = wrong.ModuleName
	a.Parsed = &parsed
	if _, err = AcquireFlatExternal(context.Background(), a, nil, limits()); !errors.Is(err, ErrIncomplete) {
		t.Fatal(err)
	}
}

func TestFlatExternalInstanceAcquisitionOrder(t *testing.T) {
	files := map[string]Input{
		"A": source("A", "LOCAL INSTANCE C", 1),
		"B": source("B", "EXTENDS D", 1),
		"C": source("C", "EXTENDS E", 1),
		"D": source("D", "DValue == 1", 1),
		"E": source("E", "EValue == 1", 1),
	}
	var calls []string
	root := source("Root", "EXTENDS A\nLOCAL INSTANCE B\nLOCAL INSTANCE B", 1)
	a, err := AcquireFlatExternal(context.Background(), root, func(_ context.Context, n string) (Input, error) { calls = append(calls, n); return files[n], nil }, limits())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"A", "B", "D", "C", "E"}) {
		t.Fatal(calls)
	}
	refs := a.Root.References()
	if len(refs) != 3 || refs[0].Kind != Extends || refs[1].Kind != Instance || refs[2].Kind != Instance {
		t.Fatal(refs)
	}
}

func TestMixedInstanceReferencesShareBindingAndRestart(t *testing.T) {
	files := map[string]Input{"A": source("A", "EXTENDS C", 1), "B": source("B", "EXTENDS D", 1), "C": source("C", "Value == 1", 1), "D": source("D", "Value == 2", 1)}
	var calls []string
	root := source("Root", "EXTENDS A\nINSTANCE A\nI == INSTANCE B\nJ == INSTANCE B", 1)
	a, err := AcquireFlatExternal(context.Background(), root, func(_ context.Context, n string) (Input, error) { calls = append(calls, n); return files[n], nil }, limits())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"A", "C", "B", "D"}) {
		t.Fatal(calls)
	}
	if len(a.Root.References()) != 4 || len(a.Root.edges) != 2 {
		t.Fatalf("references %d edges %d", len(a.Root.References()), len(a.Root.edges))
	}
}

func TestRepeatedExtendsRetainsOccurrencesAndResolvesOnce(t *testing.T) {
	root := source("Root", "EXTENDS A, A", 1)
	calls := 0
	a, err := AcquireFlatExternal(context.Background(), root, func(_ context.Context, n string) (Input, error) {
		calls++
		if n != "A" {
			t.Fatalf("unexpected request %q", n)
		}
		return source("A", "X == 1", 1), nil
	}, limits())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(a.Root.References()) != 2 || len(a.Root.edges) != 1 {
		t.Fatalf("calls %d refs %d edges %d", calls, len(a.Root.References()), len(a.Root.edges))
	}
}

func TestSelfExtendsIsAFileCycle(t *testing.T) {
	_, err := AcquireFlatExternal(context.Background(), source("Root", "EXTENDS Root", 1), nil, limits())
	var cycle *CycleError
	if !errors.As(err, &cycle) {
		t.Fatalf("expected cycle, got %v", err)
	}
}

func TestLexicalModuleAcquisition(t *testing.T) {
	tests := []struct {
		body  string
		calls []string
	}{
		{"---- MODULE A ----\nX == 1\n====\n---- MODULE B ----\nEXTENDS A\n====", nil},
		{"---- MODULE B ----\nEXTENDS A\n====\n---- MODULE A ----\nX == 1\n====", []string{"A"}},
		{"---- MODULE A ----\nX == 1\n====\nINSTANCE A", nil},
		{"INSTANCE A\n---- MODULE A ----\nX == 1\n====\nINSTANCE A", []string{"A"}},
		{"---- MODULE A ----\nEXTENDS A\n====", []string{"A"}},
	}
	for _, tc := range tests {
		t.Run(tc.body, func(t *testing.T) {
			var calls []string
			a, err := AcquireFlatExternal(context.Background(), source("Root", tc.body, 1), func(_ context.Context, n string) (Input, error) {
				calls = append(calls, n)
				return source(n, "Z == 1", 1), nil
			}, limits())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(calls, tc.calls) {
				t.Fatalf("calls %v, want %v", calls, tc.calls)
			}
			if a.Root.RootModule == nil || len(a.Root.RootModule.Children) == 0 {
				t.Fatal("missing lexical identities")
			}
		})
	}
}

func TestExternalExtendsExposesChildButInstanceDoesNot(t *testing.T) {
	files := map[string]Input{"A": source("A", "---- MODULE Inner ----\nX == 1\n====", 1), "Inner": source("Inner", "Y == 1", 1)}
	for _, tc := range []struct {
		body string
		want []string
	}{{"EXTENDS A\n---- MODULE B ----\nEXTENDS Inner\n====", []string{"A"}}, {"INSTANCE A\n---- MODULE B ----\nEXTENDS Inner\n====", []string{"Inner", "A"}}} {
		var calls []string
		_, err := AcquireFlatExternal(context.Background(), source("Root", tc.body, 1), func(_ context.Context, n string) (Input, error) { calls = append(calls, n); return files[n], nil }, limits())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(calls, tc.want) {
			t.Fatalf("%s: got %v want %v", tc.body, calls, tc.want)
		}
	}
}

func TestNestedMixedFileCycleAndLimits(t *testing.T) {
	root := source("Root", "---- MODULE Inner ----\nINSTANCE A\n====", 1)
	_, err := AcquireFlatExternal(context.Background(), root, func(_ context.Context, n string) (Input, error) { return source("A", "EXTENDS Root", 1), nil }, limits())
	var cycle *CycleError
	if !errors.As(err, &cycle) || !reflect.DeepEqual(cycle.Path, []string{"label/Root", "label/A", "label/Root"}) {
		t.Fatalf("cycle %#v %v", cycle, err)
	}
	l := limits()
	l.MaxDepth = 4
	_, err = AcquireFlatExternal(context.Background(), root, nil, l)
	if err == nil {
		t.Fatal("expected a bounded parser or acquisition depth error")
	}
}

func TestNestedResolverFailureAndChildIdentity(t *testing.T) {
	root := source("Root", "---- MODULE A ----\n---- MODULE X ----\nP == 1\n====\n====\n---- MODULE B ----\n---- MODULE X ----\nQ == 1\n====\nINSTANCE Missing\n====", 1)
	failure := errors.New("operational")
	_, err := AcquireFlatExternal(context.Background(), root, func(context.Context, string) (Input, error) { return Input{}, failure }, limits())
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	a, err := AcquireFlatExternal(context.Background(), root, func(_ context.Context, n string) (Input, error) { return source(n, "V == 1", 1), nil }, limits())
	if err != nil {
		t.Fatal(err)
	}
	x, y := a.Root.RootModule.Children[0].Children[0], a.Root.RootModule.Children[1].Children[0]
	if x == y || x.Document != y.Document || x.Name != y.Name || x.Parent == y.Parent {
		t.Fatal("nested identity")
	}
}

func TestUnambiguousIndirectExtenderPropagation(t *testing.T) {
	files := map[string]Input{"A": source("A", "EXTENDS B", 1), "B": source("B", "---- MODULE Child ----\nX == 1\n====", 1)}
	var calls []string
	a, err := AcquireFlatExternal(context.Background(), source("Root", "EXTENDS A\nINSTANCE Child", 1), func(_ context.Context, n string) (Input, error) { calls = append(calls, n); return files[n], nil }, limits())
	if err != nil || !reflect.DeepEqual(calls, []string{"A", "B"}) {
		t.Fatalf("calls %v: %v", calls, err)
	}
	child := a.Units[2].RootModule.Children[0]
	if a.Root.RootModule.bindings["Child"] != child {
		t.Fatal("indirect binding missing")
	}
}

func TestNestedFailedDependencyRetainsRequester(t *testing.T) {
	root := source("Root", "---- MODULE Inner ----\nEXTENDS Broken\n====", 1)
	a, err := AcquireFlatExternal(context.Background(), root, func(_ context.Context, n string) (Input, error) { return source(n, "X ==", 1), nil }, limits())
	if !errors.Is(err, ErrParseIncomplete) || a == nil || a.Failed == nil || a.FailedOwner == nil || a.FailedOwner.Name != "Inner" || a.FailedReference == nil || a.FailedReference.Name != "Broken" || a.FailedReference.Tree == nil {
		t.Fatalf("%#v %v", a, err)
	}
}

func TestAncestorPeerAndParentNameResolution(t *testing.T) {
	tests := []struct {
		body  string
		calls []string
	}{
		{"---- MODULE Earlier ----\nX == 1\n====\n---- MODULE Parent ----\n---- MODULE Deep ----\nEXTENDS Earlier\n====\n====", nil},
		{"---- MODULE Parent ----\n---- MODULE Deep ----\nEXTENDS Parent\n====\n====", []string{"Parent"}},
	}
	for _, tc := range tests {
		var calls []string
		_, err := AcquireFlatExternal(context.Background(), source("Root", tc.body, 1), func(_ context.Context, n string) (Input, error) {
			calls = append(calls, n)
			return source(n, "Z == 1", 1), nil
		}, limits())
		if err != nil || !reflect.DeepEqual(calls, tc.calls) {
			t.Fatalf("calls %v: %v", calls, err)
		}
	}
}

func TestProofLocalInstanceDiscovery(t *testing.T) {
	root := source("Root", "EXTENDS C\nTHEOREM TRUE\n<1>1. INSTANCE A\n<1>2. DEFINE I == INSTANCE B\n<1> QED OBVIOUS", 1)
	files := map[string]Input{"A": source("A", "", 1), "B": source("B", "", 1), "C": source("C", "", 1)}
	var calls []string
	a, err := AcquireFlatExternal(context.Background(), root, func(_ context.Context, name string) (Input, error) {
		calls = append(calls, name)
		return files[name], nil
	}, limits())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"C", "A", "B"}) {
		t.Fatalf("dependency order: %v", calls)
	}
	refs := a.Root.References()
	if len(refs) != 3 || refs[0].Kind != Extends || refs[1].Name != "A" || refs[2].Name != "B" {
		t.Fatalf("references: %v", refs)
	}
}

func TestProofInstanceDoesNotCountAsTopLevelInternalOccurrence(t *testing.T) {
	root := source("Root", "---- MODULE A ----\n====\nTHEOREM TRUE\n<1>1. INSTANCE A\n<1> QED OBVIOUS", 1)
	var calls []string
	_, err := AcquireFlatExternal(context.Background(), root, func(_ context.Context, n string) (Input, error) {
		calls = append(calls, n)
		return source(n, "", 1), nil
	}, limits())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"A"}) {
		t.Fatalf("SpecObj top-level occurrence rule: %v", calls)
	}
}

func TestNestedModuleOwnsProofInstanceAndModuleFactsDoNotLoad(t *testing.T) {
	root := source("Root", "---- MODULE Child ----\nTHEOREM TRUE\n<1>1. INSTANCE A\n<1> QED OBVIOUS\n====\nUSE MODULE Missing", 1)
	var calls []string
	acquisition, err := AcquireFlatExternal(context.Background(), root, func(_ context.Context, name string) (Input, error) {
		calls = append(calls, name)
		return source(name, "", 1), nil
	}, limits())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"A"}) {
		t.Fatalf("resolver calls: %v", calls)
	}
	child := acquisition.Root.RootModule.Children[0]
	if child.Document != acquisition.Root || len(child.refs) != 1 || child.refs[0].Name != "A" || len(acquisition.Root.RootModule.refs) != 0 {
		t.Fatal("nested proof instance ownership")
	}
}

// TestLetModuleDefinitionAdmitted covers the N_LetDefinitions placement of
// an N_ModuleDefinition and its N_NonLocalInstance: Generator.processLetIn
// hands it to processModuleDefinition (Generator.java:2998-3000), and
// ParseUnit.getInstances finds its INSTANCE in preorder
// (ParseUnit.java:660-690), after the INSTANCEs that precede it and before
// those inside its own WITH clause.
func TestLetModuleDefinitionAdmitted(t *testing.T) {
	root := source("Root", "INSTANCE A\nR == LET I(x) == INSTANCE B WITH p <- LET K == INSTANCE C IN K!v IN I(1)!w\nS == LET LOCAL J == INSTANCE D IN 1", 1)
	files := map[string]Input{"A": source("A", "u == 1", 1), "B": source("B", "CONSTANT p\nw == p", 1), "C": source("C", "v == 1", 1), "D": source("D", "z == 1", 1)}
	var calls []string
	a, e := AcquireFlatExternal(context.Background(), root, func(_ context.Context, n string) (Input, error) { calls = append(calls, n); return files[n], nil }, limits())
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(calls, []string{"A", "B", "C", "D"}) {
		t.Fatal(calls)
	}
	var refs []string
	for _, r := range a.Root.References() {
		if r.Kind != Instance || r.Tree == nil {
			t.Fatalf("%+v", r)
		}
		refs = append(refs, r.Name)
	}
	if !reflect.DeepEqual(refs, []string{"A", "B", "C", "D"}) {
		t.Fatal(refs)
	}
	// The admission is the LET placement only: the same limits still apply.
	l := limits()
	l.MaxNodes = 20
	var budget *BudgetError
	if _, e = AcquireFlatExternal(context.Background(), root, func(_ context.Context, n string) (Input, error) { return files[n], nil }, l); !errors.Is(e, ErrIncomplete) || !errors.As(e, &budget) || *budget != (BudgetError{Pass: PassSyntaxNodes, Limit: 20, Observed: 21}) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = AcquireFlatExternal(ctx, root, func(_ context.Context, n string) (Input, error) { return files[n], nil }, limits()); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
