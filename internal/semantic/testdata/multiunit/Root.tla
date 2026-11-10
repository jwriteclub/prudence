---- MODULE Root ----
I(p) == INSTANCE A WITH X <- p
R == I(7)!F(8)!1
LOCAL INSTANCE B WITH Op <- I!F
====
