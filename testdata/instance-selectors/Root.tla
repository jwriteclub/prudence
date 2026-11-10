\* Copyright 2026 Christopher O'Connell
\* SPDX-License-Identifier: Apache-2.0
---- MODULE Root ----
J(q) == INSTANCE Bridge WITH Y <- q
R == J(1)!I(2)!F(3)!Tag!1
S == J(1)!I(2)!Q!:!G(4)
Apply(Op(_, _, _)) == Op(1, 2, 3)
T == Apply(J!I!F!Tag)
====
