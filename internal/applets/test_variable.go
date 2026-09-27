package applets

// variableView is the shell behind a process view, when there is one. Only a shell can say
// whether one of its variables is set.
type variableView interface {
	VariableIsSet(name string) bool
}

// variableIsSet is `test -v name` and `[ -v name ]`: whether a shell variable is set, empty or
// not, with a subscript naming one element of an array. It is bash's, from 4.2; busybox's
// test has no -v and calls the name an unknown operand. It was that here too, so `[ -v x ]`
// was status 2 whatever x was, where `[[ -v x ]]` already answered.
//
// The shell answers when test runs inside one. Run on its own, test has only the
// environment to look in.
func (e *testEvaluator) variableIsSet(name string) bool {
	if shell, ok := e.view.(variableView); ok {
		return shell.VariableIsSet(name)
	}
	_, set := e.view.LookupEnv(name)
	return set
}

// optionView is the shell behind a process view, asked whether one of its options is on.
type optionView interface {
	ShellOptionIsOn(name string) bool
}

// optionIsOn is `test -o name` and `[ -o name ]`, bash's: whether a `set -o` option is on,
// and off for a name that is not one. busybox's test calls the name an unknown operand. Run
// on its own, test has no shell and no options, so every name is off.
func (e *testEvaluator) optionIsOn(name string) bool {
	if shell, ok := e.view.(optionView); ok {
		return shell.ShellOptionIsOn(name)
	}
	return false
}
