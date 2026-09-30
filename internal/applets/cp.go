package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// cp is busybox's (coreutils/cp.c): `cp [-arPLHpfinlsTuv] SOURCE... DEST`, and `-t DIR SOURCE...`.
// Each copy is libbb's copy_file; see cp_copy.go. The options are busybox's, long forms too:
//
//   - -R and -r copy directories, -a is -dpR, -p keeps the mode and the times;
//   - -d and -P copy a symbolic link as a link, -L follows every link and -H those named;
//   - -i and -n say what happens to a destination that is there: asked about on stderr and
//     answered on stdin, or left alone, and of the two the later wins. Otherwise it is replaced,
//     -f or no -f, as busybox-w32 replaces it;
//   - -l and -s make hard or symbolic links rather than copies, -u copies only over an older
//     file, -v prints `'src' -> 'dst'` for each, -T takes DEST as the name however it is, -t
//     names the directory first, --parents copies a SOURCE's path under it and
//     --remove-destination removes a destination before copying rather than after failing.
//
// cp took -r and -R only, so `cp -rf`, `cp -a`, `cp -p` and `cp -n` were all refused. And `cp f f`
// emptied f and answered 0, where busybox refuses to copy a file onto itself.
//
// -r and -R follow symbolic links, which is this shell's simplification for Windows, where
// making one needs a privilege an ordinary session may not have; busybox's copy the link. -d,
// -P and -a keep links as links, as they do there.
func newCpApplet() Applet {
	return simpleApplet{name: "cp", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		flags, operands, err := cpArguments(ctx, args)
		if err != nil {
			return err
		}
		view := ProcessViewFromContext(ctx)
		run := &cpRun{flags: flags, view: view, stdin: stdin, stdout: stdout, stderr: stderr, umask: processFileModeMask(view)}
		last := flags.targetDir
		sources := operands
		if last == "" {
			if len(operands) < 2 {
				return missingOperand()
			}
			last, sources = operands[len(operands)-1], operands[:len(operands)-1]
			if len(sources) > 1 && flags.noTargetDir {
				return fmt.Errorf("extra operand '%s'", operands[2])
			}
		} else if len(sources) == 0 {
			return missingOperand()
		}
		into, err := copyOperand(view, last)
		if err != nil {
			return err
		}
		if flags.targetDir == "" && len(sources) == 1 {
			if single, err := run.singleCopy(view, sources[0], into); err != nil || single {
				return run.status(err)
			}
		} else if info, err := os.Stat(into.host); into.device || err != nil || !info.IsDir() {
			// Named as the failure it is: what is wrong is that the last operand is not a directory
			// the sources can go into, as GNU says. busybox tries each and fails on each.
			return fmt.Errorf("target '%s' is not a directory", last)
		}
		for _, source := range sources {
			if err := ctx.Err(); err != nil {
				return err
			}
			operand, err := copyOperand(view, source)
			if err != nil {
				run.fail(err)
				continue
			}
			run.copyInto(operand, into)
		}
		return run.status(nil)
	}}
}

// singleCopy is `cp A B` when B may be A's new name rather than the directory it goes into:
// when neither is a directory, when -R copies a directory to a name not yet taken, and with -T.
// It answers whether it made the copy, or tried to.
func (r *cpRun) singleCopy(view ProcessView, source string, dest pathOperand) (bool, error) {
	from, err := copyOperand(view, source)
	if err != nil {
		return false, err
	}
	stat := os.Lstat
	if r.flags.dereference || r.flags.derefTop {
		stat = os.Stat
	}
	sourceInfo, err := r.statSource(from, stat)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, cannotStat(source, err)
	}
	stat = os.Stat
	if r.flags.noTargetDir {
		stat = os.Lstat
	}
	destInfo, destErr := r.statSource(dest, stat)
	if destErr != nil && !errors.Is(destErr, fs.ErrNotExist) {
		return false, cannotStat(dest.operand, destErr)
	}
	sourceIsDir := err == nil && sourceInfo.IsDir()
	destIsDir := destErr == nil && destInfo.IsDir()
	if r.flags.noTargetDir && !sourceIsDir && destIsDir {
		return false, fmt.Errorf("'%s' is a directory", dest.operand)
	}
	if r.flags.parents {
		if !destIsDir {
			return false, errors.New("with --parents, the destination must be a directory")
		}
		return false, nil
	}
	if !sourceIsDir && !destIsDir || r.flags.recurse && sourceIsDir && destErr != nil || r.flags.noTargetDir {
		r.copy(from, dest, true)
		return true, nil
	}
	return false, nil
}

