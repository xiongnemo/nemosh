package runtime_test

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRuntimeHelperProcess(t *testing.T) {
	if os.Getenv("NEMOSH_RUNTIME_HELPER_PROCESS") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg != "--" {
			continue
		}
		if os.Args[i+1] == "exit-7" {
			os.Exit(7)
		}
		if os.Args[i+1] == "executable" {
			executable, err := os.Executable()
			if err != nil {
				os.Exit(3)
			}
			fmt.Fprintln(os.Stdout, executable)
			os.Exit(0)
		}
		if os.Args[i+1] == "state" {
			cwd, err := os.Getwd()
			if err != nil {
				os.Exit(4)
			}
			fmt.Fprintln(os.Stdout, cwd)
			fmt.Fprintln(os.Stdout, os.Getenv("NEMOSH_CHILD_VALUE"))
			os.Exit(0)
		}
		if os.Args[i+1] == "argv" {
			for _, value := range os.Args[i+2:] {
				fmt.Fprintln(os.Stdout, value)
			}
			os.Exit(0)
		}
		if os.Args[i+1] == "env-class" {
			name := os.Args[i+2]
			for _, item := range os.Environ() {
				entryName, _, found := strings.Cut(item, "=")
				if found && strings.EqualFold(entryName, name) {
					fmt.Fprintln(os.Stdout, item)
				}
			}
			os.Exit(0)
		}
		if os.Args[i+1] == "copy-stdin" {
			if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
				os.Exit(5)
			}
			os.Exit(0)
		}
		if os.Args[i+1] == "write-large" {
			// A mebibyte, more than any pipe holds, so a reader that leaves early leaves this
			// still writing. It was 64 KiB, the whole of its source, which a Linux or macOS pipe
			// takes in one go: the writer was done before its reader left, and nothing failed.
			chunk := []byte(strings.Repeat("x", 64*1024))
			for range 16 {
				if _, err := os.Stdout.Write(chunk); err != nil {
					break
				}
			}
			os.Exit(0)
		}
		fmt.Fprintln(os.Stdout, os.Args[i+1])
		os.Exit(0)
	}
	os.Exit(2)
}
