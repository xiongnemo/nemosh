package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A shell that has started loads no DLL it has not used: naming the user for USER and LOGNAME
// went through os/user.Current, which loads userenv and profapi for a profile directory nothing
// read, and secur32 and sspicli for a display name nothing printed -- about 2.5 ms of every start,
// `nemosh cat` and every job's included, and 1.2 MB of working set, measured on 2026-11-19. The
// elevation behind $UID is asked of ntdll, which every process has, so an unelevated start does not
// load advapi32 either; an elevated one asks advapi32 whether it is an administrator.
func TestStartup_loadsNoDLLItHasNotUsed(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the shell")
	}
	binary := filepath.Join(t.TempDir(), "nemosh-startup.exe")
	build := exec.Command("go", "build", "-o", binary, "github.com/xiongnemo/nemosh/cmd/nemosh")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the shell: %v\n%s", err, output)
	}
	shell := exec.Command(binary, "-c", "echo ready; read x")
	environment := []string{}
	for _, variable := range os.Environ() {
		// USER and LOGNAME unset, as in a window Windows Terminal opens, so they are worked out.
		if name, _, _ := strings.Cut(variable, "="); !strings.EqualFold(name, "USER") && !strings.EqualFold(name, "LOGNAME") {
			environment = append(environment, variable)
		}
	}
	shell.Env = environment
	stdin, err := shell.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := shell.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := shell.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stdin.Close()
		shell.Wait()
	}()
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || strings.TrimSpace(line) != "ready" {
		t.Fatalf("the shell said %q, %v", line, err)
	}
	loaded := loadedModules(t, shell.Process.Pid)
	banned := []string{"userenv.dll", "profapi.dll", "secur32.dll", "sspicli.dll"}
	if !windows.GetCurrentProcessToken().IsElevated() {
		banned = append(banned, "advapi32.dll")
	}
	for _, name := range banned {
		if loaded[name] {
			t.Errorf("a started shell has %s loaded", name)
		}
	}
}

// loadedModules is the lower-cased base names of the DLLs a process has loaded.
func loadedModules(t *testing.T, pid int) map[string]bool {
	t.Helper()
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(process)
	modules := make([]windows.Handle, 512)
	var needed uint32
	if err := windows.EnumProcessModules(process, &modules[0], uint32(len(modules))*uint32(unsafe.Sizeof(modules[0])), &needed); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, module := range modules[:needed/uint32(unsafe.Sizeof(modules[0]))] {
		var name [260]uint16
		if err := windows.GetModuleBaseName(process, module, &name[0], uint32(len(name))); err == nil {
			names[strings.ToLower(windows.UTF16ToString(name[:]))] = true
		}
	}
	return names
}
