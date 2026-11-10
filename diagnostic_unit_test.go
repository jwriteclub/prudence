// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package prudence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// Diagnostic.Unit preserves the SANY parse unit that reported a diagnostic
// inside a monolith-extracted module. SANY analyzes every unit of
// semanticAnalysisVector separately, dependencies first, with one shared
// Errors (SANY.java:415-449), so an extracted copy of nested module N reports
// "... of module N" in N's own numbering (Location.java:31; the module name is
// the parse unit's first module, TLAplusParser.java:1549, given to every node,
// SyntaxTreeNode.java:458-459), and the root's nested N reports again in root
// numbering. MonolithSpecExtractor.module (MonolithSpecExtractor.java:77-107)
// copies from the first matching header line to the first "====" line.

// unitSANY is one SANY entry: module, "line l1, col c1 to line l2, col c2"
// and the exact message bytes.
type unitSANY struct {
	module         string
	l1, c1, l2, c2 int
	message        string
}

// diagnosticUnitCases are recorded sany-conformance fuzz-jazzer cases
// (tlaplus dc6a944), verbatim under testdata/diagnostic-unit (provenance in
// PROVENANCE); TLAPS.tla is one shared copy. errors and warnings are SANY's
// last printed "*** Errors" and "*** Warnings" lists (manifest stdout), which
// hold every record of the shared Errors in order. D-9421ecca2300,
// D-6b9c94d6e912 and D-9978766c7f68 declare Inner twice; the extracted unit
// is the first declaration.
var diagnosticUnitCases = []struct {
	id, root         string
	files            map[string]string // file name to SHA-256
	errors, warnings []unitSANY
}{
	{id: "D-0f79429a4cce", root: "INSTANCE_inside_LET_test.tla", files: map[string]string{
		"INSTANCE_inside_LET_test.tla": "c628d6e4b44c345980f9ba41410044a0acd3c34c65ade08f0008f3f198773a18",
		"TLAPS.tla":                    "9afe54984062748a0568966434cc0945d682f8cd89fdbc38f73b5579751b0c55",
	}, errors: []unitSANY{
		{"Inner", 2, 14, 4, 9, "Operator <> already defined or declared."},
		{"Inner", 2, 14, 4, 9, "Symbol <> is a built-in operator, and cannot be redefined."},
		{"INSTANCE_inside_LET_test", 10, 14, 12, 9, "Operator <> already defined or declared."},
		{"INSTANCE_inside_LET_test", 10, 14, 12, 9, "Symbol <> is a built-in operator, and cannot be redefined."},
		{"INSTANCE_inside_LET_test", 9, 1, 13, 22, "Multiply-defined module 'Inner': this definition or declaration conflicts \nwith the one at line 1, col 1 to line 5, col 22 of module Inner."},
		{"INSTANCE_inside_LET_test", 18, 8, 18, 10, "Unknown operator: `M!A'."},
	}, warnings: []unitSANY{}},
	{id: "D-9421ecca2300", root: "get.tla", files: map[string]string{
		"TLAPS.tla": "9afe54984062748a0568966434cc0945d682f8cd89fdbc38f73b5579751b0c55",
		"get.tla":   "4288ca0ea5196cfa4cee890f76719c75a25d7256dcdaf5750ce868b5447801cd",
	}, errors: []unitSANY{
		{"Inner", 6, 6, 6, 6, "Unknown operator: `y'."},
		{"get", 3, 10, 3, 10, "Unknown operator: `a'."},
		{"get", 3, 16, 3, 18, "Unknown operator: `Nat'."},
		{"get", 4, 9, 4, 9, "Unknown operator: `a'."},
		{"get", 4, 13, 4, 13, "Unknown operator: `a'."},
		{"get", 5, 9, 6, 44, "Label not in definition or proof step."},
		{"get", 14, 6, 14, 6, "Unknown operator: `y'."},
		{"get", 9, 1, 16, 6, "Multiply-defined module 'Inner': this definition or declaration conflicts \nwith the one at line 1, col 1 to line 8, col 6 of module Inner."},
		{"get", 18, 10, 18, 10, "Unknown operator: `a'."},
		{"get", 18, 16, 18, 18, "Unknown operator: `Nat'."},
		{"get", 19, 9, 19, 9, "Unknown operator: `a'."},
		{"get", 19, 13, 19, 13, "Unknown operator: `a'."},
		{"get", 20, 37, 20, 37, "Nonexistent operand specified by `argument selector 3'."},
		{"get", 21, 21, 21, 21, "Nonexistent operand specified by `argument selector 3'."},
		{"get", 22, 4, 22, 6, "Unknown operator: `SMS'."},
		{"get", 24, 9, 24, 25, "Unknown operator: `Op1!Op1a'."},
		{"get", 24, 35, 24, 35, "Could not find declaration or definition of symbol '+'."},
		{"get", 24, 32, 24, 37, "Couldn't resolve infix operator symbol `+'."},
		{"get", 25, 4, 25, 6, "Unknown operator: `SMT'."},
		{"get", 28, 54, 28, 54, "Unknown operator: `x'."},
		{"get", 28, 19, 28, 19, "Illegal parameter x of label `CHOOSEFUNCTION'."},
		{"get", 28, 4, 29, 44, "Label CHOOSEFUNCTION declares extra parameter(s)  x  "},
		{"get", 37, 6, 37, 6, "Unknown operator: `y'."},
		{"get", 32, 1, 39, 15, "Multiply-defined module 'Inner': this definition or declaration conflicts \nwith the one at line 1, col 1 to line 8, col 6 of module Inner."},
		{"get", 42, 6, 42, 20, "Unknown operator: `INSTANCEN_NonLo'."},
		{"get", 45, 6, 45, 31, "Unknown operator: `INSTANCEN_NonLocalInstance'."},
		{"get", 50, 38, 50, 38, "Identifier 'y' is not a legal target of a substitution. \nA legal target must be a declared CONSTANT or VARIABLE in the module being instantiated. \n(Also, check for warnings about multiple declarations of this same identifier.)"},
		{"get", 50, 13, 50, 43, "Substitution missing for symbol w declared at line 4, col 10 to line 4, col 10 of module Inner \nand instantiated in module get."},
	}, warnings: []unitSANY{
		{"get", 17, 1, 19, 13, "Multiple declarations or definitions for symbol T.  \nThis duplicates the one at line 2, col 1 to line 4, col 13 of module get."},
	}},
	{id: "D-6b9c94d6e912", root: "get.tla", files: map[string]string{
		"TLAPS.tla": "9afe54984062748a0568966434cc0945d682f8cd89fdbc38f73b5579751b0c55",
		"get.tla":   "d17e98fb67c65d1421f6f129fbcbd14434c4de3cc5eebb4cc465f08207d275df",
	}, errors: []unitSANY{
		{"Inner", 6, 6, 6, 6, "Unknown operator: `y'."},
		{"get", 3, 10, 3, 10, "Unknown operator: `a'."},
		{"get", 3, 16, 3, 18, "Unknown operator: `Nat'."},
		{"get", 4, 9, 4, 9, "Unknown operator: `a'."},
		{"get", 4, 13, 4, 13, "Unknown operator: `a'."},
		{"get", 5, 21, 5, 21, "Nonexistent operand specified by `argument selector 3'."},
		{"get", 6, 3, 6, 3, "Unknown operator: `a'."},
		{"get", 7, 21, 7, 21, "Nonexistent operand specified by `argument selector 3'."},
		{"get", 8, 21, 8, 21, "Nonexistent operand specified by `argument selector 3'."},
		{"get", 9, 4, 9, 6, "Unknown operator: `SMS'."},
		{"get", 11, 9, 11, 25, "Unknown operator: `Op1!Op1a'."},
		{"get", 11, 35, 11, 35, "Could not find declaration or definition of symbol '+'."},
		{"get", 11, 32, 11, 37, "Couldn't resolve infix operator symbol `+'."},
		{"get", 12, 4, 12, 6, "Unknown operator: `SMT'."},
		{"get", 15, 54, 15, 54, "Unknown operator: `x'."},
		{"get", 15, 19, 15, 19, "Illegal parameter x of label `CHOOSEFUNCTION'."},
		{"get", 15, 4, 16, 44, "Label CHOOSEFUNCTION declares extra parameter(s)  x  "},
		{"get", 18, 19, 18, 21, "A LET/IN expression has only one operand."},
		{"get", 18, 52, 18, 55, "Unknown operator: `Salt'."},
		{"get", 21, 19, 21, 21, "A LET/IN expression has only one operand."},
		{"get", 21, 50, 21, 53, "Unknown operator: `Salt'."},
		{"get", 29, 6, 29, 6, "Unknown operator: `y'."},
		{"get", 24, 1, 31, 10, "Multiply-defined module 'Inner': this definition or declaration conflicts \nwith the one at line 1, col 1 to line 8, col 10 of module Inner."},
		{"get", 31, 31, 31, 31, "Nonexistent operand specified by `argument selector 3'."},
		{"get", 32, 4, 32, 6, "Unknown operator: `SMS'."},
		{"get", 34, 9, 34, 25, "Unknown operator: `Op1!Op1a'."},
		{"get", 34, 35, 34, 35, "Could not find declaration or definition of symbol '+'."},
		{"get", 34, 32, 34, 37, "Couldn't resolve infix operator symbol `+'."},
		{"get", 35, 4, 35, 6, "Unknown operator: `SMT'."},
		{"get", 37, 1, 39, 44, "Operator COp already defined or declared."},
		{"get", 38, 54, 38, 54, "Unknown operator: `x'."},
		{"get", 38, 19, 38, 19, "Illegal parameter x of label `CHOOSEFUNCTION'."},
		{"get", 38, 4, 39, 44, "Label CHOOSEFUNCTION declares extra parameter(s)  x  "},
		{"get", 41, 19, 41, 21, "A LET/IN expression has only one operand."},
		{"get", 41, 52, 41, 55, "Unknown operator: `Salt'."},
		{"get", 44, 19, 44, 21, "A LET/IN expression has only one operand."},
		{"get", 44, 50, 44, 53, "Unknown operator: `Salt'."},
		{"get", 52, 6, 52, 6, "Unknown operator: `y'."},
		{"get", 47, 1, 54, 22, "Multiply-defined module 'Inner': this definition or declaration conflicts \nwith the one at line 1, col 1 to line 8, col 10 of module Inner."},
		{"get", 57, 6, 57, 31, "Unknown operator: `INSTANCEN_NonLocalInstance'."},
		{"get", 62, 38, 62, 38, "Identifier 'y' is not a legal target of a substitution. \nA legal target must be a declared CONSTANT or VARIABLE in the module being instantiated. \n(Also, check for warnings about multiple declarations of this same identifier.)"},
		{"get", 62, 13, 62, 43, "Substitution missing for symbol w declared at line 4, col 10 to line 4, col 10 of module Inner \nand instantiated in module get."},
	}, warnings: []unitSANY{
		{"get", 7, 1, 7, 21, "Multiple declarations or definitions for symbol BadSel.  \nThis duplicates the one at line 5, col 1 to line 6, col 3 of module get."},
		{"get", 8, 1, 9, 6, "Multiple declarations or definitions for symbol BadSel.  \nThis duplicates the one at line 5, col 1 to line 6, col 3 of module get."},
		{"get", 31, 11, 32, 6, "Multiple declarations or definitions for symbol BadSel.  \nThis duplicates the one at line 5, col 1 to line 6, col 3 of module get."},
		{"get", 37, 1, 39, 44, "Multiple declarations or definitions for symbol COp.  \nThis duplicates the one at line 14, col 1 to line 16, col 44 of module get."},
	}},
	{id: "D-9978766c7f68", root: "get.tla", files: map[string]string{
		"TLAPS.tla": "9afe54984062748a0568966434cc0945d682f8cd89fdbc38f73b5579751b0c55",
		"get.tla":   "94e15459dbdfd1d1a3f4577385e30e918ea6e0e5d14fd5f7d04514ddfbd11602",
	}, errors: []unitSANY{
		{"Inner", 6, 71, 6, 72, "Could not find declaration or definition of symbol '##'."},
		{"Inner", 6, 6, 6, 74, "Couldn't resolve infix operator symbol `##'."},
		{"Inner", 6, 76, 6, 76, "The operator ' requires 1 arguments."},
		{"get", 3, 10, 3, 10, "Unknown operator: `a'."},
		{"get", 3, 16, 3, 18, "Unknown operator: `Nat'."},
		{"get", 4, 9, 4, 9, "Unknown operator: `a'."},
		{"get", 4, 13, 4, 13, "Unknown operator: `a'."},
		{"get", 6, 3, 6, 3, "Unknown operator: `a'."},
		{"get", 7, 21, 7, 21, "Nonexistent operand specified by `argument selector 3'."},
		{"get", 8, 21, 8, 21, "Nonexistent operand specified by `argument selector 3'."},
		{"get", 9, 4, 9, 6, "Unknown operator: `SMS'."},
		{"get", 11, 9, 11, 25, "Unknown operator: `Op1!Op1a'."},
		{"get", 11, 35, 11, 35, "Could not find declaration or definition of symbol '+'."},
		{"get", 11, 32, 11, 37, "Couldn't resolve infix operator symbol `+'."},
		{"get", 12, 4, 12, 6, "Unknown operator: `SMT'."},
		{"get", 15, 54, 15, 54, "Unknown operator: `x'."},
		{"get", 15, 19, 15, 19, "Illegal parameter x of label `CHOOSEFUNCTION'."},
		{"get", 15, 4, 16, 44, "Label CHOOSEFUNCTION declares extra parameter(s)  x  "},
		{"get", 18, 19, 18, 21, "A LET/IN expression has only one operand."},
		{"get", 18, 52, 18, 55, "Unknown operator: `Salt'."},
		{"get", 21, 19, 21, 21, "A LET/IN expression has only one operand."},
		{"get", 21, 50, 21, 53, "Unknown operator: `Salt'."},
		{"get", 29, 71, 29, 72, "Could not find declaration or definition of symbol '##'."},
		{"get", 29, 6, 29, 74, "Couldn't resolve infix operator symbol `##'."},
		{"get", 29, 76, 29, 76, "The operator ' requires 1 arguments."},
		{"get", 24, 1, 31, 10, "Multiply-defined module 'Inner': this definition or declaration conflicts \nwith the one at line 1, col 1 to line 8, col 10 of module Inner."},
		{"get", 31, 31, 31, 31, "Nonexistent operand specified by `argument selector 3'."},
		{"get", 32, 4, 32, 6, "Unknown operator: `SMS'."},
		{"get", 34, 9, 34, 25, "Unknown operator: `Op1!Op1a'."},
		{"get", 34, 35, 34, 35, "Could not find declaration or definition of symbol '+'."},
		{"get", 34, 32, 34, 37, "Couldn't resolve infix operator symbol `+'."},
		{"get", 35, 4, 35, 6, "Unknown operator: `SMT'."},
		{"get", 37, 1, 39, 44, "Operator COp already defined or declared."},
		{"get", 38, 54, 38, 54, "Unknown operator: `x'."},
		{"get", 38, 19, 38, 19, "Illegal parameter x of label `CHOOSEFUNCTION'."},
		{"get", 38, 4, 39, 44, "Label CHOOSEFUNCTION declares extra parameter(s)  x  "},
		{"get", 41, 19, 41, 21, "A LET/IN expression has only one operand."},
		{"get", 41, 52, 41, 55, "Unknown operator: `Salt'."},
		{"get", 44, 19, 44, 21, "A LET/IN expression has only one operand."},
		{"get", 44, 50, 44, 53, "Unknown operator: `Salt'."},
		{"get", 52, 6, 52, 6, "Unknown operator: `y'."},
		{"get", 47, 1, 54, 22, "Multiply-defined module 'Inner': this definition or declaration conflicts \nwith the one at line 1, col 1 to line 8, col 10 of module Inner."},
		{"get", 57, 6, 57, 31, "Unknown operator: `INSTANCEN_NonLocalInstance'."},
		{"get", 62, 38, 62, 38, "Identifier 'y' is not a legal target of a substitution. \nA legal target must be a declared CONSTANT or VARIABLE in the module being instantiated. \n(Also, check for warnings about multiple declarations of this same identifier.)"},
		{"get", 62, 13, 62, 43, "Substitution missing for symbol w declared at line 4, col 10 to line 4, col 10 of module Inner \nand instantiated in module get."},
	}, warnings: []unitSANY{
		{"get", 7, 1, 7, 21, "Multiple declarations or definitions for symbol BadSel.  \nThis duplicates the one at line 5, col 1 to line 6, col 3 of module get."},
		{"get", 8, 1, 9, 6, "Multiple declarations or definitions for symbol BadSel.  \nThis duplicates the one at line 5, col 1 to line 6, col 3 of module get."},
		{"get", 31, 11, 32, 6, "Multiple declarations or definitions for symbol BadSel.  \nThis duplicates the one at line 5, col 1 to line 6, col 3 of module get."},
		{"get", 37, 1, 39, 44, "Multiple declarations or definitions for symbol COp.  \nThis duplicates the one at line 14, col 1 to line 16, col 44 of module get."},
	}},
	{id: "D-cabb48f9aaa0", root: "TSFSpecInsEOFt.tla", files: map[string]string{
		"TSFSpecInsEOFt.tla": "342eb7a297fc03a00314aa00fac8e9b5c8ff2068cc92db8279bd714504ac1650",
	}, errors: []unitSANY{
		{"Ring", 3, 9, 3, 14, "Unknown operator: `vality'."},
		{"Ring", 4, 9, 4, 13, "Unknown operator: `value'."},
		{"TSFSpecInsEOFt", 5, 9, 5, 14, "Unknown operator: `vality'."},
		{"TSFSpecInsEOFt", 6, 9, 6, 13, "Unknown operator: `value'."},
		{"TSFSpecInsEOFt", 3, 1, 7, 4, "Multiply-defined module 'Ring': this definition or declaration conflicts \nwith the one at line 1, col 1 to line 5, col 4 of module Ring."},
		{"TSFSpecInsEOFt", 8, 18, 8, 21, "The operator \\lor requires 2 arguments."},
		{"TSFSpecInsEOFt", 8, 23, 8, 35, "Substitution missing for symbol cardinalAty declared at line 2, col 10 to line 2, col 20 of module Ring \nand instantiated in module OFt."},
		{"TSFSpecInsEOFt", 9, 1, 9, 13, "Operator Apec already defined or declared."},
		{"TSFSpecInsEOFt", 9, 9, 9, 13, "Unknown operator: `value'."},
		{"TSFSpecInsEOFt", 11, 18, 11, 21, "The operator \\lor requires 2 arguments."},
		{"TSFSpecInsEOFt", 11, 23, 11, 35, "Substitution missing for symbol cardinalAty declared at line 2, col 10 to line 2, col 20 of module Ring \nand instantiated in module TSFSpecInsEOFt."},
		{"TSFSpecInsEOFt", 12, 14, 12, 17, "Cannot find label `Spec'."},
		{"TSFSpecInsEOFt", 13, 14, 13, 14, "Could not find declaration or definition of symbol '<'."},
		{"TSFSpecInsEOFt", 13, 8, 13, 16, "Couldn't resolve infix operator symbol `<'."},
	}, warnings: []unitSANY{
		{"TSFSpecInsEOFt", 9, 1, 9, 13, "Multiple declarations or definitions for symbol Apec.  \nThis duplicates the one at line 8, col 23 to line 8, col 35 of module TSFSpecInsEOFt."},
	}},
	{id: "D-1a969e12da5f", root: "Github1161.tla", files: map[string]string{
		"Github1161.tla": "106173e0f890576e6b4ea41296f0f299fdfcfa453bc1a978fbc327eb060b7918",
	}, errors: []unitSANY{
		{"Github1161Abstract", 8, 10, 8, 10, "Could not find declaration or definition of symbol '⊔'."},
		{"Github1161Abstract", 8, 9, 8, 11, "Couldn't resolve infix operator symbol `⊔'."},
		{"Github1161Abstract", 10, 21, 10, 29, "Unknown operator: `Ps!Double'."},
		{"Github1161", 8, 1, 8, 34, "Substitution missing for symbol Oq2 declared at line 4, col 10 to line 4, col 15 of module Github1161Abstract \nand instantiated in module Github1161."},
		{"Github1161", 19, 10, 19, 10, "Could not find declaration or definition of symbol '⊔'."},
		{"Github1161", 19, 9, 19, 11, "Couldn't resolve infix operator symbol `⊔'."},
		{"Github1161", 21, 21, 21, 29, "Unknown operator: `Ps!Double'."},
		{"Github1161", 12, 1, 25, 74, "Multiply-defined module 'Github1161Abstract': this definition or declaration conflicts \nwith the one at line 1, col 1 to line 14, col 74 of module Github1161Abstract."},
	}, warnings: []unitSANY{
		{"Github1161", 14, 10, 14, 10, "Multiple declarations or definitions for symbol s.  \nThis duplicates the one at line 3, col 10 to line 3, col 10 of module Github1161."},
	}},
	{id: "D-775b33c0c1e1", root: "Ie_to_omitted_modunits_test.tla", files: map[string]string{
		"Ie_to_omitted_modunits_test.tla": "18c167aa9fbd2c70e01b386366e2c34a5268fc5ca55d782fab3121c5b2d2d88d",
	}, errors: []unitSANY{
		{"Inner", 2, 6, 2, 6, "Unknown operator: `y'."},
		{"Ie_to_omitted_modunits_test", 6, 6, 6, 6, "Unknown operator: `y'."},
		{"Ie_to_omitted_modunits_test", 5, 1, 8, 22, "Multiply-defined module 'Inner': this definition or declaration conflicts \nwith the one at line 1, col 1 to line 4, col 22 of module Inner."},
		{"Ie_to_omitted_modunits_test", 11, 6, 11, 8, "Unknown operator: `INz'."},
		{"Ie_to_omitted_modunits_test", 16, 38, 16, 38, "Identifier 'y' is not a legal target of a substitution. \nA legal target must be a declared CONSTANT or VARIABLE in the module being instantiated. \n(Also, check for warnings about multiple declarations of this same identifier.)"},
	}, warnings: []unitSANY{}},
}

