// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ SpecObj.java and ParseUnit.java, Copyright 2003 Compaq
// Corporation and 2003 Microsoft Corporation, MIT; see LICENSE.
package modulegraph

import (
	"context"
	"errors"
	"unicode/utf16"

	parser "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/recursion"
)

// Input is private transport. Origin matches the public SourceOrigin values:
// 0 unspecified, 1 workspace, 2 standard. Path is an uninterpreted label.
//
// FileName is the root's source-file identity: the base name SANY derives
// from the file it reads (ParseUnit.java:202-231), supplied by the caller.
// Empty means the source carries no file identity and its module name is not
// compared with one. Acquisition ignores it for dependencies: SpecObj
// resolves a dependency as <name>.tla or as the monolithic module <name>, so
// a dependency's identity is always the requested module name.
type Input struct {
	Path     string
	FileName string
	Text     []byte
	Origin   uint8
	Parsed   *parser.ParseResult // Optional root parse, owned by this request.
	View     *SourceView         // Optional extracted view of the immutable root source.
}

const (
	OriginUnspecified uint8 = iota
	OriginWorkspace
	OriginStandard
)

type Resolver func(context.Context, string) (Input, error)

var ErrResolverUnavailable = errors.New("module resolver unavailable")
var ErrParseIncomplete = errors.New("module parse incomplete")

// FileNameMismatchError is ParseUnit.verifyEquivalenceOfFileAndModuleNames's
// MODULE_NAME_DIFFERENT_FROM_FILE_NAME abort (ParseUnit.java:202-231). SANY
// checks it right after a unit parses, before any of its dependencies are
// resolved. Path is the mismatching source's label.
type FileNameMismatchError struct {
	FileName, ModuleName, Path string
}

func (e *FileNameMismatchError) Error() string {
	return "File name '" + e.FileName + "' does not match the name '" + e.ModuleName + "' of the top level module it contains."
}

type AcquisitionLimits struct{ MaxDepth, MaxVisits, MaxUnits, MaxEdges, MaxNodes, MaxSourceBytes, MaxTotalBytes int }
type Reference struct {
	Name string
	Tree *parser.SyntaxTreeNode
	Kind EdgeKind
}

type Document struct {
	Unit         *Unit
	Name         string
	Input        Input
	Parse        parser.ParseResult
	ParseError   error
	dependencies []Reference
	edges        []Reference
	linked       map[string]*Document
	RootModule   *ModuleRef
}

func (d *Document) References() []Reference { return append([]Reference(nil), d.dependencies...) }

// Edges returns the relationships acquired for this unit's root module, in
// selection order. Unlike References, an edge exists only where acquisition
// bound a previously unresolved name; edge Trees are nil.
func (d *Document) Edges() []Reference { return append([]Reference(nil), d.edges...) }

// Linked returns the unit that acquisition bound to name for this document.
func (d *Document) Linked(name string) *Document { return d.linked[name] }

// Acquisition retains the external parse-unit graph and lexical relationships.
// Success does not certify semantic completeness or phase conformance.
type Acquisition struct {
	Root  *Document
	Units []*Document
	graph *Graph
	// Failed retains the copied source and parse result when parsing is incomplete.
	Failed          *Document
	FailedReference *Reference
	FailedOwner     *ModuleRef
	// Request and RequestOwner name the last unresolved reference acquisition
	// selected, SpecObj's nextExtenderOrInstancerModule and its name. When
	// resolution or linking of that reference fails they identify it.
	Request      *Reference
	RequestOwner *ModuleRef
	// Validation lists, in SANY's order and without repeats, the units whose
	// PlusCal translation validation failed (SpecObj.validateParseUnit with
	// ERROR_ENCOUNTERED): the root after it parses, and each EXTENDS target
	// when that extension is selected (SpecObj.java:909-960). The error does
	// not abort loading, so these precede any abort that follows.
	Validation []*Document
}

func (a *Acquisition) validate(d *Document) {
	for _, old := range a.Validation {
		if old == d {
			return // Errors.addMessage drops an identical repeat.
		}
	}
	if d == nil || d.Parse.Tree == nil {
		return
	}
	loc := d.Parse.Tree.Location()
	if loc != nil && pcalValidationFails(d.Input.Text, int(loc.BeginLine()), int(loc.EndLine())) {
		a.Validation = append(a.Validation, d)
	}
}

