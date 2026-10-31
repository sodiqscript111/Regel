---- MODULE TwoPrograms ----
EXTENDS Integers, TLC

CONSTANTS Workers_writer, Workers_reader

Workers == Workers_writer \cup Workers_reader

VARIABLES box, seen, pc

vars == <<box, seen, pc>>

Init ==
    /\ box = 0
    /\ seen = [self \in Workers |-> 0]
    /\ pc = [self \in Workers |-> CASE self \in Workers_writer -> "write" [] self \in Workers_reader -> "wait"]

Step_write(self) ==
    /\ pc[self] = "write"
    /\ box' = (1)
    /\ pc' = [pc EXCEPT ![self] = "done"]
    /\ UNCHANGED seen

Step_wait(self) ==
    /\ pc[self] = "wait"
    /\ box = 1
    /\ pc' = [pc EXCEPT ![self] = "read"]
    /\ UNCHANGED <<box, seen>>

Step_read(self) ==
    /\ pc[self] = "read"
    /\ seen' = [seen EXCEPT ![self] = box]
    /\ pc' = [pc EXCEPT ![self] = "done"]
    /\ UNCHANGED box

Finished ==
    /\ \A self \in Workers : pc[self] = "done"
    /\ UNCHANGED vars

Worker_writer(self) ==
    \/ Step_write(self)

Worker_reader(self) ==
    \/ Step_wait(self)
    \/ Step_read(self)

Next == (\E self \in Workers_writer : Worker_writer(self)) \/ (\E self \in Workers_reader : Worker_reader(self)) \/ Finished

Spec == Init /\ [][Next]_vars /\ (\A self \in Workers_writer : WF_vars(Worker_writer(self))) /\ (\A self \in Workers_reader : WF_vars(Worker_reader(self)))

TypeOK == pc \in [Workers -> {"write", "wait", "read", "done"}]

BoxIsABit == box \in {0, 1}

Finishes == <>(\A self \in Workers : pc[self] = "done")

====
