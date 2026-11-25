//go:build windows

package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// An sshd session is a Job Object that ends everything in it when it closes, and allows a child to
// leave: KILL_ON_JOB_CLOSE and BREAKAWAY_OK, Win32-OpenSSH's w32-doexec.c. A command nohup runs
// leaves it, so `nohup server &` over ssh outlives the ssh command, as a ChatGPT client's bootstrap
// expects of `nohup codex app-server ... &`; a command run plainly is ended with the session.
func TestNohup_outlivesTheJobThatEndsItsSession(t *testing.T) {
	binary := os.Getenv(jobBinaryVariable)
	if binary == "" {
		t.Skip("no nemosh was built for this run")
	}
	requireBreakaway(t)
	// Given: a shell in a session's Job Object, which starts a nohup'd shell that writes its pid
	// and waits, and then waits itself.
	dir := t.TempDir()
	pidFile := filepath.ToSlash(filepath.Join(dir, "pid"))
	script := `nohup "$0" -c 'echo $$ >"$1.tmp" && mv "$1.tmp" "$1" && sleep 60' inner "$1" >/dev/null 2>&1 &
while [ ! -e "$1" ]; do sleep 0.05; done
sleep 60`
	shell := exec.Command(binary, "-c", script, binary, pidFile)
	session := startInSessionJob(t, shell, windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK)
	pid := awaitPidFile(t, pidFile)
	survivor, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		t.Fatalf("open the nohup'd shell %d: %v", pid, err)
	}
	defer windows.CloseHandle(survivor)
	defer windows.TerminateProcess(survivor, 1)

	// When: the session ends.
	if err := windows.CloseHandle(session); err != nil {
		t.Fatal(err)
	}
	_ = shell.Wait()

	// Then
	if event, _ := windows.WaitForSingleObject(survivor, 500); event != uint32(windows.WAIT_TIMEOUT) {
		t.Fatal("the nohup'd shell ended with the session")
	}
}

// A session that lets nothing leave it still runs what nohup is given, and says that the session's
// end will end it: a capability that is absent is said, not approximated.
func TestNohup_saysWhenTheSessionLetsNothingLeave(t *testing.T) {
	binary := os.Getenv(jobBinaryVariable)
	if binary == "" {
		t.Skip("no nemosh was built for this run")
	}
	// Given
	var stdout, stderr strings.Builder
	shell := exec.Command(binary, "-c", `nohup "$0" -c 'echo inner'`, binary)
	shell.Stdout, shell.Stderr = &stdout, &stderr
	session := startInSessionJob(t, shell, 0)
	defer windows.CloseHandle(session)

	// When
	err := shell.Wait()

	// Then
	if err != nil || stdout.String() != "inner\n" {
		t.Fatalf("stdout %q, %v; want the command run all the same", stdout.String(), err)
	}
	if !strings.Contains(stderr.String(), "cannot leave this session's Job Object, which will end it") {
		t.Fatalf("stderr = %q, want it to say the session will end the command", stderr.String())
	}
}

// requireBreakaway skips when this test runs in a Job Object that lets nothing leave it, as a CI
// runner's may: a child can leave only the jobs that all allow it.
func requireBreakaway(t *testing.T) {
	t.Helper()
	probe := exec.Command("cmd.exe", "/c", "exit 0")
	probe.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_BREAKAWAY_FROM_JOB}
	if err := probe.Run(); err != nil {
		t.Skipf("this test runs in a Job Object that lets no child leave: %v", err)
	}
}

// startInSessionJob starts cmd suspended, holds it in a Job Object made as sshd makes a session's,
// with the limits given besides ending everything on close, and lets it run; closing the handle
// returned ends the session.
func startInSessionJob(t *testing.T, cmd *exec.Cmd, limits uint32) windows.Handle {
	t.Helper()
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | limits
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		t.Fatal(err)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_SUSPEND_RESUME, false, uint32(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		_ = windows.TerminateProcess(process, 1)
		t.Fatal(err)
	}
	if status, _, _ := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess").Call(uintptr(process)); status != 0 {
		t.Fatalf("resume the shell: status %#x", status)
	}
	return job
}

func awaitPidFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if text, err := os.ReadFile(path); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(text))); err == nil {
				return pid
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no pid in %s", path)
	return 0
}
