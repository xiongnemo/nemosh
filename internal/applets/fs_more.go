package applets

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func newLsApplet() Applet {
	return simpleApplet{name: "ls", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, paths, err := lsArgs(args)
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			paths = []string{"."}
		}
		for _, target := range paths {
			if err := listPath(stdout, target, options); err != nil {
				return err
			}
		}
		return nil
	}}
}

type lsOptions struct {
	all   bool
	long  bool
	human bool
}

func lsArgs(args []string) (lsOptions, []string, error) {
	var options lsOptions
	index := 0
	for index < len(args) {
		arg := args[index]
		if arg == "--" {
			index++
			break
		}
		if len(arg) <= 1 || arg[0] != '-' {
			break
		}
		for _, flag := range arg[1:] {
			switch flag {
			case 'a':
				options.all = true
			case 'l':
				options.long = true
			case 'h':
				options.human = true
			default:
				return lsOptions{}, nil, fmt.Errorf("unsupported ls option: -%c", flag)
			}
		}
		index++
	}
	return options, args[index:], nil
}

func listPath(stdout io.Writer, target string, options lsOptions) error {
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return printLsEntry(stdout, lsEntry{name: target, info: info}, options)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return err
	}
	items := make([]lsEntry, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !options.all && strings.HasPrefix(name, ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		items = append(items, lsEntry{name: name, info: info})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].name < items[j].name
	})
	for _, item := range items {
		if err := printLsEntry(stdout, item, options); err != nil {
			return err
		}
	}
	return nil
}

type lsEntry struct {
	name string
	info os.FileInfo
}

func printLsEntry(stdout io.Writer, entry lsEntry, options lsOptions) error {
	if options.long {
		_, err := fmt.Fprintf(stdout, "%s %s %s\n", entry.info.Mode().String(), lsSize(entry.info.Size(), options), entry.name)
		return err
	}
	_, err := fmt.Fprintln(stdout, entry.name)
	return err
}

func lsSize(size int64, options lsOptions) string {
	if !options.human || size < 1024 {
		return fmt.Sprintf("%d", size)
	}
	units := []string{"K", "M", "G", "T"}
	value := float64(size)
	unit := ""
	for _, candidate := range units {
		value /= 1024
		unit = candidate
		if value < 1024 {
			break
		}
	}
	if value >= 10 || value == float64(int64(value)) {
		return fmt.Sprintf("%.0f%s", value, unit)
	}
	return fmt.Sprintf("%.1f%s", value, unit)
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
