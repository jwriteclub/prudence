// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
// Derived from TLA+ pcal/Validator.java, pcal/ParseAlgorithm.java and
// pcal/trans.java (pinned dc6a944), MIT; see LICENSE.
package modulegraph

import "bytes"

// pcalValidationFails reports whether SpecObj.validateParseUnit
// (SpecObj.java:994-1058) adds its INTERNAL_ERROR "An unexpected problem was
// encountered attempting to validate the specification" for a parsed unit:
// pcal.Validator returns ERROR_ENCOUNTERED, which it does only when
// ParseAlgorithm.uncomment rejects the algorithm text (Validator.java:216-221).
//
// Every other validator result is a warning (4803-4805) or nothing, and is not
// modeled here. The PlusCal options scan (Validator.java:131-159) changes only
// translator settings, never the uncomment decision, so it is skipped.
//
// The Java code indexes UTF-16 units of the UTF-8-decoded lines. This port
// indexes raw bytes. Every decision depends only on ASCII characters and on
// whether a next character exists after an ASCII one; an ASCII byte is always
// that same character after Java's decoding (malformed groups never contain
// ASCII bytes), and a skipped non-ASCII character leaves only non-ASCII bytes,
// which match nothing. Validator.validate runs every check below on
// trans.removeTabs output (Validator.java:126), which replaces each tab with 1
// to 8 spaces (trans.java:1936-1941). Only findTokenPair reads spaces, and only
// as a run of one or more, so tabs are kept and it treats a tab as a space; the
// space removeTabs appends to every line (trans.java:1953) is kept, since it
// can supply a "next character".
func pcalValidationFails(text []byte, beginLine, endLine int) bool {
	lines := javaReadLines(text, beginLine, endLine)
	seenAlgo := false
	for _, line := range lines {
		// Validator.java:99: indexOf(BeginAlg.substring(2)) > 0.
		if bytes.Index(line, []byte("algorithm")) > 0 {
			seenAlgo = true
		}
	}
	if !seenAlgo {
		return false
	}
	spec := make([][]byte, len(lines))
	for i, line := range lines {
		spec[i] = append(append([]byte(nil), line...), ' ')
	}
	algLine, algCol, foundBegin := 0, -1, false
	for algLine < len(spec) && !foundBegin {
		line := spec[algLine]
		if matchesModuleClosing(line) {
			break
		}
		if i := bytes.Index(line, []byte("--algorithm")); i != -1 {
			algCol, foundBegin = i+len("--algorithm"), true
		} else if i := bytes.Index(line, []byte("--fair")); i != -1 {
			algCol, foundBegin = i+len("--fair"), true
		} else {
			algLine++
		}
	}
	translationLine := findTokenPair(spec, 0, "BEGIN", "TRANSLATION")
	endTranslationLine := findTokenPair(spec, translationLine+1, "END", "TRANSLATION")
	if translationLine == -1 && endTranslationLine == -1 {
		return false
	}
	if !foundBegin {
		return false
	}
	return uncommentFails(spec, algLine, algCol)
}

// javaReadLines returns the lines BufferedReader.readLine yields for text
// (terminated by LF, CR or CRLF) whose one-based numbers lie in
// [beginLine, endLine], as Validator.validate collects them
// (Validator.java:89-104).
func javaReadLines(text []byte, beginLine, endLine int) [][]byte {
	var out [][]byte
	n := 1
	for pos := 0; pos < len(text) && n <= endLine; n++ {
		start := pos
		for pos < len(text) && text[pos] != '\n' && text[pos] != '\r' {
			pos++
		}
		end := pos
		if pos < len(text) {
			if text[pos] == '\r' && pos+1 < len(text) && text[pos+1] == '\n' {
				pos++
			}
			pos++
		}
		if beginLine <= n {
			out = append(out, text[start:end])
		}
	}
	return out
}

// matchesModuleClosing is Pattern.compile("^[=]+$").matcher(line).matches()
// (TLAConstants.MODULE_CLOSING_REGEX).
func matchesModuleClosing(line []byte) bool {
	if len(line) == 0 {
		return false
	}
	for _, c := range line {
		if c != '=' {
			return false
		}
	}
	return true
}

// findTokenPair is trans.findTokenPair (trans.java:1887-1917) applied to
// removeTabs output: a tab expands to spaces, so it is skipped like a space
// (trans.java:1902). Neither token contains a tab or space, so indexes into the
// kept-tab line order matches exactly as indexes into the expanded line do.
func findTokenPair(spec [][]byte, lineNum int, tok1, tok2 string) int {
	for i := lineNum; i < len(spec); i++ {
		line := spec[i]
		col := bytes.Index(line, []byte(tok1))
		if col == -1 {
			continue
		}
		next := col + len(tok1)
		for next < len(line) && (line[next] == ' ' || line[next] == '\t') {
			next++
		}
		if next < len(line) && next == bytes.Index(line, []byte(tok2)) {
			return i
		}
	}
	return -1
}

// uncommentFails is ParseAlgorithm.uncomment (ParseAlgorithm.java:3732-3836)
// reduced to its one failure: a string still open at the end of a line,
// "Unterminated string in line N".
func uncommentFails(spec [][]byte, begLine, begCol int) bool {
	line, col := begLine, begCol
	depth := 0
	for done := false; !done && line < len(spec); {
		old := spec[line]
		inString := false
		for !done && col < len(old) {
			c := old[col]
			hasNext := col < len(old)-1
			switch {
			case c == '(' && !inString && hasNext && old[col+1] == '*':
				depth++
				col++
			case c == '*' && !inString && hasNext && old[col+1] == ')':
				if depth == 0 {
					done = true
				} else {
					depth--
				}
				col++
			case depth == 0:
				if inString {
					if c == '"' {
						inString = false
					} else if c == '\\' && hasNext {
						col++
					}
				} else if c == '\\' && hasNext && old[col+1] == '*' {
					col = len(old)
				} else if c == '"' {
					inString = true
				}
			}
			col++
		}
		if inString {
			return true
		}
		col = 0
		line++
	}
	return false
}
