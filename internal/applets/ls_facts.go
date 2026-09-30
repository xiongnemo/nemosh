package applets

import (
	"fmt"
)

// What the long form, -i, -s and the -c and -u times show of an entry beyond its FileInfo comes
// from the stat busybox-w32 makes of it, stat's hostStatRecord: a directory's links are two and
// one for each directory in it, as count_subdirs counts them, where Windows says 1; the inode is
// the file's index on its volume; the blocks are 512-byte ones, whole 4096-byte blocks of what
// the file holds; the change time is when the file was made. `total` added up each entry's size
// in kilobytes, so three one-byte files were `total 3` where busybox's st_blocks make them
// `total 12`, and every directory had one link. An entry the shell provides, or one that cannot
// be asked, keeps what its FileInfo says.

// lsNeedsFacts says whether the listing asks an entry for more than its FileInfo holds.
func lsNeedsFacts(options lsOptions) bool {
	return options.long || options.inode || options.blocks || options.timeKey != lsModifyTime
}

// recordLsFacts makes the stat of each entry the listing needs one of.
func recordLsFacts(items []lsEntry, options lsOptions) {
	if !lsNeedsFacts(options) {
		return
	}
	for index := range items {
		item := &items[index]
		if item.recorded || item.device {
			continue
		}
		record, err := hostStatRecord(item.path, item.followed, 0)
		item.record, item.recorded = record, err == nil
	}
}

func (e lsEntry) links() uint64 {
	if e.recorded {
		return e.record.links
	}
	if count, ok := fileLinkCount(e.path); ok {
		return uint64(count)
	}
	return 1
}

// ids are the owner's and the group's numbers, which -n shows: the current user's for an entry
// the shell provides, as busybox's synthetic stat gives a device the caller's.
func (e lsEntry) ids() (uint32, uint32) {
	if e.recorded {
		return e.record.uid, e.record.gid
	}
	id := uint32(currentUserID())
	return id, id
}

// blocks is the entry's 512-byte blocks.
func (e lsEntry) blocks() int64 {
	if e.recorded {
		return e.record.blocks
	}
	return (e.info.Size() + 511) / 512
}

// lsEntryPrefix is the columns -i and -s put before an entry, as display_single prints them: the
// inode in nineteen, and the blocks in kilobytes in six, or -h's size of them in seven.
func lsEntryPrefix(entry lsEntry, options lsOptions) string {
	prefix := ""
	if options.inode {
		prefix += fmt.Sprintf("%19d ", entry.record.inode)
	}
	if options.blocks {
		if options.human {
			prefix += fmt.Sprintf("%7s ", lsHumanSize(entry.blocks()*512, true))
		} else {
			prefix += fmt.Sprintf("%6d ", entry.blocks()/2)
		}
	}
	return prefix
}

// lsTotal is the `total` line that heads a directory's listing under -l or -s: its entries'
// blocks in kilobytes, rounded up once over the whole of them, as calculate_blocks adds them,
// and under -h that many kilobytes with no fraction, left in seven.
func lsTotal(items []lsEntry, options lsOptions) string {
	blocks := int64(1)
	for _, item := range items {
		blocks += item.blocks()
	}
	kilobytes := blocks / 2
	if options.human {
		return fmt.Sprintf("total %-7s", lsHumanSize(kilobytes*1024, false))
	}
	return fmt.Sprintf("total %d", kilobytes)
}

// lsSizeText is the size column's text: the bytes, or -h's size of them.
func lsSizeText(size int64, options lsOptions) string {
	if options.human {
		return lsHumanSize(size, true)
	}
	return fmt.Sprintf("%d", size)
}

// lsHumanSize is busybox's make_human_readable_str: under 1024 the number alone, and otherwise
// the number of the largest unit it reaches with one digit of fraction, rounded, `4.0K` and
// `1.5K` -- or with none, rounded up from half, where fraction is false. `4K` was printed for
// 4096, as GNU's -h prints it.
func lsHumanSize(size int64, fraction bool) string {
	if size == 0 {
		return "0"
	}
	value, tenth, unit := uint64(size), uint64(0), ""
	for _, next := range []string{"K", "M", "G", "T", "P", "E"} {
		if value < 1024 {
			break
		}
		tenth = (value%1024*10 + 512) / 1024
		value, unit = value/1024, next
	}
	if tenth >= 10 {
		value, tenth = value+1, 0
	}
	switch {
	case unit == "":
		return fmt.Sprintf("%d", value)
	case !fraction:
		if tenth >= 5 {
			value++
		}
		return fmt.Sprintf("%d%s", value, unit)
	}
	return fmt.Sprintf("%d.%d%s", value, tenth, unit)
}
