package runtime

import "io/fs"

// hiddenFromGlob reports an entry pathname expansion leaves out under busybox-w32's two
// Windows options. nohiddenglob leaves out any file with the Hidden attribute, and
// nohidsysglob one with both Hidden and System -- the pair Windows puts on desktop.ini and
// its kind, which a `*` otherwise hands to every command. Both are off by default, as in
// busybox, and elsewhere no file carries the attributes.
func (r Runtime) hiddenFromGlob(entry fs.DirEntry) bool {
	if r.options == nil || !r.options.noHiddenGlob && !r.options.noHidSysGlob {
		return false
	}
	hidden, system := fileAttributes(entry)
	return hidden && (r.options.noHiddenGlob || system)
}
