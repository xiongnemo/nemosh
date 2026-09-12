package applets

import "fmt"

// Calling a bc function.
//
// Split from bc_exec.go for the 250-line ceiling. **Parameters and autos are both local**,
// and both are restored when the call returns -- which is the whole of what makes recursion
// work, since the outer call's `i` has to survive the inner one's.

// bcSaved is one name's value while a call has it shadowed.
type bcSaved struct {
	value bigDecimal
	array map[int64]bigDecimal
}

// evalCall runs a user-defined function.
//
// **Parameters and autos are both local**, and both are restored when the call returns --
// which is the whole of what makes recursion work. An array argument is passed **by value**:
// POSIX says so, and a caller whose array quietly changed underneath it would be a worse
// surprise than the copy is a cost.
func (in *bcInterp) evalCall(node bcCallExpr) (bigDecimal, error) {
	function, defined := in.functions[node.name]
	if !defined {
		return bigDecimal{}, fmt.Errorf("function %s is not defined", node.name)
	}
	if len(node.args) != len(function.params) {
		return bigDecimal{}, fmt.Errorf("%s takes %d arguments, not %d",
			node.name, len(function.params), len(node.args))
	}
	if in.depth > 500 {
		// A limit at all, so a runaway recursion is a diagnostic rather than a Go stack
		// overflow, which cannot be recovered from.
		return bigDecimal{}, fmt.Errorf("%s recursed more than 500 deep", node.name)
	}
	// The arguments are evaluated in the *caller's* scope, before anything is shadowed.
	values, arrays, err := in.evalArguments(function, node.args)
	if err != nil {
		return bigDecimal{}, err
	}
	saved := in.enterCall(function, values, arrays)
	in.depth++
	flow, err := in.execBody(function.body)
	in.depth--
	in.leaveCall(saved)
	if err != nil {
		return bigDecimal{}, err
	}
	if flow == bcFlowHalt {
		in.halted = true
	}
	// Falling off the end answers zero, as a bare `return` does.
	answer := in.returned
	in.returned = decimalFromInt(0)
	return answer, nil
}

func (in *bcInterp) evalArguments(function *bcFunction, args []bcExpr) ([]bigDecimal, []map[int64]bigDecimal, error) {
	values := make([]bigDecimal, len(args))
	arrays := make([]map[int64]bigDecimal, len(args))
	for index, argument := range args {
		if function.params[index].isArray {
			name, ok := argument.(bcIndexExpr)
			if !ok || name.index != nil {
				return nil, nil, fmt.Errorf("%s wants an array as argument %d, written name[]",
					function.name, index+1)
			}
			copied := map[int64]bigDecimal{}
			for key, value := range in.arrays[name.name] {
				copied[key] = value
			}
			arrays[index] = copied
			continue
		}
		value, err := in.eval(argument)
		if err != nil {
			return nil, nil, err
		}
		values[index] = value
	}
	return values, arrays, nil
}

func (in *bcInterp) enterCall(function *bcFunction, values []bigDecimal, arrays []map[int64]bigDecimal) map[string]bcSaved {
	saved := map[string]bcSaved{}
	remember := func(parameter bcParameter) {
		if _, already := saved[parameter.name]; already {
			return
		}
		saved[parameter.name] = bcSaved{
			value: in.variables[parameter.name],
			array: in.arrays[parameter.name],
		}
	}
	for index, parameter := range function.params {
		remember(parameter)
		if parameter.isArray {
			in.arrays[parameter.name] = arrays[index]
			continue
		}
		in.variables[parameter.name] = values[index]
	}
	for _, parameter := range function.autos {
		remember(parameter)
		// An auto starts at zero, or as an empty array, whichever it was declared as.
		if parameter.isArray {
			in.arrays[parameter.name] = map[int64]bigDecimal{}
			continue
		}
		in.variables[parameter.name] = decimalFromInt(0)
	}
	return saved
}

func (in *bcInterp) leaveCall(saved map[string]bcSaved) {
	for name, previous := range saved {
		in.variables[name] = previous.value
		if previous.array == nil {
			delete(in.arrays, name)
			continue
		}
		in.arrays[name] = previous.array
	}
}

// report writes a diagnostic, flushing first so the two arrive in order.
func (in *bcInterp) report(err error) {
	in.out.Flush()
	fmt.Fprintf(in.errors, "bc: %v\n", err)
}
