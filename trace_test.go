package regel

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sample(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "tlc", name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func descriptions(trace Trace) []string {
	var steps []string
	for _, state := range trace {
		steps = append(steps, state.description())
	}
	return steps
}

func TestParseSamples(t *testing.T) {
	tests := []struct {
		sample    string
		exitCode  int
		verdict   Verdict
		violated  string
		generated int64
		distinct  int64
		steps     []string
		stutters  bool
		loopsTo   int
	}{
		{sample: "pass", exitCode: 0, verdict: Passed, generated: 6, distinct: 4},
		{
			sample: "invariant", exitCode: 12, verdict: InvariantViolated, violated: "NoOverdraft",
			generated: 13, distinct: 10,
			steps: []string{"start", "a: check", "b: check", "a: debit", "b: debit"},
		},
		{
			sample: "deadlock", exitCode: 11, verdict: Deadlock,
			generated: 5, distinct: 4,
			steps: []string{"start", "a: first", "b: first"},
		},
		{
			sample: "invariant_at_start", exitCode: 12, verdict: InvariantViolated, violated: "Positive",
			generated: 0, distinct: 0,
			steps: []string{"start"},
		},
		{
			sample: "pc_only", exitCode: 12, verdict: InvariantViolated, violated: "Never",
			generated: 2, distinct: 2,
			steps: []string{"start", "a: one"},
		},
		{
			sample: "self_loop", exitCode: 12, verdict: InvariantViolated, violated: "Small",
			generated: 8, distinct: 7,
			steps: []string{"start", "a: start", "a: spin", "a: spin"},
		},
		{
			sample: "liveness_stutter", exitCode: 13, verdict: PropertyViolated,
			generated: 6, distinct: 4,
			steps:    []string{"start", "b: withdraw"},
			stutters: true,
		},
		{
			sample: "liveness_loop", exitCode: 13, verdict: PropertyViolated,
			generated: 4, distinct: 3,
			steps:   []string{"start", "a: spin", "a: again"},
			loopsTo: 1,
		},
		{sample: "parse_error", exitCode: 150, verdict: ToolError},
		{
			sample: "evaluation_error", exitCode: 75, verdict: ToolError,
			generated: 2, distinct: 2,
			steps: []string{"start", "a: one"},
		},
	}
	for _, test := range tests {
		t.Run(test.sample, func(t *testing.T) {
			result := newResult(test.exitCode, sample(t, test.sample))
			if result.Verdict != test.verdict {
				t.Errorf("Verdict = %q, want %q", result.Verdict, test.verdict)
			}
			if result.Violated != test.violated {
				t.Errorf("Violated = %q, want %q", result.Violated, test.violated)
			}
			if result.StatesGenerated != test.generated || result.DistinctStates != test.distinct {
				t.Errorf("states = %d, %d, want %d, %d", result.StatesGenerated, result.DistinctStates, test.generated, test.distinct)
			}
			if got := descriptions(result.Trace); !reflect.DeepEqual(got, test.steps) {
				t.Errorf("trace = %q, want %q", got, test.steps)
			}
			if result.Stutters != test.stutters {
				t.Errorf("Stutters = %v, want %v", result.Stutters, test.stutters)
			}
			if result.LoopsTo != test.loopsTo {
				t.Errorf("LoopsTo = %d, want %d", result.LoopsTo, test.loopsTo)
			}
			for i, state := range result.Trace {
				if state.Number != i+1 {
					t.Errorf("state %d has Number %d", i, state.Number)
				}
			}
		})
	}
}

func TestParseKeepsVariableValues(t *testing.T) {
	result := newResult(12, sample(t, "invariant"))
	last := result.Trace[len(result.Trace)-1]
	if last.Vars["balance"] != "-20" {
		t.Errorf("balance = %q, want -20", last.Vars["balance"])
	}
	if last.Vars["pc"] != `(a :> "done" @@ b :> "done")` {
		t.Errorf("pc = %q", last.Vars["pc"])
	}

	start := newResult(12, sample(t, "invariant_at_start")).Trace[0]
	if start.Vars["x"] != "0" || start.Vars["y"] != "1" {
		t.Errorf("start vars = %v", start.Vars)
	}

	only := newResult(12, sample(t, "pc_only")).Trace[1]
	if only.Vars["pc"] != `(a :> "done" @@ b :> "one")` {
		t.Errorf("pc = %q", only.Vars["pc"])
	}
}

func TestParseHandlesWindowsLineEndings(t *testing.T) {
	output := strings.ReplaceAll(sample(t, "invariant"), "\n", "\r\n")
	result := newResult(12, output)
	if len(result.Trace) != 5 || result.Violated != "NoOverdraft" || result.Trace[4].Vars["balance"] != "-20" {
		t.Fatalf("result = %+v", result)
	}
}

func TestParseVarsJoinsWrappedValues(t *testing.T) {
	vars := parseVars([]string{
		`/\ balance = 100`,
		`/\ pc = (a :> "one" @@`,
		`    b :> "two" @@`,
		`    c :> "three")`,
		``,
	})
	want := map[string]string{"balance": "100", "pc": `(a :> "one" @@ b :> "two" @@ c :> "three")`}
	if !reflect.DeepEqual(vars, want) {
		t.Fatalf("vars = %q, want %q", vars, want)
	}
}

func TestParseVarsReadsEqualsInsideAValue(t *testing.T) {
	vars := parseVars([]string{`/\ text = "a = b"`, `/\ other = 1`})
	if vars["text"] != `"a = b"` || vars["other"] != "1" {
		t.Fatalf("vars = %q", vars)
	}
}

func TestParseMessagesFindsMarkersInsideLines(t *testing.T) {
	output := "loose\n" +
		"@!@!@STARTMSG 1000:1 @!@!@\n" +
		"outer text\n" +
		": @!@!@STARTMSG 2154:0 @!@!@\n" +
		"inner text\n" +
		"@!@!@ENDMSG 2154 @!@!@\n" +
		"@!@!@ENDMSG 1000 @!@!@\n"
	messages, loose := parseMessages(output)
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(messages))
	}
	if !strings.Contains(messages[0].text, "outer text") || !strings.Contains(messages[0].text, "inner text") {
		t.Errorf("outer = %q", messages[0].text)
	}
	if strings.Contains(messages[1].text, "outer text") || !strings.Contains(messages[1].text, "inner text") {
		t.Errorf("inner = %q", messages[1].text)
	}
	if strings.TrimSpace(loose) != "loose" {
		t.Errorf("loose = %q", loose)
	}
}