func diagnosticUnitFixture(t *testing.T, id, name, sum string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "diagnostic-unit", id, name)
	if name == "TLAPS.tla" {
		path = filepath.Join("testdata", "diagnostic-unit", name)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := sha256.Sum256(b); hex.EncodeToString(got[:]) != sum {
		t.Fatalf("%s: sha256 %x, want %s", path, got, sum)
	}
	return b
}

// firstHeaderLine is the one-based root line where MonolithSpecExtractor
// starts copying name (MonolithSpecExtractor.java:93-95). Tests use it only
// to cross-check that Span stays in root bytes.
func firstHeaderLine(t *testing.T, text []byte, name string) int {
	t.Helper()
	header := regexp.MustCompile(`^-{4,}[\t\x0B\f ]*MODULE[\t\x0B\f ]+` + regexp.QuoteMeta(name) + `[\t\x0B\f ]*-{3,}$`)
	for i, line := range strings.Split(string(text), "\n") {
		if header.MatchString(line) {
			return i + 1
		}
	}
	t.Fatalf("no header for %s", name)
	return 0
}

// projectUnit renders a diagnostic as SANY's reporting module and start:
// Unit when set, otherwise the root module in root numbering. The roots
// have no tabs before a reported column, so UTF16Column+1 is SANY's column.
func projectUnit(d Diagnostic, rootModule string) unitSANY {
	if d.Unit != nil {
		return unitSANY{module: d.Unit.Module, l1: d.Unit.Line, c1: d.Unit.Column, message: d.Message}
	}
	return unitSANY{module: rootModule, l1: d.Span.Start.Line + 1, c1: d.Span.Start.UTF16Column + 1, message: d.Message}
}

