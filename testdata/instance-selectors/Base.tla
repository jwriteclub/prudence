\* Copyright 2026 Christopher O'Connell
\* SPDX-License-Identifier: Apache-2.0
---- MODULE Base ----
CONSTANT X
F(z) == Tag:: <<X, z>>
Q == LET G(y) == <<X, y>> IN G(1)
====
