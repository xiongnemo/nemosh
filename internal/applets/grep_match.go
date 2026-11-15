package applets

import (
	"fmt"
	"regexp"
)

// What grep prints of a matching line, and of a file: -o's pieces, -w's words, and the answer -l,
// -L and -c give instead of lines. Out of grep.go for the size ceiling.

// grepEmitMatch writes a matching line, whole or in pieces under -o.
func grepEmitMatch(printer *grepPrinter, expr *regexp.Regexp, flags grepFlags, name string, lineNumber int, line string, withNames bool) error {
	if !flags.onlyMatching {
		return printer.emit(name, lineNumber, line, true, withNames)
	}
	// Each match on its own line, which is what makes -o useful for pulling
	// values out.
	//
	// With -v there is nothing to print: the line was selected for *not*
	// matching, so it has no matched text. Measured -- GNU prints nothing and
	// exits 0, where falling through to the whole line would have printed it.
	if flags.invert {
		return nil
	}
	matches := expr.FindAllString(line, -1)
	if flags.wordMatch {
		matches = grepWords(expr, line)
	}
	for _, found := range matches {
		// An empty match is not printed, as busybox's grep does not print one: `grep -o
		// '[0-9]*'` is the numbers, where it was an empty line before each.
		if found == "" {
			continue
		}
		if err := printer.emit(name, lineNumber, found, true, withNames); err != nil {
			return err
		}
	}
	return nil
}

// grepWord names the group -w's pattern holds the word in, between the characters around it.
const grepWord = "word"

// grepWords is -w -o's matches: the words alone, where the characters on either side of each
// were printed with it, `apple ` and ` foo`.
func grepWords(expr *regexp.Regexp, line string) []string {
	var words []string
	for _, span := range grepWordSpans(expr, line) {
		words = append(words, line[span[0]:span[1]])
	}
	return words
}

// grepWordSpans is where -w's words are in line. A search goes on from the end of a word rather
// than from past the character after it, so a word one space after another is found too; it
// was not. A word cannot begin where the last ended, since that is no boundary.
func grepWordSpans(expr *regexp.Regexp, line string) [][2]int {
	var spans [][2]int
	group := expr.SubexpIndex(grepWord)
	for start := 0; start <= len(line) && group >= 0; {
		found := expr.FindStringSubmatchIndex(line[start:])
		if found == nil {
			break
		}
		from, to := -1, -1
		if found[2*group] >= 0 {
			from, to = start+found[2*group], start+found[2*group+1]
		}
		if from < 0 || from == start && start > 0 {
			start += max(found[0], 1)
			continue
		}
		spans = append(spans, [2]int{from, to})
		start = max(to, from+1)
	}
	return spans
}

// grepReportFile writes the per-file answer that -l, -L and -c give instead of
// lines.
func grepReportFile(printer *grepPrinter, flags grepFlags, name string, matched bool, count int, withNames bool) error {
	switch {
	case flags.filesOnly:
		if matched {
			_, err := fmt.Fprintln(printer.stdout, printer.paint(grepColorName, name))
			printer.wrote = true
			return err
		}
	case flags.withoutMatch:
		// -L is -l inverted: the files that did not match.
		if !matched {
			_, err := fmt.Fprintln(printer.stdout, printer.paint(grepColorName, name))
			printer.wrote = true
			return err
		}
	case flags.countOnly:
		prefix := ""
		if withNames {
			prefix = printer.paint(grepColorName, name) + printer.paint(grepColorSeparator, ":")
		}
		_, err := fmt.Fprintf(printer.stdout, "%s%d\n", prefix, count)
		printer.wrote = true
		return err
	}
	return nil
}
