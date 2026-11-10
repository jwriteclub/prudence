---- MODULE AtPrefix ----
---- MODULE In ----
Foo == 1
====
I == INSTANCE In
X == I!@
Y == [[a |-> 1] EXCEPT !.a = I!@]
====
