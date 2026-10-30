package regel_test

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	regel "github.com/sodiqscript111/Regel"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func generated(t *testing.T, spec *regel.Spec) (source, config string) {
	t.Helper()
	source, err := spec.Source()
	if err != nil {
		t.Fatalf("Source() = %v", err)
	}
	config, err = spec.Config()
	if err != nil {
		t.Fatalf("Config() = %v", err)
	}
	return source, config
}

func TestGolden(t *testing.T) {
	specs := map[string]*regel.Spec{
		"Withdraw":      withdraw(),
		"WithdrawFixed": withdrawFixed(),
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			source, config := generated(t, spec)
			compareGolden(t, name+".tla", source)
			compareGolden(t, name+".cfg", config)
		})
	}
}

func compareGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s differs from the golden file\n--- got\n%s\n--- want\n%s", name, got, want)
	}
}

func TestGenerationIsDeterministic(t *testing.T) {
	spec := withdraw()
	firstSource, firstConfig := generated(t, spec)
	secondSource, secondConfig := generated(t, spec)
	rebuiltSource, rebuiltConfig := generated(t, withdraw())
	if firstSource != secondSource || firstSource != rebuiltSource {
		t.Error("Source() changed between calls")
	}
	if firstConfig != secondConfig || firstConfig != rebuiltConfig {
		t.Error("Config() changed between calls")
	}
}

func TestGenerationValidatesFirst(t *testing.T) {
	bad := regel.New("")
	if source, err := bad.Source(); err == nil || source != "" {
		t.Errorf("Source() = %q, %v, want an error", source, err)
	}
	if config, err := bad.Config(); err == nil || config != "" {
		t.Errorf("Config() = %q, %v, want an error", config, err)
	}
	var missing *regel.Spec
	if _, err := missing.Source(); err == nil {
		t.Error("Source() on a nil spec = nil error")
	}
	if _, err := missing.Config(); err == nil {
		t.Error("Config() on a nil spec = nil error")
	}
}

func TestSourceLines(t *testing.T) {
	tests := []struct {
		name string
		spec *regel.Spec
		want []string
		deny []string
	}{
		{
			name: "default flow moves to the next step, then to done",
			spec: regel.New("M").Workers("a").Step("one").Step("two"),
			want: []string{
				`/\ pc' = [pc EXCEPT ![self] = "two"]`,
				`/\ pc' = [pc EXCEPT ![self] = "done"]`,
			},
		},
		{
			name: "goto overrides the default flow",
			spec: regel.New("M").Workers("a").Step("one", regel.Goto("one")).Step("two"),
			want: []string{`/\ pc' = [pc EXCEPT ![self] = "one"]`},
		},
		{
			name: "if becomes a conditional pc update",
			spec: regel.New("M").Var("x", 0).Workers("a").Step("one", regel.If("x > 0").Goto("two").Else("done")).Step("two"),
			want: []string{`/\ pc' = [pc EXCEPT ![self] = IF x > 0 THEN "two" ELSE "done"]`},
		},
		{
			name: "await becomes a condition",
			spec: regel.New("M").Var("open", false).Workers("a").Step("one", regel.Await("open")),
			want: []string{"    /\\ open\n", `/\ UNCHANGED open`},
		},
		{
			name: "set on a shared variable keeps its expression together",
			spec: regel.New("M").Var("ok", true).Var("x", 0).Workers("a").Step("one", regel.Set("ok", `x = 0 /\ ok`)),
			want: []string{`/\ ok' = (x = 0 /\ ok)`},
		},
		{
			name: "set on a local uses except",
			spec: regel.New("M").Var("x", 0).Local("tmp", 0).Workers("a").Step("one", regel.Set("tmp", "x")),
			want: []string{
				`/\ tmp' = [tmp EXCEPT ![self] = x]`,
				`/\ UNCHANGED x`,
				`/\ tmp = [self \in Workers |-> 0]`,
			},
		},
		{
			name: "unchanged lists every variable the step leaves alone",
			spec: regel.New("M").Var("x", 0).Var("y", 0).Var("z", 0).Workers("a").Step("one", regel.Set("y", "1")),
			want: []string{`/\ UNCHANGED <<x, z>>`},
		},
		{
			name: "no unchanged line when every variable is set",
			spec: regel.New("M").Var("x", 0).Workers("a").Step("one", regel.Set("x", "1")),
			deny: []string{`/\ UNCHANGED x`},
		},
		{
			name: "initial values",
			spec: regel.New("M").
				Var("count", -3).Var("yes", true).Var("no", false).Var("text", "hi").Var("set", regel.Raw("{1, 2}")).
				Workers("a").Step("one"),
			want: []string{`/\ count = -3`, `/\ yes = TRUE`, `/\ no = FALSE`, `/\ text = "hi"`, `/\ set = ({1, 2})`},
		},
		{
			name: "strings use TLA+ escaping",
			spec: regel.New("M").Var("text", "a\"b\\c\nd\te").Workers("a").Step("one"),
			want: []string{`/\ text = "a\"b\\c\nd\te"`},
		},
		{
			name: "only the base modules by default",
			spec: regel.New("M").Workers("a").Step("one"),
			want: []string{"EXTENDS Integers, TLC\n"},
		},
		{
			name: "sequences when an expression needs them",
			spec: regel.New("M").Var("q", regel.Raw("<<>>")).Workers("a").Step("one", regel.Set("q", "Append(q, 1)")),
			want: []string{"EXTENDS Integers, TLC, Sequences\n"},
		},
		{
			name: "finite sets when an expression needs them",
			spec: regel.New("M").Var("s", regel.Raw("{}")).Workers("a").Step("one").Always("Small", "Cardinality(s) < 2"),
			want: []string{"EXTENDS Integers, TLC, FiniteSets\n"},
		},
		{
			name: "both modules when both are needed",
			spec: regel.New("M").Var("q", regel.Raw("<<1>> \\o <<2>>")).Workers("a").Step("one", regel.Await("Cardinality({1}) = 1")),
			want: []string{"EXTENDS Integers, TLC, Sequences, FiniteSets\n"},
		},
		{
			name: "a name that merely contains a module word needs nothing extra",
			spec: regel.New("M").Var("Lens", 0).Workers("a").Step("one", regel.Set("Lens", "Lens + 1")),
			want: []string{"EXTENDS Integers, TLC\n"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source, err := test.spec.Source()
			if err != nil {
				t.Fatalf("Source() = %v", err)
			}
			for _, text := range test.want {
				if !strings.Contains(source, text) {
					t.Errorf("source does not contain %q:\n%s", text, source)
				}
			}
			for _, text := range test.deny {
				if strings.Contains(source, text) {
					t.Errorf("source contains %q:\n%s", text, source)
				}
			}
		})
	}
}

