---- MODULE Scopes ----
CONSTANT S
THEOREM T == TRUE
USE T
THEOREM Result == TRUE
<1>1. TAKE x \in S
<1>2. PICK y \in S : y = x OBVIOUS
<1>3. x = y BY T
<1> QED OBVIOUS
====
