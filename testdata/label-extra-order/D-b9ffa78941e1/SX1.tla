---- MODULE SX1 ----
\* Subexpre.ssion selectors: !<< !>> !(args), labels with parameters, !n on
\* CA10
Fn == [k \in 1..3 |-> k + x]
       /\ Nest!outer(2, 4)!ilab(i, j) :: i + j > x
Sq == {k * k : k \in 1..3}
Fil == {k \in 1..5 : k > 2}
Fn == [k \in 1..3 |-> k + x]
Ch == CHOOSE k \in 1..9 : k > 4
Ite == IF x > 0 THEN x ELSE 42
App(f) == f[1, 2]
Nest == \E i \in 1..2 : \E j \in 3..4 : outer(i, j) :: est!outer(2, 3)!inner = TRUE
       /\ Nest!outer(2, 4)!inner = FALSE
====
