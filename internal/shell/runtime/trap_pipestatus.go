package runtime

// A trap leaves $PIPESTATUS as the command before it set it: bash saves the statuses before
// every trap and puts them back after (trap.c, save_pipestatus_array), so with a DEBUG trap
// `true | false; echo ${PIPESTATUS[*]}` says 0 1, where the trap's own commands had set it to
// theirs. busybox has no $PIPESTATUS.

// savePipeStatus is $PIPESTATUS as it stands, and the means to put it back.
func (r Runtime) savePipeStatus() func() {
	if r.arrays == nil {
		return func() {}
	}
	array, had := r.arrays.indexed["PIPESTATUS"]
	var saved *indexedArray
	if had {
		saved = array.clone()
	}
	return func() {
		if had {
			r.arrays.put("PIPESTATUS", saved)
			return
		}
		r.arrays.unset("PIPESTATUS")
	}
}
