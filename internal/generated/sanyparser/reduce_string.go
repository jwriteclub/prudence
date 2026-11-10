// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0

// Derived from tla+.jj, Copyright (c) 2003 Compaq Corporation,
// distributed under the MIT license. See NOTICE.
// Source SHA-256: bfc981f31c205333f478b773ab108159b5e2d339b659d33b0f920d56ebf4253d
package generated

const errReduceStringRuntime supportError = "sany reduceString: Java runtime failure"

// ReduceString uses Java UTF-16 units and preserves StringBuffer.setLength padding.
func (ctx *SupportContext) ReduceString(s *UniqueString, check func() error) (*UniqueString, error) {
	if s == nil {
		return nil, errReduceStringRuntime
	}
	units := s.Units()
	out := make([]uint16, 0, len(units))
	i, j := 0, 0
	for i != len(units) {
		if check != nil {
			if err := check(); err != nil {
				return nil, err
			}
		}
		if units[i] == '\\' {
			i++
			if i >= len(units) {
				return nil, errReduceStringRuntime
			}
			switch units[i] {
			case '\\':
				out = append(out, '\\')
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 'f':
				out = append(out, '\f')
			case 't':
				out = append(out, '\t')
			case '"':
				out = append(out, '"')
			}
		} else {
			out = append(out, units[i])
		}
		i++
		j++
	}
	for len(out) < j {
		out = append(out, 0)
	}
	return ctx.UniqueStringOfUnits(out[:j]), nil
}
