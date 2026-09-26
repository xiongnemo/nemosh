package oilsspec

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// helperNames are the names the cases run spechelper by: the Python helpers in Oils'
// spec/bin, and foo=bar, a script there that one case finds on PATH.
var helperNames = []string{"argv.py", "printenv.py", "stdout_stderr.py", "read_from_fd.py", "foo=bar"}

// Installed is where Install put what the cases run.
type Installed struct {
	// Helpers is the directory of spechelper's copies, one under each helper's name.
	Helpers string
	// Nemosh is nemosh, in a directory of its own under the name bash, because cases
	// branch on $SH and nemosh is held to bash's answers.
	Nemosh string
}

// Install builds nemosh and spechelper into dir and lays them out as Installed says. The
// Python originals are never put beside the copies: their #! line would win the lookup of
// a bare argv.py over argv.py.exe.
func Install(dir string) (Installed, error) {
	installed := Installed{
		Helpers: filepath.Join(dir, "helpers"),
		Nemosh:  filepath.Join(dir, "nemosh", "bash"+executableSuffix()),
	}
	if err := build("github.com/xiongnemo/nemosh/cmd/nemosh", installed.Nemosh); err != nil {
		return Installed{}, err
	}
	helper := filepath.Join(dir, "spechelper"+executableSuffix())
	if err := build("github.com/xiongnemo/nemosh/internal/testutil/oilsspec/spechelper", helper); err != nil {
		return Installed{}, err
	}
	if err := os.MkdirAll(installed.Helpers, 0o755); err != nil {
		return Installed{}, err
	}
	for _, name := range helperNames {
		if err := copyFile(helper, filepath.Join(installed.Helpers, name+executableSuffix()), 0o755); err != nil {
			return Installed{}, err
		}
	}
	return installed, nil
}

// RepoRoot builds under dir a tree for $REPO_ROOT to name: the Oils skeleton, each of its
// directories empty and its files empty, with the vendored spec/testdata and spec/bin in
// place, whole, and the _tmp/spec-tmp that an Oils checkout has once its suite has run.
// vendored is the copy in tests/oils.
func (u Upstream) RepoRoot(vendored, dir string) error {
	for _, name := range u.Skeleton {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			return err
		}
	}
	for name, file := range u.Files {
		if !strings.HasPrefix(name, "spec/testdata/") && !strings.HasPrefix(name, "spec/bin/") {
			continue
		}
		mode := os.FileMode(0o644)
		if file.Mode == "100755" {
			mode = 0o755
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := copyFile(filepath.Join(vendored, filepath.FromSlash(name)), target, mode); err != nil {
			return err
		}
	}
	return os.MkdirAll(filepath.Join(dir, "_tmp", "spec-tmp"), 0o755)
}

func build(pkg, output string) error {
	command := exec.Command("go", "build", "-o", output, pkg)
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("go build %s: %v\n%s", pkg, err, out)
	}
	return nil
}

func copyFile(from, to string, mode os.FileMode) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.WriteFile(to, data, mode); err != nil {
		return err
	}
	// WriteFile's mode passes through the umask; the mode upstream gave a file does not.
	return os.Chmod(to, mode)
}

func executableSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
