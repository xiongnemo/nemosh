package applets

// Calling a user-defined function.
//
// The whole of awk's parameter passing is one sentence with two halves, and both halves
// were measured: **scalars go by value and arrays go by reference.** So this leaves `x` at
// 1:
//
//	function f(a) { a = 99 }   BEGIN { x = 1; f(x); print x }
//
// while this prints 99, because `B` and the parameter `A` are two names for one array:
//
//	function f(A) { A["k"] = 99 }   BEGIN { f(B); print B["k"] }
//
// The awkward case in between is a name that is **neither yet**. `f(x)` where `x` has never
// been mentioned and the function writes `a[1] = 1` leaves the caller with a one-element
// array, which both references agree on. It is handled by binding such a name to a fresh
// array in the caller's scope *and* passing the uninitialised scalar: whichever way the
// callee uses the parameter, the right thing is already there, and a name the callee only
// reads as a scalar is left holding an empty array that answers `""` and `0` exactly as an
// unset name does.
//
// Recursion needs no special handling beyond the frame stack, but it does need a limit: awk
// has no stack to overflow gracefully, and Go's would take the process with it.

// awkCallDepth is how deep a recursion may go before the interpreter refuses.
//
// Both references manage a plain recursion 2000 deep, which is the measurement this is
// chosen to clear comfortably. The point of a limit at all is that a runaway recursion must
// produce a diagnostic rather than a Go stack overflow, which cannot be recovered from and
// would take the shell down with the applet.
const awkCallDepth = 10000

func (in *awkInterp) evalCall(node awkCallExpr) (awkValue, error) {
	function, defined := in.program.functions[node.name]
	if !defined {
		return awkValue{}, in.errorf("call to undefined function %s", node.name)
	}
	if len(node.args) > len(function.params) {
		return awkValue{}, in.errorf("function %s takes %d arguments, not %d",
			node.name, len(function.params), len(node.args))
	}
	frame, err := in.bindArguments(function, node.args)
	if err != nil {
		return awkValue{}, err
	}
	if len(in.frames) >= awkCallDepth {
		return awkValue{}, in.errorf("function %s recursed more than %d deep", node.name, awkCallDepth)
	}
	in.frames = append(in.frames, frame)
	defer func() { in.frames = in.frames[:len(in.frames)-1] }()

	flow, err := in.execBlock(function.body)
	if err != nil {
		return awkValue{}, err
	}
	if flow == awkFlowReturn {
		// returned is on the interpreter rather than in the flow signal because a flow is
		// a plain value; the returning statement leaves what it answered here.
		return in.returned, nil
	}
	// Falling off the end answers the uninitialised value, which is both "" and 0 -- so
	// `print "[" f() "]"` prints `[]` and `f() + 0` is 0.
	return awkValue{}, nil
}

// bindArguments builds the call's frame.
//
// Built *before* the frame is pushed, because the arguments are expressions in the
// **caller's** scope: `f(i)` inside another function must pass that function's `i`.
func (in *awkInterp) bindArguments(function *awkFunction, args []awkExpr) (*awkFrame, error) {
	frame := &awkFrame{
		names:  make(map[string]bool, len(function.params)),
		values: make(map[string]awkValue, len(function.params)),
		arrays: map[string]*awkArray{},
	}
	for _, name := range function.params {
		// Every parameter shadows its global from the start of the call, including the
		// extra ones the caller did not pass. That is what makes them usable as locals.
		frame.names[name] = true
	}
	for index, arg := range args {
		name := function.params[index]
		if bound, shared := in.bindByName(arg, function.arrayParams[name]); shared {
			frame.arrays[name] = bound
			continue
		}
		value, err := in.eval(arg)
		if err != nil {
			return nil, err
		}
		frame.values[name] = value
	}
	return frame, nil
}

// bindByName answers the array an argument shares with the caller, if it shares one.
//
// Only a **bare name** can go by reference: `f(a)` shares, while `f(a[1])` and `f(a "")`
// are values whatever the parameter is.
//
// A name that already holds an array always shares it. A name that holds nothing yet shares
// one only when the callee is known to use that parameter as an array -- which
// awk_paramtypes.go worked out from the body before the program started. Deciding it here
// instead, by binding anything not yet typed, silently turned a scalar accumulator into an
// array on its first call.
func (in *awkInterp) bindByName(arg awkExpr, wantsArray bool) (*awkArray, bool) {
	name, isName := arg.(awkVarExpr)
	if !isName {
		return nil, false
	}
	if array, known := in.lookupArray(name.name); known {
		return array, true
	}
	if !wantsArray {
		return nil, false
	}
	// Untyped in the caller and used as an array in the callee, so the caller ends up
	// holding the array the callee fills.
	return in.getArray(name.name), true
}

// execReturn leaves a function, with a value or without one.
func (in *awkInterp) execReturn(node awkReturnStmt) (awkFlow, error) {
	if in.frame() == nil {
		return awkFlowNone, in.errorf("return outside a function")
	}
	in.returned = awkValue{}
	if node.value == nil {
		return awkFlowReturn, nil
	}
	value, err := in.eval(node.value)
	if err != nil {
		return awkFlowNone, err
	}
	in.returned = value
	return awkFlowReturn, nil
}
