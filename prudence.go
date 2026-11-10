// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package prudence

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	sany "github.com/jwriteclub/prudence/internal/generated/sany"
	parser "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	sanyparser "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/lexer"
	"github.com/jwriteclub/prudence/internal/modulegraph"
	"github.com/jwriteclub/prudence/internal/parserrt"
	"github.com/jwriteclub/prudence/internal/recursion"
	"github.com/jwriteclub/prudence/internal/semantic"
	"github.com/jwriteclub/prudence/internal/standard"
)

// positions maps source byte boundaries to editor coordinates. Invalid UTF-8
// bytes each occupy one column (Go per-byte replacement, not the lexer's
// decoder groups); CRLF is one line break. ByteOffset is authoritative.
func positions(src []byte) []Position {
	p := make([]Position, len(src)+1)
	line, col, u := 0, 0, 0
	for i := 0; i < len(src); {
		start := i
		r, n := utf8.DecodeRune(src[i:])
		i += n
		for j := start; j < i; j++ {
			p[j] = Position{j, line, col, u}
		}
		if r == '\r' {
			line++
			col = 0
			u = 0
		} else if r == '\n' {
			if start == 0 || src[start-1] != '\r' {
				line++
			}
			col = 0
			u = 0
		} else {
			col++
			u++
			if r > 0xffff {
				u++
			}
		}
	}
	p[len(src)] = Position{len(src), line, col, u}
	return p
}
func span(p []Position, a, b int) Span {
	if a < 0 {
		a = 0
	}
	if b < a {
		b = a
	}
	if a >= len(p) {
		a = len(p) - 1
	}
	if b >= len(p) {
		b = len(p) - 1
	}
	return Span{p[a], p[b]}
}
func rangeSpan(p []Position, r lexer.SourceRange) Span { return span(p, r.ByteStart, r.ByteEnd) }

func classify(name string, t *lexer.Token, src []byte) TokenKind {
	if t.Kind < 0 {
		return TokenInvalid
	}
	a, b := t.Range.ByteStart, t.Range.ByteEnd
	if a >= 0 && b <= len(src) && a < b {
		s := string(src[a:b])
		if strings.HasPrefix(s, `\*`) {
			return TokenLineComment
		}
		if strings.HasPrefix(s, "(*") {
			return TokenBlockComment
		}
	}
	switch name {
	case "IDENTIFIER":
		return TokenIdentifier
	case "NUMBER_LITERAL", "NUMBER":
		return TokenNumber
	case "STRING_LITERAL":
		return TokenString
	case "BY", "ONLY", "DEFINED", "DF", "THEOREM", "USE", "HIDE", "HAVE", "OBVIOUS", "OMITTED", "TAKE", "PROOF", "PROVE", "BOXPROVE", "QED", "PICK", "WITNESS", "SUFFICES":
		return TokenProofKeyword
	case "COMMA", "COLON", "COLONCOLON", "DOT", "LBR", "RBR", "LSB", "RSB", "LBC", "RBC", "LAB", "RAB", "BEGIN_MODULE", "_BM0", "_BM1", "_BM2", "END_MODULE", "SEPARATOR":
		return TokenDelimiter
	}
	switch name {
	case "ACTION", "ASSUME", "BOXASSUME", "ASSUMPTION", "CASE", "CHOOSE", "CONSTANT", "ELSE", "EXCEPT", "EXISTS", "EXTENDS", "FORALL", "IF", "INSTANCE", "LET", "LETIN", "LOCAL", "MODULE", "NEW", "OTHER", "PROPOSITION", "SF", "T_EXISTS", "T_FORALL", "THEN", "LAMBDA", "RECURSIVE", "STATE", "TEMPORAL", "VARIABLE", "WF", "WITH":
		return TokenKeyword
	}
	return TokenOperator
}

// Tokens runs SANY's lexer, in its error-recovering mode, over src. Path is a
// label and no files are read. Tokens, including comments, are returned in
// source order.
func Tokens(ctx context.Context, path string, src []byte) ([]Token, []Diagnostic, error) {
	tokens, diagnostics, _, err := scan(ctx, path, src)
	return tokens, diagnostics, err
}

type javaCoord struct{ line, column int }
type coordIndex map[javaCoord][2]int

