package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/xiongnemo/nemosh/internal/proc"
)

var windowsExecutableSuffixes = [...]string{".com", ".exe", ".sh", ".bat", ".cmd"}

var errExternalNotFound = errors.New("external command not found")

var errExternalPathNotAbsolute = errors.New("external native path is not absolute")

var errExternalNotExecutable = errors.New("external command is not executable")

func (r Runtime) runExternal(ctx context.Context, args []string) int {
	workingDirectory, err := r.nativeWorkingDirectory()
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s: %v\n", args[0], err)
		return 1
	}
	workingDirectory, err = requireAbsoluteNativePath("working directory", workingDirectory)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s: %v\n", args[0], err)
		return 1
	}
	executable, err := r.externalCommandPath(args[0])
	if err != nil {
		// `shopt -s autocd`: a lone word that names a directory means `cd` to it.
		// Checked here, after the lookup has failed, because a command of that name
		// must still win -- there is a directory called `test` in many trees and
		// `test` is a command.
		if status, handled := r.autoChangeDirectory(args, err); handled {
			return status
		}
		return r.reportLookupFailure(args[0], err)
	}
	executable, err = requireAbsoluteNativePath("executable", executable)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s: %v\n", args[0], err)
		return 1
	}
	executable, launchArgs, err := r.externalLaunchTarget(executable, args[1:])
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s: %v\n", args[0], err)
		if errors.Is(err, errExternalNotFound) {
			return 127
		}
		return 126
	}
	cmd, err := r.externalCommand(ctx, executable, launchArgs)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s: %v\n", args[0], err)
		return 126
	}
	launchDirectory, err := launchWorkingDirectory(workingDirectory)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s: %v\n", args[0], err)
		return 126
	}
	cmd.Dir = launchDirectory
	cmd.Env = r.childEnvironment()
	stdin, err := r.fds.reader(0)
	if err != nil {
		if !errors.Is(err, errDescriptorAbsent) && !errors.Is(err, errDescriptorClosed) {
			fmt.Fprintf(r.streams.Stderr, "%s: stdin: %v\n", args[0], err)
			return 1
		}
	} else {
		leasedStdin, releaseStdin := externalStdin(ctx, stdin)
		cmd.Stdin = leasedStdin
		defer releaseStdin()
	}
	// Hand over the real handle where there is one, so the child inherits the
	// console instead of a pipe this process copies. See externalStreams.
	cmd.Stdout, cmd.Stderr = r.externalStreams()
	runErr := r.startChild(cmd)
	if runErr == nil {
		runErr = cmd.Wait()
	}
	// Recorded whether the child succeeded or failed: the CPU it used is spent
	// either way, and this is the one moment Go reports it.
	r.recordChildCPU(cmd)
	if err := runErr; err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return r.programStatus(exitErr.ProcessState)
		}
		// The copy into a pipe no one reads failed, and the program with it: SIGPIPE's 141.
		// When the shell ignores SIGPIPE the program only saw its write fail, and it exited 0,
		// which a status of its own would have overtaken here.
		if errors.Is(normalizePipelineWriteError(err), errPipelineDownstreamClosed) {
			if r.pipeStage.current() == pipeIgnored {
				return 0
			}
			return brokenPipeStatus
		}
		// A program that demands administrator is present and runnable and still
		// cannot be started from here. That is its own answer, not a generic
		// launch failure, and it is worth its own words.
		if requiresElevation(err) {
			r.report(args[0], elevationDiagnostic(args[0]))
			return 126
		}
		fmt.Fprintf(r.streams.Stderr, "%s: %v\n", args[0], err)
		return 126
	}
	return 0
}

// programStatus is a program's status as a shell reads it: 128+n when signal n ended it, which
// on Windows is in the exit code (see exitCodeSignal). The code was taken whole: `kill $$` from
// a job ended a nemosh with 0x0F000000, and the nemosh waiting for it had that for $?; and on
// Linux and macOS the status of one a signal ended was -1. The signal is said on stderr, as
// busybox says it of the command it waited for, but for INT and PIPE: `Terminated`.
func (r Runtime) programStatus(state *os.ProcessState) int {
	status, signal := processOutcome(state)
	if signal != 0 && signal != 2 && signal != 13 {
		fmt.Fprintln(r.streams.Stderr, proc.SignalWord(signal))
	}
	return status
}

