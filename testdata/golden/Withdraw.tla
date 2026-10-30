---- MODULE Withdraw ----
EXTENDS Integers, TLC

CONSTANT Workers

VARIABLES balance, pc

vars == <<balance, pc>>

Init ==
    /\ balance = 100
    /\ pc = [self \in Workers |-> "check"]

Step_check(self) ==
    /\ pc[self] = "check"
    /\ pc' = [pc EXCEPT ![self] = IF balance >= 60 THEN "debit" ELSE "done"]
    /\ UNCHANGED balance

Step_debit(self) ==
    /\ pc[self] = "debit"
    /\ balance' = (balance - 60)
    /\ pc' = [pc EXCEPT ![self] = "done"]

Finished ==
    /\ \A self \in Workers : pc[self] = "done"
    /\ UNCHANGED vars

Worker(self) ==
    \/ Step_check(self)
    \/ Step_debit(self)

Next == (\E self \in Workers : Worker(self)) \/ Finished

Spec == Init /\ [][Next]_vars

TypeOK == pc \in [Workers -> {"check", "debit", "done"}]

NoOverdraft == balance >= 0

====
