<!-- Copyright 2026 Christopher O'Connell -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Held case dispositions

This analysis covers fifteen held stock SANY crash cases and the separate
`E-550917c970f7` depth stop. A stock Java exception supplies no clean source
verdict, so it earns no conformance credit. Recommendations here do not change
the library, its limits or recorded comparison outcomes.

## Fifteen semantic crash cases

The pinned stock JAR reproduced all fifteen semantic crashes. Every original
remains **oracle-unavailable, with zero comparison credit**. Direct public
`Check` calls at `dc626eb` returned no Go error, incomplete completion, no error
diagnostics and one informational `analysis/incomplete` diagnostic. The
diagnostic has no source span or unit and identifies the stopped generator
site. An instrumented harness's partial error code or position is not a
completed stock diagnostic stream.

Twelve inputs have identifiable source defects. Three contain apparently
well-formed, properly scoped constant LET definitions; Java instead fails
while indexing its fixed nesting array. None of these observations supplies
a clean stock verdict for an original input.

The Java anchors below use the same `dc6a944` source and stock JAR pinned in
the depth section. Go sites are the suffix of `generator.construct.` in the
public diagnostic.

| Case | Source condition | Stock crash | Go site | Proposed disposition |
|---|---|---|---|---|
| U-40599f15574d | Outer LET declares nullary recursive `f`; an inner LET defines unary `f(x)` | WrongInvocationException, Generator.endOpDefNode:7318 | end-recursive | Keep incomplete; invalid recursive scope/arity, followed by failed Java recovery |
| U-abf4d7652723 | 101 nested constant LET definitions | ArrayIndexOutOfBoundsException, Generator.processLetIn:2978 | let-in | Keep incomplete; Java indexing limit; the 99-level control completes |
| D-95e1930a9754 | A nested module intervenes between RECURSIVE declaration and definition | WrongInvocationException, Generator.endOpDefNode:7318 | end-recursive | Keep incomplete; invalid recursive section |
| D-e0f9f72e3296 | Same illegal recursive section, different input bytes | WrongInvocationException, Generator.endOpDefNode:7318 | end-recursive | Preserve this distinct identity and its unavailable result |
| C-tests-specs-lv3b1-a6734b-delete-token | Empty action body in `<<>>_y` in LV3B1; EXTENDS Naturals | ArrayIndexOutOfBoundsException, Generator.processAction:4165 | expression-action-or-fairness | Keep incomplete; invalid action |
| E-18b7414e5e17 | `HIDE X + Y`, undeclared names and no arithmetic module | NullPointerException, UseOrHideNode.factCheck:97 | proof-fact-check | Keep incomplete; failed recovery of an invalid HIDE expression |
| E-a6caea8c013f | `HIDE 1 + 1`, without an arithmetic module | NullPointerException, UseOrHideNode.factCheck:97 | proof-fact-check | Keep incomplete; the expression is not a named fact |
| E-c6e258b8342e | 101 nested constant LET definitions with paired INs | ArrayIndexOutOfBoundsException, Generator.processLetIn:2978 | let-in | Keep incomplete; the 99-level control completes |
| E-f68eb4c7f7e2 | Outer nullary recursive declaration and inner unary definition | WrongInvocationException, Generator.endOpDefNode:7318 | end-recursive | Keep incomplete; invalid recursive scope/arity |
| D-275fbcebbf60 | `HIDE- MOD`, unresolved prefix minus, not a named fact | NullPointerException, UseOrHideNode.factCheck:97 | proof-fact-check | Keep incomplete; preserve the distinct prefix-expression path |
| D-2fecd0489510 | `LEMMA TRUE` followed by `BY DEF 0` | ArrayIndexOutOfBoundsException, Generator.genIdToSelector:516 | proof-def-selector | Keep incomplete; a numeral cannot name a defined operator |
| S-582b422bd805 | Function bound contains an action ending with no interior expression | ArrayIndexOutOfBoundsException, Generator.processAction:4165 | expression-action-or-fairness | Keep incomplete; invalid action despite grammar admission |
| E-b1f53d508dac | Exactly 100 nested constant LET definitions | ArrayIndexOutOfBoundsException, Generator.processLetIn:2978 | let-in | Keep incomplete; first failing nesting level; the 99-level control completes |
| B-tests-tlaplus-semantic-errors-e4294-modu-2f5541 | Invalid recursive section from the RFC fixtures | WrongInvocationException, Generator.endOpDefNode:7318 | end-recursive | Preserve its provenance and unavailable result |
| A-err-e4294-module | Same bytes as the RFC fixture, separate corpus identity | WrongInvocationException, Generator.endOpDefNode:7318 | end-recursive | Preserve the separate identity; do not deduplicate its result away |

