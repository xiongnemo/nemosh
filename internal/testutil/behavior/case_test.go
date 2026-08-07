package behavior_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/testutil/behavior"
)

func TestParseCase_rejectsUnknownField(t *testing.T) {
	// Given
	data := []byte("id = \"x\"\narea = \"shell\"\nkind = \"golden\"\nsemantics = \"posix\"\nplatforms = [\"windows\"]\nscript = \"echo ok\"\nunknown = true\n[expect]\nstatus = 0\nstdout = \"ok\\n\"\nstderr = \"\"\n")

	// When
	_, err := behavior.ParseCase(data)

	// Then
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected unknown-field error, got %v", err)
	}
}

func TestParseCase_rejectsBothScriptAndCommand(t *testing.T) {
	// Given
	data := []byte("id = \"x\"\narea = \"shell\"\nkind = \"golden\"\nsemantics = \"posix\"\nplatforms = [\"windows\"]\nscript = \"echo ok\"\ncommand = [\"echo\"]\n[expect]\nstatus = 0\nstdout = \"\"\nstderr = \"\"\n")

	// When
	_, err := behavior.ParseCase(data)

	// Then
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("expected script/command exclusivity error, got %v", err)
	}
}

func TestParseCase_rejectsUnsafePaths(t *testing.T) {
	for _, field := range []string{"cwd = \"../escape\"", "[files]\n\"/escape\" = \"bad\""} {
		t.Run(field, func(t *testing.T) {
			// Given
			data := []byte("id = \"x\"\narea = \"shell\"\nkind = \"golden\"\nsemantics = \"posix\"\nplatforms = [\"windows\"]\nscript = \"echo ok\"\n" + field + "\n[expect]\nstatus = 0\nstdout = \"\"\nstderr = \"\"\n")

			// When
			_, err := behavior.ParseCase(data)

			// Then
			if err == nil {
				t.Fatal("expected unsafe-path error")
			}
		})
	}
}

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
			// The other two executors honour this; this one did not, so a case
			// scoped to `platforms = ["windows"]` was compared anyway and its
			// empty result read as a mismatch. Invisible until the suite ran on
			// a platform other than Windows for the first time.
			if result.SkipReason != "" {
				t.Skip(result.SkipReason)
			}
			if result.HarnessError != nil {
				t.Fatal(result.HarnessError)
			}
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

func TestBehaviorShellCases_executeAgainstFreshNemosh(t *testing.T) {
	// Given
	absoluteBinary := buildFreshNemosh(t)
	executor := newShellExecutor(absoluteBinary, 10*time.Second)
	runner := behavior.NewRunnerWithScriptExecutor(applets.DefaultRegistry, executor)
	root := filepath.Join("..", "..", "..", "tests", "behavior", "shell")

	// When / Then
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Ext(path) != ".toml" {
			return nil
		}
		t.Run(path, func(t *testing.T) {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			caseData, parseErr := behavior.ParseCase(data)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			result := runner.Run(t.Context(), caseData)
			if result.SkipReason != "" {
				t.Skip(result.SkipReason)
			}
			if result.HarnessError != nil {
				t.Fatal(result.HarnessError)
			}
			if result.Status != caseData.Expect.Status || result.Stdout != caseData.Expect.Stdout || result.Stderr != caseData.Expect.Stderr {
				t.Fatalf("expected (%d, %q, %q), got (%d, %q, %q)", caseData.Expect.Status, caseData.Expect.Stdout, caseData.Expect.Stderr, result.Status, result.Stdout, result.Stderr)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBehaviorAppletScriptCases_executeAgainstFreshNemosh(t *testing.T) {
	// Given
	absoluteBinary := buildFreshNemosh(t)
	executor := newShellExecutor(absoluteBinary, 10*time.Second)
	runner := behavior.NewRunnerWithScriptExecutor(applets.DefaultRegistry, executor)
	root := filepath.Join("..", "..", "..", "tests", "behavior", "applets")

	// When / Then
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Ext(path) != ".toml" {
			return nil
		}
		t.Run(path, func(t *testing.T) {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			caseData, parseErr := behavior.ParseCase(data)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			if caseData.Script == "" {
				t.Skip("not a script case")
			}
			result := runner.Run(t.Context(), caseData)
			if result.SkipReason != "" {
				t.Skip(result.SkipReason)
			}
			if result.HarnessError != nil {
				t.Fatal(result.HarnessError)
			}
			if result.Status != caseData.Expect.Status || result.Stdout != caseData.Expect.Stdout || result.Stderr != caseData.Expect.Stderr {
				t.Fatalf("expected (%d, %q, %q), got (%d, %q, %q)", caseData.Expect.Status, caseData.Expect.Stdout, caseData.Expect.Stderr, result.Status, result.Stdout, result.Stderr)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func buildFreshNemosh(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "nemosh")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/nemosh")
	build.Dir = filepath.Join("..", "..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build nemosh: %v\n%s", err, output)
	}
	absoluteBinary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	return absoluteBinary
}
