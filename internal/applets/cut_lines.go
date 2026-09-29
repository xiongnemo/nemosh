package applets

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

// cutFile is busybox's cut_file: every line of one input, and what -d of a newline counts from
// one line to the next.
func (c *cutSpec) cutFile(input io.Reader, stdout io.Writer) error {
	reader := bufio.NewReader(input)
	number, printed := 0, false
	for {
		text, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return readErr
		}
		// A CRLF line is read as its text, as busybox-w32's text-mode stdio reads it.
		if line, ended := strings.CutSuffix(text, "\n"); ended {
			text = strings.TrimSuffix(line, "\r")
		} else if text == "" {
			break
		}
		var out string
		switch {
		case !c.fields:
			out = c.cutBytes(text) + "\n"
		case c.lines:
			number++
			if !c.lineSelected(number) {
				break
			}
			if printed {
				out = c.odelim
			}
			out, printed = out+text, true
		default:
			selected, kept := c.cutFields(text)
			if kept {
				out = selected + "\n"
			}
		}
		if _, err := io.WriteString(stdout, out); err != nil {
			return err
		}
		if readErr != nil {
			break
		}
	}
	// The lines -d of a newline cut are joined by -O, and end as a line does.
	if c.lines && printed {
		if _, err := io.WriteString(stdout, "\n"); err != nil {
			return err
		}
	}
	return nil
}

// cutBytes is busybox's -b: each byte once, in the order of the ranges, with -O between two
// ranges that do not touch.
func (c *cutSpec) cutBytes(line string) string {
	printed := make([]bool, len(line))
	var out strings.Builder
	separate := false
	for _, r := range c.ranges {
		for at := r.start; at < len(line); {
			if !printed[at] {
				printed[at] = true
				if separate && at != 0 && !printed[at-1] {
					separate = false
					out.WriteString(c.odelim)
				}
				out.WriteByte(line[at])
			}
			if at++; at > r.end {
				separate = c.odelim != ""
				break
			}
		}
	}
	return out.String()
}

// lineSelected is busybox's walk of the list for -d of a newline: whether line number, from 1,
// is in a range, the ranges taken in the order they are in.
func (c *cutSpec) lineSelected(number int) bool {
	pos := 0
	at := c.ranges[pos].start
	if number <= at {
		return false
	}
	for at++; at < number; at++ {
		if at <= c.ranges[pos].end {
			continue
		}
		if pos++; pos == len(c.ranges) {
			return false
		}
		if at = c.ranges[pos].start; number <= at {
			return false
		}
	}
	return true
}

// cutFields is busybox's field loop, as it is: the ranges are walked in order, counting the
// delimiters met, from the start of the line again for each range under -D; -F keeps the
// delimiters inside a range and skips an empty field. It answers false for a line -s drops.
func (c *cutSpec) cutFields(line string) (string, bool) {
	length := len(line)
	if length == 0 && c.suppress {
		return "", false
	}
	var out strings.Builder
	first := true
	pos, next, start, end, count := 0, 0, 0, 0, 0
	for {
		rangeOver := end == length || count > c.ranges[pos].end
		lineOver := false
		for {
			if rangeOver {
				if pos++; pos == len(c.ranges) {
					return out.String(), true
				}
				if c.noSort {
					start, count, next = 0, 0, 0
				}
				end = 0
			}
			if lineOver = next == length; !lineOver {
				break
			}
			end = length
			if !c.noSort && pos == 0 && count == 0 {
				if c.suppress {
					return "", false
				}
				break
			}
			if rangeOver = count < c.ranges[pos].start; !rangeOver {
				break
			}
		}
		if !lineOver {
			if c.pattern != nil {
				match := c.pattern.FindStringIndex(line[next:])
				if match == nil {
					next = length
					continue
				}
				end = next + match[0]
				next += max(match[1], 1)
			} else if end, next = next, next+1; line[end] != c.delim {
				continue
			}
			if count++; count <= c.ranges[pos].start {
				start = next
				continue
			}
			if c.pattern != nil && count <= c.ranges[pos].end {
				continue
			}
		}
		if end != start || c.pattern == nil {
			if !first {
				out.WriteString(c.odelim)
			}
			first = false
			out.WriteString(line[start:end])
		}
		start = next
	}
}
