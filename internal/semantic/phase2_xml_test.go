//go:build prudence_phase2_adapter

// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	"bytes"
	stdcontext "context"
	"encoding/xml"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/modulegraph"
)

func TestPhase2XMLSharedIdentityAndCycle(t *testing.T) {
	p := newNamePool()
	c := newContext(p)
	m, e := newModule(p, p.intern("M"), p.syntax.NewBuiltinSyntaxTreeNode(p.syntax.UniqueStringOf("M")), c)
	if e != nil {
		t.Fatal(e)
	}
	d, e := newUserOpDef(p, p.intern("A"), nil, false, nil, m, nil, p.syntax.NewBuiltinSyntaxTreeNode(p.syntax.UniqueStringOf("A")))
	if e != nil {
		t.Fatal(e)
	}
	n, e := newNumeral(p, "1", p.syntax.NewBuiltinSyntaxTreeNode(p.syntax.UniqueStringOf("1")))
	if e != nil {
		t.Fatal(e)
	}
	d.body = n
	c.add(contextKey{name: d.name}, d)
	var b bytes.Buffer
	if e = phase2XML(stdcontext.Background(), &b, m, 10, t.TempDir(), 1<<20); e != nil {
		t.Fatal(e)
	}
	var root any
	if e = xml.Unmarshal(b.Bytes(), &root); e != nil {
		t.Fatal(e)
	}
	s := b.String()
	if strings.Count(s, "<UserDefinedOpKindRef>") != 2 || !strings.Contains(s, "<originalOperator><UserDefinedOpKindRef>") || !strings.Contains(s, "<originallyDefinedInModule><ModuleNodeRef>") {
		t.Fatal(s)
	}
}
func TestPhase2XMLBoundsAndNull(t *testing.T) {
	p := newNamePool()
	c := newContext(p)
	m, e := newModule(p, p.intern("M"), p.syntax.NewBuiltinSyntaxTreeNode(p.syntax.UniqueStringOf("M")), c)
	if e != nil {
		t.Fatal(e)
	}
	d, e := newUserOpDef(p, p.intern("A"), nil, false, nil, m, nil, p.syntax.NewBuiltinSyntaxTreeNode(p.syntax.UniqueStringOf("A")))
	if e != nil {
		t.Fatal(e)
	}
	c.add(contextKey{name: d.name}, d)
	if e = phase2XML(stdcontext.Background(), &bytes.Buffer{}, m, 1, t.TempDir(), 1<<20); e == nil {
		t.Fatal("expected symbol limit")
	}
	if e = phase2XML(stdcontext.Background(), &bytes.Buffer{}, m, 10, t.TempDir(), 1<<20); e == nil {
		t.Fatal("expected missing body")
	}
}
func TestJavaWhitespace(t *testing.T) {
	if !javaWhitespace('\u2000') || !javaWhitespace('\u001c') || javaWhitespace('\u00a0') || javaWhitespace('\u2007') || javaWhitespace('\u202f') {
		t.Fatal("Java whitespace mismatch")
	}
}

