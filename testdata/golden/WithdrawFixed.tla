---- MODULE WithdrawFixed ----
EXTENDS Integers, TLC

CONSTANT Workers

VARIABLES balance, pc

vars == <<balance, pc>>

Init ==
    /\ balance = 100
    /\ pc = [self \in Workers |-> "withdraw"]

Step_withdraw(self) ==
    /\ pc[self] = "withdraw"
    /\ balance' = (IF balance >= 60 THEN balance - 60 ELSE balance)
    /\ pc' = [pc EXCEPT ![self] = "done"]

Finished ==
    /\ \A self \in Workers : pc[self] = "done"
    /\ UNCHANGED vars

Worker(self) ==
    \/ Step_withdraw(self)

Next == (\E self \in Workers : Worker(self)) \/ Finished

Spec == Init /\ [][Next]_vars

TypeOK == pc \in [Workers -> {"withdraw", "done"}]

NoOverdraft == balance >= 0

====