The LV3B1 call reports `LevelComplete` because its Naturals dependency reached
level checking. Its request still reports **CompletionIncomplete**; this is
not a completed check of the root module.

### Causes and possible follow-up decisions

- **Recursive declarations (six):** an unresolved declaration count is
  subtracted at scope exit and decremented again by a later recursive
  definition. Java's global sum becomes negative and throws
  (Generator.java:2241, 2327, 3015, 7306–7319, 7421). Go guards that state in
  `internal/semantic/generator.go:3203–3212`.
- **LET nesting (three):** Java increments level 99 to 100 and writes
  `unresolvedCnt[100]` in an array of length 100 before its later limit check
  (Generator.java:2967–2978, 7162, 7175). Go's guard is at
  `internal/semantic/generator.go:1749–1756`. This fixed semantic-generator
  bound is separate from both parser recursion and `Options.MaxDepth`.
- **HIDE expressions (three):** unresolved operators produce null while
  recording a partial source error; fact checking dereferences null before
  its named-fact restriction (Generator.java:3085–3093, 3108–3117,
  6626–6639; UseOrHideNode.java:95–104). Go guards the null expression in
  `internal/semantic/proofs.go:1002–1009`.
- **Empty angle actions (two):** the grammar admits an optional interior
  expression, but the semantic generator assumes the resulting action has
  four children (tla+.jj:3562–3579; Generator.java:4164–4165). Go checks its
  shape in `internal/semantic/generator.go:707–710`. Ordinary empty tuples
  are a separate, supported construct.
- **Numeric DEF (one):** the proof grammar admits an expression after DEF,
  while selector generation assumes a second child before reaching its
  defined-operator check (tla+.jj:2642–2648; Generator.java:516, 6680,
  6697–6699). Go guards this shape in `internal/semantic/proofs.go:1404–1405`
  and `internal/semantic/remaining_recovery.go:109–111`.

Retaining the current incomplete results needs no product change. A useful
separate improvement would identify the LET nesting bound with a precise
incomplete diagnostic; the current generic `let-in` site also covers other
shape failures, so its cause must first be distinguished. Supporting greater
LET nesting would be an explicit extension beyond the available stock verdicts,
requiring its own specification and boundary tests. Raising `MaxDepth` would
not repair this fixed-array limit.

Turning the twelve source-defect cases into completed rejections, or exposing
partial diagnostics suppressed after the stop, requires an independently
defined recovery policy and product approval. Existing harness codes alone
are insufficient. No reproduced original establishes a Go panic, incorrect
completed verdict or false completion.

### Reference and direct checks

The frozen corpus manifests are pinned at
`b77672efb75af472407872f0bfbca760dfe1c79e`. Their plain stock command is
`java -cp tla2tools.jar tla2sany.SANY ROOT`, also recorded separately with
`-error-codes`. The Linux reference used Temurin 17.0.21+9, UTF-8,
`en_US.UTF-8`, default flags, a 1 MiB stack and maximum heap 16,869,490,688
bytes. Each original exits 255 after a caught semantic exception; none has
a completed first diagnostic position.

Fresh local replay used the identical verified JAR and Temurin build on
macOS 15.5 arm64, one active CPU, SerialGC, `-Xmx96m`, `-Xss1m`, private
temporary directories and a 25-second subprocess wall. All fifteen exception
types, top semantic frames and raw stderr hashes reproduced. Stdout comparison
normalizes only recorded paths and standard-module extraction directories;
this is reproduction evidence, not conformance credit. Three edited 99-level
LET controls completed with exit 0 and remain unscored.

Fresh public `Check` calls used the unchanged library at `dc626eb`, Go 1.26.3
darwin/arm64, one CPU, CGO disabled, offline dependencies, one call at a time,
a 120-second context and default options without overrides. Missing support
files use the embedded standard library. Each root and its bytes are pinned
in the input appendix below. Both fresh probe groups passed the serial
600-second/512 MiB best-effort monitor and exited: observed stock peak
43,713,688 bytes; Go peak 236,946,416 bytes. No server work or full conformance
campaign was performed for this analysis.

## Depth stop: E-550917c970f7

Keep this case **oracle-unavailable and incomplete**, with the existing depth
defaults. The 63,202-byte input has SHA-256
`b7f8cb627e5fae78330ab3eacd334822e02bf0467f4f7db9b58193f1588a1588`.
It contains an unterminated module/expression prefix with 63,171 opening
parentheses and no closing parentheses. It is a synthetic limit input rather
than evidence that an ordinary specification needs a higher default.