func (r Runtime) externalCommandPath(name string) (string, error) {
	// An empty name is no command anywhere. Joined to a PATH directory it named the directory,
	// and `""` was refused as not executable, 126, where both references say not found.
	if name == "" {
		return "", errExternalNotFound
	}
	if hasPathSeparator(name) {
		resolved, err := r.ResolveNemoshPath(name)
		if err != nil {
			return "", err
		}
		if resolved.Device {
			return "", fmt.Errorf("%s is not executable: %w", resolved.Canonical, errExternalNotExecutable)
		}
		return executableCandidate(resolved.Native)
	}
	pathValue, present := r.vars["PATH"]
	if r.searchPath != "" {
		pathValue, present = r.searchPath, true
	}
	if !present || pathValue == "" {
		return "", errExternalNotFound
	}
	var firstCandidateErr error
	for _, dir := range filepath.SplitList(pathValue) {
		if dir == "" {
			dir = "."
		}
		resolved, err := r.ResolveNemoshPath(dir)
		if err != nil {
			if firstCandidateErr == nil {
				firstCandidateErr = err
			}
			continue
		}
		if resolved.Device {
			continue
		}
		candidate := filepath.Join(resolved.Native, name)
		found, candidateErr := executableCandidate(candidate)
		if candidateErr == nil {
			return found, nil
		}
		if !errors.Is(candidateErr, errExternalNotFound) {
			if firstCandidateErr == nil {
				firstCandidateErr = candidateErr
			}
		}
	}
	if firstCandidateErr != nil {
		return "", firstCandidateErr
	}
	return "", errExternalNotFound
}

func executableCandidate(candidate string) (string, error) {
	absolute, err := requireAbsoluteNativePath("executable", candidate)
	if err != nil {
		return "", err
	}
	executable, err := isExecutableFile(absolute)
	if err != nil {
		return "", err
	}
	if executable {
		return absolute, nil
	}
	if runtime.GOOS != "windows" || hasWindowsExecutableSuffixOrDot(absolute) {
		return "", errExternalNotFound
	}
	var firstSuffixErr error
	for _, suffix := range windowsExecutableSuffixes {
		withSuffix := absolute + suffix
		executable, err := isExecutableFile(withSuffix)
		if err != nil {
			if firstSuffixErr == nil {
				firstSuffixErr = err
			}
			continue
		}
		if executable {
			return withSuffix, nil
		}
	}
	if firstSuffixErr != nil {
		return "", firstSuffixErr
	}
	return "", errExternalNotFound
}

func requireAbsoluteNativePath(kind, path string) (string, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("%s %q: %w", kind, path, errExternalPathNotAbsolute)
	}
	return cleaned, nil
}

func isExecutableFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		// A name that cannot exist does not exist. Windows reports those
		// separately from a missing file, and letting the difference through
		// turned `not found` into a raw CreateFile failure.
		if errors.Is(err, os.ErrNotExist) || isUnusableName(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat executable %q: %w", path, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("executable %q is a directory: %w", path, errExternalNotExecutable)
	}
	if runtime.GOOS != "windows" {
		if info.Mode().Perm()&0o111 == 0 {
			return false, fmt.Errorf("executable %q: %w", path, errExternalNotExecutable)
		}
		return true, nil
	}
	// Windows has no execute bit, so busybox synthesises one from the suffix and,
	// failing that, the file's first bytes (win32/mingw.c:779). Without the sniff
	// a plain notes.txt would be handed to CreateProcess and fail there instead.
	if hasWindowsExecutableSuffix(path) {
		return true, nil
	}
	return hasExecutableFormat(path)
}

func hasPathSeparator(path string) bool {
	if strings.ContainsRune(path, '/') {
		return true
	}
	return runtime.GOOS == "windows" && (strings.ContainsRune(path, '\\') || filepath.VolumeName(path) != "")
}