// copyInto copies source into the directory dest: to its last component there, or with
// --parents to the whole of its path, making the directories that path needs.
func (r *cpRun) copyInto(source, dest pathOperand) {
	name := lastPathComponent(source.operand)
	if r.flags.parents {
		name = strings.TrimLeft(filepath.ToSlash(source.operand), "/")
		parent := filepath.Dir(filepath.Join(dest.host, filepath.FromSlash(name)))
		if err := os.MkdirAll(parent, maskedMode(0o777, r.umask)); err != nil {
			r.fail(cannotCreateDirectory(filepath.ToSlash(filepath.Dir(joinOperand(dest.operand, name))), err))
			return
		}
	}
	r.copy(source, pathOperand{host: filepath.Join(dest.host, filepath.FromSlash(name)), operand: joinOperand(dest.operand, name)}, true)
}

// pathOperand pairs a resolved host path with the operand the user typed, so the call can use
// the first while a message names the second.
type pathOperand struct {
	host    string
	operand string
	// device is a device of the shell's, /dev/null or /dev/stdin, which has no host path; see
	// cp_device.go.
	device bool
}

// joinHost is a directory's host path and the name of something in it.
func joinHost(directory, name string) string {
	return filepath.Join(directory, filepath.FromSlash(name))
}

// joinOperand is libbb's concat_path_file: a directory's name and a name in it, with one slash
// between them however many the directory's name ends in.
func joinOperand(directory, name string) string {
	return strings.TrimRight(directory, `/\`) + "/" + strings.TrimLeft(name, `/\`)
}

// cpLongOptions are the long forms busybox's cp takes that stand for a letter.
var cpLongOptions = map[string]string{
	"archive": "a", "force": "f", "interactive": "i", "no-clobber": "n", "link": "l",
	"dereference": "L", "no-dereference": "P", "recursive": "R", "symbolic-link": "s",
	"no-target-directory": "T", "verbose": "v", "update": "u",
}

// cpArguments reads cp's options, turning each long one into the letter it stands for.
func cpArguments(ctx context.Context, args []string) (cpFlags, []string, error) {
	var flags cpFlags
	var words []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" || !strings.HasPrefix(arg, "--") {
			if words = append(words, arg); arg == "--" {
				words = append(words, args[index+1:]...)
				break
			}
			continue
		}
		name, value, valued := strings.Cut(arg[2:], "=")
		switch {
		case cpLongOptions[name] != "" && !valued:
			words = append(words, "-"+cpLongOptions[name])
		case name == "target-directory":
			if !valued {
				if index+1 >= len(args) {
					return flags, nil, fmt.Errorf("option '%s' requires an argument", arg)
				}
				index++
				value = args[index]
			}
			words = append(words, "-t", value)
		case name == "remove-destination" && !valued:
			flags.removeDest = true
		case name == "parents" && !valued:
			flags.parents = true
		default:
			return flags, nil, fmt.Errorf("unrecognized option '%s'", arg)
		}
	}
	options, operands, err := parseAppletOptions(ctx, words, "pdRrfinlsLHaPvuT", "t")
	if err != nil {
		return flags, nil, err
	}
	if options.has('l') && options.has('s') {
		return flags, nil, errors.New("-l and -s cannot both be given")
	}
	archive := options.has('a')
	flags.preserve = archive || options.has('p')
	flags.recurse = archive || options.has('R') || options.has('r')
	flags.dereference = !(archive || options.has('d') || options.has('P')) || options.has('L')
	flags.derefTop = options.has('H')
	flags.interactive = options.last("in") == 'i'
	flags.noClobber = options.last("in") == 'n'
	flags.hardLink, flags.softLink = options.has('l'), options.has('s')
	flags.noTargetDir, flags.update, flags.verbose = options.has('T'), options.has('u'), options.has('v')
	flags.targetDir = options.value('t')
	return flags, operands, nil
}
