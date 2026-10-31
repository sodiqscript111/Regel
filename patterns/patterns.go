package patterns

import (
	"fmt"

	regel "github.com/sodiqscript111/Regel"
)

func workerNames(prefix string, count int) []string {
	names := make([]string, 0, max(count, 0))
	for i := 1; i <= count; i++ {
		names = append(names, fmt.Sprintf("%s%d", prefix, i))
	}
	return names
}

func CheckThenAct(limit, workers int) (broken, fixed *regel.Spec) {
	rule := fmt.Sprintf("used <= %d", limit)
	room := fmt.Sprintf("used < %d", limit)

	broken = regel.New("CheckThenAct").
		Var("used", 0).
		Workers(workerNames("w", workers)...).
		Step("check", regel.If(room).Goto("act").Else("done")).
		Step("act", regel.Set("used", "used + 1")).
		Always("WithinLimit", rule)

	fixed = regel.New("CheckThenActFixed").
		Var("used", 0).
		Workers(workerNames("w", workers)...).
		Step("act", regel.Set("used", fmt.Sprintf("IF %s THEN used + 1 ELSE used", room))).
		Always("WithinLimit", rule)
	return broken, fixed
}

func LostUpdate(workers int) (broken, fixed *regel.Spec) {
	rule := fmt.Sprintf(`(\A self \in Workers : pc[self] = "done") => counter = %d`, workers)

	broken = regel.New("LostUpdate").
		Var("counter", 0).
		Local("seen", 0).
		Workers(workerNames("w", workers)...).
		Step("read", regel.Set("seen", "counter")).
		Step("write", regel.Set("counter", "seen[self] + 1")).
		Always("NoLostUpdate", rule)

	fixed = regel.New("LostUpdateFixed").
		Var("counter", 0).
		Workers(workerNames("w", workers)...).
		Step("add", regel.Set("counter", "counter + 1")).
		Always("NoLostUpdate", rule)
	return broken, fixed
}

func takeLock(lock string) []regel.Op {
	return []regel.Op{regel.Await("~" + lock), regel.Set(lock, "TRUE")}
}

func releaseLocks() []regel.Op {
	return []regel.Op{regel.Set("lockA", "FALSE"), regel.Set("lockB", "FALSE")}
}

func locks(name string) *regel.Spec {
	return regel.New(name).Var("lockA", false).Var("lockB", false)
}

func LockOrderDeadlock(workersPerOrder int) (broken, fixed *regel.Spec) {
	broken = locks("LockOrderDeadlock")
	broken.Program("forward", workerNames("forward", workersPerOrder)...).
		Step("forward_a", takeLock("lockA")...).
		Step("forward_b", takeLock("lockB")...).
		Step("forward_release", releaseLocks()...)
	broken.Program("backward", workerNames("backward", workersPerOrder)...).
		Step("backward_b", takeLock("lockB")...).
		Step("backward_a", takeLock("lockA")...).
		Step("backward_release", releaseLocks()...)
	broken.Finishes()

	fixed = locks("LockOrderDeadlockFixed")
	fixed.Program("forward", workerNames("forward", workersPerOrder)...).
		Step("forward_a", takeLock("lockA")...).
		Step("forward_b", takeLock("lockB")...).
		Step("forward_release", releaseLocks()...)
	fixed.Program("backward", workerNames("backward", workersPerOrder)...).
		Step("backward_a", takeLock("lockA")...).
		Step("backward_b", takeLock("lockB")...).
		Step("backward_release", releaseLocks()...)
	fixed.Finishes()
	return broken, fixed
}

func DoubleProcessing(jobs, workers int) (broken, fixed *regel.Spec) {
	waiting := fmt.Sprintf(`(1..%d) \ claimed`, jobs)
	nothingLeft := waiting + " = {}"
	pick := fmt.Sprintf(`IF %s THEN 0 ELSE CHOOSE j \in %s : TRUE`, nothingLeft, waiting)
	rule := fmt.Sprintf(`\A j \in 1..%d : runs[j] <= 1`, jobs)
	queue := func(name string) *regel.Spec {
		return regel.New(name).
			Var("claimed", regel.Raw("{}")).
			Var("runs", regel.Raw(fmt.Sprintf(`[j \in 1..%d |-> 0]`, jobs))).
			Local("job", 0).
			Workers(workerNames("w", workers)...)
	}
	run := regel.Set("runs", "[runs EXCEPT ![job[self]] = @ + 1]")

	broken = queue("DoubleProcessing").
		Step("look", regel.Set("job", pick), regel.If(nothingLeft).Goto("done").Else("claim")).
		Step("claim", regel.Set("claimed", `claimed \cup {job[self]}`)).
		Step("run", run, regel.Goto("look")).
		Always("RunsOnce", rule)

	fixed = queue("DoubleProcessingFixed").
		Step("claim",
			regel.Set("job", pick),
			regel.Set("claimed", fmt.Sprintf(`claimed \cup (IF %s THEN {} ELSE {CHOOSE j \in %s : TRUE})`, nothingLeft, waiting)),
			regel.If(nothingLeft).Goto("done").Else("run")).
		Step("run", run, regel.Goto("claim")).
		Always("RunsOnce", rule)
	return broken, fixed
}
