package runtime

// inheritLocal is `shopt -s localvar_inherit`: a new local starts with the value, the array
// and the attributes the name had where the function was called, as bash's does, rather than
// with nothing, which is what both references start it with otherwise. The array is copied,
// so the call cannot reach the caller's through it, and the caller's comes back on return as
// it always does.
func (r Runtime) inheritLocal(name string, saved savedVariable) {
	if saved.indexed {
		r.arrays.put(name, saved.array.clone())
	}
	if saved.keys != nil {
		r.arrays.declareAssociative(name)
		for _, key := range saved.keys.order {
			r.arrays.setKey(name, key, saved.keys.entries[key])
		}
	}
	if saved.attributes != (variableAttributes{}) {
		r.attributes[name] = saved.attributes
	}
	if saved.set {
		r.vars[name] = saved.value
		if r.isExported(name) {
			r.env.Set(name, saved.value)
		}
	}
}
