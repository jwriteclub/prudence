// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package prudence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Extra label parameters under the approved list-order exception. formalParamsEqual
// (Generator.java:7092-7097) iterates the residual HashSet only to append
// names to "Label <name> declares extra parameter(s)  <names>"; retVal is
// already false and generateLabel discards it (6933). When the bounded Java
// cursor cannot give Java's order and every residual name is distinct,
// prudence prints the full list in UTF-16 lexical order. Only that list is
// compared as a multiset; every other byte, position, count and the order of
// all errors and warnings must be SANY's.

// labelExtraSANY is one SANY entry: module, "line l1, col c1 to line l2,
// col c2" (one-based, inclusive end) and the exact message bytes.
type labelExtraSANY struct {
	module         string
	l1, c1, l2, c2 int
	message        string
}

// labelExtraCases are the 12 recorded sany-conformance cases that stopped at
// generate-label, verbatim under testdata/label-extra-order (provenance in
// PROVENANCE). errors and warnings are SANY's in printed order (manifest
// stdout). normalised marks the case whose Java list order differs from the
// documented fallback order, so it matches only order-normalised.
var labelExtraCases = []struct {
	set, id, root    string
	files            map[string]string // file name to SHA-256
	errors, warnings []labelExtraSANY
	normalised       bool
}{
	{set: "deep-cuts/model-agents", id: "E-d0e6bde23d7c", root: "Test.tla", files: map[string]string{
		"Test.tla": "5519e67a2f5f1667385e97862b6b7ca617f492da13b615ba5c73f3b889aef4fd",
	}, errors: []labelExtraSANY{
		{"Test", 3, 17, 3, 17, "Unknown operator: `a'."},
		{"Test", 3, 21, 3, 21, "Unknown operator: `b'."},
		{"Test", 3, 8, 3, 8, "Illegal parameter a of label `L'."},
		{"Test", 3, 11, 3, 11, "Illegal parameter b of label `L'."},
		{"Test", 3, 6, 3, 21, "Label L declares extra parameter(s)  a  b  "},
	}},
	{set: "fuzz-jazzer", id: "D-07589fc5697b", root: "SubrefLL_test.tla", files: map[string]string{
		"SubrefLL_test.tla": "7546dfbfe815e574de142b7ca7faac3eefc358d9365e70e797f092e097ae9de7",
		"TLAPS.tla":         "9afe54984062748a0568966434cc0945d682f8cd89fdbc38f73b5579751b0c55",
	}, errors: []labelExtraSANY{
		{"SubrefLL_test", 19, 42, 20, 68, "Operator ++ already defined or declared."},
		{"SubrefLL_test", 19, 23, 23, 65, "Label lab declares extra parameter(s)  x  y  z  "},
		{"SubrefLL_test", 25, 17, 25, 17, "Unknown operator: `u'."},
		{"SubrefLL_test", 37, 31, 37, 36, "Cannot find label `label2'."},
		{"SubrefLL_test", 43, 12, 43, 16, "Unknown operator: `Def13'."},
		{"SubrefLL_test", 44, 12, 44, 16, "Unknown operator: `Def13'."},
		{"SubrefLL_test", 45, 12, 45, 16, "Unknown operator: `Def13'."},
		{"SubrefLL_test", 46, 12, 46, 16, "Unknown operator: `Def13'."},
		{"SubrefLL_test", 51, 26, 51, 27, "Could not find declaration or definition of symbol '++'."},
		{"SubrefLL_test", 51, 24, 51, 29, "Couldn't resolve infix operator symbol `++'."},
		{"SubrefLL_test", 51, 14, 51, 29, "Label label2 must contain formal parameter `x'."},
		{"SubrefLL_test", 51, 14, 51, 29, "Label label2 must contain formal parameter `y'."},
		{"SubrefLL_test", 51, 14, 51, 29, "Label label2 must contain formal parameter `z'."},
		{"SubrefLL_test", 53, 1, 53, 17, "Operator Foo already defined or declared."},
		{"SubrefLL_test", 53, 17, 53, 17, "Unknown operator: `u'."},
		{"SubrefLL_test", 65, 31, 65, 36, "Cannot find label `label2'."},
		{"SubrefLL_test", 69, 1, 69, 42, "Operator Op3 already defined or declared."},
		{"SubrefLL_test", 79, 9, 79, 12, "Unknown operator: `Def7'."},
		{"SubrefLL_test", 82, 9, 82, 12, "Unknown operator: `Def7'."},
		{"SubrefLL_test", 88, 23, 92, 65, "Label not in definition or proof step."},
		{"SubrefLL_test", 94, 1, 94, 17, "Operator Foo already defined or declared."},
		{"SubrefLL_test", 94, 17, 94, 17, "Unknown operator: `u'."},
		{"SubrefLL_test", 106, 31, 106, 36, "Cannot find label `label2'."},
		{"SubrefLL_test", 110, 1, 110, 42, "Operator Op3 already defined or declared."},
		{"SubrefLL_test", 112, 12, 112, 16, "Unknown operator: `Def13'."},
		{"SubrefLL_test", 113, 12, 113, 16, "Unknown operator: `Def13'."},
		{"SubrefLL_test", 114, 12, 114, 16, "Unknown operator: `Def13'."},
		{"SubrefLL_test", 115, 12, 115, 16, "Unknown operator: `Def13'."},
		{"SubrefLL_test", 115, 35, 115, 37, "Unknown operator: `Slt'."},
	}, warnings: []labelExtraSANY{
		{"SubrefLL_test", 19, 42, 20, 68, "Multiple declarations or definitions for symbol ++.  \nThis duplicates the one at line 14, col 42 to line 15, col 68 of module SubrefLL_test."},
		{"SubrefLL_test", 53, 1, 53, 17, "Multiple declarations or definitions for symbol Foo.  \nThis duplicates the one at line 25, col 1 to line 25, col 17 of module SubrefLL_test."},
		{"SubrefLL_test", 57, 1, 58, 33, "Multiple declarations or definitions for symbol Thm.  \nThis duplicates the one at line 29, col 1 to line 30, col 33 of module SubrefLL_test."},
		{"SubrefLL_test", 60, 1, 60, 28, "Multiple declarations or definitions for symbol Thm2.  \nThis duplicates the one at line 32, col 1 to line 32, col 28 of module SubrefLL_test."},
		{"SubrefLL_test", 69, 1, 69, 42, "Multiple declarations or definitions for symbol Op3.  \nThis duplicates the one at line 41, col 1 to line 41, col 42 of module SubrefLL_test."},
		{"SubrefLL_test", 94, 1, 94, 17, "Multiple declarations or definitions for symbol Foo.  \nThis duplicates the one at line 25, col 1 to line 25, col 17 of module SubrefLL_test."},
		{"SubrefLL_test", 98, 1, 99, 33, "Multiple declarations or definitions for symbol Thm.  \nThis duplicates the one at line 29, col 1 to line 30, col 33 of module SubrefLL_test."},
		{"SubrefLL_test", 101, 1, 101, 28, "Multiple declarations or definitions for symbol Thm2.  \nThis duplicates the one at line 32, col 1 to line 32, col 28 of module SubrefLL_test."},
		{"SubrefLL_test", 110, 1, 110, 42, "Multiple declarations or definitions for symbol Op3.  \nThis duplicates the one at line 41, col 1 to line 41, col 42 of module SubrefLL_test."},
	}},
	{set: "fuzz-jazzer", id: "D-15c006f52fa6", root: "SX1.tla", files: map[string]string{
		"SX1.tla": "cd6a46c8fc8cfa1883c720737afb72ac1def69dc859b34b5375d081effa09679",
	}, errors: []labelExtraSANY{
		{"SX1", 2, 15, 2, 16, "Could not find declaration or definition of symbol '..'."},
		{"SX1", 2, 14, 2, 17, "Couldn't resolve infix operator symbol `..'."},
		{"SX1", 2, 25, 2, 25, "Could not find declaration or definition of symbol '+'."},
		{"SX1", 2, 23, 2, 27, "Couldn't resolve infix operator symbol `+'."},
		{"SX1", 3, 48, 3, 48, "Could not find declaration or definition of symbol '>'."},
		{"SX1", 3, 42, 3, 50, "Couldn't resolve infix operator symbol `>'."},
		{"SX1", 3, 33, 3, 33, "Illegal parameter i of label `ilab'."},
		{"SX1", 3, 36, 3, 36, "Illegal parameter j of label `ilab'."},
		{"SX1", 3, 11, 3, 50, "Label ilab declares extra parameter(s)  i  j  "},
		{"SX1", 4, 23, 4, 24, "Could not find declaration or definition of symbol '..'."},
		{"SX1", 4, 22, 4, 25, "Couldn't resolve infix operator symbol `..'."},
		{"SX1", 4, 10, 4, 10, "Could not find declaration or definition of symbol '*'."},
		{"SX1", 4, 8, 4, 12, "Couldn't resolve infix operator symbol `*'."},
		{"SX1", 5, 16, 5, 17, "Could not find declaration or definition of symbol '..'."},
		{"SX1", 5, 15, 5, 18, "Couldn't resolve infix operator symbol `..'."},
		{"SX1", 5, 24, 5, 24, "Could not find declaration or definition of symbol '>'."},
		{"SX1", 5, 22, 5, 26, "Couldn't resolve infix operator symbol `>'."},
		{"SX1", 6, 1, 6, 28, "Operator Fn already defined or declared."},
		{"SX1", 6, 15, 6, 16, "Could not find declaration or definition of symbol '..'."},
		{"SX1", 6, 14, 6, 17, "Couldn't resolve infix operator symbol `..'."},
		{"SX1", 6, 25, 6, 25, "Could not find declaration or definition of symbol '+'."},
		{"SX1", 6, 23, 6, 27, "Couldn't resolve infix operator symbol `+'."},
		{"SX1", 7, 21, 7, 22, "Could not find declaration or definition of symbol '..'."},
		{"SX1", 7, 20, 7, 23, "Couldn't resolve infix operator symbol `..'."},
		{"SX1", 7, 29, 7, 29, "Could not find declaration or definition of symbol '>'."},
		{"SX1", 7, 27, 7, 31, "Couldn't resolve infix operator symbol `>'."},
		{"SX1", 8, 13, 8, 13, "Could not find declaration or definition of symbol '>'."},
		{"SX1", 8, 11, 8, 15, "Couldn't resolve infix operator symbol `>'."},
		{"SX1", 8, 22, 8, 22, "Unknown operator: `x'."},
		{"SX1", 10, 19, 10, 20, "Could not find declaration or definition of symbol '..'."},
		{"SX1", 10, 18, 10, 21, "Couldn't resolve infix operator symbol `..'."},
		{"SX1", 10, 25, 10, 25, "The operator \\ requires 2 arguments."},
	}, warnings: []labelExtraSANY{
		{"SX1", 6, 1, 6, 28, "Multiple declarations or definitions for symbol Fn.  \nThis duplicates the one at line 2, col 1 to line 3, col 50 of module SX1."},
	}},
	{normalised: true, set: "fuzz-jazzer", id: "D-54cb9f0ad55d", root: "SubrefLL_test.tla", files: map[string]string{
		"SubrefLL_test.tla": "4542bb39977f0aa7d4aa28cd48b530789a72c81d2f614e3b6ebd9449701b8065",
		"TLAPS.tla":         "9afe54984062748a0568966434cc0945d682f8cd89fdbc38f73b5579751b0c55",
	}, errors: []labelExtraSANY{
		{"SubrefLL_test", 3, 14, 3, 14, "Unknown operator: `C'."},
		{"SubrefLL_test", 4, 1, 4, 3, "Empty BY, USE, or HIDE"},
		{"SubrefLL_test", 4, 1, 4, 3, "Empty USE or HIDE statement."},
		{"SubrefLL_test", 4, 5, 11, 16, "Operator c4 already defined or declared."},
		{"SubrefLL_test", 8, 43, 8, 45, "Unknown operator: `Arg'."},
		{"SubrefLL_test", 8, 56, 8, 56, "Unknown operator: `x'."},
		{"SubrefLL_test", 8, 62, 8, 62, "Unknown operator: `y'."},
		{"SubrefLL_test", 8, 68, 8, 68, "Unknown operator: `z'."},
		{"SubrefLL_test", 9, 43, 9, 45, "Unknown operator: `Arg'."},
		{"SubrefLL_test", 9, 52, 9, 54, "Unknown operator: `Arg'."},
		{"SubrefLL_test", 9, 61, 9, 61, "Unknown operator: `C'."},
		{"SubrefLL_test", 10, 47, 10, 47, "Unknown operator: `x'."},
		{"SubrefLL_test", 10, 54, 10, 54, "Unknown operator: `y'."},
		{"SubrefLL_test", 10, 60, 10, 60, "Unknown operator: `z'."},
		{"SubrefLL_test", 11, 15, 11, 16, "Unknown operator: `US'."},
		{"SubrefLL_test", 7, 27, 7, 27, "Illegal parameter x of label `lab'."},
		{"SubrefLL_test", 7, 29, 7, 29, "Illegal parameter i of label `lab'."},
		{"SubrefLL_test", 7, 31, 7, 31, "Illegal parameter z of label `lab'."},
		{"SubrefLL_test", 7, 23, 11, 16, "Label lab declares extra parameter(s)  x  i  z  "},
		{"SubrefLL_test", 4, 5, 11, 16, "Multiply-defined symbol 'c4': this definition or declaration conflicts \nwith the one at line 3, col 1 to line 3, col 18 of module SubrefLL_test."},
		{"SubrefLL_test", 13, 9, 13, 11, "Unknown operator: `COp'."},
		{"SubrefLL_test", 13, 47, 13, 58, "Unknown operator: `SalEFUNCTION'."},
	}},
	{set: "fuzz-jazzer", id: "D-6299d741cdd4", root: "SubrefLL_test.tla", files: map[string]string{
		"SubrefLL_test.tla": "6a946a8d1de007d1026e8260367e5e3934dcc600f0e09240606256554e9e04bf",
		"TLAPS.tla":         "9afe54984062748a0568966434cc0945d682f8cd89fdbc38f73b5579751b0c55",
	}, errors: []labelExtraSANY{
		{"SubrefLL_test", 4, 7, 4, 7, "Unknown operator: `s'."},
		{"SubrefLL_test", 4, 11, 4, 11, "Unknown operator: `s'."},
		{"SubrefLL_test", 5, 9, 5, 12, "Unknown operator: `Salt'."},
		{"SubrefLL_test", 5, 9, 5, 12, "DEF clause entry should describe a defined operator."},
		{"SubrefLL_test", 5, 1, 5, 12, "Empty USE or HIDE statement."},
		{"SubrefLL_test", 11, 61, 11, 61, "Unknown operator: `C'."},
		{"SubrefLL_test", 14, 42, 15, 68, "Operator ++ already defined or declared."},
		{"SubrefLL_test", 16, 61, 16, 61, "Unknown operator: `C'."},
		{"SubrefLL_test", 18, 64, 18, 64, "Unknown operator: `C'."},
		{"SubrefLL_test", 14, 23, 18, 65, "Label lab declares extra parameter(s)  x  y  z  "},
		{"SubrefLL_test", 20, 9, 20, 9, "Unknown operator: `T'."},
		{"SubrefLL_test", 20, 20, 20, 20, "Unknown operator: `z'."},
		{"SubrefLL_test", 21, 12, 21, 16, "Unknown operator: `Def23'."},
		{"SubrefLL_test", 22, 12, 22, 16, "Unknown operator: `Def23'."},
		{"SubrefLL_test", 22, 37, 22, 40, "Unknown operator: `Salt'."},
	}, warnings: []labelExtraSANY{
		{"SubrefLL_test", 14, 42, 15, 68, "Multiple declarations or definitions for symbol ++.  \nThis duplicates the one at line 9, col 42 to line 10, col 68 of module SubrefLL_test."},
	}},
	{set: "fuzz-jazzer", id: "D-75d805746643", root: "SubrefLL_test.tla", files: map[string]string{
		"SubrefLL_test.tla": "c8774ee0df969734a96cb65fa1950e23498e0783d22914280141487d1cd0d34d",
		"TLAPS.tla":         "9afe54984062748a0568966434cc0945d682f8cd89fdbc38f73b5579751b0c55",
	}, errors: []labelExtraSANY{
		{"SubrefLL_test", 12, 61, 12, 61, "Unknown operator: `C'."},
		{"SubrefLL_test", 15, 42, 16, 68, "Operator ++ already defined or declared."},
		{"SubrefLL_test", 17, 61, 17, 61, "Unknown operator: `C'."},
		{"SubrefLL_test", 19, 64, 19, 64, "Unknown operator: `C'."},
		{"SubrefLL_test", 15, 23, 19, 65, "Label lab declares extra parameter(s)  x  y  z  "},
		{"SubrefLL_test", 21, 9, 21, 9, "Unknown operator: `T'."},
		{"SubrefLL_test", 21, 20, 21, 20, "Unknown operator: `z'."},
		{"SubrefLL_test", 22, 12, 22, 16, "Unknown operator: `Def23'."},
		{"SubrefLL_test", 23, 12, 23, 16, "Unknown operator: `Def23'."},
	}, warnings: []labelExtraSANY{
		{"SubrefLL_test", 15, 42, 16, 68, "Multiple declarations or definitions for symbol ++.  \nThis duplicates the one at line 10, col 42 to line 11, col 68 of module SubrefLL_test."},
	}},
	{set: "fuzz-jazzer", id: "D-b3669c6d3c74", root: "SubrefLL_test.tla", files: map[string]string{
		"SubrefLL_test.tla": "13f11c2a96b383947a650dde0a8b7e5b8a908886ca668d1762ca39bb49aa914c",
		"TLAPS.tla":         "9afe54984062748a0568966434cc0945d682f8cd89fdbc38f73b5579751b0c55",
	}, errors: []labelExtraSANY{
		{"SubrefLL_test", 19, 42, 20, 68, "Operator ++ already defined or declared."},
		{"SubrefLL_test", 19, 23, 23, 65, "Label lab declares extra parameter(s)  x  y  z  "},
		{"SubrefLL_test", 25, 17, 25, 17, "Unknown operator: `u'."},
		{"SubrefLL_test", 37, 31, 37, 36, "Cannot find label `label2'."},
		{"SubrefLL_test", 43, 12, 43, 16, "Unknown operator: `Def13'."},
		{"SubrefLL_test", 44, 12, 44, 16, "Unknown operator: `Def13'."},
		{"SubrefLL_test", 45, 12, 45, 16, "Unknown operator: `Def13'."},
		{"SubrefLL_test", 46, 12, 46, 16, "Unknown operator: `Def13'."},
		{"SubrefLL_test", 51, 26, 51, 27, "Could not find declaration or definition of symbol '++'."},
		{"SubrefLL_test", 51, 24, 51, 29, "Couldn't resolve infix operator symbol `++'."},
		{"SubrefLL_test", 51, 14, 51, 29, "Label label2 must contain formal parameter `x'."},
		{"SubrefLL_test", 51, 14, 51, 29, "Label label2 must contain formal parameter `y'."},
		{"SubrefLL_test", 51, 14, 51, 29, "Label label2 must contain formal parameter `z'."},
		{"SubrefLL_test", 53, 1, 53, 17, "Operator Foo already defined or declared."},
		{"SubrefLL_test", 53, 17, 53, 17, "Unknown operator: `u'."},
		{"SubrefLL_test", 65, 31, 65, 36, "Cannot find label `label2'."},
		{"SubrefLL_test", 69, 1, 69, 42, "Operator Op3 already defined or declared."},
		{"SubrefLL_test", 85, 9, 85, 12, "Unknown operator: `Def2'."},
		{"SubrefLL_test", 88, 9, 88, 12, "Unknown operator: `Def2'."},
		{"SubrefLL_test", 161, 1, 163, 32, "Operator Def23 already defined or declared."},
		{"SubrefLL_test", 165, 26, 165, 26, "An unexpected null node specified by 'argument selector 2'.\nThis is probably due to a previous error."},
	}, warnings: []labelExtraSANY{
		{"SubrefLL_test", 19, 42, 20, 68, "Multiple declarations or definitions for symbol ++.  \nThis duplicates the one at line 14, col 42 to line 15, col 68 of module SubrefLL_test."},
		{"SubrefLL_test", 53, 1, 53, 17, "Multiple declarations or definitions for symbol Foo.  \nThis duplicates the one at line 25, col 1 to line 25, col 17 of module SubrefLL_test."},
		{"SubrefLL_test", 57, 1, 58, 33, "Multiple declarations or definitions for symbol Thm.  \nThis duplicates the one at line 29, col 1 to line 30, col 33 of module SubrefLL_test."},
		{"SubrefLL_test", 60, 1, 60, 28, "Multiple declarations or definitions for symbol Thm2.  \nThis duplicates the one at line 32, col 1 to line 32, col 28 of module SubrefLL_test."},
		{"SubrefLL_test", 69, 1, 69, 42, "Multiple declarations or definitions for symbol Op3.  \nThis duplicates the one at line 41, col 1 to line 41, col 42 of module SubrefLL_test."},
		{"SubrefLL_test", 161, 1, 163, 32, "Multiple declarations or definitions for symbol Def23.  \nThis duplicates the one at line 49, col 1 to line 51, col 30 of module SubrefLL_test."},
	}},
	{set: "fuzz-jazzer", id: "D-b70169b8aa32", root: "SubrefLL_test.tla", files: map[string]string{
		"SubrefLL_test.tla": "746cb8249f1bdf82de03d7a6495542b58d9191f984e94d51d2d8a039a680f7bd",
		"TLAPS.tla":         "9afe54984062748a0568966434cc0945d682f8cd89fdbc38f73b5579751b0c55",
	}, errors: []labelExtraSANY{
		{"SubrefLL_test", 13, 23, 13, 23, "Multiply-defined symbol 'y': this definition or declaration conflicts \nwith the one at line 13, col 20 to line 13, col 20 of module SubrefLL_test."},
		{"SubrefLL_test", 15, 68, 15, 68, "Unknown operator: `z'."},
		{"SubrefLL_test", 17, 60, 17, 60, "Unknown operator: `z'."},
		{"SubrefLL_test", 14, 27, 14, 29, "Illegal parameter x2y of label `lab'."},
		{"SubrefLL_test", 14, 31, 14, 31, "Illegal parameter z of label `lab'."},
		{"SubrefLL_test", 14, 23, 18, 65, "Label lab must contain formal parameter `x'."},
		{"SubrefLL_test", 14, 23, 18, 65, "Label lab must contain formal parameter `y'."},
		{"SubrefLL_test", 14, 23, 18, 65, "Label lab declares extra parameter(s)  x2y  z  "},
		{"SubrefLL_test", 20, 9, 20, 11, "Unknown operator: `Foo'."},
		{"SubrefLL_test", 21, 12, 21, 14, "Unknown operator: `Foo'."},
		{"SubrefLL_test", 21, 12, 21, 14, "DEF clause entry should describe a defined operator."},
		{"SubrefLL_test", 23, 12, 23, 14, "Unknown operator: `Foo'."},
		{"SubrefLL_test", 23, 20, 23, 22, "Label `lab' used with wrong number of arguments."},
		{"SubrefLL_test", 23, 43, 23, 45, "Unknown operator: `Foo'."},
		{"SubrefLL_test", 23, 51, 23, 53, "Label `lab' used with wrong number of arguments."},
		{"SubrefLL_test", 50, 12, 50, 16, "Unknown operator: `Def24'."},
	}},
	{set: "fuzz-jazzer", id: "D-b9ffa78941e1", root: "SX1.tla", files: map[string]string{
		"SX1.tla": "50f6a8ab13941f014cbda7de14ba0d4c64a60795846e46760102fb1764eface5",
	}, errors: []labelExtraSANY{
		{"SX1", 4, 15, 4, 16, "Could not find declaration or definition of symbol '..'."},
		{"SX1", 4, 14, 4, 17, "Couldn't resolve infix operator symbol `..'."},
		{"SX1", 4, 25, 4, 25, "Could not find declaration or definition of symbol '+'."},
		{"SX1", 4, 23, 4, 27, "Couldn't resolve infix operator symbol `+'."},
		{"SX1", 5, 48, 5, 48, "Could not find declaration or definition of symbol '>'."},
		{"SX1", 5, 42, 5, 50, "Couldn't resolve infix operator symbol `>'."},
		{"SX1", 5, 33, 5, 33, "Illegal parameter i of label `ilab'."},
		{"SX1", 5, 36, 5, 36, "Illegal parameter j of label `ilab'."},
		{"SX1", 5, 11, 5, 50, "Label ilab declares extra parameter(s)  i  j  "},
		{"SX1", 6, 23, 6, 24, "Could not find declaration or definition of symbol '..'."},
		{"SX1", 6, 22, 6, 25, "Couldn't resolve infix operator symbol `..'."},
		{"SX1", 6, 10, 6, 10, "Could not find declaration or definition of symbol '*'."},
		{"SX1", 6, 8, 6, 12, "Couldn't resolve infix operator symbol `*'."},
		{"SX1", 7, 16, 7, 17, "Could not find declaration or definition of symbol '..'."},
		{"SX1", 7, 15, 7, 18, "Couldn't resolve infix operator symbol `..'."},
		{"SX1", 7, 24, 7, 24, "Could not find declaration or definition of symbol '>'."},
		{"SX1", 7, 22, 7, 26, "Couldn't resolve infix operator symbol `>'."},
		{"SX1", 8, 1, 8, 28, "Operator Fn already defined or declared."},
		{"SX1", 8, 15, 8, 16, "Could not find declaration or definition of symbol '..'."},
		{"SX1", 8, 14, 8, 17, "Couldn't resolve infix operator symbol `..'."},
		{"SX1", 8, 25, 8, 25, "Could not find declaration or definition of symbol '+'."},
		{"SX1", 8, 23, 8, 27, "Couldn't resolve infix operator symbol `+'."},
		{"SX1", 9, 21, 9, 22, "Could not find declaration or definition of symbol '..'."},
		{"SX1", 9, 20, 9, 23, "Couldn't resolve infix operator symbol `..'."},
		{"SX1", 9, 29, 9, 29, "Could not find declaration or definition of symbol '>'."},
		{"SX1", 9, 27, 9, 31, "Couldn't resolve infix operator symbol `>'."},
		{"SX1", 10, 13, 10, 13, "Could not find declaration or definition of symbol '>'."},
		{"SX1", 10, 11, 10, 15, "Couldn't resolve infix operator symbol `>'."},
		{"SX1", 10, 22, 10, 22, "Unknown operator: `x'."},
		{"SX1", 12, 19, 12, 20, "Could not find declaration or definition of symbol '..'."},
		{"SX1", 12, 18, 12, 21, "Couldn't resolve infix operator symbol `..'."},
		{"SX1", 12, 35, 12, 36, "Could not find declaration or definition of symbol '..'."},
		{"SX1", 12, 34, 12, 37, "Couldn't resolve infix operator symbol `..'."},
		{"SX1", 12, 56, 12, 76, "Unknown operator: `est!outer!inner'."},
		{"SX1", 13, 11, 13, 32, "Unknown operator: `Nest!outer!inner'."},
	}, warnings: []labelExtraSANY{
		{"SX1", 8, 1, 8, 28, "Multiple declarations or definitions for symbol Fn.  \nThis duplicates the one at line 4, col 1 to line 5, col 50 of module SX1."},
	}},
	{set: "fuzz-jazzer", id: "D-d0e270b834ea", root: "SX1.tla", files: map[string]string{
		"SX1.tla":        "2f2394f37a894faa295adaf86bdf67cdb0b6534d045c9d315cae43a83fa42973",
		"FiniteSets.tla": "eca905df95c16fb12f0f205a38b429c216a11b5c8429d270af044f05600a6380",
	}, errors: []labelExtraSANY{
		{"SX1", 19, 59, 19, 59, "Multiply-defined symbol 'i': this definition or declaration conflicts \nwith the one at line 19, col 12 to line 19, col 12 of module SX1."},
		{"SX1", 19, 71, 19, 71, "Multiply-defined symbol 'j': this definition or declaration conflicts \nwith the one at line 19, col 28 to line 19, col 28 of module SX1."},
		{"SX1", 19, 83, 19, 104, "Label lab must contain formal parameter `i'."},
		{"SX1", 19, 83, 19, 104, "Label lab must contain formal parameter `j'."},
		{"SX1", 19, 83, 19, 104, "Label lab declares extra parameter(s)  i  j  "},
		{"SX1", 20, 1, 20, 26, "Operator Sq already defined or declared."},
		{"SX1", 21, 1, 21, 27, "Operator Fil already defined or declared."},
		{"SX1", 22, 1, 22, 28, "Operator Fn already defined or declared."},
		{"SX1", 23, 1, 23, 31, "Operator Ch already defined or declared."},
		{"SX1", 24, 1, 24, 55, "Operator Cs already defined or declared."},
		{"SX1", 25, 1, 25, 32, "Operator Rc already defined or declared."},
		{"SX1", 26, 1, 26, 29, "Operator Lt already defined or declared."},
		{"SX1", 27, 1, 27, 30, "Operator Ite already defined or declared."},
		{"SX1", 28, 1, 28, 17, "Operator App already defined or declared."},
		{"SX1", 29, 1, 29, 84, "Operator Nest already defined or declared."},
		{"SX1", 39, 1, 46, 40, "Operator Inv already defined or declared."},
		{"SX1", 44, 13, 44, 15, "Label `lab' used with wrong number of arguments."},
		{"SX1", 45, 37, 45, 41, "Cannot find label `inner'."},
		{"SX1", 46, 28, 46, 32, "Cannot find label `inner'."},
	}, warnings: []labelExtraSANY{
		{"SX1", 20, 1, 20, 26, "Multiple declarations or definitions for symbol Sq.  \nThis duplicates the one at line 10, col 1 to line 10, col 26 of module SX1."},
		{"SX1", 21, 1, 21, 27, "Multiple declarations or definitions for symbol Fil.  \nThis duplicates the one at line 11, col 1 to line 11, col 27 of module SX1."},
		{"SX1", 22, 1, 22, 28, "Multiple declarations or definitions for symbol Fn.  \nThis duplicates the one at line 12, col 1 to line 12, col 28 of module SX1."},
		{"SX1", 23, 1, 23, 31, "Multiple declarations or definitions for symbol Ch.  \nThis duplicates the one at line 13, col 1 to line 13, col 31 of module SX1."},
		{"SX1", 24, 1, 24, 55, "Multiple declarations or definitions for symbol Cs.  \nThis duplicates the one at line 14, col 1 to line 14, col 55 of module SX1."},
		{"SX1", 25, 1, 25, 32, "Multiple declarations or definitions for symbol Rc.  \nThis duplicates the one at line 15, col 1 to line 15, col 32 of module SX1."},
		{"SX1", 26, 1, 26, 29, "Multiple declarations or definitions for symbol Lt.  \nThis duplicates the one at line 16, col 1 to line 16, col 29 of module SX1."},
		{"SX1", 27, 1, 27, 30, "Multiple declarations or definitions for symbol Ite.  \nThis duplicates the one at line 17, col 1 to line 17, col 30 of module SX1."},
		{"SX1", 28, 1, 28, 17, "Multiple declarations or definitions for symbol App.  \nThis duplicates the one at line 18, col 1 to line 18, col 17 of module SX1."},
		{"SX1", 29, 1, 29, 84, "Multiple declarations or definitions for symbol Nest.  \nThis duplicates the one at line 19, col 1 to line 19, col 104 of module SX1."},
		{"SX1", 39, 1, 46, 40, "Multiple declarations or definitions for symbol Inv.  \nThis duplicates the one at line 35, col 1 to line 37, col 28 of module SX1."},
	}},
	{set: "fuzz-jazzer", id: "D-dc16bc8a4aee", root: "SX1.tla", files: map[string]string{
		"SX1.tla": "25b51c7a0ae6fac9d1e383d11d28096f34bf1324eda7ef82005d4eea2fe90f0f",
	}, errors: []labelExtraSANY{
		{"SX1", 2, 15, 2, 16, "Could not find declaration or definition of symbol '..'."},
		{"SX1", 2, 14, 2, 17, "Couldn't resolve infix operator symbol `..'."},
		{"SX1", 2, 25, 2, 25, "Could not find declaration or definition of symbol '+'."},
		{"SX1", 2, 23, 2, 27, "Couldn't resolve infix operator symbol `+'."},
		{"SX1", 3, 42, 3, 42, "Unknown operator: `i'."},
		{"SX1", 3, 33, 3, 33, "Illegal parameter i of label `ilab'."},
		{"SX1", 3, 36, 3, 36, "Illegal parameter j of label `ilab'."},
		{"SX1", 3, 11, 3, 42, "Label ilab declares extra parameter(s)  i  j  "},
		{"SX1", 3, 57, 3, 58, "Could not find declaration or definition of symbol '..'."},
		{"SX1", 3, 56, 3, 59, "Couldn't resolve infix operator symbol `..'."},
		{"SX1", 3, 67, 3, 67, "Could not find declaration or definition of symbol '+'."},
		{"SX1", 3, 65, 3, 69, "Couldn't resolve infix operator symbol `+'."},
		{"SX1", 4, 14, 4, 34, "Unknown operator: `est!outer!inner'."},
		{"SX1", 5, 11, 5, 32, "Unknown operator: `Nest!outer!inner'."},
	}},
	{set: "model-one-shot", id: "E-9469ef8c3889", root: "Target.tla", files: map[string]string{
		"Target.tla": "b3fd04a01b5db288b5e03835f5b03e6a25aa0dc0866f3b86f011ea4078bcfe04",
	}, errors: []labelExtraSANY{
		{"Target", 2, 28, 2, 28, "Could not find declaration or definition of symbol '+'."},
		{"Target", 2, 26, 2, 30, "Couldn't resolve infix operator symbol `+'."},
		{"Target", 2, 13, 2, 30, "Label lbl declares extra parameter(s)  x  y  "},
	}},
}

