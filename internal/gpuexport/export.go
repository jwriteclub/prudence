// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Package gpuexport exports a checked semantic graph and ordered model
// configuration for compiler backends. File paths are labels and all source
// bytes and state belong to a single request.
package gpuexport

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jwriteclub/prudence/internal/modulegraph"
	"github.com/jwriteclub/prudence/internal/recursion"
	"github.com/jwriteclub/prudence/internal/semantic"
	"github.com/jwriteclub/prudence/internal/standard"
)

const MaxSourceBytes = 32 << 20
const MaxConfigBytes = 1 << 20

type InputIdentity struct {
	Module string `json:"module"`
	Path   string `json:"path"`
	Origin string `json:"origin"`
	SHA256 string `json:"sha256"`
}
type Document struct {
	Schema  string `json:"schema"`
	Version int    `json:"version"`
	*semantic.ResolvedGraph
	ConfigSHA256 string          `json:"cfg_sha256"`
	RootSHA256   string          `json:"root_source_sha256"`
	Sources      []InputIdentity `json:"sources"`
	Config       []Directive     `json:"cfg"`
}

func digest(blob []byte) string { return fmt.Sprintf("%x", sha256.Sum256(blob)) }

// Export parses a separate cfg and acquires the complete semantic graph.
// A missing dependency (fs.ErrNotExist) falls through to the embedded standard
// library. Caller-provided sources always have workspace provenance.
func Export(ctx context.Context, root modulegraph.Input, resolve modulegraph.Resolver, cfgPath string, cfg []byte) (*Document, error) {
	if ctx == nil {
		return nil, fmt.Errorf("missing context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(cfg) > MaxConfigBytes {
		return nil, fmt.Errorf("%s: cfg exceeds %d bytes", cfgPath, MaxConfigBytes)
	}
	directives, err := parseConfig(ctx, cfgPath, cfg)
	if err != nil {
		return nil, err
	}
	root.Origin = modulegraph.OriginWorkspace
	source := func(ctx context.Context, name string) (modulegraph.Input, error) {
		if resolve != nil {
			in, err := resolve(ctx, name)
			if err == nil {
				in.Origin = modulegraph.OriginWorkspace
				return in, nil
			}
			if !errors.Is(err, fs.ErrNotExist) {
				return modulegraph.Input{}, err
			}
		}
		if err := ctx.Err(); err != nil {
			return modulegraph.Input{}, err
		}
		if buf := standard.Lookup(name); buf != nil {
			return modulegraph.Input{Path: name + ".tla", Text: buf, Origin: modulegraph.OriginStandard}, nil
		}
		return modulegraph.Input{}, fmt.Errorf("module %q: %w", name, fs.ErrNotExist)
	}
	limits := modulegraph.AcquisitionLimits{MaxDepth: recursion.DefaultMaxDepth, MaxVisits: 1_000_000, MaxUnits: 10_000, MaxEdges: 100_000, MaxNodes: 1_000_000, MaxSourceBytes: MaxSourceBytes, MaxTotalBytes: 128 << 20}
	a, err := modulegraph.AcquireFlatExternal(ctx, root, source, limits)
	if err != nil {
		if a != nil && a.Failed != nil {
			return nil, fmt.Errorf("%s: %w", a.Failed.Input.Path, err)
		}
		return nil, fmt.Errorf("%s: %w", root.Path, err)
	}
	if len(a.Validation) > 0 {
		return nil, fmt.Errorf("%s: PlusCal translation validation failed", a.Validation[0].Input.Path)
	}
	graph, err := semantic.ExportResolved(ctx, a, limits.MaxDepth)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", root.Path, err)
	}
	doc := &Document{Schema: "prudence.resolved", Version: 1, ResolvedGraph: graph, ConfigSHA256: digest(cfg), RootSHA256: digest(a.Root.Input.Text), Config: directives}
	for _, d := range a.Units {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		origin := "workspace"
		if d.Input.Origin == modulegraph.OriginStandard {
			origin = "standard"
		}
		doc.Sources = append(doc.Sources, InputIdentity{Module: d.Name, Path: d.Input.Path, Origin: origin, SHA256: digest(d.Input.Text)})
	}
	sort.Slice(doc.Sources, func(i, j int) bool { return doc.Sources[i].Module < doc.Sources[j].Module })
	return doc, nil
}