func TestPhase2XMLExceptAndCycles(t *testing.T) {
	p := newNamePool()
	tree := p.syntax.NewBuiltinSyntaxTreeNode(p.syntax.UniqueStringOf("x"))
	one, e := newNumeral(p, "1", tree)
	if e != nil {
		t.Fatal(e)
	}
	two, e := newNumeral(p, "2", tree)
	if e != nil {
		t.Fatal(e)
	}
	h, e := newExprHeader(p, 9, tree)
	if e != nil {
		t.Fatal(e)
	}
	exc := &opApplNode{exprHeader: h, operands: []exprOrOpArg{one}}
	h, e = newExprHeader(p, 9, tree)
	if e != nil {
		t.Fatal(e)
	}
	component := &opApplNode{exprHeader: h, operands: []exprOrOpArg{two}}
	h, e = newExprHeader(p, 0, tree)
	if e != nil {
		t.Fatal(e)
	}
	at := &atNode{exprHeader: h, except: exc, component: component}
	var b bytes.Buffer
	x := &xmlExporter{ctx: stdcontext.Background(), enc: xml.NewEncoder(&b), seen: map[levelNode]bool{}, active: map[any]bool{}, maxNodes: 20}
	x.expr(at, false)
	if x.err != nil {
		t.Fatal(x.err)
	}
	if e = x.enc.Flush(); e != nil {
		t.Fatal(e)
	}
	s := b.String()
	if !strings.Contains(s, "<IntValue>1</IntValue>") || !strings.Contains(s, "<IntValue>2</IntValue>") {
		t.Fatal(s)
	}
	exc.operands[0] = at
	x = &xmlExporter{ctx: stdcontext.Background(), enc: xml.NewEncoder(&bytes.Buffer{}), seen: map[levelNode]bool{}, active: map[any]bool{}, maxNodes: 20}
	x.expr(at, false)
	if x.err == nil {
		t.Fatal("cyclic EXCEPT must be incomplete")
	}
}
func TestPhase2XMLNilAndEmptyParams(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params []*formalParamNode
		want   bool
	}{{"nil", nil, false}, {"empty", []*formalParamNode{}, true}} {
		t.Run(tc.name, func(t *testing.T) {
			p := newNamePool()
			tree := p.syntax.NewBuiltinSyntaxTreeNode(p.syntax.UniqueStringOf("x"))
			h, e := newNodeHeader(p, p.intern("b"), kindBuiltIn, tree)
			if e != nil {
				t.Fatal(e)
			}
			n := &builtinOpDefNode{nodeHeader: h, params: tc.params}
			var b bytes.Buffer
			x := &xmlExporter{ctx: stdcontext.Background(), enc: xml.NewEncoder(&b), seen: map[levelNode]bool{}, active: map[any]bool{}, maxNodes: 20}
			x.definition(n)
			if x.err != nil {
				t.Fatal(x.err)
			}
			if e = x.enc.Flush(); e != nil {
				t.Fatal(e)
			}
			if strings.Contains(b.String(), "<params>") != tc.want {
				t.Fatal(b.String())
			}
		})
	}
}

// SANY's SymbolContext copy aliases the OTHER flag. A CASE sets it before
// recursively exporting its operator and subsequent operands. The second
// definition below is first reached through the first CASE operand.
func TestCaseOtherFlagFollowsDefinitionTraversal(t *testing.T) {
	for _, caseFirst := range []bool{true, false} {
		t.Run(strconv.FormatBool(caseFirst), func(t *testing.T) {
			p := newNamePool()
			tree := p.syntax.NewBuiltinSyntaxTreeNode(p.syntax.UniqueStringOf("x"))
			c := newContext(p)
			m, e := newModule(p, p.intern("M"), tree, c)
			if e != nil {
				t.Fatal(e)
			}
			builtin := func(name string) *builtinOpDefNode {
				h, e := newNodeHeader(p, p.intern(name), kindBuiltIn, tree)
				if e != nil {
					t.Fatal(e)
				}
				return &builtinOpDefNode{nodeHeader: h, arity: -1}
			}
			app := func(op leafNode, args ...exprOrOpArg) *opApplNode {
				h, e := newExprHeader(p, 9, tree)
				if e != nil {
					t.Fatal(e)
				}
				return &opApplNode{exprHeader: h, operator: op, operands: args}
			}
			a, e := newUserOpDef(p, p.intern("A"), nil, false, nil, m, nil, tree)
			if e != nil {
				t.Fatal(e)
			}
			b, e := newUserOpDef(p, p.intern("B"), nil, false, nil, m, nil, tree)
			if e != nil {
				t.Fatal(e)
			}
			caseOp, pairOp, otherOp := builtin("$Case"), builtin("$Pair"), builtin("$Synthetic")
			n, e := newNumeral(p, "1", tree)
			if e != nil {
				t.Fatal(e)
			}
			b.body = app(otherOp, nil)
			a.body = app(caseOp, app(b), app(pairOp, nil, n))
			if caseFirst {
				c.add(contextKey{name: a.name}, a)
				c.add(contextKey{name: b.name}, b)
			} else {
				c.add(contextKey{name: b.name}, b)
				c.add(contextKey{name: a.name}, a)
			}
			dir := t.TempDir()
			var out bytes.Buffer
			e = phase2XML(stdcontext.Background(), &out, m, 100, dir, 1<<20)
			entries, readErr := os.ReadDir(dir)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("spool cleanup: %v %v", entries, readErr)
			}
			if caseFirst {
				if e != nil {
					t.Fatal(e)
				}
				if strings.Count(out.String(), "<StringValue>$Other</StringValue>") != 2 {
					t.Fatal(out.String())
				}
			} else if e == nil {
				t.Fatal("null before CASE must be incomplete")
			}
		})
	}
}
func TestSpoolLimitCleansUp(t *testing.T) {
	p := newNamePool()
	c := newContext(p)
	tree := p.syntax.NewBuiltinSyntaxTreeNode(p.syntax.UniqueStringOf("M"))
	m, e := newModule(p, p.intern("M"), tree, c)
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	if e = phase2XML(stdcontext.Background(), &bytes.Buffer{}, m, 10, dir, 1); e == nil {
		t.Fatal("missing spool limit error")
	}
	entries, e := os.ReadDir(dir)
	if e != nil || len(entries) != 0 {
		t.Fatalf("spool cleanup: %v %v", entries, e)
	}
}

