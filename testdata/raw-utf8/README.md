<!-- Copyright 2026 Christopher O'Connell -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Raw-byte UTF-8 corpus

This directory holds a finite corpus of 99 exact byte strings. Each one exercises how Prudence handles source bytes that are not valid UTF-8, or that sit at the edges of UTF-8 decoding. The corpus covers the product lexer, strict parser, recovering scan, monolith extraction and public `Check`. `raw_utf8_test.go` runs each row through the paths relevant to its expectations and requires the row count to stay at 99.

The rows come from section 8 of the invalid-UTF-8 source contract, a design note kept in the development repository. The pins are:
- SANY `dc6a944429fe3e26104d38f9ff792d2d5d83f307`;
- JDK Temurin 17.0.21+9: `sun/nio/cs/UTF_8.java`, `CharsetDecoder.java`, `StreamDecoder.java`, `BufferedReader.java`, `PrintWriter.java`.

Anchors in the `basis` column without a path refer to those files. No JDK code is copied here. No row has been run against Java.

## Format

`manifest.tsv` has one row per line, with seven tab-separated fields. Lines starting with `#` are comments.

| Field | Meaning |
|---|---|
| `id` | Row name, unique. |
| `form` | How `bytes` becomes the input (see below). |
| `bytes` | The exact bytes. Printable ASCII stands for itself. The escapes are `\xHH`, `\n`, `\r`, `\t` and `\\`. The file contains no other bytes, and no fixture is stored raw. |
| `groups` | The expected decoder groups, in order. `bad/N` is one malformed group of N bytes, decoded to one U+FFFD. `U+XXXX/N` is a valid scalar encoded in N bytes. `U+FFFD/3` is a literal EF BF BD, which is not a malformed group. |
| `expect` | Extra checks that the test reads, or `-` if there are none. |
| `basis` | Source anchors for the expectation. |
| `status` | How far the expectation is settled (see below). |

## Forms

The template forms put `bytes` (X) into a fixed module:

| Form | Input | X starts at | Rule-derived checks |
|---|---|---|---|
| `C` | `---- MODULE M ----\n(* ` X ` *) @\n====\n` | byte 22 | One strict parse error, `Encountered "@" at line 2, column c`. c is SimpleCharStream's column over the decoded line: tab stops every 8, and a supplementary scalar takes 2 columns. The span is [26+len(X), 27+len(X)) on public line 1. |
| `C@N` | `C`, with ASCII `a` padding before X | byte N | Checks the 8192/16384-byte StreamDecoder buffer edges and the 4096/8192-char SimpleCharStream buffer edges. |
| `K` | `---- MODULE M ----\nx == ` X `\n====\n` | byte 24 | First diagnostic: `Lexical error at line 2, column 6.  Encountered: "\uXXXX" (n)` for the first UTF-16 unit. Non-ASCII is escaped, as in TokenMgrError.addEscapes. The span is [24, end of the first group). The recovering scan gives the same span. |
| `S` | `---- MODULE M ----\nx == "a` X `b"\n====\n` | byte 26 | `Check` reaches the semantic level with no errors. The string image units are `a`, then X's units, then `b`. |
| `T` | `---- MODULE M ----\nx == "a` X, at EOF | byte 26 | X's groups follow the EOF rule, and the first diagnostic is an `<EOF>` lexical error. |

The other two forms take `bytes` whole:
- `raw`: `bytes` is the whole root. `groups` lists every non-ASCII group in order.
- `resolver`: the root is `---- MODULE M ----\nEXTENDS N\n====\n`, and the resolver returns `bytes` as `N.tla`. `groups` lists N's non-ASCII groups.

The test checks every row, whatever its form, for:
- groups: in the template forms, the groups must tile X exactly;
- raw-byte identity: `Parse`, `Tokens` and `Check` leave the caller's bytes unchanged;
- span alignment: no public token or diagnostic span boundary falls inside a decoder group, for the root or for the resolver source.

## Expectation keys

