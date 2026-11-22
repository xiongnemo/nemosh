package runtime

// bashTemporaryPrefix is whether an assignment in front of a special builtin is the builtin's
// alone, as bash has it outside its POSIX mode: seen while it runs -- by eval's text and a
// sourced file -- and gone after it, `x=1 :`, `x=1 export y=2`, `x=1 . file`, and bash's local,
// which is no special builtin there. Measured against Git for Windows' bash 5.3. POSIX keeps it,
// as busybox does and bash's POSIX mode, and as nemosh and sh do.
func (r Runtime) bashTemporaryPrefix() bool {
	return r.BashMode() && !r.options.posix
}

// specialErrorsEndScript is whether a special builtin's own error -- its redirection that fails
// -- ends a script that is not a session, as POSIX 2.8.1 and busybox have it, and bash only in
// its POSIX mode: outside it bash's builtin answers 1 and the script goes on, `: > /no/dir/f`,
// `exec 3> /no/dir/f` and eval's, `.`'s, export's and set's alike. Measured against Git for
// Windows' bash 5.3.
func (r Runtime) specialErrorsEndScript() bool {
	return !r.bashTemporaryPrefix()
}