// Hashtable enumerates higher buckets first; declaration caches reverse that
// order. ASCII A and B occupy buckets 10 and 0 in the initial 11 buckets.
func TestDeclarationOrderLocationAndConstructorLevel(t *testing.T) {
	p := newNamePool()
	tree := p.syntax.NewBuiltinSyntaxTreeNode(p.syntax.UniqueStringOf("x"))
	c := newContext(p)
	m, e := newModule(p, p.intern("M"), tree, c)
	if e != nil {
		t.Fatal(e)
	}
	a, e := newOpDecl(p, p.intern("A"), 2, 0, 0, tree, nil, m)
	if e != nil {
		t.Fatal(e)
	}
	b, e := newOpDecl(p, p.intern("B"), 2, 0, 0, tree, nil, m)
	if e != nil {
		t.Fatal(e)
	}
	c.add(contextKey{name: a.name}, a)
	c.add(contextKey{name: b.name}, b)
	var out bytes.Buffer
	if e = phase2XML(stdcontext.Background(), &out, m, 10, t.TempDir(), 1<<20); e != nil {
		t.Fatal(e)
	}
	s := out.String()
	begin := strings.Index(s, "<ModuleNode>")
	end := strings.Index(s, "</ModuleNode>")
	if begin < 0 || end < begin {
		t.Fatal(s)
	}
	module := s[begin:end]
	ref := func(id uint64) string {
		return "<OpDeclNodeRef><UID>" + strconv.FormatUint(id, 10) + "</UID></OpDeclNodeRef>"
	}
	if strings.Index(module, ref(b.identity)) < 0 || strings.Index(module, ref(a.identity)) < strings.Index(module, ref(b.identity)) {
		t.Fatal(module)
	}
	if !strings.Contains(s, "</location><level>0</level><uniquename>A</uniquename>") || !strings.Contains(s, "<filename>--TLA+ BUILTINS--</filename>") {
		t.Fatal(s)
	}
}

func TestNestedModuleGraphIsAvailable(t *testing.T) {
	tree := parsedModule(t, "---- MODULE Root ----\n---- MODULE Inner ----\nX == 1\n====\n====\n")
	result := Phase2Observe(stdcontext.Background(), tree, 0)
	if !result.Complete {
		t.Fatal(result.Err)
	}
	var out bytes.Buffer
	if err := result.WriteXML(stdcontext.Background(), &out, 100, t.TempDir(), 1<<20); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "<uniquename>Inner</uniquename>") || strings.Count(out.String(), "<ModuleNodeRef>") < 2 {
		t.Fatal(out.String())
	}
}

func TestMultiunitRootsAndInheritedInstances(t *testing.T) {
	p := newNamePool()
	tree := p.syntax.NewBuiltinSyntaxTreeNode(p.syntax.UniqueStringOf("T"))
	makeModule := func(name string) *moduleNode {
		m, err := newModule(p, p.intern(name), tree, newContext(p))
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	a, b, root := makeModule("A"), makeModule("B"), makeModule("Root")
	inst := &instanceNode{semanticHeader: semanticHeader{owner: p, tree: tree}, module: a}
	b.topLevel = []levelNode{inst}
	root.extendees = []*moduleNode{b, a, b}
	root.topLevel = []levelNode{inst, inst}
	var out bytes.Buffer
	if err := phase2XMLRoots(stdcontext.Background(), &out, root, []*moduleNode{a, b, root}, 100, t.TempDir(), 1<<20); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "<extends><uniquename>A</uniquename><uniquename>B</uniquename></extends>") || strings.Count(s, "<InstanceNode>") != 3 {
		t.Fatal(s)
	}
	tail := s[strings.Index(s, "</context>")+len("</context>"):]
	want := func(m *moduleNode) string {
		return "<ModuleNodeRef><UID>" + strconv.FormatUint(m.identity, 10) + "</UID></ModuleNodeRef>"
	}
	if tail != want(a)+want(b)+want(root)+"</modules>" {
		t.Fatal(tail)
	}
}

