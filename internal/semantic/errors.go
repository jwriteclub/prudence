// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ SANY Errors.java, ErrorCode.java, AbortException.java and
// Location.java, Copyright 2003 Compaq Corporation, 2003 Microsoft Corporation,
// and 2025 Linux Foundation, under the MIT license; see LICENSE.
package semantic

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

// Location stores the source fields used by SANY Location.equals. Null marks
// the distinguished nullLoc object, whose rendering differs from an equal value.
// NamePresent distinguishes a null name from an empty module name.
type Location struct {
	SourceTree                                 *generated.SyntaxTreeNode // Provenance only; excluded from Java location equality.
	Name                                       string
	NamePresent                                bool
	BeginLine, BeginColumn, EndLine, EndColumn int
	Null                                       bool
}

// Parameter deliberately preserves Java boxed-value type or object identity.
// Node identities are allocated by the owning request and must never be reused
// during that request. More Java value types require explicit source evidence.
type ParameterKind uint8

const (
	StringParameter ParameterKind = iota + 1
	IntParameter
	LongParameter
	BoolParameter
	ObjectIdentityParameter
	LocationParameter
	SemanticNodeParameter
)

type Parameter struct {
	Kind                ParameterKind
	Text                string
	Integer             int64
	Boolean             bool
	Identity            uint64
	Location            Location
	UTF16Units          []uint16
	UTF16Present        bool
	TextUnrepresentable bool
	JavaClass           string // Optional source-proved Java parameter class.
}

// ErrPortIncomplete marks an unimplemented internal port operation; it is not a SANY rejection.
var ErrPortIncomplete = errors.New("semantic port incomplete")

type Severity uint8

const (
	Error Severity = iota + 1
	Warning
)

type Code struct {
	Name           string
	Value          int
	Severity       Severity
	ParameterCount int
}

const Variadic = -1

type Detail struct {
	Code       Code
	Location   Location
	Format     string
	Parameters []Parameter
}
type Errors struct {
	messages []Detail
	interned map[uint64]string
}
type Abort struct {
	Detail Detail
	Source *Errors
}

func (a *Abort) Error() string {
	if a == nil {
		return "semantic abort"
	}
	text, err := a.Detail.Render()
	if err != nil {
		return "semantic abort"
	}
	return text
}

func copyDetail(d Detail) Detail {
	d.Parameters = append([]Parameter(nil), d.Parameters...)
	for i := range d.Parameters {
		if d.Parameters[i].UTF16Present {
			d.Parameters[i].UTF16Units = append([]uint16{}, d.Parameters[i].UTF16Units...)
		}
	}
	return d
}
func sameLocation(a, b Location) bool {
	return a.Name == b.Name && a.NamePresent == b.NamePresent && a.BeginLine == b.BeginLine && a.BeginColumn == b.BeginColumn && a.EndLine == b.EndLine && a.EndColumn == b.EndColumn
}

// A marked Java class must agree with the port kind; an empty mark retains
// the historic kind-based semantics for existing callers.
func validParameterClass(p Parameter) bool {
	if p.JavaClass == "" {
		return true
	}
	switch p.Kind {
	case StringParameter:
		return p.JavaClass == "java.lang.String"
	case ObjectIdentityParameter:
		return p.JavaClass == "util.UniqueString"
	case IntParameter:
		return p.JavaClass == "java.lang.Integer"
	case LongParameter:
		return p.JavaClass == "java.lang.Long"
	case BoolParameter:
		return p.JavaClass == "java.lang.Boolean"
	default:
		return false
	}
}

func sameParameter(a, b Parameter) bool {
	if !validParameterClass(a) || !validParameterClass(b) || a.Kind != b.Kind || (a.JavaClass != "" && b.JavaClass != "" && a.JavaClass != b.JavaClass) {
		return false
	}
	switch a.Kind {
	case StringParameter:
		if a.UTF16Present && b.UTF16Present {
			if len(a.UTF16Units) != len(b.UTF16Units) {
				return false
			}
			for i := range a.UTF16Units {
				if a.UTF16Units[i] != b.UTF16Units[i] {
					return false
				}
			}
			return true
		}
		if a.TextUnrepresentable || b.TextUnrepresentable {
			return false
		}
		return a.Text == b.Text
	case IntParameter, LongParameter:
		return a.Integer == b.Integer
	case BoolParameter:
		return a.Boolean == b.Boolean
	case ObjectIdentityParameter, SemanticNodeParameter:
		return a.Identity == b.Identity
	case LocationParameter:
		return sameLocation(a.Location, b.Location)
	default:
		return false
	}
}
func sameDetail(a, b Detail) bool {
	if a.Code.Name != b.Code.Name || !sameLocation(a.Location, b.Location) || a.Format != b.Format || len(a.Parameters) != len(b.Parameters) {
		return false
	}
	for i := range a.Parameters {
		if !sameParameter(a.Parameters[i], b.Parameters[i]) {
			return false
		}
	}
	return true
}

