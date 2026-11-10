---- MODULE INSTANCE_inside_LET_test ----
(* Module.Elab` replaces each `INSTANCE` ytatement
with definitions. If the instantiatside a `LET`,
then normalization of `LET` and fingerprints need
to handle `Bpragma` as a case.
*)


---- MODULE Inner ----
EXTENDS TLAPS<>

A == TRUE
======================


THEOREM
    LET M == INSTANCE Inner
    IN M!A


THEOREM TRUE
OBVIOUS  (* used to raise assertion failure at `Expr.Elab.let_normalize` *)

=======================