func (idx coordIndex) offset(line, column int, end bool, size int) int {
	if r, ok := idx[javaCoord{line, column}]; ok {
		if end {
			return r[1]
		}
		return r[0]
	}
	if line <= 0 || column <= 0 {
		return 0
	}
	return size
}
func scan(ctx context.Context, path string, src []byte) ([]Token, []Diagnostic, coordIndex, error) {
	if ctx == nil {
		return nil, nil, nil, errors.New("prudence: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, err
	}
	p := positions(src)
	names := sany.TokenNames()
	ed := sany.NewEditor(string(src))
	javaPositions, byteRanges := ed.InputCoordinates()
	coords := make(coordIndex, len(javaPositions))
	for i, pos := range javaPositions {
		coords[javaCoord{pos.Line, pos.Column}] = byteRanges[i]
	}
	var out []Token
	var ds []Diagnostic
	add := func(t *lexer.Token, special bool) {
		if t == nil || t.Kind == 0 {
			return
		}
		r := t.Range
		if r.ByteEnd <= r.ByteStart {
			return
		}
		// A lexer may split a supplementary scalar into two UTF-16 tokens. Its byte
		// range covers the scalar in both cases; keep one public token for those bytes.
		if len(out) > 0 && r.ByteStart < out[len(out)-1].Span.End.ByteOffset {
			if r.ByteEnd <= out[len(out)-1].Span.End.ByteOffset {
				return
			}
			r.ByteStart = out[len(out)-1].Span.End.ByteOffset
		}
		name := ""
		if t.Kind >= 0 && t.Kind < len(names) {
			name = names[t.Kind]
		}
		kind := classify(name, t, src)
		if special {
			kind = TokenComment
			a, b := r.ByteStart, r.ByteEnd
			if strings.HasPrefix(string(src[a:b]), `\*`) {
				kind = TokenLineComment
			} else if strings.HasPrefix(string(src[a:b]), "(*") {
				kind = TokenBlockComment
			}
		}
		sp := rangeSpan(p, r)
		out = append(out, Token{kind, sp})
		if t.Diagnostic != nil {
			ds = append(ds, Diagnostic{Phase: LevelLexical, Severity: SeverityError, Code: CodeLexical, Message: t.Diagnostic.Error.Error(), Path: path, Span: sp})
		}
	}
	for {
		t, err := ed.NextTokenContext(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil, nil, nil, ctx.Err()
			}
			return nil, nil, nil, err
		}
		for _, s := range t.Specials() {
			add(s, true)
		}
		if t.Kind == 0 {
			break
		}
		add(t, false)
	}
	return out, ds, coords, nil
}

func diagSpan(p []Position, src []byte, coords coordIndex, e error) Span {
	var pe *parserrt.ParseError
	if errors.As(e, &pe) && pe.Diagnostic != nil {
		return rangeSpan(p, pe.Diagnostic.ErrorToken.Range)
	}
	var br *parserrt.BindingRejection
	if errors.As(e, &br) && br.Position.Line > 0 {
		a := coords.offset(br.Position.Line, br.Position.Column, false, len(src))
		b := coords.offset(br.Position.Line, br.Position.Column, true, len(src))
		return span(p, a, b)
	}
	var le *lexer.LexicalError
	if errors.As(e, &le) {
		// The coordinate index holds each unit's whole decoder group, so the
		// span never ends inside a malformed or multi-byte sequence.
		a := coords.offset(le.Position.Line, le.Position.Column, false, len(src))
		b := coords.offset(le.Position.Line, le.Position.Column, true, len(src))
		return span(p, a, b)
	}
	var de *recursion.DepthError
	if errors.As(e, &de) {
		a := coords.offset(de.Line, de.Column, false, len(src))
		return span(p, a, a)
	}
	return span(p, 0, 0)
}

// recoveryFailure preserves the original lexical diagnostic while classifying
// any failure of the bounded prefix parse using the normal Parse contract.
func recoveryFailure(ctx context.Context, result *Result, err error, path string, p []Position, src []byte, coords coordIndex) error {
	if ctx.Err() != nil {
		result.Completion = CompletionIncomplete
		return ctx.Err()
	}
	if err == nil {
		return nil
	}
	result.Completion = CompletionIncomplete
	var depth *recursion.DepthError
	if errors.As(err, &depth) {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Phase: LevelSyntax, Severity: SeverityError, Code: CodeDepthLimit, Message: err.Error(), Path: path, Span: diagSpan(p, src, coords, err)})
		result.Completion = CompletionIncomplete
		return nil
	}
	return err
}

