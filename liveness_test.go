package regel_test

import (
	"strings"
	"testing"

	regel "github.com/sodiqscript111/Regel"
)

func spinner() *regel.Spec {
	return regel.New("Spinner").
		Var("tries", 0).
		Workers("a").
		Step("try", regel.Set("tries", "(tries + 1) % 2"), regel.If("tries = 5").Goto("done").Else("try"))
}

func TestEventuallyGeneratesATemporalProperty(t *testing.T) {
	spec := withdrawFixed().Eventually("Spent", "balance < 100")
	source, config := generated(t, spec)
	for _, text := range []string{
		"Spent == <>(balance < 100)",
		`Spec == Init /\ [][Next]_vars /\ \A self \in Workers : WF_vars(Worker(self))`,
	} {
		if !strings.Contains(source, text) {
			t.Errorf("source does not contain %q:\n%s", text, source)
		}
	}
	if !strings.Contains(config, "PROPERTIES\n    Spent\n") {
		t.Errorf("config does not list the property:\n%s", config)
	}
	if strings.Contains(strings.Split(config, "PROPERTIES")[0], "Spent") {
		t.Errorf("config lists the property as an invariant:\n%s", config)
	}
}

func TestFinishesMeansEveryWorkerReachesDone(t *testing.T) {
	source, config := generated(t, withdrawFixed().Finishes())
	if !strings.Contains(source, `Finishes == <>(\A self \in Workers : pc[self] = "done")`) {
		t.Errorf("source:\n%s", source)
	}
	if !strings.Contains(config, "PROPERTIES\n    Finishes\n") {
		t.Errorf("config:\n%s", config)
	}
}

func TestNoFairnessWithoutLiveness(t *testing.T) {
	source, config := generated(t, withdrawFixed())
	if strings.Contains(source, "WF_vars") {
		t.Errorf("source has fairness:\n%s", source)
	}
	if strings.Contains(config, "PROPERTIES") {
		t.Errorf("config has properties:\n%s", config)
	}
}

func TestLivenessValidation(t *testing.T) {
	tests := []struct {
		name string
		spec *regel.Spec
		want string
	}{
		{"empty expression", withdrawFixed().Eventually("Later", ""), `rule "Later": expression is empty`},
		{"invalid name", withdrawFixed().Eventually("not valid", "TRUE"), `rule "not valid" is not a valid TLA+ identifier`},
		{"reserved name", withdrawFixed().Eventually("Next", "TRUE"), `rule "Next" clashes with a reserved name`},
		{"same name as an invariant", withdrawFixed().Eventually("NoOverdraft", "TRUE"), `rule "NoOverdraft" is declared more than once`},
		{"finishes twice", withdrawFixed().Finishes().Finishes(), `rule "Finishes" is declared more than once`},
		{"rule named like deadlock", withdrawFixed().Always(regel.NoDeadlock, "TRUE"), `rule "NoDeadlock" clashes with a reserved name`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.spec.Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestNilSpecLivenessDoesNotPanic(t *testing.T) {
	var spec *regel.Spec
	if err := spec.Eventually("Later", "TRUE").Finishes().Validate(); err == nil {
		t.Fatal("Validate() = nil, want an error for a nil spec")
	}
}

func TestSANYAcceptsLiveness(t *testing.T) {
	java, jar := tlcTools(t)
	source, _ := generated(t, withdrawFixed().Finishes().Eventually("Spent", "balance < 100"))
	if err := sany(java, jar, t.TempDir(), "WithdrawFixed", source); err != nil {
		t.Fatal(err)
	}
}

func TestCheckFinishesPasses(t *testing.T) {
	result := check(t, withdrawFixed().Finishes())
	if !result.Passed() {
		t.Fatalf("Verdict = %q\n%s", result.Verdict, result.RawOutput)
	}
	if result.StatesGenerated != 6 || result.DistinctStates != 4 {
		t.Errorf("states = %d generated, %d distinct, want 6, 4", result.StatesGenerated, result.DistinctStates)
	}
}

func TestCheckWorkerThatLoopsForeverFailsFinishes(t *testing.T) {
	result := check(t, spinner().Finishes())
	if result.Verdict != regel.PropertyViolated {
		t.Fatalf("Verdict = %q, want %q\n%s", result.Verdict, regel.PropertyViolated, result.RawOutput)
	}
	if result.ExitCode != 13 {
		t.Errorf("ExitCode = %d, want 13", result.ExitCode)
	}
	if result.Violated != "Finishes" {
		t.Errorf("Violated = %q, want Finishes", result.Violated)
	}
	if result.LoopsTo == 0 {
		t.Errorf("trace does not show the loop:\n%s", result)
	}
	if !strings.Contains(result.String(), "repeats forever") {
		t.Errorf("printed result does not show the loop:\n%s", result)
	}
	if len(result.Trace) == 0 || result.Trace[len(result.Trace)-1].Action != "try" {
		t.Errorf("trace = %v", result.Trace)
	}
}

func TestCheckFairnessIsPerWorker(t *testing.T) {
	spec := regel.New("Fair").
		Var("count", 0).
		Workers("a", "b").
		Step("first", regel.Set("count", "count + 1")).
		Step("second", regel.Set("count", "count + 1")).
		Finishes()
	result := check(t, spec)
	if !result.Passed() {
		t.Fatalf("Verdict = %q\n%s", result.Verdict, result)
	}
}

func TestCheckNamesTheBrokenPropertyAmongSeveral(t *testing.T) {
	tests := []struct {
		name string
		spec *regel.Spec
		want string
	}{
		{
			"second property is broken",
			spinner().Eventually("Started", "pc[CHOOSE self \\in Workers : TRUE] = \"try\"").Finishes(),
			"Finishes",
		},
		{
			"first property is broken",
			spinner().Finishes().Eventually("Started", "pc[CHOOSE self \\in Workers : TRUE] = \"try\""),
			"Finishes",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := check(t, test.spec)
			if result.Verdict != regel.PropertyViolated || result.Violated != test.want {
				t.Fatalf("Verdict = %q, Violated = %q, want %q\n%s", result.Verdict, result.Violated, test.want, result.RawOutput)
			}
			if result.LoopsTo == 0 {
				t.Errorf("trace does not show the loop:\n%s", result)
			}
		})
	}
}

func TestCheckInvariantStillWinsOverLiveness(t *testing.T) {
	result := check(t, withdraw().Finishes())
	if result.Verdict != regel.InvariantViolated || result.Violated != "NoOverdraft" {
		t.Fatalf("Verdict = %q, Violated = %q\n%s", result.Verdict, result.Violated, result.RawOutput)
	}
}

func TestCheckNamesDeadlock(t *testing.T) {
	spec := regel.New("Blocked").Var("open", false).Workers("a").Step("wait", regel.Await("open"))
	result := check(t, spec)
	if result.Verdict != regel.Deadlock || result.Violated != regel.NoDeadlock {
		t.Fatalf("Verdict = %q, Violated = %q", result.Verdict, result.Violated)
	}
}
