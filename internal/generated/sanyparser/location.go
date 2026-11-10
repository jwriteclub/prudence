// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Location behavior derives from TLA+ tla2sany/st/Location.java,
// Copyright (c) 2003 Compaq Corporation, distributed under the MIT license.
package generated

import "fmt"

// Location is an immutable Java int-coordinate source location.
type Location struct {
	context        *SupportContext
	source         *UniqueString
	bl, bc, el, ec int32
}

func (ctx *SupportContext) NewLocation(source *UniqueString, bl, bc, el, ec int32) *Location {
	return &Location{context: ctx, source: source, bl: bl, bc: bc, el: el, ec: ec}
}
func (l *Location) BeginLine() int32      { return l.bl }
func (l *Location) BeginColumn() int32    { return l.bc }
func (l *Location) EndLine() int32        { return l.el }
func (l *Location) EndColumn() int32      { return l.ec }
func (l *Location) Source() *UniqueString { return l.source }
func (l *Location) String() string {
	if l == nil {
		return "<nil>"
	}
	if l.context != nil && l == l.context.nullLocation {
		return "Unknown location"
	}
	name := "null"
	if l.source != nil {
		name = l.source.String()
		if (l.context == nil || l.source != l.context.unknownSource) && l.bl == 0 && l.bc == 0 && l.el == 0 && l.ec == 0 {
			return "In module " + name
		}
	}
	return fmt.Sprintf("line %d, col %d to line %d, col %d of module %s", l.bl, l.bc, l.el, l.ec, name)
}
