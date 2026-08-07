// Package version reports what this binary was built from.
//
// The format and the rules are AGENTS.md's, in the Versioning section:
//
//	v{major}.{minor}.{patch}-{branch}-{commit12}[-dirty]
//
// The patch number is the nearest exact semver tag's patch plus the commits
// since that tag, so it advances by itself and is never hand-maintained. A
// prerelease tag is not a base; the fallback base is v0.0.1.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
)

// Injected with -ldflags -X at release time. Absent for `go run` and a plain
// `go build`, which is why Current falls back to debug.ReadBuildInfo.
var (
	tag             string
	commitsSinceTag string
	branch          string
	commit          string
	dirty           string
)

const (
	fallbackBase = "v0.0.1"
	unknown      = "unknown"
	commitLength = 12
)

var (
	runtimeGOOS   = runtime.GOOS
	runtimeGOARCH = runtime.GOARCH
)

// Build is what the version string is computed from. It is the raw material,
// not the answer, so the derivation stays testable without a git checkout.
type Build struct {
	Tag             string
	CommitsSinceTag int
	Branch          string
	Commit          string
	Dirty           bool
}

// String renders the documented format. It never returns empty: a version
// string is what a bug report quotes, so an unknown field becomes a placeholder
// rather than a gap.
func (b Build) String() string {
	base := b.Tag
	if !isExactSemverTag(base) {
		base = fallbackBase
	}
	version := advancePatch(base, b.CommitsSinceTag)

	name := sanitizeBranch(b.Branch)
	if name == "" {
		name = unknown
	}
	revision := shortCommit(b.Commit)
	if revision == "" {
		revision = unknown
	}

	rendered := version + "-" + name + "-" + revision
	if b.Dirty {
		rendered += "-dirty"
	}
	return rendered
}

// Current reads the injected values, then fills what it can from the build
// info the toolchain stamps in. Both paths are best-effort by design; neither
// requires a terminal or a git binary.
func Current() Build {
	build := Build{
		Tag:             tag,
		CommitsSinceTag: atoi(commitsSinceTag),
		Branch:          branch,
		Commit:          commit,
		Dirty:           dirty == "true",
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return build
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if build.Commit == "" {
				build.Commit = setting.Value
			}
		case "vcs.modified":
			if dirty == "" {
				build.Dirty = setting.Value == "true"
			}
		}
	}
	return build
}

// Describe is the one line `nemosh --version` prints.
func Describe() string {
	return fmt.Sprintf("nemosh %s (%s, %s/%s)", Current(), runtime.Version(), runtimeGOOS, runtimeGOARCH)
}

// isExactSemverTag accepts only `vMAJOR.MINOR.PATCH`. A prerelease or dev tag
// must not become the base for a patch calculation, or every dev build would
// rebase onto the previous dev build.
func isExactSemverTag(candidate string) bool {
	rest, found := strings.CutPrefix(candidate, "v")
	if !found {
		return false
	}
	parts := strings.Split(rest, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		if _, err := strconv.ParseUint(part, 10, 64); err != nil {
			return false
		}
	}
	return true
}

// advancePatch adds the commit count to the tag's own patch number, so the tag
// itself renders unchanged and the commit after it is patch + 1.
func advancePatch(base string, commits int) string {
	parts := strings.Split(strings.TrimPrefix(base, "v"), ".")
	patch, err := strconv.Atoi(parts[2])
	if err != nil || commits < 0 {
		return base
	}
	return fmt.Sprintf("v%s.%s.%d", parts[0], parts[1], patch+commits)
}

// sanitizeBranch replaces what a version string or an artifact name cannot
// carry. Runs are deliberately not collapsed, so the mapping stays legible.
func sanitizeBranch(name string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ' ', '\t', ':', '*', '?', '"', '<', '>', '|', '~', '^':
			return '-'
		}
		if r < 0x20 || r == 0x7f {
			return '-'
		}
		return r
	}, name)
}

// shortCommit takes twelve characters, and leaves a shorter one alone rather
// than padding it into something that looks like a real hash.
func shortCommit(full string) string {
	if len(full) <= commitLength {
		return full
	}
	return full[:commitLength]
}

func atoi(text string) int {
	parsed, err := strconv.Atoi(text)
	if err != nil {
		return 0
	}
	return parsed
}
