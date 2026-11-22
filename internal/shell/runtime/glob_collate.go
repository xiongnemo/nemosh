package runtime

import (
	"bytes"
	"slices"
	"strings"
)

// sortMatches puts a pattern's matches in the order bash mode's locale collates them, and by
// byte without one; see globCollation. A key Windows cannot give, for a locale it does not know,
// leaves the byte order, as bash keeps the C locale when it cannot change to the one asked for.
func (r Runtime) sortMatches(matches []string) {
	locale := r.globCollation()
	if locale == "" {
		slices.Sort(matches)
		return
	}
	type keyed struct {
		name string
		key  []byte
	}
	sorted := make([]keyed, len(matches))
	for index, name := range matches {
		if sorted[index] = (keyed{name, sortKey(locale, name)}); sorted[index].key == nil {
			slices.Sort(matches)
			return
		}
	}
	// Two names Windows weighs the same keep an order by byte, so a listing is the same each time.
	slices.SortFunc(sorted, func(a, b keyed) int {
		if order := bytes.Compare(a.key, b.key); order != 0 {
			return order
		}
		return strings.Compare(a.name, b.name)
	})
	for index := range sorted {
		matches[index] = sorted[index].name
	}
}

// globCollation is the locale bash mode sorts matches in, as Windows names one, `en-US` for
// en_US.UTF-8: LC_ALL's, else LC_COLLATE's, else LANG's, each a shell variable whether exported or
// not, as bash reads them. Empty for byte order: in nemosh and sh, as busybox sorts whatever the
// locale, and in bash mode with none of them set or the C locale, C.UTF-8 and POSIX among it.
// Git bash collates through Windows, which is why its en_US order is not glibc's. Measured
// 2026-11-22 with Git for Windows' bash 5.3.
func (r Runtime) globCollation() string {
	if !r.BashMode() {
		return ""
	}
	for _, name := range [...]string{"LC_ALL", "LC_COLLATE", "LANG"} {
		value := r.vars[name]
		if value == "" {
			continue
		}
		value, _, _ = strings.Cut(value, "@")
		if value, _, _ = strings.Cut(value, "."); value == "C" || value == "POSIX" {
			return ""
		}
		return strings.ReplaceAll(value, "_", "-")
	}
	return ""
}
