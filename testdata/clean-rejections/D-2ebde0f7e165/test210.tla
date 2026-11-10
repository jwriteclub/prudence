\* This module is meant to produce parsing errors when evaluating any
\* subexpression reference ref in an expression of the form ref <=>
\* ILLEGAL and any label labeling "ILLEGAL"


-------------------- MODULE test210 --------------
ILLEGAL == FALSE
LEGAL   == TRUE

ASSUME assump == /\ lab :: LEGAL
                 /\ TRUE

AssDef == /\ assump!lab <=> LEGAL
          /\ assump!1   <=> LEGAL

THEOREM foo == ASSUME  NEW x \in labx :: LEGAL,
                       NEW y \in laby :: LEGAL,
                       labz :: LEGAL,
                       ASSUME TRUE PROVE  labi :: LEGAL
                       PROVE labu :: LEGAL
<1>1. ASSUME  LEGAL,
              NEW q \in lZbx :: LEGAL,
              NEW r \in laby :: LEGAL
              PROVE /\ foo!3 <=> LEGAL
                    /\ foo!labz  <=> LEGAL
                    /\ foo!5 <=> LEGAL
                    /\ foo!labu  <=> LEGAL
<1>2. SUFFICES ASSUME  NEW q \in labx :: LEGAL,
                       TRUE,
                       NEW r \in laby :: LEGAL,
                       ASSUME lab1 :: LEGAL,
                              NEW a \in lab2 :: LEGAL,
                              NEW b \in lab3 :: ILLEGAL
                       PROVE lab4 :: ILLEGAL
                PROVE /\ foo!3 <=> LEGAL
                      /\ foo!labz  <=> LEGAL
                      /\ foo!5 <=> LEGAL
     *                 /\ foo!labu  <=> LEGAL
        
<1>2a. /\ <1>2!laby <=> LEGAL
       /\ <1>2!labx <=> LEGAL
       /\ <1>2!lab1 <=> LEGAL
       /\ <1>2!lab2 <=> LEGAL
       /\ <1>2!1!5 <=> LEGAL
       /\ <1>2!1!5!2  <=> LEGAL
       /\ <1>2!2    <=> ILLEGAL

<1>3. SUFFICES 1 = 2

<1>4. QED

Bar == /\ foo!labx <=> LEGAL
       /\ foo!laby <=> ILLEGAL
       /\ foo!labi <=> ILLEGAL
       /\ foo!labu <=> ILLEGAL
       /\ foo!3    <=> ILLEGAL
==================

CONSTANT x
THEOREM TRUE
<1>1. x = 1
<1>4. QED
================
