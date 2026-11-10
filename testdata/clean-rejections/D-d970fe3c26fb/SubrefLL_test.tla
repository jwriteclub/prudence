(* Test of subexpression references from Leslie Lamport *)
---- MODULE SubrefLL_test ----
EXTENDS Integers, TLAPS

CONSTANT C
ASSUME c4 == C = 4
USE c4

Salt(s) == s = s
USE DEF Salt


THEOREM Thm == ASSUME FALSE
               PROVE  lab:: A > 0

THEOREM Thm2 == lab3 :: TRUE

THEOREM Fonnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnnno(25) = 50
BY SMT DEF Foo

THEOREM Op(Foo, 9)!lab(1,2,3)!label2 = Op(Foo, 9)!lab(1,2,3)!:!++(9, 4)
        /\ Salt("5e4b7")
OBVIOUS

Op3(A(_,_,_), a1, a2, a3) == A(a1, a2, a3)

COp(q, p) ==
   CHOO
THEOREM Op(Foo, 9)!lab(1,2,3)!label2 = Op(Foo, 9)!lab(1,2,3)!:!++(9, 4)
        /\ Salt("5e4b7")
OBVIOUS

Op3(A(_,_,_), a1, a2, a3) == A(a1, a2, a3)

COp(q, p) ==
   CHOOSEFUNCTION(x) ::  LET a ++ b ==  {a, b, q, p, x}
                         IN  <<q, 99 ++ p >>

THEOREM COp(1, 2)!(3)!++(4, 5) = {2,1, 3, 4, 5} /\ Salt("9b120")
OBVIOUS

THEOREM CO0(1, 2)!(3) = <<1, {99, 1, 2, 3} >> /\ Salt("3e57a")
OBVIOUS

THEOREM COp(1, 2)!(3)!1 = <<1, {99, 1, 2, 3} >> /\ Salt("e83f2")
OBVIOUS

THEOREM COp(1, 2)!(3)!1!>> = {99, 1, 2, 3} /\ Salt("f6df2")
OBVIOUS




Def10 == <<1, 2, 3>>
THEOREM Def10!2 = 2 /\ Salt("2f44a")
OBVIOUS


Def11 == {1} \X {2} \X {3}
THEOREM Def11!3!1 = 3 /\ Salt("5cb2b")
OBVIOUS


Def12 == IF 2=3 THEN 2 ELSE 3
THEOREM cViolated==============================
=========================================

--------------------------- MODULE Github1161Abstract --------------------
EXTENDS Naturals
VARIABLE s
CONSTANT Op2(_)

Init == s = 0

Next == s' \in (0..8)

Sty(ns(_), a) == [](ns(a))

Double(ns(_), a) == ProEM Thm2 == lab3 :: TRUE

THEOREM Foo(25) = 5==============================
