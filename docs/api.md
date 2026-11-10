<!-- Copyright 2026 Christopher O'Connell -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# API

The package has three entry points. `Check` runs the full check (syntax,
semantic analysis and level checking) on one root module. `Parse` runs syntax
analysis only, and `Tokens` runs the lexer only. Neither loads other modules.
The [README](../README.md#use) has a `Check` example; diagnostics and
positions are in [diagnostics.md](diagnostics.md).

## What it checks

`Request.Stage` sets how far `Check` goes:

| `Request.Stage` | What runs |
| --- | --- |
| `LevelSyntax` | The lexer and parser, on the root module. |
| `LevelSemantic` | Also loads the modules the root extends or instantiates, and runs SANY's semantic analysis (its `Generator`): name resolution, arity, `INSTANCE` substitutions, proofs and its other semantic checks. |
| `""` or `LevelComplete` (the default) | Also runs level checking. |

As in SANY, modules are analysed in dependency order with one shared error
log, and each module is level-checked only while no error has been reported.

`Result.Completion` says whether the requested analysis finished. It does not
say whether the spec is valid.

- `CompletionComplete` means the check reached the point where SANY would
  stop. That includes stopping after reporting errors, so a complete result
  can contain errors.
- `CompletionIncomplete` means Prudence stopped before reaching a verdict, so
  treat the result as unknown. The causes are an unsupported construct or
  module structure (for example `INSTANCE` inside `LET`), a non-standard
  module with no resolver configured, the depth or visit limits in
  [Options](#options), the 32 MiB source size limit, cancellation, and inputs
  on which Java SANY itself throws an exception. Most of these stops
  add a diagnostic that says why: `analysis/incomplete` in general, and
  `analysis/acquisition-limit` for the visit limit.

`Result.Level` is the furthest phase examined. It is `LevelComplete` once any
module has been level-checked.

## Module resolution

`Request.Path` is only a label for diagnostics; `Check` never opens it. The
standard modules (`Naturals`, `Sequences`, `TLC` and the others) are embedded,
so a spec that uses only those needs no resolver. For a spec that uses its own
modules, set `req.Resolve` (`Request.Resolve`) to a function that returns a
module's source by name:

```go
dir := "specs"
req.Resolve = func(ctx context.Context, name string) (prudence.Source, error) {
	path := filepath.Join(dir, name+".tla")
	text, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return prudence.Source{}, prudence.ErrModuleNotFound
	}
	if err != nil {
		return prudence.Source{}, err
	}
	return prudence.Source{Path: path, Text: text, Origin: prudence.OriginWorkspace}, nil
}
```

For each `EXTENDS` or `INSTANCE` target, `Check` tries, in order:

1. the resolver, if there is one;
2. the embedded standard modules, if the resolver returned `ErrModuleNotFound`
   (or an error wrapping it) or there is no resolver;
3. a module of that name inside the root file.

Any other resolver error stops the check. SANY looks in the working directory
before its built-in standard modules. To match it, a resolver should return
the caller's own modules first, including ones named like standard modules
(`Naturals.tla`, `TLC.tla`), with `OriginWorkspace`, and return
`ErrModuleNotFound` only when it has nothing. The embedded standard modules
are byte-for-byte copies from SANY's release JAR; see
[internal/standard/README.md](../internal/standard/README.md).

## Outline and tokens

`Result.Outline` lists the module name, `EXTENDS`, `INSTANCE`s and top-level
declarations, taken from the syntax tree. For a module with a syntax error it
holds what was complete before the error: the module header, the `EXTENDS`
clause right after it, and finished top-level definitions. Names and parameter
counts for some operator forms, unnamed assertions and nested constructs can
be missing (`Decl.Params` is -1 when unknown).

Set `Options.IncludeTokens` to get `Result.Tokens`, or call `Tokens`. Only the
text SANY tokenizes is returned: nothing before the module header or after the
closing delimiter. Where SANY's lexer splits a UTF-16 surrogate pair across
two tokens that share the same bytes, Prudence returns one token for them.

`Result.Tree` is an opaque syntax tree with no accessors yet. Syntax-error
locations come from the parser's tokens where available; some of SANY's
custom rejections carry only a point or no location.

## Options

`Options` applies to `Parse` and `Check`.

**`MaxAcquisitionVisits`** limits the work `Check` does while loading the root
module and the modules it depends on. Loading makes several passes: the
syntax-node walk, module discovery, module resolution, cycle checks and
dependency ordering. Each pass has its own counter, and each counter is held
to this limit. It is not a count of distinct syntax nodes, a limit on total
work across passes, or a memory limit.

- Zero selects `DefaultMaxAcquisitionVisits`, which is 1,000,000.
- A positive value overrides it for that call.
- A negative value makes `Check` return an error.

When a pass reaches the limit, `Check` returns `CompletionIncomplete` with an
informational `analysis/acquisition-limit` diagnostic that names the pass, the
limit and `Options.MaxAcquisitionVisits`. Raise it for very large inputs such
as `TE.tla` (see [conformance.md](conformance.md)). `Parse` does not use this option.

**`MaxDepth`** limits recursion depth. Zero selects the defaults: 12,000 in the
parser, and 2,000 in module loading and semantic analysis. A positive value
sets all of them, and a negative value is an error. Reaching the limit stops
the check with `CompletionIncomplete`.

**`IncludeTokens`** fills `Result.Tokens`.

`Check` also refuses a root source over 32 MiB, with `CompletionIncomplete`
and an `analysis/incomplete` diagnostic.