The retained stock reference pins TLA+ revision
`dc6a944429fe3e26104d38f9ff792d2d5d83f307`, jar SHA-256
`171c46c1ec562ff1893a709916ab2616865bb3a635529ec7da22c8699ae9c35a`,
Adoptium OpenJDK 17.0.21+9 on Linux amd64, and plain stock `tla2sany.SANY`
with default JVM flags and a reported 1,024 KiB thread stack. It overflows
inside recursive expression parsing. Retained stock reruns also overflow at
`-Xss512k` and `-Xss16m`. These are existing reference observations, not new
runs made for this analysis; the historical manifest's crash-counting
`reject` label is not a syntax verdict.

The recorded public comparison at `b801ed2` reports incomplete syntax
analysis with `parser:2:3005: recursion depth exceeds 12000`. Its projected
`parse-error` class must not be read as a clean rejection. Current source
inspection at `dc626eb` confirms the relevant policy:

- `internal/parserrt/runtime.go` defaults to **12,000 active production and
  scanner calls combined**, returning a sticky depth error before exceeding
  the configured counter.
- `prudence.go` converts that stop to `CodeDepthLimit` and retains incomplete
  completion. Acquisition and semantic checking do not run after this stop.
- The separate downstream default in `internal/recursion/options.go` is
  **2,000**. This case never reached that limit. `Options.MaxDepth` already
  permits a positive caller override, applied across stages with different
  counters; its safety at extreme values has not been established.

The retained depth survey found accepted upstream production depth at most
136 and other accepted corpus depth at most 382. These are empirical parser-call
measurements, not bounds on all specifications or equivalent semantic/tree
depths. They give no basis for a default increase here.

Revisit only with a reproducible real or plausibly generated specification
that stock Java completes and which reaches the actual Go limit. Pin its
bytes, provenance, jar/JVM/stack settings and complete Java outcome; measure
the relevant Go stage and counter, then compare a bounded positive override
with its memory, time and cancellation behavior. A different Java stack is a
different reference setting and does not relabel the canonical crash.

## Input byte identities

| Case | SHA-256 of root input bytes |
|---|---|
| A-err-e4294-module | `0bee1d49de6ffde9b89555f0ad88d3aa4357575fdb23a3baa3558b2b8f6b6b05` |
| B-tests-tlaplus-semantic-errors-e4294-modu-2f5541 | `0bee1d49de6ffde9b89555f0ad88d3aa4357575fdb23a3baa3558b2b8f6b6b05` |
| C-tests-specs-lv3b1-a6734b-delete-token | `b27786bfe7c04c4388f570cf4685597eff8366a3ec176212e260f520ff2d3c6b` |
| D-275fbcebbf60 | `a63dd7227aca41adb8b1a75a32361d8764771302f3f1aa206db95e23050bfef2` |
| D-2fecd0489510 | `d0f1d5585cdc4834ae1b9097275abba7234dc4fe90ec81ef88eecf7fd997cb45` |
| D-95e1930a9754 | `2f4bea7ff64f2208053204ad44a5425cd589de7067dc86171707d50840ba8e98` |
| D-e0f9f72e3296 | `67bb83c0446587ed9df57e7f4cf48e79fe0f518d06a5a5f71e44d640e0beef19` |
| E-18b7414e5e17 | `38244f4e4983dc11f8473c5fe02918f4898926fa2feb29a49dcb07f990284a6a` |
| E-a6caea8c013f | `80dcffeb168c3173b8f07ea2b8ae0f5785cb985edc37c9d51b3bb1181e156715` |
| E-b1f53d508dac | `72ac577b1863b4aa4ce6f7982fa0def32850895fe3b22c36ed7635de58d69684` |
| E-c6e258b8342e | `e9dc1c1dab20480005bf4437ce35958293a2c03549d617b4c6c1ab69d4e317cc` |
| E-f68eb4c7f7e2 | `bdf236cdb81cce432a1d3b6d661515694ab4973c13d1721c0e1ffb5d2fededbb` |
| S-582b422bd805 | `c3bb0df368f7ad3062b84306f89a5096e7016ff6fd463c11ac07664bbeaf94a9` |
| U-40599f15574d | `2cd5d0b2dfd77f752ea3c441e30bdd149d4298e8dccc3754f70e28ced6cd2e99` |
| U-abf4d7652723 | `48a4bb7236896c6fa36278f5b32da83b3b6b7c4b1e369c95607d007fdc442ea2` |
| E-550917c970f7 | `b7f8cb627e5fae78330ab3eacd334822e02bf0467f4f7db9b58193f1588a1588` |
