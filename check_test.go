package regel_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	regel "github.com/sodiqscript111/Regel"
)

func withdrawAmount(amount int, workers ...string) *regel.Spec {
	return regel.New("Withdraw").
		Var("balance", 100).
		Workers(workers...).
		Step("check", regel.If(fmt.Sprintf("balance >= %d", amount)).Goto("debit").Else("done")).
		Step("debit", regel.Set("balance", fmt.Sprintf("balance - %d", amount))).
		Always("NoOverdraft", "balance >= 0")
}

func withdrawFixedAmount(amount int, workers ...string) *regel.Spec {
	expr := fmt.Sprintf("IF balance >= %[1]d THEN balance - %[1]d ELSE balance", amount)
	return regel.New("WithdrawFixed").
		Var("balance", 100).
		Workers(workers...).
		Step("withdraw", regel.Set("balance", expr)).
		Always("NoOverdraft", "balance >= 0")
}

func check(t *testing.T, spec *regel.Spec) *regel.Result {
	t.Helper()
	java, jar := tlcTools(t)
	result, err := spec.Check(context.Background(), regel.WithJava(java), regel.WithJar(jar))
	if err != nil {
		t.Fatalf("Check() = %v", err)
	}
	return result
}

func TestCheckReferenceResults(t *testing.T) {
	tests := []struct {
		name      string
		spec      *regel.Spec
		verdict   regel.Verdict
		exitCode  int
		generated int64
		distinct  int64
	}{
		{name: "Withdraw", spec: withdraw(), verdict: regel.InvariantViolated, exitCode: 12},
		{name: "WithdrawFixed", spec: withdrawFixed(), verdict: regel.Passed, generated: 6, distinct: 4},
		{name: "Withdraw amount 50", spec: withdrawAmount(50, "a", "b"), verdict: regel.Passed, generated: 14, distinct: 9},
		{name: "Withdraw amount 50, 3 workers", spec: withdrawAmount(50, "a", "b", "c"), verdict: regel.InvariantViolated, exitCode: 12},
		{name: "WithdrawFixed amount 50, 3 workers", spec: withdrawFixedAmount(50, "a", "b", "c"), verdict: regel.Passed, generated: 14, distinct: 8},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := check(t, test.spec)
			if result.Verdict != test.verdict {
				t.Fatalf("Verdict = %q, want %q\n%s", result.Verdict, test.verdict, result.RawOutput)
			}
			if result.ExitCode != test.exitCode {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, test.exitCode)
			}
			if test.verdict != regel.Passed {
				return
			}
			if !result.Passed() {
				t.Errorf("Passed() = false")
			}
			if result.StatesGenerated != test.generated || result.DistinctStates != test.distinct {
				t.Errorf("states = %d generated, %d distinct, want %d, %d",
					result.StatesGenerated, result.DistinctStates, test.generated, test.distinct)
			}
		})
	}
}

func TestCheckTraces(t *testing.T) {
	tests := []struct {
		name    string
		spec    *regel.Spec
		states  int
		balance string
	}{
		{"Withdraw", withdraw(), 5, "-20"},
		{"Withdraw amount 50, 3 workers", withdrawAmount(50, "a", "b", "c"), 7, "-50"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := check(t, test.spec)
			if result.Violated != "NoOverdraft" {
				t.Errorf("Violated = %q, want NoOverdraft", result.Violated)
			}
			if len(result.Trace) != test.states {
				t.Fatalf("trace has %d states, want %d\n%s", len(result.Trace), test.states, result)
			}
			if got := result.Trace[test.states-1].Vars["balance"]; got != test.balance {
				t.Errorf("final balance = %s, want %s", got, test.balance)
			}
		})
	}
}

func TestCheckPrintsTheExpectedShape(t *testing.T) {
	result := check(t, withdraw())
	lines := strings.Split(result.String(), "\n")
	if lines[0] != "FAILED: NoOverdraft was broken after 5 steps" {
		t.Errorf("headline = %q", lines[0])
	}
	if len(lines) != 7 || lines[1] != "" {
		t.Fatalf("output has the wrong shape:\n%s", result)
	}
	want := [][]string{
		{"1", "start", "balance=100"},
		{"2", "check", "balance=100"},
		{"3", "check", "balance=100"},
		{"4", "debit", "balance=40"},
		{"5", "debit", "balance=-20"},
	}
	for i, fields := range want {
		for _, field := range fields {
			if !strings.Contains(lines[i+2], field) {
				t.Errorf("line %q does not contain %q", lines[i+2], field)
			}
		}
	}
	if got := fmt.Sprint(result.Trace); !strings.Contains(result.String(), got) {
		t.Errorf("Trace prints differently from the result:\n%s", got)
	}
}

func TestCheckBlockedAwaitIsDeadlock(t *testing.T) {
	spec := regel.New("Blocked").
		Var("open", false).
		Workers("a", "b").
		Step("wait", regel.Await("open"))
	result := check(t, spec)
	if result.Verdict != regel.Deadlock {
		t.Fatalf("Verdict = %q, want %q\n%s", result.Verdict, regel.Deadlock, result.RawOutput)
	}
	if result.ExitCode != 11 {
		t.Errorf("ExitCode = %d, want 11", result.ExitCode)
	}
	if result.Passed() {
		t.Error("Passed() = true")
	}
}

func TestCheckInvariantBrokenAtTheStart(t *testing.T) {
	spec := regel.New("Broken").Var("x", 0).Workers("a").Step("one").Always("Positive", "x > 0")
	result := check(t, spec)
	if result.Verdict != regel.InvariantViolated {
		t.Fatalf("Verdict = %q, want %q\n%s", result.Verdict, regel.InvariantViolated, result.RawOutput)
	}
}