// InspectionLimits returns the depth and visit limits this acquisition ran
// under, so read-only inspections of its result can stay within them. It
// returns zeros for an acquisition without a graph.
func (a *Acquisition) InspectionLimits() (maxDepth, maxVisits int) {
	if a == nil || a.graph == nil {
		return 0, 0
	}
	return a.graph.maxDepth, a.graph.maxVisits
}

func (a *Acquisition) Order(ctx context.Context) ([]*Unit, error) {
	return a.graph.Order(ctx, a.Root.Unit)
}

func exactName(u *parser.UniqueString) (string, string, error) {
	if u == nil {
		return "", "", ErrIncomplete
	}
	units := u.Units()
	decoded := string(utf16.Decode(units))
	round := utf16.Encode([]rune(decoded))
	if len(units) != len(round) {
		return "", "", ErrIncomplete
	}
	key := make([]byte, 0, 2*len(units))
	for i, v := range units {
		if v != round[i] {
			return "", "", ErrIncomplete
		}
		key = append(key, byte(v>>8), byte(v))
	}
	if len(key) == 0 {
		return "", "", ErrIncomplete
	}
	return decoded, string(key), nil
}

// AcquireFlatExternal retains its historical private name while admitting the
// modeled lexical-module subset. Resolver results remain call-owned copies. A non-nil
// resolver error is returned untouched, so the public adapter can preserve
// errors.Is(ErrModuleNotFound), operational errors and cancellation.
func AcquireFlatExternal(ctx context.Context, root Input, resolve Resolver, lim AcquisitionLimits) (*Acquisition, error) {
	if ctx == nil || lim.MaxNodes <= 0 || lim.MaxSourceBytes <= 0 || lim.MaxTotalBytes <= 0 {
		return nil, ErrIncomplete
	}
	g, err := New(lim.MaxDepth, lim.MaxVisits, lim.MaxUnits, lim.MaxEdges)
	if err != nil {
		return nil, err
	}
	a := &Acquisition{graph: g}
	byName := map[string]*Document{}
	totalBytes := 0
	totalNodes := 0
	parse := func(input Input, expected string) (*Document, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if input.Origin > OriginStandard || len(input.Text) > lim.MaxSourceBytes || len(input.Text) > lim.MaxTotalBytes-totalBytes {
			return nil, ErrIncomplete
		}
		input.Text = append([]byte(nil), input.Text...)
		totalBytes += len(input.Text)
		if input.View != nil {
			cost := len(input.View.Segments) * 40
			if cost > lim.MaxTotalBytes-totalBytes {
				return nil, ErrIncomplete
			}
			totalBytes += cost
		}
		var result parser.ParseResult
		var e error
		if expected == "" && input.Parsed != nil {
			result = *input.Parsed
		} else {
			result, e = parser.Parse(ctx, input.Text, parser.ParseOptions{Recursion: recursion.Options{MaxDepth: lim.MaxDepth}})
		}
		input.Parsed = nil
		if e != nil {
			var depth *recursion.DepthError
			if errors.As(e, &depth) {
				a.Failed = &Document{Name: expected, Input: input, Parse: result, ParseError: e}
				return nil, ErrParseIncomplete
			}
			return nil, e
		}
		if !result.Accepted || result.Tree == nil {
			a.Failed = &Document{Name: expected, Input: input, Parse: result}
			return nil, ErrParseIncomplete
		}
		name, key, e := exactName(result.ModuleName)
		if e != nil {
			return nil, e
		}
		heirs := result.Tree.Heirs()
		if result.Tree.Kind() != parser.N_Module || len(heirs) < 1 || heirs[0] == nil || len(heirs[0].Heirs()) < 2 {
			return nil, ErrIncomplete
		}
		headerName, _, e := exactName(heirs[0].Heirs()[1].Image())
		if e != nil || headerName != name {
			return nil, ErrIncomplete
		}
		// ParseUnit.parseFile checks the file name once the unit parses and
		// before its relationships are followed (ParseUnit.java:356-365).
		identity := input.FileName
		if expected != "" {
			identity = expected
		}
		if identity != "" && identity != name {
			return nil, &FileNameMismatchError{FileName: identity, ModuleName: name, Path: input.Path}
		}
		type frame struct {
			node        *parser.SyntaxTreeNode
			depth       int
			header      bool
			parent      int
			moduleDepth int
			insideProof bool
			// letDefinition marks the heirs of an N_ModuleDefinition that
			// is an N_LetDefinitions entry: Generator.processLetIn hands
			// it to processModuleDefinition (Generator.java:2998-3000).
			letDefinition bool
		}
		stack := []frame{{node: result.Tree, depth: 1, moduleDepth: 1}}
		for len(stack) > 0 {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if f.node == nil || f.depth > lim.MaxDepth {
				return nil, ErrIncomplete
			}
			source := f.node.Filename()
			if source == nil && !f.header {
				return nil, ErrIncomplete
			}
			if source != nil {
				sourceName, _, sourceErr := exactName(source)
				if sourceErr != nil || sourceName != name {
					return nil, ErrIncomplete
				}
			}
			totalNodes++
			if totalNodes > lim.MaxNodes {
				return nil, &BudgetError{Pass: PassSyntaxNodes, Limit: lim.MaxNodes, Observed: totalNodes}
			}
			switch f.node.Kind() {
			case parser.N_ModuleDefinition:
				if f.depth != f.moduleDepth+2 && !(f.insideProof && f.parent == parser.N_DefStep) && f.parent != parser.N_LetDefinitions {
					return nil, ErrIncomplete
				}
			case parser.N_Instance:
				if f.depth != f.moduleDepth+2 {
					return nil, ErrIncomplete
				}
				h := f.node.Heirs()
				if (len(h) != 1 && (len(h) != 2 || h[0].Kind() != 54)) || h[len(h)-1].Kind() != parser.N_NonLocalInstance || len(h[len(h)-1].Heirs()) < 2 {
					return nil, ErrIncomplete
				}
			case parser.N_NonLocalInstance:
				topLevel := f.depth == f.moduleDepth+3 && (f.parent == parser.N_Instance || f.parent == parser.N_ModuleDefinition)
				proofLocal := f.insideProof && (f.parent == parser.N_ProofStep || f.parent == parser.N_ModuleDefinition)
				letLocal := f.letDefinition && f.parent == parser.N_ModuleDefinition
				if !topLevel && !proofLocal && !letLocal {
					return nil, ErrIncomplete
				}
			}
			for i, child := range f.node.Heirs() {
				md := f.moduleDepth
				if child != nil && child.Kind() == parser.N_Module {
					md = f.depth + 1
				}
				letDefinition := f.node.Kind() == parser.N_ModuleDefinition && f.parent == parser.N_LetDefinitions
				stack = append(stack, frame{node: child, depth: f.depth + 1, header: f.header || (f.node.Kind() == parser.N_Module && i == 0), parent: f.node.Kind(), moduleDepth: md, insideProof: f.insideProof || f.node.Kind() == parser.N_Proof, letDefinition: letDefinition})
			}
		}
		// The cycle message prints each ParseUnit's getFileName
		// (SpecObj.java:341-349). A monolith-extracted unit is read from the
		// temporary file new File(name).getName()+".tla", not from the root
		// (MonolithSpecExtractor.java:77-82,107; FileUtil.java:600-612;
		// NamedInputStream.java:33-57).
		label := input.Path
		if input.View != nil {
			label = expected + ".tla"
		}
		unit, e := g.Unit(key, label)
		if e != nil {
			return nil, e
		}
		d := &Document{Unit: unit, Name: name, Input: input, Parse: result, linked: map[string]*Document{}}
		d.RootModule, e = discoverModules(ctx, d, result.Tree, lim)
		if e != nil {
			return nil, e
		}
		d.dependencies = append(d.dependencies, d.RootModule.refs...)

		return d, nil
	}
	d, err := parse(root, "")
	if err != nil {
		if a.Failed != nil {
			return a, err
		}
		return nil, err
	}
	a.Root = d
	a.Units = append(a.Units, d)
	byName[d.Name] = d
	a.validate(d)
	if err = a.acquireModules(ctx, resolve, byName, parse, lim); err != nil {
		if a.Failed != nil || a.Request != nil {
			return a, err
		}
		return nil, err
	}
	return a, nil
}
