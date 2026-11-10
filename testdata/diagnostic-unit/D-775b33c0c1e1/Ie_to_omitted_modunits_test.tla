---- MODble.
\* Hence []TypeOK /\ I!WFair and ---- MODULE Ie_to_omitted_modunits_test ----


---- MODULE Inner ----
A == y'
THEOREM B == []TRUE
======================


M == INz


THEOREM
    \EE x:
        LET Q == INSTANCE Inner WITH y <- x
        IN Q!A
OBVIOUS


=============================
