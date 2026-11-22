package runtime

// dynamicUnset is unset of a variable a caller made local, as bash does it with localvar_unset
// off, its default: the local is taken away, the value it hid is the variable's again, and the
// caller's return puts nothing back, so an assignment after the unset is to what was hidden. Unset
// in the function that made the variable local leaves it unset there, as before, and so does every
// unset with the option on -- busybox's way, which nemosh and sh start with and bash mode starts
// without (EnterBashMode). It answers whether it took a local away. Measured against Git for
// Windows' bash 5.3: with x local to f and to g, f calling g and g calling c, c's `unset x` shows
// f's x, and a second unset the global.
//
// A prefix assignment's binding is taken away wherever it is, the running call's too, as bash's
// unset takes its innermost cell: `v=t f` with f's `unset v` shows the v that t hid, and so does an
// unset in `v=t eval`; see tempenv_scope.go.
func (r Runtime) dynamicUnset(name string) bool {
	if r.options.localVarUnset || r.locals == nil {
		return false
	}
	scope, saved, current := r.binding(name)
	if scope == nil || current && !saved.temporary {
		return false
	}
	delete(scope.saved, name)
	r.restoreVariable(name, saved)
	return true
}