func TestDiagnosticUnitCorpus(t *testing.T) {
	for _, c := range diagnosticUnitCases {
		t.Run(c.id, func(t *testing.T) {
			sources := map[string][]byte{}
			for name, sum := range c.files {
				sources[name] = diagnosticUnitFixture(t, c.id, name, sum)
			}
			text := sources[c.root]
			resolve := func(_ context.Context, name string) (Source, error) {
				if b, ok := sources[name+".tla"]; ok && name+".tla" != c.root {
					return Source{Path: name + ".tla", Text: b, Origin: OriginWorkspace}, nil
				}
				return Source{}, ErrModuleNotFound
			}
			r := checkText(t, "", c.root, string(text), resolve)
			if r.Completion != CompletionComplete {
				t.Fatalf("completion %v: %+v", r.Completion, r.Diagnostics)
			}
			rootModule := strings.TrimSuffix(c.root, ".tla")
			var errs, warns []Diagnostic
			for _, d := range r.Diagnostics {
				switch d.Severity {
				case SeverityError:
					errs = append(errs, d)
				case SeverityWarning:
					warns = append(warns, d)
				default:
					t.Fatalf("unexpected diagnostic %+v", d)
				}
			}
			units := 0
			for _, list := range []struct {
				name string
				got  []Diagnostic
				want []unitSANY
			}{{"error", errs, c.errors}, {"warning", warns, c.warnings}} {
				if len(list.got) != len(list.want) {
					t.Fatalf("%d %ss, want %d: %+v", len(list.got), list.name, len(list.want), list.got)
				}
				for i, d := range list.got {
					want := list.want[i]
					want.l2, want.c2 = 0, 0
					if got := projectUnit(d, rootModule); got != want {
						t.Errorf("%s %d: %+v (unit %+v), want %+v", list.name, i, got, d.Unit, want)
					}
					if d.Path != c.root {
						t.Errorf("%s %d: path %q", list.name, i, d.Path)
					}
					if d.Unit == nil {
						continue
					}
					units++
					// Span stays in root bytes: the extracted line maps to
					// the first header's line plus Unit.Line - 1.
					line := firstHeaderLine(t, text, d.Unit.Module) + d.Unit.Line - 1
					if d.Span.Start.Line+1 != line || d.Span.Start.UTF16Column+1 != d.Unit.Column {
						t.Errorf("%s %d: span %+v, unit %+v, header line + unit line %d", list.name, i, d.Span, *d.Unit, line)
					}
				}
			}
			want := 0
			for _, e := range append(append([]unitSANY(nil), c.errors...), c.warnings...) {
				if e.module != rootModule {
					want++
				}
			}
			if units != want || want == 0 {
				t.Fatalf("%d unit-bearing records, SANY reports %d outside the root", units, want)
			}
		})
	}
}

