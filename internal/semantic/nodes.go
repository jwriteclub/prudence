// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ SemanticNode.java, LevelNode.java, FormalParamNode.java,
// OpDeclNode.java, ModuleNode.java and OpDefNode.java, Copyright 2003 Compaq
// Corporation and Microsoft Corporation, under the MIT license; see LICENSE.
package semantic

import (
	"strconv"

	generated "github.com/jwriteclub/prudence/internal/generated/sanyparser"
)

// This header covers the modeled symbol-node slice, not all semantic nodes.
// Context and level sets retain the concrete node itself. The header contains
// the actual parser node pointer, not a reconstructed location or Outline.
type leafNode interface {
	levelNode
	header() *nodeHeader
}

// definitionNode is the shared Java OpDefNode identity of user and builtin definitions.
type definitionNode interface {
	leafNode
	definitionNode()
}
type semanticHeader struct {
	owner    *namePool
	identity uint64
	kind     int
	tree     *generated.SyntaxTreeNode
}
type nodeHeader struct {
	semanticHeader
	name *name
}

func newNodeHeader(owner *namePool, name *name, kind int, tree *generated.SyntaxTreeNode) (nodeHeader, error) {
	if owner == nil || name == nil || name.owner != owner {
		return nodeHeader{}, ErrPortIncomplete
	}
	h, err := newSemanticHeader(owner, kind, tree)
	return nodeHeader{semanticHeader: h, name: name}, err
}
func newSemanticHeader(owner *namePool, kind int, tree *generated.SyntaxTreeNode) (semanticHeader, error) {
	if owner == nil {
		return semanticHeader{}, ErrPortIncomplete
	}
	owner.nextNode++
	return semanticHeader{owner: owner, identity: owner.nextNode, kind: kind, tree: tree}, nil
}
func nodeLocation(n leafNode) (Location, error) {
	h := n.header()
	if h.tree == nil {
		return Location{Null: true, Name: "--unknown--", NamePresent: true}, nil
	}
	l := h.tree.Location()
	if l == nil {
		return Location{}, ErrPortIncomplete
	}
	out := Location{SourceTree: h.tree, BeginLine: int(l.BeginLine()), BeginColumn: int(l.BeginColumn()), EndLine: int(l.EndLine()), EndColumn: int(l.EndColumn())}
	if src := l.Source(); src != nil {
		name := h.owner.internUnits(src.Units())
		if !name.displayExact {
			return Location{}, ErrPortIncomplete
		}
		out.Name = name.text
		out.NamePresent = true
	}
	return out, nil
}

type leafLevel struct {
	level, checked         int
	levelParams, allParams map[leafNode]struct{}
}

func newLeafLevel() leafLevel {
	return leafLevel{levelParams: make(map[leafNode]struct{}), allParams: make(map[leafNode]struct{})}
}

type formalParamNode struct {
	nodeHeader
	arity  int
	module *moduleNode
	levels leafLevel
}

func (n *formalParamNode) header() *nodeHeader { return &n.nodeHeader }
func (n *formalParamNode) levelCheck(iter int) {
	if n.levels.checked == 0 {
		n.levels.checked = iter
		n.levels.levelParams[n] = struct{}{}
		n.levels.allParams[n] = struct{}{}
	}
}
func newFormalParam(pool *namePool, name *name, arity int, tree *generated.SyntaxTreeNode, table *symbolTable, module *moduleNode) (*formalParamNode, error) {
	if module != nil && module.owner != pool || table != nil && table.owner != pool {
		return nil, ErrPortIncomplete
	}
	h, err := newNodeHeader(pool, name, kindFormal, tree)
	if err != nil {
		return nil, err
	}
	n := &formalParamNode{nodeHeader: h, arity: arity, module: module, levels: newLeafLevel()}
	if table != nil {
		if _, err = table.addSymbol(name, n); err != nil {
			return n, err
		}
	}
	return n, nil
}

type opDeclNode struct {
	nodeHeader
	arity  int
	module *moduleNode
	levels leafLevel
}

func (n *opDeclNode) header() *nodeHeader { return &n.nodeHeader }
func newOpDecl(pool *namePool, name *name, kind, level, arity int, tree *generated.SyntaxTreeNode, table *symbolTable, module *moduleNode) (*opDeclNode, error) {
	if (kind < 2 || kind > 4) && (kind < 24 || kind > 28) || module != nil && module.owner != pool || table != nil && table.owner != pool {
		return nil, ErrPortIncomplete
	}
	h, err := newNodeHeader(pool, name, kind, tree)
	if err != nil {
		return nil, err
	}
	n := &opDeclNode{nodeHeader: h, arity: arity, module: module, levels: newLeafLevel()}
	n.levels.level = level
	if kind == 2 {
		n.levels.levelParams[n] = struct{}{}
		n.levels.allParams[n] = struct{}{}
	}
	n.levels.checked = 1
	if table != nil {
		if _, err = table.addSymbol(name, n); err != nil {
			return n, err
		}
	}
	return n, nil
}

