package regel

const (
	NoDeadlock = "NoDeadlock"

	finishesRule = "Finishes"
)

type Spec struct {
	name     string
	vars     []variable
	programs []*program
	rules    []rule
}

type variable struct {
	name    string
	initial any
	local   bool
}

type program struct {
	name    string
	workers []string
	steps   []step
}

type rule struct {
	name     string
	expr     string
	liveness bool
}

type RawValue string

func Raw(expr string) RawValue {
	return RawValue(expr)
}

func New(name string) *Spec {
	return &Spec{name: name}
}

func (s *Spec) Var(name string, value any) *Spec {
	return s.declare(variable{name: name, initial: value})
}

func (s *Spec) Local(name string, value any) *Spec {
	return s.declare(variable{name: name, initial: value, local: true})
}

func (s *Spec) declare(v variable) *Spec {
	if s == nil {
		return nil
	}
	s.vars = append(s.vars, v)
	return s
}

func (s *Spec) Workers(names ...string) *Spec {
	if s == nil {
		return nil
	}
	p := s.defaultProgram()
	p.workers = append(p.workers, names...)
	return s
}

func (s *Spec) Step(label string, ops ...Op) *Spec {
	if s == nil {
		return nil
	}
	p := s.defaultProgram()
	p.steps = append(p.steps, step{label: label, ops: ops})
	return s
}

func (s *Spec) Always(name, expr string) *Spec {
	return s.addRule(rule{name: name, expr: expr})
}

func (s *Spec) Eventually(name, expr string) *Spec {
	return s.addRule(rule{name: name, expr: expr, liveness: true})
}

func (s *Spec) Finishes() *Spec {
	return s.Eventually(finishesRule, `\A self \in Workers : pc[self] = "done"`)
}

func (s *Spec) addRule(r rule) *Spec {
	if s == nil {
		return nil
	}
	s.rules = append(s.rules, r)
	return s
}

func (s *Spec) defaultProgram() *program {
	for _, p := range s.programs {
		if p.name == "" {
			return p
		}
	}
	p := &program{}
	s.programs = append(s.programs, p)
	return p
}
