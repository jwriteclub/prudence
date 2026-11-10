// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ Generator.java, Copyright 2003 Compaq Corporation and
// Microsoft Corporation, under the MIT license; see LICENSE.
package semantic

import (
	"math"
	"slices"
	"unicode/utf16"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

// Source-derived recovery branches for declarations, proofs and statements,
// pinned to Generator.java at dc6a944. Each helper names the Java lines it
// follows; any shape the source does not settle stays port-incomplete.

// Selector.ops values from Generator.java:333-338 that the DEF-entry path
// distinguishes. A numeric StructOp keeps its non-negative operand number.
const (
	defSelName = -1
	defSelNull = -2
)

// defSelectorItem is one Selector.addSelector entry (Generator.java:388-396):
// the ops syntax node and its argument list, or nil.
type defSelectorItem struct{ op, args *generated.SyntaxTreeNode }

// malformedProofDef follows generateUseOrHide's DEF loop
// (Generator.java:6680-6700) for an entry that is neither an N_GeneralId nor
// an N_OpApplication, such as an infix expression, a record component or a
// tuple. The entry is passed unchanged to genIdToSelector (506-579), which
// reads heirs()[0] as the prefix and heirs()[1] as the last selector, then to
// Selector.finish (398-475) and selectorToNode with expectedArity -1
// (622-760). Every modeled path stops at selector index 0 and returns nullOAN,
// so generateUseOrHide reports "DEF clause entry should describe a defined
// operator." No SemanticNode is constructed.
//
// Java throws (and the port stays incomplete) when the entry has no heirs or
// a single heir (ArrayIndexOutOfBounds at 514 or 516), when a numeric StructOp
// is malformed or does not fit Integer.parseInt (424), and when
// selectorItemToString reads a missing StructOp number (2101). A first
// selector that is a name without arguments would continue into symbol
// resolution; that path stays incomplete here. The source aborts at 557 and
// 443 are reported as the Errors abort, after the messages that precede them.
func (g *generator) malformedProofDef(tree *generated.SyntaxTreeNode) (leafNode, error) {
	heirs := tree.Heirs()
	if len(heirs) == 0 || heirs[0] == nil {
		return nil, g.unported("generator.construct.proof-def-selector")
	}
	// genIdToSelector, without effects: the loop either breaks at the first
	// prefix element with no heirs (after one "Was expecting a GeneralId."),
	// or aborts at an element whose first heir is a name with other than two
	// or three heirs.
	var items []defSelectorItem
	expecting := false
	var abortElement *generated.SyntaxTreeNode
	for _, element := range heirs[0].Heirs() {
		if element == nil {
			return nil, g.unported("generator.construct.proof-def-selector")
		}
		pe := element.Heirs()
		if len(pe) == 0 {
			expecting = true
			break
		}
		first := pe[0]
		if first == nil {
			return nil, g.unported("generator.construct.proof-def-selector")
		}
		switch first.Kind() {
		case generated.N_OpArgs:
			items = append(items, defSelectorItem{first, first})
		case generated.N_StructOp:
			items = append(items, defSelectorItem{first, nil})
		default:
			switch len(pe) {
			case 2:
				items = append(items, defSelectorItem{first, nil})
			case 3:
				if pe[1] == nil {
					return nil, g.unported("generator.construct.proof-def-selector")
				}
				items = append(items, defSelectorItem{first, pe[1]})
			default:
				abortElement = element
			}
		}
		if abortElement != nil {
			break
		}
	}
	if abortElement != nil {
		loc, err := g.diagnosticLocation(abortElement)
		if err != nil {
			return nil, err
		}
		code, ok := CodeFromValue(4003)
		if !ok {
			return nil, g.unported("generator.construct.proof-def-selector")
		}
		abort, err := g.table.errors.Add(code, &loc, "Internal error: IdPrefixElement has other than 2 or 3 heirs.")
		if err != nil {
			return nil, err
		}
		return nil, abort
	}
	if len(heirs) < 2 || heirs[1] == nil {
		return nil, g.unported("generator.construct.proof-def-selector")
	}
	last := heirs[1]
	if last.Kind() == generated.N_OpArgs {
		items = append(items, defSelectorItem{last, last})
	} else {
		items = append(items, defSelectorItem{last, nil})
	}
	// Selector.finish, without effects: classify every item and find the
	// first that aborts.
	ops := make([]int, len(items))
	abortAt := -1
	for i, item := range items {
		op, abort, ok := defSelectorOp(item.op)
		if !ok {
			return nil, g.unported("generator.construct.proof-def-selector")
		}
		ops[i] = op
		if abort {
			abortAt = i
			break
		}
	}
	// selectorToNode at index 0 (Generator.java:704-760): supplied arguments
	// are rejected first, because expectedArity is -1; otherwise a first
	// selector that is not a name has no starting point.
	var argText Parameter
	if abortAt < 0 {
		switch {
		case items[0].args != nil:
			text, ok := defSelectorItemText(items[0].op)
			if !ok {
				return nil, g.unported("generator.construct.proof-def-selector")
			}
			argText = text
		case ops[0] == defSelName:
			return nil, g.unported("generator.construct.proof-def-selector")
		}
	}
	// Effects, in source order.
	if expecting {
		loc, err := g.diagnosticLocation(tree)
		if err != nil {
			return nil, err
		}
		if err = addError(g.table.errors, 4003, loc, "Was expecting a GeneralId."); err != nil {
			return nil, err
		}
	}
	for i, item := range items {
		loc, err := g.diagnosticLocation(item.op)
		if err != nil {
			return nil, err
		}
		if i == abortAt {
			// Generator.java:443 passes the String stn.getImage().
			image := item.op.Image()
			param := Parameter{Kind: StringParameter, Text: image.String()}
			if slices.Equal(utf16.Encode([]rune(image.String())), image.Units()) {
				param = exactStringParam(image.String())
			}
			code, ok := CodeFromValue(4003)
			if !ok {
				return nil, g.unported("generator.construct.proof-def-selector")
			}
			abort, err := g.table.errors.Add(code, &loc, "Internal error: Unexpected selector `%s'.", param)
			if err != nil {
				return nil, err
			}
			return nil, abort
		}
		if ops[i] == 0 && !defSelectorNumeric(item.op) {
			if err = addError(g.table.errors, 4003, loc, "Unexpected token found."); err != nil {
				return nil, err
			}
		}
	}
	loc, err := g.diagnosticLocation(items[0].op)
	if err != nil {
		return nil, err
	}
	if items[0].args != nil {
		if err = addError(g.table.errors, 4005, loc, "Selector `%s' should not have argument(s).", argText); err != nil {
			return nil, err
		}
	} else {
		name, err := g.name(items[0].op)
		if err != nil {
			return nil, err
		}
		if err = addError(g.table.errors, 4005, loc, "Need name or step number here, not `%s'.", nameParam(name)); err != nil {
			return nil, err
		}
	}
	return g.rejectedProofDef(tree)
}

// defSelectorOp is Selector.finish's classification of one ops node
// (Generator.java:409-473). abort reports the "Unexpected selector" abort at
// 443; ok is false where Java throws an unchecked exception or the image is
// unavailable. A default kind is 0, which finish reports as an unexpected
// token; a numeric StructOp also yields its number, which may be 0.
func defSelectorOp(n *generated.SyntaxTreeNode) (op int, abort, ok bool) {
	if n == nil || n.Image() == nil {
		return 0, false, false
	}
	switch n.Kind() {
	case 289, 290, 291, generated.N_InfixOp, generated.N_NonExpPrefixOp, generated.N_PostfixOp, generated.N_PrefixOp: // IDENTIFIER, ProofStepLexeme, ProofImplicitStepLexeme
		return defSelName, false, true
	case generated.N_StructOp:
		if len(n.Heirs()) > 0 {
			value, ok := defSelectorNumber(n)
			return value, false, ok
		}
		switch n.Image().String() {
		case ">>":
			return -3, false, true
		case "<<":
			return -4, false, true
		case ":":
			return -5, false, true
		case "@":
			return -6, false, true
		}
		return 0, true, true
	case generated.N_OpArgs:
		return defSelNull, false, true
	default:
		return 0, false, true
	}
}

// defSelectorNumber reads stn.heirs()[0].heirs()[0].getImage() and applies
// Integer.parseInt (Generator.java:424). Only an ASCII decimal that fits an
// int is admitted; anything else is a Java exception or unproved.
func defSelectorNumber(n *generated.SyntaxTreeNode) (int, bool) {
	h := n.Heirs()
	if len(h) == 0 || h[0] == nil || len(h[0].Heirs()) == 0 || h[0].Heirs()[0] == nil || h[0].Heirs()[0].Image() == nil {
		return 0, false
	}
	text := h[0].Heirs()[0].Image().String()
	if text == "" {
		return 0, false
	}
	value := 0
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return 0, false
		}
		digit := int(text[i] - '0')
		// Check before multiplying so a 32-bit int cannot wrap past the
		// Java int range.
		if value > (math.MaxInt32-digit)/10 {
			return 0, false
		}
		value = 10*value + digit
	}
	return value, true
}