func unitDiagnostics(r Result, message string) []Diagnostic {
	var out []Diagnostic
	for _, d := range r.Diagnostics {
		if d.Message == message {
			out = append(out, d)
		}
	}
	return out
}

// Two extracted units each report in their own numbering; the root's copies
// of the same nested modules report the same records with Unit nil, at the
// same Path and Span.
func TestDiagnosticUnitRootAndExtractedCopies(t *testing.T) {
	text := "---- MODULE Root ----\nEXTENDS A, B\n" +
		"---- MODULE A ----\n\nABad == xa\n====\n" +
		"\n---- MODULE B ----\nBBad ==\n  xb\n====\n" +
		"RootBad == xr\n====\n"
	r := checkText(t, "", "dir/Root.tla", text, noModules)
	if r.Completion != CompletionComplete {
		t.Fatalf("completion %v: %+v", r.Completion, r.Diagnostics)
	}
	for _, c := range []struct {
		message, module string
		line, column    int
		rootLine        int // zero-based root line of the shared span
	}{
		{"Unknown operator: `xa'.", "A", 3, 9, 4},
		{"Unknown operator: `xb'.", "B", 3, 3, 9},
	} {
		ds := unitDiagnostics(r, c.message)
		if len(ds) != 2 || ds[0].Unit == nil || ds[1].Unit != nil {
			t.Fatalf("%s: %+v", c.message, ds)
		}
		if got := *ds[0].Unit; got != (DiagnosticUnit{Module: c.module, Line: c.line, Column: c.column}) {
			t.Fatalf("%s: unit %+v", c.message, got)
		}
		if ds[0].Path != "dir/Root.tla" || ds[1].Path != ds[0].Path || ds[0].Span != ds[1].Span || ds[0].Span.Start.Line != c.rootLine {
			t.Fatalf("%s: physical coordinates %+v / %+v", c.message, ds[0], ds[1])
		}
	}
	for _, d := range r.Diagnostics {
		if d.Unit != nil && d.Unit.Module != "A" && d.Unit.Module != "B" {
			t.Fatalf("unit outside the extracted modules: %+v", d)
		}
		if strings.Contains(d.Message, "xr") && d.Unit != nil {
			t.Fatalf("root record with unit: %+v", d)
		}
	}
	if ds := unitDiagnostics(r, "Unknown operator: `xr'."); len(ds) != 1 || ds[0].Unit != nil {
		t.Fatalf("root record %+v", ds)
	}
}

