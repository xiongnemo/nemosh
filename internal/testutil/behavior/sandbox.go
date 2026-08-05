package behavior

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func (r Runner) runScript(ctx context.Context, c Case) Result {
	if r.scriptExecutor == nil {
		return Result{HarnessError: fmt.Errorf("script case %q has no executor", c.ID)}
	}
	root, err := os.MkdirTemp("", "nemosh-behavior-")
	if err != nil {
		return Result{HarnessError: fmt.Errorf("create sandbox: %w", err)}
	}
	defer os.RemoveAll(root)
	for path, content := range c.Files {
		if !safeRelativePath(path) {
			return Result{HarnessError: fmt.Errorf("unsafe file path %q", path)}
		}
		absolute := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			return Result{HarnessError: fmt.Errorf("create fixture directory: %w", err)}
		}
		if err := os.WriteFile(absolute, []byte(content), 0o600); err != nil {
			return Result{HarnessError: fmt.Errorf("write fixture %q: %w", path, err)}
		}
	}
	dir := root
	if c.CWD != "" {
		if !safeRelativePath(c.CWD) {
			return Result{HarnessError: fmt.Errorf("unsafe cwd %q", c.CWD)}
		}
		dir = filepath.Join(root, filepath.FromSlash(c.CWD))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Result{HarnessError: fmt.Errorf("create cwd: %w", err)}
		}
	}
	env := make([]string, 0, len(c.Env))
	for key, value := range c.Env {
		env = append(env, key+"="+value)
	}
	sort.Strings(env)
	process, err := r.scriptExecutor(ctx, ScriptRequest{Script: c.Script, Stdin: c.Stdin, Dir: dir, Env: env})
	if err != nil {
		return Result{HarnessError: fmt.Errorf("execute script: %w", err)}
	}
	return Result{Status: process.Status, Stdout: process.Stdout, Stderr: process.Stderr}
}
