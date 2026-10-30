--------------------------- MODULE WithdrawFixed ---------------------------
EXTENDS Integers

CONSTANTS Workers, InitialBalance, Amount

VARIABLES balance, pc

vars == <<balance, pc>>

Init ==
    /\ balance = InitialBalance
    /\ pc = [w \in Workers |-> "start"]

Withdraw(w) ==
    /\ pc[w] = "start"
    /\ balance' = IF balance >= Amount THEN balance - Amount ELSE balance
    /\ pc' = [pc EXCEPT ![w] = "done"]

Finished ==
    /\ \A w \in Workers : pc[w] = "done"
    /\ UNCHANGED vars

Next ==
    \/ \E w \in Workers : Withdraw(w)
    \/ Finished

Spec == Init /\ [][Next]_vars /\ WF_vars(Next)

TypeOK ==
    /\ balance \in Int
    /\ pc \in [Workers -> {"start", "done"}]

NoOverdraft == balance >= 0

AllFinish == <>(\A w \in Workers : pc[w] = "done")

=============================================================================
