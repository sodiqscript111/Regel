package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	regel "github.com/sodiqscript111/Regel"
	"github.com/sodiqscript111/Regel/patterns"
)

func execute(args ...string) (stdout, stderr string, code int) {
	var out, errors bytes.Buffer
	code = run(context.Background(), args, &out, &errors)
	return out.String(), errors.String(), code
}

func needTLC(t *testing.T) {
	t.Helper()
	if os.Getenv("REGEL_SKIP_TLC") == "1" {
		t.Skip("REGEL_SKIP_TLC=1 is set")
	}
	if os.Getenv("REGEL_JAVA") == "" || os.Getenv("REGEL_TLA2TOOLS") == "" {
		t.Fatal("this test runs TLC: set REGEL_JAVA and REGEL_TLA2TOOLS, or set REGEL_SKIP_TLC=1 to skip it")
	}
}

func removeTools(t *testing.T) {
	t.Helper()
	missing := filepath.Join(t.TempDir(), "missing")
	t.Setenv("REGEL_JAVA", missing)
	t.Setenv("REGEL_TLA2TOOLS", missing)
}

func expectContains(t *testing.T, text string, wanted ...string) {
	t.Helper()
	for _, want := range wanted {
		if !strings.Contains(text, want) {
			t.Errorf("output does not contain %q:\n%s", want, text)
		}
	}
}

func TestList(t *testing.T) {
	stdout, stderr, code := execute("list")
	if code != exitPassed || stderr != "" {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	for _, pattern := range patterns.All() {
		expectContains(t, stdout, pattern.Name, pattern.Summary)
	}
}

func TestShow(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"broken by default", []string{"show", "CheckThenAct"}, "---- MODULE CheckThenAct ----"},
		{"fixed after the name", []string{"show", "CheckThenAct", "--fixed"}, "---- MODULE CheckThenActFixed ----"},
		{"fixed before the name", []string{"show", "--fixed", "CheckThenAct"}, "---- MODULE CheckThenActFixed ----"},
		{"single dash flag", []string{"show", "CheckThenAct", "-fixed"}, "---- MODULE CheckThenActFixed ----"},
		{"any case", []string{"show", "lostupdate"}, "---- MODULE LostUpdate ----"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, code := execute(test.args...)
			if code != exitPassed || stderr != "" {
				t.Fatalf("code = %d, stderr = %q", code, stderr)
			}
			if !strings.HasPrefix(stdout, test.want) {
				t.Errorf("stdout = %q", stdout)
			}
		})
	}
}

func TestShowPrintsEveryPattern(t *testing.T) {
	for _, pattern := range patterns.All() {
		for _, args := range [][]string{{"show", pattern.Name}, {"show", pattern.Name, "--fixed"}} {
			stdout, _, code := execute(args...)
			if code != exitPassed {
				t.Errorf("%v: code = %d", args, code)
			}
			expectContains(t, stdout, "MODULE", "Next ==", "Spec ==")
		}
	}
}

func TestShowDoesNotNeedTLC(t *testing.T) {
	removeTools(t)
	if _, _, code := execute("show", "CheckThenAct"); code != exitPassed {
		t.Fatalf("code = %d", code)
	}
}

func TestUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no command", nil, "no command given"},
		{"unknown command", []string{"explode"}, `unknown command "explode"`},
		{"show without a pattern", []string{"show"}, "show takes one pattern name"},
		{"check without a pattern", []string{"check"}, "check takes one pattern name"},
		{"two patterns", []string{"show", "LostUpdate", "CheckThenAct"}, "show takes one pattern name"},
		{"unknown pattern", []string{"show", "Nope"}, `unknown pattern "Nope"`},
		{"unknown pattern for check", []string{"check", "Nope"}, `unknown pattern "Nope"`},
		{"unknown flag", []string{"show", "LostUpdate", "--nope"}, "flag provided but not defined"},
		{"list with arguments", []string{"list", "x"}, "list takes no arguments"},
		{"demo with arguments", []string{"demo", "x"}, "demo takes no arguments"},
		{"doctor with arguments", []string{"doctor", "x"}, "doctor takes no arguments"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, code := execute(test.args...)
			if code != exitError {
				t.Errorf("code = %d, want %d", code, exitError)
			}
			if stdout != "" {
				t.Errorf("stdout = %q", stdout)
			}
			expectContains(t, stderr, test.want, "Usage:")
		})
	}
}

func TestHelp(t *testing.T) {
	stdout, _, code := execute("help")
	if code != exitPassed {
		t.Fatalf("code = %d", code)
	}
	expectContains(t, stdout, "Usage:", "regel doctor")
}

func TestCheckExitCodes(t *testing.T) {
	needTLC(t)
	tests := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"broken fails with 1", []string{"check", "CheckThenAct"}, exitFailed, "FAILED: WithinLimit was broken after 5 steps"},
		{"deadlock fails with 1", []string{"check", "LockOrderDeadlock"}, exitFailed, "FAILED: deadlock"},
		{"fixed passes with 0", []string{"check", "CheckThenAct", "--fixed"}, exitPassed, "PASSED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, code := execute(test.args...)
			if code != test.code || stderr != "" {
				t.Fatalf("code = %d, want %d, stderr = %q\n%s", code, test.code, stderr, stdout)
			}
			expectContains(t, stdout, test.want)
		})
	}
}

