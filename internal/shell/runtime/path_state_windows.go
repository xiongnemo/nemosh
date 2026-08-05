//go:build windows

package runtime

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

type pathState struct {
	model   pathmodel.Model
	tmpRoot string
	err     error
}

func validateInitialCwd(cwd WorkingDirectory, settings PathSettings) error {
	if cwd == "" {
		return ErrStateCwdRequired
	}
	path := filepath.ToSlash(string(cwd))
	if settings.Config.EnableDev && (path == "/dev" || strings.HasPrefix(path, "/dev/")) {
		return fmt.Errorf("cwd %q: %w", cwd, ErrStateCwdNeedsRoot)
	}
	if settings.Config.EnableTmp && (path == "/tmp" || strings.HasPrefix(path, "/tmp/")) {
		return fmt.Errorf("cwd %q: %w", cwd, ErrStateCwdNeedsRoot)
	}
	if !filepath.IsAbs(string(cwd)) && !strings.HasPrefix(path, "/") {
		return fmt.Errorf("cwd %q: %w", cwd, ErrStateCwdNotAbsolute)
	}
	if strings.HasPrefix(path, "//") || (filepath.IsAbs(string(cwd)) && !strings.HasPrefix(path, "/")) {
		return nil
	}
	if explicitDriveAlias(path) {
		return nil
	}
	prefix := settings.Config.MountPrefix
	if prefix == "" {
		prefix = "/mnt"
	}
	if settings.Config.EnableMountPath && strings.HasPrefix(path, prefix+"/") && explicitDriveAlias(strings.TrimPrefix(path, prefix)) {
		return nil
	}
	if settings.Config.AcceptCygdrive && strings.HasPrefix(path, "/cygdrive/") && explicitDriveAlias(strings.TrimPrefix(path, "/cygdrive")) {
		return nil
	}
	return fmt.Errorf("cwd %q: %w", cwd, ErrStateCwdNotAbsolute)
}

func explicitDriveAlias(path string) bool {
	return len(path) >= 2 && path[0] == '/' && isDriveLetter(rune(path[1])) && (len(path) == 2 || path[2] == '/')
}

func newPathState(cwd WorkingDirectory, settings PathSettings) pathState {
	seed := pathmodel.New(settings.Config, "/c")
	canonical, err := seed.Resolve(filepath.ToSlash(string(cwd)))
	if err != nil {
		return pathState{model: seed, tmpRoot: string(settings.TmpRoot), err: fmt.Errorf("resolve cwd %q: %w", cwd, err)}
	}
	return pathState{model: seed.WithCWD(canonical), tmpRoot: string(settings.TmpRoot)}
}

func (s pathState) resolve(input string) (pathmodel.ResolvedPath, error) {
	if s.err != nil {
		return pathmodel.ResolvedPath{}, s.err
	}
	canonical, err := s.model.Resolve(input)
	if err != nil {
		return pathmodel.ResolvedPath{}, err
	}
	text := string(canonical)
	if text == "/dev" || strings.HasPrefix(text, "/dev/") {
		return pathmodel.ResolvedPath{Canonical: canonical, Device: true}, nil
	}
	if text == "/tmp" || strings.HasPrefix(text, "/tmp/") {
		relative := strings.TrimPrefix(text, "/tmp")
		native := s.tmpRoot
		if relative != "" {
			native = filepath.Join(native, filepath.FromSlash(strings.TrimPrefix(relative, "/")))
		}
		return pathmodel.ResolvedPath{Canonical: canonical, Native: native}, nil
	}
	native, err := pathmodel.WindowsPath(canonical)
	if err != nil {
		return pathmodel.ResolvedPath{}, err
	}
	return pathmodel.ResolvedPath{Canonical: canonical, Native: native}, nil
}

func (s pathState) workingDirectory() string {
	return string(s.model.CWD())
}

func (s pathState) nativeWorkingDirectory() (string, error) {
	resolved, err := s.resolve(string(s.model.CWD()))
	if err != nil {
		return "", err
	}
	if resolved.Device {
		return "", fmt.Errorf("%s is not a host working directory", resolved.Canonical)
	}
	return resolved.Native, nil
}

func (s *pathState) setWorkingDirectory(resolved pathmodel.ResolvedPath) {
	s.model = s.model.WithCWD(resolved.Canonical)
}

func (s pathState) canonicalizeNativePath(origin pathmodel.Path, native string) (pathmodel.Path, error) {
	if origin == "/tmp" || strings.HasPrefix(string(origin), "/tmp/") {
		relative, err := filepath.Rel(s.tmpRoot, native)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			if relative == "." {
				return "/tmp", nil
			}
			return pathmodel.Path("/tmp/" + filepath.ToSlash(relative)), nil
		}
	}
	return s.model.Resolve(filepath.ToSlash(native))
}
