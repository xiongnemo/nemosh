package applets

import "fmt"

// Assignment in bc, and the three variables that are also settings.
//
// Split from bc_eval.go for the 250-line ceiling AGENTS.md sets. The seam is between reading
// a value and writing one, which is also where the only surprise lives: **`scale`, `ibase`
// and `obase` are ordinary variables**, so assigning to them is what changes the arithmetic
// that follows.

func (in *bcInterp) evalAssign(node bcAssignExpr) (bigDecimal, error) {
	value, err := in.eval(node.value)
	if err != nil {
		return bigDecimal{}, err
	}
	if node.operator != "=" {
		current, err := in.eval(node.target)
		if err != nil {
			return bigDecimal{}, err
		}
		value, err = applyDecimal(node.operator[0], current, value, in.scale())
		if err != nil {
			return bigDecimal{}, err
		}
	}
	return value, in.store(node.target, value)
}

func (in *bcInterp) store(target bcExpr, value bigDecimal) error {
	switch node := target.(type) {
	case bcNameExpr:
		if err := bcCheckSetting(node.name, value); err != nil {
			return err
		}
		in.variables[node.name] = value
		return nil
	case bcIndexExpr:
		index, err := in.arrayIndex(node.index)
		if err != nil {
			return err
		}
		if in.arrays[node.name] == nil {
			in.arrays[node.name] = map[int64]bigDecimal{}
		}
		in.arrays[node.name][index] = value
		return nil
	}
	return fmt.Errorf("cannot assign to this")
}

// bcCheckSetting refuses a value the special variables cannot take.
//
// Loud rather than clamped: a program that sets `ibase=1` has a mistake in it, and reading
// every number afterwards as zero would hide it.
func bcCheckSetting(name string, value bigDecimal) error {
	number := value.rescale(0).integer()
	switch name {
	case "ibase", "obase":
		if !number.IsInt64() || number.Int64() < 2 || number.Int64() > 16 {
			return fmt.Errorf("%s must be between 2 and 16", name)
		}
	case "scale":
		if value.sign() < 0 {
			return fmt.Errorf("scale must not be negative")
		}
		if !number.IsInt64() || number.Int64() > 100000 {
			return fmt.Errorf("scale is too large")
		}
	}
	return nil
}

func (in *bcInterp) evalIncDec(node bcIncDecExpr) (bigDecimal, error) {
	current, err := in.eval(node.target)
	if err != nil {
		return bigDecimal{}, err
	}
	step := decimalFromInt(1)
	updated := addDecimal(current, step)
	if node.operator == "--" {
		updated = subDecimal(current, step)
	}
	if err := in.store(node.target, updated); err != nil {
		return bigDecimal{}, err
	}
	if node.prefix {
		return updated, nil
	}
	return current, nil
}

func (in *bcInterp) evalBuiltin(node bcBuiltinExpr) (bigDecimal, error) {
	if node.name == "read" {
		// Reading a line would make bc's own input and the program's input the same
		// stream, which is a source of surprises rather than a feature here.
		return bigDecimal{}, fmt.Errorf("read() is not supported")
	}
	if len(node.args) != 1 {
		return bigDecimal{}, fmt.Errorf("%s takes one argument", node.name)
	}
	if name, ok := node.args[0].(bcIndexExpr); ok && name.index == nil {
		// `length(a[])` is the number of elements, which is a GNU extension both
		// references carry.
		return decimalFromInt(int64(len(in.arrays[name.name]))), nil
	}
	value, err := in.eval(node.args[0])
	if err != nil {
		return bigDecimal{}, err
	}
	switch node.name {
	case "length":
		return decimalFromInt(int64(digitCount(value))), nil
	case "scale":
		return decimalFromInt(int64(value.scale)), nil
	case "sqrt":
		return sqrtDecimal(value, in.scale())
	}
	return bigDecimal{}, fmt.Errorf("unknown function %s", node.name)
}
