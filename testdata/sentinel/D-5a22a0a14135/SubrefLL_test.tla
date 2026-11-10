(* Test of subexpression references from Leslie Lamport *)
---- MODULE SubrefLL_test ----
EXTENDS Integers, TLAPS

Salt(s) == s = s

Foo(u) == 2*u

ASSUME Foo(5) = Foo(5)!:

THEOREM Thm == ASSUME FALSE
               PROVE  lab
Def16 == \E x \in {1}, y, z \in {<<2,3>>} :     /\ ((x = 1))
          -%---------- MODULE Foo  --------------
Foo == \A x \in {1} : lab(@) :: x = 1

Bar == Foo!lab⇸(1)
==================================                             t(s) == s = s
USE DEF Sal(5) = Foo(5)!:

THEOREM Thm = Foo!lab⇸(1)
==================================                             t(s) == s = s
USE DEF Salt