func TestParseSurvivesGarbage(t *testing.T) {
	inputs := []string{
		"",
		"not TLC output at all",
		"@!@!@ENDMSG 2217 @!@!@",
		"@!@!@STARTMSG 2217:4 @!@!@",
		"@!@!@STARTMSG 2217:4 @!@!@\n@!@!@ENDMSG 2217 @!@!@",
		"@!@!@STARTMSG 2107:1 @!@!@\n@!@!@ENDMSG 2107 @!@!@",
		"@!@!@STARTMSG 2122:4 @!@!@\nnonsense\n@!@!@ENDMSG 2122 @!@!@",
		"@!@!@STARTMSG 99999999999999999999:4 @!@!@\n@!@!@ENDMSG 99999999999999999999 @!@!@",
	}
	for _, input := range inputs {
		result := newResult(0, input)
		if result.Passed() {
			t.Errorf("newResult(%q) passed", input)
		}
		if result.String() == "" {
			t.Errorf("newResult(%q).String() is empty", input)
		}
	}
}

func TestWorkerIsLeftBlankWhenTLCDoesNotShowWhichOne(t *testing.T) {
	before := State{Action: "start", Vars: map[string]string{"pc": `(a :> "spin" @@ b :> "spin")`}}
	after := State{Action: "spin", Vars: map[string]string{"pc": `(a :> "spin" @@ b :> "spin")`}}
	if got := movedWorker(before, after); got != "" {
		t.Fatalf("movedWorker() = %q, want blank", got)
	}
}

