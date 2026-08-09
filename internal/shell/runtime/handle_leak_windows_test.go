//go:build windows

package runtime

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A handle is what a descriptor is on Windows, and it is the resource this shell
// can exhaust with no Go-level evidence: a dropped *os.File is returned by a
// finalizer eventually, a dropped raw handle never is. The count is readable for
// the process itself, so a leak of one per operation is directly visible.
//
// GetProcessHandleCount is a kernel32 export that x/sys/windows does not wrap,
// so it is declared here rather than taking on a dependency for one call.
var procGetProcessHandleCount = kernel32.NewProc("GetProcessHandleCount")

func processHandleCount(t *testing.T) int {
	t.Helper()
	var count uint32
	result, _, err := procGetProcessHandleCount.Call(
		uintptr(windows.CurrentProcess()),
		uintptr(unsafe.Pointer(&count)),
	)
	if result == 0 {
		t.Fatalf("GetProcessHandleCount: %v", err)
	}
	return int(count)
}

// leakedHandles runs the work twice and reports the growth across the second
// run only.
//
// Measuring the first run measures the wrong thing. The Go runtime opens handles
// of its own as it warms up -- threads as the scheduler grows, the file cache --
// and that growth plateaus rather than continuing. Measured here, a first batch
// of 100 redirects moved the count by 36, the next 300 by 0, and a later 900 by
// 18: not proportional to the work, so not a leak. Running the identical batch a
// second time puts the plateau behind the measurement, and what is left is the
// handles that batch genuinely failed to return.
func leakedHandles(t *testing.T, work func()) int {
	t.Helper()
	work()
	before := processHandleCount(t)
	work()
	return processHandleCount(t) - before
}

func TestHandles_surviveRepeatedFileRedirects(t *testing.T) {
	// Given
	directory := filepath.ToSlash(t.TempDir())
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
	})

	// When: two redirects per operation, so a leak of one handle per redirect
	// moves the count by twelve hundred and cannot be mistaken for noise.
	const operations = 600
	growth := leakedHandles(t, func() {
		script := new(strings.Builder)
		for index := range operations {
			fmt.Fprintf(script, "echo %d > %s/out\ncat < %s/out > /dev/null\n", index, directory, directory)
		}
		if status := rt.RunScript(context.Background(), script.String()); status != 0 {
			t.Fatalf("status = %d, stderr = %q", status, stderr.String())
		}
	})

	// Then
	if growth > 16 {
		t.Fatalf("handle count grew by %d across a repeat of %d redirect pairs, after the runtime's own growth had settled", growth, operations)
	}
}

func TestHandles_surviveRepeatedPipelines(t *testing.T) {
	// Given: a pipeline creates handles in pairs and hands them to stages, which
	// is the arrangement most likely to drop one.
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
	})

	// When
	const operations = 300
	growth := leakedHandles(t, func() {
		for index := range operations {
			script := fmt.Sprintf("echo %d | cat | cat | cat > /dev/null\n", index)
			if status := rt.RunScript(context.Background(), script); status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr.String())
			}
		}
	})

	// Then
	if growth > 16 {
		t.Fatalf("handle count grew by %d across a repeat of %d pipelines", growth, operations)
	}
}
