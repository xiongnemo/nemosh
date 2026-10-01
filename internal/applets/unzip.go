package applets

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
)

// unzip: the archive format Windows actually uses, and one it has no command-line
// tool for -- `System32` holds `tar.exe` and `curl.exe` and no `unzip` (measured).
//
// zip is read through archive/zip, which needs a ReaderAt and a size, so unlike
// tar this cannot work on a pipe: the central directory lives at the *end* of the
// file. An operand is therefore required, and saying so is better than reading
// the whole of stdin into memory to pretend otherwise.
//
// What it prints is busybox's: `Archive:  NAME` first, then `   creating:` and
// `  inflating:` lines on standard output, -l's table and -v's.

func newUnzipApplet() Applet {
	return simpleApplet{name: "unzip", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		request, name, err := unzipArgs(args)
		if err != nil {
			return err
		}
		if name == "" || name == "-" {
			// Not a pipe: the central directory is at the end of the file, so the
			// reader must be able to seek.
			return fmt.Errorf("an archive operand is required; zip cannot be read from a pipe")
		}
		request.view = ProcessViewFromContext(ctx)
		file, shown, err := unzipOpen(request.view, name)
		if err != nil {
			return err
		}
		defer file.Close()
		root, err := request.changeDirectory()
		if err != nil {
			return err
		}
		if request.test {
			// busybox's -t sends standard output to /dev/null, its listing with it.
			stdout = io.Discard
		}
		request.header(stdout, shown)
		info, err := file.Stat()
		if err != nil {
			return operandFailure(shown, err)
		}
		archive, err := zip.NewReader(file, info.Size())
		if err != nil {
			return operandFailure(shown, fmt.Errorf("cannot read as a zip archive"))
		}
		return request.run(archive, root, stdout, stderr)
	}}
}

type unzipRequest struct {
	list     bool
	verbose  bool
	test     bool
	toStdout bool
	flatten  bool
	// overwrite is the last of -n and -o, 'n' or 'o', or 0 for neither.
	overwrite byte
	// quiet counts -q, and -p and -t each count one: one leaves out `Archive:` and what is
	// extracted, two -l's heads and total as well.
	quiet     int
	directory string
	wanted    []string
	exclude   []string
	// view is the shell unzip runs in, whose umask what it extracts is made through.
	view ProcessView
}

// unzipArgs reads unzip's arguments as busybox's getopt string "-d:lnotpqxjvK" has them: an
// option anywhere, the first operand the archive, the operands after it the members wanted,
// and those after -x the members left out. -x took the next argument alone, so `unzip -x
// a.zip b` read a.zip as the member to leave out and b as the archive, and `-x a b` left
// out a alone.
func unzipArgs(args []string) (unzipRequest, string, error) {
	var request unzipRequest
	archive, excluding, operandsOnly := "", false, false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" && !operandsOnly {
			operandsOnly = true
			continue
		}
		if operandsOnly || len(arg) < 2 || arg[0] != '-' {
			switch {
			case archive == "":
				archive = arg
			case excluding:
				request.exclude = append(request.exclude, arg)
			default:
				request.wanted = append(request.wanted, arg)
			}
			continue
		}
		for position := 1; position < len(arg); position++ {
			letter := arg[position]
			switch letter {
			case 'd':
				value, consumed, err := optionArgument(args, index, arg, position, letter)
				if err != nil {
					return request, "", err
				}
				request.directory, index, position = value, index+consumed, len(arg)
			case 'l', 'v':
				request.list, request.verbose = true, request.verbose || letter == 'v'
			case 'n', 'o':
				request.overwrite = letter
			case 't', 'p', 'q':
				request.test = request.test || letter == 't'
				request.toStdout = request.toStdout || letter == 'p'
				request.quiet++
			case 'x':
				excluding = true
			case 'j':
				request.flatten = true
			case 'K':
				// Keeping a set-user-ID bit, which no entry restores here.
			default:
				return request, "", invalidOption(letter)
			}
		}
	}
	return request, archive, nil
}

// unzipOpen opens the archive where busybox's unzip looks for it: NAME, then NAME.zip, then
// NAME.ZIP, so `unzip a` reads a.zip. A directory is no archive. What is not there is
// `cannot open NAME[.zip]`; it was that NAME could not be read as a zip archive.
func unzipOpen(view ProcessView, name string) (*os.File, string, error) {
	for _, suffix := range []string{"", ".zip", ".ZIP"} {
		native, err := resolveHostPath(view, name+suffix)
		if err != nil {
			return nil, "", operandFailure(name, err)
		}
		file, err := os.Open(native)
		if err != nil {
			continue
		}
		if info, err := file.Stat(); err == nil && !info.IsDir() {
			return file, name + suffix, nil
		}
		file.Close()
	}
	return nil, "", fmt.Errorf("cannot open %s[.zip]", name)
}

// changeDirectory is where the entries go: -d's DIR, made if it is not there, one level as
// busybox's mkdir(2) makes it, and then a directory to change into or a failure. Its missing
// parents were made too, so `-d /tpm/x` made /tpm.
func (r unzipRequest) changeDirectory() (string, error) {
	target := r.directory
	if target == "" {
		target = "."
	}
	native, err := resolveHostPath(r.view, target)
	if err != nil {
		return "", operandFailure(target, err)
	}
	if r.directory == "" {
		return native, nil
	}
	os.Mkdir(native, createMode(r.view, 0o777))
	info, err := os.Stat(native)
	if err == nil && !info.IsDir() {
		err = errNotADirectory
	}
	if err != nil {
		return "", fmt.Errorf("cannot change directory to '%s': %s", target, causeText(err))
	}
	return native, nil
}

// header is what busybox's unzip prints before the entries: `Archive:  NAME` unless -q, and
// -l's column heads unless -qq.
func (r unzipRequest) header(stdout io.Writer, name string) {
	if r.quiet == 0 {
		fmt.Fprintf(stdout, "Archive:  %s\n", name)
	}
	if r.list && r.quiet <= 1 {
		heads := "  Length      Date    Time    Name\n---------  ---------- -----   ----\n"
		if r.verbose {
			heads = " Length   Method    Size  Cmpr    Date    Time   CRC-32   Name\n" +
				"--------  ------  ------- ---- ---------- ----- --------  ----\n"
		}
		io.WriteString(stdout, heads)
	}
}

func (r unzipRequest) run(archive *zip.Reader, root string, stdout, stderr io.Writer) error {
	var selected []*zip.File
	for _, entry := range archive.File {
		if !unzipMatches(r.exclude, entry.Name) && (len(r.wanted) == 0 || unzipMatches(r.wanted, entry.Name)) {
			selected = append(selected, entry)
		}
	}
	if r.list {
		return r.writeListing(selected, stdout)
	}
	collisions := newArchiveCollisions()
	for _, entry := range selected {
		if err := r.oneEntry(entry, root, collisions, stdout, stderr); err != nil {
			return err
		}
	}
	return nil
}

// unzipMatches is busybox's find_list_entry: fnmatch(3) without FNM_PATHNAME, so a `*`
// crosses a slash and `unzip a.zip '*.txt'` takes sub/b.txt as well. filepath.Match stopped
// at one.
func unzipMatches(patterns []string, name string) bool {
	for _, pattern := range patterns {
		if pattern == name {
			return true
		}
		if matcher, err := compileFindPathPattern(pattern); err == nil && matcher.MatchString(name) {
			return true
		}
	}
	return false
}
