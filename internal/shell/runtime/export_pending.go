package runtime

import "sort"

// A name can be marked for export before it has a value -- `export X` with X unset, or
// `declare -x X` -- and it reaches the environment when it gets one. It went there at once,
// empty, so a child saw `X=` where both references give it nothing, and `export -p` said
// `export X=''` where busybox says `export X`.

// markExported is `export NAME` with no value: into the environment now if NAME has one,
// and otherwise when it is assigned.
func (r Runtime) markExported(name string) {
	if value, set := r.vars[name]; set {
		r.env.Set(name, value)
		return
	}
	if r.attributes == nil {
		return
	}
	attributes := r.attributes[name]
	attributes.exported = true
	r.attributes[name] = attributes
}

// isExported reports a name whose assignments go to the environment.
func (r Runtime) isExported(name string) bool {
	if _, in := r.env.LookupEnv(name); in {
		return true
	}
	return r.attributes[name].exported
}

// unexport is `declare +x`: out of the environment, and no longer waiting to go in.
func (r Runtime) unexport(name string) {
	r.env.Unset(name)
	if attributes, ok := r.attributes[name]; ok {
		attributes.exported = false
		r.attributes[name] = attributes
	}
}

// pendingExports are the names marked for export that have no value yet, for `export -p`.
func (r Runtime) pendingExports() []string {
	var names []string
	for name, attributes := range r.attributes {
		if _, in := r.env.LookupEnv(name); attributes.exported && !in {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// unsetName removes a whole name: its value, its array, its place in the environment and
// its attributes. The attributes stayed, so after `declare -i n; unset n` the next `n=2+3`
// was still evaluated -- 5, where bash gives 2+3.
func (r Runtime) unsetName(name string) {
	delete(r.vars, name)
	r.arrays.unset(name)
	r.env.Unset(name)
	delete(r.attributes, name)
	r.markVarMutation(name)
}
