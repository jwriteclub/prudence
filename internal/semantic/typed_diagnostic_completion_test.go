//go:build prudence_phase2_adapter

// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package semantic

import (
	"bytes"
	stdcontext "context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
	"github.com/jwriteclub/prudence/internal/modulegraph"
)

type typedRow struct {
	Code       int `json:"code"`
	Parameters []struct {
		Type string `json:"type"`
	} `json:"parameters"`
	Rendered string `json:"rendered"`
}

func javaTypes(kinds []ParameterKind) []string {
	out := []string{}
	for _, k := range kinds {
		switch k {
		case StringParameter:
			out = append(out, "java.lang.String")
		case ObjectIdentityParameter:
			out = append(out, "util.UniqueString")
		case IntParameter:
			out = append(out, "java.lang.Integer")
		default:
			out = append(out, "unexpected")
		}
	}
	return out
}

// checkTypedGeneration converts one produced detail and checks the Java
// classes against the pinned signature and the rendering against Render.
func checkTypedGeneration(t *testing.T, d Detail) {
	t.Helper()
	kinds, ok := generationSourceSignature(d.Code.Value, d.Format)
	if !ok {
		t.Fatalf("not an added signature: %d %q", d.Code.Value, d.Format)
	}
	b, err := Phase2TypedGenerationStageDiagnostics([]Detail{d})
	if err != nil {
		t.Fatalf("%d %q: %v %#v", d.Code.Value, d.Format, err, d.Parameters)
	}
	var rows []typedRow
	if err = json.Unmarshal(b, &rows); err != nil || len(rows) != 1 || rows[0].Code != d.Code.Value {
		t.Fatalf("%s %v", b, err)
	}
	want := javaTypes(kinds)
	if len(rows[0].Parameters) != len(want) {
		t.Fatalf("%s want %v", b, want)
	}
	for i := range want {
		if rows[0].Parameters[i].Type != want[i] {
			t.Fatalf("%s want %v", b, want)
		}
	}
	text, err := d.Render()
	if err != nil || rows[0].Rendered != text {
		t.Fatalf("rendered %q, Render %q %v", rows[0].Rendered, text, err)
	}
}

func findDetail(msgs []Detail, code int, format string) (Detail, bool) {
	for _, d := range msgs {
		if d.Code.Value == code && d.Format == format {
			return d, true
		}
	}
	return Detail{}, false
}

func singleModuleMessages(t *testing.T, source string) []Detail {
	t.Helper()
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, _ = g.module(parsedModule(t, source)) // Aborting producers still record their message.
	return g.table.errors.Messages()
}

func flatExternalMessages(t *testing.T, root string, modules map[string]string) []Detail {
	t.Helper()
	a, err := modulegraph.AcquireFlatExternal(stdcontext.Background(), modulegraph.Input{Path: "Target", Text: []byte(root)}, func(_ stdcontext.Context, n string) (modulegraph.Input, error) {
		text, ok := modules[n]
		if !ok {
			return modulegraph.Input{}, os.ErrNotExist
		}
		return modulegraph.Input{Path: n, Text: []byte(text)}, nil
	}, modulegraph.AcquisitionLimits{MaxDepth: 1000, MaxVisits: 10000, MaxUnits: 100, MaxEdges: 100, MaxNodes: 10000, MaxSourceBytes: 10240, MaxTotalBytes: 102400})
	if err != nil {
		t.Fatal(err)
	}
	msgs, _, _, _ := analyzeFlatExternalOwned(stdcontext.Background(), a, 1000)
	return msgs
}

const typedModule = "---- MODULE M ----\n"

