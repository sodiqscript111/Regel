package regel

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const stepPrefix = "Step_"

var (
	sequencesUse  = regexp.MustCompile(`\b(Seq|Len|Append|Head|Tail|SubSeq|SelectSeq)\b|\\o\b|\\circ\b`)
	finiteSetsUse = regexp.MustCompile(`\b(Cardinality|IsFiniteSet)\b`)
)

func (s *Spec) Source() (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	g := &generator{spec: s}
	g.header()
	g.declarations()
	g.init()
	g.actions()
	g.finished()
	g.worker()
	g.next()
	g.specification()
	g.typeOK()
	g.rules()
	g.footer()
	return g.out.String(), nil
}

func (s *Spec) Config() (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteString("SPECIFICATION Spec\n\nCONSTANTS\n")
	fmt.Fprintf(&out, "    Workers = {%s}\n", strings.Join(s.allWorkers(), ", "))
	out.WriteString("\nINVARIANTS\n    TypeOK\n")
	for _, rule := range s.rules {
		fmt.Fprintf(&out, "    %s\n", rule.name)
	}
	return out.String(), nil
}

func (s *Spec) allWorkers() []string {
	var workers []string
	for _, p := range s.programs {
		workers = append(workers, p.workers...)
	}
	return workers
}

func (s *Spec) variableNames() []string {
	names := make([]string, 0, len(s.vars)+1)
	for _, variable := range s.vars {
		names = append(names, variable.name)
	}
	return append(names, "pc")
}

func (s *Spec) isLocal(name string) bool {
	for _, variable := range s.vars {
		if variable.name == name {
			return variable.local
		}
	}
	return false
}

func (s *Spec) expressions() []string {
	var exprs []string
	for _, variable := range s.vars {
		if raw, ok := variable.initial.(RawValue); ok {
			exprs = append(exprs, string(raw))
		}
	}
	for _, p := range s.programs {
		for _, step := range p.steps {
			exprs = append(exprs, step.expressions()...)
		}
	}
	for _, rule := range s.rules {
		exprs = append(exprs, rule.expr)
	}
	return exprs
}

func (s step) expressions() []string {
	var exprs []string
	for _, op := range s.ops {
		switch op := op.(type) {
		case setOp:
			exprs = append(exprs, op.expr)
		case awaitOp:
			exprs = append(exprs, op.condition)
		case Branch:
			exprs = append(exprs, op.condition)
		}
	}
	return exprs
}

type generator struct {
	spec *Spec
	out  strings.Builder
}

func (g *generator) printf(format string, args ...any) {
	fmt.Fprintf(&g.out, format, args...)
}

func (g *generator) header() {
	g.printf("---- MODULE %s ----\n", g.spec.name)
	g.printf("EXTENDS %s\n\n", strings.Join(g.modules(), ", "))
}

func (g *generator) modules() []string {
	modules := []string{"Integers", "TLC"}
	exprs := strings.Join(g.spec.expressions(), "\n")
	if sequencesUse.MatchString(exprs) {
		modules = append(modules, "Sequences")
	}
	if finiteSetsUse.MatchString(exprs) {
		modules = append(modules, "FiniteSets")
	}
	return modules
}

func (g *generator) declarations() {
	names := strings.Join(g.spec.variableNames(), ", ")
	g.printf("CONSTANT Workers\n\n")
	g.printf("VARIABLES %s\n\n", names)
	g.printf("vars == <<%s>>\n\n", names)
}

func (g *generator) init() {
	g.printf("Init ==\n")
	for _, variable := range g.spec.vars {
		g.printf("    /\\ %s\n", initialState(variable))
	}
	g.printf("    /\\ pc = [self \\in Workers |-> %s]\n\n", tlaString(g.spec.programs[0].firstLabel()))
}

func (p *program) firstLabel() string {
	return p.steps[0].label
}

func initialState(v variable) string {
	if v.local {
		return fmt.Sprintf("%s = [self \\in Workers |-> %s]", v.name, tlaValue(v.initial))
	}
	if raw, ok := v.initial.(RawValue); ok {
		return fmt.Sprintf("%s = (%s)", v.name, raw)
	}
	return fmt.Sprintf("%s = %s", v.name, tlaValue(v.initial))
}

