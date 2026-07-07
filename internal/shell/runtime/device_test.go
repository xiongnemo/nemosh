package runtime_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_readsNulBytes_whenStdinRedirectsFromDevZero(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	registry := applets.NewRegistry(devZeroProbeApplet{})
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "dev-zero-probe < /dev/zero\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d with stderr %q", status, stderr.String())
	}
	if got := stdout.String(); got != "zero-ok\n" {
		t.Fatalf("expected dev zero output %q, got %q", "zero-ok\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestRuntime_readsRandomBytes_whenStdinRedirectsFromDevURandom(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	registry := applets.NewRegistry(devRandomProbeApplet{})
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "dev-random-probe < /dev/urandom\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d with stderr %q", status, stderr.String())
	}
	if got := stdout.String(); got != "random-ok\n" {
		t.Fatalf("expected dev urandom output %q, got %q", "random-ok\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestRuntime_readsRandomBytes_whenStdinRedirectsFromDevRandom(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	registry := applets.NewRegistry(devRandomProbeApplet{})
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "dev-random-probe < /dev/random\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d with stderr %q", status, stderr.String())
	}
	if got := stdout.String(); got != "random-ok\n" {
		t.Fatalf("expected dev random output %q, got %q", "random-ok\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestRuntime_readsEOF_whenStdinRedirectsFromDevNull(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	registry := applets.NewRegistry(devNullInputProbeApplet{})
	rt := runtime.New(registry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "dev-null-input-probe < /dev/null\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d with stderr %q", status, stderr.String())
	}
	if got := stdout.String(); got != "null-input-ok\n" {
		t.Fatalf("expected dev null input output %q, got %q", "null-input-ok\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestRuntime_discardsStdout_whenStdoutRedirectsToDevNull(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "echo hidden > /dev/null\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d with stderr %q", status, stderr.String())
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestRuntime_readsCurrentStdin_whenStdinRedirectsFromDevStdin(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	registry := applets.NewRegistry(devStdinProbeApplet{})
	rt := runtime.New(registry, runtime.Streams{Stdin: bytes.NewBufferString("stdin-alias"), Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "dev-stdin-probe < /dev/stdin\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d with stderr %q", status, stderr.String())
	}
	if got := stdout.String(); got != "stdin-alias-ok\n" {
		t.Fatalf("expected dev stdin output %q, got %q", "stdin-alias-ok\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestRuntime_writesCurrentStdout_whenStdoutRedirectsToDevStdout(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "echo visible > /dev/stdout\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d with stderr %q", status, stderr.String())
	}
	if got := stdout.String(); got != "visible\n" {
		t.Fatalf("expected stdout %q, got %q", "visible\n", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestRuntime_writesCurrentStderr_whenStdoutRedirectsToDevStderr(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "echo visible > /dev/stderr\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d with stderr %q", status, stderr.String())
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("expected empty stdout, got %q", got)
	}
	if got := stderr.String(); got != "visible\n" {
		t.Fatalf("expected stderr %q, got %q", "visible\n", got)
	}
}

type devZeroProbeApplet struct{}

func (devZeroProbeApplet) Name() string {
	return "dev-zero-probe"
}

func (devZeroProbeApplet) Run(_ context.Context, _ []string, stdin io.Reader, stdout, _ io.Writer) error {
	buf := make([]byte, 4)
	if _, err := io.ReadFull(stdin, buf); err != nil {
		return err
	}
	if !bytes.Equal(buf, []byte{0, 0, 0, 0}) {
		return fmt.Errorf("expected four NUL bytes, got %v", buf)
	}
	_, err := fmt.Fprintln(stdout, "zero-ok")
	return err
}

type devRandomProbeApplet struct{}

func (devRandomProbeApplet) Name() string {
	return "dev-random-probe"
}

func (devRandomProbeApplet) Run(_ context.Context, _ []string, stdin io.Reader, stdout, _ io.Writer) error {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(stdin, buf); err != nil {
		return err
	}
	_, err := fmt.Fprintln(stdout, "random-ok")
	return err
}

type devNullInputProbeApplet struct{}

func (devNullInputProbeApplet) Name() string {
	return "dev-null-input-probe"
}

func (devNullInputProbeApplet) Run(_ context.Context, _ []string, stdin io.Reader, stdout, _ io.Writer) error {
	buf := make([]byte, 1)
	n, err := stdin.Read(buf)
	if n != 0 || err != io.EOF {
		return fmt.Errorf("expected EOF with zero bytes, got n=%d err=%v", n, err)
	}
	_, err = fmt.Fprintln(stdout, "null-input-ok")
	return err
}

type devStdinProbeApplet struct{}

func (devStdinProbeApplet) Name() string {
	return "dev-stdin-probe"
}

func (devStdinProbeApplet) Run(_ context.Context, _ []string, stdin io.Reader, stdout, _ io.Writer) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return err
	}
	if string(data) != "stdin-alias" {
		return fmt.Errorf("expected stdin alias data, got %q", string(data))
	}
	_, err = fmt.Fprintln(stdout, "stdin-alias-ok")
	return err
}
