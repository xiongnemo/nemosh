package pathmodel

import (
	"errors"
	"fmt"
	"strings"
)

type Path string

type Config struct {
	MountPrefix     string
	AcceptCygdrive  bool
	EnableTmp       bool
	EnableDev       bool
	EnableMountPath bool
}

type Model struct {
	config Config
	cwd    Path
	root   Path
}

var ErrCygdriveDisabled = errors.New("cygdrive paths are disabled")

var ErrNoWindowsPath = errors.New("path has no Windows spelling")

var ErrDriveRelativePath = errors.New("drive-relative Windows paths are not supported")

type HostOnlyUNCError struct {
	Host string
}

func (e HostOnlyUNCError) Error() string {
	return fmt.Sprintf("//%s is not a directory root; use //%s/share", e.Host, e.Host)
}

func DefaultConfig() Config {
	return Config{MountPrefix: "/mnt", EnableTmp: true, EnableDev: true, EnableMountPath: true}
}

func New(config Config, cwd Path) Model {
	canonical := clean(cwd)
	root, _ := currentRoot(canonical)
	return Model{config: config, cwd: canonical, root: root}
}

func (m Model) CWD() Path {
	return m.cwd
}

func (m Model) WithCWD(cwd Path) Model {
	m.cwd = clean(cwd)
	if root, err := currentRoot(m.cwd); err == nil {
		m.root = root
	}
	return m
}

func WindowsPath(path Path) (string, error) {
	s := string(path)
	if strings.HasPrefix(s, "//") {
		parts := strings.Split(strings.Trim(s, "/"), "/")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return "", ErrNoWindowsPath
		}
		return string(clean(path)), nil
	}
	drive, rest, ok := driveShort(s)
	if !ok {
		return "", ErrNoWindowsPath
	}
	if rest == "" || rest == "/" {
		return strings.ToUpper(drive) + ":/", nil
	}
	return strings.ToUpper(drive) + ":" + rest, nil
}

func (m Model) Resolve(input string) (Path, error) {
	path := strings.ReplaceAll(input, "\\", "/")
	if path == "" {
		return m.cwd, nil
	}
	if isWindowsDriveRelativePath(path) {
		return "", ErrDriveRelativePath
	}
	if isWindowsDrivePath(path) {
		return clean(Path("/" + strings.ToLower(path[:1]) + path[2:])), nil
	}
	if strings.HasPrefix(path, "//") {
		return normalizeUNC(path)
	}
	virtualRoot := ""
	if m.config.EnableDev && (path == "/dev" || strings.HasPrefix(path, "/dev/")) {
		virtualRoot = "/dev"
	}
	if m.config.EnableTmp && (path == "/tmp" || strings.HasPrefix(path, "/tmp/")) {
		virtualRoot = "/tmp"
	}
	if virtualRoot != "" {
		canonical := clean(Path(path))
		if canonical == Path(virtualRoot) || strings.HasPrefix(string(canonical), virtualRoot+"/") {
			return canonical, nil
		}
		path = string(canonical)
	}
	if strings.HasPrefix(path, "/cygdrive/") {
		if !m.config.AcceptCygdrive {
			return "", ErrCygdriveDisabled
		}
		candidate := strings.TrimPrefix(path, "/cygdrive")
		if drive, rest, ok := driveShort(candidate); ok {
			return clean(Path("/" + drive + rest)), nil
		}
		return "", fmt.Errorf("malformed cygdrive path %q", input)
	}
	if drive, rest, ok := driveShort(path); ok {
		return clean(Path("/" + drive + rest)), nil
	}
	if m.config.EnableMountPath {
		prefix := m.config.MountPrefix
		if prefix == "" {
			prefix = "/mnt"
		}
		if strings.HasPrefix(path, prefix+"/") {
			candidate := strings.TrimPrefix(path, prefix)
			if drive, rest, ok := driveShort(candidate); ok {
				return clean(Path("/" + drive + rest)), nil
			}
		}
	}
	if strings.HasPrefix(path, "/") {
		if m.root == "" {
			return "", errors.New("current root is not a drive or UNC path")
		}
		if path == "/" {
			return m.root, nil
		}
		return clean(Path(string(m.root) + path)), nil
	}
	return m.Resolve(string(clean(Path(string(m.cwd) + "/" + path))))
}

func (m Model) ResolveWindowsSpelling(input string) (Path, bool, error) {
	path := strings.ReplaceAll(input, "\\", "/")
	_, _, shortDrive := driveShort(path)
	if !isWindowsDrivePath(path) && !isWindowsDriveRelativePath(path) && !strings.HasPrefix(path, "//") && !shortDrive {
		return "", false, nil
	}
	resolved, err := m.Resolve(input)
	return resolved, true, err
}

func isWindowsDrivePath(path string) bool {
	return len(path) >= 3 && isDriveLetter(rune(path[0])) && path[1] == ':' && path[2] == '/'
}

func isWindowsDriveRelativePath(path string) bool {
	return len(path) >= 2 && isDriveLetter(rune(path[0])) && path[1] == ':' && (len(path) == 2 || path[2] != '/')
}

func driveShort(path string) (string, string, bool) {
	if len(path) < 2 || path[0] != '/' || !isDriveLetter(rune(path[1])) {
		return "", "", false
	}
	if len(path) > 2 && path[2] != '/' {
		return "", "", false
	}
	drive := strings.ToLower(path[1:2])
	rest := ""
	if len(path) > 2 {
		rest = path[2:]
	}
	return drive, rest, true
}

func normalizeUNC(path string) (Path, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 1 && parts[0] != "" {
		return "", HostOnlyUNCError{Host: parts[0]}
	}
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", errors.New("malformed UNC path")
	}
	return clean(Path("//" + strings.Join(parts, "/"))), nil
}

func currentRoot(path Path) (Path, error) {
	s := string(path)
	if strings.HasPrefix(s, "//") {
		parts := strings.Split(strings.Trim(s, "/"), "/")
		if len(parts) < 2 {
			return "", errors.New("current UNC root is missing share")
		}
		return Path("//" + parts[0] + "/" + parts[1]), nil
	}
	if drive, _, ok := driveShort(s); ok {
		return Path("/" + drive), nil
	}
	return "", errors.New("current root is not a drive or UNC path")
}

func clean(path Path) Path {
	s := string(path)
	unc := strings.HasPrefix(s, "//")
	parts := strings.Split(s, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if len(cleaned) > rootPartCount(unc, cleaned) {
				cleaned = cleaned[:len(cleaned)-1]
			}
			continue
		}
		cleaned = append(cleaned, part)
	}
	if unc {
		return Path("//" + strings.Join(cleaned, "/"))
	}
	return Path("/" + strings.Join(cleaned, "/"))
}

func rootPartCount(unc bool, parts []string) int {
	if unc && len(parts) >= 2 {
		return 2
	}
	if !unc && len(parts) >= 1 && len(parts[0]) == 1 && isDriveLetter(rune(parts[0][0])) {
		return 1
	}
	return 0
}

func isDriveLetter(r rune) bool {
	return ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
}