func TestInstanceSubstitutionXMLAndRootPublication(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name + ".tla", Origin: modulegraph.OriginWorkspace, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	root := input("Root", "CONSTANT X\nI(p) == INSTANCE A WITH Y <- p\nR == I(1)!F(2)")
	dep := input("A", "CONSTANT X, Y\nF(z) == X\nG == Y")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, _ string) (modulegraph.Input, error) { return dep, nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	analysis := Phase2ObserveFlat(stdcontext.Background(), a, 1000)
	if !analysis.Complete || len(analysis.Details) != 0 || len(analysis.roots) != 2 || analysis.roots[0].name.text != "A" || analysis.roots[1] != analysis.module {
		t.Fatalf("%+v", analysis)
	}
	var out bytes.Buffer
	if err = analysis.WriteXML(stdcontext.Background(), &out, 10000, t.TempDir(), 1<<20); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, fragment := range []string{"<RootModule>Root</RootModule>", "<InstanceNode>", "<level>0</level><uniquename>I</uniquename><module>A</module><substs>", "<SubstInNode>", "<instFrom><ModuleNodeRef>", "<instTo><ModuleNodeRef>"} {
		if !strings.Contains(s, fragment) {
			t.Fatalf("missing %s in %s", fragment, s)
		}
	}
}

// The two-stage INSTANCE fixture exercises operator and expression
// substitutions, local import clones, and module backreferences together.
func TestChainedLocalOperatorSubstitutionExport(t *testing.T) {
	read := func(n string) modulegraph.Input {
		data, err := os.ReadFile("testdata/multiunit/" + n + ".tla")
		if err != nil {
			t.Fatal(err)
		}
		return modulegraph.Input{Path: n + ".tla", Text: data, Origin: modulegraph.OriginWorkspace}
	}
	root := read("Root")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, n string) (modulegraph.Input, error) { return read(n), nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	analysis := Phase2ObserveFlat(stdcontext.Background(), a, 1000)
	if !analysis.Complete || len(analysis.Details) != 0 {
		t.Fatalf("%v %+v", analysis.Err, analysis.Details)
	}
	var out bytes.Buffer
	if err := analysis.WriteXML(stdcontext.Background(), &out, 10000, t.TempDir(), 1<<20); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, fragment := range []string{"<module>A</module>", "<module>B</module>", "<OpArgNode>", "<SubstInNode>", "<local></local>", "<instFrom><ModuleNodeRef>", "<instTo><ModuleNodeRef>"} {
		if !strings.Contains(s, fragment) {
			t.Fatalf("missing %s in %s", fragment, s)
		}
	}
	if len(analysis.module.instances) != 2 || !analysis.module.instances[1].local {
		t.Fatal("local instance not retained")
	}
}

type failingXMLWriter struct{}

