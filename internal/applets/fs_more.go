package applets

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func newLsApplet() Applet {
	return simpleApplet{name: "ls", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		paths := args
		if len(paths) == 0 {
			paths = []string{"."}
		}
		for _, target := range paths {
			if err := listPath(stdout, target); err != nil {
				return err
			}
		}
		return nil
	}}
}

func listPath(stdout io.Writer, target string) error {
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		_, err := fmt.Fprintln(stdout, target)
		return err
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		if _, err := fmt.Fprintln(stdout, name); err != nil {
			return err
		}
	}
	return nil
}

func newCpApplet() Applet {
	return simpleApplet{name: "cp", run: func(args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		if len(args) != 2 {
			return ErrExitFalse
		}
		return copyFile(args[0], destinationPath(args[0], args[1]))
	}}
}

func copyFile(sourcePath, destPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	dest, err := os.Create(destPath)
	if err != nil {
		if closeErr := source.Close(); closeErr != nil {
			return closeErr
		}
		return err
	}
	_, copyErr := io.Copy(dest, source)
	sourceCloseErr := source.Close()
	closeErr := dest.Close()
	if copyErr != nil {
		return copyErr
	}
	if sourceCloseErr != nil {
		return sourceCloseErr
	}
	return closeErr
}

func newMvApplet() Applet {
	return simpleApplet{name: "mv", run: func(args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		if len(args) != 2 {
			return ErrExitFalse
		}
		destPath := destinationPath(args[0], args[1])
		if err := os.Rename(args[0], destPath); err == nil {
			return nil
		}
		if err := copyFile(args[0], destPath); err != nil {
			return err
		}
		return os.Remove(args[0])
	}}
}

func destinationPath(sourcePath, destPath string) string {
	info, err := os.Stat(destPath)
	if err == nil && info.IsDir() {
		return filepath.Join(destPath, filepath.Base(sourcePath))
	}
	return destPath
}