// rootEndBoundary returns the byte offset just past the real root module's
// closing delimiter. SANY's CompilationUnit returns after the root Module's
// EndModule and never fetches another token, so Java tokenizes nothing past
// this offset (TLAplusParser.java:83, :1458-1472). The boundary is proved from
// the strict parse tree: the root N_Module's last heir must be its
// N_EndModule, whose Java coordinates must map exactly to unit byte ranges
// covering only '=' bytes, and the recovering scan must hold one delimiter
// token over exactly those bytes. Anything else is unproved.
func rootEndBoundary(tree *parser.SyntaxTreeNode, src []byte, coords coordIndex, toks []Token) (int, bool) {
	if tree == nil || tree.Kind() != parser.N_Module {
		return 0, false
	}
	heirs := tree.Heirs()
	if len(heirs) == 0 {
		return 0, false
	}
	end := heirs[len(heirs)-1]
	if end == nil || end.Kind() != parser.N_EndModule {
		return 0, false
	}
	loc := end.Location()
	first, ok := coords[javaCoord{int(loc.BeginLine()), int(loc.BeginColumn())}]
	if !ok {
		return 0, false
	}
	last, ok := coords[javaCoord{int(loc.EndLine()), int(loc.EndColumn())}]
	if !ok {
		return 0, false
	}
	a, b := first[0], last[1]
	if a < 0 || b > len(src) || b-a < 4 {
		return 0, false
	}
	for _, c := range src[a:b] {
		if c != '=' {
			return 0, false
		}
	}
	i := sort.Search(len(toks), func(i int) bool { return toks[i].Span.Start.ByteOffset >= a })
	if i >= len(toks) || toks[i].Kind != TokenDelimiter || toks[i].Span.Start.ByteOffset != a || toks[i].Span.End.ByteOffset != b {
		return 0, false
	}
	return b, true
}

// classifyTrailing returns a copy of the recovering-scan diagnostics in which
// those starting at or after the proved root end become informational
// CodeTrailingLexical diagnostics. Diagnostics before the boundary keep
// their original code and severity, so they still block Check.
func classifyTrailing(lex []Diagnostic, end int) []Diagnostic {
	out := make([]Diagnostic, len(lex))
	copy(out, lex)
	for i := range out {
		if out[i].Code == CodeLexical && out[i].Span.Start.ByteOffset >= end {
			out[i].Severity = SeverityInfo
			out[i].Code = CodeTrailingLexical
		}
	}
	return out
}

// Parse runs syntax analysis on src without loading other modules. Path is a
// label and no files are read.
func Parse(ctx context.Context, path string, src []byte, opts Options) (Result, error) {
	result, _, err := parseOwned(ctx, path, src, opts)
	return result, err
}

// proofRuntimeIncomplete is the message of the CodeIncomplete error Parse and
// Check report when the parse stops at a proof-step runtime failure, where
// Java SANY throws instead of reporting a verdict.
const proofRuntimeIncomplete = "syntax analysis could not complete: proof-step processing reached a runtime failure at which Java SANY throws an exception; the source's validity is undetermined"

