// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Derived from TLA+ JunctionListContext.java, Copyright (c) 2003 Compaq
// Corporation, distributed under the MIT license. See NOTICE.
// Source SHA-256: ff7658f4c3d2a99686f350cff56c7d8b37252acffe119b34687790b7f363eea7
package generated

import (
	"errors"
	"strconv"
)

type junctionInfo struct{ kind, column int32 }
type JunctionListContext struct{ stack []junctionInfo }

func NewJunctionListContext() *JunctionListContext { return &JunctionListContext{} }
func isJunctionListBulletToken(kind int32) bool    { return kind == tokenAnd || kind == tokenOr }
func (j *JunctionListContext) Start(column, kind int32) error {
	if !isJunctionListBulletToken(kind) {
		return errors.New("sany junction: invalid kind " + strconv.FormatInt(int64(kind), 10))
	}
	j.stack = append(j.stack, junctionInfo{kind: kind, column: column})
	return nil
}
func (j *JunctionListContext) Terminate() error {
	if len(j.stack) == 0 {
		return errors.New("sany junction: empty stack")
	}
	j.stack = j.stack[:len(j.stack)-1]
	return nil
}
func (j *JunctionListContext) IsNewBullet(column, kind int32) bool {
	if len(j.stack) == 0 || !isJunctionListBulletToken(kind) {
		return false
	}
	head := j.stack[len(j.stack)-1]
	return head.column == column && head.kind == kind
}
func (j *JunctionListContext) IsAboveCurrent(column int32) bool {
	return len(j.stack) == 0 || j.stack[len(j.stack)-1].column < column
}
