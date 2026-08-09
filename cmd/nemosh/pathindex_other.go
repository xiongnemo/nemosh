//go:build !windows

package main

// Elsewhere the executable bit decides, not the name, so a file with no suffix
// is exactly the usual case and is indexed.
const runsWithoutASuffix = true

// indexKey is the name itself: these filesystems distinguish case, and `Make`
// and `make` can both exist and be different programs.
func indexKey(name string) string {
	return name
}