func parseOwned(ctx context.Context, path string, src []byte, opts Options) (result Result, raw parser.ParseResult, err error) {
	result = Result{Level: LevelSyntax, Completion: CompletionIncomplete}
	if ctx == nil {
		return result, raw, errors.New("prudence: nil context")
	}
	if err := ctx.Err(); err != nil {
		return result, raw, err
	}
	if opts.MaxDepth < 0 {
		return result, raw, fmt.Errorf("prudence: negative maximum depth")
	}
	p := positions(src)
	toks, lex, coords, scanErr := scan(ctx, path, src)
	if scanErr != nil {
		return result, raw, scanErr
	}
	raw, err = parser.Parse(ctx, src, parser.ParseOptions{Recursion: recursion.Options{MaxDepth: opts.MaxDepth}})
	if ctx.Err() != nil {
		return result, raw, ctx.Err()
	}
	if err == nil {
		result.Completion = CompletionComplete
	}
	if err != nil {
		var depth *recursion.DepthError
		if errors.As(err, &depth) {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{Phase: LevelSyntax, Severity: SeverityError, Code: CodeDepthLimit, Message: err.Error(), Path: path, Span: diagSpan(p, src, coords, err)})
		} else if parser.IsProofRuntimeFailure(err) {
			// SANY's proof-step handling throws a Java runtime exception
			// here. Report that analysis stopped, with no verdict on the
			// source and no location; completion stays incomplete.
			result.Diagnostics = append(result.Diagnostics, Diagnostic{Phase: LevelSyntax, Severity: SeverityError, Code: CodeIncomplete, Message: proofRuntimeIncomplete, Path: path})
		} else {
			return result, raw, err
		}
	}
	if raw.Tree != nil {
		result.Tree = &Tree{root: raw.Tree}
	}
	for _, e := range raw.Errors {
		code := CodeSyntax
		var le *lexer.LexicalError
		if errors.As(raw.Failure, &le) {
			code = CodeLexical
		}
		sp := diagSpan(p, src, coords, raw.Failure)
		if pos := e.Position(); pos.Line > 0 {
			sp = span(p, coords.offset(pos.Line, pos.Column, false, len(src)), coords.offset(pos.Line, pos.Column, true, len(src)))
		}
		if code == CodeLexical && len(lex) > 0 {
			sp = lex[0].Span
		}
		phase := LevelSyntax
		if code == CodeLexical {
			phase = LevelLexical
		}
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Phase: phase, Severity: SeverityError, Code: code, Message: e.ReportedError().String(), Path: path, Span: sp})
	}
	// A scan also supplies the outline with source-accurate token ranges.
	if opts.IncludeTokens {
		result.Tokens = toks
	}
	if len(result.Diagnostics) == 0 {
		scanned := lex
		if err == nil && raw.Accepted && len(raw.Errors) == 0 {
			if end, ok := rootEndBoundary(raw.Tree, src, coords, toks); ok {
				scanned = classifyTrailing(lex, end)
			}
		}
		result.Diagnostics = append(result.Diagnostics, scanned...)
	}
	if raw.Tree != nil {
		result.Outline = outline([]*parser.SyntaxTreeNode{raw.Tree}, src, coords, toks, "")
	} else {
		result.Outline = partialOutline(src, toks)
		// Strict SANY lookahead may encounter an unfinished comment while
		// completing the preceding declaration. Parse only the intact prefix
		// with a synthetic terminator to recover completed top-level nodes.
		var lexical *lexer.LexicalError
		if errors.As(raw.Failure, &lexical) && len(lex) > 0 {
			cut := lex[0].Span.Start.ByteOffset
			for i := len(toks) - 1; i >= 0; i-- {
				t := toks[i]
				if t.Span.End.ByteOffset > cut {
					continue
				}
				if t.Span.End.ByteOffset == cut && (t.Kind == TokenBlockComment || t.Kind == TokenComment) {
					cut = t.Span.Start.ByteOffset
					continue
				}
				break
			}
			prefix := append(append([]byte(nil), src[:cut]...), []byte("\n====\n")...)
			recovered, recoverErr := parser.Parse(ctx, prefix, parser.ParseOptions{Recursion: recursion.Options{MaxDepth: opts.MaxDepth}})
			if failure := recoveryFailure(ctx, &result, recoverErr, path, p, src, coords); failure != nil {
				return result, raw, failure
			}
			if recoverErr == nil && recovered.Tree != nil {
				result.Outline = outline([]*parser.SyntaxTreeNode{recovered.Tree}, src, coords, toks, "")
				return result, raw, nil
			}
		}
		body := outline(raw.PartialBody, src, coords, toks, result.Outline.Module)
		result.Outline.Decls = append(result.Outline.Decls, body.Decls...)
		result.Outline.Instances = append(result.Outline.Instances, body.Instances...)
	}
	return result, raw, nil
}