func TestSourceHasNoComments(t *testing.T) {
	source, _ := generated(t, withdraw())
	if strings.Contains(source, `\*`) || strings.Contains(source, "(*") {
		t.Errorf("generated source has a comment:\n%s", source)
	}
}

func TestConfigListsWorkersAndRules(t *testing.T) {
	config, err := regel.New("M").Workers("x", "y", "z").Step("one").Always("A", "TRUE").Always("B", "TRUE").Config()
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"SPECIFICATION Spec", "Workers = {x, y, z}", "TypeOK", "    A\n", "    B\n"} {
		if !strings.Contains(config, text) {
			t.Errorf("config does not contain %q:\n%s", text, config)
		}
	}
}

func TestSANYAcceptsGeneratedSpecs(t *testing.T) {
	java, jar := tlcTools(t)
	specs := map[string]*regel.Spec{
		"Withdraw":      withdraw(),
		"WithdrawFixed": withdrawFixed(),
		"Everything": regel.New("Everything").
			Var("count", -1).
			Var("flag", true).
			Var("text", "a \"quoted\" \\ word\n").
			Var("queue", regel.Raw("<<>>")).
			Var("seen", regel.Raw("{}")).
			Local("tmp", 0).
			Local("note", "none").
			Workers("a", "b", "c").
			Step("init", regel.Set("tmp", "count"), regel.Set("note", `"read"`)).
			Step("lock", regel.Await(`flag \/ count > 0`), regel.Set("flag", `count > 0 /\ flag`)).
			Step("next", regel.Set("queue", "Append(queue, self)"), regel.Set("seen", `seen \union {self}`)).
			Step("loop", regel.If("tmp[self] < 2").Goto("init").Else("done")).
			Always("Bounded", "Cardinality(seen) <= 3").
			Always("Short", "Len(queue) >= 0"),
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			source, _ := generated(t, spec)
			if err := sany(java, jar, t.TempDir(), name, source); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSANYCheckCatchesBadSpecs(t *testing.T) {
	java, jar := tlcTools(t)
	bad := map[string]string{
		"Unknown": "---- MODULE Unknown ----\nInit == missing = 1\n====\n",
		"Broken":  "---- MODULE Broken ----\nInit == /\\ x =\n====\n",
	}
	for name, source := range bad {
		t.Run(name, func(t *testing.T) {
			if err := sany(java, jar, t.TempDir(), name, source); err == nil {
				t.Fatal("sany() = nil, want an error")
			}
		})
	}
}

func sany(java, jar, dir, name, source string) error {
	file := name + ".tla"
	if err := os.WriteFile(filepath.Join(dir, file), []byte(source), 0o644); err != nil {
		return err
	}
	command := exec.Command(java, "-cp", jar, "tla2sany.SANY", file)
	command.Dir = dir
	output, err := command.CombinedOutput()
	text := string(output)
	if err != nil {
		return fmt.Errorf("SANY failed: %w\n%s\n%s", err, text, source)
	}
	if strings.Contains(text, "Semantic errors") || strings.Contains(text, "*** Errors") {
		return fmt.Errorf("SANY reported errors:\n%s\n%s", text, source)
	}
	if !strings.Contains(text, "Semantic processing of module "+name) {
		return fmt.Errorf("SANY did not finish:\n%s\n%s", text, source)
	}
	return nil
}
