package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

var ErrResolvedNativePathNotAbsolute = errors.New("resolved native path is not absolute")

type PathSettings struct {
	Config  pathmodel.Config
	TmpRoot WorkingDirectory
}

func DefaultPathSettings() PathSettings {
	return PathSettings{Config: pathmodel.DefaultConfig(), TmpRoot: WorkingDirectory(os.TempDir())}
}

func (r Runtime) ResolveNemoshPath(path string) (pathmodel.ResolvedPath, error) {
	if r.initErr != nil {
		return pathmodel.ResolvedPath{}, r.initErr
	}
	resolved, err := r.paths.resolve(path)
	if err != nil {
		return pathmodel.ResolvedPath{}, err
	}
	if !resolved.Device && !filepath.IsAbs(resolved.Native) {
		return pathmodel.ResolvedPath{}, fmt.Errorf("resolved native path %q: %w: %w", resolved.Native, ErrResolvedNativePathNotAbsolute, errExternalPathNotAbsolute)
	}
	return resolved, nil
}

func (r Runtime) nativeWorkingDirectory() (string, error) {
	if r.initErr != nil {
		return "", r.initErr
	}
	return r.paths.nativeWorkingDirectory()
}

func (r Runtime) CanonicalizeNativePath(origin pathmodel.Path, native string) (pathmodel.Path, error) {
	if r.initErr != nil {
		return "", r.initErr
	}
	return r.paths.canonicalizeNativePath(origin, native)
}
