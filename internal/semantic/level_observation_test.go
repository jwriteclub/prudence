// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"github.com/jwriteclub/prudence/internal/modulegraph"
	"testing"
)

func TestLevelCandidateInterleavesAndRetainsLevelWinner(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name + ".tla", Origin: modulegraph.OriginWorkspace, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	ctx := stdcontext.Background()
	dep, root := input("A", "VARIABLE x\nASSUME x"), input("Root", "EXTENDS A")
	a, err := modulegraph.AcquireFlatExternal(ctx, root, func(stdcontext.Context, string) (modulegraph.Input, error) { return dep, nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	o := observeLevelCandidate(ctx, a, 1000)
	if o.err != nil || !o.finished || len(o.attempts) != 2 || !o.attempts[0].entered || !o.attempts[0].completed || !o.attempts[1].skipped {
		t.Fatalf("candidate: %+v", o)
	}
	if len(o.generation) != 0 || len(o.level) != 1 || o.level[0].Code.Value != 4206 {
		t.Fatalf("stages: generation=%+v level=%+v", o.generation, o.level)
	}
}

func TestLevelCandidateInvalidInput(t *testing.T) {
	o := observeLevelCandidate(nil, nil, 1000)
	if o.err != ErrPortIncomplete || o.finished || len(o.attempts) != 0 {
		t.Fatalf("candidate: %+v", o)
	}
}

func TestLevelEnabledSuccessAndTheoremInternerBoundary(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name + ".tla", Origin: modulegraph.OriginWorkspace, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	ctx := stdcontext.Background()
	acquire := func(root, dep modulegraph.Input) *modulegraph.Acquisition {
		a, e := modulegraph.AcquireFlatExternal(ctx, root, func(stdcontext.Context, string) (modulegraph.Input, error) { return dep, nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
		if e != nil {
			t.Fatal(e)
		}
		return a
	}
	o := observeLevelCandidate(ctx, acquire(input("Root", "EXTENDS A\nB == X"), input("A", "X == 1")), 1000)
	if o.err != nil || !o.bound || len(o.attempts) != 2 || !o.attempts[0].completed || !o.attempts[1].completed {
		t.Fatalf("successful chain: %+v", o)
	}
	o = observeLevelCandidate(ctx, acquire(input("Root", "THEOREM TRUE"), input("A", "X == 1")), 1000)
	if o.err != nil || !o.bound || len(o.attempts) != 1 || !o.attempts[0].completed {
		t.Fatalf("theorem entry: %+v", o)
	}
}

func TestStructuralParameterFreeDiffersFromLevelConstant(t *testing.T) {
	p := newNamePool()
	ctx := newContext(p)
	k := &opDeclNode{nodeHeader: nodeHeader{semanticHeader: semanticHeader{kind: 2}, name: p.intern("K")}}
	ctx.add(contextKey{name: k.name}, k)
	m := &moduleNode{context: ctx}
	c := testLevelChecker(t, k)
	if ok, e := c.check(m, 1); e != nil || !ok {
		t.Fatalf("level: %v %v", ok, e)
	}
	constant, e := c.moduleConstant(m)
	if e != nil || !constant || m.parameterFree() {
		t.Fatalf("constant=%v parameterFree=%v err=%v", constant, m.parameterFree(), e)
	}
}

func TestFirstWinnerKeepsJavaStringAndUniqueStringDistinct(t *testing.T) {
	errors := &Errors{}
	code, _ := CodeFromValue(4206)
	p := newNamePool()
	name := p.internUnits([]uint16{'x', 0xd800})
	unique := nameParam(name)
	text := Parameter{Kind: StringParameter, JavaClass: "java.lang.String", UTF16Present: true, UTF16Units: []uint16{'x', 0xd800}, TextUnrepresentable: true}
	if _, e := errors.Add(code, nil, "%s", text); e != nil {
		t.Fatal(e)
	}
	generation := len(errors.Messages())
	if _, e := errors.Add(code, nil, "%s", unique); e != nil {
		t.Fatal(e)
	}
	if _, e := errors.Add(code, nil, "%s", text); e != nil {
		t.Fatal(e)
	}
	if _, e := errors.Add(code, nil, "%s", unique); e != nil {
		t.Fatal(e)
	}
	got := errors.Messages()
	if generation != 1 || len(got) != 2 || got[0].Parameters[0].Kind != StringParameter || got[1].Parameters[0].Kind != ObjectIdentityParameter {
		t.Fatalf("first winners: %+v", got)
	}
	bad := text
	bad.JavaClass = "util.UniqueString"
	if _, e := errors.Add(code, nil, "%s", bad); e != ErrPortIncomplete {
		t.Fatalf("contradictory class: %v", e)
	}
	if !sameParameter(Parameter{Kind: StringParameter, Text: "x"}, Parameter{Kind: StringParameter, JavaClass: "java.lang.String", Text: "x"}) {
		t.Fatal("legacy unmarked string changed")
	}
}

func TestEnabledNamedAndUnnamedInstanceComposition(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name + ".tla", Origin: modulegraph.OriginWorkspace, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	ctx := stdcontext.Background()
	dep := input("A", "K == 1")
	for _, body := range []string{"I == INSTANCE A", "INSTANCE A"} {
		a, e := modulegraph.AcquireFlatExternal(ctx, input("M", body), func(stdcontext.Context, string) (modulegraph.Input, error) { return dep, nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
		if e != nil {
			t.Fatal(e)
		}
		o := observeLevelCandidate(ctx, a, 1000)
		if o.err != nil || !o.bound || !o.levelComplete || len(o.attempts) != 2 || !o.attempts[0].completed || !o.attempts[1].completed {
			t.Fatalf("%s composition: %+v", body, o)
		}
	}
	mixed := input("M", "EXTENDS B\nI == INSTANCE A")
	a, e := modulegraph.AcquireFlatExternal(ctx, mixed, func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		if n == "A" {
			return dep, nil
		}
		return input("B", "V == 2"), nil
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if e != nil {
		t.Fatal(e)
	}
	// The general driver admits the mixed graph (general_driver_profile.go).
	o := observeLevelCandidate(ctx, a, 1000)
	if o.err != nil || !o.bound || !o.levelComplete || o.cause.driverProfile != generalDriverProfile || len(o.attempts) != 3 {
		t.Fatalf("mixed profile: %+v", o)
	}
}

func TestEnabledAssumeProveAndStructuredProof(t *testing.T) {
	root := modulegraph.Input{Path: "M.tla", Origin: modulegraph.OriginWorkspace, Text: []byte("---- MODULE M ----\nTHEOREM T == ASSUME NEW x PROVE x = x\n<1>1. TRUE OBVIOUS\n<1> QED OBVIOUS\n====\n")}
	ctx := stdcontext.Background()
	a, e := modulegraph.AcquireFlatExternal(ctx, root, func(stdcontext.Context, string) (modulegraph.Input, error) {
		return modulegraph.Input{}, ErrPortIncomplete
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if e != nil {
		t.Fatal(e)
	}
	o := observeLevelCandidate(ctx, a, 1000)
	if o.err != nil || !o.bound || !o.levelComplete || len(o.attempts) != 1 || !o.attempts[0].completed {
		t.Fatalf("AP proof: %+v", o)
	}
}

func TestGenerationErrorsSkipAllLevelsWithoutLosingExactPrefix(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name + ".tla", Origin: modulegraph.OriginWorkspace, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	ctx := stdcontext.Background()
	dep := input("A", "THEOREM TRUE\n<1>1. USE MODULE A\n<1>2. <1>1(123)\n<1> QED OBVIOUS\nAfter == TRUE")
	root := input("M", "EXTENDS A\nB == TRUE")
	a, e := modulegraph.AcquireFlatExternal(ctx, root, func(stdcontext.Context, string) (modulegraph.Input, error) { return dep, nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if e != nil {
		t.Fatal(e)
	}
	o := observeLevelCandidate(ctx, a, 1000)
	if o.err != nil || !o.bound || !o.generationComplete || !o.levelComplete || len(o.generation) != 1 || len(o.attempts) != 2 || !o.attempts[0].skipped || !o.attempts[1].skipped {
		t.Fatalf("candidate: %+v", o)
	}
}