// Check analyses the root module in req and the modules it loads, up to
// req.Stage: by default the full semantic and level check. Problems in the
// source are reported as diagnostics. Check returns a Go error for an
// invalid request, a nil or cancelled context, a resolver error other than
// ErrModuleNotFound, or an operational failure. Result.Completion says
// whether the analysis reached a verdict; see Completion.
func Check(ctx context.Context, req Request) (Result, error) {
	if req.Stage != "" && req.Stage != LevelSyntax && req.Stage != LevelSemantic && req.Stage != LevelComplete {
		return Result{Completion: CompletionIncomplete}, fmt.Errorf("prudence: invalid analysis stage %q", req.Stage)
	}
	if req.Origin > OriginStandard {
		return Result{Completion: CompletionIncomplete}, fmt.Errorf("prudence: invalid source origin %d", req.Origin)
	}
	if ctx == nil {
		return Result{Completion: CompletionIncomplete}, errors.New("prudence: nil context")
	}
	if err := ctx.Err(); err != nil {
		return Result{Completion: CompletionIncomplete}, err
	}
	if req.Options.MaxDepth < 0 {
		return Result{Completion: CompletionIncomplete}, fmt.Errorf("prudence: negative maximum depth")
	}
	if req.Options.MaxAcquisitionVisits < 0 {
		return Result{Completion: CompletionIncomplete}, fmt.Errorf("prudence: negative maximum acquisition visits")
	}
	maxVisits := req.Options.MaxAcquisitionVisits
	if maxVisits == 0 {
		maxVisits = DefaultMaxAcquisitionVisits
	}
	const maxSourceBytes = 32 << 20
	if len(req.Text) > maxSourceBytes {
		return Result{Level: LevelSyntax, Completion: CompletionIncomplete, Diagnostics: []Diagnostic{{Phase: LevelSyntax, Severity: SeverityInfo, Code: CodeIncomplete, Message: "source size limit exceeded", Path: req.Path}}}, nil
	}
	// The parse and acquisition share one immutable request-owned root snapshot.
	req.Text = append([]byte(nil), req.Text...)
	result, raw, err := parseOwned(ctx, req.Path, req.Text, req.Options)
	if err != nil || result.Completion != CompletionComplete || req.Stage == LevelSyntax {
		return result, err
	}
	for _, d := range result.Diagnostics {
		if d.Severity == SeverityError {
			return result, nil
		}
	}
	// Completion stays incomplete on every error and interruption path; only
	// a verdict at SANY's own stopping point sets it.
	result.Completion = CompletionIncomplete
	depth, err := (recursion.Options{MaxDepth: req.Options.MaxDepth}).Limit()
	if err != nil {
		return result, err
	}
	limits := modulegraph.AcquisitionLimits{MaxDepth: depth, MaxVisits: maxVisits, MaxUnits: 10000, MaxEdges: 100000, MaxNodes: maxVisits, MaxSourceBytes: maxSourceBytes, MaxTotalBytes: 128 << 20}
	root := modulegraph.Input{Path: req.Path, FileName: sourceFileName(req.Path), Text: req.Text, Origin: acquisitionOrigin(req.Origin), Parsed: &raw}
	monolithBudget := 128 << 20
	resolver := func(ctx context.Context, name string) (modulegraph.Input, error) {
		if err := ctx.Err(); err != nil {
			return modulegraph.Input{}, err
		}
		if req.Resolve != nil {
			src, err := req.Resolve(ctx, name)
			if err == nil {
				if err := ctx.Err(); err != nil {
					return modulegraph.Input{}, err
				}
				if src.Origin > OriginStandard {
					return modulegraph.Input{}, fmt.Errorf("prudence: invalid resolver source origin %d", src.Origin)
				}
				return modulegraph.Input{Path: src.Path, Text: src.Text, Origin: acquisitionOrigin(src.Origin)}, nil
			}
			if !errors.Is(err, ErrModuleNotFound) {
				return modulegraph.Input{}, err
			}
		}
		if err := ctx.Err(); err != nil {
			return modulegraph.Input{}, err
		}
		if data := standard.Lookup(name); data != nil {
			return modulegraph.Input{Path: name + ".tla", Text: data, Origin: modulegraph.OriginStandard}, nil
		}
		if view, found, e := modulegraph.ExtractMonolith(ctx, root, name, &monolithBudget); e != nil {
			return modulegraph.Input{}, e
		} else if found {
			return view, nil
		}
		if req.Resolve == nil {
			return modulegraph.Input{}, modulegraph.ErrResolverUnavailable
		}
		return modulegraph.Input{}, ErrModuleNotFound
	}
	complete := func() (Result, error) {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		result.Completion = CompletionComplete
		return result, nil
	}
	acquisitionLimit := func(e *modulegraph.BudgetError) (Result, error) {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		result.Completion = CompletionIncomplete
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Phase: LevelSemantic, Severity: SeverityInfo, Code: CodeAcquisitionLimit, Message: acquisitionLimitMessage(e), Path: req.Path})
		return result, nil
	}
	incomplete := func(phase AnalysisLevel, message string) (Result, error) {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		result.Completion = CompletionIncomplete
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Phase: phase, Severity: SeverityInfo, Code: CodeIncomplete, Message: message, Path: req.Path})
		return result, nil
	}
	acq, acquireErr := modulegraph.AcquireFlatExternal(ctx, root, resolver, limits)
	// PlusCal validation errors do not abort SANY's loading; they precede any
	// abort that ends it and stop SANY before semantic analysis.
	validation := validationDiagnostics(acq)
	if acquireErr != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		var cycle *modulegraph.CycleError
		var mismatch *modulegraph.FileNameMismatchError
		var budget *modulegraph.BudgetError
		switch {
		case errors.Is(acquireErr, modulegraph.ErrParseIncomplete):
			// SANY stops after a dependency's parse errors; a recursion-depth
			// stop is a resource limit, not a parse verdict.
			if acq == nil || acq.Failed == nil {
				return incomplete(LevelSyntax, "dependency parsing did not complete")
			}
			ds, e := failedParseDiagnostics(ctx, acq.Failed)
			if e != nil {
				return result, e
			}
			var depthErr *recursion.DepthError
			if errors.As(acq.Failed.ParseError, &depthErr) || !hasError(ds) {
				result.Diagnostics = append(result.Diagnostics, ds...)
				return incomplete(LevelSyntax, "dependency parsing did not complete")
			}
			result.Diagnostics = append(append(result.Diagnostics, validation...), ds...)
			return complete()
		case errors.Is(acquireErr, modulegraph.ErrResolverUnavailable):
			return incomplete(LevelSemantic, "semantic analysis is incomplete because no module resolver is configured")
		case errors.Is(acquireErr, ErrModuleNotFound):
			d, ok := moduleNotFoundDiagnostic(ctx, acq)
			if !ok {
				return incomplete(LevelSemantic, "module was not found by the configured resolver or monolithic fallback")
			}
			result.Level = LevelSemantic
			result.Diagnostics = append(append(result.Diagnostics, validation...), d)
			return complete()
		case errors.As(acquireErr, &cycle):
			d, ok := moduleCycleDiagnostic(ctx, req.Path, acq, cycle)
			if !ok {
				return incomplete(LevelSemantic, "module cycle could not be located")
			}
			result.Level = LevelSemantic
			result.Diagnostics = append(append(result.Diagnostics, validation...), d)
			return complete()
		case errors.As(acquireErr, &mismatch):
			result.Level = LevelSemantic
			result.Diagnostics = append(append(result.Diagnostics, validation...), fileNameMismatchDiagnostic(mismatch))
			return complete()
		case errors.As(acquireErr, &budget):
			return acquisitionLimit(budget)
		case errors.Is(acquireErr, modulegraph.ErrIncomplete):
			return incomplete(LevelSemantic, "semantic analysis stopped at unsupported module structure or an acquisition limit")
		default:
			return result, acquireErr
		}
	}
	if len(validation) > 0 {
		result.Level = LevelSemantic
		result.Diagnostics = append(result.Diagnostics, validation...)
		return complete()
	}
	levels := req.Stage != LevelSemantic
	outcome := semantic.CheckFlatExternal(ctx, acq, depth, levels)
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if outcome.Examined {
		result.Level = LevelSemantic
	}
	if outcome.Levels {
		result.Level = LevelComplete
	}
	if outcome.Err != nil {
		// Records accepted before an unsupported or budgeted stop are not
		// reported: SANY would continue and could add or reorder records.
		var semanticDepth *semantic.DepthLimitError
		var budget *modulegraph.BudgetError
		switch {
		case errors.As(outcome.Err, &semanticDepth):
			return incomplete(LevelSemantic, fmt.Sprintf("semantic analysis stopped at recursion depth limit %d", semanticDepth.Limit))
		case errors.As(outcome.Err, &budget):
			return acquisitionLimit(budget)
		case errors.Is(outcome.Err, modulegraph.ErrIncomplete):
			return incomplete(LevelSemantic, "module relationship traversal exceeded its resource limit")
		case errors.Is(outcome.Err, semantic.ErrPortIncomplete):
			return incomplete(incompletePhase(outcome.Site), incompleteMessage(outcome.Site))
		default:
			return result, outcome.Err
		}
	}
	ds, mapped, e := documentSemanticDiagnostics(ctx, req.Path, acq, outcome.Details, outcome.Stages)
	if e != nil {
		return result, e
	}
	result.Diagnostics = append(result.Diagnostics, ds...)
	if !mapped {
		return incomplete(LevelSemantic, "some semantic diagnostic locations could not be mapped to their owning documents")
	}
	return complete()
}

