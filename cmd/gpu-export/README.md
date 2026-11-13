<!-- Copyright 2026 Christopher O'Connell -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# GPU compiler input

Run the following command from the repository root.

```sh
go run ./cmd/gpu-export MODULE.tla MODEL.cfg > export.json
```

The command reads dependencies beside the root module, then consults the bundled standard modules. A successful call writes one JSON document to stdout. A rejected or unsupported input produces a nonzero exit code and a reason on stderr.

## Document identity

| Field | Meaning |
| --- | --- |
| `schema`, `version` | `"prudence.resolved"` and `1` |
| `root`, `modules` | Root module ID and external module IDs in dependency order |
| `nodes` | Flat array of semantic objects with positive IDs local to the document |
| `sources` | One record per acquired module with `module`, `path`, `origin` (`workspace` or `standard`) and `sha256` |
| `root_source_sha256`, `cfg_sha256` | Lowercase SHA-256 of the exact root and cfg bytes |
| `cfg` | Parsed directives in input order |

The CLI supplies basenames as path labels. Each source span has a `source` string and one-based inclusive `start` and `end` line/column pairs in SANY coordinates. Synthetic nodes can lack a span. A numeric `level` records the checked TLA+ level where available.

## Semantic objects

A reference is a numeric ID. An omitted list is empty. Array order follows the corresponding semantic vector. The following fields describe the executable graph.

| Kind | Fields |
| --- | --- |
| `module` | `name`, `standard`, `symbols`, `variables`, `constants`, `definitions`, `extends`, `instances`, `assumptions`, `statements`, `nestedModules` |
| `definition`, `moduleInstanceDefinition` | `name`, `arity`, `params`, `module`, optional `sourceDefinition`, `local`, `recursive`, `labels`. An ordinary definition has a `body` |
| `builtin`, `variable`, `constant`, `formal` | `name` and `arity`. Variable, constant and formal records can reference their `module` |
| `apply` | Resolved `operator`, ordered `args` and `binders` |
| `opArg` | Resolved `operator` and `module` for an operator passed as an argument |
| `integer`, `decimal`, `string` | String `value`. Integers use decimal notation, decimals preserve their parts, and strings carry the value from SANY |
| `let` | Local `definitions`, `instances` and `body` |
| `instance` | `toModule`, `params`, `substitutions`, optional `name` and `local` |
| `substIn` | `fromModule` (instantiating), `toModule` (instantiated), `substitutions` and `body` |
| `at` | Links back to the enclosing `except` application and update `component` |
| `label` | `name`, `params`, `body` and nested `labels` |
| `assume`, `assumeDefinition` | `body` and `module`. An assumption can reference its named declaration via `sourceDefinition` |

Use `root.symbols` to look up cfg names in the root's resolved context. A module's `definitions` includes generated body definitions for local LETs. The declaration lists follow SANY's module accessors. A formal's ID distinguishes it from every other declaration with the same spelling.

The following application binds two formals as a tuple over domain 80.

```json
{"id":90,"kind":"apply","operator":22,"args":[87],"binders":[{"params":[81,82],"domain":80,"tuple":true}],"level":1}
```

An unbounded binder omits `domain`. The missing test of a CASE OTHER arm has ID zero in its pair's operand list. An `at` reference can create a cycle. INSTANCE substitutions carry a declaration `target` ID, an expression or operator argument `value` ID, an `implicit` flag and an optional `span`.

## Configuration

Plural list directives normalize to their singular spelling. Each constant assignment or selected operator gets a separate record. Every record carries its own `span`.

```json
[
  {"kind":"CONSTANT","name":"Nodes","value":{"kind":"set","text":"","items":[{"kind":"model","text":"n1"}]},"span":{"source":"run.cfg","start":[2,3],"end":[2,14]}},
  {"kind":"SPECIFICATION","name":"Spec","span":{"source":"run.cfg","start":[3,15],"end":[3,18]}},
  {"kind":"CHECK_DEADLOCK","value":{"kind":"boolean","text":"FALSE"},"span":{"source":"run.cfg","start":[4,16],"end":[4,20]}}
]
```

Supported value tags are `integer`, `string`, `boolean`, `model` and `set`. Set items retain lexical order and multiplicity. The parser accepts named selectors for INIT, NEXT, SPECIFICATION, INVARIANT, CONSTRAINT, ACTION_CONSTRAINT, PROPERTY, VIEW and SYMMETRY. The Rust consumer elaborates constants, validates selector arity and checks which operators and directives it supports.

The v1 frontend accepts separate cfg and TLA+ files. It rejects unsupported cfg syntax (including module or operator overrides), theorem/proof nodes, semantic errors, resource stops and incomplete checks for the whole request.