func (failingXMLWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestMultiunitXMLFailureCleanupAndOrigin(t *testing.T) {
	input := func(n, body string, origin uint8) modulegraph.Input {
		return modulegraph.Input{Path: n + ".tla", Text: []byte("---- MODULE " + n + " ----\n" + body + "\n====\n"), Origin: origin}
	}
	root, dep := input("Root", "EXTENDS A", modulegraph.OriginWorkspace), input("A", "X == 1", modulegraph.OriginStandard)
	acquire := func() *modulegraph.Acquisition {
		a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, _ string) (modulegraph.Input, error) { return dep, nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	analysis := Phase2ObserveFlat(stdcontext.Background(), acquire(), 1000)
	if !analysis.Complete {
		t.Fatal(analysis.Err)
	}
	for _, cancelled := range []bool{false, true} {
		dir := t.TempDir()
		ctx := stdcontext.Background()
		if cancelled {
			c, cancel := stdcontext.WithCancel(ctx)
			cancel()
			ctx = c
		}
		if err := analysis.WriteXML(ctx, failingXMLWriter{}, 10000, dir, 1<<20); err == nil {
			t.Fatal("failure accepted")
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			t.Fatalf("spools: %v %v", entries, err)
		}
	}
	invalid := acquire()
	invalid.Units[1].Input.Origin = 255
	bad := Phase2ObserveFlat(stdcontext.Background(), invalid, 1000)
	if bad.Complete || bad.Err == nil {
		t.Fatal("unknown origin accepted")
	}
	var out bytes.Buffer
	if err := bad.WriteXML(stdcontext.Background(), &out, 10000, t.TempDir(), 1<<20); err == nil || out.Len() != 0 {
		t.Fatalf("partial graph %v", err)
	}
}

func TestStatementGraphMapsWrappersAndScopes(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nASSUME A == TRUE\nTHEOREM T == ASSUME NEW x PROVE x = x OBVIOUS\n====\n")
	result := Phase2Observe(stdcontext.Background(), tree, 0)
	var out bytes.Buffer
	if err := result.WriteXML(stdcontext.Background(), &out, 100, t.TempDir(), 1<<20); err != nil {
		t.Fatal(err)
	}
	var limited bytes.Buffer
	if err := result.WriteXML(stdcontext.Background(), &limited, 2, t.TempDir(), 1<<20); err == nil || limited.Len() != 0 {
		t.Fatal("limited graph escaped")
	}
	text := out.String()
	for _, tag := range []string{"<AssumeNodeRef>", "<TheoremNodeRef>", "<AssumeDefRef>", "<TheoremDefRef>", "<AssumeProveNode>", "<NewSymbNode>", "<obvious>"} {
		if !strings.Contains(text, tag) {
			t.Fatalf("missing %s: %s", tag, text)
		}
	}
}

// The shared statement identities must survive both the EXTENDS diamond and
// the interleaved instance sequence in every published external module.
func TestStatementDiamondAndInterleavedInstanceExport(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name + ".tla", Origin: modulegraph.OriginWorkspace, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	files := map[string]modulegraph.Input{
		"A": input("A", "ASSUME H == TRUE\nTHEOREM T == ASSUME NEW x, x = x PROVE x = x"),
		"B": input("B", "EXTENDS A"),
		"C": input("C", "EXTENDS A"),
		"D": input("D", "V == 1"),
	}
	root := input("Root", "EXTENDS B, C\nASSUME TRUE\nINSTANCE D\nTHEOREM TRUE OBVIOUS")
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), root, func(_ stdcontext.Context, n string) (modulegraph.Input, error) { return files[n], nil }, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	analysis := Phase2ObserveFlat(stdcontext.Background(), a, 1000)
	if !analysis.Complete || len(analysis.Details) != 0 {
		t.Fatalf("%v %+v", analysis.Err, analysis.Details)
	}
	seq := analysis.module.topLevel
	if len(seq) != 7 || seq[0] != seq[2] || seq[1] != seq[3] {
		t.Fatalf("diamond sequence: %#v", seq)
	}
	if _, ok := seq[4].(*assumeNode); !ok {
		t.Fatal("missing assumption")
	}
	if _, ok := seq[5].(*instanceNode); !ok {
		t.Fatal("missing instance")
	}
	if _, ok := seq[6].(*theoremNode); !ok {
		t.Fatal("missing theorem")
	}
	var out bytes.Buffer
	if err := analysis.WriteXML(stdcontext.Background(), &out, 10000, t.TempDir(), 1<<20); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	assume := seq[0].(*assumeNode)
	theorem := seq[1].(*theoremNode)
	ref := func(tag string, id uint64) string {
		return "<" + tag + "><UID>" + strconv.FormatUint(id, 10) + "</UID></" + tag + ">"
	}
	for _, pair := range []struct {
		tag string
		id  uint64
	}{{"AssumeNodeRef", assume.identity}, {"TheoremNodeRef", theorem.identity}} {
		if strings.Count(s, ref(pair.tag, pair.id)) != 5 {
			t.Fatalf("shared %s: %s", pair.tag, s)
		}
	}
	for _, fragment := range []string{"<AssumeDefRef>", "<TheoremDefRef>", "<AssumeProveNode>", "<NewSymbNode>", "<InstanceNode>", "<obvious>"} {
		if !strings.Contains(s, fragment) {
			t.Fatalf("missing %s: %s", fragment, s)
		}
	}
	tail := s[strings.Index(s, "</context>")+len("</context>"):]
	var want string
	for _, m := range analysis.roots {
		want += ref("ModuleNodeRef", m.identity)
	}
	if tail != want+"</modules>" {
		t.Fatal(tail)
	}
}

// An imported theorem definition can be reached by a reference even though
// INSTANCE does not publish a statement wrapper for that definition.
func TestImportedTheoremAPSubstitutionXML(t *testing.T) {
	input := func(name, body string) modulegraph.Input {
		return modulegraph.Input{Path: name + ".tla", Origin: modulegraph.OriginWorkspace, Text: []byte("---- MODULE " + name + " ----\n" + body + "\n====\n")}
	}
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), input("B", "CONSTANT x\nINSTANCE A"), func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		return input("A", "CONSTANT x\nTHEOREM T == ASSUME NEW y, y = x PROVE y = x"), nil
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	analysis := Phase2ObserveFlat(stdcontext.Background(), a, 1000)
	if !analysis.Complete || len(analysis.Details) != 0 {
		t.Fatalf("%v %+v", analysis.Err, analysis.Details)
	}
	def := analysis.module.definitions[0].(*thmOrAssumpDefNode)
	if _, ok := def.body.(*apSubstInNode); !ok {
		t.Fatal("missing AP substitution")
	}
	dir := t.TempDir()
	x := &xmlExporter{ctx: stdcontext.Background(), enc: xml.NewEncoder(io.Discard), seen: make(map[levelNode]bool), active: make(map[any]bool), maxNodes: 10000, spoolDir: dir, spoolLimit: 1 << 20}
	x.ref(def)
	if x.err != nil {
		t.Fatal(x.err)
	}
	var all strings.Builder
	for _, path := range x.spools {
		b, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		all.Write(b)
	}
	for _, fragment := range []string{"<TheoremDefNode>", "<APSubstInNode>", "<Subst>", "<AssumeProveNode>", "<NewSymbNode>", "<instFrom><ModuleNodeRef>", "<instTo><ModuleNodeRef>"} {
		if !strings.Contains(all.String(), fragment) {
			t.Fatalf("missing %s: %s", fragment, all.String())
		}
	}
}

