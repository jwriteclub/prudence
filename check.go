// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package prudence

import (
	"context"
	"math"
	"strconv"
	"strings"

	sany "github.com/jwriteclub/prudence/internal/generated/sany"
	sanyparser "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/modulegraph"
	"github.com/jwriteclub/prudence/internal/semantic"
)

// SANY ErrorCode values for the errors of SpecObj.loadSpec and ParseUnit.
const (
	sanyInternalError                 = 4003
	sanyModuleFileCannotBeFound       = 4220
	sanyModuleNameDifferentFromFile   = 4221
	sanyModuleDependenciesAreCircular = 4222
)

// sanyBuiltinsModule is the file name SANY gives the syntax nodes of built-in
// operators (SyntaxTreeNode.java:216-222).
const sanyBuiltinsModule = "--TLA+ BUILTINS--"

// neverSetLocation reports SANY's never-set location: a syntax node whose
// range was never assigned keeps the SyntaxTreeNode constructor's Integer.MAX_VALUE
// begin and Integer.MIN_VALUE end (the SyntaxTreeNode location array and Location.java).
// SANY prints such an error as "line 2147483647, col 2147483647 to line
// -2147483648, col -2147483648". Only this exact range qualifies; the
// diagnostic is published with an empty span, as for a null location.
func neverSetLocation(loc semantic.Location) bool {
	return loc.NamePresent && !loc.Null &&
		loc.BeginLine == math.MaxInt32 && loc.BeginColumn == math.MaxInt32 &&
		loc.EndLine == math.MinInt32 && loc.EndColumn == math.MinInt32
}

// builtinLocation reports a location SANY prints as "In module --TLA+
// BUILTINS--": a built-in operator's node, which has no source position.
func builtinLocation(loc semantic.Location) bool {
	return loc.NamePresent && loc.Name == sanyBuiltinsModule && !loc.Null &&
		loc.BeginLine == 0 && loc.BeginColumn == 0 && loc.EndLine == 0 && loc.EndColumn == 0
}

