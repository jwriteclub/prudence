// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ ExprNode.java, NumeralNode.java, DecimalNode.java,
// StringNode.java and OpDefNode.java, Copyright 2003 Compaq Corporation and
// Microsoft Corporation, under the MIT license; see LICENSE.
package semantic

import (
	"math/big"
	"strconv"
	"strings"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

// Expressions have semantic identity but are not symbols and cannot enter a Context.
type exprOrOpArg interface {
	levelNode
	argumentHeader() *semanticHeader
}
type exprNode interface {
	levelNode
	exprOrOpArg
	expression() *exprHeader
}
type exprHeader struct {
	semanticHeader
	levels          leafLevel
	subExpressionOf leafNode
}

func (h *exprHeader) expression() *exprHeader         { return h }
func (h *exprHeader) argumentHeader() *semanticHeader { return &h.semanticHeader }
func newExprHeader(pool *namePool, kind int, tree *generated.SyntaxTreeNode) (exprHeader, error) {
	h, err := newSemanticHeader(pool, kind, tree)
	return exprHeader{semanticHeader: h, levels: newLeafLevel()}, err
}
func (h *exprHeader) levelCheck(iter int) { h.levels.checked = iter }

type numeralNode struct {
	exprHeader
	image    string
	value    int32
	bigValue *big.Int
}

func newNumeral(pool *namePool, image string, tree *generated.SyntaxTreeNode) (*numeralNode, error) {
	num := strings.ToLower(image)
	radix := 10
	if len(num) == 0 {
		return nil, ErrPortIncomplete
	}
	if num[0] == '\\' {
		if len(num) < 3 {
			return nil, ErrPortIncomplete
		}
		switch num[1] {
		case 'b':
			radix = 2
		case 'o':
			radix = 8
		case 'h':
			radix = 16
		default:
			return nil, ErrPortIncomplete
		}
		num = num[2:]
	}
	v, ok := new(big.Int).SetString(num, radix)
	if !ok {
		return nil, ErrPortIncomplete
	}
	h, err := newExprHeader(pool, 16, tree)
	if err != nil {
		return nil, err
	}
	n := &numeralNode{exprHeader: h, image: image}
	if v.IsInt64() && v.Int64() >= -2147483648 && v.Int64() <= 2147483647 {
		n.value = int32(v.Int64())
	} else {
		n.bigValue = v
	}
	return n, nil
}

type decimalNode struct {
	exprHeader
	integral, fractional string
	mantissa             int64
	exponent             int
	bigValue             *big.Int
	scale                int
}

func newDecimal(pool *namePool, a, b string, tree *generated.SyntaxTreeNode) (*decimalNode, error) {
	v, ok := new(big.Int).SetString(a+b, 10)
	if !ok || len(b) == 0 {
		return nil, ErrPortIncomplete
	}
	h, err := newExprHeader(pool, 17, tree)
	if err != nil {
		return nil, err
	}
	n := &decimalNode{exprHeader: h, integral: a, fractional: b}
	if x, e := strconv.ParseInt(a+b, 10, 64); e == nil {
		n.mantissa = x
		n.exponent = -len(b)
	} else {
		n.bigValue = v
		n.scale = len(b)
	}
	return n, nil
}

type stringNode struct {
	exprHeader
	value *name
}

func newString(pool *namePool, tree *generated.SyntaxTreeNode, strip bool) (*stringNode, error) {
	if pool == nil || tree == nil || tree.Image() == nil {
		return nil, ErrPortIncomplete
	}
	units := tree.Image().Units()
	if strip {
		if len(units) < 2 {
			return nil, ErrPortIncomplete
		}
		units = units[1 : len(units)-1]
	}
	h, err := newExprHeader(pool, 18, tree)
	if err != nil {
		return nil, err
	}
	return &stringNode{exprHeader: h, value: pool.internUnits(units)}, nil
}

type userOpDefNode struct {
	nodeHeader
	arity                           int
	params                          []*formalParamNode
	local, defined                  bool
	letInLevel                      int
	inRecursive, inRecursiveSection bool
	recursiveSection                int
	labels                          *labelSet
	body                            exprNode
	stepNode                        levelNode
	module                          *moduleNode
	source                          *userOpDefNode
	compoundID                      []*name
	levels                          leafLevel
	maxLevels, weights              []int
	isLeibniz                       bool
	isLeibnizArg                    []bool
}

func (n *userOpDefNode) sourceNode() *userOpDefNode {
	if n.source != nil {
		return n.source
	}
	return n
}
func (n *userOpDefNode) compound() []*name {
	if n.compoundID != nil {
		return n.compoundID
	}
	return []*name{n.name}
}
func newModuleMarker(pool *namePool, name *name, params []*formalParamNode, local bool, module *moduleNode, source *userOpDefNode, tree *generated.SyntaxTreeNode) (*userOpDefNode, error) {
	if module == nil || module.owner != pool {
		return nil, ErrPortIncomplete
	}
	for _, p := range params {
		if p == nil || p.owner != pool {
			return nil, ErrPortIncomplete
		}
	}
	h, err := newNodeHeader(pool, name, 6, tree)
	if err != nil {
		return nil, err
	}
	arity := -1
	if params != nil {
		arity = len(params)
	}
	return &userOpDefNode{nodeHeader: h, arity: arity, params: params, local: local, module: module, source: source, levels: newLeafLevel()}, nil
}

func (*userOpDefNode) definitionNode()       {}
func (n *userOpDefNode) header() *nodeHeader { return &n.nodeHeader }
func newUserOpDef(pool *namePool, name *name, params []*formalParamNode, local bool, body exprNode, module *moduleNode, table *symbolTable, tree *generated.SyntaxTreeNode) (*userOpDefNode, error) {
	if body != nil && body.expression().owner != pool || module == nil || module.owner != pool || table != nil && table.owner != pool {
		return nil, ErrPortIncomplete
	}
	for _, p := range params {
		if p == nil || p.owner != pool {
			return nil, ErrPortIncomplete
		}
	}
	h, err := newNodeHeader(pool, name, 5, tree)
	if err != nil {
		return nil, err
	}
	if params == nil {
		params = make([]*formalParamNode, 0)
	}
	n := &userOpDefNode{nodeHeader: h, arity: len(params), params: params, local: local, defined: true, letInLevel: -1, recursiveSection: -1, body: body, module: module, levels: newLeafLevel(), maxLevels: make([]int, len(params)), weights: make([]int, len(params)), isLeibniz: true, isLeibnizArg: make([]bool, len(params))}
	for i := range params {
		n.maxLevels[i] = 3
		n.isLeibnizArg[i] = true
	}
	if table != nil {
		_, err = table.addSymbol(name, n)
	}
	return n, err
}

// opApplNode represents the base constructor's unbounded, range-free case.
// Matching is restricted to exact arity and expression-valued operands;
// higher-order arguments, mismatch diagnostics and level propagation remain unsupported.
type opApplNode struct {
	exprHeader
	operator    leafNode
	unbounded   []*formalParamNode
	bounded     [][]*formalParamNode
	tupleBounds []bool
	operands    []exprOrOpArg
	ranges      []exprNode
}

func newOpAppl(pool *namePool, op leafNode, args []exprOrOpArg, tree *generated.SyntaxTreeNode) (*opApplNode, error) {
	if op == nil || op.header().owner != pool || args == nil || nodeArity(op) != len(args) {
		return nil, ErrPortIncomplete
	}
	switch v := op.(type) {
	case *formalParamNode, *opDeclNode, *builtinOpDefNode, *thmOrAssumpDefNode:
	case *userOpDefNode:
		_ = v
	default:
		return nil, ErrPortIncomplete
	}
	for _, arg := range args {
		if arg != nil && arg.argumentHeader().owner != pool {
			return nil, ErrPortIncomplete
		}
	}
	h, e := newExprHeader(pool, 9, tree)
	if e != nil {
		return nil, e
	}
	return &opApplNode{exprHeader: h, operator: op, operands: args, ranges: make([]exprNode, 0)}, nil
}

// letInNode holds the definitions and body in their request-owned context.
// As LetInNode's opDefs (SymbolNode[]) and insts, definitions hold OpDef and
// ThmOrAssumpDef nodes, including ModuleInstanceKind markers, and instances
// the LET-local InstanceNodes (LetInNode.java, Generator.java:3028-3040).
type letInNode struct {
	exprHeader
	definitions []leafNode
	instances   []*instanceNode
	body        exprNode
	context     *context
}

// opArgNode is the source OpArgNode constructor, with no level propagation yet.
type opArgNode struct {
	semanticHeader
	operator leafNode
	name     *name
	arity    int
	module   *moduleNode
	levels   leafLevel
}

func (n *opArgNode) argumentHeader() *semanticHeader { return &n.semanticHeader }
func newOpArg(pool *namePool, op leafNode, tree *generated.SyntaxTreeNode, module *moduleNode) (*opArgNode, error) {
	if op == nil || op.header().owner != pool || module == nil || module.owner != pool {
		return nil, ErrPortIncomplete
	}
	h, e := newSemanticHeader(pool, 8, tree)
	if e != nil {
		return nil, e
	}
	return &opArgNode{semanticHeader: h, operator: op, name: op.header().name, arity: nodeArity(op), module: module, levels: newLeafLevel()}, nil
}
func expressionOperands(args []exprNode) []exprOrOpArg {
	if args == nil {
		return nil
	}
	out := make([]exprOrOpArg, len(args))
	for i, arg := range args {
		if arg != nil {
			out[i] = arg
		}
	}
	return out
}

// atNode retains the actual enclosing EXCEPT and component applications.
type atNode struct {
	exprHeader
	except, component *opApplNode
}

// labelNode models an expression label and its nested label set.
type labelNode struct {
	exprHeader
	name          *name
	params        []*formalParamNode
	body          levelNode
	labels        *labelSet
	isAssumeProve bool
	goal          *thmOrAssumpDefNode
	goalClause    int
}

func (n *labelNode) expressionBody() exprNode { e, _ := n.body.(exprNode); return e }
