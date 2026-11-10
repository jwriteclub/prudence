//go:build prudence_phase2_adapter

// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	stdcontext "context"
	"errors"
	"io"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/modulegraph"
)

// Phase2Analysis retains one request-owned analysis and its ordered raw details.
// Complete means the modeled generator returned normally, not phase acceptance.
// Aborted means the flat driver stopped at a modeled AbortException: Details
// is then the whole semantic log at the throw, which SANY.java:503 reports
// before SANY.parse returns ERROR (SANY.java:215). It never sets Complete.
type Phase2Analysis struct {
	Details                     []Detail
	Examined, Complete, Aborted bool
	Err                         error
	module                      *moduleNode
	roots                       []*moduleNode
}

func Phase2Observe(ctx stdcontext.Context, tree *generated.SyntaxTreeNode, maxDepth int) Phase2Analysis {
	if ctx == nil || tree == nil {
		return Phase2Analysis{Err: errors.New("invalid semantic observation arguments")}
	}
	g, m, e := analyzeOwned(ctx, tree, maxDepth)
	out := Phase2Analysis{Examined: m != nil, Complete: e == nil && g != nil && m != nil, Err: e, module: m, roots: []*moduleNode{m}}
	if g != nil {
		out.Details = g.table.errors.Messages()
	}
	return out
}

// Phase2ObserveFlat uses the same request-owned generator as public analysis.
func Phase2ObserveFlat(ctx stdcontext.Context, acquisition *modulegraph.Acquisition, maxDepth int) Phase2Analysis {
	details, examined, root, roots, err := analyzeFlatExternalRoots(ctx, acquisition, maxDepth)
	return Phase2Analysis{Details: details, Examined: examined, Complete: err == nil && root != nil, Aborted: sourceAbort(err, details), Err: err, module: root, roots: roots}
}

// WriteXML rejects an incomplete walk or an error diagnostic, matching the
// semantic-only Java adapter's successful-Errors gate. Warnings do not block it.
func (a Phase2Analysis) WriteXML(ctx stdcontext.Context, w io.Writer, maxNodes int, spoolDir string, spoolLimit int64) error {
	if !a.Complete || a.module == nil {
		if a.Err != nil {
			return a.Err
		}
		return errors.New("semantic module unavailable")
	}
	for _, d := range a.Details {
		if d.Code.Severity == Error {
			return errors.New("semantic errors prevent graph export")
		}
	}
	return phase2XMLRoots(ctx, w, a.module, a.roots, maxNodes, spoolDir, spoolLimit)
}
func Phase2Export(ctx stdcontext.Context, tree *generated.SyntaxTreeNode, w io.Writer, maxDepth, maxNodes int, spoolDir string, spoolLimit int64) error {
	return Phase2Observe(ctx, tree, maxDepth).WriteXML(ctx, w, maxNodes, spoolDir, spoolLimit)
}
