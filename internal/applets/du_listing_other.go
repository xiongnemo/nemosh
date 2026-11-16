//go:build !windows

package applets

// readDuDirectory is a directory's entries in the order it lists them, every one for walk to
// stat: the listing here says nothing a stat does not.
func readDuDirectory(native string) ([]duChild, error) {
	return unlistedChildren(native)
}

// listedBlocks is never asked here, nothing being listed.
func listedBlocks(_ string, child duChild, _ bool) int64 { return child.size }
