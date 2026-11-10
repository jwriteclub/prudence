// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package prudence

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// LET-local INSTANCE: ParseUnit.getInstances finds the INSTANCE of a LET
// module definition in preorder (ParseUnit.java:660-690), and
// Generator.processLetIn hands the definition to processModuleDefinition with
// the LET's own defVec and instVec (Generator.java:2952-3044, 4739-5072).
// LetInNode.levelCheck checks the instances after the body
// (LetInNode.java:114-197).

type letSANYError struct {
	module    string
	line, col int
	message   string
}

type letCorpusCase struct {
	id, set, class, root string
	files                map[string]string
	want                 []letSANYError
}

// letCorpusCases are recorded sany-conformance cases (case directory files
// verbatim; want is the recorded stock-SANY output as compare normalises
// it: sorted by line, column and text, whitespace runs collapsed, 0:0 where
// SANY prints no position). All of them stopped at acquisition before this
// chapter.
var letCorpusCases = []letCorpusCase{
	{
		id: "E-9cbaf8e2009f", set: "deep-cuts/model-pit-survivors", class: "ok", root: "Target.tla",
		files: map[string]string{
			"Target.tla": `\* Copyright 2026 Christopher O'Connell
\* All rights reserved
\* Aim: tla2sany/semantic/LetInNode.java:230:MathMutator (model @cf/zai-org/glm-5.3)
---- MODULE Target ----
EXTENDS Naturals

CONSTANT x

Foo == [ x |-> LET M == INSTANCE Naturals IN 42 ]
====
`,
		},
	},
	{
		id: "E-63b54cee5a46", set: "model-one-shot", class: "ok", root: "Target.tla",
		files: map[string]string{
			"Sub.tla": `---- MODULE Sub ----
A == TRUE
====
`,
			"Target.tla": `---- MODULE Target ----
op == LET LOCAL M == INSTANCE Sub IN M!A
====
`,
		},
	},
	{
		id: "B-org-lamport-tlatools-test-model-letinsta-de41f0", set: "tlaplus-test-models", class: "ok", root: "LetInstanceThmXml.tla",
		files: map[string]string{
			"LetInstanceThmXml.tla": `---- MODULE LetInstanceThmXml ----
op == LET I == INSTANCE LetInstanceThmXmlDefs IN TRUE
====
`,
			"LetInstanceThmXmlDefs.tla": `---- MODULE LetInstanceThmXmlDefs ----
THEOREM Thm == TRUE
ASSUME Asm == TRUE
====
`,
		},
	},
	{
		id: "B-org-lamport-tlatools-test-model-github14-65e851", set: "tlaplus-test-models", class: "ok", root: "Github1417.tla",
		files: map[string]string{
			"Github1417.tla": `---- MODULE Github1417 ----
op == LET M == INSTANCE Github1417Empty IN TRUE
====
`,
			"Github1417Empty.tla": `---- MODULE Github1417Empty ----
====
`,
		},
	},
	{
		id: "E-10a2d56e7f64", set: "deep-cuts/model-one-shot", class: "ok", root: "Target.tla",
		files: map[string]string{
			"Target.tla": `---- MODULE Target ----
EXTENDS Naturals

---- MODULE Inner ----
CONSTANT cc
f == cc
====

A == LET B == INSTANCE Inner WITH cc <- 1 IN B!f

C == LET D == 2 E == INSTANCE Inner WITH cc <- D IN E!f

F == LET G == LET H == INSTANCE Inner WITH cc <- 3 IN H!f IN G

I == INSTANCE Inner WITH cc <- 4

J == LET K == 5 IN K

P == LET R(x) == INSTANCE Inner WITH cc <- x IN R(6)!f

Z == A + C + F + I!f + J + P
====
`,
		},
	},
	{
		id: "E-c0b3580b1006", set: "inspection-pit-survivors", class: "ok", root: "Other19.tla",
		files: map[string]string{
			"Other19.tla": `\* Copyright 2026 Christopher O'Connell
\* All rights reserved
\* Aim: Linter.java:173 RemoveConditional_EQUAL_IF: any Naturals op counts as Nat; a*b's stub uses no parameter, so warning for g1 goes
\* Aim: Linter.java:178 RemoveConditional_EQUAL_IF: any Reals op counts: warning for g2 (Real, upstream tests the name "Reals") goes
\* Aim: OpApplNode.java:1179 RemoveConditional_ORDER_ELSE: ranges not among children: constant c unseen: warning for g3 appears
\* Aim: LetInNode.java:237 RemoveConditional_ORDER_ELSE: INSTANCEs not among children: warning for g4 appears
\* Aim: LetInNode.java:238 MathMutator: second INSTANCE overwrites the last LET def z: warning for g6 appears (unsure of the array order)
\* Aim: Linter.java:102 EmptyObjectReturn: records of extended modules ignored: warning for g5 appears. Helpers: Other19a, Other19b, Other19c
---- MODULE Other19 ----
EXTENDS Naturals, Reals, Other19a
CONSTANT c
g1 == 1
g2 == 1
g3 == 1
g4 == 1
g5 == 1
R1 == [g1 |-> 2 * 3]
R2 == [g2 |-> Real]
R3 == [g3 |-> {i \in c : TRUE}]
R4 == [g4 |-> LET J1 == INSTANCE Other19b WITH p <- 1
                  J2 == INSTANCE Other19c WITH q <- c
              IN 1]
R5 == [g5 |-> 1]
g6 == 1
R6 == [g6 |-> LET J1 == INSTANCE Other19b WITH p <- 1
                  J2 == INSTANCE Other19c WITH q <- 1
                  z == c
              IN 1]
====
`,
			"Other19a.tla": `\* Copyright 2026 Christopher O'Connell
\* All rights reserved
---- MODULE Other19a ----
CONSTANT d
S == [g5 |-> d]
====
`,
			"Other19b.tla": `\* Copyright 2026 Christopher O'Connell
\* All rights reserved
---- MODULE Other19b ----
CONSTANT p
====
`,
			"Other19c.tla": `\* Copyright 2026 Christopher O'Connell
\* All rights reserved
---- MODULE Other19c ----
CONSTANT q
====
`,
		},
	},
	{
		id: "B-tests-nested-counter-f9cec7", set: "tlaplus-communitymodules", class: "ok", root: "Counter.tla",
		files: map[string]string{
			"Counter.tla": `----- MODULE Counter -----
EXTENDS Naturals

ASSUME LET T == INSTANCE TLC IN T!PrintT("IOUtilsTests!C!b!Counter")

CONSTANT UpperBound

VARIABLE x

Init ==
    /\ x = 0

Next ==
    /\ x' = x + 1

Spec ==
    Init /\ [][Next]_x

Constraint ==
    /\ x < UpperBound

Inv ==
    /\ x < UpperBound

==========================`,
		},
	},
	{
		id: "B-tlapm-let-in-004", set: "tlapm", class: "ok", root: "Test.tla",
		files: map[string]string{
			"Test.tla": `
---- MODULE Test ----
EXTENDS TestPrelude
op ==
  /\ LET M1 == INSTANCE M2 IN P
  /\ Q
====
---- MODULE M2 ----
====
---- MODULE TestPrelude ----
P == TRUE
Q == TRUE
====

`,
		},
	},
	{
		id: "B-org-lamport-tlatools-test-model-cyclicre-2b2a6b", set: "tlaplus-test-models", class: "ok", root: "CyclicRedefineInstance.tla",
		files: map[string]string{
			"CyclicRedefineInstance.tla": `---- MODULE CyclicRedefineInstance ----
EXTENDS Base

ReDefNext ==
	LET B == INSTANCE Base
	IN \/ B!Next
	   \/ x' = TRUE
=====
----- MODULE Base -----
VARIABLE x

Next ==
    x' = FALSE

Spec ==
	x = FALSE /\ [][Next]_x

Inv ==
	x = FALSE
====
`,
		},
	},
	{
		id: "B-ts-let-in-004", set: "tree-sitter-tlaplus", class: "missing-module", root: "Test.tla",
		files: map[string]string{
			"Test.tla": `
---- MODULE Test ----
op ==
  /\ LET M1 == INSTANCE M2 IN P
  /\ Q
====

`,
		},
		want: []letSANYError{
			{"Test", 0, 0, "Cannot find source file for module M2 imported in module Test."},
		},
	},
	{
		id: "E-ef2080f1f245", set: "model-one-shot", class: "module-cycle", root: "Target.tla",
		files: map[string]string{
			"Target.tla": `---- MODULE Target ----
EXTENDS Naturals

Foo ==
  LET compound == 1 + 2 IN
  LET nested(a) == LET inner == a + 1 IN inner IN
  nested(compound)

Bar ==
  LET localmod == INSTANCE Target IN
  localmod!Foo

====
`,
		},
		want: []letSANYError{
			{"", 0, 0, "Circular dependency among .tla files; dependency cycle is: Target.tla --> Target.tla"},
		},
	},
	{
		id: "A-fix-sany-nestedmoduleletinstance", set: "tlaplus-test-models", class: "semantic-error", root: "NestedModuleLetInstance.tla",
		files: map[string]string{
			"NestedModuleLetInstance.tla": `---- MODULE NestedModuleLetInstance ----
---- MODULE LetInner ----
foo == 42
====
op == LET Inst == INSTANCE LetInner IN Inst!foo
====
`,
		},
		want: []letSANYError{
			{"NestedModuleLetInstance", 2, 1, "Multiply-defined module 'LetInner': this definition or declaration conflicts with the one at line 1, col 1 to line 3, col 4 of module LetInner."},
		},
	},
	{
		id: "E-1302e0ae550a", set: "inspection-pit-survivors", class: "semantic-error", root: "Gap19.tla",
		files: map[string]string{
			"Gap19.tla": `\* Copyright 2026 Christopher O'Connell
\* All rights reserved
\* Aim: Generator.java:5039 VoidMethodCall: LET INSTANCE not in LetInNode.insts, so the level error on c <- v goes unreported; helper Gap20
---- MODULE Gap19 ----
VARIABLE v, w
B == LET M == INSTANCE Gap20 WITH c <- v IN M!A
====
`,
			"Gap20.tla": `\* Copyright 2026 Christopher O'Connell
\* All rights reserved
\* Aim: Generator.java:5039 VoidMethodCall: helper of Gap19; LET INSTANCE of an inner module is rejected
---- MODULE Gap20 ----
CONSTANT c
VARIABLE w
A == c
====
`,
		},
		want: []letSANYError{
			{"Gap19", 6, 10, "Level error in instantiating module 'Gap20': The level of the expression or operator substituted for 'c' must be at most 0."},
		},
	},
	{
		id: "D-0ec2dfbd0ece", set: "deep-cuts/fuzz-jazzer", class: "semantic-error", root: "INSTANCE_shift_due_to_omitted_modunits_test.tla",
		files: map[string]string{
			"INSTANCE_shift_due_to_omitted_modunits_test.tla": `---- MODULE INSTANCE_shift_due_to_omitted_modunits_test ----


---- MODULE Inner ----
VARIABLE y

A == y= []TRUE
======================


VARIABLE z


M == INSTANCE Inner WITH y <- z


THEOREM
    \EE x:
        LET Q == INSTANCE Inner WITH y <- x
        IN Q!A
OBVIOUS


======================='
THEOREM B == []TRUE
======================


`,
		},
		want: []letSANYError{
			{"INSTANCE_shift_due_to_omitted_modunits_test", 7, 6, "Level error in applying operator =: The level of argument 2 exceeds the maximum level allowed by the operator."},
		},
	},
	{
		id: "E-6743b429dd14", set: "inspection-pit-survivors", class: "semantic-error", root: "Other22.tla",
		files: map[string]string{
			"Other22.tla": `\* Copyright 2026 Christopher O'Connell
\* All rights reserved
\* Aim: LetInNode.java:127 RemoveConditional_EQUAL_ELSE: LET stays level-correct after a bad definition: E1 gets a second error from the outer prime
\* Aim: LetInNode.java:131 RemoveConditional_EQUAL_ELSE: same for a bad body: E2
\* Aim: LetInNode.java:135 RemoveConditional_EQUAL_ELSE: same for a bad INSTANCE: E3
\* Aim: InstanceNode.java:148 RemoveConditional_ORDER_ELSE: bad substitution not noted: E3
\* Aim: InstanceNode.java:149 RemoveConditional_EQUAL_ELSE: same: E3
\* Aim: InstanceNode.java:145 RemoveConditional_EQUAL_ELSE: bad instantiated module not noted: E4
\* Aim: OpApplNode.java:472 RemoveConditional_EQUAL_ELSE: bad range not noted: E5
\* Aim: OpDefNode.java:1177 BooleanTrueReturn: bad definition reported as fine: E6 gets an argument level error
\* Aim: SubstInNode.java:328 RemoveConditional_EQUAL_ELSE: same through an instantiated definition: E7
\* Aim: SubstInNode.java:457 BooleanTrueReturn: same: E7
\* Aim: SubstInNode.java:332 RemoveConditional_EQUAL_ELSE: bad substitution: E8
\* Aim: APSubstInNode.java:319 RemoveConditional_EQUAL_ELSE: bad ASSUME under an instantiated theorem not noted: E9 gets an argument level error
\* Aim: APSubstInNode.java:323 RemoveConditional_EQUAL_ELSE: bad substitution: E10 gets an argument level error
---- MODULE Other22 ----
VARIABLE x
---- MODULE K ----
CONSTANT c
====
---- MODULE B ----
VARIABLE v
bad == (v')'
====
---- MODULE Q ----
VARIABLE v
b(q) == q' /\ (v')'
====
---- MODULE Q2 ----
VARIABLE v
g(q) == q' /\ v = v
====
IK == INSTANCE K WITH c <- 1
IB == INSTANCE B WITH v <- x
---- MODULE Q3 ----
VARIABLE v
CONSTANT a
THEOREM T == ASSUME (v')' PROVE a' = 1
====
---- MODULE Q3b ----
VARIABLE v
CONSTANT a
THEOREM T == ASSUME v = v PROVE a' = 1
====
I3(ap) == INSTANCE Q3 WITH v <- x, a <- ap
E9 == I3(x')!T
I4(ap) == INSTANCE Q3b WITH v <- (x')', a <- ap
E10 == I4(x')!T
E1 == (LET d == (x')' IN x')'
E2 == (LET d == 1 IN (x')')'
E3 == (LET J == INSTANCE K WITH c <- (x')' IN x')'
E4 == (LET J == INSTANCE B WITH v <- x IN x')'
E5 == (\E i \in (x')' : x')'
F(q) == q' /\ (x')'
E6 == F(x')
I1 == INSTANCE Q WITH v <- x
E7 == I1!b(x')
I2 == INSTANCE Q2 WITH v <- (x')'
E8 == I2!g(x')
====
`,
		},
		want: []letSANYError{
			{"Other22", 23, 8, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Other22", 27, 15, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Other22", 38, 21, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Other22", 47, 34, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Other22", 49, 17, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Other22", 50, 22, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Other22", 51, 38, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Other22", 53, 17, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Other22", 54, 15, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Other22", 58, 29, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
		},
	},
	{
		id: "E-044a8a7005dd", set: "inspection-pit-survivors", class: "semantic-error", root: "Del04.tla",
		files: map[string]string{
			"Del04.tla": `\* Copyright 2026 Christopher O'Connell
\* All rights reserved
\* Aim: LetInNode.java:115 BooleanTrueReturn: the second check of a bad LET must stay false; outer prime error of E1 must stay hidden
\* Aim: LetInNode.java:197 BooleanTrueReturn: a bad LET under an operator application (E12) must keep the outer prime error hidden
\* Aim: InstanceNode.java:370 BooleanTrueReturn: bad LET INSTANCE substitution (E3)
\* Aim: LabelNode.java:222 ConditionalsBoundary and ORDER_ELSE: second check of a label returns true in the baseline, so E13's outer prime error appears; mutants hide it
\* Aim: OpArgNode.java:83 BooleanTrueReturn: bad operator passed as argument (E14)
\* Aim: APSubstInNode.java:381, AssumeProveNode.java:204, ThmOrAssumpDefNode.java:425/529, NewSymbNode.java:125 BooleanTrueReturn: bad ASSUME or NEW set under an instantiated theorem (E9, E9b, E10)
\* Aim: AssumeProveNode.java:212 NegateConditionals and ORDER_ELSE: message order of the ASSUME clause errors (here the first error comes from Q4's NEW set)
---- MODULE Del04 ----
VARIABLE x
---- MODULE K ----
CONSTANT c
====
---- MODULE Q3 ----
VARIABLE v
CONSTANT a
THEOREM T == ASSUME (v')' PROVE a' = 1
====
---- MODULE Q4 ----
VARIABLE v
CONSTANT a
THEOREM T == ASSUME NEW n \in (v')' PROVE a' = 1
====
IK == INSTANCE K WITH c <- 1
I3(ap) == INSTANCE Q3 WITH v <- x, a <- ap
E9 == I3(x')!T
E9b == I3(x')!T /\ I3(x')!T
I4(ap) == INSTANCE Q4 WITH v <- x, a <- ap
E10 == I4(x')!T
E1 == (LET d == (x')' IN x')'
E3 == (LET J == INSTANCE K WITH c <- (x')' IN x')'
E12 == ((LET d == (x')' IN x') = 1)'
E13 == (l2 :: (x')')'
Bad(q) == (x')' /\ q
F1(G(_), q) == G(q) /\ x'
E14 == (F1(Bad, 1))'
====
`,
		},
		want: []letSANYError{
			{"Del04", 18, 21, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Del04", 23, 31, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Del04", 31, 17, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Del04", 32, 38, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Del04", 33, 19, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Del04", 34, 8, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Del04", 34, 15, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
			{"Del04", 35, 11, "Level error in applying operator ': The level of argument 1 exceeds the maximum level allowed by the operator."},
		},
	},
}

func letCaseResolver(files map[string]string) Resolver {
	return func(_ context.Context, name string) (Source, error) {
		text, ok := files[name+".tla"]
		if !ok {
			return Source{}, ErrModuleNotFound
		}
		return Source{Path: name + ".tla", Text: []byte(text), Origin: OriginWorkspace}, nil
	}
}

func letCollapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// letErrors renders a result's errors as compare does for single-file
// errors in the root module of an ASCII source without tabs.
func letErrors(t *testing.T, r Result) (string, []letSANYError) {
	t.Helper()
	class := "ok"
	var got []letSANYError
	for _, d := range r.Diagnostics {
		if d.Code == CodeIncomplete {
			t.Fatalf("incomplete: %+v", r.Diagnostics)
		}
		if d.Severity != SeverityError {
			continue
		}
		switch d.Code {
		case "semantic/4220":
			class = "missing-module"
		case "semantic/4222":
			class = "module-cycle"
		default:
			if class == "ok" {
				class = "semantic-error"
			}
		}
		got = append(got, letSANYError{line: d.Span.Start.Line + 1, col: d.Span.Start.Column + 1, message: letCollapse(d.Message)})
	}
	return class, got
}

func TestLetInstanceCorpusCases(t *testing.T) {
	for _, c := range letCorpusCases {
		t.Run(c.id, func(t *testing.T) {
			r, err := Check(context.Background(), Request{Path: c.root, Origin: OriginWorkspace, Text: []byte(c.files[c.root]), Resolve: letCaseResolver(c.files)})
			if err != nil || r.Completion != CompletionComplete {
				t.Fatalf("%v %+v", err, r)
			}
			class, got := letErrors(t, r)
			if class != c.class || len(got) != len(c.want) {
				t.Fatalf("class %s, want %s; %d errors, want %d: %+v", class, c.class, len(got), len(c.want), got)
			}
			// SANY entries without a position match by text alone (rule 5).
			used := make([]bool, len(got))
			var positioned []letSANYError
			for _, w := range c.want {
				if w.line != 0 {
					positioned = append(positioned, letSANYError{line: w.line, col: w.col, message: letCollapse(w.message)})
					continue
				}
				found := false
				for i, g := range got {
					if !used[i] && g.message == letCollapse(w.message) {
						used[i], found = true, true
						break
					}
				}
				if !found {
					t.Fatalf("no error %q in %+v", w.message, got)
				}
			}
			var rest []letSANYError
			for i, g := range got {
				if !used[i] {
					rest = append(rest, g)
				}
			}
			sort.SliceStable(rest, func(i, j int) bool {
				a, b := rest[i], rest[j]
				return fmt.Sprint(a.line, a.col, a.message) < fmt.Sprint(b.line, b.col, b.message)
			})
			sort.SliceStable(positioned, func(i, j int) bool {
				a, b := positioned[i], positioned[j]
				return fmt.Sprint(a.line, a.col, a.message) < fmt.Sprint(b.line, b.col, b.message)
			})
			for i := range rest {
				if rest[i] != positioned[i] {
					t.Fatalf("error %d: %+v, want %+v", i, rest[i], positioned[i])
				}
			}
		})
	}
}

// letModules wraps each body in its own MODULE header and footer, keyed by
// file name as letCaseResolver expects.
func letModules(bodies map[string]string) map[string]string {
	files := map[string]string{}
	for name, body := range bodies {
		files[name+".tla"] = "---- MODULE " + name + " ----\n" + body + "\n====\n"
	}
	return files
}

func letCheck(t *testing.T, files map[string]string, calls *[]string) Result {
	t.Helper()
	resolve := letCaseResolver(files)
	r, err := Check(context.Background(), Request{Path: "Root.tla", Origin: OriginWorkspace, Text: []byte(files["Root.tla"]), Resolve: func(ctx context.Context, name string) (Source, error) {
		if calls != nil {
			*calls = append(*calls, name)
		}
		return resolve(ctx, name)
	}})
	if err != nil || r.Completion != CompletionComplete {
		t.Fatalf("%v %+v", err, r)
	}
	return r
}

// TestLetInstanceBoundaries covers the authored edges of the chapter: the
// scope of LET-local aliases, parameters and WITH, the internal-module rule,
// LETs inside substitutions and proofs, and the missing, cyclic and clashing
// targets. Shared helper modules: A has a constant, B none.
func TestLetInstanceBoundaries(t *testing.T) {
	helpers := map[string]string{
		"A": "CONSTANT X\nValue == X\nLOCAL Hidden == X",
		"B": "w == 1\nTHEOREM Thm == TRUE",
	}
	with := func(root string) map[string]string {
		files := letModules(helpers)
		files["Root.tla"] = "---- MODULE Root ----\n" + root + "\n====\n"
		return files
	}
	t.Run("aliases parameters and WITH", func(t *testing.T) {
		// One alias name in sibling LETs, one with a parameter used in its
		// WITH clause and an outer parameter, one without parameters.
		var calls []string
		r := letCheck(t, with("F(c) == LET J(x) == INSTANCE A WITH X <- <<x, c>> IN J(1)!Value\nG == LET J == INSTANCE B IN J!w\nH == LET J == INSTANCE A WITH X <- 2\n         K == J!Value IN K"), &calls)
		if len(r.Diagnostics) != 0 || r.Level != LevelComplete || strings.Join(calls, ",") != "A,B" {
			t.Fatalf("%v %+v", calls, r.Diagnostics)
		}
	})
	t.Run("alias scope", func(t *testing.T) {
		// The LET context is popped with its expression: J and J!Value are
		// unknown afterwards (Generator.java:3040-3042).
		r := letCheck(t, with("F == LET J == INSTANCE B IN J!w\nG == J!w"), nil)
		if len(r.Diagnostics) != 1 {
			t.Fatalf("%+v", r.Diagnostics)
		}
		sanyAt(t, r.Diagnostics[0], "semantic/4200", LevelSemantic, "Root.tla", 3, 6, 3, 8, "Unknown operator: `J!w'.")
	})
	t.Run("LOCAL hidden", func(t *testing.T) {
		// Instantiation imports no LOCAL definition (Generator.java:4885).
		r := letCheck(t, with("F == LET J == INSTANCE A WITH X <- 1 IN J!Hidden"), nil)
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "semantic/4200" || r.Diagnostics[0].Message != "Unknown operator: `J!Hidden'." {
			t.Fatalf("%+v", r.Diagnostics)
		}
	})
	t.Run("internal target", func(t *testing.T) {
		// An earlier top-level INSTANCE of the inner module makes every
		// INSTANCE of that name internal (SpecObj.java:528-600): the LET
		// one resolves to the inner module, and the resolver is not asked.
		var calls []string
		r := letCheck(t, with("---- MODULE Inner ----\nCONSTANT c\nA == c\n====\nI == INSTANCE Inner WITH c <- 0\nFoo == LET J == INSTANCE Inner WITH c <- 1 IN J!A"), &calls)
		if len(r.Diagnostics) != 0 || len(calls) != 0 {
			t.Fatalf("%v %+v", calls, r.Diagnostics)
		}
	})
	t.Run("LET in WITH", func(t *testing.T) {
		// getInstances scans the substitutions of an N_NonLocalInstance,
		// so the LET inside the WITH is found after A (ParseUnit.java:670-688).
		var calls []string
		r := letCheck(t, with("I == INSTANCE A WITH X <- LET K == INSTANCE B IN K!w\nV == I!Value"), &calls)
		if len(r.Diagnostics) != 0 || strings.Join(calls, ",") != "A,B" {
			t.Fatalf("%v %+v", calls, r.Diagnostics)
		}
	})
	t.Run("proofs", func(t *testing.T) {
		r := letCheck(t, with("THEOREM T == LET J == INSTANCE B IN J!w = 1\n  <1>1. LET K == INSTANCE B IN K!w = 1\n    OBVIOUS\n  <1> QED\n    BY <1>1"), nil)
		if len(r.Diagnostics) != 0 || r.Level != LevelComplete {
			t.Fatalf("%+v", r.Diagnostics)
		}
	})
	t.Run("theorem clone", func(t *testing.T) {
		// Theorems are instantiated into the LET as ThmOrAssumpDefNodes.
		r := letCheck(t, with("F == LET J == INSTANCE B IN J!Thm"), nil)
		if len(r.Diagnostics) != 0 || r.Level != LevelComplete {
			t.Fatalf("%+v", r.Diagnostics)
		}
	})
	t.Run("LET selector", func(t *testing.T) {
		// An ordinary LET selector reaches a LET operator whose context
		// also holds the instance definitions.
		r := letCheck(t, with("F == LET J == INSTANCE B\n         K == J!w IN K\nG == F!:!K"), nil)
		if len(r.Diagnostics) != 0 || r.Level != LevelComplete {
			t.Fatalf("%+v", r.Diagnostics)
		}
	})
	t.Run("missing", func(t *testing.T) {
		r := letCheck(t, with("R == LET J == INSTANCE Gone IN 1"), nil)
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "semantic/4220" || r.Diagnostics[0].Message != "Cannot find source file for module Gone imported in module Root." {
			t.Fatalf("%+v", r.Diagnostics)
		}
	})
	t.Run("cycle", func(t *testing.T) {
		files := with("R == LET J == INSTANCE C IN 1")
		files["C.tla"] = "---- MODULE C ----\nS == LET K == INSTANCE Root IN 1\n====\n"
		r := letCheck(t, files, nil)
		if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "semantic/4222" || !strings.HasPrefix(r.Diagnostics[0].Message, "Circular dependency among .tla files") {
			t.Fatalf("%+v", r.Diagnostics)
		}
	})
	t.Run("clash", func(t *testing.T) {
		// The marker of a LET module definition enters the LET context
		// through SymbolTable.addSymbol (OpDefNode.java:579,
		// SymbolTable.java:156-170): an outer J of another kind is a
		// Multiply-defined symbol, not processOperator's "Operator J
		// already defined or declared" (Generator.java:2646-2651).
		r := letCheck(t, with("J == 1\nR == LET J == INSTANCE B IN 1"), nil)
		if len(r.Diagnostics) != 1 {
			t.Fatalf("%+v", r.Diagnostics)
		}
		sanyAt(t, r.Diagnostics[0], "semantic/4201", LevelSemantic, "Root.tla", 3, 10, 3, 24, "Multiply-defined symbol 'J': this definition or declaration conflicts \nwith the one at line 2, col 1 to line 2, col 6 of module Root.")
	})
}
