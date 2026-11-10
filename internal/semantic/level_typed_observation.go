//go:build prudence_phase2_adapter

// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	"encoding/json"
	"fmt"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

// javaString emits UTF-16 units directly, including unpaired surrogates.
type javaString []uint16

func (s javaString) MarshalJSON() ([]byte, error) {
	b := []byte{'"'}
	for _, u := range s {
		b = fmt.Appendf(b, "\\u%04x", u)
	}
	return append(b, '"'), nil
}
func typedUnits(s string) javaString { return javaString(utf16.Encode([]rune(s))) }

type typedLocation struct {
	Source *string `json:"source"`
	BL     int     `json:"bl"`
	BC     int     `json:"bc"`
	EL     int     `json:"el"`
	EC     int     `json:"ec"`
}
type typedParameter struct {
	Type  string     `json:"type"`
	Value javaString `json:"value"`
}
type typedDiagnostic struct {
	Code       int              `json:"code"`
	Severity   string           `json:"severity"`
	Location   typedLocation    `json:"location"`
	Format     string           `json:"format"`
	Parameters []typedParameter `json:"parameters"`
	Rendered   javaString       `json:"rendered"`
}

// Phase2TypedLevelDiagnostics serializes only representations whose Java type
// and rendering are available. The candidate identity and source mapping are
// separate obligations. A nil slice produces an empty JSON array.
func Phase2TypedLevelDiagnostics(details []Detail) ([]byte, error) {
	records := make([]typedDiagnostic, 0, len(details))
	for _, d := range details {
		if !validLocation(d.Location) || !utf8.ValidString(d.Format) {
			return nil, ErrPortIncomplete
		}
		sev := "ERROR"
		if d.Code.Severity == Warning {
			sev = "WARNING"
		} else if d.Code.Severity != Error {
			return nil, ErrPortIncomplete
		}
		source := d.Location.Name
		r := typedDiagnostic{Code: d.Code.Value, Severity: sev, Location: typedLocation{Source: &source, BL: d.Location.BeginLine, BC: d.Location.BeginColumn, EL: d.Location.EndLine, EC: d.Location.EndColumn}, Format: d.Format, Parameters: make([]typedParameter, 0, len(d.Parameters))}
		values := make([]javaString, 0, len(d.Parameters))
		for _, p := range d.Parameters {
			if !validParameterClass(p) {
				return nil, ErrPortIncomplete
			}
			var v typedParameter
			switch p.Kind {
			case StringParameter, ObjectIdentityParameter:
				if p.Kind == StringParameter {
					v.Type = "java.lang.String"
				} else if p.JavaClass == "util.UniqueString" && p.Identity != 0 {
					v.Type = "util.UniqueString"
				} else {
					return nil, ErrPortIncomplete
				}
				if p.UTF16Present {
					v.Value = append(javaString{}, p.UTF16Units...)
				} else if p.TextUnrepresentable || !utf8.ValidString(p.Text) {
					return nil, ErrPortIncomplete
				} else {
					v.Value = typedUnits(p.Text)
				}
			case IntParameter:
				if p.Integer < -2147483648 || p.Integer > 2147483647 {
					return nil, ErrPortIncomplete
				}
				v.Type = "java.lang.Integer"
				v.Value = typedUnits(strconv.FormatInt(p.Integer, 10))
			case LongParameter:
				v.Type = "java.lang.Long"
				v.Value = typedUnits(strconv.FormatInt(p.Integer, 10))
			case BoolParameter:
				v.Type = "java.lang.Boolean"
				v.Value = typedUnits(strconv.FormatBool(p.Boolean))
			default:
				return nil, ErrPortIncomplete
			}
			r.Parameters = append(r.Parameters, v)
			values = append(values, v.Value)
		}
		format := typedUnits(d.Format)
		if len(values) == 0 {
			r.Rendered = append(javaString{}, format...)
		} else {
			next := 0
			for i := 0; i < len(format); i++ {
				if format[i] != '%' {
					r.Rendered = append(r.Rendered, format[i])
					continue
				}
				if i+1 >= len(format) || format[i+1] != 's' || next >= len(values) {
					return nil, ErrPortIncomplete
				}
				r.Rendered = append(r.Rendered, values[next]...)
				next++
				i++
			}
		}
		records = append(records, r)
	}
	data, err := json.Marshal(records)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// Phase2TypedLevelStageDiagnostics admits only call sites whose format and
// argument classes are established from the pinned level implementations.
// Unsupported objects or any unrecognized producer remain incomplete.
func Phase2TypedLevelStageDiagnostics(details []Detail) ([]byte, error) {
	for _, d := range details {
		canonical, ok := CodeFromValue(d.Code.Value)
		if !ok || canonical != d.Code {
			return nil, ErrPortIncomplete
		}
		var format string
		var kinds []ParameterKind
		switch d.Code.Value {
		case 4290:
			format = "Argument %s of recursive operator %s is primed"
			kinds = []ParameterKind{IntParameter, ObjectIdentityParameter}
		case 4272:
			format = "Level error in applying operator %s:\nThe permitted level of argument %s of the operator argument %s \nmust be at least %s."
			kinds = []ParameterKind{ObjectIdentityParameter, IntParameter, IntParameter, IntParameter}
		case 4273, 4205:
			format = "Level error in applying operator %s:\nThe level of argument %s exceeds the maximum level allowed by the operator."
			kinds = []ParameterKind{ObjectIdentityParameter, IntParameter}
		case 4206:
			format = "Level error: assumptions must be level 0 (Constant), \nbut this one has level %s."
			kinds = []ParameterKind{IntParameter}
		case 4244:
			format = "Error in instantiating module '%s':\n A non-Leibniz operator substituted for '%s'."
			kinds = []ParameterKind{ObjectIdentityParameter, ObjectIdentityParameter}
		case 4245:
			format = "Level error in instantiating module '%s':\nThe level of the expression or operator substituted for '%s' \nmust be at most %s."
			kinds = []ParameterKind{ObjectIdentityParameter, ObjectIdentityParameter, IntParameter}
		case 4246:
			format = "Level error in instantiating module '%s':\nThe level of the argument %s of the operator %s \nmust be at least %s."
			kinds = []ParameterKind{ObjectIdentityParameter, IntParameter, ObjectIdentityParameter, IntParameter}
		case 4247:
			format = "Level error when instantiating module '%s':\nThe level of the argument %s of the operator %s' \nmust be at most %s."
			kinds = []ParameterKind{ObjectIdentityParameter, IntParameter, ObjectIdentityParameter, IntParameter}
		case 4310:
			format = "[] followed by action not of form [A]_v."
		case 4311:
			format = "<> followed by action not of form <<A>>_v."
		case 4312:
			format = "Action used where only temporal formula or state predicate allowed."
		case 4313:
			format = "%s has both temporal formula and action as arguments."
			kinds = []ParameterKind{StringParameter}
		case 4314:
			format = "Action-level bound of quantified temporal formula."
		case 4352:
			format = "Non-constant TAKE, WITNESS, or HAVE for temporal goal."
		case 4353:
			format = "Non-constant CASE for temporal goal."
		case 4354:
			format = "Non-constant bound of temporal PICK."
		case 4356:
			format = "Level error:\nTemporal formula used as set."
		default:
			return nil, ErrPortIncomplete
		}
		if d.Format != format || len(d.Parameters) != len(kinds) {
			return nil, ErrPortIncomplete
		}
		for i, k := range kinds {
			if d.Parameters[i].Kind != k {
				return nil, ErrPortIncomplete
			}
		}
	}
	return Phase2TypedLevelDiagnostics(details)
}

// Phase2TypedGenerationStageDiagnostics admits only pinned Java call signatures.
// String arguments additionally require source-owned UTF-16 provenance.
func Phase2TypedGenerationStageDiagnostics(details []Detail) ([]byte, error) {
	for _, d := range details {
		canonical, ok := CodeFromValue(d.Code.Value)
		if !ok || canonical != d.Code {
			return nil, ErrPortIncomplete
		}
		var kinds []ParameterKind
		switch {
		case d.Code.Value == 4242 && d.Format == "Identifier '%s' is not a legal target of a substitution. \nA legal target must be a declared CONSTANT or VARIABLE in the module being instantiated. \n(Also, check for warnings about multiple declarations of this same identifier.)":
			kinds = []ParameterKind{ObjectIdentityParameter}
		case d.Code.Value == 4241 && d.Format == "Multiple substitutions for symbol '%s' in substitution.":
			kinds = []ParameterKind{StringParameter}
		case d.Code.Value == 4243 && d.Format == "An operator must be substituted for symbol '%s', and it must have arity %s.":
			kinds = []ParameterKind{ObjectIdentityParameter, IntParameter}
		case d.Code.Value == 4004 && d.Format == "Arity %s operator (not an expression) is expected \nto substitute for CONSTANT '%s'.":
			kinds = []ParameterKind{IntParameter, ObjectIdentityParameter}
		case d.Code.Value == 4004 && d.Format == "Could not find declaration or definition of symbol '%s'.":
			kinds = []ParameterKind{ObjectIdentityParameter}
		case d.Code.Value == 4005 && d.Format == "Module `%s' used without being extended or instantiated.":
			kinds = []ParameterKind{ObjectIdentityParameter}
		case d.Code.Value == 4201 && d.Format == "Multiply-defined symbol '%s': this definition or declaration conflicts \nwith the one at %s.":
			kinds = []ParameterKind{ObjectIdentityParameter, StringParameter}
		case d.Code.Value == 4801 && d.Format == "Multiple declarations or definitions for symbol %s.  \nThis duplicates the one at %s.":
			kinds = []ParameterKind{ObjectIdentityParameter, StringParameter}
		case d.Code.Value == 4223 && d.Format == "Multiply-defined module '%s': this definition or declaration conflicts \nwith the one at %s.":
			kinds = []ParameterKind{ObjectIdentityParameter, StringParameter}
		// Generator.java:810, UniqueString curName.
		case d.Code.Value == 4200 && d.Format == "Unknown operator: `%s'.":
			kinds = []ParameterKind{ObjectIdentityParameter}
		case d.Code.Value == 4204 && d.Format == "The operator %s requires %s arguments.":
			kinds = []ParameterKind{StringParameter, IntParameter}
		case d.Code.Value == 4201 && (d.Format == "Operator %s already defined or declared." || d.Format == "Function name `%s' already defined or declared."):
			kinds = []ParameterKind{StringParameter}
		case d.Code.Value == 4291 && d.Format == "Symbol %s declared in RECURSIVE statement but not defined.":
			kinds = []ParameterKind{StringParameter}
		case d.Code.Value == 4292 && (d.Format == "Definition of %s has different arity than its RECURSIVE declaration." || d.Format == "Function %s has operator arguments in its RECURSIVE declaration."):
			kinds = []ParameterKind{ObjectIdentityParameter}
		case d.Code.Value == 4293 && (d.Format == "Recursive operator %s defined at wrong LET/IN level." || d.Format == "Recursive function %s defined at wrong LET/IN level."):
			kinds = []ParameterKind{StringParameter}
		case d.Code.Value == 4294 && d.Format == "%s may not appear within a recursive definition section.":
			kinds = []ParameterKind{StringParameter}
		case d.Code.Value == 4260 && d.Format == "Function '%s' is defined with %s parameters, but is applied to %s arguments.":
			kinds = []ParameterKind{ObjectIdentityParameter, IntParameter, IntParameter}
		case d.Code.Value == 4270 && d.Format == "An expression appears as argument number %s (counting from 1) to operator '%s', in a position an operator is required.":
			kinds = []ParameterKind{IntParameter, ObjectIdentityParameter}
		case d.Code.Value == 4271 && d.Format == "Expected arity %s but found operator of arity %s.":
			kinds = []ParameterKind{IntParameter, IntParameter}
		case d.Code.Value == 4271 && d.Format == "Argument number %s to operator '%s' \nshould be a %s-parameter operator.":
			kinds = []ParameterKind{IntParameter, ObjectIdentityParameter, IntParameter}
		case d.Code.Value == 4274 && d.Format == "Lambda expression with arity %s used as argument %s of operator `%s', \nbut an operator of arity %s is required.":
			kinds = []ParameterKind{IntParameter, IntParameter, ObjectIdentityParameter, IntParameter}
		case d.Code.Value == 4004 && d.Format == "Wrong number of arguments (%s) given to operator '%s', \nwhich requires %s arguments.":
			kinds = []ParameterKind{IntParameter, ObjectIdentityParameter, IntParameter}
		case d.Code.Value == 4004 && d.Format == "Cannot find label `%s'.":
			kinds = []ParameterKind{StringParameter}
		case d.Code.Value == 4005 && (d.Format == "Selector with %s argument(s) used for quantifier with %s bound identifier(s)." || d.Format == "Selector with %s arguments used for LAMBDA expression taking %s arguments."):
			kinds = []ParameterKind{IntParameter, IntParameter}
		case d.Code.Value == 4337 && d.Format == "Label `%s' used with wrong number of arguments.":
			kinds = []ParameterKind{StringParameter}
		case d.Code.Value == 4336 && d.Format == "Duplicate label `%s'.":
			kinds = []ParameterKind{StringParameter}
		case d.Code.Value == 4332 && d.Format == "Illegal parameter %s of label `%s'.":
			kinds = []ParameterKind{StringParameter, StringParameter}
		case d.Code.Value == 4330 && d.Format == "Repeated formal parameter %s \nin label `%s'.":
			kinds = []ParameterKind{StringParameter, StringParameter}
		case d.Code.Value == 4331 && d.Format == "Label %s must contain formal parameter `%s'.":
			kinds = []ParameterKind{StringParameter, StringParameter}
		case d.Code.Value == 4003 && (d.Format == "Empty USE or HIDE statement." || d.Format == "Empty BY" || d.Format == "Empty BY, USE, or HIDE"):
		case d.Code.Value == 4275 && d.Format == "LAMBDA expression used where an expression is required.":
		case d.Code.Value == 4355 && d.Format == "ASSUME/PROVE used where an expression is required.":
		case d.Code.Value == 4262 && d.Format == "Non-unique fields in constructor.":
		case d.Code.Value == 4333 && d.Format == "Label not in definition or proof step.":
		default:
			var ok bool
			if kinds, ok = generationSourceSignature(d.Code.Value, d.Format); !ok {
				return nil, ErrPortIncomplete
			}
		}
		if len(kinds) != len(d.Parameters) {
			return nil, ErrPortIncomplete
		}
		for i, k := range kinds {
			p := d.Parameters[i]
			if p.Kind != k || (k == StringParameter && (p.JavaClass != "java.lang.String" || !p.UTF16Present)) {
				return nil, ErrPortIncomplete
			}
		}
	}
	return Phase2TypedLevelDiagnostics(details)
}

// generationSourceSignatures lists further implemented Generator call sites
// whose code, exact format and argument classes are pinned in tla2sany/semantic
// at dc6a944. Sites are file:line of the addMessage call. Java String arguments
// are StringParameter (with source UTF-16 provenance, checked by the caller);
// util.UniqueString arguments are ObjectIdentityParameter; Java int arguments
// box to Integer. Sites passing Location or semantic-node objects are absent:
// the Java observer marks them unsupported.
var generationSourceSignatures = []struct {
	code   int
	format string
	kinds  []ParameterKind
}{
	// INTERNAL_ERROR.
	{4003, "Substitution error, probably due to error \nin module being instantiated.", nil},                                          // SubstInNode.java:77
	{4003, "Unexpected token found.", nil},                                                                                            // Generator.java:463
	{4003, "Was expecting a GeneralId.", nil},                                                                                         // Generator.java:525
	{4003, "Internal error: IdPrefixElement has other than 2 or 3 heirs.", nil},                                                       // Generator.java:557
	{4003, "Internal error: Expecting label to be in AssumeProveNode, but it's not.", nil},                                            // Generator.java:3579
	{4003, "Internal error: Unexpected selector `%s'.", []ParameterKind{StringParameter}},                                             // Generator.java:443, String stn.getImage()
	{4003, "An unexpected null node specified by '%s'.\nThis is probably due to a previous error.", []ParameterKind{StringParameter}}, // Generator.java:2130, String selectorItemToString
	{4003, "Internal error: No context available for module `%s'.", []ParameterKind{StringParameter}},                                 // Generator.java:5374, String moduleId.toString()
	{4003, "Internal error: Unexpected SyntaxTreeNode kind: %s", []ParameterKind{IntParameter}},                                       // Generator.java:6361, int getKind()
	{4003, "Unsupported expression type `%s'.", []ParameterKind{StringParameter}},                                                     // Generator.java:3499, String getImage()
	// SUSPECTED_UNREACHABLE_CHECK.
	{4004, "Operator used with the wrong number of arguments.", nil},                                   // OpDeclNode.java:84
	{4004, "Substitution error, probably due to error in module being instantiated.", nil},             // APSubstInNode.java:74
	{4004, "DEF clause entry refers to a non-definition step.", nil},                                   // Generator.java:1663
	{4004, "Labeled ASSUME/PROVE used where an expression is required.", nil},                          // Generator.java:3488
	{4004, "%s", []ParameterKind{StringParameter}},                                                     // Generator.java:1675, String literal
	{4004, "Expected %s arguments but found %s.", []ParameterKind{IntParameter, IntParameter}},         // Generator.java:2041, int length/size()
	{4004, "Couldn't resolve infix operator symbol `%s'.", []ParameterKind{ObjectIdentityParameter}},   // Generator.java:3089 and 6154, getCompoundIDUS()
	{4004, "Couldn't resolve prefix operator symbol `%s'.", []ParameterKind{ObjectIdentityParameter}},  // Generator.java:3113, getCompoundIDUS()
	{4004, "Couldn't resolve postfix operator symbol `%s'.", []ParameterKind{ObjectIdentityParameter}}, // Generator.java:3131, getCompoundIDUS()
	// UNSUPPORTED_LANGUAGE_FEATURE.
	{4005, "Selector `%s' should not have argument(s).", []ParameterKind{StringParameter}},                                                                                             // Generator.java:706, String selectorItemToString
	{4005, "Need name or step number here, not `%s'.", []ParameterKind{ObjectIdentityParameter}},                                                                                       // Generator.java:758, UniqueString opNames[idx]
	{4005, "Cannot take subexpression of `%s'.", []ParameterKind{StringParameter}},                                                                                                     // Generator.java:856, String toString()
	{4005, "Subexpression of  `%s' used inside the operator's definition.", []ParameterKind{StringParameter}},                                                                          // Generator.java:964, String toString()
	{4005, "Accessing subexpression labeled `%s' of ASSUME/PROVE clause within the scope of a declaration\n from outside that declaration's scope.", []ParameterKind{StringParameter}}, // Generator.java:1079, String toString()
	{4005, "`!:' can be used only after a name and either at the end after an\noperator name or before an operator name.", nil},                                                        // Generator.java:1133
	{4005, "Subexpression selection failed, probably due to an error in the selected expression.", nil},                                                                                // Generator.java:1141
	{4005, "A LET/IN expression has only one operand.", nil},                                                                                                                           // Generator.java:1152
	{4005, "Subexpression of CASE must have form !i!j.", nil},                                                                                                                          // Generator.java:1207
	{4005, "Second selector for CASE subexpression must specify  one of two operands.", nil},                                                                                           // Generator.java:1231
	{4005, "Selecting OTHER in a CASE statement.", nil},                                                                                                                                // Generator.java:1240
	{4005, "Selecting subexpression of an EXCEPT not yet implemented.", nil},                                                                                                           // Generator.java:1268
	{4005, "Accessing non-existent subexpression of a SUFFICES", nil},                                                                                                                  // Generator.java:1418
	{4005, "Accessing ASSUME/PROVE clause within the scope of a declaration\n from outside that declaration's scope.", nil},                                                            // Generator.java:1439
	{4005, "Selected a subexpression of a NEW clause of an ASSUME.", nil},                                                                                                              // Generator.java:1457
	{4005, "Trying to select subexpression of an operator argument.", nil},                                                                                                             // Generator.java:1472
	{4005, "Cannot use !%s to select subexpression of a LAMBDA.", []ParameterKind{StringParameter}},                                                                                    // Generator.java:1480, String toString()
	{4005, "Selecting subexpression of expression that has none.", nil},                                                                                                                // Generator.java:1524
	{4005, "A name selector here must be from a LET clause.", nil},                                                                                                                     // Generator.java:1564
	{4005, "!: should not follow an operand selector.", nil},                                                                                                                           // Generator.java:1571
	{4005, "Selected ASSUME/PROVE instead of expression.", nil},                                                                                                                        // Generator.java:1608
	{4005, "Selected a NEW declaration as an expression or operator.", nil},                                                                                                            // Generator.java:1626
	{4005, "Selected operator argument when expression expected.", nil},                                                                                                                // Generator.java:1791
	{4005, "Nonexistent operand specified by `%s'.", []ParameterKind{StringParameter}},                                                                                                 // Generator.java:2115, String selectorItemToString
	{4005, "Choosing operand `%s' of subexpression with no operands.", []ParameterKind{StringParameter}},                                                                               // Generator.java:1395, String selectorItemToString
	{4005, "[]ASSUME matched by PROVE instead of []PROVE", nil},                                                                                                                        // Generator.java:3652
	{4005, "ASSUME matched by []PROVE instead of PROVE", nil},                                                                                                                          // Generator.java:3660
	{4005, "[]ASSUME used within the scope of an ordinary ASSUME's assumptions", nil},                                                                                                  // Generator.java:3693
	// Remaining codes.
	{4200, "DEF clause entry should describe a defined operator.", nil},                                             // Generator.java:1653 and 6697
	{4202, "Symbol %s is a built-in operator, and cannot be redefined.", []ParameterKind{ObjectIdentityParameter}},  // SymbolTable.java:145, UniqueString name
	{4203, "Operator name %s is incomplete.", []ParameterKind{StringParameter}},                                     // Generator.java:946, String toString()
	{4261, "@ used where its meaning is not defined.", nil},                                                         // Generator.java:3176
	{4261, "Attempted to declare '@' as a variable.", nil},                                                          // Generator.java:2390
	{4332, "%s", []ParameterKind{StringParameter}},                                                                  // Generator.java:7099, String res
	{4334, "Label not allowed within scope of declaration in nested ASSUME/PROVE.", nil},                            // Generator.java:6821
	{4335, "Labels inside EXCEPT clauses are not yet implemented.", nil},                                            // Generator.java:6837
	{4350, "<*> and <+> cannot be used for a named step.", nil},                                                     // Generator.java:5898
	{4351, "%s proof step selected instead of expression.", []ParameterKind{StringParameter}},                       // Generator.java:1889, String exprType
	{4357, "The only expression allowed as a fact in a HIDE is \nthe name of a theorem, assumption, or step.", nil}, // UseOrHideNode.java:100
}

func generationSourceSignature(code int, format string) ([]ParameterKind, bool) {
	for _, s := range generationSourceSignatures {
		if s.code == code && s.format == format {
			return s.kinds, true
		}
	}
	return nil, false
}
