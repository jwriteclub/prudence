// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Command gpu-export writes a resolved semantic graph as JSON to stdout.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/jwriteclub/prudence/internal/gpuexport"
	"github.com/jwriteclub/prudence/internal/modulegraph"
)

func read(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds %d bytes", path, limit)
	}
	return data, err
}
func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: gpu-export MODULE.tla MODEL.cfg")
	}
	path := args[0]
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if !strings.EqualFold(ext, ".tla") {
		return fmt.Errorf("expected .tla module file")
	}
	src, err := read(path, gpuexport.MaxSourceBytes)
	if err != nil {
		return err
	}
	cfg, err := read(args[1], gpuexport.MaxConfigBytes)
	if err != nil {
		return err
	}
	root := modulegraph.Input{FileName: strings.TrimSuffix(base, ext), Path: base, Text: src}
	resolve := func(ctx context.Context, name string) (modulegraph.Input, error) {
		if err := ctx.Err(); err != nil {
			return modulegraph.Input{}, err
		}
		filename := name + ".tla"
		if filename != filepath.Base(filename) {
			return modulegraph.Input{}, fmt.Errorf("invalid module name %q", name)
		}
		data, err := read(filepath.Join(filepath.Dir(path), filename), gpuexport.MaxSourceBytes)
		return modulegraph.Input{Path: filename, Text: data}, err
	}
	doc, err := gpuexport.Export(ctx, root, resolve, filepath.Base(args[1]), cfg)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(doc)
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Args[1:], os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