func TestCheckToolErrorsNeverLookLikeAPass(t *testing.T) {
	base := func() *regel.Spec {
		return regel.New("Faulty").Var("x", 0).Workers("a")
	}
	tests := []struct {
		name string
		spec *regel.Spec
	}{
		{"syntax error", base().Step("one").Always("Bad", "x >=")},
		{"unknown name", base().Step("one").Always("Bad", "missing > 0")},
		{"evaluation error", base().Step("one").Always("Bad", `x >= "text"`)},
		{"local used outside a worker", regel.New("Faulty").Local("x", 0).Workers("a").Step("one").Always("Bad", "x[self] = 0")},
		{"failed assertion", base().Step("one", regel.Await(`Assert(FALSE, "stop")`))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := check(t, test.spec)
			if result.Verdict != regel.ToolError {
				t.Errorf("Verdict = %q, want %q\n%s", result.Verdict, regel.ToolError, result.RawOutput)
			}
			if result.Passed() {
				t.Error("Passed() = true")
			}
			if result.RawOutput == "" {
				t.Error("RawOutput is empty")
			}
		})
	}
}

func endless() *regel.Spec {
	return regel.New("Endless").
		Var("count", 0).
		Workers("a").
		Step("count", regel.Set("count", "count + 1"), regel.Goto("count"))
}

func TestCheckCancellingStopsTLC(t *testing.T) {
	java, jar := tlcTools(t)
	temp := useTempFolder(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	started := time.Now()
	result, err := endless().Check(ctx, regel.WithJava(java), regel.WithJar(jar))

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Check() = %v, %v, want a deadline error", result, err)
	}
	if elapsed := time.Since(started); elapsed > 20*time.Second {
		t.Errorf("Check() took %v after being cancelled", elapsed)
	}
	expectEmpty(t, temp)
}

func TestCheckCancelledContextDoesNotStartTLC(t *testing.T) {
	java, jar := tlcTools(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := withdraw().Check(ctx, regel.WithJava(java), regel.WithJar(jar)); !errors.Is(err, context.Canceled) {
		t.Fatalf("Check() = %v, want context.Canceled", err)
	}
}

func TestCheckLeavesNoTempFolderBehind(t *testing.T) {
	tests := []struct {
		name string
		spec *regel.Spec
	}{
		{"pass", withdrawFixed()},
		{"failure", withdraw()},
		{"tool error", regel.New("Faulty").Workers("a").Step("one").Always("Bad", "missing")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			temp := useTempFolder(t)
			check(t, test.spec)
			expectEmpty(t, temp)
		})
	}
}

func useTempFolder(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
	return dir
}

func expectEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		t.Errorf("left behind: %s", entry.Name())
	}
}

func TestCheckReportsMissingTools(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "tla2tools.jar")
	if err := os.WriteFile(jar, []byte("not really a jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing")

	tests := []struct {
		name    string
		env     map[string]string
		options []regel.CheckOption
		want    []string
	}{
		{
			name: "no java anywhere",
			env:  map[string]string{"REGEL_JAVA": "", "REGEL_TLA2TOOLS": jar, "PATH": ""},
			want: []string{"java not found", "REGEL_JAVA", "WithJava", "PATH"},
		},
		{
			name: "REGEL_JAVA points nowhere",
			env:  map[string]string{"REGEL_JAVA": missing, "REGEL_TLA2TOOLS": jar},
			want: []string{"java not found at", "missing"},
		},
		{
			name:    "option points nowhere",
			env:     map[string]string{"REGEL_TLA2TOOLS": jar},
			options: []regel.CheckOption{regel.WithJava(missing)},
			want:    []string{"java not found at"},
		},
		{
			name:    "no jar anywhere",
			env:     map[string]string{"REGEL_TLA2TOOLS": ""},
			options: []regel.CheckOption{regel.WithJava(os.Args[0])},
			want:    []string{"tla2tools.jar not found", "REGEL_TLA2TOOLS", "WithJar"},
		},
		{
			name:    "jar points nowhere",
			env:     map[string]string{"REGEL_TLA2TOOLS": missing},
			options: []regel.CheckOption{regel.WithJava(os.Args[0])},
			want:    []string{"tla2tools.jar not found at"},
		},
		{
			name:    "jar is a folder",
			options: []regel.CheckOption{regel.WithJava(os.Args[0]), regel.WithJar(t.TempDir())},
			want:    []string{"that path is a folder"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for key, value := range test.env {
				t.Setenv(key, value)
			}
			result, err := withdraw().Check(context.Background(), test.options...)
			if err == nil {
				t.Fatalf("Check() = %v, want an error", result)
			}
			if result != nil {
				t.Errorf("Check() returned a result alongside the error: %+v", result)
			}
			for _, text := range test.want {
				if !strings.Contains(err.Error(), text) {
					t.Errorf("error %q does not contain %q", err, text)
				}
			}
		})
	}
}

func TestCheckRejectsBadInput(t *testing.T) {
	if _, err := regel.New("").Check(context.Background()); err == nil {
		t.Error("Check() on an invalid spec = nil error")
	}
	var missing *regel.Spec
	if _, err := missing.Check(context.Background()); err == nil {
		t.Error("Check() on a nil spec = nil error")
	}
	var ctx context.Context
	if _, err := withdraw().Check(ctx); err == nil {
		t.Error("Check() with a nil context = nil error")
	}
	if _, err := withdraw().Check(context.Background(), nil, regel.WithJava("definitely-missing-tool")); err == nil {
		t.Error("Check() with a missing tool = nil error")
	}
}