const labelExtraInfix = " declares extra parameter(s)  "

// labelExtraNames splits the approved template into its label prefix and its
// residual names. ok is false for any other text.
func labelExtraNames(message string) (prefix string, names []string, ok bool) {
	i := strings.Index(message, labelExtraInfix)
	if !strings.HasPrefix(message, "Label ") || i <= len("Label ") || strings.Contains(message[len("Label "):i], " ") {
		return "", nil, false
	}
	prefix, rest := message[:i+len(labelExtraInfix)], message[i+len(labelExtraInfix):]
	if !strings.HasSuffix(rest, "  ") {
		return "", nil, false
	}
	names = strings.Split(strings.TrimSuffix(rest, "  "), "  ")
	for _, n := range names {
		if n == "" || strings.ContainsAny(n, " \n") {
			return "", nil, false
		}
	}
	return prefix, names, true
}

// labelExtraEqual compares a prudence message with SANY's. Messages equal
// byte for byte match exactly. Otherwise both must be the approved template
// with the same prefix and distinct names forming the same multiset.
func labelExtraEqual(got, want string) (exact, normalised bool) {
	if got == want {
		return true, false
	}
	gp, gn, gok := labelExtraNames(got)
	wp, wn, wok := labelExtraNames(want)
	if !gok || !wok || gp != wp || len(gn) != len(wn) {
		return false, false
	}
	gs, ws := slices.Clone(gn), slices.Clone(wn)
	slices.Sort(gs)
	slices.Sort(ws)
	if !slices.Equal(gs, ws) || len(slices.Compact(slices.Clone(ws))) != len(ws) {
		return false, false
	}
	return false, true
}

