<!-- Copyright 2026 Christopher O'Connell -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Diagnostics and positions

Each `Diagnostic` has a `Phase`, `Severity`, `Code`, `Message`, `Path` and
`Span`. Error messages are SANY's text. The codes are:

| Code | Meaning |
| --- | --- |
| `lexical`, `syntax/expected` | Lexer and parser errors. |
| `semantic/<n>` | SANY error code `n`, from module loading, semantic analysis or level checking. Level-checking diagnostics have `Phase` `LevelComplete`. |
| `depth-limit` | The parser reached `Options.MaxDepth` in the root or a dependency. |
| `analysis/incomplete`, `analysis/acquisition-limit` | Why a check stopped early (see [What it checks](api.md#what-it-checks)). |
| `lexical/trailing` | Informational: a lexical problem after the root module's closing `====`. SANY never reads that text, so it never blocks `Check`. |
| `internal` | Reserved; not currently emitted. |

Module loading reports these SANY codes, all with `Phase` `LevelSemantic`:

- `semantic/4220`: no source supplies a module (not the resolver, the
  embedded standard modules or the root file). This needs a resolver; with
  none configured, a missing non-standard module makes the check incomplete
  instead. `CodeModuleNotFound` is reserved and not used yet.
- `semantic/4221`: the root module's name does not match its file name. A
  `Path` that ends in `.tla` (in any letter case) is treated as a file name,
  and as in SANY the module must carry the file's base name. Any other `Path`, including an
  empty one, imposes no name. A dependency's file name is always the module
  name it was requested by.
- `semantic/4222`: a module cycle.
- `semantic/4003`: SANY's PlusCal validator cannot uncomment the algorithm
  text (for example an unterminated string). Loading continues, and semantic
  analysis is skipped.

SANY's checksum warnings (4803 to 4805) are not reported. A diagnostic that
SANY prints with no source position, such as one "In module --TLA+
BUILTINS--", gets an empty span at the start of the root path.

When one file holds several modules, SANY extracts each one and analyses it as
a unit of its own (its `MonolithSpecExtractor`). A semantic or level diagnostic
inside such an extracted module keeps the root's `Path` and `Span`, so editors
can place it. It also sets `Diagnostic.Unit` to the module name and the
one-based line and column that SANY prints in its message. Those are counted
in the extracted text, in UTF-16 units with tab stops of 8. `Unit` is nil for
every other diagnostic.

## Positions

Lines and columns are zero-based, and spans are half-open byte ranges.
`Column` counts Unicode code points and `UTF16Column` counts UTF-16 units; a
tab counts as one in both. `ByteOffset` is authoritative, and a span never
splits a multi-byte character. Inside malformed UTF-8 the columns count each
invalid byte as one character, as Go does, while SANY's decoder turns a whole
malformed sequence into one U+FFFD. Columns there can differ from the line and
column quoted in SANY's message text, so editors should map spans from
`ByteOffset`.

