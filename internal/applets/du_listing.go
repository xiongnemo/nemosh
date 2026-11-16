package applets

import "path/filepath"

// duChild is an entry of a directory du walks: its name, and, for a plain file -- no directory,
// no link -- what its directory's listing says of it, which is everything du asks of one: its
// size, whether it is compressed or sparse, and which file it is. Listed is false for whatever
// du has to ask itself, and on every platform but Windows, whose listing is the one that says.
type duChild struct {
	name       string
	listed     bool
	size       int64
	attributes uint32
	id         fileID
}

// unlistedChildren is a directory's entries in the order it lists them, none listed.
func unlistedChildren(native string) ([]duChild, error) {
	entries, err := readDirectoryInOrder(native)
	children := make([]duChild, len(entries))
	for index, entry := range entries {
		children[index].name = entry.Name()
	}
	return children, err
}

// children is walk for each entry of the directory at native, in the order it listed them.
func (r *duRun) children(native, shown string, entries []duChild) int64 {
	var sum int64
	for _, child := range entries {
		r.depth++
		path, name := filepath.Join(native, child.name), duSubpath(shown, child.name)
		if child.listed {
			sum += r.listedFile(child, path, name)
		} else {
			sum += r.walk(path, name)
		}
		r.depth--
	}
	return sum
}

// listedFile is walk for a plain file its directory listed, asking it nothing. A plain file is
// never on another volume than its directory, so -x has nothing to ask; it is no link, so -H
// and -L have nothing to follow. It is counted once, unless -l, and that is asked of every one,
// where walk asks it of a file with several links: the listing does not say how many a file
// has. Met once in a walk that follows no link, a file with one link is met once all the same,
// so the answer is busybox's. A listing that gives no file an index, as FAT's does, counts each.
func (r *duRun) listedFile(child duChild, native, shown string) int64 {
	if r.ctx.Err() != nil {
		return 0
	}
	if !r.countLinks && child.id.index != 0 {
		if r.seen[child.id] {
			return 0
		}
		r.seen[child.id] = true
	}
	sum := listedBlocks(native, child, r.bytes)
	if r.all && r.depth <= r.maxDepth {
		r.print(sum, shown)
	}
	return sum
}