// Add records one ordered message if its full structured identity is new. It
// always returns a new abort object, including for a duplicate; only a caller
// deciding to abort should propagate it. Nil location is SANY's nullLoc.
func validLocation(l Location) bool {
	if !l.NamePresent {
		return false
	}
	return !l.Null || (l.Name == "--unknown--" && l.BeginLine == 0 && l.BeginColumn == 0 && l.EndLine == 0 && l.EndColumn == 0)
}
func (e *Errors) Add(code Code, loc *Location, format string, params ...Parameter) (*Abort, error) {
	canonical, ok := CodeFromValue(code.Value)
	if !ok || canonical != code {
		return nil, ErrPortIncomplete
	}
	l := Location{Null: true, Name: "--unknown--", NamePresent: true}
	if loc != nil {
		l = *loc
	}
	if !validLocation(l) {
		return nil, ErrPortIncomplete
	}
	pending := make(map[uint64]string)
	for _, p := range params {
		if !validParameterClass(p) {
			return nil, ErrPortIncomplete
		}
		switch p.Kind {
		case StringParameter, LongParameter, BoolParameter:
		case SemanticNodeParameter:
			if p.Identity == 0 {
				return nil, ErrPortIncomplete
			}
		case IntParameter:
			if p.Integer < -2147483648 || p.Integer > 2147483647 {
				return nil, ErrPortIncomplete
			}
		case ObjectIdentityParameter:
			if p.Identity == 0 {
				return nil, ErrPortIncomplete
			}
			if v, ok := e.interned[p.Identity]; ok && v != p.Text {
				return nil, ErrPortIncomplete
			}
			if v, ok := pending[p.Identity]; ok && v != p.Text {
				return nil, ErrPortIncomplete
			}
			pending[p.Identity] = p.Text
		case LocationParameter:
			if !validLocation(p.Location) {
				return nil, ErrPortIncomplete
			}
		default:
			return nil, ErrPortIncomplete
		}
	}
	if len(pending) > 0 {
		if e.interned == nil {
			e.interned = make(map[uint64]string)
		}
		for k, v := range pending {
			e.interned[k] = v
		}
	}
	d := copyDetail(Detail{Code: code, Location: l, Format: format, Parameters: params})
	found := false
	for _, old := range e.messages {
		if sameDetail(old, d) {
			found = true
			break
		}
	}
	if !found {
		e.messages = append(e.messages, d)
	}
	return &Abort{Detail: copyDetail(d), Source: e}, nil
}
func (e *Errors) Messages() []Detail {
	out := make([]Detail, len(e.messages))
	for i, d := range e.messages {
		out[i] = copyDetail(d)
	}
	return out
}
func (e *Errors) OfLevel(level Severity) []Detail {
	var out []Detail
	for _, d := range e.messages {
		if d.Code.Severity == level {
			out = append(out, copyDetail(d))
		}
	}
	return out
}
func (e *Errors) Success() bool { return len(e.OfLevel(Error)) == 0 }

// Render is intentionally bounded to the plain %s conversions used by the
// initial Context/SymbolTable slice. Other Java Formatter conversions remain
// explicit port limitations. Zero-parameter messages are returned verbatim.
func (d Detail) Render() (string, error) {
	if len(d.Parameters) == 0 {
		return d.Format, nil
	}
	var out strings.Builder
	n := 0
	for i := 0; i < len(d.Format); i++ {
		if d.Format[i] != '%' {
			out.WriteByte(d.Format[i])
			continue
		}
		if i+1 >= len(d.Format) || d.Format[i+1] != 's' || n >= len(d.Parameters) {
			return "", ErrPortIncomplete
		}
		i++
		p := d.Parameters[n]
		n++
		switch p.Kind {
		case StringParameter, ObjectIdentityParameter, SemanticNodeParameter:
			if p.TextUnrepresentable {
				return "", ErrPortIncomplete
			}
			out.WriteString(p.Text)
		case IntParameter, LongParameter:
			out.WriteString(strconv.FormatInt(p.Integer, 10))
		case BoolParameter:
			out.WriteString(strconv.FormatBool(p.Boolean))
		case LocationParameter:
			text, err := renderLocation(p.Location)
			if err != nil {
				return "", err
			}
			out.WriteString(text)
		default:
			return "", ErrPortIncomplete
		}
	}
	// String.format permits unused extra arguments.
	return out.String(), nil
}
func renderLocation(l Location) (string, error) {
	if l.Null {
		return "Unknown location", nil
	}
	if !l.NamePresent {
		return "", ErrPortIncomplete
	}
	if l.Name != "--unknown--" && l.BeginLine == 0 && l.BeginColumn == 0 && l.EndLine == 0 && l.EndColumn == 0 {
		return "In module " + l.Name, nil
	}
	return fmt.Sprintf("line %d, col %d to line %d, col %d of module %s", l.BeginLine, l.BeginColumn, l.EndLine, l.EndColumn, l.Name), nil
}
