package runtime

// localScope remembers what each name was before a function call made it its own. One per
// call; a nested call gets its own, so restoring unwinds in the order the calls did.
type localScope struct {
	saved map[string]savedVariable
	// options are the `set` options as they stood at `local -`, and target is where
	// they go back to when the call returns. nil when the call did not ask.
	options []bool
	target  *shellOptions
}

// savedVariable is all of a name before a call shadowed it: its value, an array of either
// kind, its attributes, and whether it was read-only or in the environment. Only the value
// was kept, so a local array wrote into the caller's -- `local a; a[0]=x` changed the
// global `a` -- and a local's attributes and export outlived the call.
type savedVariable struct {
	value, env string
	set, inEnv bool
	elements   []string
	indexed    bool
	indices    map[int]bool
	keys       *associativeArray
	attributes variableAttributes
	readonly   bool
}

func newLocalScope() *localScope {
	return &localScope{saved: map[string]savedVariable{}}
}

// makeLocal gives the call a name of its own. The first time, what the name was is saved
// and the name is cleared: no value, no array, no attributes. The export flag stays, the
// rule in both references, so the local reaches a child once it has a value -- and until
// then nothing does. A name the call has already made local is left as it is, as both
// references leave it: `local x=1; local x` keeps 1.
func (r Runtime) makeLocal(name string) {
	if _, already := r.locals.saved[name]; already {
		return
	}
	saved := savedVariable{attributes: r.attributes[name]}
	saved.value, saved.set = r.vars[name]
	saved.env, saved.inEnv = r.env.LookupEnv(name)
	saved.elements, saved.indexed = r.arrays.values[name]
	saved.indices, saved.keys = r.arrays.present[name], r.arrays.associative[name]
	_, saved.readonly = r.readonly[name]
	r.locals.saved[name] = saved
	exported := r.isExported(name)
	delete(r.vars, name)
	r.arrays.unset(name)
	r.env.Unset(name)
	delete(r.attributes, name)
	if exported {
		r.markExported(name)
	}
	r.markVarMutation(name)
}

// restore puts back every name the call made local, and the options if it asked.
func (s *localScope) restore(r Runtime) {
	for name, saved := range s.saved {
		r.restoreVariable(name, saved)
	}
	if s.target != nil {
		for index, spec := range shellOptionSpecs {
			*spec.field(s.target) = s.options[index]
		}
	}
}

func (r Runtime) restoreVariable(name string, saved savedVariable) {
	delete(r.vars, name)
	if saved.set {
		r.vars[name] = saved.value
	}
	r.env.Unset(name)
	if saved.inEnv {
		r.env.Set(name, saved.env)
	}
	r.arrays.unset(name)
	if saved.indexed {
		r.arrays.values[name] = saved.elements
		if saved.indices != nil {
			r.arrays.present[name] = saved.indices
		}
	}
	if saved.keys != nil {
		if r.arrays.associative == nil {
			r.arrays.associative = map[string]*associativeArray{}
		}
		r.arrays.associative[name] = saved.keys
	}
	delete(r.attributes, name)
	if saved.attributes != (variableAttributes{}) {
		r.attributes[name] = saved.attributes
	}
	delete(r.readonly, name)
	if saved.readonly {
		r.readonly[name] = struct{}{}
	}
	r.markVarMutation(name)
}

// saveOptions is `local -`: the `set` options -- `-e`, `-u`, pipefail and the rest --
// belong to the call from here on, so a function can `set -e` for its own body without
// leaving it set for its caller. busybox has it and so does bash; here it was `bad
// variable name`. Only the options `set` reaches, as in both: a `shopt` setting is not
// saved. Asked twice, the first answer stands, as it does for a variable.
func (s *localScope) saveOptions(options *shellOptions) {
	if s.target != nil {
		return
	}
	s.target = options
	for _, spec := range shellOptionSpecs {
		s.options = append(s.options, *spec.field(options))
	}
}
