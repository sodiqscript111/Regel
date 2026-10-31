package regeltest

import (
	"os"
	"testing"

	regel "github.com/sodiqscript111/Regel"
)

func Holds(t testing.TB, spec *regel.Spec, options ...regel.CheckOption) {
	t.Helper()
	result := run(t, spec, options)
	if !result.Passed() {
		t.Fatalf("expected the spec to hold\n\n%s", result)
	}
}

func Fails(t testing.TB, spec *regel.Spec, rule string, options ...regel.CheckOption) {
	t.Helper()
	result := run(t, spec, options)
	switch {
	case result.Passed():
		t.Fatalf("expected %s to be broken, but the spec passed\n\n%s", rule, result)
	case result.Verdict == regel.ToolError:
		t.Fatalf("expected %s to be broken, but TLC could not check the spec\n\n%s", rule, result)
	case result.Violated != rule:
		t.Fatalf("expected %s to be broken, but %s was\n\n%s", rule, result.Violated, result)
	}
}

func run(t testing.TB, spec *regel.Spec, options []regel.CheckOption) *regel.Result {
	t.Helper()
	if err := spec.Validate(); err != nil {
		t.Fatalf("the spec is not valid:\n%v", err)
	}
	if os.Getenv("REGEL_SKIP_TLC") == "1" {
		t.Skip("REGEL_SKIP_TLC=1 is set")
	}
	result, err := spec.Check(t.Context(), options...)
	if err != nil {
		t.Fatalf("could not run TLC: %v", err)
	}
	return result
}