// A module declared twice is extracted from its first header only: every
// unit-bearing record comes from the first copy and is numbered from its
// header, while the second copy reports only in root numbering.
func TestDiagnosticUnitDuplicateNameFirstExtraction(t *testing.T) {
	text := "---- MODULE Root ----\nEXTENDS A\n" +
		"---- MODULE A ----\nFirst == x1\n====\n" +
		"---- MODULE A ----\n\nSecond == x2\n====\n" +
		"====\n"
	r := checkText(t, "", "Root.tla", text, noModules)
	if r.Completion != CompletionComplete {
		t.Fatalf("completion %v: %+v", r.Completion, r.Diagnostics)
	}
	first := unitDiagnostics(r, "Unknown operator: `x1'.")
	if len(first) != 2 || first[0].Unit == nil || *first[0].Unit != (DiagnosticUnit{Module: "A", Line: 2, Column: 10}) || first[1].Unit != nil || first[0].Span != first[1].Span || first[0].Span.Start.Line != 3 {
		t.Fatalf("first copy %+v", first)
	}
	second := unitDiagnostics(r, "Unknown operator: `x2'.")
	if len(second) != 1 || second[0].Unit != nil || second[0].Span.Start.Line != 7 {
		t.Fatalf("second copy %+v", second)
	}
	for _, d := range r.Diagnostics {
		if d.Unit != nil && d.Span.Start.Line > 4 {
			t.Fatalf("unit record outside the first copy: %+v", d)
		}
	}
}

