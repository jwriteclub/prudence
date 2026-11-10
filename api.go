// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Package prudence checks TLA+ modules for the errors that SANY, the parser
// and semantic checker in the TLA+ tools, reports. It is a Go port of SANY's
// parser, semantic analysis and level checking.
//
// Check runs the full check on a root module and the modules it loads. Parse
// runs syntax analysis only, and Tokens runs the lexer only.
//
// No function reads the filesystem or changes global state. A path is a
// label used in diagnostics: the caller passes source bytes, and
// Request.Resolve supplies other modules by name.
package prudence

import (
	"context"
	sanyparser "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

const Version = "0.1.0"

// DefaultMaxAcquisitionVisits is the per-pass visit limit for module loading
// that Check uses when Options.MaxAcquisitionVisits is zero.
const DefaultMaxAcquisitionVisits = 1000000

// Options holds the per-call limits for Parse and Check.
type Options struct {
	// MaxDepth limits recursion depth. Zero selects the defaults: 12,000 in
	// the parser, and 2,000 in module loading and semantic analysis. A
	// positive value sets all of them, and a negative value is an error.
	MaxDepth int
	// MaxAcquisitionVisits limits the visits Check makes in each pass while
	// loading the root module and the modules it depends on: the syntax-node
	// walk, module discovery, module resolution, cycle checks and dependency
	// ordering. Each pass has its own counter, and each counter is held to
	// this one value. It is not a count of distinct syntax nodes, a limit on
	// total work across passes, or a memory limit. Zero selects
	// DefaultMaxAcquisitionVisits, a positive value overrides it, and a
	// negative value makes Check return an error. Reaching it stops Check
	// with CompletionIncomplete and a CodeAcquisitionLimit diagnostic. Parse
	// does not use it.
	MaxAcquisitionVisits int
	IncludeTokens        bool // Fill Result.Tokens.
}

// Position is a point in a source. Line, Column and UTF16Column are
// zero-based. Column counts Unicode code points and UTF16Column counts UTF-16
// units; a tab counts as one in both.
//
// ByteOffset is authoritative, and a span never splits a decoded character.
// Inside malformed UTF-8, Column and UTF16Column count each invalid byte as
// one replacement character, as Go does. The analyser decodes as Java SANY
// does, with one U+FFFD per malformed sequence, so a multi-byte malformed
// sequence can take more columns here than the one character the lexer saw,
// and more than the Java columns quoted in diagnostic messages. Editors
// should map spans from ByteOffset.
type Position struct {
	ByteOffset, Line, Column, UTF16Column int
}

// Span is a half-open range: it includes Start and excludes End.
type Span struct{ Start, End Position }

type Severity uint8

const (
	SeverityError Severity = iota + 1
	SeverityWarning
	SeverityInfo
	SeverityHint
)

// Diagnostic is one error, warning or note. Path and Span locate it in the
// caller's document, for editors. Unit is set only when SANY reported the
// diagnostic in a parse unit other than the document Path names.
type Diagnostic struct {
	Phase               AnalysisLevel
	Severity            Severity
	Code, Message, Path string
	Span                Span
	Unit                *DiagnosticUnit // Optional; owned by the caller.
}

// DiagnosticUnit names the SANY parse unit that reported a diagnostic, and
// where in that unit's own text the diagnostic starts. Check sets it for a
// semantic or level diagnostic located in a module that no resolver supplied
// and that SANY's MonolithSpecExtractor took from the root text. SANY
// analyses such a module as a unit of its own and prints its locations as
// "line L, col C ... of module Module", counted in the extracted text. The
// root's own copy of the same nested module reports in the root's numbering,
// with Unit nil. Unit is also nil for diagnostics in an ordinary document
// (the root or a resolver source) and for diagnostics with no source
// position.
//
// Module is the name SANY prints after "of module". Line and Column are
// SANY's one-based start in the extracted text, which begins at the first
// line of the root that matches the module's header. Column counts UTF-16
// code units, and a tab advances to the next multiple of 8 plus one, as
// JavaCC's SimpleCharStream does. These are not Position coordinates and do
// not follow Path's numbering; Span stays authoritative for editors.
type DiagnosticUnit struct {
	Module       string
	Line, Column int
}

const (
	CodeLexical    = "lexical"
	CodeSyntax     = "syntax/expected"
	CodeDepthLimit = "depth-limit"
	CodeInternal   = "internal"
	CodeIncomplete = "analysis/incomplete"
	// CodeAcquisitionLimit marks the informational diagnostic of a Check
	// that reached Options.MaxAcquisitionVisits while loading modules.
	// Completion is CompletionIncomplete; a higher limit may let the check
	// finish.
	CodeAcquisitionLimit = "analysis/acquisition-limit"
	// CodeTrailingLexical marks an informational lexical diagnostic in text
	// after the root module's closing delimiter. SANY never tokenizes that
	// text, so it never blocks Check. Parse and Check report it for editors.
	CodeTrailingLexical = "lexical/trailing"
)

type TokenKind uint8

const (
	TokenInvalid TokenKind = iota
	TokenKeyword
	TokenIdentifier
	TokenOperator
	TokenNumber
	TokenString
	TokenComment
	TokenDelimiter
	TokenProofKeyword
	TokenLineComment
	TokenBlockComment
)

type Token struct {
	Kind TokenKind
	Span Span
}

type AnalysisLevel string

const (
	LevelLexical  AnalysisLevel = "lexical"
	LevelSyntax   AnalysisLevel = "syntax"
	LevelSemantic AnalysisLevel = "semantic"
	LevelComplete AnalysisLevel = "complete"
)

// Completion reports whether the requested analysis finished. It says nothing
// about whether the source is valid. CompletionComplete means the analysis
// reached the point where SANY stops, which can be after reporting errors.
// CompletionIncomplete means it stopped early, with no verdict.
type Completion uint8

const (
	CompletionUnknown Completion = iota
	CompletionComplete
	CompletionIncomplete
)

// SourceOrigin records where the caller says a source came from. It is never
// inferred from a path or name. OriginUnspecified is treated as nonstandard.
type SourceOrigin uint8

const (
	OriginUnspecified SourceOrigin = iota
	OriginWorkspace
	OriginStandard
)

// Tree is an opaque syntax tree owned by the caller. It has no node
// accessors yet.
type Tree struct{ root *sanyparser.SyntaxTreeNode }

type DeclKind string

const (
	DeclDefinition DeclKind = "definition"
	DeclVariable   DeclKind = "variable"
	DeclConstant   DeclKind = "constant"
	DeclTheorem    DeclKind = "theorem"
	DeclAssumption DeclKind = "assumption"
	DeclModule     DeclKind = "module"
	DeclSubmodule  DeclKind = "submodule"
	DeclRecursive  DeclKind = "recursive"
	DeclOperator   DeclKind = "operator"
)

type Decl struct {
	Name               string
	Kind               DeclKind
	NameSpan, FullSpan Span
	Params             int // -1 when unavailable from a partial parse.
}

type Instance struct {
	Module string
	Span   Span
}

type Outline struct {
	Module    string
	Extends   []string
	Instances []Instance
	Decls     []Decl
}

type Result struct {
	Tree        *Tree
	Tokens      []Token
	Diagnostics []Diagnostic
	Level       AnalysisLevel // Highest analysis phase examined, independently of validity.
	Completion  Completion
	Outline     Outline
}

type Source struct {
	Path   string
	Text   []byte
	Origin SourceOrigin
}

// Resolver returns the source of an EXTENDS or INSTANCE target by module
// name. Check calls it before trying the embedded standard modules.
// Returning ErrModuleNotFound, or an error wrapping it, lets Check fall back
// to the standard modules and then to a module of that name inside the root
// text; any other error stops the check.
//
// SANY looks in the working directory before its standard modules. To match
// it, a resolver should return the caller's own modules first, including ones
// named like standard modules (Naturals.tla, TLC.tla), with OriginWorkspace,
// and return ErrModuleNotFound only when it has nothing. Check never searches
// a filesystem itself: Source.Path is a label, and choosing the source is the
// caller's job.
type Resolver func(context.Context, string) (Source, error)

// Request is the input to one Check call.
//
// Path labels the root source; Check never opens it. If the last element of
// the label, after the final '/' or '\', is <base>.tla with a nonempty base
// and the extension in any letter case, the label names a TLA+ source file.
// As in SANY, the root module must then be named base. Otherwise Check
// reports semantic/4221 ("File name 'base' does not match the name 'M' of the
// top level module it contains.") without a span, before resolving any
// dependency. Any other label, including the empty label, imposes no name. A
// dependency's file name is always the module name it was requested by,
// whatever label the Resolver returns, because SANY reads it from <name>.tla
// or from the module of that name in the root file. A resolved module with a
// different name is semantic/4221 at the resolver's label.
//
// As SANY does, Check validates the PlusCal translation of the root and of
// each EXTENDS target. When the algorithm text cannot be uncommented (for
// example an unterminated string), it reports semantic/4003 "An unexpected
// problem was encountered attempting to validate the specification for M" at
// the start of that module's source. Loading continues, and semantic
// analysis is skipped. SANY's checksum warnings (4803-4805) are not reported.
//
// Module-loading diagnostics (4003, 4220, 4221, 4222) have Phase
// LevelSemantic, and Result.Level is then LevelSemantic. A semantic
// diagnostic that SANY locates at no source position (its null location, or
// a built-in operator's node, printed "In module --TLA+ BUILTINS--") gets an
// empty span at the start of the root path.
type Request struct {
	Stage   AnalysisLevel // Zero requests the complete semantic and level check.
	Origin  SourceOrigin
	Path    string
	Text    []byte
	Options Options
	Resolve Resolver
}

type moduleNotFoundError string

func (e moduleNotFoundError) Error() string { return string(e) }

// ErrModuleNotFound is what a Resolver returns, possibly wrapped, when it
// has no source for a module. Check then tries the embedded standard modules,
// then a module of that name inside the root text.
const ErrModuleNotFound moduleNotFoundError = "module not found"

// CodeModuleNotFound is reserved for a future semantic diagnostic.
const CodeModuleNotFound = "module/not-found"