func TestCheckEveryPattern(t *testing.T) {
	needTLC(t)
	for _, pattern := range patterns.All() {
		t.Run(pattern.Name, func(t *testing.T) {
			if _, _, code := execute("check", pattern.Name); code != exitFailed {
				t.Errorf("broken: code = %d, want %d", code, exitFailed)
			}
			if _, _, code := execute("check", pattern.Name, "--fixed"); code != exitPassed {
				t.Errorf("fixed: code = %d, want %d", code, exitPassed)
			}
		})
	}
}

func TestCheckWithoutToolsIsAToolError(t *testing.T) {
	removeTools(t)
	stdout, stderr, code := execute("check", "CheckThenAct")
	if code != exitError {
		t.Fatalf("code = %d, want %d", code, exitError)
	}
	if stdout != "" {
		t.Errorf("stdout = %q", stdout)
	}
	expectContains(t, stderr, "java not found")
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		result *regel.Result
		want   int
	}{
		{&regel.Result{Verdict: regel.Passed}, exitPassed},
		{&regel.Result{Verdict: regel.InvariantViolated}, exitFailed},
		{&regel.Result{Verdict: regel.PropertyViolated}, exitFailed},
		{&regel.Result{Verdict: regel.Deadlock}, exitFailed},
		{&regel.Result{Verdict: regel.ToolError}, exitError},
		{nil, exitError},
	}
	for _, test := range tests {
		if got := exitCode(test.result); got != test.want {
			t.Errorf("exitCode(%+v) = %d, want %d", test.result, got, test.want)
		}
	}
}

func TestDemo(t *testing.T) {
	needTLC(t)
	stdout, stderr, code := execute("demo")
	if code != exitPassed || stderr != "" {
		t.Fatalf("code = %d, stderr = %q\n%s", code, stderr, stdout)
	}
	expectContains(t, stdout, "The broken version:", "FAILED: WithinLimit was broken", "The fixed version:", "PASSED")
	if strings.Index(stdout, "FAILED") > strings.Index(stdout, "PASSED") {
		t.Errorf("the failure should come first:\n%s", stdout)
	}
}

func TestDemoWithoutToolsIsAToolError(t *testing.T) {
	removeTools(t)
	_, stderr, code := execute("demo")
	if code != exitError {
		t.Fatalf("code = %d, want %d", code, exitError)
	}
	expectContains(t, stderr, "java not found")
}

func TestDoctorFindsTools(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "tla2tools.jar")
	if err := os.WriteFile(jar, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REGEL_JAVA", os.Args[0])
	t.Setenv("REGEL_TLA2TOOLS", jar)
	stdout, stderr, code := execute("doctor")
	if code != exitPassed || stderr != "" {
		t.Fatalf("code = %d, stderr = %q\n%s", code, stderr, stdout)
	}
	expectContains(t, stdout, "Java: found at", "tla2tools.jar: found at "+jar)
}

func TestDoctorReportsEachMissingTool(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "tla2tools.jar")
	if err := os.WriteFile(jar, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{
			name: "both missing",
			env:  map[string]string{"REGEL_JAVA": missing, "REGEL_TLA2TOOLS": missing},
			want: []string{"Java: NOT FOUND", "tla2tools.jar: NOT FOUND", "REGEL_JAVA", "REGEL_TLA2TOOLS"},
		},
		{
			name: "nothing set and no java on PATH",
			env:  map[string]string{"REGEL_JAVA": "", "REGEL_TLA2TOOLS": "", "PATH": ""},
			want: []string{"Java: NOT FOUND", "tla2tools.jar: NOT FOUND"},
		},
		{
			name: "only java missing",
			env:  map[string]string{"REGEL_JAVA": missing, "REGEL_TLA2TOOLS": jar},
			want: []string{"Java: NOT FOUND", "tla2tools.jar: found at"},
		},
		{
			name: "only the jar missing",
			env:  map[string]string{"REGEL_JAVA": os.Args[0], "REGEL_TLA2TOOLS": ""},
			want: []string{"Java: found at", "tla2tools.jar: NOT FOUND"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for key, value := range test.env {
				t.Setenv(key, value)
			}
			stdout, _, code := execute("doctor")
			if code != exitError {
				t.Fatalf("code = %d, want %d\n%s", code, exitError, stdout)
			}
			expectContains(t, stdout, test.want...)
		})
	}
}

func TestCancelledCheckIsAToolError(t *testing.T) {
	needTLC(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errors bytes.Buffer
	if code := run(ctx, []string{"check", "CheckThenAct"}, &out, &errors); code != exitError {
		t.Fatalf("code = %d, want %d", code, exitError)
	}
	expectContains(t, errors.String(), "context canceled")
}
