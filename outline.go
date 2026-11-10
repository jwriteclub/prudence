// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

package prudence

import (
	"sort"
	"strings"

	parser "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

func outline(roots []*parser.SyntaxTreeNode, src []byte, coords coordIndex, tokens []Token, outerModule string) Outline {
	out := Outline{Module: outerModule}
	text := func(t Token) string { return string(src[t.Span.Start.ByteOffset:t.Span.End.ByteOffset]) }

	// Filter once; the resulting slice stays ordered and can be sliced by byte range.
	filtered := make([]Token, 0, len(tokens))
	instancePrefix := []int{0}
	for _, t := range tokens {
		if t.Kind == TokenComment || t.Kind == TokenLineComment || t.Kind == TokenBlockComment {
			continue
		}
		filtered = append(filtered, t)
		k := instancePrefix[len(instancePrefix)-1]
		if text(t) == "INSTANCE" {
			k++
		}
		instancePrefix = append(instancePrefix, k)
	}
	bounds := func(n *parser.SyntaxTreeNode) (int, int) {
		loc := n.Location()
		a := coords.offset(int(loc.BeginLine()), int(loc.BeginColumn()), false, len(src))
		b := coords.offset(int(loc.EndLine()), int(loc.EndColumn()), true, len(src))
		lo := sort.Search(len(filtered), func(i int) bool { return filtered[i].Span.Start.ByteOffset >= a })
		hi := sort.Search(len(filtered), func(i int) bool { return filtered[i].Span.End.ByteOffset > b })
		if hi < lo {
			hi = lo
		}
		return lo, hi
	}
	nodeTokens := func(n *parser.SyntaxTreeNode) []Token { lo, hi := bounds(n); return filtered[lo:hi] }

	add := func(kind DeclKind, n *parser.SyntaxTreeNode, t Token, params int) {
		ts := nodeTokens(n)
		full := t.Span
		if len(ts) > 0 {
			full = Span{ts[0].Span.Start, ts[len(ts)-1].Span.End}
		}
		out.Decls = append(out.Decls, Decl{Name: text(t), Kind: kind, NameSpan: t.Span, FullSpan: full, Params: params})
	}
	var walk func(*parser.SyntaxTreeNode, bool)
	walk = func(n *parser.SyntaxTreeNode, top bool) {
		if n == nil {
			return
		}
		if n.Kind() == parser.N_Module && out.Module != "" {
			for _, c := range n.Heirs() {
				if c != nil && c.Kind() == parser.N_BeginModule {
					walk(c, false)
					break
				}
			}
			return
		}
		var ts []Token
		switch n.Kind() {
		case parser.N_BeginModule, parser.N_Extends, parser.N_Instance, parser.N_VariableDeclaration, parser.N_ConsDecl, parser.N_Recursive, parser.N_OperatorDefinition, parser.N_FunctionDefinition, parser.N_ModuleDefinition, parser.N_ParamDeclaration, parser.N_Theorem, parser.N_Assumption:
			ts = nodeTokens(n)
		}
		switch n.Kind() {
		case parser.N_BeginModule:
			for i, t := range ts {
				if strings.HasSuffix(text(t), "MODULE") && i+1 < len(ts) {
					if out.Module == "" {
						out.Module = text(ts[i+1])
						add(DeclModule, n, ts[i+1], 0)
					} else {
						add(DeclSubmodule, n, ts[i+1], 0)
					}
					break
				}
			}
		case parser.N_Extends:
			for _, t := range ts {
				if t.Kind == TokenIdentifier {
					out.Extends = append(out.Extends, text(t))
				}
			}
		case parser.N_Instance:
			for i, t := range ts {
				if text(t) == "INSTANCE" && i+1 < len(ts) && ts[i+1].Kind == TokenIdentifier {
					out.Instances = append(out.Instances, Instance{text(ts[i+1]), ts[i+1].Span})
					break
				}
			}
		case parser.N_ParamDeclaration:
			if top {
				for _, child := range n.Heirs() {
					if child == nil {
						continue
					}
					switch child.Kind() {
					case parser.N_IdentDecl, parser.N_InfixDecl, parser.N_PrefixDecl, parser.N_PostfixDecl:
						items := nodeTokens(child)
						var name *Token
						params := 0
						for i := range items {
							v := text(items[i])
							if v == "_" {
								params++
								continue
							}
							if name == nil && (items[i].Kind == TokenIdentifier || items[i].Kind == TokenOperator) {
								name = &items[i]
							}
						}
						if name != nil {
							add(DeclConstant, n, *name, params)
						}
					}
				}
			}
		case parser.N_VariableDeclaration, parser.N_ConsDecl, parser.N_Recursive:
			if top {
				kind := DeclVariable
				if n.Kind() == parser.N_ConsDecl {
					kind = DeclConstant
				}
				if n.Kind() == parser.N_Recursive {
					kind = DeclRecursive
				}
				for _, child := range n.Heirs() {
					cs := nodeTokens(child)
					if len(cs) > 0 && cs[0].Kind == TokenIdentifier {
						add(kind, n, cs[0], -1)
					}
				}
			}
		case parser.N_OperatorDefinition, parser.N_FunctionDefinition, parser.N_ModuleDefinition:
			if top {
				var lhs *parser.SyntaxTreeNode
				for _, child := range n.Heirs() {
					if child != nil && (child.Kind() == parser.N_InfixLHS || child.Kind() == parser.N_PrefixLHS || child.Kind() == parser.N_PostfixLHS || child.Kind() == parser.N_IdentLHS) {
						lhs = child
						break
					}
				}
				if lhs != nil && (lhs.Kind() == parser.N_InfixLHS || lhs.Kind() == parser.N_PrefixLHS || lhs.Kind() == parser.N_PostfixLHS) {
					for _, t := range nodeTokens(lhs) {
						if t.Kind == TokenOperator {
							count := 1
							if lhs.Kind() == parser.N_InfixLHS {
								count = 2
							}
							add(DeclOperator, n, t, count)
							break
						}
					}
				} else {
					for i, t := range ts {
						if t.Kind == TokenIdentifier {
							params := 0
							if i+1 < len(ts) && text(ts[i+1]) == "(" {
								params = 1
								depth := 0
								for j := i + 1; j < len(ts); j++ {
									switch text(ts[j]) {
									case "(":
										depth++
									case ")":
										depth--
										if depth == 0 {
											j = len(ts)
										}
									case ",":
										if depth == 1 {
											params++
										}
									}
								}
							}
							add(DeclDefinition, n, t, params)
							break
						}
					}
				}
			}
		case parser.N_Theorem, parser.N_Assumption:
			if top && len(ts) > 2 && ts[1].Kind == TokenIdentifier && text(ts[2]) == "==" {
				kind := DeclTheorem
				if n.Kind() == parser.N_Assumption {
					kind = DeclAssumption
				}
				add(kind, n, ts[1], 0)
			}
		}
		// Only module and body children are top-level declarations; other nodes
		// are visited solely to find explicit INSTANCE nodes.
		childTop := n.Kind() == parser.N_Module || n.Kind() == parser.N_Body || (top && n.Kind() == parser.N_ParamDeclaration)
		if !childTop {
			lo, hi := bounds(n)
			if instancePrefix[hi] == instancePrefix[lo] {
				return
			}
		}
		for _, c := range n.Heirs() {
			walk(c, childTop)
		}
	}
	for _, root := range roots {
		walk(root, true)
	}
	return out
}

// partialOutline only recognizes the completed opening header and the
// immediately following EXTENDS clause. It does not guess body declarations.
func partialOutline(src []byte, tokens []Token) Outline {
	var out Outline
	var ts []Token
	for _, t := range tokens {
		if t.Kind != TokenComment && t.Kind != TokenLineComment && t.Kind != TokenBlockComment {
			ts = append(ts, t)
		}
	}
	text := func(t Token) string { return string(src[t.Span.Start.ByteOffset:t.Span.End.ByteOffset]) }
	for i := 0; i+2 < len(ts); i++ {
		if strings.HasPrefix(text(ts[i]), "----") && strings.HasSuffix(text(ts[i]), "MODULE") && ts[i+1].Kind == TokenIdentifier && strings.HasPrefix(text(ts[i+2]), "----") {
			out.Module = text(ts[i+1])
			out.Decls = append(out.Decls, Decl{Name: out.Module, Kind: DeclModule, NameSpan: ts[i+1].Span, FullSpan: Span{ts[i].Span.Start, ts[i+2].Span.End}, Params: 0})
			j := i + 3
			if j < len(ts) && text(ts[j]) == "EXTENDS" {
				j++
				for j < len(ts) && ts[j].Kind == TokenIdentifier {
					out.Extends = append(out.Extends, text(ts[j]))
					j++
					if j >= len(ts) || text(ts[j]) != "," {
						break
					}
					j++
				}
			}
			break
		}
	}
	return out
}
