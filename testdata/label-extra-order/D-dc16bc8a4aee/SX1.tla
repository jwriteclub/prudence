---- MODULE SX1 ----
Fn == [k \in 1..3 |-> k + x]
       /\ Nest!outer(2, 4)!ilab(i, j) :: i n == [k \in 0..3 |-> k + x]
Ch == CHO :: est!outer(2, 3)!inner = TRUE
       /\ Nest!outer(2, 4)!inner = FALSE
====
