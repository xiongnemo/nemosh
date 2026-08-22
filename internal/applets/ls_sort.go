package applets

import (
	"os"
	"sort"
)

// sortLsEntries orders one directory's worth of entries.
//
// Before 2026-08-22 this was a bare name comparison and -t, -S and -r were all
// refused, which made `ls -ltr` -- about as well-worn a command as there is --
// fail on its options.
func sortLsEntries(items []lsEntry, options lsOptions) {
	sort.SliceStable(items, func(i, j int) bool {
		return lsEntryLess(items[i], items[j], options)
	})
}

// lsEntryLess answers for one pair.
//
// The name is the tie-break for every key, and it has to be: two files of the
// same size in the same second must come out in the same order on every run, or
// two listings of an unchanged directory would differ and a diff between them
// would mean nothing.
//
// -r reverses the tie-break along with the key, which is what makes `ls -tr` the
// exact reverse of `ls -t` rather than nearly it.
func lsEntryLess(left, right lsEntry, options lsOptions) bool {
	switch options.sortKey {
	case lsSortByTime:
		// Newest first, which is what -t means and the reason it is reached for.
		if !left.info.ModTime().Equal(right.info.ModTime()) {
			return lsMaybeReverse(left.info.ModTime().After(right.info.ModTime()), options)
		}
	case lsSortBySize:
		if left.info.Size() != right.info.Size() {
			return lsMaybeReverse(left.info.Size() > right.info.Size(), options)
		}
	}
	return lsMaybeReverse(left.name < right.name, options)
}

func lsMaybeReverse(less bool, options lsOptions) bool {
	if options.reverse {
		return !less
	}
	return less
}

// lsDisplayName is one entry's name as written: painted when colour is on, with
// the -F indicator after it.
//
// Both the short form and the long form go through here, because -F applies to
// every layout and two copies of "name plus indicator" is two answers eventually.
func lsDisplayName(entry lsEntry, options lsOptions) string {
	return paintLsName(entry.name, entry.info, options.colored) + classifyLsSuffix(entry.info, options)
}

// lsMeasuredName is the same text with no colour escapes, which is what the
// column grid has to measure: an escape is bytes that occupy no cells, and an
// indicator is one cell that must be counted. Getting this pair wrong shifts
// every column after the first coloured name.
func lsMeasuredName(entry lsEntry, options lsOptions) string {
	return entry.name + classifyLsSuffix(entry.info, options)
}

// classifyLsSuffix is the -F indicator: `/` for a directory, `@` for a symlink,
// `*` for an executable, nothing for anything else.
//
// It goes outside the colour escapes, which is where busybox puts it -- an
// indicator inside the reset would be coloured as though it were part of the
// name, and a script stripping the colour would keep it.
func classifyLsSuffix(info os.FileInfo, options lsOptions) string {
	if !options.classify || info == nil {
		return ""
	}
	switch {
	case info.IsDir():
		return "/"
	case info.Mode()&os.ModeSymlink != 0:
		return "@"
	case isExecutableEntry(info):
		// The same suffix list the shell uses for lookup, since Windows has no
		// execute bit to read. See isExecutableEntry in ls_color.go.
		return "*"
	}
	return ""
}
