package runtime

// A command named sh or bash that nothing on PATH answers to is this shell, as both names are
// busybox-w32's own shell: `sh -c 'exit 33'`, `bash build.sh`, `#!/bin/bash` and
// `#!/usr/bin/env bash` were `not found` on a machine without Git, where busybox runs them
// itself. An sh or a bash that PATH does hold still wins -- Git's bash is bash, which this
// shell is not quite -- so a machine with Git runs what it ran, and only what failed changes.
// `command -v bash` names this binary then, since it is what runs.

// fallbackShellNames are the names this shell answers to when PATH has no program by them.
var fallbackShellNames = map[string]bool{"sh": true, "bash": true}

// notOnPath is what a bare name PATH has no program for comes to: not found, but for sh and
// bash, which are this binary, the one a job process runs.
func notOnPath(name string) (string, error) {
	if fallbackShellNames[name] {
		if self, err := jobExecutable(); err == nil {
			return self, nil
		}
	}
	return "", errExternalNotFound
}