// acquisitionLimitMessage explains an acquisition budget stop and names the
// override.
func acquisitionLimitMessage(e *modulegraph.BudgetError) string {
	return fmt.Sprintf("semantic analysis is incomplete: the module acquisition %s pass made %d visits, exceeding the budget of %d; set Options.MaxAcquisitionVisits to a larger value to raise it", e.Pass, e.Observed, e.Limit)
}

func acquisitionOrigin(origin SourceOrigin) uint8 {
	switch origin {
	case OriginUnspecified:
		return modulegraph.OriginUnspecified
	case OriginWorkspace:
		return modulegraph.OriginWorkspace
	case OriginStandard:
		return modulegraph.OriginStandard
	default:
		return 255
	}
}

func failedParseDiagnostics(ctx context.Context, d *modulegraph.Document) ([]Diagnostic, error) {
	_, lex, coords, err := scan(ctx, d.Input.Path, d.Input.Text)
	if err != nil {
		return nil, err
	}
	p := positions(d.Input.Text)
	var ownerPositions []Position
	if d.Input.View != nil {
		ownerPositions = positions(d.Input.View.Owner)
	}
	var out []Diagnostic
	var depth *recursion.DepthError
	if errors.As(d.ParseError, &depth) {
		sp := diagSpan(p, d.Input.Text, coords, depth)
		if d.Input.View != nil {
			a, b, ok := d.Input.View.MapSpan(sp.Start.ByteOffset, sp.End.ByteOffset)
			if ok {
				sp = span(ownerPositions, a, b)
			}
		}
		out = append(out, Diagnostic{Phase: LevelSyntax, Severity: SeverityError, Code: CodeDepthLimit, Message: depth.Error(), Path: d.Input.Path, Span: sp})
	}
	for _, e := range d.Parse.Errors {
		code, phase := CodeSyntax, LevelSyntax
		var le *lexer.LexicalError
		if errors.As(d.Parse.Failure, &le) {
			code, phase = CodeLexical, LevelLexical
		}
		sp := diagSpan(p, d.Input.Text, coords, d.Parse.Failure)
		if pos := e.Position(); pos.Line > 0 {
			sp = span(p, coords.offset(pos.Line, pos.Column, false, len(d.Input.Text)), coords.offset(pos.Line, pos.Column, true, len(d.Input.Text)))
		}
		if code == CodeLexical && len(lex) > 0 {
			sp = lex[0].Span
		}
		if d.Input.View != nil {
			a, b, ok := d.Input.View.MapSpan(sp.Start.ByteOffset, sp.End.ByteOffset)
			if !ok {
				continue
			}
			sp = span(ownerPositions, a, b)
		}
		out = append(out, Diagnostic{Phase: phase, Severity: SeverityError, Code: code, Message: e.ReportedError().String(), Path: d.Input.Path, Span: sp})
	}
	if len(out) == 0 {
		if d.Input.View == nil {
			out = append(out, lex...)
		} else {
			for _, diagnostic := range lex {
				a, b, ok := d.Input.View.MapSpan(diagnostic.Span.Start.ByteOffset, diagnostic.Span.End.ByteOffset)
				if ok {
					diagnostic.Span = span(ownerPositions, a, b)
					out = append(out, diagnostic)
				}
			}
		}
	}
	return out, nil
}

