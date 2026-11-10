// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package prudence

import (
	"context"
	"os"
	"testing"
)

func TestPublicBoundedLabelOrdering(t *testing.T) {
	for _, tt := range []struct{ file, want string }{
		{"Reverse", "Label L declares extra parameter(s)  a  b  "},
		{"Wrap", "Label L declares extra parameter(s)  j  a  "},
	} {
		src, e := os.ReadFile("testdata/label-order/" + tt.file + ".tla")
		if e != nil {
			t.Fatal(e)
		}
		r, e := Check(context.Background(), Request{Stage: LevelSemantic, Path: tt.file + ".tla", Text: src})
		if e != nil {
			t.Fatal(e)
		}
		if len(r.Diagnostics) < 1 || r.Diagnostics[0].Code != "semantic/4332" || r.Diagnostics[0].Message != tt.want {
			t.Fatalf("%s: %#v", tt.file, r.Diagnostics)
		}
	}
}
