---- MODULE LexicalInstance ----
CONSTANT C
---- MODULE Inner ----
CONSTANT K
Value == K
====
I == INSTANCE Inner WITH K <- C
Result == I!Value
====