// Unit.Column is SANY's column: UTF-16 units with JavaCC tab stops, unlike
// Position's columns, where a tab is one unit.
func TestDiagnosticUnitUTF16AndTabColumn(t *testing.T) {
	text := "---- MODULE Root ----\nEXTENDS A\n\n---- MODULE A ----\n\nBad ==\t(* \U0001F600 *) yy\n====\n====\n"
	r := checkText(t, "", "Root.tla", text, noModules)
	ds := unitDiagnostics(r, "Unknown operator: `yy'.")
	if r.Completion != CompletionComplete || len(ds) != 2 {
		t.Fatalf("%v %+v", r.Completion, r.Diagnostics)
	}
	// "Bad ==" is columns 1-6; the tab ends at column 8, "(* " is 9-11, the
	// supplementary scalar is 12-13 and " *) " 14-17.
	if ds[0].Unit == nil || *ds[0].Unit != (DiagnosticUnit{Module: "A", Line: 3, Column: 18}) || ds[1].Unit != nil {
		t.Fatalf("%+v", ds)
	}
	start := ds[0].Span.Start
	if start.Line != 5 || start.Column != 15 || start.UTF16Column != 16 || ds[0].Span != ds[1].Span || string([]byte(text)[start.ByteOffset:ds[0].Span.End.ByteOffset]) != "yy" {
		t.Fatalf("span %+v", ds[0].Span)
	}
}

