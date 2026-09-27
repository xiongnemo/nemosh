package runtime

// ShellOptionIsOn is `[[ -o name ]]`, and `test -o name` and `[ -o name ]` through the
// process view: whether a `set -o` option is on. A name that is not one is off, as bash
// answers. busybox has no option test.
func (r Runtime) ShellOptionIsOn(name string) bool {
	for _, spec := range shellOptionSpecs {
		if spec.name == name {
			return *spec.field(r.options)
		}
	}
	return false
}