func TestResultString(t *testing.T) {
	tests := []struct {
		sample   string
		exitCode int
		violated string
		want     string
	}{
		{
			sample: "invariant", exitCode: 12,
			want: strings.Join([]string{
				"FAILED: NoOverdraft was broken after 5 steps",
				"",
				"1  start     balance=100  a=check  b=check",
				"2  a: check  balance=100  a=debit  b=check",
				"3  b: check  balance=100  a=debit  b=debit",
				"4  a: debit  balance=40   a=done   b=debit",
				"5  b: debit  balance=-20  a=done   b=done",
			}, "\n"),
		},
		{sample: "pass", exitCode: 0, want: "PASSED: 6 states generated, 4 distinct"},
		{
			sample: "deadlock", exitCode: 11,
			want: strings.Join([]string{
				"FAILED: deadlock after 3 steps, the workers are stuck",
				"",
				"1  start     count=0  open=FALSE  a=first  b=first",
				"2  a: first  count=1  open=FALSE  a=wait   b=first",
				"3  b: first  count=2  open=FALSE  a=wait   b=wait",
			}, "\n"),
		},
		{
			sample: "invariant_at_start", exitCode: 12,
			want: strings.Join([]string{
				"FAILED: Positive was broken after 1 step",
				"",
				"1  start  x=0  y=1  a=one",
			}, "\n"),
		},
		{
			sample: "liveness_stutter", exitCode: 13, violated: "AllFinish",
			want: strings.Join([]string{
				"FAILED: AllFinish was broken after 2 steps",
				"",
				"1  start        balance=100  a=withdraw  b=withdraw",
				"2  b: withdraw  balance=40   a=withdraw  b=done",
				"",
				"Then nothing happens, forever.",
			}, "\n"),
		},
		{
			sample: "liveness_loop", exitCode: 13, violated: "Finishes",
			want: strings.Join([]string{
				"FAILED: Finishes was broken after 3 steps",
				"",
				"1  start     flag=FALSE  a=spin",
				"2  a: spin   flag=TRUE   a=again",
				"3  a: again  flag=TRUE   a=spin",
				"",
				"Then it goes back to step 1 and repeats forever.",
			}, "\n"),
		},
	}
	for _, test := range tests {
		t.Run(test.sample, func(t *testing.T) {
			result := newResult(test.exitCode, sample(t, test.sample))
			if test.violated != "" {
				result.Violated = test.violated
			}
			if got := result.String(); got != test.want {
				t.Errorf("String() =\n%s\n\nwant\n%s", got, test.want)
			}
		})
	}
}

func TestResultStringExplainsToolErrors(t *testing.T) {
	tests := []struct {
		sample   string
		exitCode int
		want     []string
	}{
		{"parse_error", 150, []string{"ERROR: TLC could not check this spec (exit code 150)", "Parse Error", "Parsing or semantic analysis failed."}},
		{"evaluation_error", 75, []string{"ERROR: TLC could not check this spec (exit code 75)", "evaluating", "a: one"}},
	}
	for _, test := range tests {
		t.Run(test.sample, func(t *testing.T) {
			got := newResult(test.exitCode, sample(t, test.sample)).String()
			for _, text := range test.want {
				if !strings.Contains(got, text) {
					t.Errorf("String() does not contain %q:\n%s", text, got)
				}
			}
			if strings.Contains(got, "Parsing file") || strings.Contains(got, "Semantic processing") {
				t.Errorf("String() contains progress noise:\n%s", got)
			}
		})
	}
}

func TestNilResultString(t *testing.T) {
	var missing *Result
	if missing.String() == "" {
		t.Error("nil result printed nothing")
	}
}
