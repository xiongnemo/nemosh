package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var windowsExecutableSuffixes = [...]string{".com", ".exe", ".sh", ".bat", ".cmd"}

func (r Runtime) runExternal(ctx context.Context, args []string) int {
	executable := externalCommandPath(args[0])
	cmd := exec.CommandContext(ctx, executable, args[1:]...)
	cmd.Stdin = r.streams.Stdin
	cmd.Stdout = r.streams.Stdout
	cmd.Stderr = r.streams.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(r.streams.Stderr, "%s: not found\n", args[0])
		return 127
	}
	return 0
}

func externalCommandPath(name string) string {
	path := platformPath(name)
	if filepath.Ext(path) != "" {
		return path
	}
	if hasPathSeparator(path) {
		return firstExecutableCandidate(filepath.Dir(path), filepath.Base(path), path)
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			dir = "."
		}
		if candidate := firstExecutableCandidate(dir, path, ""); candidate != "" {
			return candidate
		}
	}
	return path
}

func firstExecutableCandidate(dir string, base string, fallback string) string {
	for _, suffix := range windowsExecutableSuffixes {
		candidate := filepath.Join(dir, base+suffix)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return fallback
}

func hasPathSeparator(path string) bool {
	return strings.ContainsAny(path, `/\\`) || filepath.VolumeName(path) != ""
}
