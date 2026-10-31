package regel_test

import (
	"strings"
	"testing"

	regel "github.com/sodiqscript111/Regel"
)

func twoPrograms() *regel.Spec {
	spec := regel.New("TwoPrograms").
		Var("box", 0).
		Local("seen", 0)
	spec.Program("writer", "w").
		Step("write", regel.Set("box", "1"))
	spec.Program("reader", "r1", "r2").
		Step("wait", regel.Await("box = 1")).
		Step("read", regel.Set("seen", "box"))
	return spec.
		Always("BoxIsABit", `box \in {0, 1}`).
		Finishes()
}

func TestProgramGolden(t *testing.T) {
	source, config := generated(t, twoPrograms())
	compareGolden(t, "TwoPrograms.tla", source)
	compareGolden(t, "TwoPrograms.cfg", config)
}

func TestProgramAcceptedShapes(t *testing.T) {
	single := regel.New("M")
	single.Program("only", "a").Step("one")

	mixed := regel.New("M").Workers("a").Step("one")
	mixed.Program("other", "b").Step("two")

	tests := []struct {
		name string
		spec *regel.Spec
	}{
		{"one named program", single},
		{"default program next to a named one", mixed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.spec.Validate(); err != nil {
				t.Fatalf("Validate() = %v", err)
			}
		})
	}
}