// Each case reaches the implemented producer from source and converts its
// recorded detail through the generation typed bridge.
func TestTypedGenerationReachedProducers(t *testing.T) {
	for _, tc := range []struct {
		code   int
		format string
		source string
	}{
		{4003, "Unexpected token found.", typedModule + "op == {<<x, y>> \\in Nat}\n====\n"},
		{4003, "Was expecting a GeneralId.", typedModule + "F(x) == x\nTHEOREM TRUE BY DEF F(1)\nG == TRUE\n====\n"},
		{4003, "Internal error: IdPrefixElement has other than 2 or 3 heirs.", typedModule + "F(x) == x\nTHEOREM TRUE BY DEF I!F(1)\n====\n"},
		{4003, "Internal error: No context available for module `%s'.", typedModule + "THEOREM TRUE\n<1>1. INSTANCE A\n<1>2. DEFINE I == INSTANCE A\n<1>3. I!F BY I!T DEF I!F\n<1> QED OBVIOUS\n====\n"},
		{4004, "%s", typedModule + "THEOREM TRUE\n<1>1. USE MODULE M\n<1>2. TRUE BY <1>1\n<1> QED OBVIOUS\nF == TRUE\n====\n"},
		{4004, "Couldn't resolve infix operator symbol `%s'.", "---- MODULE Aux ----\nCONSTANT c\nTHEOREM Th == c > 0\n====\n"},
		{4004, "Couldn't resolve prefix operator symbol `%s'.", typedModule + "A == -1\n====\n"},
		{4004, "Couldn't resolve postfix operator symbol `%s'.", typedModule + "A == 1^+\n====\n"},
		{4004, "DEF clause entry refers to a non-definition step.", typedModule + "THEOREM TRUE\n<1>1. USE TRUE\n<1>2. TRUE BY DEF <1>1\n<1>3. TRUE\n<1> QED OBVIOUS\n====\n"},
		{4004, "Expected %s arguments but found %s.", typedModule + "F == \\A x \\in {1}: x = 1\nA == F!@\n====\n"},
		{4004, "Operator used with the wrong number of arguments.", typedModule + "CONSTANT Op(_)\nApply(P(_)) == P(1)\nF(x) == Apply(Op)\nUse(Q(_,_)) == Q(1, 2)\nA == Use(F!1)\n====\n"},
		{4005, "Selector `%s' should not have argument(s).", typedModule + "F(x) == x\nTHEOREM TRUE BY DEF F(1)\nG == TRUE\n====\n"},
		{4005, "Need name or step number here, not `%s'.", typedModule + "op == {<<x, y>> \\in Nat}\n====\n"},
		{4005, "Need name or step number here, not `%s'.", typedModule + "HIDE DEF r.f\n====\n"},
		{4005, "Selector `%s' should not have argument(s).", typedModule + "HIDE DEF x \\cup y \\cup z \\cup w\n====\n"},
		{4005, "Cannot take subexpression of `%s'.", typedModule + "Apply(P(_)) == P(1)\nA == Apply(STRING!mark)\n====\n"},
		{4005, "Subexpression of  `%s' used inside the operator's definition.", typedModule + "THEOREM T == ASSUME <<LET RECURSIVE G(_) IN TRUE>> PROVE TRUE\nApply(Op(_)) == Op(1)\nA == T!1!1!G(Missing)\nB == Apply(T!1!1!G)\n====\n"},
		{4005, "Accessing subexpression labeled `%s' of ASSUME/PROVE clause within the scope of a declaration\n from outside that declaration's scope.", typedModule + "THEOREM T == ASSUME L::TRUE, NEW x, P::TRUE PROVE TRUE\nA == T!1\nB == T!L\nC == T!P\nD == T!3\n====\n"},
		{4005, "`!:' can be used only after a name and either at the end after an\noperator name or before an operator name.", typedModule + "G(x) == <<x, 2>>\nA == G(1)!1!:\nB == G(1)!:!1\n====\n"},
		{4005, "Subexpression selection failed, probably due to an error in the selected expression.", typedModule + "F == 1 + 2\nA == F!1!1\n====\n"},
		{4005, "A LET/IN expression has only one operand.", typedModule + "F == LET X == 1 IN X\nA == F!1\nB == F!2\nG == 2\nC == G!1\n====\n"},
		{4005, "Subexpression of CASE must have form !i!j.", typedModule + "CONSTANT f\nC == CASE TRUE -> 1 [] OTHER -> 2\nCC == C!1\n====\n"},
		{4005, "Second selector for CASE subexpression must specify  one of two operands.", typedModule + "F == CASE TRUE -> 1 [] OTHER -> 2\nA == F!1!G\n====\n"},
		{4005, "Selecting OTHER in a CASE statement.", typedModule + "F == CASE TRUE -> 1 [] OTHER -> 2\nA == F!2!1!G\n====\n"},
		{4003, "Unsupported expression type `%s'.", typedModule + "op == [1, 2, 3]\n====\n"},
		{4005, "Choosing operand `%s' of subexpression with no operands.", typedModule + "VARIABLE v\nOp == v\nA == Op!1\n====\n"},
		{4005, "Selecting subexpression of an EXCEPT not yet implemented.", typedModule + "F == [[a |-> 1] EXCEPT !.a = 2]\nA == F!2!G\n====\n"},
		{4005, "Accessing non-existent subexpression of a SUFFICES", typedModule + "THEOREM TRUE\n<1>1. SUFFICES ASSUME TRUE PROVE TRUE\n<1>2. TRUE BY <1>1!1!2, <1>1!@\n<1> QED OBVIOUS\nAfter == TRUE\n====\n"},
		{4005, "Accessing ASSUME/PROVE clause within the scope of a declaration\n from outside that declaration's scope.", typedModule + "THEOREM T == ASSUME L::TRUE, NEW x, P::TRUE PROVE TRUE\nA == T!1\nB == T!L\nC == T!P\nD == T!3\n====\n"},
		{4005, "Selected a subexpression of a NEW clause of an ASSUME.", typedModule + "THEOREM T == ASSUME NEW x, TRUE PROVE TRUE\nApply(Op(_)) == Op(1)\nA == T!1\nB == T!1!L\nC == Apply(T!1)\n====\n"},
		{4005, "Trying to select subexpression of an operator argument.", typedModule + "Apply(Op(_)) == Op(1)\nH(x) == x\nF == Apply(LAMBDA x: x)\nN == Apply(H)\nA == F!1!(2)\nB == F!1!(2, 3)\nC == F!1!1\nD == N!1!(2)\n====\n"},
		{4005, "Cannot use !%s to select subexpression of a LAMBDA.", typedModule + "Apply(Op(_)) == Op(1)\nH(x) == x\nF == Apply(LAMBDA x: x)\nN == Apply(H)\nA == F!1!(2)\nB == F!1!(2, 3)\nC == F!1!1\nD == N!1!(2)\n====\n"},
		{4005, "Selecting subexpression of expression that has none.", typedModule + "F == LET X == 1 IN X\nA == F!1\nB == F!2\nG == 2\nC == G!1\n====\n"},
		{4005, "A name selector here must be from a LET clause.", typedModule + "F == <<TRUE, FALSE>>\nTHEOREM TRUE BY F!1!Missing\n====\n"},
		{4005, "!: should not follow an operand selector.", typedModule + "G(x) == <<x, 2>>\nA == G(1)!1!:\nB == G(1)!:!1\n====\n"},
		{4005, "Selected ASSUME/PROVE instead of expression.", typedModule + "THEOREM T == ASSUME L:: ASSUME TRUE PROVE TRUE PROVE TRUE\nApply(Op(_)) == Op(1)\nA == Apply(T!L)\nB == Apply(T!L!:)\n====\n"},
		{4005, "Selected a NEW declaration as an expression or operator.", typedModule + "THEOREM T == ASSUME NEW x, TRUE PROVE TRUE\nApply(Op(_)) == Op(1)\nA == T!1\nB == T!1!L\nC == Apply(T!1)\n====\n"},
		{4005, "Selected operator argument when expression expected.", typedModule + "Apply(Op(_)) == Op(1)\nH(x) == x\nF == Apply(H)\nA == F!1\nB == Apply(F!1)\n====\n"},
		{4005, "Nonexistent operand specified by `%s'.", typedModule + "F == <<1,2>>\nA == F!(1)\nB == F!@\n====\n"},
		{4005, "[]ASSUME matched by PROVE instead of []PROVE", typedModule + "THEOREM []ASSUME TRUE PROVE TRUE\n====\n"},
		{4005, "ASSUME matched by []PROVE instead of PROVE", typedModule + "THEOREM ASSUME TRUE []PROVE TRUE\n====\n"},
		{4005, "[]ASSUME used within the scope of an ordinary ASSUME's assumptions", typedModule + "THEOREM ASSUME []ASSUME TRUE []PROVE TRUE PROVE TRUE\n====\n"},
		{4200, "DEF clause entry should describe a defined operator.", typedModule + "F == <<<<1>>>>\nTHEOREM TRUE BY DEF F!1!1, F!3\n====\n"},
		{4202, "Symbol %s is a built-in operator, and cannot be redefined.", typedModule + "~ x == x\ny ^+ == y\nA == ~ TRUE\nB == 2^+\n====\n"},
		{4203, "Operator name %s is incomplete.", typedModule + "---- MODULE Inner ----\nX == 1\n====\nI == INSTANCE Inner\nA == I\n====\n"},
		{4203, "Operator name %s is incomplete.", typedModule + "---- MODULE Inner ----\nCONSTANT c\nX == c\n====\nI(y) == INSTANCE Inner WITH c <- y\nA == I(1)\n====\n"},
		{4261, "@ used where its meaning is not defined.", typedModule + "CONSTANT f\nA == [f EXCEPT ![1] = @, !.a = @]\nB == @\n====\n"},
		{4261, "Attempted to declare '@' as a variable.", typedModule + "VARIABLE @\n====\n"},
		{4332, "%s", typedModule + "CONSTANT C\nQ == \\A x \\in {1}: mark(x, x):: x = x\nA == other(C):: 1\n====\n"},
		{4334, "Label not allowed within scope of declaration in nested ASSUME/PROVE.", typedModule + "THEOREM T == ASSUME ASSUME NEW x, L::TRUE PROVE TRUE PROVE TRUE\n====\n"},
		{4335, "Labels inside EXCEPT clauses are not yet implemented.", typedModule + "Q == \\A x \\in {1}: mark:: x = x\nA == <<dup:: 1, dup:: 2>>\nCONSTANT f\nB == [f EXCEPT ![1] = bad:: 2]\n====\n"},
		{4350, "<*> and <+> cannot be used for a named step.", typedModule + "THEOREM TRUE\n<1>1. TRUE\n<*>Named. TRUE\n<1>2. TRUE BY <1>Named\n<1> QED OBVIOUS\n====\n"},
		{4351, "%s proof step selected instead of expression.", typedModule + "THEOREM TRUE\n<1>1. HAVE TRUE\n<1>2. <1>1\n<1>3. TRUE BY <1>1\n<1> QED OBVIOUS\n====\n"},
		{4357, "The only expression allowed as a fact in a HIDE is \nthe name of a theorem, assumption, or step.", typedModule + "THEOREM TRUE\n<1>1. DEFINE Foo == TRUE\n<1>2. USE Foo DEF <1>1\n<1>3. HIDE Foo\n<1> QED OBVIOUS\n====\n"},
	} {
		d, ok := findDetail(singleModuleMessages(t, tc.source), tc.code, tc.format)
		if !ok {
			t.Errorf("producer not reached: %d %q", tc.code, tc.format)
			continue
		}
		checkTypedGeneration(t, d)
	}
}

