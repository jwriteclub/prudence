---- MODULE SiblingExtends ----
---- MODULE A ----
X == 1
====
---- MODULE B ----
EXTENDS A
Y == X
====
INSTANCE B
Result == Y
====
