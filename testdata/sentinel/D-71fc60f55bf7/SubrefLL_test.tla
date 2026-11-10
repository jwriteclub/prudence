(* Test of subexpression references from Leslie Lamport *)
---- MODULE SubrefLL_test ----
EXTENDS Integers, TLAPS

CONSTANT C
ASSUME c4 == C = 4
USE c4

Salt(s) == s = s
USE DEF Salt

Op(Arg(_), p) ==
   \A x \in {1,2}, y, z \in {<<3, 4>>} :
                      lab(x,y,z) ::  LET a ++ b ==
                                          Arg(a) + b + x + 2*y + 3*z
                                     IN  (Arg(1) + Arg(p) + C +
                                            2*x  + 4*y + 6*z) =
                                         1 +
                      lab(x,y,z) :>  LET a ++ b ==
                                          Arg(a) + b + x + 2*y + 3*z
                                     IN  (Arg(1) + Arg(p) + C +
                                            2*x  + 4*y + 6*z) =
                                         1 ++ ( label2 :: p ++ C)

Foo(PICKu) == 2*u

ASSUME Foo(5) = Foo(5)!:

THEOREM Thm == ASSUME FALSE
               PROVE  lab:: C > 0

THEOREM Thm2 == lab3 :: TRUE

THEOREM Foo(25) = 50
BY SMT DEF Foo

THEOREM Op(Foo, 9)!lab(1,2,3)!label2 = Op(Foo, 9)!lab(1,2,3)!:!++(9, 4)
        /\ Salt("5e4b7")
OBVIOUS

Op3(A(_,_,_), a1, a2, a3) == A(a1, a2, a3)

THEOREM /\ Def13!(42)!1!<<!<< = 42
        /\ Def13!(42)!1!>> = 1
        /\ Def13!(42)!2!>> = 2
        /\ Def13!(42)!3!>> = 3 /\ Salt("ed4e7")
OBVIOUS

Def23 == \EE x , y, z : /\ ((x = 1))
                        /\ y = 2
         + ( label2 :: p ++ C)

Foo(PICKu) == 2*u

ASSUME Foo(5) = Foo(5)!:

THEOREM Thm == ASSUME FALSE
               PROVE  lab:: C > 0

THEOREM Thm2 == lab3 :: TRUE

THEOREM Foo(25) = 50
BY SMT DEF Foo
THEOREM Def2!1 = {1, 2, 3} /\ Salt("427cf")
OBVIOUS

THEOREM Def2!(14)!<< = 14 /\ Salt("441e8")
OBVIOUS


Def3 == {<<x, y>> : x \in {1}, y \in {2}}
THEOREM Def3!2!1 = 2 /\ Salt("1b17c")
OBVIOUS

THEOREM Def3!(1, 2)!<< = 1 /\ Salt("d266a")
OBVIOUS

Def4 == SUBSET UNION {{1}, {2}}
THEOREM Def4!<<!<<!2 = {2} /\ Salt("09b61")
OBVIOUS

Def5 == [i \in 1..10 |-> i+1][3]
THEOREM Def5!>> = 3 /\ Salt("424b4")
OBVIOUS

THEOREM Def5!<<[4] = [i \in 1 .. 10 |-> i + 1][4] /\ Salt("564c2")
OBVIOUS

THEOREM Def5!<<!1!>> = 10 /\ Salt("db493")
OBVIOUS

Def6 == [[i \in 1..10 |-> i+1] EXCEPT ![2] = 1, ![3] = 2]
THEOREM Def6!:[3] = [i \in 1..10 |-> i+1][3] /\ Salt("254bd")
OBVIOUS

Def7 == 3.a
THEOREM Def7!1 = 3 /\ Salt("a1f95")
OBVIOUS

THEOREM Def7!2 = "a" /\ Salt("00d13")
OBVIOUS


Def8 == [a |-> 1, b |-> 2]
THEOREM Def8!2 = 2 /\ Salt("1caa0")
OBVIOUS


Def9 == [a : 1, b : 2]
THEOREM Def9!2 = 2 /\ Salt("a0120")
OBVIOUS


Def10 == <<1, 2, 3>>
THEOREM Def10!2 = 2 /\ Salt("2f44a")
OBVIOUS


Def11 == {1} \X {2} \X {3}
THEOREM Def11!3!1 = 3 /\ Salt("5cb2b")
OBVIOUS


Def12 == IF 2=3 THEN 2 ELSE 3
THEOREM /\ Def12!1!<< = 2
        /\ Def12!2 = 2
        /\ Def12!3 = 3 /\ Salt("1fc6d")
OBVIOUS


Def13 == \A x : CASE x=1 -> 1
                  [] x=2 -> 2
                  [] OTHER -> 3
THEOREM /\ Def13!(42)!1!<<!<< = 42
        /\ Def13!(42)!1!>> = 1
        /\ Def13!(42)!2!>> = 2
        /\ Def13!(42)!3!>> = 3 /\ Salt("ed4e7")
OBVIOUS

Def23 == \EE x , y, z : /\ ((x = 1))
                        /\ y = 2
                        /\ (z=3)
THEOREM /\ Def23!(2,3,4)!1!1 = 2
        /\ Def23!(2,3,4)!2!1 = 3 /\ Salt("92077")
OBVIOUS

=============================================================================