func TestInlineLevelCycleRejected(t *testing.T) {
	pool := newNamePool()
	tree := pool.syntax.NewBuiltinSyntaxTreeNode(pool.syntax.UniqueStringOf("T"))
	n := &assumeProveNode{semanticHeader: semanticHeader{owner: pool, tree: tree}}
	n.assumptions = []levelNode{n}
	x := &xmlExporter{ctx: stdcontext.Background(), enc: xml.NewEncoder(io.Discard), active: make(map[any]bool), maxNodes: 100}
	x.inlineLevel(n)
	if x.err == nil || !strings.Contains(x.err.Error(), "cyclic level") {
		t.Fatalf("%v", x.err)
	}
}

func TestPhase2XMLCommentLineEndings(t *testing.T) {
	source := "---- MODULE M ----\n(* first\r\nsecond\rthird *)\nF == 1\n====\n"
	g, m, err := analyzeOwned(stdcontext.Background(), parsedModule(t, source), 0)
	if err != nil || len(g.table.errors.Messages()) != 0 {
		t.Fatalf("%v", err)
	}
	var b bytes.Buffer
	if err = phase2XML(stdcontext.Background(), &b, m, 100, t.TempDir(), 1<<20); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "<pre-comments>(* first\nsecond\nthird *)</pre-comments>") {
		t.Fatal(b.String())
	}
}

