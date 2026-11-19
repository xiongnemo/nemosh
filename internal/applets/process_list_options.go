package applets

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ps's arguments, read as procps-ng reads them: UNIX options after a dash (`-ef`, `-o pid`), BSD
// ones without (`aux`, `axo pid`), GNU long ones after two, and a bare list of pids. `-aux` is the
// BSD `aux` it is always meant as, which procps also takes it for when there is no user called x.

type psStyle int

const (
	psDefault psStyle = iota
	psFull
	psUserStyle
	psCustom
)

type psHierarchy int

const (
	psFlat psHierarchy = iota
	psIndented
	psForest
)

// psColumn is a field as one format asks for it, with its header and width.
type psColumn struct {
	field  psField
	header string
	width  int
}

type psSortKey struct {
	field      psField
	descending bool
}

type psOptions struct {
	// all is -e, -A or BSD's a; bsd is any BSD word, which without a lists this account's own.
	all, bsd, running, negate, noHeader bool
	pids, ppids                         []int
	users, commands                     []string
	style                               psStyle
	columns                             []psColumn
	sort                                []psSortKey
	hierarchy                           psHierarchy
	wide, width                         int
}

// errLongFormat refuses -l rather than print five columns of dashes under its headers.
var errLongFormat = errors.New("-l: F, PRI, NI, ADDR and WCHAN have no Windows equivalent; -f and -o FORMAT print the rest")

// psArguments is the words still to read, for an option that takes the next as its value.
type psArguments struct {
	words []string
	next  int
}

func (a *psArguments) value(rest, name string) (string, error) {
	if rest != "" {
		return rest, nil
	}
	if a.next >= len(a.words) {
		return "", fmt.Errorf("option requires an argument -- %s", name)
	}
	a.next++
	return a.words[a.next-1], nil
}

func parsePsArgs(args []string) (psOptions, error) {
	var options psOptions
	words := &psArguments{words: args}
	for words.next < len(words.words) {
		word := words.words[words.next]
		words.next++
		var err error
		switch {
		case word == "--":
		case strings.HasPrefix(word, "--"):
			err = options.long(word[2:], words)
		case len(word) > 1 && word[0] == '-' && strings.Trim(word[1:], "aux") == "" && strings.ContainsRune(word, 'x'):
			err = options.bsdWord(word[1:], words)
		case len(word) > 1 && word[0] == '-':
			err = options.unixLetters(word[1:], words)
		default:
			err = options.bsdWord(word, words)
		}
		if err != nil {
			return options, err
		}
	}
	return options, nil
}

func (o *psOptions) unixLetters(letters string, words *psArguments) error {
	for index := 0; index < len(letters); index++ {
		switch letter := letters[index]; letter {
		case 'e', 'A':
			o.all = true
		case 'f':
			if err := o.setStyle(psFull); err != nil {
				return err
			}
		case 'H':
			o.hierarchy = psIndented
		case 'N':
			o.negate = true
		case 'w':
			o.wide++
		case 'l':
			return errLongFormat
		case 'o', 'p', 'q', 'u', 'U', 'C':
			value, err := words.value(letters[index+1:], string(letter))
			if err != nil {
				return err
			}
			return o.valued(letter, value)
		default:
			return invalidOption(letter)
		}
	}
	return nil
}

// bsdWord is a word of BSD letters, or a list of pids, which BSD's ps takes bare.
func (o *psOptions) bsdWord(word string, words *psArguments) error {
	if pids, err := parsePsPids(word); err == nil {
		o.pids = append(o.pids, pids...)
		return nil
	}
	if word == "" || strings.ContainsFunc(word, func(r rune) bool { return !('a' <= r && r <= 'z' || 'A' <= r && r <= 'Z') }) {
		return fmt.Errorf("%s: neither process IDs nor BSD-style letters", word)
	}
	o.bsd = true
	for index := 0; index < len(word); index++ {
		switch letter := word[index]; letter {
		case 'a':
			o.all = true
		case 'x':
			// Lifts the need for a controlling terminal, which no process has here: implied.
		case 'u':
			if err := o.setStyle(psUserStyle); err != nil {
				return err
			}
		case 'f':
			o.hierarchy = psForest
		case 'w':
			o.wide++
		case 'h':
			o.noHeader = true
		case 'r':
			o.running = true
		case 'l':
			return errLongFormat
		case 'o', 'p', 'q', 'U', 'k':
			value, err := words.value(word[index+1:], string(letter))
			if err != nil {
				return err
			}
			if letter == 'k' {
				return o.addSort(value)
			}
			return o.valued(letter, value)
		default:
			return invalidOption(letter)
		}
	}
	return nil
}

func (o *psOptions) long(text string, words *psArguments) error {
	name, value, given := strings.Cut(text, "=")
	argument := func() (string, error) {
		if given {
			return value, nil
		}
		return words.value("", name)
	}
	switch name {
	case "no-headers", "no-heading":
		o.noHeader = true
		return nil
	case "forest":
		o.hierarchy = psForest
		return nil
	case "deselect":
		o.negate = true
		return nil
	case "pid", "ppid", "user", "User", "sort", "format", "width", "cols", "columns":
	default:
		return unknownLongOption("--" + name)
	}
	value, err := argument()
	if err != nil {
		return err
	}
	switch name {
	case "pid":
		return o.valued('p', value)
	case "ppid":
		pids, err := parsePsPids(value)
		o.ppids = append(o.ppids, pids...)
		return err
	case "user", "User":
		return o.valued('u', value)
	case "sort":
		return o.addSort(value)
	case "format":
		return o.valued('o', value)
	}
	width, err := strconv.Atoi(value)
	if err != nil || width <= 0 {
		return fmt.Errorf("invalid width: %s", value)
	}
	o.width = width
	return nil
}
