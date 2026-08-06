package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
)

func newHeadApplet() Applet {
	return simpleApplet{name: "head", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		count, paths, err := lineCountArgs(args, 10)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			return copyHead(stdout, stdin, count)
		}
		view := ProcessViewFromContext(ctx)
		for _, path := range paths {
			file, err := OpenProcessInput(ctx, view, path)
			if err != nil {
				return operandFailure(path, err)
			}
			copyErr := copyHead(stdout, file, count)
			closeErr := file.Close()
			if err := errors.Join(copyErr, closeErr); err != nil {
				return err
			}
		}
		return nil
	}}
}

func copyHead(stdout io.Writer, input io.Reader, count int) error {
	scanner := bufio.NewScanner(input)
	for i := 0; i < count && scanner.Scan(); i++ {
		if _, err := fmt.Fprintln(stdout, scanner.Text()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func newTailApplet() Applet {
	return simpleApplet{name: "tail", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		count, paths, err := lineCountArgs(args, 10)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			return copyTail(stdout, stdin, count)
		}
		view := ProcessViewFromContext(ctx)
		for _, path := range paths {
			file, err := OpenProcessInput(ctx, view, path)
			if err != nil {
				return cannotOpen(path, err)
			}
			copyErr := copyTail(stdout, file, count)
			closeErr := file.Close()
			if err := errors.Join(copyErr, closeErr); err != nil {
				return err
			}
		}
		return nil
	}}
}

func copyTail(stdout io.Writer, input io.Reader, count int) error {
	scanner := bufio.NewScanner(input)
	lines := make([]string, 0, count)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > count {
			lines = lines[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return err
		}
	}
	return nil
}

func lineCountArgs(args []string, defaultCount int) (int, []string, error) {
	if len(args) < 2 || args[0] != "-n" {
		return defaultCount, args, nil
	}
	count, err := strconv.Atoi(args[1])
	if err != nil || count < 0 {
		return 0, nil, fmt.Errorf("invalid line count: %s", args[1])
	}
	return count, args[2:], nil
}
