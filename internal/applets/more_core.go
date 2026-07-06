package applets

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"unicode"
)

func newPwdApplet() Applet {
	return simpleApplet{name: "pwd", run: func(_ []string, _ io.Reader, stdout, _ io.Writer) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, cwd)
		return err
	}}
}

func newHeadApplet() Applet {
	return simpleApplet{name: "head", run: func(args []string, stdin io.Reader, stdout, _ io.Writer) error {
		if len(args) == 0 {
			return copyHead(stdout, stdin)
		}
		for _, path := range args {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			copyErr := copyHead(stdout, file)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
		return nil
	}}
}

func copyHead(stdout io.Writer, input io.Reader) error {
	scanner := bufio.NewScanner(input)
	for i := 0; i < 10 && scanner.Scan(); i++ {
		if _, err := fmt.Fprintln(stdout, scanner.Text()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func newWcApplet() Applet {
	return simpleApplet{name: "wc", run: func(args []string, stdin io.Reader, stdout, _ io.Writer) error {
		if len(args) == 0 {
			counts, err := countBytes(stdin)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(stdout, "%d %d %d\n", counts.lines, counts.words, counts.bytes)
			return err
		}
		for _, path := range args {
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
			if _, err := fmt.Fprintf(stdout, "%d %d %d %s\n", counts.lines, counts.words, counts.bytes, path); err != nil {
				return err
			}
		}
		return nil
	}}
}

type wcCounts struct {
	lines int
	words int
	bytes int
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