type builtinOpDefNode struct {
	nodeHeader
	arity              int
	params             []*formalParamNode
	defined            bool
	levels             leafLevel
	maxLevels, weights []int
	isLeibniz          bool
	isLeibnizArg       []bool
	labels             *labelSet
}

func (*builtinOpDefNode) definitionNode()       {}
func (n *builtinOpDefNode) header() *nodeHeader { return &n.nodeHeader }
func newBuiltin(pool *namePool, d *builtinDescriptor) (*builtinOpDefNode, error) {
	if pool == nil || pool.syntax == nil || d == nil || d.arity < -1 {
		return nil, ErrPortIncomplete
	}
	name := pool.intern(d.text)
	tree := pool.syntax.NewBuiltinSyntaxTreeNode(pool.syntax.UniqueStringOf(d.text))
	h, err := newNodeHeader(pool, name, kindBuiltIn, tree)
	if err != nil {
		return nil, err
	}
	n := &builtinOpDefNode{nodeHeader: h, arity: d.arity, defined: true, levels: newLeafLevel()}
	if d.arity >= 0 {
		n.params = make([]*formalParamNode, d.arity)
		for i := range n.params {
			n.params[i], err = newFormalParam(pool, pool.intern("Formal_"+strconv.Itoa(i)), 0, nil, nil, nil)
			if err != nil {
				return nil, err
			}
		}
	}
	if d.max != nil && d.weights != nil {
		if d.arity == -1 {
			if len(d.max) > 0 {
				if len(d.weights) == 0 {
					return nil, ErrPortIncomplete
				}
				n.maxLevels = []int{d.max[0]}
				n.weights = []int{d.weights[0]}
			} else {
				n.maxLevels = []int{}
				n.weights = []int{}
			}
		} else {
			n.maxLevels = d.max
			n.weights = d.weights
		}
		n.isLeibniz = true
		n.isLeibnizArg = make([]bool, len(d.weights))
		for i, v := range d.weights {
			n.isLeibnizArg[i] = v > 0
			n.isLeibniz = n.isLeibniz && n.isLeibnizArg[i]
		}
		n.levels.level = d.level
		n.levels.checked = 99
	}
	return n, nil
}

type moduleNode struct {
	nodeHeader
	standard                             bool
	instantiated                         bool
	instances                            []*instanceNode
	context                              *context
	extendees                            []*moduleNode
	definitions                          []leafNode
	bodyDefinitions                      []leafNode
	recursiveDecls, recursiveSectionDefs []*userOpDefNode
	records                              []*opApplNode
	constantCache, variableCache         []leafNode
	constantsCached, variablesCached     bool
	opDefCache                           []*userOpDefNode
	assumptions                          []*assumeNode
	theorems                             []*theoremNode
	topLevel                             []levelNode
	thmDefCache                          []*thmOrAssumpDefNode
}

func (n *moduleNode) header() *nodeHeader { return &n.nodeHeader }
func newModule(pool *namePool, name *name, tree *generated.SyntaxTreeNode, c *context) (*moduleNode, error) {
	if c == nil || c.owner != pool {
		return nil, ErrPortIncomplete
	}
	h, err := newNodeHeader(pool, name, kindModule, tree)
	if err != nil {
		return nil, err
	}
	return &moduleNode{nodeHeader: h, context: c}, nil
}
func (n *moduleNode) hasDeclarations(kind int) bool {
	cached := &n.constantsCached
	values := &n.constantCache
	if kind == 3 {
		cached = &n.variablesCached
		values = &n.variableCache
	}
	if !*cached {
		*values = make([]leafNode, 0)
		for _, value := range n.context.orderedValues() {
			if d, ok := value.(*opDeclNode); ok && d.kind == kind {
				*values = append(*values, d)
			}
		}
		for i, j := 0, len(*values)-1; i < j; i, j = i+1, j-1 {
			(*values)[i], (*values)[j] = (*values)[j], (*values)[i]
		}
		*cached = true
	}
	return len(*values) > 0
}

// opDefs snapshots the reverse of the Context Pair traversal, independently
// of the declaration caches and their Hashtable enumeration.
func (n *moduleNode) opDefs() []*userOpDefNode {
	if n.opDefCache == nil {
		n.opDefCache = make([]*userOpDefNode, 0)
		for p := n.context.last; p != nil; p = p.prior {
			if d, ok := p.value.(*userOpDefNode); ok && d.kind != 6 && d.kind != kindBuiltIn {
				n.opDefCache = append(n.opDefCache, d)
			}
		}
		for i, j := 0, len(n.opDefCache)-1; i < j; i, j = i+1, j-1 {
			n.opDefCache[i], n.opDefCache[j] = n.opDefCache[j], n.opDefCache[i]
		}
	}
	return n.opDefCache
}
func (n *moduleNode) parameterFree() bool { return !n.hasDeclarations(2) && !n.hasDeclarations(3) }
