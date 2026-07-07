package appletmanifest

import (
	"regexp"
	"sort"
	"strings"
)

var (
	busyBoxAppletPattern = regexp.MustCompile(`\bAPPLET(?:_ODDNAME|_NOEXEC|_NOFORK|_SCRIPTED)?\(([^,)]*)`)
	nemoshAppletPattern  = regexp.MustCompile(`\bnew([A-Za-z0-9]+)Applet\((?:"([^"]*)")?`)
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

func ParseNemoshRegistry(source string) []string {
	applets := make([]string, 0)
	matches := nemoshAppletPattern.FindAllStringSubmatch(source, -1)
	for _, match := range matches {
		if match[2] != "" {
			applets = append(applets, match[2])
			continue
		}
		applets = append(applets, strings.ToLower(match[1]))
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
