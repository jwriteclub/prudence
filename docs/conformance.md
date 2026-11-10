<!-- Copyright 2026 Christopher O'Connell -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# How conformance was measured

We compared Prudence with stock SANY at commit `dc6a944` (run on Linux,
Temurin 17) on a corpus of 9,453 TLA+ modules: 4,130 from the TLA+ project
and other open-source projects, and 5,323 generated (fuzz outputs and
mechanical mutants). A module matches when both give the same result (accept,
parse error, semantic error, missing module or cycle) and, for a rejection,
the same error count, line, column, module and message text. The comparison
called the public `Check` with the full semantic and level check.

At source `b801ed2`, Prudence agrees with SANY on all 9,425 modules where SANY
reaches a verdict, including all 4,106 modules SANY accepts. One of the 9,425
matches after sorting a list that SANY prints in hash-set order; the other
9,424 match as printed. SANY crashes on the remaining 28 modules, so they are
not scored. On those 28, Prudence returns `CompletionIncomplete` for 19 and a
clean rejection for 9, and accepts none.

One large file, `TE.tla` (about 2.9 MB, from the TLA+ test-model suite), needs
about 2.04 million acquisition visits, more than the default limit (see
[Options](api.md#options)). The run set `MaxAcquisitionVisits` to 3,000,000 for that
file only, and the [published report](https://github.com/jwriteclub/sany-conformance/blob/main/results/prudence-b801ed2/report.md) records the override. With the default, `TE.tla` returns `CompletionIncomplete`.

Limits of this result:

- Prudence was developed and fixed against this corpus, so it is not an
  independent test. There is no holdout set yet.
- The corpus is not a random sample of TLA+ files, and many cases are
  related: mutants come from upstream parents, and test models share modules.
  Zero disagreements in 3,527 distinct accepted modules would put the
  disagreement rate below about 0.1% at 95% confidence if they were a random
  sample, so that figure is a rough guide at best.
- The comparison does not cover warnings, error codes, the syntax tree, the
  semantic graph, end positions or SANY's stderr.
- The oracle is one SANY build on one platform.
- The last differences we found and fixed were in error reporting, and we
  expect any that remain to be there too.

The corpus and the comparison harness are public in
[sany-conformance](https://github.com/jwriteclub/sany-conformance), and
[`results/prudence-b801ed2/`](https://github.com/jwriteclub/sany-conformance/tree/main/results/prudence-b801ed2) holds
this run's report and per-case results. Its `scripts/compare.sh` reruns the
comparison against a prudence checkout; the published run used
`--deep-cuts --jobs 32 --timeout 120s --info --no-3d`. Prudence v0.1.0 has the
same code as `b801ed2` apart from its `Version` constant; the other changes are
to comments and documentation.

