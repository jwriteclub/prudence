//go:build prudence_phase2_adapter

// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Source mapping: TLA+ XMLExporter and semantic node exports, Copyright 2003
// Compaq Corporation and Microsoft Corporation, under the MIT license; see licenses/TLAplus.txt and NOTICE.
package semantic

import (
	stdcontext "context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

// xmlExporter streams one request's XML. Symbol discovery follows source-order
// depth-first traversal and registers each identity before expanding its definition.
type xmlExporter struct {
	ctx        stdcontext.Context
	enc        *xml.Encoder
	seen       map[levelNode]bool
	spools     []string
	spoolDir   string
	spoolBytes int64
	spoolLimit int64
	depth      int
	other      bool
	maxNodes   int
	active     map[any]bool
	visits     int
	err        error
}

func (x *xmlExporter) token(t xml.Token) {
	if x.err == nil {
		x.err = x.ctx.Err()
		if x.err == nil {
			x.err = x.enc.EncodeToken(t)
		}
	}
}
func (x *xmlExporter) start(s string) { x.token(xml.StartElement{Name: xml.Name{Local: s}}) }
func (x *xmlExporter) end(s string)   { x.token(xml.EndElement{Name: xml.Name{Local: s}}) }
func (x *xmlExporter) text(s, v string) {
	for _, r := range v {
		if !(r == 0x9 || r == 0xa || r == 0xd || r >= 0x20 && r <= 0xd7ff || r >= 0xe000 && r <= 0xfffd || r >= 0x10000 && r <= 0x10ffff) {
			x.fail("text is not XML 1.0 representable")
			return
		}
	}
	x.start(s)
	x.token(xml.CharData(v))
	x.end(s)
}
func (x *xmlExporter) num(s string, v int) { x.text(s, strconv.Itoa(v)) }
func (x *xmlExporter) fail(s string) {
	if x.err == nil {
		x.err = errors.New(s)
	}
}
func (x *xmlExporter) named(s string, n *name) {
	if n == nil || !n.displayExact {
		x.fail("XML text has missing or unrepresentable UTF-16 name")
		return
	}
	x.text(s, n.text)
}
func (x *xmlExporter) ref(n levelNode) {
	if n == nil {
		x.fail("nil symbol reference")
		return
	}
	var tag string
	switch n.(type) {
	case *moduleNode:
		tag = "ModuleNodeRef"
	case *formalParamNode:
		tag = "FormalParamNodeRef"
	case *opDeclNode:
		tag = "OpDeclNodeRef"
	case *builtinOpDefNode:
		tag = "BuiltInKindRef"
	case *userOpDefNode:
		if n.(*userOpDefNode).kind == 37 {
			x.fail("numbered proof step kind is unsupported by Java XML export")
			return
		}
		if n.(*userOpDefNode).kind == 6 {
			tag = "ModuleInstanceKindRef"
		} else {
			tag = "UserDefinedOpKindRef"
		}
	case *thmOrAssumpDefNode:
		if n.(*thmOrAssumpDefNode).theorem {
			tag = "TheoremDefRef"
		} else {
			tag = "AssumeDefRef"
		}
	case *theoremNode:
		tag = "TheoremNodeRef"
	case *assumeNode:
		tag = "AssumeNodeRef"
	default:
		x.fail("unsupported symbol type")
		return
	}
	if !x.seen[n] {
		if len(x.seen) >= x.maxNodes {
			x.fail("symbol limit")
			return
		}
		// Java registers the UID before invoking exportDefinition recursively.
		x.seen[n] = true
		x.spoolDefinition(n)
		if x.err != nil {
			return
		}
	}
	x.start(tag)
	x.text("UID", strconv.FormatUint(n.levelHeader().identity, 10))
	x.end(tag)
}

// A single budget covers all spools, including partially written entries.
type spoolWriter struct {
	x *xmlExporter
	w io.Writer
}

func (s spoolWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > s.x.spoolLimit-s.x.spoolBytes {
		return 0, errors.New("XML spool byte limit")
	}
	n, e := s.w.Write(p)
	s.x.spoolBytes += int64(n)
	return n, e
}
func (x *xmlExporter) spoolDefinition(n levelNode) {
	if x.depth >= 128 {
		x.fail("symbol definition depth limit")
		return
	}
	f, e := os.CreateTemp(x.spoolDir, "entry-*.xml")
	if e != nil {
		x.fail("create XML spool: " + e.Error())
		return
	}
	x.spools = append(x.spools, f.Name())
	x.depth++
	previous := x.enc
	x.enc = xml.NewEncoder(spoolWriter{x: x, w: f})
	x.start("entry")
	x.text("UID", strconv.FormatUint(n.levelHeader().identity, 10))
	x.definition(n)
	x.end("entry")
	if x.err == nil {
		x.err = x.enc.Flush()
	}
	if e = f.Close(); x.err == nil {
		x.err = e
	}
	x.enc = previous
	x.depth--
}
func (x *xmlExporter) location(h *semanticHeader) {
	if h == nil || h.tree == nil {
		x.fail("missing XML source location")
		return
	}
	l := h.tree.Location()
	if l == nil || l.Source() == nil {
		x.fail("missing XML source location")
		return
	}
	src := h.owner.internUnits(l.Source().Units())
	if !src.displayExact {
		x.fail("unrepresentable XML filename")
		return
	}
	x.start("location")
	x.start("column")
	x.num("begin", int(l.BeginColumn()))
	x.num("end", int(l.EndColumn()))
	x.end("column")
	x.start("line")
	x.num("begin", int(l.BeginLine()))
	x.num("end", int(l.EndLine()))
	x.end("line")
	x.named("filename", src)
	x.end("location")
}
func (x *xmlExporter) level(l leafLevel) {
	if l.checked != 0 {
		x.num("level", l.level)
	}
}
func (x *xmlExporter) params(ps []*formalParamNode, leib []bool) {
	x.start("params")
	for i, p := range ps {
		x.start("leibnizparam")
		x.ref(p)
		if i < len(leib) && leib[i] {
			x.start("leibniz")
			x.end("leibniz")
		}
		x.end("leibnizparam")
	}
	x.end("params")
}

// Java getAttachedComments follows the first descendant to a token. Java
// stripTrailing uses Character.isWhitespace (excluding the three NBSPs).
func (x *xmlExporter) comments(t *generated.SyntaxTreeNode) {
	for t != nil && t.Kind() >= generated.NULL_ID {
		heirs := t.Heirs()
		if len(heirs) == 0 {
			return
		}
		t = heirs[0]
	}
	if t == nil {
		x.fail("missing comments source tree")
		return
	}
	comments := t.PreComments()
	if len(comments) == 0 {
		return
	}
	values := make([]string, 0, len(comments))
	for _, c := range comments {
		if c == nil {
			continue
		}
		n := toplevelName(c.Units())
		if !n.ok {
			x.fail("unrepresentable comment UTF-16")
			return
		}
		value := strings.TrimRightFunc(n.text, javaWhitespace)
		if value != "" {
			values = append(values, value)
		}
	}
	// Java writes these comments as CDATA. XML 1.0 normalizes literal CRLF
	// and CR in that CDATA to LF when the graph is read.
	text := strings.Join(values, "\n")
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	x.text("pre-comments", text)
}

type xmlText struct {
	text string
	ok   bool
}

func toplevelName(units []uint16) xmlText {
	runes := utf16.Decode(units)
	back := utf16.Encode(runes)
	if len(back) != len(units) {
		return xmlText{}
	}
	for i := range back {
		if back[i] != units[i] {
			return xmlText{}
		}
	}
	return xmlText{string(runes), true}
}
func javaWhitespace(r rune) bool {
	return r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r' || r >= 0x1c && r <= 0x1f || (unicode.Is(unicode.Zs, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r)) && r != 0xa0 && r != 0x2007 && r != 0x202f
}
func (x *xmlExporter) definition(n levelNode) {
	switch v := n.(type) {
	case *moduleNode:
		x.start("ModuleNode")
		x.location(&v.semanticHeader)
		x.named("uniquename", v.name)
		x.start("extends")
		seen := make(map[*moduleNode]bool)
		var names []*name
		for _, m := range v.extendees {
			if m == nil {
				x.fail("nil extendee")
				continue
			}
			if !seen[m] {
				seen[m] = true
				names = append(names, m.name)
			}
		}
		sort.Slice(names, func(i, j int) bool { return javaNameLess(names[i], names[j]) })
		for _, n := range names {
			x.named("uniquename", n)
		}
		x.end("extends")
		v.hasDeclarations(2)
		for _, d := range v.constantCache {
			x.ref(d)
		}
		v.hasDeclarations(3)
		for _, d := range v.variableCache {
			x.ref(d)
		}
		for _, d := range v.opDefs() {
			x.ref(d)
		}
		for _, statement := range v.topLevel {
			switch entry := statement.(type) {
			case *instanceNode:
				x.instance(entry)
			case *useOrHideNode:
				x.inlineLevel(entry)
			default:
				x.ref(entry)
			}
		}
		for _, d := range v.bodyDefinitions {
			if child, ok := d.(*moduleNode); ok {
				x.ref(child)
			}
		}
		x.end("ModuleNode")
	case *assumeNode:
		x.start("AssumeNode")
		x.location(&v.semanticHeader)
		if v.definition != nil {
			x.start("definition")
			x.ref(v.definition)
			x.end("definition")
		}
		x.start("body")
		x.inlineLevel(v.expression)
		x.end("body")
		x.end("AssumeNode")
	case *theoremNode:
		x.start("TheoremNode")
		x.location(&v.semanticHeader)
		if v.definition != nil {
			x.start("definition")
			x.ref(v.definition)
			x.end("definition")
		}
		x.start("body")
		x.inlineLevel(v.body)
		x.end("body")
		if v.proof != nil {
			x.inlineLevel(v.proof)
		}
		if v.suffices {
			x.start("suffices")
			x.end("suffices")
		}
		x.end("TheoremNode")
	case *thmOrAssumpDefNode:
		tag := "AssumeDef"
		if v.theorem {
			tag = "TheoremDefNode"
		}
		x.start(tag)
		x.location(&v.semanticHeader)
		x.level(v.levels)
		x.named("uniquename", v.name)
		x.inlineLevel(v.body)
		x.end(tag)
	case *formalParamNode:
		x.start("FormalParamNode")
		if v.tree != nil {
			x.location(&v.semanticHeader)
		}
		x.level(v.levels)
		x.named("uniquename", v.name)
		x.num("arity", v.arity)
		x.end("FormalParamNode")
	case *opDeclNode:
		x.start("OpDeclNode")
		x.location(&v.semanticHeader)
		x.level(v.levels)
		x.named("uniquename", v.name)
		x.num("arity", v.arity)
		x.num("kind", v.kind)
		x.end("OpDeclNode")
	case *builtinOpDefNode:
		x.start("BuiltInKind")
		x.location(&v.semanticHeader)
		x.level(v.levels)
		x.named("uniquename", v.name)
		x.num("arity", v.arity)
		if v.params != nil {
			x.params(v.params, v.isLeibnizArg)
		}
		x.end("BuiltInKind")
	case *userOpDefNode:
		if v.kind == 6 {
			x.start("ModuleInstanceKind")
			if v.tree != nil {
				x.location(&v.semanticHeader)
			}
			x.level(v.levels)
			x.named("uniquename", v.name)
			if v.local {
				x.start("local")
				x.end("local")
			}
			x.end("ModuleInstanceKind")
			return
		}
		x.start("UserDefinedOpKind")
		if v.tree != nil {
			x.location(&v.semanticHeader)
		}
		x.level(v.levels)
		x.named("uniquename", v.name)
		x.num("arity", v.arity)
		x.start("originalOperator")
		x.ref(v.sourceNode())
		x.end("originalOperator")
		x.start("originallyDefinedInModule")
		x.ref(v.module)
		x.end("originallyDefinedInModule")
		x.start("body")
		x.expr(v.body, x.other)
		x.end("body")
		x.params(v.params, v.isLeibnizArg)
		x.comments(v.tree)
		if v.inRecursive {
			x.start("recursive")
			x.end("recursive")
		}
		if v.recursiveSection >= 0 {
			x.num("recursiveSection", v.recursiveSection)
		}
		if v.local {
			x.start("local")
			x.end("local")
		}
		x.end("UserDefinedOpKind")
	default:
		x.fail("unsupported symbol definition")
	}
}
func (x *xmlExporter) expr(a exprOrOpArg, other bool) {
	if a == nil {
		if other {
			x.start("StringNode")
			x.text("StringValue", "$Other")
			x.end("StringNode")
		} else {
			x.fail("unexpected null operand")
		}
		return
	}
	if len(x.active) >= 2048 {
		x.fail("expression depth limit")
		return
	}
	if x.active[a] {
		x.fail("cyclic expression expansion")
		return
	}
	x.visits++
	if x.visits > x.maxNodes {
		x.fail("expression visit limit")
		return
	}
	x.active[a] = true
	defer delete(x.active, a)
	switch v := a.(type) {
	case *substInNode:
		x.start("SubstInNode")
		x.location(&v.semanticHeader)
		x.level(v.levels)
		x.substs(v.substs)
		x.start("body")
		x.expr(v.body, other)
		x.end("body")
		x.start("instFrom")
		x.ref(v.instantiatingModule)
		x.end("instFrom")
		x.start("instTo")
		x.ref(v.instantiatedModule)
		x.end("instTo")
		x.end("SubstInNode")
	case *numeralNode:
		x.start("NumeralNode")
		x.location(&v.semanticHeader)
		x.level(v.levels)
		if v.bigValue != nil {
			x.text("IntValue", v.bigValue.String())
		} else {
			x.num("IntValue", int(v.value))
		}
		x.end("NumeralNode")
	case *decimalNode:
		x.start("DecimalNode")
		x.location(&v.semanticHeader)
		x.level(v.levels)
		if v.bigValue != nil {
			x.text("mantissa", v.bigValue.String())
			x.num("exponent", v.scale)
		} else {
			x.text("mantissa", strconv.FormatInt(v.mantissa, 10))
			x.num("exponent", v.exponent)
		}
		x.text("integralPart", v.integral)
		x.text("fractionalPart", v.fractional)
		x.end("DecimalNode")
	case *stringNode:
		x.start("StringNode")
		x.location(&v.semanticHeader)
		x.level(v.levels)
		x.named("StringValue", v.value)
		x.end("StringNode")
	case *opArgNode:
		x.start("OpArgNode")
		x.location(&v.semanticHeader)
		x.level(v.levels)
		x.start("argument")
		x.ref(v.operator)
		x.end("argument")
		x.end("OpArgNode")
	case *letInNode:
		x.start("LetInNode")
		x.location(&v.semanticHeader)
		x.level(v.levels)
		x.start("body")
		x.expr(v.body, other)
		x.end("body")
		x.start("opDefs")
		for _, d := range v.definitions {
			x.ref(d)
		}
		x.end("opDefs")
		x.end("LetInNode")
	case *labelNode:
		x.start("LabelNode")
		x.location(&v.semanticHeader)
		x.level(v.levels)
		x.named("uniquename", v.name)
		x.num("arity", len(v.params))
		x.start("body")
		x.inlineLevel(v.body)
		x.end("body")
		x.start("params")
		for _, p := range v.params {
			x.ref(p)
		}
		x.end("params")
		x.end("LabelNode")
	case *atNode:
		x.start("AtNode")
		x.location(&v.semanticHeader)
		x.level(v.levels)
		if v.except == nil || v.component == nil || len(v.except.operands) == 0 || len(v.component.operands) == 0 {
			x.fail("missing EXCEPT links")
		} else {
			x.expr(v.except.operands[0], other)
			x.expr(v.component.operands[0], other)
		}
		x.end("AtNode")
	case *opApplNode:
		x.start("OpApplNode")
		x.location(&v.semanticHeader)
		x.level(v.levels)
		// The copied SymbolContext aliases its parent's flag array. Java sets
		// this before exporting the operator, which can recursively define more symbols.
		flag := x.other
		if v.operator != nil && v.operator.header().name.text == "$Case" && len(v.operands) > 1 {
			if p, ok := v.operands[len(v.operands)-1].(*opApplNode); ok && p.operator != nil && p.operator.header().name.text == "$Pair" && len(p.operands) > 0 && p.operands[0] == nil {
				x.other = true
				flag = true
			}
		}
		x.start("operator")
		x.ref(v.operator)
		x.end("operator")
		x.start("operands")
		for i, a := range v.operands {
			flag = x.other
			if a == nil && !(flag && i == 0) {
				x.fail("unexpected null operand")
			} else {
				x.expr(a, flag)
			}
		}
		x.end("operands")
		if v.unbounded != nil || v.bounded != nil {
			x.start("boundSymbols")
			for i, p := range v.unbounded {
				x.start("unbound")
				x.ref(p)
				if i < len(v.tupleBounds) && v.tupleBounds[i] {
					x.start("tuple")
					x.end("tuple")
				}
				x.end("unbound")
			}
			for i, ps := range v.bounded {
				x.start("bound")
				for _, p := range ps {
					x.ref(p)
				}
				if i < len(v.tupleBounds) && v.tupleBounds[i] {
					x.start("tuple")
					x.end("tuple")
				}
				if i >= len(v.ranges) {
					x.fail("missing bound range")
				} else {
					x.expr(v.ranges[i], x.other)
				}
				x.end("bound")
			}
			x.end("boundSymbols")
		}
		x.end("OpApplNode")
	default:
		x.fail(fmt.Sprintf("unsupported expression %T", a))
	}
}

func javaNameLess(a, b *name) bool {
	if a == nil || b == nil {
		return a == nil && b != nil
	}
	x, y := a.units, b.units
	for i := 0; i < len(x) && i < len(y); i++ {
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return len(x) < len(y)
}
func (x *xmlExporter) substs(substs []*subst) {
	x.start("substs")
	for _, s := range substs {
		if s == nil {
			x.fail("nil substitution")
			continue
		}
		x.start("Subst")
		x.ref(s.op)
		x.expr(s.expr, x.other)
		x.end("Subst")
	}
	x.end("substs")
}
func (x *xmlExporter) instance(v *instanceNode) {
	if v == nil || v.module == nil {
		x.fail("nil instance")
		return
	}
	x.start("InstanceNode")
	x.location(&v.semanticHeader)
	// InstanceNode.getLevel always returns zero, even before level checking.
	x.num("level", 0)
	if v.name != nil {
		x.named("uniquename", v.name)
	}
	x.named("module", v.module.name)
	x.substs(v.substs)
	x.start("params")
	for _, p := range v.params {
		x.ref(p)
	}
	x.end("params")
	if v.local {
		x.start("local")
		x.end("local")
	}
	x.end("InstanceNode")
}

// phase2XML writes a candidate graph for a single external root. On error the
// caller must discard the partial artifact and mark the graph incomplete.
func phase2XML(ctx stdcontext.Context, w io.Writer, root *moduleNode, maxSymbols int, spoolDir string, spoolLimit int64) (result error) {
	return phase2XMLRoots(ctx, w, root, []*moduleNode{root}, maxSymbols, spoolDir, spoolLimit)
}

func phase2XMLRoots(ctx stdcontext.Context, w io.Writer, root *moduleNode, roots []*moduleNode, maxSymbols int, spoolDir string, spoolLimit int64) (result error) {
	if ctx == nil || w == nil || root == nil || maxSymbols <= 0 || spoolDir == "" || spoolLimit <= 0 {
		return errors.New("invalid XML exporter arguments")
	}
	x := &xmlExporter{ctx: ctx, enc: xml.NewEncoder(io.Discard), seen: make(map[levelNode]bool), active: make(map[any]bool), maxNodes: maxSymbols, spoolDir: spoolDir, spoolLimit: spoolLimit}
	defer func() {
		for _, p := range x.spools {
			if e := os.Remove(p); e != nil {
				result = errors.Join(result, fmt.Errorf("remove XML spool: %w", e))
			}
		}
	}()
	// The first root reference recursively discovers definitions in Java order.
	for _, module := range roots {
		x.ref(module)
	}
	if x.err != nil {
		return x.err
	}
	x.enc = xml.NewEncoder(w)
	x.start("modules")
	x.named("RootModule", root.name)
	x.start("context")
	if x.err == nil {
		x.err = x.enc.Flush()
	}
	for _, p := range x.spools {
		if x.err != nil {
			break
		}
		var f *os.File
		f, x.err = os.Open(p)
		if x.err != nil {
			break
		}
		_, x.err = io.Copy(w, f)
		e := f.Close()
		if x.err == nil {
			x.err = e
		}
	}
	x.end("context")
	for _, module := range roots {
		x.ref(module)
	}
	x.end("modules")
	if x.err == nil {
		x.err = x.enc.Flush()
	}
	return x.err
}

// inlineLevel exports supported non-symbol level nodes in their source order.
func (x *xmlExporter) inlineLevel(n levelNode) {
	if n == nil {
		x.fail("nil level node")
		return
	}
	if x.depth >= 128 {
		x.fail("level node depth limit")
		return
	}
	x.depth++
	defer func() { x.depth-- }()
	if _, ok := n.(exprOrOpArg); !ok {
		if x.active[n] {
			x.fail("cyclic level node expansion")
			return
		}
		x.active[n] = true
		defer delete(x.active, n)
		x.visits++
		if x.visits > x.maxNodes {
			x.fail("level node visit limit")
			return
		}
	}
	switch v := n.(type) {
	case exprOrOpArg:
		x.expr(v, x.other)
	case *assumeProveNode:
		x.start("AssumeProveNode")
		x.location(&v.semanticHeader)
		x.start("assumes")
		for _, a := range v.assumptions {
			x.inlineLevel(a)
		}
		x.end("assumes")
		x.start("prove")
		x.inlineLevel(v.prove)
		x.end("prove")
		if v.suffices {
			x.start("suffices")
			x.end("suffices")
		}
		if v.boxed {
			x.start("boxed")
			x.end("boxed")
		}
		x.end("AssumeProveNode")
	case *newSymbNode:
		x.start("NewSymbNode")
		x.location(&v.semanticHeader)
		x.ref(v.declaration)
		if v.set != nil {
			x.inlineLevel(v.set)
		}
		x.end("NewSymbNode")
	case *leafProofNode:
		tag := "obvious"
		if v.omitted {
			tag = "omitted"
		} else if len(v.facts)+len(v.defs) > 0 {
			tag = "by"
		}
		x.start(tag)
		x.location(&v.semanticHeader)
		if tag == "by" {
			x.proofFactsDefs(v.facts, v.defs)
			if v.only {
				x.start("only")
				x.end("only")
			}
		}
		x.end(tag)
	case *nonLeafProofNode:
		if v.tree == nil || v.tree.Kind() != generated.N_Proof {
			x.fail("missing proof syntax")
			return
		}
		x.start("steps")
		x.location(&v.semanticHeader)
		x.num("proofLevel", int(v.tree.ProofLevel()))
		for _, step := range v.steps {
			x.proofLevelNode(step)
		}
		x.end("steps")
	case *useOrHideNode:
		x.start("UseOrHideNode")
		x.location(&v.semanticHeader)
		x.proofFactsDefs(v.facts, v.defs)
		if v.only {
			x.start("only")
			x.end("only")
		}
		if v.hide {
			x.start("hide")
			x.end("hide")
		}
		x.end("UseOrHideNode")
	case *defStepNode:
		x.start("DefStepNode")
		x.location(&v.semanticHeader)
		for _, def := range v.defs {
			x.ref(def)
		}
		x.end("DefStepNode")
	case *instanceNode:
		x.instance(v)
	case *theoremNode, *assumeNode, *moduleNode, *thmOrAssumpDefNode, *userOpDefNode, *builtinOpDefNode, *formalParamNode, *opDeclNode:
		x.ref(n)
	case *apSubstInNode:
		x.start("APSubstInNode")
		x.location(&v.semanticHeader)
		x.start("substs")
		for _, s := range v.substs {
			if s == nil {
				x.fail("nil substitution")
				break
			}
			x.start("Subst")
			x.ref(s.op)
			if e, ok := s.expr.(levelNode); ok {
				x.inlineLevel(e)
			} else {
				x.fail("unsupported substitution expression")
			}
			x.end("Subst")
		}
		x.end("substs")
		x.start("body")
		x.inlineLevel(v.body)
		x.end("body")
		x.start("instFrom")
		x.ref(v.instantiatingModule)
		x.end("instFrom")
		x.start("instTo")
		x.ref(v.instantiatedModule)
		x.end("instTo")
		x.end("APSubstInNode")
	default:
		x.fail("unsupported level node")
	}
}

// proofLevelNode follows export edges, not proof-context graph-walk edges.
func (x *xmlExporter) proofLevelNode(n levelNode) {
	switch n.(type) {
	case leafNode, *theoremNode, *assumeNode:
		x.ref(n)
	default:
		x.inlineLevel(n)
	}
}
func (x *xmlExporter) proofFactsDefs(facts []levelNode, defs []leafNode) {
	x.start("facts")
	for _, fact := range facts {
		x.proofLevelNode(fact)
	}
	x.end("facts")
	x.start("defs")
	for _, def := range defs {
		x.ref(def)
	}
	x.end("defs")
}
