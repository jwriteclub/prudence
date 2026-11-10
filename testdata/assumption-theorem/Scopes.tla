---- MODULE Scopes ----
CONSTANT S
THEOREM Goal == ASSUME L::TRUE, NEW x \in S, x = x PROVE x = x
F == Goal!L
====
