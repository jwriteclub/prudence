// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ Generator.java, SymbolTable.java, OpApplNode.java and
// OpDefNode.java, Copyright 2003 Compaq Corporation and Microsoft
// Corporation, under the MIT license; see LICENSE.
package semantic

import (
	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

// Source-proved history admissions for resolution and redefinition paths.
// Each predicate names exactly the branches whose SemanticNode schedule is
// established from pinned Generator.java (dc6a944). Everything else keeps the
// caller's existing invalidation.

// nullableExpression evaluates tree for a consumer that Java proves to store a
// null expression unchanged: an operator definition body (Generator.java:2660,
// then endOpDefNode 7290-7296 or the OpDefNode constructor, OpDefNode.java:
// 481-516), a function definition body (2912, stored by setArgs at 2933,
// OpApplNode.java:277; 2947-2948 reads only the operator), an operand of an infix, prefix or postfix application
// (Generator.java:3097-3098, 3121, 3138, OpApplNode constructor 2 at
// OpApplNode.java:149-162, whose match methods use only instanceof and
// arities: OpDefNode.java:755-870, FormalParamNode.java:55-62,
// OpDeclNode.java:79-89, ThmOrAssumpDefNode.java:281-288), or an operand of
// the builtin constructor-3 applications $SetEnumerate, $Tuple,
// $CartesianProd, $IfThenElse, $ConjList, $DisjList and $SetOfFcns
// (Generator.java:3141-3155, 3286-3292, 3411-3430, 3438-3443; constructor 3
// at OpApplNode.java:171-183 stores the operands). The mark applies only to
// the next expression call, which consumes it on entry.
func (g *generator) nullableExpression(tree *generated.SyntaxTreeNode, depth int) (exprNode, error) {
	g.nullResultConsumer = true
	return g.expression(tree, depth)
}

// unresolvedOperatorHistoryAdmitted admits the unresolved infix, prefix or
// postfix operator path (Generator.java:3080-3139) when its null result goes
// to a consumer named by nullableExpression. generateGenID constructs a GenID
// and interns its name (4486-4552); finalAppend resolves it and reports
// "Could not find declaration or definition of symbol" (264-285); the switch
// resolves it again, reports "Couldn't resolve ... operator symbol" and
// returns null before generating any operand. GenID, UniqueString and the
// error log are not SemanticNodes, so no UID is taken. A resolution error, a
// resolved operator without a known ticket, a lossy display name or any other
// consumer stays unproved.
func unresolvedOperatorHistoryAdmitted(resolveErr error, op leafNode, nullConsumer bool, raw *name) bool {
	return resolveErr == nil && op == nil && nullConsumer && raw != nil && raw.displayExact
}

// existingDefinitionHistoryAdmitted admits processOperator when its name
// already resolves (Generator.java:2598-2655). Every branch then generates the
// body (2660) and pops the parameter context (2663). A RECURSIVE placeholder
// at the current LET level is completed by endOpDefNode (2666), which
// allocates nothing. Every other branch (an existing definition, declaration,
// formal, theorem or builtin; LOCAL then global; a placeholder at another LET
// level, 2639-2644) reports its error first and then constructs one new
// OpDefNode (2674). That constructor takes its UID before SymbolTable.addSymbol
// (OpDefNode.java:481-516), and addSymbol only reports and declines the
// binding (SymbolTable.java:126-225; Go context.go addSymbol). The schedule is
// therefore that of a fresh definition. The existing node's identity must
// still be known, and any other node class stays unproved.
func existingDefinitionHistoryAdmitted(old leafNode, known bool) bool {
	if !known {
		return false
	}
	switch old.(type) {
	case *userOpDefNode, *builtinOpDefNode, *formalParamNode, *opDeclNode, *thmOrAssumpDefNode:
		return true
	}
	return false
}

// existingFunctionHistoryAdmitted admits processFunction when the function
// name already resolves (Generator.java:2794). The bound formals (2791), the
// self FormalParamNode without a symbol table (2817) and the $NonRecursiveFcnSpec
// OpApplNode (2824) are constructed first. No OpDefNode is constructed. An
// existing OpDefNode is reused (2836-2879, appended at 2886); a declaration,
// formal or theorem is not an OpDefNode, so odn stays null, the error at 2875
// is reported, nothing is appended, and the body is generated without
// pushing the parameter context (2882-2912). Builtin names are reserved words
// the parser does not accept here, so a builtin target stays unproved, as do
// unknown identities and any other class.
func existingFunctionHistoryAdmitted(old leafNode, known bool) bool {
	if !known {
		return false
	}
	switch old.(type) {
	case *userOpDefNode, *formalParamNode, *opDeclNode, *thmOrAssumpDefNode:
		return true
	}
	return false
}

// moduleBodyHistoryAdmitted names the module-body kind added by this
// chapter. A module-level N_UseOrHide runs checkIfInRecursiveSection
// (Generator.java:7379-7392), generateUseOrHide (6533-6716, the producer
// already admitted for proof steps, with its own guards), factCheck
// (UseOrHideNode.java:94-107), the empty-statement error and addTopLevel
// (2285-2300); only generateUseOrHide allocates. A module-level N_Proof
// (2271-2273) would allocate nothing, but the parser builds no such node, so
// it stays unproved. Unknown kinds still abort in Java (2316-2324) and stay
// unproved.
func moduleBodyHistoryAdmitted(kind int) bool {
	return kind == generated.N_UseOrHide
}
