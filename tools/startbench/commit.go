package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A session's commit charge: what Windows has promised the process, resident or not, which is
// what Task Manager's "Commit size" shows and what a machine runs out of. A package can cost it
// without costing a page of RAM -- crypto's FIPS entropy source committed a 32 MiB buffer and never
// touched it -- so it is measured apart from init's allocations and the time a start takes.

// sessionScript holds a session where an interactive one sits: started, and waiting for input.
const sessionScript = "echo ready; read x"

// commitOf starts binary on sessionScript and answers its PrivateUsage once it has said it is
// ready, then lets it go.
func commitOf(binary string) (uint64, error) {
	command := exec.Command(binary, "-c", sessionScript)
	stdin, err := command.StdinPipe()
	if err != nil {
		return 0, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return 0, err
	}
	if err := command.Start(); err != nil {
		return 0, err
	}
	defer command.Wait()
	defer stdin.Close()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if strings.TrimSpace(line) != "ready" {
		return 0, fmt.Errorf("%s -c %q said %q, %v, before it was measured", binary, sessionScript, line, err)
	}
	const access = windows.PROCESS_QUERY_LIMITED_INFORMATION | windows.PROCESS_VM_READ
	process, err := windows.OpenProcess(access, false, uint32(command.Process.Pid))
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(process)
	var counters struct { // PROCESS_MEMORY_COUNTERS_EX
		size, faults                             uint32
		peakWorkingSet, workingSet               uintptr
		peakPaged, paged, peakNonPaged, nonPaged uintptr
		pagefile, peakPagefile, private          uintptr
	}
	counters.size = uint32(unsafe.Sizeof(counters))
	info := windows.NewLazySystemDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")
	if ok, _, err := info.Call(uintptr(process), uintptr(unsafe.Pointer(&counters)), uintptr(counters.size)); ok == 0 {
		return 0, fmt.Errorf("K32GetProcessMemoryInfo: %w", err)
	}
	return uint64(counters.private), nil
}

// reportCommit says head's commit charge, against base's when there is one, and holds it to the
// ceiling.
func reportCommit(o options, out io.Writer) bool {
	head, err := commitOf(o.head)
	if err != nil {
		fmt.Fprintln(out, "commit:", err)
		return false
	}
	line := fmt.Sprintf("commit: head %.1f MiB", mebibytes(head))
	if o.base != "" {
		base, err := commitOf(o.base)
		if err != nil {
			fmt.Fprintln(out, "commit:", err)
			return false
		}
		line += fmt.Sprintf(", base %.1f MiB (%+.1f)", mebibytes(base), mebibytes(head)-mebibytes(base))
	}
	if o.commitCeiling > 0 {
		line += fmt.Sprintf(", ceiling %d MiB", o.commitCeiling)
	}
	fmt.Fprintln(out, line)
	if o.commitCeiling > 0 && mebibytes(head) > float64(o.commitCeiling) {
		fmt.Fprintln(out, "commit grew past the ceiling: something new is committed in every process, touched or not")
		return false
	}
	return true
}

func mebibytes(bytes uint64) float64 { return float64(bytes) / (1 << 20) }
