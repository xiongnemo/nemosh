package applets_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestP05WaveA_streamingInputApplets_preserveOrdinaryHostFileBehavior(t *testing.T) {
	// Given
	directory := t.TempDir()
	first := filepath.Join(directory, "first.txt")
	second := filepath.Join(directory, "second.txt")
	if err := os.WriteFile(first, []byte("beta one\nalpha two\n"), 0o600); err != nil {
		t.Fatalf("write first fixture: %v", err)
	}
	if err := os.WriteFile(second, []byte("alpha two\nalpha two\n"), 0o600); err != nil {
		t.Fatalf("write second fixture: %v", err)
	}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "cat preserves operand order", args: []string{"cat", first, second}, want: "beta one\nalpha two\nalpha two\nalpha two\n"},
		{name: "wc preserves operand labels", args: []string{"wc", "-l", first, second}, want: "2 " + first + "\n2 " + second + "\n"},
		{name: "head streams host input", args: []string{"head", "-n", "1", first}, want: "beta one\n"},
		{name: "tail streams host input", args: []string{"tail", "-n", "1", first}, want: "alpha two\n"},
		{name: "grep streams host input", args: []string{"grep", "alpha", first}, want: "alpha two\n"},
		{name: "cut streams host input", args: []string{"cut", "-d", " ", "-f", "1", first}, want: "beta\nalpha\n"},
		{name: "sort combines host inputs", args: []string{"sort", first, second}, want: "alpha two\nalpha two\nalpha two\nbeta one\n"},
		{name: "uniq streams host input", args: []string{"uniq", second}, want: "alpha two\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			var stdout bytes.Buffer
			err := runRegisteredApplet(context.Background(), test.args, bytes.NewReader(nil), &stdout)

			// Then
			if err != nil {
				t.Fatalf("run %s: %v", test.args[0], err)
			}
			if got := stdout.String(); got != test.want {
				t.Fatalf("%s output: got %q want %q", test.args[0], got, test.want)
			}
		})
	}
}

func runRegisteredApplet(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer) error {
	applet, ok := applets.DefaultRegistry.Lookup(args[0])
	if !ok {
		return os.ErrNotExist
	}
	return applet.Run(ctx, args[1:], stdin, stdout, io.Discard)
}
