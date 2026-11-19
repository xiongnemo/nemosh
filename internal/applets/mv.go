package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
)

// mv is busybox's (coreutils/mv.c): `mv [-finTv] SOURCE... DEST`, and `-t DIR SOURCE...`. -i asks
// `mv: overwrite 'x'?` before replacing a destination and reads the answer from stdin, -n leaves
// one alone, -f neither asks nor leaves; of the three the last wins. Without any of them what
// cannot be written is asked about when stdin is a terminal, as busybox asks. -T takes DEST as
// the new name however it is, -t names the directory first, -v prints `'a' -> 'b'`, and the long
// forms stand for their letters.
//
// mv took -f and nothing else. A move across volumes is a copy that keeps modes and times and
// links as links, then the removal of what was moved, as busybox's is; a read-only destination
// is replaced, as busybox-w32's rename replaces one. Several SOURCEs with a last operand that is
// not a directory are refused up front, as cp refuses them.
type mvRun struct {
	// ctx ends a copy across volumes under way, as it ends cp's.
	ctx                                                 context.Context
	force, interactive, noClobber, noTargetDir, verbose bool
	// update is GNU's -u: a destination no older than its source is left as it is.
	update         bool
	terminal       bool
	umask          uint32
	stdin          io.Reader
	stdout, stderr io.Writer
	failed         bool
}

func newMvApplet() Applet {
	return simpleApplet{name: "mv", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := mvArguments(ctx, args)
		if err != nil {
			return err
		}
		view := ProcessViewFromContext(ctx)
		last := options.last("fin")
		run := &mvRun{ctx: ctx, force: last == 'f', interactive: last == 'i', noClobber: last == 'n', noTargetDir: options.has('T'), update: options.has('u'),
			verbose: options.has('v'), terminal: inputIsTerminal(ctx, stdin), umask: processFileModeMask(view),
			stdin: stdin, stdout: stdout, stderr: stderr}
		target, sources := options.value('t'), operands
		if target == "" {
			if len(operands) < 2 {
				return missingOperand()
			}
			target, sources = operands[len(operands)-1], operands[:len(operands)-1]
			if len(sources) > 1 && run.noTargetDir {
				return fmt.Errorf("extra operand '%s'", operands[2])
			}
		} else if len(sources) == 0 {
			return missingOperand()
		}
		targetHost, err := resolveHostPath(view, target)
		if err != nil {
			return err
		}
		into := pathOperand{host: targetHost, operand: target}
		info, statErr := os.Stat(targetHost)
		if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
			return cannotStat(target, statErr)
		}
		if !options.has('t') && len(sources) == 1 && (statErr != nil || !info.IsDir() || run.noTargetDir) {
			sourceHost, err := resolveHostPath(view, sources[0])
			if err != nil {
				return err
			}
			if run.noTargetDir && statErr == nil && info.IsDir() {
				if source, err := os.Stat(sourceHost); err == nil && !source.IsDir() {
					return fmt.Errorf("'%s' is a directory", target)
				}
			}
			run.move(pathOperand{host: sourceHost, operand: sources[0]}, into, statErr == nil)
			return run.status()
		}
		if statErr != nil || !info.IsDir() {
			return fmt.Errorf("target '%s' is not a directory", target)
		}
		for _, source := range sources {
			if err := ctx.Err(); err != nil {
				return err
			}
			sourceHost, err := resolveHostPath(view, source)
			if err != nil {
				run.fail(err)
				continue
			}
			name := lastPathComponent(source)
			dest := pathOperand{host: joinHost(targetHost, name), operand: joinOperand(target, name)}
			_, destErr := os.Stat(dest.host)
			if destErr != nil && !errors.Is(destErr, fs.ErrNotExist) {
				run.fail(cannotStat(dest.operand, destErr))
				continue
			}
			run.move(pathOperand{host: sourceHost, operand: source}, dest, destErr == nil)
		}
		return run.status()
	}}
}