func TestTypedGenerationReachedInstanceProducers(t *testing.T) {
	inner, err := os.ReadFile("testdata/instance_diagnostics/Inner.tla")
	if err != nil {
		t.Fatal(err)
	}
	msgs := flatExternalMessages(t, "---- MODULE Target ----\nINSTANCE Inner WITH F <- -, k <- 1\n====\n", map[string]string{"Inner": string(inner)})
	d, ok := findDetail(msgs, 4003, "Substitution error, probably due to error \nin module being instantiated.")
	if !ok {
		t.Fatalf("SubstInNode producer not reached: %#v", msgs)
	}
	checkTypedGeneration(t, d)
	msgs = flatExternalMessages(t, "---- MODULE Target ----\nI == INSTANCE Aux WITH c <- 1\n====\n", map[string]string{"Aux": "---- MODULE Aux ----\nCONSTANT c\nTHEOREM Th == c > 0\n====\n"})
	d, ok = findDetail(msgs, 4004, "Substitution error, probably due to error in module being instantiated.")
	if !ok {
		t.Fatalf("APSubstInNode producer not reached: %#v", msgs)
	}
	checkTypedGeneration(t, d)
}

// The label invariant producer is reached through its owning method, as in
// TestIllegalLabelInvariantAborts; no source tree violates it.
func TestTypedGenerationReachedLabelInvariant(t *testing.T) {
	tree := parsedModule(t, typedModule+"THEOREM T == TRUE\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	module, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	_, err = g.illegalProofLabelRef(&labelNode{goal: module.theorems[0].definition}, tree)
	var abort *Abort
	d, ok := findDetail(g.table.errors.Messages(), 4003, "Internal error: Expecting label to be in AssumeProveNode, but it's not.")
	if !errors.As(err, &abort) || !ok {
		t.Fatalf("%v %#v", err, g.table.errors.Messages())
	}
	checkTypedGeneration(t, d)
}

// The selected-null producer is reached through numericExpression after the
// operand is cleared, as in TestSelectedNullOperandUsesSourceSelector.
func TestTypedGenerationReachedNullSelection(t *testing.T) {
	tree := parsedModule(t, typedModule+"F == <<1>>\nR == F!1\n====\n")
	g, _ := newGenerator(stdcontext.Background(), 0)
	m, err := g.module(tree)
	if err != nil {
		t.Fatal(err)
	}
	f := m.context.lookup(contextKey{name: g.pool.intern("F")}).(*userOpDefNode)
	f.body.(*opApplNode).operands[0] = nil
	var find func(*generated.SyntaxTreeNode) *generated.SyntaxTreeNode
	find = func(n *generated.SyntaxTreeNode) *generated.SyntaxTreeNode {
		if n.Kind() == generated.N_StructOp {
			return n
		}
		for _, c := range n.Heirs() {
			if got := find(c); got != nil {
				return got
			}
		}
		return nil
	}
	selector := find(tree)
	if selector == nil {
		t.Fatal("missing selector")
	}
	if _, err = g.numericExpression(tree, []*generated.SyntaxTreeNode{selector}, f, f.body, nil, nil, 1, true); err != nil {
		t.Fatal(err)
	}
	d, ok := findDetail(g.table.errors.Messages(), 4003, "An unexpected null node specified by '%s'.\nThis is probably due to a previous error.")
	if !ok {
		t.Fatalf("%#v", g.table.errors.Messages())
	}
	checkTypedGeneration(t, d)
}

// Before the repair the selector producer recorded a String with no Java
// class or UTF-16 provenance, which the bridge rejected. It now carries the
// image's source units; the recorded message, location, order and rendering
// are pinned to the pre-repair values.
func TestTypedGenerationUnexpectedSelectorProvenance(t *testing.T) {
	g, _ := newGenerator(stdcontext.Background(), 0)
	_, err := g.module(parsedModule(t, "---- MODULE Target ----\nUSE Expr!\u27e9\n====\n"))
	var abort *Abort
	msgs := g.table.errors.Messages()
	if !errors.As(err, &abort) || len(msgs) != 1 {
		t.Fatalf("%v %#v", err, msgs)
	}
	d := msgs[0]
	const rendered = "Internal error: Unexpected selector `\u27e9'."
	text, rerr := d.Render()
	if d.Code.Value != 4003 || d.Format != "Internal error: Unexpected selector `%s'." || !sameLocation(d.Location, Location{Name: "Target", NamePresent: true, BeginLine: 2, BeginColumn: 10, EndLine: 2, EndColumn: 10}) || rerr != nil || text != rendered || abort.Error() != rendered {
		t.Fatalf("legacy record changed: %#v %q %v %q", d, text, rerr, abort.Error())
	}
	p := d.Parameters[0]
	if len(d.Parameters) != 1 || p.Kind != StringParameter || p.Text != "\u27e9" || p.JavaClass != "java.lang.String" || !p.UTF16Present || p.TextUnrepresentable || len(p.UTF16Units) != 1 || p.UTF16Units[0] != 0x27e9 {
		t.Fatalf("parameter %#v", p)
	}
	checkTypedGeneration(t, d)
	b, err := Phase2TypedGenerationStageDiagnostics(msgs)
	if err != nil || !bytes.Contains(b, []byte(`{"type":"java.lang.String","value":"\u27e9"}`)) || !bytes.Contains(b, []byte(`"rendered":"\u0049`)) {
		t.Fatalf("%s %v", b, err)
	}
	before := copyDetail(d)
	before.Parameters[0] = Parameter{Kind: StringParameter, Text: "\u27e9"}
	if _, err = Phase2TypedGenerationStageDiagnostics([]Detail{before}); err != ErrPortIncomplete {
		t.Fatalf("pre-repair parameter admitted: %v", err)
	}
	if text, rerr = before.Render(); rerr != nil || text != rendered {
		t.Fatalf("pre-repair rendering %q %v", text, rerr)
	}
}

// Signatures excluded because Java passes a Location or semantic-node object,
// which the Java observer cannot type, stay incomplete when reached.
func TestTypedGenerationUnsupportedJavaObjectsRemainIncomplete(t *testing.T) {
	d, ok := findDetail(singleModuleMessages(t, typedModule+"Apply(Op(_,_)) == Op(1, 2)\nA == Apply(\\cup)\nB == Apply(\\oplus)\nUnary(Op(_)) == Op(1)\nC == Unary(\\cup)\n====\n"), 4004, "Operator with incorrect arity passed as argument. \nOperator '%s' of arity %s is argument number %s (counting from 1) to operator `%s', \nbut an operator of arity %s was expected.")
	if !ok {
		t.Fatal("producer not reached")
	}
	if _, err := Phase2TypedGenerationStageDiagnostics([]Detail{d}); err != ErrPortIncomplete {
		t.Fatalf("semantic node admitted: %v", err)
	}
	p := newNamePool()
	for _, x := range []Detail{
		{Code: mustCode(t, 4800), Format: "Warning: the %s of '%s' conflicts with \nits %s at %s.", Parameters: []Parameter{exactStringParam("definition"), nameStringParam(p.intern("x")), exactStringParam("definition"), locParam(Location{Name: "A", NamePresent: true, BeginLine: 1})}},
		{Code: mustCode(t, 4224), Format: "The %s of '%s' conflicts with \nits %s at %s.", Parameters: []Parameter{exactStringParam("definition"), nameStringParam(p.intern("x")), exactStringParam("declaration"), locParam(Location{Name: "A", NamePresent: true, BeginLine: 1})}},
		{Code: mustCode(t, 4240), Format: "Substitution missing for symbol %s declared at %s \nand instantiated in module %s.", Parameters: []Parameter{nameParam(p.intern("c")), locParam(Location{Name: "A", NamePresent: true, BeginLine: 1}), nameParam(p.intern("M"))}},
	} {
		x.Location = Location{Name: "M", NamePresent: true}
		if _, err := Phase2TypedGenerationStageDiagnostics([]Detail{x}); err != ErrPortIncomplete {
			t.Fatalf("location object admitted: %d %v", x.Code.Value, err)
		}
	}
	level := Detail{Code: mustCode(t, 4315), Location: Location{Name: "M", NamePresent: true}, Format: "Level error in applying operator %s:\nThe level of the range for the bounded variable %s \nexceeds the maximum level allowed by the operator.", Parameters: []Parameter{nameParam(p.intern("\\E")), {Kind: SemanticNodeParameter, Identity: 9, Text: "x", UTF16Units: []uint16{'x'}, UTF16Present: true}}}
	if _, err := Phase2TypedLevelStageDiagnostics([]Detail{level}); err != ErrPortIncomplete {
		t.Fatalf("level semantic node admitted: %v", err)
	}
}

func mustCode(t *testing.T, value int) Code {
	t.Helper()
	c, ok := CodeFromValue(value)
	if !ok {
		t.Fatalf("code %d", value)
	}
	return c
}

func validSignatureDetail(p *namePool, code Code, format string, kinds []ParameterKind) Detail {
	d := Detail{Code: code, Location: Location{Name: "M", NamePresent: true, BeginLine: 1, BeginColumn: 1, EndLine: 1, EndColumn: 2}, Format: format}
	for i, k := range kinds {
		switch k {
		case StringParameter:
			d.Parameters = append(d.Parameters, nameStringParam(p.intern("s")))
		case ObjectIdentityParameter:
			d.Parameters = append(d.Parameters, nameParam(p.intern("u")))
		case IntParameter:
			d.Parameters = append(d.Parameters, Parameter{Kind: IntParameter, JavaClass: "java.lang.Integer", Integer: int64(i + 1)})
		}
	}
	return d
}

// Direct controls cover every added signature, including producers the
// existing admission cannot reach from source (for example the proof-step
// SyntaxTreeNode kind abort and the labeled ASSUME/PROVE expression check).
func TestTypedGenerationSignatureControls(t *testing.T) {
	p := newNamePool()
	for _, s := range generationSourceSignatures {
		code := mustCode(t, s.code)
		d := validSignatureDetail(p, code, s.format, s.kinds)
		checkTypedGeneration(t, d)
		reject := func(what string, x Detail) {
			t.Helper()
			if _, err := Phase2TypedGenerationStageDiagnostics([]Detail{x}); err != ErrPortIncomplete {
				t.Errorf("%d %q %s admitted: %v", s.code, s.format, what, err)
			}
		}
		x := copyDetail(d)
		x.Format += " "
		reject("wrong format", x)
		x = copyDetail(d)
		other := 4003
		if s.code == 4003 {
			other = 4004
		}
		x.Code = mustCode(t, other)
		reject("wrong code", x)
		x = copyDetail(d)
		x.Parameters = append(x.Parameters, Parameter{Kind: IntParameter, Integer: 1})
		reject("extra argument", x)
		if len(d.Parameters) == 0 {
			continue
		}
		x = copyDetail(d)
		x.Parameters = x.Parameters[:len(x.Parameters)-1]
		reject("missing argument", x)
		for i, k := range s.kinds {
			x = copyDetail(d)
			switch k {
			case StringParameter:
				x.Parameters[i] = nameParam(p.intern("s"))
				reject("UniqueString for String", x)
				x.Parameters[i] = strParam("s")
				reject("String without UTF-16 provenance", x)
				x.Parameters[i] = nameStringParam(p.intern("s"))
				x.Parameters[i].JavaClass = ""
				reject("String without Java class", x)
				x.Parameters[i] = Parameter{Kind: LocationParameter, Location: d.Location}
				reject("Location object", x)
				// Unpaired UTF-16 from the source is carried exactly.
				x.Parameters[i] = nameStringParam(p.internUnits([]uint16{0xd800}))
				b, err := Phase2TypedGenerationStageDiagnostics([]Detail{x})
				if err != nil || !bytes.Contains(b, []byte(`\ud800`)) {
					t.Errorf("%d %q unpaired UTF-16: %s %v", s.code, s.format, b, err)
				}
			case ObjectIdentityParameter:
				x.Parameters[i] = nameStringParam(p.intern("u"))
				reject("String for UniqueString", x)
				x.Parameters[i] = Parameter{Kind: SemanticNodeParameter, Identity: 7, Text: "u", UTF16Units: []uint16{'u'}, UTF16Present: true}
				reject("semantic node", x)
				x.Parameters[i] = nameParam(p.intern("u"))
				x.Parameters[i].Identity = 0
				reject("UniqueString without identity", x)
			case IntParameter:
				x.Parameters[i] = Parameter{Kind: LongParameter, Integer: 1}
				reject("Long for Integer", x)
				x.Parameters[i] = Parameter{Kind: IntParameter, Integer: 1 << 31}
				reject("Integer overflow", x)
				x.Parameters[i] = Parameter{Kind: IntParameter, JavaClass: "java.lang.Long", Integer: 1}
				reject("contradictory Integer class", x)
				x.Parameters[i] = Parameter{Kind: BoolParameter, Boolean: true}
				reject("Boolean for Integer", x)
			}
		}
	}
	// A recognized signature does not admit an unrelated diagnostic beside it.
	d := validSignatureDetail(p, mustCode(t, 4005), "Selecting OTHER in a CASE statement.", nil)
	unknown := validSignatureDetail(p, mustCode(t, 4004), "Module instantiation selected instead of expression.", nil)
	if _, err := Phase2TypedGenerationStageDiagnostics([]Detail{d, unknown}); err != ErrPortIncomplete {
		t.Fatalf("unimplemented producer admitted: %v", err)
	}
}
