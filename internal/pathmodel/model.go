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
}

var ErrCygdriveDisabled = errors.New("cygdrive paths are disabled")

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
	return Model{config: config, cwd: clean(cwd)}
}

func (m Model) Resolve(input string) (Path, error) {
	path := strings.ReplaceAll(input, "\\", "/")
	if path == "" {
		return m.cwd, nil
	}
	if isWindowsDrivePath(path) {
		return clean(Path("/" + strings.ToLower(path[:1]) + path[2:])), nil
	}
	if strings.HasPrefix(path, "//") {
		return normalizeUNC(path)
	}
	if m.config.EnableDev && (path == "/dev" || strings.HasPrefix(path, "/dev/")) {
		return clean(Path(path)), nil
	}
	if m.config.EnableTmp && (path == "/tmp" || strings.HasPrefix(path, "/tmp/")) {
		return clean(Path(path)), nil
	}
	if strings.HasPrefix(path, "/cygdrive/") && !m.config.AcceptCygdrive {
		return "", ErrCygdriveDisabled
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
		root, err := currentRoot(m.cwd)
		if err != nil {
			return "", err
		}
		if path == "/" {
			return root, nil
		}
		return clean(Path(string(root) + path)), nil
	}
	return clean(Path(string(m.cwd) + "/" + path)), nil
}

func isWindowsDrivePath(path string) bool {
	return len(path) >= 2 && isDriveLetter(rune(path[0])) && path[1] == ':'
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
