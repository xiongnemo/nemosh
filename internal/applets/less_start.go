package applets

import (
	"fmt"
	"strconv"
	"strings"
)

// less +CMD, GNU's: a command run as less starts, `less +F log` to follow a log and `less +G`,
// `less +/error` and `less +120` to start where the reading is. busybox's less has none, and took
// the word for a FILE. The commands here are G, g, F, /PATTERN, ?PATTERN and a line number, with
// or without the g GNU reads after one; any other is refused by name rather than run as half of
// itself.
//
// One that needs a line waits for it: a search on a pipe whose match has not arrived looks again
// at each line that does, and says it found nothing only once the input has ended, and G stays at
// the end while more comes. A key the reader presses takes over from it.

// lessStart is a command to start with: its letter -- 'l' for a line number -- and what it needs.
type lessStart struct {
	kind    byte
	pattern string
	line    int
}

func parseLessStart(word string) (lessStart, error) {
	command := word[1:]
	switch {
	case command == "G" || command == "g" || command == "F":
		return lessStart{kind: command[0]}, nil
	case len(command) > 1 && (command[0] == '/' || command[0] == '?'):
		return lessStart{kind: command[0], pattern: command[1:]}, nil
	}
	digits := strings.TrimSuffix(command, "g")
	if line, err := strconv.Atoi(digits); err == nil && line > 0 && digits[0] != '+' && digits[0] != '-' {
		return lessStart{kind: 'l', line: line}, nil
	}
	return lessStart{}, fmt.Errorf("%s: the commands less starts with here are +G, +g, +F, +/PATTERN, +?PATTERN and +N", word)
}

// settle runs the start command as far as what has arrived lets it.
func (p *lessPager) settle() {
	switch p.start.kind {
	case 0:
		return
	case 'g':
		p.top, p.start.kind = 0, 0
	case 'G':
		p.top = len(p.lines)
		if !p.waiting {
			p.start.kind = 0
		}
	case 'F':
		p.startFollowing()
		p.start.kind = 0
	case 'l':
		if len(p.lines) >= p.start.line || !p.waiting {
			p.top, p.start.kind = p.start.line-1, 0
		}
	case '/', '?':
		p.pattern = p.start.pattern
		forward, from := p.start.kind == '/', 0
		if !forward {
			// The last match is not known until the last line is.
			if p.waiting {
				return
			}
			from = len(p.lines) - 1
		}
		if index := p.find(forward, from); index >= 0 {
			p.top, p.start.kind = index, 0
		} else if !p.waiting {
			p.message, p.start.kind = "pattern not found: "+p.pattern, 0
		}
	}
	p.clampTop()
}

// find is the first line from from, forward or back, that the pattern matches, or -1.
func (p *lessPager) find(forward bool, from int) int {
	compiled, ok := p.compiledPattern()
	if !ok {
		return -1
	}
	step := 1
	if !forward {
		step = -1
	}
	for index := from; index >= 0 && index < len(p.lines); index += step {
		if compiled.MatchString(p.searched(p.lines[index])) {
			return index
		}
	}
	return -1
}

// startFollowing is F: the screen stays at the end as lines come, and a FILE is read on past its
// end as it grows. stopFollowing is ^C's, or any key's that moves away.
func (p *lessPager) startFollowing() {
	p.following = true
	if p.feed != nil {
		p.feed.follow(true)
	}
}

func (p *lessPager) stopFollowing() {
	p.following = false
	if p.feed != nil {
		p.feed.follow(false)
	}
}
