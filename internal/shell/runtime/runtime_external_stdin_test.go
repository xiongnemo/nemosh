package runtime_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_externalDoesNotWaitForOpenFileStdinAfterChildExit(t *testing.T) {
	// Given
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("test executable: %v", err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	defer reader.Close()
	defer writer.Close()
	t.Setenv("NEMOSH_RUNTIME_HELPER_PROCESS", "1")
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdin: reader})
	done := make(chan int, 1)

	// When
	go func() {
		done <- rt.RunScript(context.Background(), filepath.ToSlash(executable)+" -test.run=TestRuntimeHelperProcess -- external-ok\n")
	}()

	// Then
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case status := <-done:
		if status != 0 {
			t.Fatalf("status = %d, want 0", status)
		}
	case <-timer.C:
		if err := writer.Close(); err != nil {
			t.Fatalf("close writer after timeout: %v", err)
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("external command did not stop after stdin cleanup")
		}
		t.Fatal("external command waited for another stdin write after child exit")
	}
}
