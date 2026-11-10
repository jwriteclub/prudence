---- MODULE SX1 ----
\* SubexpressioÚn selectors: !<< !>> !(args), labels with parameters, !n on
\* CASE, records, LET, IF, function application and b=inding constructs.
EXTENDS Naturals, FiniteSets

VARIABLE x

Sum == x + 10
Q == \A i \in 1..3, j \in {4} : lab(i, j) :: i + j > x
Sq == {k * k : k \in 1..3}
Fil == {k \in 1..5 : k > 2}
Fn == [k \in 1..3 |-> k + x]
Ch == CHOOSE k \in 1..9 : k > 4
Cs == CASE x = 0 -> 100 [] x = 1 -> 200 [] OTHER -> 300
Rc == [a |-> x + 1, b |-> x + 2]
Lt == LET y == x + 5 IN y * 2
Ite == IF x > 0 THEN x ELSE 42
App(f) == f[1, 2]
Nest == \E i \in 1..2 : \E j \in 3..4 : outer(i, j) :: \A i \in 1..3, j \in {4} : lab(i, j) :: i + j > x
Sq == {k * k : k \in 1..3}
Fil == {k \in 1..5 : k > 2}
Fn == [k \in 1..3 |-> k + x]
Ch == CHOOSE k \in 1..9 : k > 4
Cs == CASE x = 0 -> 100 [] x = 1 -> 200 [] OTHER -> 300
Rc == [a |-> x + 1, b |-> x + 2]
Lt == LET y == x + 5 IN y * 2
Ite == IF x > 0 THEN x ELSE 42
App(f) == f[1, 2]
Nest == \E i \in 1..2 : \E j \in 3..4 : outer(i, j) :: (i < j /\ innor :: i + j = 5)


Init == x = 0
Next == x'  =(x + 1) % 3

Inv == /\ Sum!<< = x
       /\ Sum!>> = 10
       /\ Q!1'  =(x + 1) % 3

Inv == /\ Sum!<< = x
       /\ Sum!>> = 10
       /\ Q!1 = 1..3
       /\ Q!2 = {4}
       /\ Q!(2, 4) = (6 > x)
       /\ Q!lab= 300
       /\ Rc!2  /\ Nest!outer(2, 3)!inner = TRUE
       /\ Nest!outer(2, 4)!inner = FALSE
====
