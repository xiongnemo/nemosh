package applets

// Scope: the globals, and one call's locals.
//
// awk has no declaration, and **a function's parameters are its only locals**. A function
// that wants a temporary takes an extra parameter the caller never passes, which is why
// `function f(a,    i)` is written with that gap -- the spacing is a convention for saying
// "the rest are mine", not syntax.
//
// A parameter shadows the global of the same name for the whole call **whether or not it is
// ever assigned**. Reading an untouched one answers the uninitialised value rather than the
// global, and that is exactly what makes the extra-parameter idiom safe: `f(a,  i)` must not
// see, or damage, a global `i` the caller was using as a loop counter.

// awkFrame is one call's locals.
type awkFrame struct {
	// names is every parameter the function declared, so a name shadows its global from
	// the moment the call starts rather than from its first assignment.
	names  map[string]bool
	values map[string]awkValue
	// arrays holds the parameters used as arrays. One passed by name shares the caller's
	// object, which is what makes arrays by reference and scalars by value.
	arrays map[string]*awkArray
}

// frame answers the innermost call's locals, or nil at the top level.
func (in *awkInterp) frame() *awkFrame {
	if len(in.frames) == 0 {
		return nil
	}
	return in.frames[len(in.frames)-1]
}

// isLocal reports whether a name belongs to the running call rather than to the globals.
func (in *awkInterp) isLocal(name string) bool {
	frame := in.frame()
	return frame != nil && frame.names[name]
}

// getVar reads a variable, settling NF first if the fields have moved.
func (in *awkInterp) getVar(name string) awkValue {
	if name == "NF" {
		in.ensureFields()
		return awkNum(float64(len(in.fields)))
	}
	if in.isLocal(name) {
		return in.frame().values[name]
	}
	return in.vars[name]
}

// setVar writes a variable, giving the ones that mean something to the record loop their
// side effects.
func (in *awkInterp) setVar(name string, value awkValue) {
	if name == "NF" {
		in.setFieldCount(int(value.num()))
		return
	}
	if in.isLocal(name) {
		in.frame().values[name] = value
		return
	}
	in.vars[name] = value
}

// getArray answers a name's array, making it if this is the first mention.
//
// awk has no declaration, so `a[1]=1` on an unseen name creates the array. That is why this
// never fails.
func (in *awkInterp) getArray(name string) *awkArray {
	if in.isLocal(name) {
		frame := in.frame()
		array, present := frame.arrays[name]
		if !present {
			// A local used as an array for the first time and not passed one: it is the
			// call's own, and goes when the call does.
			array = newAwkArray()
			frame.arrays[name] = array
		}
		return array
	}
	array, present := in.arrays[name]
	if !present {
		array = newAwkArray()
		in.arrays[name] = array
	}
	return array
}

// lookupArray answers an array without creating one, which is what `in` and `length(a)` ask.
func (in *awkInterp) lookupArray(name string) (*awkArray, bool) {
	if in.isLocal(name) {
		array, present := in.frame().arrays[name]
		return array, present
	}
	array, present := in.arrays[name]
	return array, present
}
