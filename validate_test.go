package regel_test

import (
	"strings"
	"testing"

	regel "github.com/sodiqscript111/Regel"
)

func withdraw() *regel.Spec {
	return regel.New("Withdraw").
		Var("balance", 100).
		Workers("a", "b").
		Step("check", regel.If("balance >= 60").Goto("debit").Else("done")).
		Step("debit", regel.Set("balance", "balance - 60")).
		Always("NoOverdraft", "balance >= 0")
}

func TestValidateAcceptsGoodSpecs(t *testing.T) {
	tests := []struct {
		name string
		spec *regel.Spec
	}{
		{"withdraw", withdraw()},
		{"all initial value types", regel.New("M").
			Var("count", 1).Var("flag", true).Var("label", "x").Var("set", regel.Raw("{1, 2}")).
			Local("tmp", 0).
			Workers("a").
			Step("one", regel.Set("tmp", "count"), regel.Set("count", "tmp[self] + 1"))},
		{"await and goto", regel.New("M").
			Var("open", false).
			Workers("a").
			Step("wait", regel.Await("open"), regel.Goto("wait"))},
		{"goto done", regel.New("M").Workers("a").Step("one", regel.Goto("done"))},
		{"several awaits", regel.New("M").Var("x", 0).Workers("a").Step("one", regel.Await("x = 0"), regel.Await("x < 1"))},
		{"step with no operations", regel.New("M").Workers("a").Step("one")},
		{"step sharing a name with a variable", regel.New("M").Var("lock", 0).Workers("a").Step("lock")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.spec.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestValidateRejectsBadSpecs(t *testing.T) {
	tests := []struct {
		name string
		spec *regel.Spec
		want string
	}{
		{"empty module name", regel.New("").Workers("a").Step("one"), "module name is empty"},
		{"invalid module name", regel.New("With draw").Workers("a").Step("one"), `module "With draw" is not a valid TLA+ identifier`},
		{"reserved module name", regel.New("TLC").Workers("a").Step("one"), `module "TLC" clashes with a reserved name`},
		{"no workers", regel.New("M").Step("one"), "spec has no workers"},
		{"no steps", regel.New("M").Workers("a"), "spec has no steps"},
		{"nothing declared", regel.New("M"), "spec has no workers"},
		{"duplicate workers", regel.New("M").Workers("a", "a").Step("one"), `worker "a" is declared more than once`},
		{"duplicate workers across calls", regel.New("M").Workers("a").Workers("a").Step("one"), `worker "a" is declared more than once`},
		{"duplicate step labels", regel.New("M").Workers("a").Step("one").Step("one"), `step "one" is declared more than once`},
		{"duplicate variables", regel.New("M").Var("x", 1).Var("x", 2).Workers("a").Step("one"), `variable "x" is declared more than once`},
		{"variable declared as shared and local", regel.New("M").Var("x", 1).Local("x", 2).Workers("a").Step("one"), `variable "x" is declared more than once`},
		{"goto target missing", regel.New("M").Workers("a").Step("one", regel.Goto("nowhere")), `step "one": Goto target "nowhere" does not exist`},
		{"goto target empty", regel.New("M").Workers("a").Step("one", regel.Goto("")), `step "one": Goto has no target`},
		{"if goto target missing", regel.New("M").Workers("a").Step("one", regel.If("TRUE").Goto("nowhere").Else("done")), `step "one": If Goto target "nowhere" does not exist`},
		{"if else target missing", regel.New("M").Workers("a").Step("one", regel.If("TRUE").Goto("done").Else("nowhere")), `step "one": If Else target "nowhere" does not exist`},
		{"if without goto", regel.New("M").Workers("a").Step("one", regel.If("TRUE").Else("done")), `step "one": If Goto has no target`},
		{"if without else", regel.New("M").Workers("a").Step("one", regel.If("TRUE").Goto("done")), `step "one": If Else has no target`},
		{"if with empty condition", regel.New("M").Workers("a").Step("one", regel.If("").Goto("done").Else("done")), `step "one": expression is empty`},
		{"set undeclared variable", regel.New("M").Workers("a").Step("one", regel.Set("x", "1")), `step "one": Set on undeclared variable "x"`},
		{"set twice", regel.New("M").Var("x", 0).Workers("a").Step("one", regel.Set("x", "1"), regel.Set("x", "2")), `step "one": variable "x" is set more than once`},
		{"set with empty expression", regel.New("M").Var("x", 0).Workers("a").Step("one", regel.Set("x", " ")), `step "one": expression is empty`},
		{"await with empty condition", regel.New("M").Workers("a").Step("one", regel.Await("")), `step "one": expression is empty`},
		{"two gotos", regel.New("M").Workers("a").Step("one", regel.Goto("done"), regel.Goto("one")), `step "one" has 2 control operations`},
		{"goto and if", regel.New("M").Workers("a").Step("one", regel.Goto("done"), regel.If("TRUE").Goto("done").Else("one")), `step "one" has 2 control operations`},
		{"nil operation", regel.New("M").Workers("a").Step("one", nil), `step "one": operation is nil`},
		{"invalid variable name", regel.New("M").Var("bad-name", 1).Workers("a").Step("one"), `variable "bad-name" is not a valid TLA+ identifier`},
		{"invalid worker name", regel.New("M").Workers("1a").Step("one"), `worker "1a" is not a valid TLA+ identifier`},
		{"invalid step label", regel.New("M").Workers("a").Step("one two"), `step "one two" is not a valid TLA+ identifier`},
		{"invalid rule name", regel.New("M").Workers("a").Step("one").Always("no good", "TRUE"), `rule "no good" is not a valid TLA+ identifier`},
		{"fairness prefix is not an identifier", regel.New("M").Var("WF_x", 1).Workers("a").Step("one"), `variable "WF_x" is not a valid TLA+ identifier`},
		{"empty variable name", regel.New("M").Var("", 1).Workers("a").Step("one"), "variable name is empty"},
		{"empty step label", regel.New("M").Workers("a").Step(""), "step name is empty"},
		{"empty worker name", regel.New("M").Workers("").Step("one"), "worker name is empty"},
		{"empty rule name", regel.New("M").Workers("a").Step("one").Always("", "TRUE"), "rule name is empty"},
		{"unsupported float", regel.New("M").Var("x", 1.5).Workers("a").Step("one"), `variable "x" has an unsupported initial value of type float64`},
		{"unsupported nil", regel.New("M").Var("x", nil).Workers("a").Step("one"), `variable "x" has an unsupported initial value of type <nil>`},
		{"unsupported slice", regel.New("M").Local("x", []int{1}).Workers("a").Step("one"), `variable "x" has an unsupported initial value of type []int`},
		{"empty raw value", regel.New("M").Var("x", regel.Raw("")).Workers("a").Step("one"), `variable "x": expression is empty`},
		{"rule with empty expression", regel.New("M").Workers("a").Step("one").Always("Safe", ""), `rule "Safe": expression is empty`},
		{"duplicate rules", regel.New("M").Workers("a").Step("one").Always("Safe", "TRUE").Always("Safe", "TRUE"), `rule "Safe" is declared more than once`},
		{"rule named like a variable", regel.New("M").Var("x", 0).Workers("a").Step("one").Always("x", "TRUE"), `rule "x" has the same name as a variable`},
		{"worker named like a variable", regel.New("M").Var("a", 0).Workers("a").Step("one"), `worker "a" has the same name as a variable`},
		{"name with the step action prefix", regel.New("M").Var("Step_one", 0).Workers("a").Step("one"), `variable "Step_one" clashes with the reserved prefix Step_`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.spec.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error containing %q", test.want)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() = %q, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestValidateRejectsReservedNames(t *testing.T) {
	reserved := []string{
		"pc", "self", "done", "vars", "Init", "Next", "Spec", "Worker", "Workers", "Finished", "TypeOK",
		"IF", "THEN", "ELSE", "CHOOSE", "EXCEPT", "UNCHANGED", "VARIABLE", "TRUE", "FALSE", "MODULE",
		"INVARIANT", "SPECIFICATION", "VIEW",
		"Int", "Nat", "Seq", "Len", "Cardinality", "Print",
	}
	for _, name := range reserved {
		t.Run(name, func(t *testing.T) {
			specs := map[string]*regel.Spec{
				"variable": regel.New("M").Var(name, 0).Workers("a").Step("one"),
				"local":    regel.New("M").Local(name, 0).Workers("a").Step("one"),
				"worker":   regel.New("M").Workers(name).Step("one"),
				"step":     regel.New("M").Workers("a").Step(name),
				"rule":     regel.New("M").Workers("a").Step("one").Always(name, "TRUE"),
				"module":   regel.New(name).Workers("a").Step("one"),
			}
			for kind, spec := range specs {
				err := spec.Validate()
				if err == nil || !strings.Contains(err.Error(), "clashes with a reserved name") {
					t.Errorf("%s named %q: Validate() = %v, want a reserved name error", kind, name, err)
				}
			}
		})
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	spec := regel.New("bad name").
		Var("x", 1.5).
		Var("x", 2).
		Workers("a", "a").
		Step("one", regel.Set("missing", "1"), regel.Goto("nowhere")).
		Step("one").
		Always("Safe", "")

	err := spec.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want errors")
	}
	want := []string{
		`module "bad name"`,
		`variable "x" has an unsupported initial value`,
		`variable "x" is declared more than once`,
		`worker "a" is declared more than once`,
		`step "one" is declared more than once`,
		`Set on undeclared variable "missing"`,
		`Goto target "nowhere" does not exist`,
		`rule "Safe": expression is empty`,
	}
	for _, text := range want {
		if !strings.Contains(err.Error(), text) {
			t.Errorf("Validate() = %q, want it to contain %q", err, text)
		}
	}
	if got := len(strings.Split(err.Error(), "\n")); got != len(want) {
		t.Errorf("Validate() reported %d problems, want %d:\n%v", got, len(want), err)
	}
}

func TestNilSpecDoesNotPanic(t *testing.T) {
	var spec *regel.Spec
	if err := spec.Var("x", 1).Local("y", 2).Workers("a").Step("one").Always("Safe", "TRUE").Validate(); err == nil {
		t.Fatal("Validate() = nil, want an error for a nil spec")
	}
}

func TestStepOperationsCanSharePartialBranch(t *testing.T) {
	partial := regel.If("TRUE")
	first := partial.Goto("done").Else("done")
	spec := regel.New("M").Workers("a").Step("one", first)
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	second := regel.New("M").Workers("a").Step("one", partial)
	if err := second.Validate(); err == nil {
		t.Fatal("Validate() = nil, want the unfinished branch to be rejected")
	}
}
