//go:build windows

package proc

import (
	"path/filepath"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The three things that do need a handle, and therefore sometimes cannot be had.
//
// Everything in sample_windows.go comes from the global table and always works. These do not:
// the command line, the executable path and the owning user all require opening the process, and
// Windows refuses that for anything this session does not own. Measured unelevated: 176 of 436
// processes gave up a path, so a third is the realistic hit rate.
//
// So each answer is `(value, ok)` and every caller has a fallback -- the image name, which the
// table always has. A column of blanks for two thirds of the rows would be worse than a column
// that says less.
//
// The results are cached by process identity because a monitor asks again every second, and
// re-attempting four hundred denied opens per refresh would cost more than everything else here
// put together. A denial is cached too: it will not change while the process lives.

// processCommandLineInformation is ProcessCommandLineInformation, the information class that
// answers with a command line given only PROCESS_QUERY_LIMITED_INFORMATION.
//
// Windows 8.1 and later. The older way is to read the PEB out of the process with
// PROCESS_VM_READ, which is a far higher bar and refused for most of what this asks about.
const processCommandLineInformation = 60

// Details is what could be learned about a process beyond the table.
type Details struct {
	// Path is the executable's full path, empty when it could not be read.
	Path string
	// CommandLine is the process's whole command line, empty when refused.
	CommandLine string
	// User is the account the process runs as, empty when refused. Mapped the way `ls -l`
	// maps a file owner: a real user account gives its name and a service identity gives
	// root, which is busybox-w32's uid-0 emulation.
	User string
}

// Command is the best available description of what is running: the command line if it could be
// read, otherwise the image name.
func (d Details) Command(name string) string {
	if d.CommandLine != "" {
		return d.CommandLine
	}
	if d.Path != "" {
		return d.Path
	}
	return name
}

// DetailCache remembers what was learned, and what was refused.
type DetailCache struct {
	mu      sync.Mutex
	entries map[detailKey]Details
}

// detailKey is pid and creation time, because a pid alone is not an identity on Windows: the
// number is reissued, and a cache keyed on it alone would describe one process with another's
// command line.
type detailKey struct {
	pid     int
	created int64
}

// NewDetailCache returns an empty cache.
func NewDetailCache() *DetailCache {
	return &DetailCache{entries: map[detailKey]Details{}}
}

// Lookup returns what is known about a process, asking the system only the first time.
func (c *DetailCache) Lookup(process Process) Details {
	key := detailKey{pid: process.PID, created: process.Created.UnixNano()}
	c.mu.Lock()
	if cached, ok := c.entries[key]; ok {
		c.mu.Unlock()
		return cached
	}
	c.mu.Unlock()
	details := readDetails(process.PID)
	c.mu.Lock()
	c.entries[key] = details
	c.mu.Unlock()
	return details
}

// Forget drops processes that are no longer in the given set, so a long-running monitor does not
// remember every process the machine has ever run.
func (c *DetailCache) Forget(live map[int]bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.entries {
		if !live[key.pid] {
			delete(c.entries, key)
		}
	}
}

// readDetails opens the process once and asks it everything.
//
// One open for three questions, because the open is the expensive and the refusable part.
func readDetails(pid int) Details {
	if pid <= 0 {
		// The idle process is not a process anything can be asked about.
		return Details{}
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return Details{}
	}
	defer windows.CloseHandle(handle)
	return Details{
		Path:        imagePath(handle),
		CommandLine: commandLine(handle),
		User:        processUser(handle),
	}
}

// imagePath is the executable's path, in this shell's spelling.
func imagePath(handle windows.Handle) string {
	buffer := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return ""
	}
	return filepath.ToSlash(windows.UTF16ToString(buffer[:size]))
}

// commandLine is the whole command line as the process was started with.
//
// The buffer is sized by asking first: the call answers STATUS_INFO_LENGTH_MISMATCH with the
// length needed, in the same shape as the system table query.
func commandLine(handle windows.Handle) string {
	var needed uint32
	status := windows.NtQueryInformationProcess(handle, processCommandLineInformation, nil, 0, &needed)
	if needed == 0 {
		return ""
	}
	_ = status
	buffer := make([]byte, needed)
	if err := windows.NtQueryInformationProcess(handle, processCommandLineInformation,
		unsafe.Pointer(&buffer[0]), needed, &needed); err != nil {
		return ""
	}
	text := (*windows.NTUnicodeString)(unsafe.Pointer(&buffer[0]))
	if text.Buffer == nil || text.Length == 0 {
		return ""
	}
	return text.String()
}

// processUser is the account the process runs as, mapped the way `ls -l` maps a file's owner.
//
// The same rule and for the same reason: a real user account gives its name, and a service
// identity or the system gives `root`, which is busybox-w32's uid-0 emulation and what a POSIX
// reader expects to see against a kernel process.
func processUser(handle windows.Handle) string {
	var token windows.Token
	if err := windows.OpenProcessToken(handle, windows.TOKEN_QUERY, &token); err != nil {
		return ""
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return ""
	}
	account, _, accountType, err := user.User.Sid.LookupAccount("")
	if err != nil {
		return ""
	}
	if accountType != windows.SidTypeUser {
		return "root"
	}
	return account
}