// labelExtraCompare compares one severity's entries in order. It returns
// the number of entries that matched only order-normalised.
func labelExtraCompare(t *testing.T, kind string, got []Diagnostic, want []labelExtraSANY) int {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d %s, want %d: %+v", len(got), kind, len(want), got)
	}
	normalised := 0
	for i, w := range want {
		d := got[i]
		if d.Path != w.module+".tla" || d.Span.Start.Line+1 != w.l1 || d.Span.Start.Column+1 != w.c1 || d.Span.End.Line+1 != w.l2 || d.Span.End.Column != w.c2 {
			t.Fatalf("%s %d: %s %+v, SANY %+v", kind, i, d.Path, d.Span, w)
		}
		exact, norm := labelExtraEqual(d.Message, w.message)
		if !exact && !norm {
			t.Fatalf("%s %d: %q, want %q", kind, i, d.Message, w.message)
		}
		if norm {
			normalised++
		}
	}
	return normalised
}

func labelExtraBySeverity(r Result, s Severity) []Diagnostic {
	var out []Diagnostic
	for _, d := range r.Diagnostics {
		if d.Severity == s {
			out = append(out, d)
		}
	}
	return out
}

func TestLabelExtraCorpusCases(t *testing.T) {
	for _, c := range labelExtraCases {
		t.Run(c.id, func(t *testing.T) {
			files := map[string]string{}
			for name, sum := range c.files {
				data, err := os.ReadFile(filepath.Join("testdata", "label-extra-order", c.id, name))
				if err != nil {
					t.Fatal(err)
				}
				digest := sha256.Sum256(data)
				if hex.EncodeToString(digest[:]) != sum {
					t.Fatalf("%s is not the recorded case bytes", name)
				}
				files[name] = string(data)
			}
			r, err := Check(context.Background(), Request{Path: c.root, Origin: OriginWorkspace, Text: []byte(files[c.root]), Resolve: letCaseResolver(files)})
			if err != nil || r.Completion != CompletionComplete {
				t.Fatalf("%v %+v", err, r)
			}
			for _, d := range r.Diagnostics {
				if d.Code == CodeIncomplete {
					t.Fatalf("incomplete: %+v", d)
				}
			}
			extras := 0
			for _, d := range labelExtraBySeverity(r, SeverityError) {
				if _, names, ok := labelExtraNames(d.Message); ok {
					extras++
					// All 12 take the fallback; their names are ASCII, where
					// UTF-16 and byte order agree.
					if !slices.IsSorted(names) {
						t.Fatalf("fallback list not in documented order: %q", d.Message)
					}
				}
			}
			if extras != 1 {
				t.Fatalf("%d extra-parameter errors", extras)
			}
			n := labelExtraCompare(t, "errors", labelExtraBySeverity(r, SeverityError), c.errors)
			if w := labelExtraCompare(t, "warnings", labelExtraBySeverity(r, SeverityWarning), c.warnings); w != 0 {
				t.Fatalf("%d warnings matched order-normalised", w)
			}
			if (n == 1) != c.normalised || n > 1 {
				t.Fatalf("%d order-normalised entries, recorded normalised=%v", n, c.normalised)
			}
		})
	}
}

