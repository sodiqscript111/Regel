package regel

type Op interface {
	op()
}

type step struct {
	label string
	ops   []Op
}

type setOp struct {
	variable string
	expr     string
}

type awaitOp struct {
	condition string
}

type gotoOp struct {
	target string
}

type Branch struct {
	condition string
	then      string
	otherwise string
}

func (setOp) op()   {}
func (awaitOp) op() {}
func (gotoOp) op()  {}
func (Branch) op()  {}

func Set(variable, expr string) Op {
	return setOp{variable: variable, expr: expr}
}

func Await(condition string) Op {
	return awaitOp{condition: condition}
}

func Goto(label string) Op {
	return gotoOp{target: label}
}

func If(condition string) Branch {
	return Branch{condition: condition}
}

func (b Branch) Goto(label string) Branch {
	b.then = label
	return b
}

func (b Branch) Else(label string) Branch {
	b.otherwise = label
	return b
}
