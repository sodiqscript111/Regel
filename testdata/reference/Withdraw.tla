------------------------------ MODULE Withdraw ------------------------------
EXTENDS Integers

CONSTANTS Workers, InitialBalance, Amount

VARIABLES balance, pc

vars == <<balance, pc>>

Init ==
    /\ balance = InitialBalance
    /\ pc = [w \in Workers |-> "check"]

Check(w) ==
    /\ pc[w] = "check"
    /\ pc' = [pc EXCEPT ![w] =
                IF balance >= Amount THEN "debit" ELSE "done"]
    /\ UNCHANGED balance

Debit(w) ==
    /\ pc[w] = "debit"
    /\ balance' = balance - Amount
    /\ pc' = [pc EXCEPT ![w] = "done"]

Finished ==
    /\ \A w \in Workers : pc[w] = "done"
    /\ UNCHANGED vars

Next ==
    \/ \E w \in Workers : Check(w) \/ Debit(w)
    \/ Finished

Spec == Init /\ [][Next]_vars

TypeOK ==
    /\ balance \in Int
    /\ pc \in [Workers -> {"check", "debit", "done"}]

NoOverdraft == balance >= 0

=============================================================================
