package applets

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// GNU cp's --preserve[=LIST] and --no-preserve=LIST, which busybox's has not got. LIST is mode,
// ownership, timestamps, links, context, xattr or all, by commas; plain --preserve is mode,
// ownership and timestamps, which is -p, and -a is -dR --preserve=all. Each is a letter no one
// can type, \x01 to \x05 to keep and \x11 to \x15 not to, so that it is read in order with -p
// and -a, as GNU reads them: `cp -a --no-preserve=timestamps` keeps all but the times.
//
// ownership is kept as -p keeps it, which on Windows is not at all: a file's owner is the one
// who made it, as busybox-w32's chown changes nothing either. context is SELinux's, which
// Windows has not got, and xattr would be a file's alternate data streams, which this cp copies
// only where CopyFile2 does; asked for by name each is refused, as GNU refuses context on a
// kernel without SELinux, and under all each is passed over, as GNU passes over its failure.

// cpKeep is what a copy keeps of its source.
type cpKeep struct{ mode, owner, times, links bool }

// cpKeepAll is -a's and all's.
var cpKeepAll = cpKeep{mode: true, owner: true, times: true, links: true}

// cpPreserveLetters are the letters --preserve=LIST or, not keeping, --no-preserve=LIST stands
// for, as an option word.
func cpPreserveLetters(option, list string, keeping bool) (string, error) {
	letters := []byte{'-'}
	for _, name := range strings.Split(list, ",") {
		var letter byte
		switch name {
		case "mode":
			letter = 1
		case "ownership":
			letter = 2
		case "timestamps":
			letter = 3
		case "links":
			letter = 4
		case "all":
			letter = 5
		case "context", "xattr":
			if !keeping {
				continue
			}
			if name == "context" {
				return "", errors.New("cannot preserve security context without an SELinux-enabled kernel")
			}
			return "", errors.New("cannot preserve extended attributes: Windows keeps them as alternate data streams, which this cp does not always copy")
		default:
			return "", fmt.Errorf("invalid argument '%s' for '--%s'", name, option)
		}
		if !keeping {
			letter |= 0x10
		}
		letters = append(letters, letter)
	}
	if len(letters) == 1 {
		return "", nil
	}
	return string(letters), nil
}

// cpPreserveFlags are the letters cpPreserveLetters makes, for the option reader.
const cpPreserveFlags = "\x01\x02\x03\x04\x05\x11\x12\x13\x14\x15"

// cpKeepFrom reads -a, -p and the --preserve letters in the order they were given.
func cpKeepFrom(order []byte) cpKeep {
	var keep cpKeep
	for _, letter := range order {
		on := letter < 0x10
		switch letter {
		case 'a', 0x05:
			keep = cpKeepAll
		case 0x15:
			keep = cpKeep{}
		case 'p':
			keep.mode, keep.owner, keep.times = true, true, true
		case 0x01, 0x11:
			keep.mode = on
		case 0x02, 0x12:
			keep.owner = on
		case 0x03, 0x13:
			keep.times = on
		case 0x04, 0x14:
			keep.links = on
		}
	}
	return keep
}

// copyKeepingLinks is copyRegular under --preserve=links: a file with more than one name, met
// again under another, is linked to its first copy rather than copied twice, as GNU's
// remember_copied has it.
func (r *cpRun) copyKeepingLinks(source, dest pathOperand, info os.FileInfo) bool {
	id, links, ok := fileIdentity(source.host, info)
	if !ok || links < 2 {
		return r.copyRegular(source, dest, info)
	}
	first, seen := r.linked[id]
	if !seen {
		made := r.copyRegular(source, dest, info)
		if made {
			if r.linked == nil {
				r.linked = map[fileID]string{}
			}
			r.linked[id] = dest.host
		}
		return made
	}
	if err := os.Link(first, dest.host); err != nil {
		if answer := r.askAndUnlink(dest, err); answer != unlinked {
			return answer == unlinkDeclined
		}
		if err := os.Link(first, dest.host); err != nil {
			return r.fail(fmt.Errorf("cannot create hard link '%s': %s", dest.operand, causeText(err)))
		}
	}
	r.report(source, dest)
	return true
}
