package oilsspec

import (
	"fmt"
	"sort"
	"strings"
)

// Matrix renders docs/testing/oils-spec.md from the data alone: the vendored cases, what
// the baseline's platform leaves out, the references' calibration, and nemosh's baseline.
// Nothing is run, so a test renders it again and fails when the committed page differs,
// and the page cannot go stale.
func Matrix(suite Suite, record Upstream, calibration Calibration, baseline Baseline) string {
	bash, busybox := calibration.Shells["bash"], calibration.Shells["busybox"]
	files := make([]string, 0, len(suite.Specs))
	for file := range suite.Specs {
		files = append(files, file)
	}
	sort.Strings(files)
	type row struct{ measured, bash, nemosh, ashOnly, busybox int }
	var total row
	var rows []string
	for _, file := range files {
		var r row
		for _, c := range suite.Specs[file].Cases {
			r.measured++
			if busybox.Passes(file, c.ID) {
				r.busybox++
			}
			if !bash.Passes(file, c.ID) {
				continue
			}
			r.bash++
			switch baseline.Standing(file, c.ID) {
			case Passes:
				r.nemosh++
			case AshOnly:
				r.ashOnly++
			}
		}
		total.measured += r.measured
		total.bash += r.bash
		total.nemosh += r.nemosh
		total.ashOnly += r.ashOnly
		total.busybox += r.busybox
		rows = append(rows, fmt.Sprintf("| %s | %d | %d | %d | %s | %d | %d |", file, r.measured, r.bash, r.nemosh, percent(r.nemosh, r.bash), r.ashOnly, r.busybox))
	}
	var left []string
	leftTotal := 0
	for rule, count := range suite.Left {
		left = append(left, fmt.Sprintf("- %s: %d", rule, count))
		leftTotal += count
	}
	sort.Strings(left)
	platform := platformNames[baseline.Platform]
	vendored := 0
	for _, file := range record.Files {
		vendored += file.Cases
	}

	var page strings.Builder
	fmt.Fprintf(&page, "# The Oils spec suite: how far nemosh is from bash\n\n")
	fmt.Fprintf(&page, "This page is written from `tests/oils/baseline.json` and `tests/oils/calibration.json`\n")
	fmt.Fprintf(&page, "by `NEMOSH_OILS=update`, and a test fails when it is out of date. What the suite is, and\n")
	fmt.Fprintf(&page, "why none of its cases says what nemosh must do, is in `tests/oils/README.md`.\n\n")
	fmt.Fprintf(&page, "Measured on %s, with the cases of Oils `%.7s` (%s), against\n\n", platform, record.Commit, record.Committed)
	fmt.Fprintf(&page, "- bash: %s\n- busybox: %s\n\n", bash.Version, busybox.Version)
	fmt.Fprintf(&page, "**nemosh passes %d of the %d cases bash passes: %s.**\n\n", total.nemosh, total.bash, percent(total.nemosh, total.bash))
	fmt.Fprintf(&page, "Of the other %d, it does %d the way the files record of ash, which may be busybox's\n", total.bash-total.nemosh, total.ashOnly)
	fmt.Fprintf(&page, "way, and %d neither way.\n\n", total.bash-total.nemosh-total.ashOnly)
	fmt.Fprintf(&page, "| | cases |\n|---|---:|\n")
	fmt.Fprintf(&page, "| in the vendored files | %d |\n", vendored)
	fmt.Fprintf(&page, "| in files Oils itself does not run | %d |\n", suite.Disabled)
	fmt.Fprintf(&page, "| left out on %s | %d |\n", platform, leftTotal)
	fmt.Fprintf(&page, "| measured | %d |\n", total.measured)
	fmt.Fprintf(&page, "| that bash passes | %d |\n", total.bash)
	fmt.Fprintf(&page, "| that nemosh passes of those | %d |\n\n", total.nemosh)
	fmt.Fprintf(&page, "Left out on %s, as cases no shell can be measured on there, for the reasons\n", platform)
	fmt.Fprintf(&page, "`tests/oils/exclusions.json` gives:\n\n%s\n\n", strings.Join(left, "\n"))
	fmt.Fprintf(&page, "## File by file\n\n")
	fmt.Fprintf(&page, "Of each file's measured cases: how many bash passes; how many of those nemosh passes,\n")
	fmt.Fprintf(&page, "and does the way ash does instead; and how many busybox does the way the file records\n")
	fmt.Fprintf(&page, "of ash.\n\n")
	fmt.Fprintf(&page, "| file | measured | bash | nemosh | %% | as ash | busybox |\n|---|---:|---:|---:|---:|---:|---:|\n")
	fmt.Fprintf(&page, "%s\n", strings.Join(rows, "\n"))
	fmt.Fprintf(&page, "| all | %d | %d | %d | %s | %d | %d |\n", total.measured, total.bash, total.nemosh, percent(total.nemosh, total.bash), total.ashOnly, total.busybox)
	return page.String()
}

// platformNames are how the page names the platforms runtime.GOOS names.
var platformNames = map[string]string{"windows": "Windows", "linux": "Linux", "darwin": "macOS"}

func percent(part, whole int) string {
	if whole == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(part)/float64(whole))
}