| Key | Check |
|---|---|
| `parse=L:C@B` | Strict parse error `Encountered "@" at line L, column C`, with span [B, B+1). |
| `lex=L:C@A-B` | First lexical diagnostic from `Parse` at Java L:C, with span [A, B). |
| `jpos=B:L:C` | The unit whose group starts at byte B is at Java line L, column C. |
| `check=semantic` | `Check` reaches the semantic level with no errors. |
| `checksem=A-B` | `Check` reaches the semantic level with exactly one error: a semantic error on the root over [A, B). |
| `checklex=P:L:C@A-B` | `Check` reports a lexical error in path P at L:C, over [A, B). |
| `strict=clean` | The strict root parse has no errors. Java never tokenizes past the root's end. |
| `trail=A-B[,A-B...]` | The root's end is proved, and `Parse` completes with no error. Its only diagnostics are informational `CodeTrailingLexical` ones over exactly these spans, in order, all at or after the root's end. `Tokens` reports the same spans and messages as lexical errors, and `Check` carries the same trailing diagnostics. |
| `checktail=root` | `Check` of the row equals `Check` of the row cut at the proved root end in level, completion and every non-trailing diagnostic, and only the whole row has trailing diagnostics. |
| `extract=N@A-B` | Monolith extraction finds N. The extracted text is the owner lines in [A, B), copied unchanged plus LF. Each terminator maps back to the original one. Each group decoded from the extracted text maps to the same group in the owner. |
| `extract=N:none` | Extraction does not find N, and returns no error. |
| `extractlex=N:L:C@A-B` | N's extracted parse fails lexically at N's L:C, and the failed-unit diagnostic maps to owner bytes [A, B). |
| `token=A-B` | `Tokens` returns a token exactly over [A, B). |
| `fallback=L:C@A-B` | The lexical-error span fallback, forced with no scan diagnostic, covers [A, B). |

A few rows have additional checks in the test:
- B4: the resolver source gives the same diagnostics as parsing it as a root.
- I2-*: FF, EF BF BD and C0 give the same messages and the same level.
- I3: the fallback covers both units of a supplementary scalar.

## Status

- `source`: settled by the pinned SANY or JDK source, and observed in Prudence.
- `inferred`: reasoned from source but not observed against Java. These rows are **not verified by the oracle**. They assume a UTF-8 default charset, which both oracle environments pin. They are B5, M1, M2, M2z, M3, M4, M4z, M5, M6a, M6b and M10.
- `oracle=eof-message`: the groups are settled. The EOF message text and column still need the pinned Java oracle (E1–E10). The test checks only for an `<EOF>` lexical error.
- `oracle=bom-outcome`: the BOM decoding is settled. Whether Java accepts a BOM before the header depends on SANY's pre-module lexer state, so B1, B2 and B4 need the oracle. The test asserts no outcome for them.
- `tail=info`: recovering-scan diagnostics after the root module's end are informational `CodeTrailingLexical` diagnostics and never block `Check` (Chris's decision). These rows assert the trailing spans (`trail`) and `Check`'s semantic progression (`check`), its independence from the tail (`checktail`) or the dependency rejection mapped to the owner (`checklex`), alongside the strict parse, extraction and extracted-unit facts. These rows are M3, M5, M6a, M6b, M9a–M9d and M10. Their Java outcomes remain oracle-unverified where the row is `inferred`.

No row asserts the public `Column` or `UTF16Column`. Chris settled their policy: they keep Go's per-byte replacement, counting each invalid byte as one column, and `ByteOffset` is authoritative (repository `README.md`). `TestColumnPolicyPerByte` pins both contracts together. Byte offsets, Java line/column, decoded units and message text are checked here.

## Changes from section 8

- I2 is split into three rows: I2-ff, I2-efbfbd and I2-c0.
- L1 is a `C` row.
- The X rows are named `Xn-<byte>` for byte offsets and `X6-u<index>` for UTF-16 indices.
- M2z and M4z add a semantic error after a malformed comment line, to show that spans map through extraction for LF and for CR.
- M6 is split into M6a (lone 85) and M6b (C2 85).
- M9 is split into M9a–M9d.
