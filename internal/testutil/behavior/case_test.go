package behavior_test

import (
	"os"
	"path/filepath"
	"testing"

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
