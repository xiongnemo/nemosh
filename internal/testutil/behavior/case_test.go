package behavior_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/testutil/behavior"
)

func TestBehaviorCaseSamples(t *testing.T) {
	root := filepath.Join("..", "..", "..", "tests", "behavior")
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".toml" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no behavior samples found under %s", root)
	}

	for _, path := range files {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			c, err := behavior.ParseCase(data)
			if err != nil {
				t.Fatal(err)
			}
			for _, problem := range c.Validate() {
				t.Error(problem)
			}
		})
	}
}

func TestBehaviorCommandCases_executeAgainstAppletRegistry(t *testing.T) {
	root := filepath.Join("..", "..", "..", "tests", "behavior", "applets")
	runner := behavior.NewRunner(applets.DefaultRegistry)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".toml" {
			return nil
		}
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			c, err := behavior.ParseCase(data)
			if err != nil {
				t.Fatal(err)
			}
			if len(c.Command) == 0 {
				t.Skip("not a command case")
			}
			result := runner.Run(t.Context(), c)
			if result.Status != c.Expect.Status {
				t.Fatalf("expected status %d, got %d", c.Expect.Status, result.Status)
			}
			if result.Stdout != c.Expect.Stdout {
				t.Fatalf("expected stdout %q, got %q", c.Expect.Stdout, result.Stdout)
			}
			if result.Stderr != c.Expect.Stderr {
				t.Fatalf("expected stderr %q, got %q", c.Expect.Stderr, result.Stderr)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
