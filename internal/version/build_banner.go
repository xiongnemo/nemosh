package version

import (
	"fmt"
	"strings"
)

// Who built this, where, and when.
//
// The shape is the Linux kernel's banner, which every Android device prints and
// which reads:
//
//	Linux version 5.10.101 (build@host) (clang 12.0.5) #1 SMP PREEMPT Mon Jan 1 …
//
// It answers the question a version number cannot: two binaries claiming the
// same commit can still differ, and when one of them misbehaves the first thing
// worth knowing is whether it came off CI or somebody's laptop.
//
// Empty unless the build set them. A `go build ./cmd/nemosh` with no ldflags
// says nothing rather than inventing a time, which keeps the default build
// reproducible -- a timestamp baked in by default would make two builds of the
// same source differ, and that is a property worth more than a line of output.
var (
	buildTime string
	buildUser string
	buildHost string
)

// BuildStamp is the `user@host` and time, or "" when this build did not record
// them.
func BuildStamp() string {
	who := buildWho()
	switch {
	case who == "" && buildTime == "":
		return ""
	case who == "":
		return buildTime
	case buildTime == "":
		return who
	}
	return who + ", " + buildTime
}

func buildWho() string {
	user := strings.TrimSpace(buildUser)
	host := strings.TrimSpace(buildHost)
	switch {
	case user == "" && host == "":
		return ""
	case host == "":
		return user
	case user == "":
		return host
	}
	return user + "@" + host
}

// DescribeBuild is the second line of `--version`, or "" when there is nothing
// to say. A separate line rather than a longer first one: the first line is what
// a script parses, and it should not grow a field that depends on how the binary
// was produced.
func DescribeBuild() string {
	stamp := BuildStamp()
	if stamp == "" {
		return ""
	}
	return fmt.Sprintf("built by %s", stamp)
}
