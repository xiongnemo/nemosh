package runtime

import (
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/version"
)

// promptFacts answers what a prompt's escapes stand for; see decodePrompt. \# is one more than
// the commands read so far, drawn or not, as bash 5.3 answers both; \! is the next history
// entry's number when the prompt is drawn, and the running command's in ${var@P}.
func (r Runtime) promptFacts(drawn bool) func(byte) string {
	next := 0
	if drawn {
		next = 1
	}
	return func(escape byte) string {
		switch escape {
		case 'u':
			return r.promptUser()
		case 'h', 'H':
			host, err := os.Hostname()
			if err != nil || host == "" {
				return "host"
			}
			if escape == 'h' {
				host, _, _ = strings.Cut(host, ".")
			}
			return host
		case 'w', 'W':
			return r.promptDirectory(escape == 'W')
		case 's':
			return shellName()
		case 'v', 'V':
			return shellVersion(escape == 'V')
		case '#':
			return strconv.Itoa(r.commandsRun() + 1)
		case '!':
			return strconv.Itoa(max(len(r.history.list())+next, 1))
		case 'j':
			return strconv.Itoa(r.runningJobs())
		case 'l':
			return terminalName(r.streams.Stdin)
		case '$':
			if applets.CurrentUserID() == 0 {
				return "#"
			}
			return "$"
		}
		return ""
	}
}

// countCommand counts one command read at the top level, a script's line or a session's; see
// specialState.commands.
func (r Runtime) countCommand() {
	if r.special != nil {
		r.special.commands.Add(1)
	}
}

func (r Runtime) commandsRun() int {
	if r.special == nil {
		return 0
	}
	return int(r.special.commands.Load())
}

// promptUser is \u: the name belonging to the shell's identity, as bash and busybox take it
// from the passwd entry for the effective uid rather than from $USER -- so an elevated shell
// says root, as `id` does. The variables are for when the identity cannot be told at all.
func (r Runtime) promptUser() string {
	if name := applets.CurrentUserName(); name != "" {
		return name
	}
	for _, name := range []string{"USER", "USERNAME"} {
		if value := r.vars[name]; value != "" {
			return value
		}
	}
	return "user"
}

// promptDirectory is \w, $PWD with $HOME at its start written ~, and with last \W, its last
// component, or ~ when it is $HOME. HOME is read as a path, since on Windows it is spelled
// C:/Users/... where PWD is /c/Users/..., and compared as busybox-w32 compares it there,
// whatever the case.
func (r Runtime) promptDirectory(last bool) string {
	directory := r.vars["PWD"]
	if directory == "" {
		directory = r.WorkingDirectory()
	}
	home := r.vars["HOME"]
	if resolved, err := r.ResolveNemoshPath(home); err == nil && home != "" && !resolved.Device {
		home = string(resolved.Canonical)
	}
	rest, inHome := "", false
	if len(home) > 1 && len(directory) >= len(home) && samePromptPath(directory[:len(home)], home) {
		rest = directory[len(home):]
		inHome = rest == "" || rest[0] == '/'
	}
	switch {
	case last && inHome && rest == "":
		return "~"
	case last && directory != "/" && directory != "//":
		return directory[strings.LastIndexByte(directory, '/')+1:]
	case inHome:
		return "~" + rest
	}
	return directory
}

func samePromptPath(a, b string) bool {
	if goruntime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// shellName is \s: the name the shell was started by, without its directory, and without the
// .exe Windows gives it.
func shellName() string {
	name := filepath.Base(filepath.ToSlash(os.Args[0]))
	if extension := filepath.Ext(name); strings.EqualFold(extension, ".exe") {
		name = strings.TrimSuffix(name, extension)
	}
	return name
}

// shellVersion is \v, major.minor, and \V, with the patch, of this shell: bash names its own.
func shellVersion(full bool) string {
	release, _, _ := strings.Cut(strings.TrimPrefix(version.Current().String(), "v"), "-")
	numbers := strings.SplitN(release, ".", 3)
	for len(numbers) < 3 {
		numbers = append(numbers, "0")
	}
	if full {
		return strings.Join(numbers, ".")
	}
	return numbers[0] + "." + numbers[1]
}

// runningJobs is \j: the jobs not yet finished.
func (r Runtime) runningJobs() int {
	records, _ := r.listedJobs()
	running := 0
	for _, record := range records {
		select {
		case <-record.done:
		default:
			running++
		}
	}
	return running
}

// terminalName is \l: the terminal's device without its directory, as bash has it from
// ttyname, and "tty" when the input is none. Only Linux says what a terminal is called here,
// through /proc; elsewhere it is "tty" too.
func terminalName(input io.Reader) string {
	if input == nil {
		return "tty"
	}
	if _, isTerminal := terminalDescriptor(input); !isTerminal {
		return "tty"
	}
	if target, err := os.Readlink("/proc/self/fd/0"); err == nil && strings.HasPrefix(target, "/dev/") {
		return filepath.Base(target)
	}
	return "tty"
}
