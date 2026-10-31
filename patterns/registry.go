package patterns

import (
	"strings"

	regel "github.com/sodiqscript111/Regel"
)

type Pattern struct {
	Name    string
	Summary string
	Rule    string
	Build   func() (broken, fixed *regel.Spec)
}

func All() []Pattern {
	return []Pattern{
		{
			Name:    "CheckThenAct",
			Summary: "Workers check a limit, then act on it in a separate step",
			Rule:    "WithinLimit",
			Build:   func() (*regel.Spec, *regel.Spec) { return CheckThenAct(1, 2) },
		},
		{
			Name:    "LostUpdate",
			Summary: "Workers read a counter, then write back the old value plus one",
			Rule:    "NoLostUpdate",
			Build:   func() (*regel.Spec, *regel.Spec) { return LostUpdate(2) },
		},
		{
			Name:    "LockOrderDeadlock",
			Summary: "Two programs take the same two locks in opposite orders",
			Rule:    regel.NoDeadlock,
			Build:   func() (*regel.Spec, *regel.Spec) { return LockOrderDeadlock(1) },
		},
		{
			Name:    "DoubleProcessing",
			Summary: "Workers pick a job from a queue, then claim it in a separate step",
			Rule:    "RunsOnce",
			Build:   func() (*regel.Spec, *regel.Spec) { return DoubleProcessing(1, 2) },
		},
	}
}

func Find(name string) (Pattern, bool) {
	for _, pattern := range All() {
		if strings.EqualFold(pattern.Name, name) {
			return pattern, true
		}
	}
	return Pattern{}, false
}
