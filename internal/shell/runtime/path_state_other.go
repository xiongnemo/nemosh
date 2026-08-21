//go:build !windows

package runtime

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

type pathState struct {
	model     pathmodel.Model
	config    pathmodel.Config
	cwd       WorkingDirectory
	nativeCwd string
	tmpRoot   string
	tmpCwd    bool
}

func validateInitialCwd(cwd WorkingDirectory, _ PathSettings) error {
	if cwd == "" {
		return ErrStateCwdRequired
	}
	if !filepath.IsAbs(string(cwd)) {
		return fmt.Errorf("cwd %q: %w", cwd, ErrStateCwdNotAbsolute)
	}
	return nil
}

func newPathState(cwd WorkingDirectory, settings PathSettings) pathState {
	nativeCwd := filepath.Clean(string(cwd))
	return pathState{
		model:     pathmodel.New(settings.Config, "/c"),
		config:    settings.Config,
		cwd:       WorkingDirectory(filepath.ToSlash(nativeCwd)),
		nativeCwd: nativeCwd,
		tmpRoot:   filepath.Clean(string(settings.TmpRoot)),
	}
}

func (s pathState) resolve(input string) (pathmodel.ResolvedPath, error) {
	canonical, policyPath, err := s.canonicalPath(input)
	if err != nil {
		return pathmodel.ResolvedPath{}, err
	}
	text := string(canonical)
	// `/dev` is *not* intercepted here, and that is the whole point of this being the
	// non-Windows file.
	//
	// These systems have a real /dev with hundreds of entries in it, so a synthetic one would
	// shadow the genuine article: `ls /dev` would answer with the eight names this shell
	// invents instead of the machine's devices, and `cat /dev/sda` would stop working. The
	// device model exists because *Windows* has no /dev; where the platform provides one, the
	// platform's is the right answer and the ordinary filesystem path reaches it.
	//
	// CI made the case better than the argument: with this interception in place, the
	// completion tests listed the real /dev on ubuntu and macos -- /dev/loop0, /dev/nvme0n1,
	// three hundred ttys -- through a code path meant to serve eight synthetic names.
	native := filepath.Clean(input)
	if !filepath.IsAbs(input) && !policyPath {
		native = filepath.Join(s.nativeCwd, input)
		if s.tmpCwd {
			native = s.nativePath(canonical)
		}
	}
	if s.config.EnableTmp && (text == "/tmp" || strings.HasPrefix(text, "/tmp/")) && (filepath.IsAbs(input) || s.tmpCwd) {
		native = s.nativePath(canonical)
	}
	if native == "" {
		return pathmodel.ResolvedPath{}, fmt.Errorf("path is empty")
	}
	return pathmodel.ResolvedPath{Canonical: canonical, Native: filepath.Clean(native)}, nil
}

func (s pathState) canonicalPath(input string) (pathmodel.Path, bool, error) {
	path := filepath.ToSlash(input)
	if s.config.EnableTmp && (path == "/tmp" || strings.HasPrefix(path, "/tmp/")) {
		cleaned := filepath.ToSlash(filepath.Clean(path))
		if cleaned != "/tmp" && !strings.HasPrefix(cleaned, "/tmp/") {
			return pathmodel.Path(cleaned), false, nil
		}
	}
	if s.isPolicyPath(input) {
		canonical, err := s.model.Resolve(input)
		return canonical, true, err
	}
	if filepath.IsAbs(input) {
		return pathmodel.Path(filepath.ToSlash(filepath.Clean(input))), false, nil
	}
	return pathmodel.Path(filepath.ToSlash(filepath.Clean(filepath.Join(string(s.cwd), input)))), false, nil
}

func (s pathState) isPolicyPath(input string) bool {
	path := filepath.ToSlash(input)
	if strings.HasPrefix(path, "/cygdrive/") {
		return true
	}
	if s.config.EnableTmp && (path == "/tmp" || strings.HasPrefix(path, "/tmp/")) {
		return true
	}
	// No /dev here either, for the reason resolve gives: this platform has its own.
	prefix := s.config.MountPrefix
	if prefix == "" {
		prefix = "/mnt"
	}
	return s.config.EnableMountPath && hasDriveAlias(strings.TrimPrefix(path, prefix))
}

func hasDriveAlias(path string) bool {
	return len(path) >= 2 && path[0] == '/' && isDriveLetter(rune(path[1])) && (len(path) == 2 || path[2] == '/')
}

func (s pathState) nativePath(canonical pathmodel.Path) string {
	text := string(canonical)
	if s.config.EnableTmp && (text == "/tmp" || strings.HasPrefix(text, "/tmp/")) {
		relative := strings.TrimPrefix(text, "/tmp")
		if relative == "" {
			return s.tmpRoot
		}
		return filepath.Join(s.tmpRoot, filepath.FromSlash(strings.TrimPrefix(relative, "/")))
	}
	return filepath.FromSlash(text)
}

func (s pathState) workingDirectory() string {
	return string(s.cwd)
}

func (s pathState) nativeWorkingDirectory() (string, error) {
	if s.nativeCwd == "" {
		return "", fmt.Errorf("working directory is empty")
	}
	return s.nativeCwd, nil
}

func (s *pathState) setWorkingDirectory(resolved pathmodel.ResolvedPath) {
	s.cwd = WorkingDirectory(filepath.ToSlash(filepath.Clean(string(resolved.Canonical))))
	s.tmpCwd = s.config.EnableTmp && (s.cwd == "/tmp" || strings.HasPrefix(string(s.cwd), "/tmp/"))
	s.nativeCwd = filepath.Clean(resolved.Native)
}

func (s pathState) canonicalizeNativePath(origin pathmodel.Path, native string) (pathmodel.Path, error) {
	if s.config.EnableTmp && (origin == "/tmp" || strings.HasPrefix(string(origin), "/tmp/")) {
		originRelative, originErr := filepath.Rel(s.tmpRoot, filepath.FromSlash(string(origin)))
		originUsesBacking := originErr == nil && pathWithinRoot(originRelative)
		relative, err := filepath.Rel(s.tmpRoot, native)
		if !originUsesBacking && err == nil && pathWithinRoot(relative) {
			if relative == "." {
				return "/tmp", nil
			}
			return pathmodel.Path("/tmp/" + filepath.ToSlash(relative)), nil
		}
	}
	return pathmodel.Path(filepath.ToSlash(filepath.Clean(native))), nil
}

func pathWithinRoot(relative string) bool {
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
