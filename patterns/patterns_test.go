package patterns_test

import (
	"testing"

	regel "github.com/sodiqscript111/Regel"
	"github.com/sodiqscript111/Regel/patterns"
	"github.com/sodiqscript111/Regel/regeltest"
)

type pair struct {
	name   string
	broken *regel.Spec
	fixed  *regel.Spec
	rule   string
}

func newPair(name, rule string, broken, fixed *regel.Spec) pair {
	return pair{name: name, broken: broken, fixed: fixed, rule: rule}
}

func pairs() []pair {
	var all []pair
	add := func(name, rule string, specs func() (*regel.Spec, *regel.Spec)) {
		broken, fixed := specs()
		all = append(all, newPair(name, rule, broken, fixed))
	}
	add("CheckThenAct limit 1, 2 workers", "WithinLimit", func() (*regel.Spec, *regel.Spec) { return patterns.CheckThenAct(1, 2) })
	add("CheckThenAct limit 2, 3 workers", "WithinLimit", func() (*regel.Spec, *regel.Spec) { return patterns.CheckThenAct(2, 3) })
	add("LostUpdate 2 workers", "NoLostUpdate", func() (*regel.Spec, *regel.Spec) { return patterns.LostUpdate(2) })
	add("LostUpdate 3 workers", "NoLostUpdate", func() (*regel.Spec, *regel.Spec) { return patterns.LostUpdate(3) })
	add("LockOrderDeadlock 1 each", regel.NoDeadlock, func() (*regel.Spec, *regel.Spec) { return patterns.LockOrderDeadlock(1) })
	add("LockOrderDeadlock 2 each", regel.NoDeadlock, func() (*regel.Spec, *regel.Spec) { return patterns.LockOrderDeadlock(2) })
	add("DoubleProcessing 1 job, 2 workers", "RunsOnce", func() (*regel.Spec, *regel.Spec) { return patterns.DoubleProcessing(1, 2) })
	add("DoubleProcessing 2 jobs, 2 workers", "RunsOnce", func() (*regel.Spec, *regel.Spec) { return patterns.DoubleProcessing(2, 2) })
	return all
}

func TestBrokenVersionsFailForTheExpectedReason(t *testing.T) {
	for _, p := range pairs() {
		t.Run(p.name, func(t *testing.T) {
			regeltest.Fails(t, p.broken, p.rule)
		})
	}
}

func TestFixedVersionsPass(t *testing.T) {
	for _, p := range pairs() {
		t.Run(p.name, func(t *testing.T) {
			regeltest.Holds(t, p.fixed)
		})
	}
}

func TestBrokenVersionPassesWhenThereIsNoContention(t *testing.T) {
	broken, _ := patterns.CheckThenAct(2, 2)
	regeltest.Holds(t, broken)
	lone, _ := patterns.LostUpdate(1)
	regeltest.Holds(t, lone)
	single, _ := patterns.DoubleProcessing(2, 1)
	regeltest.Holds(t, single)
}

func TestPatternsGenerateValidSpecs(t *testing.T) {
	for _, p := range pairs() {
		t.Run(p.name, func(t *testing.T) {
			if err := p.broken.Validate(); err != nil {
				t.Errorf("broken: %v", err)
			}
			if err := p.fixed.Validate(); err != nil {
				t.Errorf("fixed: %v", err)
			}
		})
	}
}

func TestBadParametersReturnErrorsInsteadOfPanicking(t *testing.T) {
	specs := map[string]func() (*regel.Spec, *regel.Spec){
		"CheckThenAct":      func() (*regel.Spec, *regel.Spec) { return patterns.CheckThenAct(1, 0) },
		"LostUpdate":        func() (*regel.Spec, *regel.Spec) { return patterns.LostUpdate(-1) },
		"LockOrderDeadlock": func() (*regel.Spec, *regel.Spec) { return patterns.LockOrderDeadlock(0) },
		"DoubleProcessing":  func() (*regel.Spec, *regel.Spec) { return patterns.DoubleProcessing(1, -3) },
	}
	for name, build := range specs {
		t.Run(name, func(t *testing.T) {
			broken, fixed := build()
			if err := broken.Validate(); err == nil {
				t.Error("broken: Validate() = nil")
			}
			if err := fixed.Validate(); err == nil {
				t.Error("fixed: Validate() = nil")
			}
		})
	}
}
