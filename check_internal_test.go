package regel

import "testing"

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
	output := "Progress(5) at 2026-10-31: 13 states generated (413 s/min), 10 distinct states found (318 ds/min), 1 states left on queue.\n" +
		"1,234,567 states generated, 7,654 distinct states found, 1 states left on queue.\n"
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
