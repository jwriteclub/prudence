// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

// This private callback is a phase-gate preparation aid. It never turns a
// partial Generator walk into a complete semantic verdict or oracle match.
import (
	stdcontext "context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/recursion"
)

type differentialFile struct{ Path, Role, SHA256 string }
type differentialLimits struct{ InputBytes, SemanticNodes, SyntaxNodes, MaxDepth int }
type differentialInput struct {
	Ordinal                                                     int
	Set, ID, RootFile, InputManifestSHA, SourcePin, SettingsPin string
	Files                                                       []differentialFile
	RootOrigin, ProviderOrigin, ResolverResult, RequestedStage  string
	Source                                                      []byte
	Limits                                                      differentialLimits
}
type differentialField struct{ State, Reason string }
type differentialOutput struct {
	Schema                                                                    int
	Ordinal                                                                   int
	Set, ID, RootFile, InputManifestSHA, SourcePin, SettingsPin               string
	Files                                                                     []differentialFile
	RootOrigin, ProviderOrigin, ResolverResult, RequestedStage, ExaminedStage string
	Completion, Outcome                                                       string
	Fields                                                                    map[string]differentialField
	Graph                                                                     *projectedGraph
}

func partialDifferential(ctx stdcontext.Context, in differentialInput, emit func(differentialOutput) error) error {
	if ctx == nil || emit == nil {
		return ErrPortIncomplete
	}
	out := differentialOutput{Schema: 1, Ordinal: in.Ordinal, Set: in.Set, ID: in.ID, RootFile: in.RootFile, InputManifestSHA: in.InputManifestSHA, SourcePin: in.SourcePin, SettingsPin: in.SettingsPin, Files: append([]differentialFile(nil), in.Files...), RootOrigin: in.RootOrigin, ProviderOrigin: in.ProviderOrigin, ResolverResult: in.ResolverResult, RequestedStage: in.RequestedStage, ExaminedStage: "none", Completion: "incomplete", Outcome: "incomplete", Fields: map[string]differentialField{
		"parse_diagnostics":    {"incomplete", "exact rendered diagnostics and raw spans are not projected"},
		"semantic_diagnostics": {"incomplete", "exact rendering and full diagnostic paths are not implemented"},
		"syntax":               {"incomplete", "module provenance and complete failure-tree mapping are not implemented"},
		"semantic_graph":       {"incomplete", "node coverage and fields are partial"},
		"level":                {"incomplete", "general level propagation is not implemented"},
		"oracle_mapping":       {"incomplete", "XML exporter mapping is not established"},
		"transport":            {"incomplete", "byte, collection and elapsed-time bounds are not fully implemented"},
	}}
	if err := ctx.Err(); err != nil {
		out.Fields["operation"] = differentialField{"incomplete", err.Error()}
		return emit(out)
	}
	if in.Limits.InputBytes <= 0 || in.Limits.SemanticNodes <= 0 || in.Limits.SyntaxNodes <= 0 || in.Limits.MaxDepth < 0 || len(in.Source) > in.Limits.InputBytes {
		out.Fields["operation"] = differentialField{"incomplete", "input or node limit"}
		return emit(out)
	}
	found := false
	sum := sha256.Sum256(in.Source)
	digest := hex.EncodeToString(sum[:])
	for _, f := range in.Files {
		if f.Path == in.RootFile {
			found = true
			if f.SHA256 != digest {
				out.Outcome = "infra"
				out.Fields["operation"] = differentialField{"incomplete", "root hash mismatch"}
				return emit(out)
			}
		}
	}
	if !found {
		out.Outcome = "infra"
		out.Fields["operation"] = differentialField{"incomplete", "root metadata missing"}
		return emit(out)
	}
	result, err := generated.Parse(ctx, in.Source, generated.ParseOptions{Recursion: recursion.Options{MaxDepth: in.Limits.MaxDepth}})
	if err != nil {
		out.Fields["operation"] = differentialField{"incomplete", err.Error()}
		return emit(out)
	}
	out.ExaminedStage = "syntax"
	if !result.Accepted || result.Tree == nil {
		return emit(out)
	}
	g, m, err := analyzeOwned(ctx, result.Tree, in.Limits.MaxDepth)
	if g == nil {
		return err
	}
	if m != nil {
		out.ExaminedStage = "semantic"
		graph := projectGraphLimited(g, m, in.Limits.SemanticNodes, in.Limits.SyntaxNodes)
		out.Graph = &graph
		if graph.Truncated {
			out.Fields["semantic_graph"] = differentialField{"incomplete", "node limit"}
		}
	}
	if err != nil {
		if errors.Is(err, ErrPortIncomplete) {
			out.Fields["operation"] = differentialField{"incomplete", "unsupported semantic construct"}
		} else {
			out.Fields["operation"] = differentialField{"incomplete", err.Error()}
		}
	} else {
		out.Fields["operation"] = differentialField{"incomplete", "partial semantic generation only"}
	}
	return emit(out)
}
func TestPartialDifferentialNeverClaimsCompletion(t *testing.T) {
	source := []byte("---- MODULE M ----\nA == 1\n====\n")
	sum := sha256.Sum256(source)
	in := differentialInput{RootFile: "M.tla", Source: source, Files: []differentialFile{{Path: "M.tla", Role: "root", SHA256: hex.EncodeToString(sum[:])}}, Limits: differentialLimits{InputBytes: 1024, SemanticNodes: 1000, SyntaxNodes: 1000}, RequestedStage: "semantic"}
	var got differentialOutput
	err := partialDifferential(stdcontext.Background(), in, func(out differentialOutput) error { got = out; return nil })
	if err != nil || got.Completion != "incomplete" || got.Outcome != "incomplete" || got.Graph == nil || got.Fields["level"].State != "incomplete" {
		t.Fatal(got, err)
	}
	in.Files[0].SHA256 = "bad"
	err = partialDifferential(stdcontext.Background(), in, func(out differentialOutput) error { got = out; return nil })
	if err != nil || got.Outcome != "infra" || got.Graph != nil {
		t.Fatal(got, err)
	}
}

func TestPartialDifferentialLimitAndCancellation(t *testing.T) {
	in := differentialInput{Source: []byte("too long"), Limits: differentialLimits{InputBytes: 1, SemanticNodes: 1, SyntaxNodes: 1}}
	var got differentialOutput
	if e := partialDifferential(stdcontext.Background(), in, func(out differentialOutput) error { got = out; return nil }); e != nil || got.Outcome != "incomplete" || got.Fields["operation"].Reason != "input or node limit" {
		t.Fatal(got, e)
	}
	ctx, cancel := stdcontext.WithCancel(stdcontext.Background())
	cancel()
	if e := partialDifferential(ctx, in, func(out differentialOutput) error { got = out; return nil }); e != nil || got.Outcome != "incomplete" || got.ExaminedStage != "none" {
		t.Fatal(got, e)
	}
}