// The comparison helper admits only the approved template, a permuted list
// of the same distinct names; everything else must be byte-identical.
func TestLabelExtraEqualScope(t *testing.T) {
	want := "Label lab declares extra parameter(s)  x  i  z  "
	for _, tt := range []struct {
		got               string
		exact, normalised bool
	}{
		{want, true, false},
		{"Label lab declares extra parameter(s)  i  x  z  ", false, true},
		{"Label lab declares extra parameter(s)  z  x  i  ", false, true},
		{"Label lab declares extra parameter(s)  i  x  ", false, false},       // missing
		{"Label lab declares extra parameter(s)  i  x  z  q  ", false, false}, // extra
		{"Label lab declares extra parameter(s)  i  x  z  z  ", false, false}, // duplicated
		{"Label lab declares extra parameter(s)  i  x  ", false, false},       // truncated
		{"Label lbl declares extra parameter(s)  i  x  z  ", false, false},    // label
		{"Label lab declares extra parameter(s) i x z", false, false},         // spacing
		{"Label lab declares extra parameter(s)  i  x  z ", false, false},     // suffix
		{"Label lab declares extra parameter(s)   i  x  z  ", false, false},   // prefix
		{"Label lab must contain formal parameter `x'.", false, false},        // other text
		{"Repeated formal parameter x \nin label `lab'.", false, false},       // other text
	} {
		exact, norm := labelExtraEqual(tt.got, want)
		if exact != tt.exact || norm != tt.normalised {
			t.Errorf("%q: exact=%v normalised=%v", tt.got, exact, norm)
		}
	}
	// SANY's own list with a repeated name is never normalised.
	if _, norm := labelExtraEqual("Label L declares extra parameter(s)  y  x  x  ", "Label L declares extra parameter(s)  x  x  y  "); norm {
		t.Error("repeated-name list normalised")
	}
}
