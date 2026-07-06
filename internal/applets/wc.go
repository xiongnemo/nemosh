package applets

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"unicode"
)

func newWcApplet() Applet {
	return simpleApplet{name: "wc", run: func(args []string, stdin io.Reader, stdout, _ io.Writer) error {
		flags, paths := wcArgs(args)
		if len(paths) == 0 {
			counts, err := countBytes(stdin)
			if err != nil {
				return err
			}
			err = printWcCounts(stdout, flags, counts, "")
			return err
		}
		for _, path := range paths {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			counts, countErr := countBytes(file)
			closeErr := file.Close()
			if countErr != nil {
				return countErr
			}
			if closeErr != nil {
				return closeErr
			}
			if err := printWcCounts(stdout, flags, counts, path); err != nil {
				return err
			}
		}
		return nil
	}}
}

type wcFlags struct {
	lines bool
	words bool
	bytes bool
}

type wcCounts struct {
	lines int
	words int
	bytes int
}

func wcArgs(args []string) (wcFlags, []string) {
	flags := wcFlags{lines: true, words: true, bytes: true}
	pathsStart := 0
	seenFlag := false
	for pathsStart < len(args) && len(args[pathsStart]) > 1 && args[pathsStart][0] == '-' {
		for _, r := range args[pathsStart][1:] {
			if !seenFlag {
				flags = wcFlags{}
				seenFlag = true
			}
			switch r {
			case 'l':
				flags.lines = true
			case 'w':
				flags.words = true
			case 'c':
				flags.bytes = true
			}
		}
		pathsStart++
	}
	return flags, args[pathsStart:]
}

func printWcCounts(stdout io.Writer, flags wcFlags, counts wcCounts, path string) error {
	values := make([]int, 0, 3)
	if flags.lines {
		values = append(values, counts.lines)
	}
	if flags.words {
		values = append(values, counts.words)
	}
	if flags.bytes {
		values = append(values, counts.bytes)
	}
	for i, value := range values {
		if i > 0 {
			if _, err := fmt.Fprint(stdout, " "); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(stdout, value); err != nil {
			return err
		}
	}
	if path != "" {
		if _, err := fmt.Fprintf(stdout, " %s", path); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(stdout)
	return err
}

func countBytes(input io.Reader) (wcCounts, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return wcCounts{}, err
	}
	return wcCounts{lines: bytes.Count(data, []byte("\n")), words: countWords(data), bytes: len(data)}, nil
}

func countWords(data []byte) int {
	words := 0
	inWord := false
	for _, r := range string(data) {
		if unicode.IsSpace(r) {
			inWord = false
			continue
		}
		if !inWord {
			words++
			inWord = true
		}
	}
	return words
}
