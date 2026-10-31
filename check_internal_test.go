package regel

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestWithdrawFixedWithoutFairnessFailsAllFinish(t *testing.T) {
	if os.Getenv("REGEL_SKIP_TLC") == "1" {
		t.Skip("REGEL_SKIP_TLC=1 is set")
	}
	tools, err := findTools(nil)
	if err != nil {
		t.Fatal(err)
	}
	spec := New("WithdrawFixed").
		Var("balance", 100).
		Workers("a", "b").
		Step("withdraw", Set("balance", "IF balance >= 60 THEN balance - 60 ELSE balance")).
		Always("NoOverdraft", "balance >= 0").
		Eventually("AllFinish", `\A self \in Workers : pc[self] = "done"`)
	source, err := spec.Source()
	if err != nil {
		t.Fatal(err)
	}
	config, err := spec.Config()
	if err != nil {
		t.Fatal(err)
	}
	fairness := ` /\ \A self \in Workers : WF_vars(Worker(self))`
	if !strings.Contains(source, fairness) {
		t.Fatalf("source has no fairness to remove:\n%s", source)
	}

	result, err := tools.check(context.Background(), spec.name, strings.Replace(source, fairness, "", 1), config)

	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != PropertyViolated || result.ExitCode != 13 {
		t.Fatalf("Verdict = %q, ExitCode = %d\n%s", result.Verdict, result.ExitCode, result.RawOutput)
	}
	if !result.Stutters {
		t.Errorf("trace does not end in stuttering:\n%s", result)
	}
}

func TestVerdictFor(t *testing.T) {
	const finished = "Model checking completed. No error has been found."
	tests := []struct {
		name     string
		exitCode int
		output   string
		want     Verdict
	}{
		{"pass needs both signals", 0, finished, Passed},
		{"exit 0 without the finished line is not a pass", 0, "Starting...", ToolError},
		{"empty output is not a pass", 0, "", ToolError},
		{"the finished line without exit 0 is not a pass", 150, finished, ToolError},
		{"deadlock", 11, "", Deadlock},
		{"invariant", 12, "", InvariantViolated},
		{"property", 13, "", PropertyViolated},
		{"parse error", 150, "", ToolError},
		{"config error", 151, "", ToolError},
		{"evaluation error", 75, "", ToolError},
		{"killed", -1, "", ToolError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := verdictFor(test.exitCode, test.output); got != test.want {
				t.Fatalf("verdictFor(%d) = %q, want %q", test.exitCode, got, test.want)
			}
		})
	}
}

func TestNewResultReadsTheFinalStateCounts(t *testing.T) {
	output := "@!@!@STARTMSG 2200:0 @!@!@\n" +
		"Progress(5) at 2026-10-31: 13 states generated (413 s/min), 10 distinct states found (318 ds/min), 1 states left on queue.\n" +
		"@!@!@ENDMSG 2200 @!@!@\n" +
		"@!@!@STARTMSG 2199:0 @!@!@\n" +
		"1,234,567 states generated, 7,654 distinct states found, 1 states left on queue.\n" +
		"@!@!@ENDMSG 2199 @!@!@\n"
	result := newResult(12, output)
	if result.StatesGenerated != 1234567 || result.DistinctStates != 7654 {
		t.Fatalf("counts = %d, %d", result.StatesGenerated, result.DistinctStates)
	}
}

func TestZeroResultIsNotAPass(t *testing.T) {
	var missing *Result
	if missing.Passed() {
		t.Error("a nil result passed")
	}
	if (&Result{}).Passed() {
		t.Error("an empty result passed")
	}
}
