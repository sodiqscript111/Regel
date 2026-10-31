package regeltest_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	regel "github.com/sodiqscript111/Regel"
	"github.com/sodiqscript111/Regel/regeltest"
)

type recorder struct {
	testing.TB
	failed  bool
	skipped bool
	output  string
}

func (r *recorder) Helper() {}

func (r *recorder) Context() context.Context {
	return context.Background()
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.failed = true
	r.output = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

func (r *recorder) Skip(args ...any) {
	r.skipped = true
	r.output = fmt.Sprint(args...)
	runtime.Goexit()
}

func record(test func(t testing.TB)) *recorder {
	r := &recorder{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		test(r)
	}()
	<-done
	return r
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

func broken() *regel.Spec {
	return regel.New("Withdraw").
		Var("balance", 100).
		Workers("a", "b").
		Step("check", regel.If("balance >= 60").Goto("debit").Else("done")).
		Step("debit", regel.Set("balance", "balance - 60")).
		Always("NoOverdraft", "balance >= 0")
}

func fixed() *regel.Spec {
	return regel.New("WithdrawFixed").
		Var("balance", 100).
		Workers("a", "b").
		Step("withdraw", regel.Set("balance", "IF balance >= 60 THEN balance - 60 ELSE balance")).
		Always("NoOverdraft", "balance >= 0")
}

func blocked() *regel.Spec {
	return regel.New("Blocked").Var("open", false).Workers("a").Step("wait", regel.Await("open"))
}

func faulty() *regel.Spec {
	return regel.New("Faulty").Var("x", 0).Workers("a").Step("one").Always("Bad", "x >=")
}

func TestHoldsAcceptsAPassingSpec(t *testing.T) {
	needTLC(t)
	got := record(func(t testing.TB) { regeltest.Holds(t, fixed()) })
	if got.failed {
		t.Fatalf("Holds failed: %s", got.output)
	}
}

func TestFailsAcceptsTheExpectedBrokenRule(t *testing.T) {
	needTLC(t)
	got := record(func(t testing.TB) { regeltest.Fails(t, broken(), "NoOverdraft") })
	if got.failed {
		t.Fatalf("Fails failed: %s", got.output)
	}
}

func TestFailsAcceptsADeadlock(t *testing.T) {
	needTLC(t)
	got := record(func(t testing.TB) { regeltest.Fails(t, blocked(), regel.NoDeadlock) })
	if got.failed {
		t.Fatalf("Fails failed: %s", got.output)
	}
}

func TestHelpersReportFailures(t *testing.T) {
	needTLC(t)
	tests := []struct {
		name string
		call func(t testing.TB)
		want []string
	}{
		{
			name: "Holds on a broken spec prints the trace",
			call: func(t testing.TB) { regeltest.Holds(t, broken()) },
			want: []string{"expected the spec to hold", "FAILED: NoOverdraft was broken after 5 steps", "balance=-20"},
		},
		{
			name: "Holds on a deadlock",
			call: func(t testing.TB) { regeltest.Holds(t, blocked()) },
			want: []string{"expected the spec to hold", "FAILED: deadlock"},
		},
		{
			name: "Holds on a tool error",
			call: func(t testing.TB) { regeltest.Holds(t, faulty()) },
			want: []string{"expected the spec to hold", "ERROR: TLC could not check this spec"},
		},
		{
			name: "Fails on a passing spec",
			call: func(t testing.TB) { regeltest.Fails(t, fixed(), "NoOverdraft") },
			want: []string{"expected NoOverdraft to be broken, but the spec passed", "PASSED"},
		},
		{
			name: "Fails when a different rule broke prints the trace",
			call: func(t testing.TB) { regeltest.Fails(t, broken(), "SomethingElse") },
			want: []string{"expected SomethingElse to be broken, but NoOverdraft was", "balance=-20"},
		},
		{
			name: "Fails on a tool error",
			call: func(t testing.TB) { regeltest.Fails(t, faulty(), "Bad") },
			want: []string{"expected Bad to be broken, but TLC could not check the spec", "ERROR"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := record(test.call)
			if !got.failed {
				t.Fatal("the helper did not report a failure")
			}
			for _, text := range test.want {
				if !strings.Contains(got.output, text) {
					t.Errorf("output does not contain %q:\n%s", text, got.output)
				}
			}
		})
	}
}

func TestMissingToolsFailAndNeverLookLikeAPass(t *testing.T) {
	t.Setenv("REGEL_SKIP_TLC", "")
	t.Setenv("REGEL_JAVA", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("REGEL_TLA2TOOLS", "")
	tests := []struct {
		name string
		call func(t testing.TB)
	}{
		{"Holds", func(t testing.TB) { regeltest.Holds(t, fixed()) }},
		{"Fails", func(t testing.TB) { regeltest.Fails(t, broken(), "NoOverdraft") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := record(test.call)
			if !got.failed || got.skipped {
				t.Fatalf("failed = %v, skipped = %v", got.failed, got.skipped)
			}
			if !strings.Contains(got.output, "java not found") {
				t.Errorf("output = %q", got.output)
			}
		})
	}
}

func TestMissingJarIsReportedToo(t *testing.T) {
	t.Setenv("REGEL_SKIP_TLC", "")
	t.Setenv("REGEL_TLA2TOOLS", "")
	got := record(func(t testing.TB) { regeltest.Holds(t, fixed(), regel.WithJava(os.Args[0])) })
	if !got.failed || !strings.Contains(got.output, "tla2tools.jar not found") {
		t.Fatalf("failed = %v, output = %q", got.failed, got.output)
	}
}

func TestSkipOnlyWhenAskedTo(t *testing.T) {
	t.Setenv("REGEL_SKIP_TLC", "1")
	t.Setenv("REGEL_JAVA", filepath.Join(t.TempDir(), "missing"))
	got := record(func(t testing.TB) { regeltest.Holds(t, fixed()) })
	if !got.skipped || got.failed {
		t.Fatalf("skipped = %v, failed = %v", got.skipped, got.failed)
	}
}

func TestInvalidSpecFailsEvenWhenSkipping(t *testing.T) {
	t.Setenv("REGEL_SKIP_TLC", "1")
	got := record(func(t testing.TB) { regeltest.Holds(t, regel.New("")) })
	if !got.failed || got.skipped {
		t.Fatalf("failed = %v, skipped = %v", got.failed, got.skipped)
	}
	if !strings.Contains(got.output, "module name is empty") {
		t.Errorf("output = %q", got.output)
	}
}

func TestNilSpecFails(t *testing.T) {
	got := record(func(t testing.TB) { regeltest.Fails(t, nil, "Anything") })
	if !got.failed {
		t.Fatal("the helper did not report a failure")
	}
}
