package runtime

// A prefix assignment's scope in bash mode. bash binds the assignments before eval, `.`, source
// and unset in a scope of their own for the builtin's run (variables.c's push_scope over the
// temporary environment), and a function call's in the call's own; local and unset then act on
// whichever binding of a name is innermost. A name's binding is in r.vars here and what it hid is
// in the scope that bound it, so a layer is a localScope marked tempenv on the chain, which saves
// what its names were and puts them back when the builtin is done. None is made outside bash
// mode, nor in its POSIX mode, where a special builtin's prefix stays, as it does in busybox and
// nemosh and sh. Measured against Git for Windows' bash 5.3; Oils' ble-unset and temp-binding.

// pushTempenv binds assignments in a layer over the running scopes, answering the runtime that
// runs in it and the layer for popTempenv; false for an assignment refused, which has said so.
func (r Runtime) pushTempenv(assignments []assignment) (Runtime, *localScope, bool) {
	layer := &localScope{outer: r.locals, tempenv: true}
	for _, assignment := range assignments {
		name := assignment.name
		if _, saved := layer.saved[name]; !saved {
			below := r.variableAsItIs(name)
			below.temporary = true
			layer.save(name, below)
			// The name is a string while the layer stands, an array of its name hidden.
			r.arrays.unset(name)
		}
		if status := r.assignVar(name, r.assignedValue(assignment)); status != 0 && !r.prefixRefused(name) {
			r.popTempenv(layer)
			return r, nil, false
		}
		// In the environment of what the builtin runs, as a prefix assignment is.
		r.env.Set(name, r.vars[name])
	}
	r.locals = layer
	return r, layer, true
}

// popTempenv takes the layer away once its builtin is done: each name it still binds is what it
// was again, one made local in it too, which bash's layer takes with it. A binding unset to what it
// hid has left the layer already.
func (r Runtime) popTempenv(layer *localScope) {
	for name, saved := range layer.saved {
		r.restoreVariable(name, saved)
	}
}

// callScope is the running call's own scope, under the layers its builtins have put over it; nil
// outside a function.
func (r Runtime) callScope() *localScope {
	for scope := r.locals; scope != nil; scope = scope.outer {
		if !scope.tempenv {
			return scope
		}
	}
	return nil
}

// binding is the innermost scope that binds name, what the name was beneath it, and whether that
// scope is the running call's: its own, or a layer over it.
func (r Runtime) binding(name string) (*localScope, savedVariable, bool) {
	current := true
	for scope := r.locals; scope != nil; scope = scope.outer {
		if saved, ok := scope.saved[name]; ok {
			return scope, saved, current
		}
		if !scope.tempenv {
			current = false
		}
	}
	return nil, savedVariable{}, false
}

// bashLocal is local over a binding of name, as bash's make_local_variable has it, answering
// whether that was all local had to do: a prefix binding of the running call's becomes its local
// where it stands, value and all; an outer call's starts a local of this call's with its value and
// export; and one already the call's own is left as it is.
func (r Runtime) bashLocal(call *localScope, name string) bool {
	scope, saved, current := r.binding(name)
	switch {
	case scope == nil:
		return false
	case saved.temporary && current:
		saved.temporary = false
		scope.saved[name] = saved
		return true
	case saved.temporary:
		value, set := r.vars[name]
		_, exported := r.env.LookupEnv(name)
		r.newLocal(call, name)
		if set {
			r.vars[name] = value
			if exported {
				r.env.Set(name, value)
			}
		}
		return true
	}
	return current
}
