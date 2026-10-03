package runtime

// A temporary assignment to an array's name, `A=x cmd`, is a string for the command -- in its
// environment too -- and the array is as it was after, as bash has it. The string went into the
// array's first element, for good, and the command's environment had no A at all: pip's
// completion, `COMP_WORDS="${COMP_WORDS[*]}" ... pip`, never saw its words.

// shadowedArray is an array a temporary assignment took out of the store while its command
// runs, to put back after.
type shadowedArray struct {
	name        string
	indexed     *indexedArray
	associative *associativeArray
}

// shadow takes the array of this name out of the store, for restore to put back.
func (a *shellArrays) shadow(name string) shadowedArray {
	saved := shadowedArray{name: name, indexed: a.indexed[name], associative: a.associative[name]}
	a.unset(name)
	return saved
}

// restore puts a shadowed array back as it was, over whatever the command left under its name.
func (a *shellArrays) restore(saved shadowedArray) {
	a.unset(saved.name)
	if saved.indexed != nil {
		a.put(saved.name, saved.indexed)
	}
	if saved.associative != nil {
		if a.associative == nil {
			a.associative = map[string]*associativeArray{}
		}
		a.associative[saved.name] = saved.associative
	}
}

// shadowArray takes an array out of the store for a command whose temporary assignment names
// it, remembering it on the command's runtime; restoreShadowed puts it back.
func (r *Runtime) shadowArray(name string) {
	if r.arrays != nil && (r.arrays.has(name) || r.arrays.isAssociative(name)) {
		r.shadowed = append(r.shadowed, r.arrays.shadow(name))
	}
}

// restoreShadowed puts back what a command's temporary assignments took out of the store.
func (r Runtime) restoreShadowed(commandRuntime Runtime) {
	for _, saved := range commandRuntime.shadowed {
		r.arrays.restore(saved)
	}
}
