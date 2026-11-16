package appletmanifest

import (
	"regexp"
	"sort"
	"strings"
)

var (
	busyBoxAppletPattern = regexp.MustCompile(`\bAPPLET(?:_ODDNAME|_NOEXEC|_NOFORK|_SCRIPTED)?\(([^,)]*)`)
	// A row of internal/applets' defaultApplets: `{"cat", newCatApplet},`.
	nemoshAppletPattern = regexp.MustCompile(`(?m)^\s*\{"([^"]+)",`)
)

type AppletComparison struct {
	Implemented []string
	Missing     []string
	NemoshOnly  []string
}

func ParseBusyBoxApplets(source string) []string {
	applets := make([]string, 0)
	for line := range strings.SplitSeq(source, "\n") {
		if !strings.Contains(line, "//applet:") {
			continue
		}

		match := busyBoxAppletPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		applets = append(applets, strings.TrimSpace(match[1]))
	}
	return applets
}

// ParseNemoshRegistry is the names in the source of internal/applets' table, in its order.
func ParseNemoshRegistry(source string) []string {
	applets := make([]string, 0)
	for _, match := range nemoshAppletPattern.FindAllStringSubmatch(source, -1) {
		applets = append(applets, match[1])
	}
	return applets
}

func CompareApplets(busyboxNames []string, nemoshNames []string) AppletComparison {
	busyboxSet := stringSet(busyboxNames)
	nemoshSet := stringSet(nemoshNames)

	comparison := AppletComparison{
		Implemented: make([]string, 0),
		Missing:     make([]string, 0),
		NemoshOnly:  make([]string, 0),
	}

	for name := range busyboxSet {
		if nemoshSet[name] {
			comparison.Implemented = append(comparison.Implemented, name)
			continue
		}
		comparison.Missing = append(comparison.Missing, name)
	}

	for name := range nemoshSet {
		if !busyboxSet[name] {
			comparison.NemoshOnly = append(comparison.NemoshOnly, name)
		}
	}

	sort.Strings(comparison.Implemented)
	sort.Strings(comparison.Missing)
	sort.Strings(comparison.NemoshOnly)
	return comparison
}

func stringSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}