func TestProofXMLClosureAndUnsupportedAlias(t *testing.T) {
	for _, tc := range []struct {
		name, steps string
		unsupported bool
	}{
		{"steps", "<1>1. TRUE BY ONLY MODULE M\n<1> QED OBVIOUS\n", false},
		{"alias", "<1>1. DEFINE F == TRUE\n<1>2. USE F DEF <1>1\n<1> QED OBVIOUS\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, err := newGenerator(stdcontext.Background(), 0)
			if err != nil {
				t.Fatal(err)
			}
			module, err := g.module(parsedModule(t, "---- MODULE M ----\n====\n"))
			if err != nil {
				t.Fatal(err)
			}
			syntax := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n"+tc.steps+"====\n")
			var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
			find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
				if n.Kind() == generated.N_Proof {
					return n
				}
				for _, c := range n.Heirs() {
					if v := find(c); v != nil {
						return v
					}
				}
				return nil
			}
			tree := find(syntax)
			proof, err := g.proof(tree, module, 1)
			if err != nil {
				t.Fatal(err)
			}
			body, err := g.builtinApplication("TRUE", nil, tree)
			if err != nil {
				t.Fatal(err)
			}
			h, err := newSemanticHeader(g.pool, 12, tree)
			if err != nil {
				t.Fatal(err)
			}
			module.topLevel = append(module.topLevel, &theoremNode{semanticHeader: h, body: body, proof: proof, module: module})
			dir := t.TempDir()
			var out bytes.Buffer
			err = phase2XML(stdcontext.Background(), &out, module, 300, dir, 1<<20)
			entries, readErr := os.ReadDir(dir)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("spool cleanup: %v %v", entries, readErr)
			}
			if tc.unsupported {
				if err == nil || !strings.Contains(err.Error(), "numbered proof step") {
					t.Fatalf("alias export: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			xml := out.String()
			for _, required := range []string{"<steps>", "<proofLevel>1</proofLevel>", "<by>", "<facts>", "<ModuleNodeRef>", "<defs>", "<only>", "<obvious>"} {
				if !strings.Contains(xml, required) {
					t.Fatalf("missing %s in %s", required, xml)
				}
			}
		})
	}
}

func TestObservedProofGraphAndAliasAvailability(t *testing.T) {
	for _, tc := range []struct {
		name, proof string
		gap         bool
	}{
		{"available", "<1>1. TRUE BY ONLY MODULE M\n<1> QED OBVIOUS", false},
		{"unreferenced definition alias", "<1>1. DEFINE F == TRUE\n<1>2. USE F\n<1> QED OBVIOUS", false},
		{"alias gap", "<1>1. DEFINE F == TRUE\n<1>2. USE F DEF <1>1\n<1> QED OBVIOUS", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n"+tc.proof+"\n====\n")
			analysis := Phase2Observe(stdcontext.Background(), tree, 512)
			if !analysis.Complete || len(analysis.Details) != 0 {
				t.Fatalf("semantic observation: %+v", analysis)
			}
			var out bytes.Buffer
			dir := t.TempDir()
			err := analysis.WriteXML(stdcontext.Background(), &out, 1000, dir, 1<<20)
			entries, readErr := os.ReadDir(dir)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("spool cleanup: %v %v", entries, readErr)
			}
			if tc.gap {
				if err == nil || !strings.Contains(err.Error(), "numbered proof step") {
					t.Fatalf("missing graph gap: %v", err)
				}
				return
			}
			if err != nil || !strings.Contains(out.String(), "<proofLevel>1</proofLevel>") {
				t.Fatalf("proof graph: %v %s", err, out.String())
			}
			if tc.name == "unreferenced definition alias" && (!strings.Contains(out.String(), "<DefStepNode>") || !strings.Contains(out.String(), "<UseOrHideNode>") || strings.Contains(out.String(), "NumberedProofStep")) {
				t.Fatal("definition-step export closure")
			}
		})
	}
}

func TestProofExportLimitsAndCancellationCleanUp(t *testing.T) {
	tree := parsedModule(t, "---- MODULE M ----\nTHEOREM TRUE\n<1>1. TRUE BY ONLY MODULE M\n<1> QED OBVIOUS\n====\n")
	analysis := Phase2Observe(stdcontext.Background(), tree, 512)
	if !analysis.Complete {
		t.Fatalf("semantic observation: %+v", analysis)
	}
	canceled, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	for _, tc := range []struct {
		name    string
		ctx     stdcontext.Context
		symbols int
		bytes   int64
	}{
		{"cancel", canceled, 1000, 1 << 20},
		{"bytes", stdcontext.Background(), 1000, 1},
		{"symbols", stdcontext.Background(), 1, 1 << 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := analysis.WriteXML(tc.ctx, &bytes.Buffer{}, tc.symbols, dir, tc.bytes); err == nil {
				t.Fatal("expected bounded export failure")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("spool cleanup: %v %v", entries, err)
			}
			if !analysis.Complete {
				t.Fatal("export failure changed semantic availability")
			}
		})
	}
}
