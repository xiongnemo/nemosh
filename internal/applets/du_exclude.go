package applets

import (
	"os"
	"path"
	"strings"
)

// du's GNU long options, and the two of them busybox's du has not got: --apparent-size and
// --exclude. Those have no letter of their own, so each stands for a control character no one
// types; -X is GNU's letter for --exclude-from.
const (
	duApparent    = "\x01"
	duExclude     = "\x02"
	duLongOptions = "all=a apparent-size=" + duApparent + " bytes=b total=c max-depth=d human-readable=h " +
		"count-links=l dereference=L dereference-args=H summarize=s one-file-system=x " +
		"exclude=" + duExclude + " exclude-from=X"
)

// duExcludePatterns are --exclude's patterns, and -X's files' a line at a time, as GNU reads them.
func duExcludePatterns(view ProcessView, options appletOptions) ([]string, error) {
	patterns := append([]string(nil), options.all(duExclude[0])...)
	for _, name := range options.all('X') {
		native, err := resolveHostPath(view, name)
		if err != nil {
			return nil, err
		}
		content, err := os.ReadFile(native)
		if err != nil {
			return nil, cannotOpen(name, err)
		}
		for _, line := range strings.Split(string(content), "\n") {
			if line = strings.TrimSuffix(line, "\r"); line != "" {
				patterns = append(patterns, line)
			}
		}
	}
	return patterns, nil
}

// excluded is whether an entry is left out and counted nowhere: a pattern matches its name, or,
// for a pattern with a slash in it, the path du would print for it.
func (r *duRun) excluded(name, shown string) bool {
	for _, pattern := range r.exclude {
		subject := name
		if strings.Contains(pattern, "/") {
			subject = shown
		}
		if matched, _ := path.Match(pattern, subject); matched {
			return true
		}
	}
	return false
}

// inBytes is whether sizes are counted in bytes, what a file holds, rather than in the blocks it
// takes: -b's, and --apparent-size's.
func (r *duRun) inBytes() bool { return r.bytes || r.apparent }