// documentSemanticDiagnostics maps generation and level details to their
// owning documents. Stages parallels details; level details carry Phase
// LevelComplete. A detail SANY reports at its null location, or at the
// location of a built-in operator's node ("In module --TLA+ BUILTINS--",
// SyntaxTreeNode.java:216-222), has no source position; it keeps its text and
// is attached to the root path with an empty span at the start of the file.
//
// A located detail owned by a monolith-extracted document (Input.View set)
// also carries Unit: the module SANY prints after "of module" and the Java
// start coordinate, taken before View.MapSpan moves the span to the root's
// bytes. SANY analyzes each unit of semanticAnalysisVector separately with
// one shared Errors (SANY.java:415-449), and every node of a parse unit
// carries the unit's first module name (TLAplusParser.java:1549,
// SyntaxTreeNode.java:458-459), which for an extracted unit is the name it
// was extracted under (MonolithSpecExtractor.java:77-107). Unit is set only
// when the location's name is the owning document's name.
func documentSemanticDiagnostics(ctx context.Context, rootPath string, acq *modulegraph.Acquisition, details []semantic.Detail, stages []semantic.CheckStage) ([]Diagnostic, bool, error) {
	if len(details) == 0 {
		return nil, true, nil
	}
	owners := make(map[*sanyparser.SyntaxTreeNode]*modulegraph.Document)
	for _, d := range acq.Units {
		if d == nil || d.Parse.Tree == nil {
			return nil, false, nil
		}
		stack := []*sanyparser.SyntaxTreeNode{d.Parse.Tree}
		for len(stack) > 0 {
			if err := ctx.Err(); err != nil {
				return nil, false, err
			}
			node := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if node == nil {
				return nil, false, nil
			}
			if old := owners[node]; old != nil {
				if old != d {
					return nil, false, nil
				}
				continue
			}
			owners[node] = d
			if len(owners) > 1000000 {
				return nil, false, nil
			}
			stack = append(stack, node.Heirs()...)
		}
	}
	type index struct {
		positions      []Position
		coords         coordIndex
		ownerPositions []Position
	}
	indexes := map[*modulegraph.Document]index{}
	var rootOwnerPositions []Position
	var out []Diagnostic
	complete := true
	for i, detail := range details {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		phase := LevelSemantic
		if i < len(stages) && stages[i] == semantic.StageLevel {
			phase = LevelComplete
		}
		severity := SeverityError
		if detail.Code.Severity == semantic.Warning {
			severity = SeverityWarning
		}
		code := semanticCode(detail.Code.Value)
		loc := detail.Location
		d := owners[loc.SourceTree]
		message, e := detail.Render()
		if e == nil && (loc.Null || (d == nil && builtinLocation(loc))) {
			out = append(out, Diagnostic{Phase: phase, Severity: severity, Code: code, Message: message, Path: rootPath})
			continue
		}
		if e == nil && neverSetLocation(loc) {
			path := rootPath
			if d != nil {
				path = d.Input.Path
			}
			out = append(out, Diagnostic{Phase: phase, Severity: severity, Code: code, Message: message, Path: path})
			continue
		}
		if e != nil || !loc.NamePresent || d == nil {
			complete = false
			continue
		}
		ix, ok := indexes[d]
		if !ok {
			ed := sany.NewEditor(string(d.Input.Text))
			jp, ranges := ed.InputCoordinates()
			ix = index{positions: positions(d.Input.Text), coords: make(coordIndex, len(jp))}
			for i, pos := range jp {
				if i%1024 == 0 && ctx.Err() != nil {
					return nil, false, ctx.Err()
				}
				ix.coords[javaCoord{pos.Line, pos.Column}] = ranges[i]
			}
			if d.Input.View != nil {
				if rootOwnerPositions == nil {
					rootOwnerPositions = positions(d.Input.View.Owner)
				}
				ix.ownerPositions = rootOwnerPositions
			}
			indexes[d] = ix
		}
		start, a := ix.coords[javaCoord{loc.BeginLine, loc.BeginColumn}]
		end, b := ix.coords[javaCoord{loc.EndLine, loc.EndColumn}]
		if !a || !b || end[1] < start[0] {
			complete = false
			continue
		}
		mappedStart, mappedEnd := start[0], end[1]
		ownerPositions := ix.positions
		if d.Input.View != nil {
			var mapped bool
			mappedStart, mappedEnd, mapped = d.Input.View.MapSpan(mappedStart, mappedEnd)
			if !mapped {
				complete = false
				continue
			}
			ownerPositions = ix.ownerPositions
		}
		diagnostic := Diagnostic{Phase: phase, Severity: severity, Code: code, Message: message, Path: d.Input.Path, Span: span(ownerPositions, mappedStart, mappedEnd)}
		if d.Input.View != nil && loc.Name == d.Name {
			diagnostic.Unit = &DiagnosticUnit{Module: loc.Name, Line: loc.BeginLine, Column: loc.BeginColumn}
		}
		out = append(out, diagnostic)
	}
	return out, complete, nil
}
