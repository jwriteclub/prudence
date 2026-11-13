// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Based on TLC ModelConfig.java, Copyright 2003 Compaq Corporation and
// Microsoft Corporation, under the MIT license.
package gpuexport

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	sany "github.com/jwriteclub/prudence/internal/generated/sany"
	"github.com/jwriteclub/prudence/internal/lexer"
	"github.com/jwriteclub/prudence/internal/semantic"
)

type Directive struct {
	Kind  string              `json:"kind"`
	Name  string              `json:"name,omitempty"`
	Value *ConfigValue        `json:"value,omitempty"`
	Span  semantic.SourceSpan `json:"span"`
}
type ConfigValue struct {
	Kind  string        `json:"kind"`
	Text  string        `json:"text"`
	Items []ConfigValue `json:"items,omitempty"`
}
type configParser struct {
	ctx   context.Context
	lex   *lexer.Lexer
	token *lexer.Token
	names []string
	path  string
	end   [2]int
}

func configSection(s string) (kind string, list bool) {
	switch s {
	case "CONSTANT", "CONSTANTS":
		return "CONSTANT", true
	case "CONSTRAINT", "CONSTRAINTS":
		return "CONSTRAINT", true
	case "ACTION_CONSTRAINT", "ACTION_CONSTRAINTS":
		return "ACTION_CONSTRAINT", true
	case "INVARIANT", "INVARIANTS":
		return "INVARIANT", true
	case "PROPERTY", "PROPERTIES":
		return "PROPERTY", true
	case "INIT", "NEXT", "VIEW", "SYMMETRY", "SPECIFICATION", "CHECK_DEADLOCK":
		return s, false
	// TLC keywords must terminate a list even when unsupported. Otherwise a
	// following ALIAS, for example, would be misread as an invariant name.
	case "ALIAS", "POSTCONDITION", "POSTCONDITIONS", "_PERIODIC", "_RL_REWARD", "_POSSIBLE":
		return "unsupported", false
	default:
		return "", false
	}
}

func parseConfig(ctx context.Context, path string, data []byte) ([]Directive, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%s: unsupported cfg encoding", path)
	}
	// ModelConfig uses the SANY manager in SPEC state and appends a newline
	// to terminate a possible final line comment.
	lex := sany.New(string(data) + "\n")
	if err := lex.SwitchTo(2); err != nil {
		return nil, err
	}
	p := configParser{ctx: ctx, lex: lex, names: sany.TokenNames(), path: path}
	if err := p.next(); err != nil {
		return nil, err
	}
	directives := []Directive{}
	single := make(map[string]bool)
	for p.token.Kind != 0 {
		kind, list := configSection(p.token.Image)
		if kind == "" || kind == "unsupported" {
			return nil, p.invalid("unsupported configuration directive")
		}
		if !list && single[kind] {
			return nil, p.invalid("duplicate " + kind)
		}
		single[kind] = true
		if err := p.next(); err != nil {
			return nil, err
		}
		for {
			start := [2]int{p.token.Begin.Line, p.token.Begin.Column}
			d := Directive{Kind: kind}
			var err error
			if kind == "CHECK_DEADLOCK" {
				if p.token.Image != "TRUE" && p.token.Image != "FALSE" {
					return nil, p.invalid("expected TRUE or FALSE")
				}
				d.Value = &ConfigValue{Kind: "boolean", Text: p.token.Image}
				err = p.next()
			} else {
				d.Name, err = p.identifier()
				if err == nil && kind == "CONSTANT" {
					if p.token.Image != "=" {
						return nil, p.invalid("expected = (operator and module overrides unsupported)")
					}
					if err = p.next(); err == nil {
						d.Value, err = p.value(1)
					}
				}
			}
			if err != nil {
				return nil, err
			}
			d.Span = semantic.SourceSpan{Source: path, Start: start, End: p.end}
			directives = append(directives, d)
			following, _ := configSection(p.token.Image)
			if !list || p.token.Kind == 0 || following != "" {
				break
			}
		}
	}
	return directives, nil
}
func (p *configParser) next() error {
	if p.token != nil {
		p.end = [2]int{p.token.End.Line, p.token.End.Column}
	}
	t, err := p.lex.NextTokenContext(p.ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", p.path, err)
	}
	p.token = t
	return nil
}
func (p *configParser) invalid(why string) error {
	return fmt.Errorf("%s:%d:%d: %s near %q", p.path, p.token.Begin.Line, p.token.Begin.Column, why, p.token.Image)
}
func (p *configParser) identifier() (string, error) {
	if s, _ := configSection(p.token.Image); s != "" || p.names[p.token.Kind] != "IDENTIFIER" {
		return "", p.invalid("expected identifier")
	}
	name := p.token.Image
	return name, p.next()
}
func (p *configParser) value(depth int) (*ConfigValue, error) {
	if depth > 128 {
		return nil, p.invalid("cfg set nesting exceeds 128")
	}
	t := p.token
	v := &ConfigValue{Text: t.Image}
	switch {
	case t.Image == "{":
		v.Kind = "set"
		v.Text = ""
		if err := p.next(); err != nil {
			return nil, err
		}
		if p.token.Image != "}" {
			for {
				item, err := p.value(depth + 1)
				if err != nil {
					return nil, err
				}
				v.Items = append(v.Items, *item)
				if p.token.Image != "," {
					break
				}
				if err := p.next(); err != nil {
					return nil, err
				}
			}
		}
		if p.token.Image != "}" {
			return nil, p.invalid("expected }")
		}
	case t.Image == "TRUE" || t.Image == "FALSE":
		v.Kind = "boolean"
	case p.names[t.Kind] == "NUMBER_LITERAL":
		if _, err := strconv.ParseInt(t.Image, 10, 32); err != nil {
			return nil, p.invalid("expected TLC 32-bit decimal integer")
		}
		v.Kind = "integer"
	case p.names[t.Kind] == "STRING_LITERAL":
		v.Kind = "string"
		v.Text = t.Image[1 : len(t.Image)-1]
		if strings.Contains(v.Text, "\\") {
			return nil, p.invalid("escapes unsupported in cfg strings")
		}
	case p.names[t.Kind] == "IDENTIFIER":
		if section, _ := configSection(t.Image); section != "" {
			return nil, p.invalid("expected constant value")
		}
		v.Kind = "model"
	default:
		return nil, p.invalid("expected constant value")
	}
	return v, p.next()
}
