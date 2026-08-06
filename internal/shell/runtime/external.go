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
		if errors.Is(err, errExternalNotFound) {
			fmt.Fprintf(r.streams.Stderr, "%s: not found\n", args[0])
			return 127
		}
		if errors.Is(err, errExternalNotExecutable) {
			fmt.Fprintf(r.streams.Stderr, "%s: %v\n", args[0], err)
			return 126
		}
		fmt.Fprintf(r.streams.Stderr, "%s: %v\n", args[0], err)
		return 1
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
	cmd.Dir = workingDirectory
	cmd.Env = r.env.childEnviron(hostEnvironmentPlatform())
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
	cmd.Stdout = r.streams.Stdout
	cmd.Stderr = r.streams.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return exitErr.ExitCode()
		}
		if errors.Is(normalizePipelineWriteError(err), errPipelineDownstreamClosed) {
			return 0
		}
		fmt.Fprintf(r.streams.Stderr, "%s: %v\n", args[0], err)
		return 126
	}
	return 0
}

func (r Runtime) externalCommandPath(name string) (string, error) {
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
		if errors.Is(err, os.ErrNotExist) {
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