func defSelectorNumeric(n *generated.SyntaxTreeNode) bool {
	return n.Kind() == generated.N_StructOp && len(n.Heirs()) > 0
}

// defSelectorItemText is selectorItemToString (Generator.java:2099-2108): the
// ops node's image, with the literal images "N_StructOp" and "N_OpArgs"
// rewritten. It compares strings, as the source does.
func defSelectorItemText(n *generated.SyntaxTreeNode) (Parameter, bool) {
	image := n.Image()
	if image == nil {
		return Parameter{}, false
	}
	switch image.String() {
	case "N_StructOp":
		h := n.Heirs()
		if len(h) == 0 || h[0] == nil || len(h[0].Heirs()) == 0 || h[0].Heirs()[0] == nil || h[0].Heirs()[0].Image() == nil {
			return Parameter{}, false
		}
		return asciiStringParam("argument selector " + h[0].Heirs()[0].Image().String()), true
	case "N_OpArgs":
		return asciiStringParam("!(...)"), true
	}
	units := image.Units()
	text := image.String()
	return Parameter{Kind: StringParameter, JavaClass: "java.lang.String", Text: text, UTF16Units: append([]uint16{}, units...), UTF16Present: true, TextUnrepresentable: !slices.Equal(utf16.Encode([]rune(text)), units)}, true
}

// atDeclaration is the "@" check that processVariables (Generator.java:
// 2386-2395) and processParameters (2463-2470) make before constructing each
// OpDeclNode. processVariables tests the declared identifier. processParameters
// tests treeNodes[lvi].getUS(), the image of the N_IdentDecl, N_PrefixDecl,
// N_InfixDecl or N_PostfixDecl node rather than the declared name, so for the
// parser's declaration nodes it never matches; the test is kept as written.
// Neither check stops the declaration: the OpDeclNode named "@" is still
// constructed and inserted (2399; buildParameter at 2471 and 2457).
func (g *generator) atDeclaration(declaration, node *generated.SyntaxTreeNode) error {
	image := node.Image()
	if image == nil {
		return nil
	}
	if units := image.Units(); len(units) != 1 || units[0] != '@' {
		return nil
	}
	message := "Attempted to declare '@' as a variable."
	if declaration.Kind() == generated.N_ParamDeclaration {
		message = "Attempted to declare '@' as a constant."
	}
	loc, err := g.diagnosticLocation(node)
	if err != nil {
		return err
	}
	return addError(g.table.errors, 4261, loc, message)
}