// sourceFileName returns the source-file identity of a path label: the base
// name SANY compares with the root module's name (FileUtil.java:347-380,
// ParseUnit.java:202-231). Only a label whose last element, after the final
// '/' or '\', is <base>.tla with a nonempty base and the extension in any
// letter case has one. Any other label, including the empty label, is opaque
// and yields "".
func sourceFileName(label string) string {
	base := label[strings.LastIndexAny(label, `/\`)+1:]
	const ext = ".tla"
	if len(base) <= len(ext) || !strings.EqualFold(base[len(base)-len(ext):], ext) {
		return ""
	}
	return base[:len(base)-len(ext)]
}

// fileNameMismatchDiagnostic reports ParseUnit's
// MODULE_NAME_DIFFERENT_FROM_FILE_NAME abort. SANY's location is null; the
// diagnostic is attached to the mismatching source with an empty span.
func fileNameMismatchDiagnostic(e *modulegraph.FileNameMismatchError) Diagnostic {
	return Diagnostic{Phase: LevelSemantic, Severity: SeverityError, Code: semanticCode(sanyModuleNameDifferentFromFile), Message: e.Error(), Path: e.Path}
}

// validationDiagnostics reports SpecObj.validateParseUnit's INTERNAL_ERROR
// for each unit whose PlusCal validation failed, in SANY's order
// (SpecObj.java:1043-1050). SANY's location names only the module; the span is
// empty at the start of that module's source.
func validationDiagnostics(acq *modulegraph.Acquisition) []Diagnostic {
	if acq == nil {
		return nil
	}
	var out []Diagnostic
	for _, d := range acq.Validation {
		var sp Span
		if d.Input.View != nil {
			if a, _, ok := d.Input.View.MapSpan(0, 0); ok {
				sp = span(positions(d.Input.View.Owner), a, a)
			}
		}
		message := "An unexpected problem was encountered attempting to validate the specification for " + d.Name
		out = append(out, Diagnostic{Phase: LevelSemantic, Severity: SeverityError, Code: semanticCode(sanyInternalError), Message: message, Path: d.Input.Path, Span: sp})
	}
	return out
}

func semanticCode(value int) string { return "semantic/" + strconv.Itoa(value) }

func hasError(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// incompletePhase and incompleteMessage describe an unsupported or budgeted
// stop by its fixed origin site ("<stage>.<category>.<detail>").
func incompletePhase(site string) AnalysisLevel {
	if strings.HasPrefix(site, "level.") {
		return LevelComplete
	}
	return LevelSemantic
}

func incompleteMessage(site string) string {
	stage := "semantic analysis"
	if strings.HasPrefix(site, "level.") {
		stage = "level analysis"
	}
	parts := strings.SplitN(site, ".", 3)
	reason := "an unsupported construct"
	if len(parts) == 3 && parts[1] == "resource" {
		reason = "a resource limit"
	}
	if len(parts) != 3 {
		return stage + " stopped at " + reason
	}
	return stage + " stopped at " + reason + " (" + site + ")"
}

// referenceSpan locates a syntax node of an acquired document in the
// coordinates of the document's owner (the root for a monolithic view).
func referenceSpan(ctx context.Context, d *modulegraph.Document, tree *sanyparser.SyntaxTreeNode) (Span, bool) {
	if d == nil || tree == nil || ctx.Err() != nil {
		return Span{}, false
	}
	loc := tree.Location()
	if loc == nil {
		return Span{}, false
	}
	javaPositions, ranges := sany.NewEditor(string(d.Input.Text)).InputCoordinates()
	coords := make(coordIndex, len(javaPositions))
	for i, pos := range javaPositions {
		coords[javaCoord{pos.Line, pos.Column}] = ranges[i]
	}
	first, a := coords[javaCoord{int(loc.BeginLine()), int(loc.BeginColumn())}]
	last, b := coords[javaCoord{int(loc.EndLine()), int(loc.EndColumn())}]
	if !a || !b || last[1] < first[0] {
		return Span{}, false
	}
	start, end := first[0], last[1]
	if d.Input.View != nil {
		var ok bool
		if start, end, ok = d.Input.View.MapSpan(start, end); !ok {
			return Span{}, false
		}
		return span(positions(d.Input.View.Owner), start, end), true
	}
	return span(positions(d.Input.Text), start, end), true
}

// moduleNotFoundDiagnostic reports SpecObj's MODULE_FILE_CANNOT_BE_FOUND abort
// for the reference acquisition was resolving (SpecObj.java:270-293). SANY's
// location names only the importing module; the span is the reference name.
func moduleNotFoundDiagnostic(ctx context.Context, acq *modulegraph.Acquisition) (Diagnostic, bool) {
	if acq == nil || acq.Request == nil || acq.RequestOwner == nil || acq.RequestOwner.Document == nil {
		return Diagnostic{}, false
	}
	owner := acq.RequestOwner
	sp, ok := referenceSpan(ctx, owner.Document, acq.Request.Tree)
	if !ok {
		return Diagnostic{}, false
	}
	message := "Cannot find source file for module " + acq.Request.Name + " imported in module " + owner.Name + "."
	return Diagnostic{Phase: LevelSemantic, Severity: SeverityError, Code: semanticCode(sanyModuleFileCannotBeFound), Message: message, Path: owner.Document.Input.Path, Span: sp}, true
}

// moduleCycleDiagnostic reports SpecObj's MODULE_DEPENDENCIES_ARE_CIRCULAR
// abort (SpecObj.java:376-389). SANY's location is null; the span is the
// reference whose link closed the cycle when it is known, otherwise the
// diagnostic is attached to the root path with an empty span. File names in
// the message are the caller's path labels.
func moduleCycleDiagnostic(ctx context.Context, rootPath string, acq *modulegraph.Acquisition, cycle *modulegraph.CycleError) (Diagnostic, bool) {
	if cycle == nil || len(cycle.Path) < 2 {
		return Diagnostic{}, false
	}
	message := "Circular dependency among .tla files; dependency cycle is:\n\n  " + strings.Join(cycle.Path, " --> ")
	d := Diagnostic{Phase: LevelSemantic, Severity: SeverityError, Code: semanticCode(sanyModuleDependenciesAreCircular), Message: message, Path: rootPath}
	if acq != nil && acq.Request != nil && acq.RequestOwner != nil && acq.RequestOwner.Document != nil {
		if sp, ok := referenceSpan(ctx, acq.RequestOwner.Document, acq.Request.Tree); ok {
			d.Path, d.Span = acq.RequestOwner.Document.Input.Path, sp
		}
	}
	return d, true
}
