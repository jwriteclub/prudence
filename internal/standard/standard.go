// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Package standard provides the pinned SANY standard resource namespace.
package standard

import (
	"embed"
	"io/fs"
)

//go:embed modules/*.tla
var modules embed.FS

// Lookup returns a fresh copy of a resource, or nil if the name is absent.
func Lookup(name string) []byte {
	// Module names are identifiers, not paths. Reject path syntax before lookup.
	for _, r := range name {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return nil
		}
	}
	if name == "" {
		return nil
	}
	b, err := fs.ReadFile(modules, "modules/"+name+".tla")
	if err != nil {
		return nil
	}
	return b
}