func tlaValue(value any) string {
	switch value := value.(type) {
	case int:
		return strconv.Itoa(value)
	case bool:
		if value {
			return "TRUE"
		}
		return "FALSE"
	case string:
		return tlaString(value)
	case RawValue:
		return string(value)
	}
	return ""
}

func tlaString(text string) string {
	return `"` + stringEscaper.Replace(text) + `"`
}

var stringEscaper = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
	"\n", `\n`,
	"\t", `\t`,
	"\r", `\r`,
	"\f", `\f`,
)

func (g *generator) actions() {
	for _, p := range g.spec.programs {
		for i, step := range p.steps {
			g.action(step, p.following(i))
		}
	}
}

func (p *program) following(index int) string {
	if index+1 < len(p.steps) {
		return p.steps[index+1].label
	}
	return doneLabel
}

func actionName(label string) string {
	return stepPrefix + label
}

func (g *generator) action(s step, following string) {
	g.printf("%s(self) ==\n", actionName(s.label))
	g.printf("    /\\ pc[self] = %s\n", tlaString(s.label))
	written := map[string]bool{}
	for _, op := range s.ops {
		switch op := op.(type) {
		case setOp:
			g.printf("    /\\ %s\n", g.assignment(op))
			written[op.variable] = true
		case awaitOp:
			g.printf("    /\\ %s\n", op.condition)
		}
	}
	g.printf("    /\\ pc' = [pc EXCEPT ![self] = %s]\n", s.nextPC(following))
	if unchanged := g.unchanged(written); unchanged != "" {
		g.printf("    /\\ UNCHANGED %s\n", unchanged)
	}
	g.printf("\n")
}

func (g *generator) assignment(op setOp) string {
	if g.spec.isLocal(op.variable) {
		return fmt.Sprintf("%[1]s' = [%[1]s EXCEPT ![self] = %[2]s]", op.variable, op.expr)
	}
	return fmt.Sprintf("%s' = (%s)", op.variable, op.expr)
}

func (s step) nextPC(following string) string {
	for _, op := range s.ops {
		switch op := op.(type) {
		case gotoOp:
			return tlaString(op.target)
		case Branch:
			return fmt.Sprintf("IF %s THEN %s ELSE %s", op.condition, tlaString(op.then), tlaString(op.otherwise))
		}
	}
	return tlaString(following)
}

func (g *generator) unchanged(written map[string]bool) string {
	var names []string
	for _, variable := range g.spec.vars {
		if !written[variable.name] {
			names = append(names, variable.name)
		}
	}
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return "<<" + strings.Join(names, ", ") + ">>"
}

func (g *generator) finished() {
	g.printf("Finished ==\n")
	g.printf("    /\\ \\A self \\in Workers : pc[self] = %s\n", tlaString(doneLabel))
	g.printf("    /\\ UNCHANGED vars\n\n")
}

func (g *generator) worker() {
	g.printf("Worker(self) ==\n")
	for _, step := range g.spec.programs[0].steps {
		g.printf("    \\/ %s(self)\n", actionName(step.label))
	}
	g.printf("\n")
}

func (g *generator) next() {
	g.printf("Next == (\\E self \\in Workers : Worker(self)) \\/ Finished\n\n")
}

func (g *generator) specification() {
	g.printf("Spec == Init /\\ [][Next]_vars\n\n")
}

func (g *generator) typeOK() {
	var labels []string
	for _, p := range g.spec.programs {
		for _, step := range p.steps {
			labels = append(labels, tlaString(step.label))
		}
	}
	labels = append(labels, tlaString(doneLabel))
	g.printf("TypeOK == pc \\in [Workers -> {%s}]\n\n", strings.Join(labels, ", "))
}

func (g *generator) rules() {
	for _, rule := range g.spec.rules {
		g.printf("%s == %s\n\n", rule.name, rule.expr)
	}
}

func (g *generator) footer() {
	g.printf("====\n")
}
