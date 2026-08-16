// Package completions carries the bundled completion specs.
//
// Embedded rather than installed beside the executable, because Nemosh is one
// static binary with no runtime sidecars -- a directory that had to travel with
// nemosh.exe would end that, and a Scoop shim points at one file.
//
// go:embed cannot reach outside its own package directory, which is why this
// five-line file exists here rather than the specs living under internal/. The
// directory is also the one place outsiders are meant to contribute to, and
// internal/ says the opposite.
package completions

import "embed"

// Files is looked up by name -- `adb.toml` for `adb` -- which is the same rule
// the user's directory uses. One lookup rule, two sources, nothing scanned at
// startup.
//
//go:embed *.toml
var Files embed.FS
