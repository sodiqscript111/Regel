# Regel

Regel (German for "rule") lets you write [TLA+](https://lamport.azurewebsites.net/tla/tla.html)
models in readable Go. It generates the real `.tla` and `.cfg` files, runs the real
TLC model checker, and gives you the result as Go data.

Regel never checks anything itself. TLC does the checking, so a Regel result means
what a TLC result means.

## Why

Concurrency bugs hide in the order things happen. Tests run one order at a time, and
the bad order may only appear in production. TLC tries every order, but TLA+ is a
separate language that many teams never get round to learning.

Regel keeps the part that matters. You describe your design in Go, one step per
critical section, and TLC finds the order that breaks it:

```go
spec := regel.New("Withdraw").
	Var("balance", 100).
	Workers("a", "b").
	Step("check", regel.If("balance >= 60").Goto("debit").Else("done")).
	Step("debit", regel.Set("balance", "balance - 60")).
	Always("NoOverdraft", "balance >= 0")
```

```text
FAILED: NoOverdraft was broken after 5 steps

1  start     balance=100  a=check  b=check
2  a: check  balance=100  a=debit  b=check
3  b: check  balance=100  a=debit  b=debit
4  a: debit  balance=40   a=done   b=debit
5  b: debit  balance=-20  a=done   b=done
```

Both workers saw enough money, then both took it. That's the bug, found from the
design alone, with no Go code to run and no timing to get lucky with.

## Install

You need Go 1.25 or newer. Add the library to your module:

```bash
go get github.com/sodiqscript111/Regel
```

Install the command-line tool if you want it:

```bash
go install github.com/sodiqscript111/Regel/cmd/regel@latest
```

The module path has a capital R. The package is called `regel`:

```go
import regel "github.com/sodiqscript111/Regel"
```

### Java and tla2tools.jar

Regel runs TLC, which needs Java and `tla2tools.jar`. CI tests with Java 21 and TLC
2.19, from the [v1.7.4 release](https://github.com/tlaplus/tlaplus/releases/tag/v1.7.4)
of the TLA+ tools.

1. Install Java. [Eclipse Temurin](https://adoptium.net/) works.
2. Download the jar:

   ```bash
   curl -L -o tla2tools.jar https://github.com/tlaplus/tlaplus/releases/download/v1.7.4/tla2tools.jar
   ```

3. Tell Regel where the jar is:

   ```bash
   export REGEL_TLA2TOOLS="$PWD/tla2tools.jar"
   ```

4. If `java` isn't on your `PATH`, tell Regel where that is too:

   ```bash
   export REGEL_JAVA=/path/to/java
   ```

5. Check the setup:

   ```bash
   regel doctor
   ```

   It prints where it found each tool, or `NOT FOUND` and how to fix it.

Regel looks for Java in this order: `regel.WithJava`, then `REGEL_JAVA`, then `java` on
your `PATH`. It looks for the jar through `regel.WithJar`, then `REGEL_TLA2TOOLS`. The
jar has no default location.

## Try it

From a clone of this repository:

```bash
go run ./examples/withdraw
```

```text
FAILED: NoOverdraft was broken after 5 steps

1  start     balance=100  a=check  b=check
2  a: check  balance=100  a=debit  b=check
3  b: check  balance=100  a=debit  b=debit
4  a: debit  balance=40   a=done   b=debit
5  b: debit  balance=-20  a=done   b=done

PASSED: 6 states generated, 4 distinct
```

The fix makes the check and the debit one atomic step:

```go
spec := regel.New("WithdrawFixed").
	Var("balance", 100).
	Workers("a", "b").
	Step("withdraw", regel.Set("balance", "IF balance >= 60 THEN balance - 60 ELSE balance")).
	Always("NoOverdraft", "balance >= 0").
	Finishes()
```

## Writing a spec

| Call | Meaning |
|---|---|
| `New(name)` | Starts a spec. The name becomes the TLA+ module name. |
| `Var(name, value)` | A shared variable. |
| `Local(name, value)` | A variable each worker has its own copy of. |
| `Workers(names...)` | The workers that run the steps. |
| `Step(label, ops...)` | One atomic action. |
| `Always(name, expr)` | A rule that must hold in every state. |
| `Eventually(name, expr)` | A rule that must become true at some point. |
| `Finishes()` | A rule called `Finishes`: every worker reaches `done`. |
| `Program(name, workers...)` | Workers that run their own steps. |

Initial values can be `int`, `bool`, `string`, or `Raw(expr)` for a raw TLA+ value.

Inside a step:

| Operation | Meaning |
|---|---|
| `Set(variable, expr)` | Writes a variable. A step can write each variable once. |
| `Await(condition)` | The step can only run while the condition is true. |
| `Goto(label)` | Moves to that step instead of the next one. |
| `If(condition).Goto(a).Else(b)` | Moves to `a` or `b`. |

The rules that matter:

- **A step is one atomic action.** One `Step` should match one critical section in
  your real code. Nothing can happen between the operations in a step.
- **After a step, a worker moves to the next one.** After the last step it moves to
  `done`, which is a reserved label. `Goto` and `If` override this.
- **Expressions are raw TLA+.** Regel doesn't parse them. A worker's own name is
  `self`.
- **Locals are per worker.** `Set("tmp", "balance")` writes `tmp[self]`. To read one,
  write `tmp[self]`.
- **Variables a step doesn't set stay unchanged.** Regel handles that for you.

`spec.Validate()` reports every problem in a spec at once. `spec.Source()` and
`spec.Config()` return the generated files. `spec.Check(ctx)` runs TLC:

```go
result, err := spec.Check(ctx)
if err != nil {
	log.Fatal(err)
}
fmt.Println(result.Passed())
fmt.Println(result.Trace)
```

`err` means TLC couldn't run, for example because Java is missing. Otherwise,
`result.Verdict` is one of `Passed`, `InvariantViolated`, `PropertyViolated`,
`Deadlock`, or `ToolError`. `ToolError` means TLC ran and couldn't finish, usually
because of a mistake in an expression, and `fmt.Println(result)` shows TLC's message.
Only a run that exits with code 0 and reports that no error was found counts as a
pass.

`result.Violated` names the broken rule, and `result.Trace` holds the steps. A
deadlock is reported as `regel.NoDeadlock`.

### Different code for different workers

```go
spec := regel.New("Handoff").Var("box", 0)
spec.Program("writer", "w").
	Step("write", regel.Set("box", "1"))
spec.Program("reader", "r").
	Step("wait", regel.Await("box = 1"))
spec.Finishes()
```

Step labels must be unique across a spec.

### In `go test`

```go
func TestBrokenWithdrawOverdraws(t *testing.T) {
	regeltest.Fails(t, broken(), "NoOverdraft")
}

func TestFixedWithdrawHolds(t *testing.T) {
	regeltest.Holds(t, fixed())
}
```

If one of these fails, it prints the readable trace. If TLC isn't available, the test
fails with a clear message. It only skips if you set `REGEL_SKIP_TLC=1`, so a missing
tool can't quietly look like a pass.

## Patterns

The `patterns` package has common concurrency bugs ready to use. Each function takes
sizes and returns a broken spec and a fixed spec.

| Pattern | The bug | Broken version fails with |
|---|---|---|
| `CheckThenAct(limit, workers)` | Workers check a limit, then act on it in a separate step | `WithinLimit`: the limit is exceeded |
| `LostUpdate(workers)` | Workers read a counter, then write back the old value plus one | `NoLostUpdate`: the counter ends up too low |
| `LockOrderDeadlock(workersPerOrder)` | Two programs take the same two locks in opposite orders | `regel.NoDeadlock`: a deadlock |
| `DoubleProcessing(jobs, workers)` | Workers pick a job from a queue, then claim it in a separate step | `RunsOnce`: a job runs twice |

```go
broken, fixed := patterns.LostUpdate(2)
regeltest.Fails(t, broken, "NoLostUpdate")
regeltest.Holds(t, fixed)
```

A broken version only fails if there's something to fight over. `CheckThenAct(2, 2)`
passes, because two workers can't exceed a limit of two.

## Command line

| Command | What it does |
|---|---|
| `regel list` | Lists the patterns |
| `regel show <pattern> [--fixed]` | Prints the generated TLA+ |
| `regel check <pattern> [--fixed]` | Runs TLC and prints the result |
| `regel demo` | Runs a broken pattern, then its fix |
| `regel doctor` | Reports whether Java and `tla2tools.jar` were found |

The exit code is 0 for a pass, 1 for a failed check, and 2 for a usage or tool error.

```bash
regel check LockOrderDeadlock
```

```text
FAILED: deadlock after 3 steps, the workers are stuck

1  start                  lockA=FALSE  lockB=FALSE  forward1=forward_a  backward1=backward_b
2  forward1: forward_a    lockA=TRUE   lockB=FALSE  forward1=forward_b  backward1=backward_b
3  backward1: backward_b  lockA=TRUE   lockB=TRUE   forward1=forward_b  backward1=backward_a
```

```bash
regel show CheckThenAct --fixed
```

```text
---- MODULE CheckThenActFixed ----
EXTENDS Integers, TLC

CONSTANT Workers

VARIABLES used, pc

vars == <<used, pc>>

Init ==
    /\ used = 0
    /\ pc = [self \in Workers |-> "act"]

Step_act(self) ==
    /\ pc[self] = "act"
    /\ used' = (IF used < 1 THEN used + 1 ELSE used)
    /\ pc' = [pc EXCEPT ![self] = "done"]

Finished ==
    /\ \A self \in Workers : pc[self] = "done"
    /\ UNCHANGED vars

Worker(self) ==
    \/ Step_act(self)

Next == (\E self \in Workers : Worker(self)) \/ Finished

Spec == Init /\ [][Next]_vars

TypeOK == pc \in [Workers -> {"act", "done"}]

WithinLimit == used <= 1

====
```

## What Regel can't do

**Regel checks a model of your design, not your Go code.** It never reads or runs your
program. If the code doesn't match the model, a pass tells you nothing about the code.

**A pass only covers the rules you wrote and the sizes you checked.** TLC tries every
order, but only for the spec you gave it. `Withdraw` from a balance of 100 with an
amount of 50 passes with 2 workers and fails with 3. Try bigger sizes, and don't read a
pass as proof of anything beyond them.

**A step that's too big hides bugs.** Everything in a step happens at once. If you put
two critical sections in one step, TLC can't find the race between them.

Some other limits:

- A spec with no end, like a counter that grows forever, has endless states and TLC
  won't finish. Pass a `ctx` with a timeout.
- Regel doesn't parse expressions, so a mistake in one shows up as `ToolError` when TLC
  reads it.
- Steps have no way to choose between outcomes. The only choice in a spec is which
  worker moves next.
- TLC doesn't say which worker took a step that leaves that worker's position unchanged.
  If several workers could have taken it, the trace shows only the step.
- TLC doesn't say which `Eventually` rule broke. If a spec has several and one fails,
  Regel runs TLC again for each until it finds out.
- You need Java and `tla2tools.jar` for everything except generating the files.