// newerThan is -u's question, as cp -u asks it: whether source was changed after dest. A stat
// that fails leaves the move to say why.
func newerThan(source, dest string) bool {
	sourceInfo, sourceErr := os.Lstat(source)
	destInfo, destErr := os.Stat(dest)
	return sourceErr != nil || destErr != nil || sourceInfo.ModTime().After(destInfo.ModTime())
}

// moveRename is how mv renames; a test makes it say the volumes differ.
var moveRename = renameForMove

// move renames source to dest, and across volumes copies and removes it, as busybox's mv_main
// does. -v's line is printed whatever became of it, as there.
func (r *mvRun) move(source, dest pathOperand, destExists bool) {
	if destExists && r.update && !newerThan(source.host, dest.host) {
		return
	}
	if r.verbose {
		defer fmt.Fprintf(r.stdout, "'%s' -> '%s'\n", source.operand, dest.operand)
	}
	if destExists {
		if r.noClobber {
			return
		}
		if !r.force && (r.interactive || r.terminal && !r.writable(dest)) {
			fmt.Fprintf(r.stderr, "mv: overwrite '%s'? ", dest.operand)
			if !askYes(r.stdin) {
				return
			}
		}
	}
	err := moveRename(source.host, dest.host)
	if err == nil {
		return
	}
	sourceInfo, statErr := os.Lstat(source.host)
	if !isCrossDeviceRename(err) || statErr != nil {
		r.fail(cannotRename(source.operand, err))
		return
	}
	if destExists {
		if destInfo, err := os.Stat(dest.host); err == nil && destInfo.IsDir() != sourceInfo.IsDir() {
			if destInfo.IsDir() {
				r.fail(errors.New("cannot overwrite directory with non-directory"))
			} else {
				r.fail(errors.New("cannot overwrite non-directory with directory"))
			}
			return
		}
		if err := removeForOverwrite(dest.host); err != nil && !errors.Is(err, fs.ErrNotExist) {
			r.fail(cannotRemove(dest.operand, err))
			return
		}
	}
	copier := &cpRun{ctx: r.ctx, applet: "mv", flags: cpFlags{recurse: true, preserve: true, move: true}, stderr: r.stderr, stdout: io.Discard, umask: r.umask}
	remover := &rmRun{applet: "mv", recursive: true, force: true, stdout: io.Discard, stderr: r.stderr}
	if !copier.copy(source, dest, true) || !remover.remove(source.host, source.operand) {
		r.failed = true
	}
}

// writable is whether dest may be written, the question asked before replacing it.
func (r *mvRun) writable(dest pathOperand) bool {
	info, err := os.Stat(dest.host)
	return err != nil || canWrite(dest.host, info)
}

// fail names a failure and makes mv's status 1; once Ctrl-C has ended the move, as cpRun.fail,
// it names nothing.
func (r *mvRun) fail(err error) {
	r.failed = true
	if r.ctx == nil || r.ctx.Err() == nil {
		fmt.Fprintf(r.stderr, "mv: %v\n", err)
	}
}

func (r *mvRun) status() error {
	if r.failed {
		return ExitStatus(1)
	}
	return nil
}

// mvLongOptions are the long forms busybox's mv takes that stand for a letter.
const mvLongOptions = "interactive=i force=f no-clobber=n no-target-directory=T verbose=v update=u"

// mvArguments reads mv's options, turning each long one into the letter it stands for.
func mvArguments(ctx context.Context, args []string) (appletOptions, []string, error) {
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
		case longOptionNamed(mvLongOptions, name) != 0 && !valued:
			words = append(words, "-"+string(longOptionNamed(mvLongOptions, name)))
		case name == "target-directory":
			if !valued {
				if index+1 >= len(args) {
					return appletOptions{}, nil, fmt.Errorf("option '%s' requires an argument", arg)
				}
				index++
				value = args[index]
			}
			words = append(words, "-t", value)
		default:
			return appletOptions{}, nil, unknownLongOption(arg)
		}
	}
	options, operands, err := parseAppletOptions(ctx, words, "finTvu", "t")
	if err == nil && options.has('t') && options.has('T') {
		err = errors.New("-t and -T cannot both be given")
	}
	return options, operands, err
}