func TestProgramValidation(t *testing.T) {
	build := func(configure func(s *regel.Spec)) *regel.Spec {
		spec := regel.New("M").Var("x", 0)
		configure(spec)
		return spec
	}
	tests := []struct {
		name string
		spec *regel.Spec
		want string
	}{
		{
			"duplicate labels across programs",
			build(func(s *regel.Spec) {
				s.Program("one", "a").Step("same")
				s.Program("two", "b").Step("same")
			}),
			`step "same" is declared more than once`,
		},
		{
			"duplicate labels within a program",
			build(func(s *regel.Spec) { s.Program("one", "a").Step("same").Step("same") }),
			`step "same" is declared more than once`,
		},
		{
			"duplicate labels with the default program",
			build(func(s *regel.Spec) {
				s.Workers("a").Step("same")
				s.Program("two", "b").Step("same")
			}),
			`step "same" is declared more than once`,
		},
		{
			"duplicate workers across programs",
			build(func(s *regel.Spec) {
				s.Program("one", "a").Step("first")
				s.Program("two", "a").Step("second")
			}),
			`worker "a" is declared more than once`,
		},
		{
			"duplicate program names",
			build(func(s *regel.Spec) {
				s.Program("one", "a").Step("first")
				s.Program("one", "b").Step("second")
			}),
			`program "one" is declared more than once`,
		},
		{
			"program named like the default program",
			build(func(s *regel.Spec) {
				s.Workers("a").Step("first")
				s.Program("main", "b").Step("second")
			}),
			`program "main" is declared more than once`,
		},
		{
			"empty program name",
			build(func(s *regel.Spec) { s.Program("", "a").Step("first") }),
			"program name is empty",
		},
		{
			"invalid program name",
			build(func(s *regel.Spec) { s.Program("not valid", "a").Step("first") }),
			`program "not valid" is not a valid TLA+ identifier`,
		},
		{
			"reserved program name",
			build(func(s *regel.Spec) { s.Program("Next", "a").Step("first") }),
			`program "Next" clashes with a reserved name`,
		},
		{
			"program without workers",
			build(func(s *regel.Spec) { s.Program("one").Step("first") }),
			`program "one" has no workers`,
		},
		{
			"program without steps",
			build(func(s *regel.Spec) { s.Program("one", "a") }),
			`program "one" has no steps`,
		},
		{
			"goto into another program",
			build(func(s *regel.Spec) {
				s.Program("one", "a").Step("first", regel.Goto("second"))
				s.Program("two", "b").Step("second")
			}),
			`step "first": Goto target "second" does not exist`,
		},
		{
			"name with the worker action prefix",
			build(func(s *regel.Spec) {
				s.Program("one", "a").Step("first")
				s.Always("Worker_one", "TRUE")
			}),
			`rule "Worker_one" clashes with the reserved prefix Worker_`,
		},
		{
			"name with the worker set prefix",
			build(func(s *regel.Spec) {
				s.Program("one", "a").Step("first")
				s.Always("Workers_one", "TRUE")
			}),
			`rule "Workers_one" clashes with the reserved prefix Workers_`,
		},
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

func TestNilProgramDoesNotPanic(t *testing.T) {
	var spec *regel.Spec
	if builder := spec.Program("one", "a").Step("first"); builder != nil {
		t.Fatalf("Step() = %v, want nil", builder)
	}
}

func TestProgramKeepsItsOwnWorkers(t *testing.T) {
	names := []string{"a", "b"}
	spec := regel.New("M")
	spec.Program("one", names...).Step("first")
	names[0] = "changed"
	if _, config := generated(t, spec); !strings.Contains(config, "Workers_one = {a, b}") {
		t.Fatalf("config:\n%s", config)
	}
}

func TestProgramSteps(t *testing.T) {
	source, config := generated(t, twoPrograms())
	for _, text := range []string{
		"CONSTANTS Workers_writer, Workers_reader",
		`Workers == Workers_writer \cup Workers_reader`,
		`/\ pc = [self \in Workers |-> CASE self \in Workers_writer -> "write" [] self \in Workers_reader -> "wait"]`,
		"Worker_writer(self) ==\n    \\/ Step_write(self)\n",
		"Worker_reader(self) ==\n    \\/ Step_wait(self)\n    \\/ Step_read(self)\n",
		`Next == (\E self \in Workers_writer : Worker_writer(self)) \/ (\E self \in Workers_reader : Worker_reader(self)) \/ Finished`,
		`(\A self \in Workers_writer : WF_vars(Worker_writer(self))) /\ (\A self \in Workers_reader : WF_vars(Worker_reader(self)))`,
		`TypeOK == pc \in [Workers -> {"write", "wait", "read", "done"}]`,
	} {
		if !strings.Contains(source, text) {
			t.Errorf("source does not contain %q:\n%s", text, source)
		}
	}
	for _, text := range []string{"Workers_writer = {w}", "Workers_reader = {r1, r2}"} {
		if !strings.Contains(config, text) {
			t.Errorf("config does not contain %q:\n%s", text, config)
		}
	}
}

func TestSANYAcceptsPrograms(t *testing.T) {
	java, jar := tlcTools(t)
	source, _ := generated(t, twoPrograms())
	if err := sany(java, jar, t.TempDir(), "TwoPrograms", source); err != nil {
		t.Fatal(err)
	}
}

func TestCheckProgramsRunDifferentCode(t *testing.T) {
	result := check(t, twoPrograms())
	if !result.Passed() {
		t.Fatalf("Verdict = %q\n%s", result.Verdict, result)
	}
}

func TestCheckProgramTraceNamesTheWorkers(t *testing.T) {
	spec := regel.New("Handoff").Var("x", 0)
	spec.Program("first", "a").Step("add", regel.Set("x", "x + 1"))
	spec.Program("second", "b").Step("add_more", regel.Await("x = 1"), regel.Set("x", "x + 1"))
	spec.Always("Small", "x < 2")

	result := check(t, spec)

	if result.Verdict != regel.InvariantViolated || len(result.Trace) != 3 {
		t.Fatalf("Verdict = %q\n%s", result.Verdict, result)
	}
	if result.Trace[1].Worker != "a" || result.Trace[1].Action != "add" {
		t.Errorf("step 2 = %+v", result.Trace[1])
	}
	if result.Trace[2].Worker != "b" || result.Trace[2].Action != "add_more" {
		t.Errorf("step 3 = %+v", result.Trace[2])
	}
}

func TestCheckDefaultProgramWorksNextToANamedOne(t *testing.T) {
	spec := regel.New("Mixed").Var("x", 0).Workers("a").Step("one", regel.Set("x", "x + 1"))
	spec.Program("other", "b").Step("two", regel.Set("x", "x + 1"))
	spec.Always("AtMostTwo", "x <= 2").Finishes()

	result := check(t, spec)

	if !result.Passed() {
		t.Fatalf("Verdict = %q\n%s", result.Verdict, result)
	}
}

func TestCheckFinishesFailsAcrossPrograms(t *testing.T) {
	spec := regel.New("Forever").Var("n", 0)
	spec.Program("quick", "a").Step("stop")
	spec.Program("spin", "b").Step("turn", regel.Set("n", "1 - n"), regel.Goto("turn"))
	spec.Finishes()

	result := check(t, spec)

	if result.Verdict != regel.PropertyViolated || result.Violated != "Finishes" {
		t.Fatalf("Verdict = %q, Violated = %q\n%s", result.Verdict, result.Violated, result)
	}
}
