\* Copyright 2026 Christopher O'Connell
\* SPDX-License-Identifier: Apache-2.0
---- MODULE Matching ----
D(Op(_)) == Op(42)
G(x) == x
Good == D(G)
Bad == D(D)
Two(A(_), B(_)) == A(B(1))
Both == Two(D, D)
====