// Unit is nil for an ordinary physical document, here the resolver's source
// for the same module, and for the root.
func TestDiagnosticUnitNilForPhysicalDocuments(t *testing.T) {
	text := "---- MODULE Root ----\nEXTENDS A\nRootBad == xr\n---- MODULE A ----\nABad == xa\n====\n====\n"
	resolve := func(_ context.Context, name string) (Source, error) {
		if name == "A" {
			return Source{Path: "lib/A.tla", Text: []byte("---- MODULE A ----\nABad == xa\n====\n"), Origin: OriginWorkspace}, nil
		}
		return Source{}, ErrModuleNotFound
	}
	r := checkText(t, "", "Root.tla", text, resolve)
	if r.Completion != CompletionComplete || len(r.Diagnostics) == 0 {
		t.Fatalf("%v %+v", r.Completion, r.Diagnostics)
	}
	paths := map[string]bool{}
	for _, d := range r.Diagnostics {
		if d.Unit != nil {
			t.Fatalf("unit on a physical document: %+v", d)
		}
		paths[d.Path] = true
	}
	if !paths["lib/A.tla"] || !paths["Root.tla"] {
		t.Fatalf("paths %v: %+v", paths, r.Diagnostics)
	}
}

// Diagnostics without a source position keep Unit nil even when they belong
// to an extracted unit: here the never-set "Unexpected token found." records
// of the D-c412e23757be construct moved into module A. A and P follow the
// root's end, so the root has no copy and every record of A is the extracted
// unit's.
func TestDiagnosticUnitNilForUnlocated(t *testing.T) {
	text := "---- MODULE Root ----\nEXTENDS A\n====\n" +
		"---- MODULE A ----\nEXTENDS P\nUSE x, MODULE M, 1 + 3HHIDE DEFS M^DULE\\ Lt!1!<< = x + 5\n       /\\ Nest!outer(2, 3)!i\n====\n" +
		"---- MODULE P ----\nx == 4\n====\n"
	r := checkText(t, "", "Root.tla", text, noModules)
	if r.Completion != CompletionComplete {
		t.Fatalf("completion %v: %+v", r.Completion, r.Diagnostics)
	}
	unlocated, located := 0, 0
	for _, d := range r.Diagnostics {
		if d.Span == (Span{}) {
			unlocated++
			if d.Unit != nil {
				t.Fatalf("unit on an unlocated diagnostic: %+v", d)
			}
			continue
		}
		if d.Unit != nil {
			located++
			if d.Unit.Module != "A" || d.Unit.Line < 1 || d.Unit.Column < 1 {
				t.Fatalf("unit %+v", d)
			}
		}
	}
	if unlocated == 0 || located == 0 {
		t.Fatalf("%d unlocated, %d unit-bearing: %+v", unlocated, located, r.Diagnostics)
	}
}

