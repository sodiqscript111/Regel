package regel

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const doneLabel = "done"

var identifierPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

var reservedNames = nameSet(
	"pc", "self", doneLabel, "vars", "Init", "Next", "Spec", "Worker", "Workers", "Finished", "TypeOK",

	"ACTION", "ASSUME", "ASSUMPTION", "AXIOM", "BOOLEAN", "BY", "CASE", "CHOOSE", "CONSTANT", "CONSTANTS",
	"COROLLARY", "DEF", "DEFINE", "DEFS", "DOMAIN", "ELSE", "ENABLED", "EXCEPT", "EXTENDS", "FALSE", "HAVE",
	"HIDE", "IF", "IN", "INSTANCE", "LAMBDA", "LEMMA", "LET", "LOCAL", "MODULE", "NEW", "OBVIOUS", "OMITTED",
	"ONLY", "OTHER", "PICK", "PROOF", "PROPOSITION", "PROVE", "QED", "RECURSIVE", "STATE", "STRING", "SUBSET",
	"SUFFICES", "TAKE", "TEMPORAL", "THEN", "THEOREM", "TRUE", "UNCHANGED", "UNION", "USE", "VARIABLE",
	"VARIABLES", "WITH", "WITNESS",

	"ACTION_CONSTRAINT", "ACTION_CONSTRAINTS", "ALIAS", "CHECK_DEADLOCK", "CONSTRAINT", "CONSTRAINTS", "INIT",
	"INVARIANT", "INVARIANTS", "NEXT", "POSTCONDITION", "PROPERTIES", "PROPERTY", "SPECIFICATION", "SYMMETRY",
	"VIEW",

	"Integers", "Naturals", "Sequences", "FiniteSets", "TLC",
	"Nat", "Int", "Seq", "Len", "Append", "Head", "Tail", "SubSeq", "SelectSeq", "IsFiniteSet", "Cardinality",
	"Print", "PrintT", "Assert", "JavaTime", "Permutations", "SortSeq", "ToString", "RandomElement", "Any",
	"TLCGet", "TLCSet", "TLCEval",
)

func nameSet(names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

type validator struct {
	problems []error
}

func (s *Spec) Validate() error {
	if s == nil {
		return errors.New("spec is nil")
	}
	v := &validator{}
	v.checkName("module", s.name)
	declared := v.checkVariables(s)
	v.checkPrograms(s, declared)
	v.checkRules(s, declared)
	return errors.Join(v.problems...)
}

func (v *validator) report(format string, args ...any) {
	v.problems = append(v.problems, fmt.Errorf(format, args...))
}

func (v *validator) checkName(kind, name string) {
	switch {
	case name == "":
		v.report("%s name is empty", kind)
	case !isIdentifier(name):
		v.report("%s %q is not a valid TLA+ identifier: use a letter, then letters, digits, or underscores", kind, name)
	case reservedNames[name]:
		v.report("%s %q clashes with a reserved name", kind, name)
	case strings.HasPrefix(name, stepPrefix):
		v.report("%s %q clashes with the reserved prefix %s", kind, name, stepPrefix)
	}
}

func isIdentifier(name string) bool {
	if strings.HasPrefix(name, "WF_") || strings.HasPrefix(name, "SF_") {
		return false
	}
	return identifierPattern.MatchString(name)
}

func (v *validator) checkExpr(owner, expr string) {
	if strings.TrimSpace(expr) == "" {
		v.report("%s: expression is empty", owner)
	}
}

func (v *validator) checkVariables(s *Spec) map[string]bool {
	declared := map[string]bool{}
	for _, variable := range s.vars {
		v.checkName("variable", variable.name)
		if declared[variable.name] {
			v.report("variable %q is declared more than once", variable.name)
		}
		declared[variable.name] = true
		v.checkInitial(variable)
	}
	return declared
}

func (v *validator) checkInitial(variable variable) {
	switch initial := variable.initial.(type) {
	case int, bool, string:
	case RawValue:
		v.checkExpr(fmt.Sprintf("variable %q", variable.name), string(initial))
	default:
		v.report("variable %q has an unsupported initial value of type %T: use int, bool, string, or Raw", variable.name, initial)
	}
}

func (v *validator) checkPrograms(s *Spec, declared map[string]bool) {
	if len(s.programs) == 0 {
		v.report("spec has no workers")
		v.report("spec has no steps")
		return
	}
	workers := map[string]bool{}
	labels := map[string]bool{}
	for _, p := range s.programs {
		v.checkWorkers(p, workers, declared)
		v.checkSteps(p, labels, declared)
	}
}

func (p *program) title() string {
	if p.name == "" {
		return "spec"
	}
	return fmt.Sprintf("program %q", p.name)
}

func (v *validator) checkWorkers(p *program, workers, declared map[string]bool) {
	if len(p.workers) == 0 {
		v.report("%s has no workers", p.title())
	}
	for _, worker := range p.workers {
		v.checkName("worker", worker)
		if workers[worker] {
			v.report("worker %q is declared more than once", worker)
		}
		workers[worker] = true
		if declared[worker] {
			v.report("worker %q has the same name as a variable", worker)
		}
	}
}

func (v *validator) checkSteps(p *program, labels, declared map[string]bool) {
	if len(p.steps) == 0 {
		v.report("%s has no steps", p.title())
	}
	targets := map[string]bool{doneLabel: true}
	for _, step := range p.steps {
		targets[step.label] = true
	}
	for _, step := range p.steps {
		v.checkName("step", step.label)
		if labels[step.label] {
			v.report("step %q is declared more than once", step.label)
		}
		labels[step.label] = true
		v.checkOps(step, targets, declared)
	}
}

func (v *validator) checkOps(step step, targets, declared map[string]bool) {
	owner := fmt.Sprintf("step %q", step.label)
	written := map[string]bool{}
	controls := 0
	for _, op := range step.ops {
		switch op := op.(type) {
		case setOp:
			if !declared[op.variable] {
				v.report("%s: Set on undeclared variable %q", owner, op.variable)
			}
			if written[op.variable] {
				v.report("%s: variable %q is set more than once", owner, op.variable)
			}
			written[op.variable] = true
			v.checkExpr(owner, op.expr)
		case awaitOp:
			v.checkExpr(owner, op.condition)
		case gotoOp:
			controls++
			v.checkTarget(owner, "Goto", op.target, targets)
		case Branch:
			controls++
			v.checkExpr(owner, op.condition)
			v.checkTarget(owner, "If Goto", op.then, targets)
			v.checkTarget(owner, "If Else", op.otherwise, targets)
		default:
			v.report("%s: operation is nil", owner)
		}
	}
	if controls > 1 {
		v.report("%s has %d control operations: use at most one Goto or If", owner, controls)
	}
}

func (v *validator) checkTarget(owner, kind, target string, targets map[string]bool) {
	if target == "" {
		v.report("%s: %s has no target", owner, kind)
		return
	}
	if !targets[target] {
		v.report("%s: %s target %q does not exist", owner, kind, target)
	}
}

func (v *validator) checkRules(s *Spec, declared map[string]bool) {
	seen := map[string]bool{}
	for _, rule := range s.rules {
		v.checkName("rule", rule.name)
		if seen[rule.name] {
			v.report("rule %q is declared more than once", rule.name)
		}
		seen[rule.name] = true
		if declared[rule.name] {
			v.report("rule %q has the same name as a variable", rule.name)
		}
		v.checkExpr(fmt.Sprintf("rule %q", rule.name), rule.expr)
	}
}