// Every Unit is call-owned: concurrent Checks of one input give equal,
// unaliased values, and mutating one result changes no other.
func TestDiagnosticUnitOwnership(t *testing.T) {
	c := diagnosticUnitCases[0]
	text := diagnosticUnitFixture(t, c.id, c.root, c.files[c.root])
	library := diagnosticUnitFixture(t, c.id, "TLAPS.tla", c.files["TLAPS.tla"])
	resolve := func(_ context.Context, name string) (Source, error) {
		if name == "TLAPS" {
			return Source{Path: "TLAPS.tla", Text: library, Origin: OriginWorkspace}, nil
		}
		return Source{}, ErrModuleNotFound
	}
	const n = 4
	results := make([]Result, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = Check(context.Background(), Request{Path: c.root, Text: text, Resolve: resolve})
		}(i)
	}
	wg.Wait()
	seen := map[*DiagnosticUnit]bool{}
	for i, r := range results {
		if errs[i] != nil || len(r.Diagnostics) != len(results[0].Diagnostics) {
			t.Fatalf("run %d: %v %+v", i, errs[i], r.Diagnostics)
		}
		units := 0
		for j, d := range r.Diagnostics {
			want := results[0].Diagnostics[j]
			if (d.Unit == nil) != (want.Unit == nil) || d.Unit != nil && *d.Unit != *want.Unit {
				t.Fatalf("run %d diagnostic %d: %+v, want %+v", i, j, d.Unit, want.Unit)
			}
			if d.Unit == nil {
				continue
			}
			units++
			if seen[d.Unit] {
				t.Fatalf("run %d diagnostic %d: aliased unit", i, j)
			}
			seen[d.Unit] = true
		}
		if units != 2 {
			t.Fatalf("run %d: %d units", i, units)
		}
	}
	var snapshot []DiagnosticUnit
	for _, d := range results[1].Diagnostics {
		if d.Unit != nil {
			snapshot = append(snapshot, *d.Unit)
		}
	}
	for _, d := range results[0].Diagnostics {
		if d.Unit != nil {
			d.Unit.Module, d.Unit.Line, d.Unit.Column = "mutated", -1, -1
		}
	}
	again, err := Check(context.Background(), Request{Path: c.root, Text: text, Resolve: resolve})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []Result{results[1], again} {
		k := 0
		for _, d := range r.Diagnostics {
			if d.Unit == nil {
				continue
			}
			if k >= len(snapshot) || *d.Unit != snapshot[k] {
				t.Fatalf("unit changed by another result's mutation: %+v", d.Unit)
			}
			k++
		}
		if k != len(snapshot) {
			t.Fatalf("%d units, want %d", k, len(snapshot))
		}
	}
}
