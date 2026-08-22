package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/completions"
	"github.com/xiongnemo/nemosh/internal/completionspec"
)

// What the word being typed is, decided from the words before it.
//
// This is the knowledge zsh keeps in a per-command argument grammar and
// bash-completion hand-writes as a `case $prev in`. Getting it wrong is not a
// missing answer but a wrong one: `ssh -i <TAB>` offering the machines in
// ~/.ssh/config, or `ssh <TAB>` offering the files in the current directory.
func TestOperandTargetFor(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   operandTarget
	}{
		{name: "the first operand of ssh is a host", prefix: "ssh ", want: targetHost},
		{name: "and still is after a flag", prefix: "ssh -v ", want: targetHost},
		{
			// -i takes a file, which is the case the whole mechanism exists for.
			name: "an option whose argument is a file", prefix: "ssh -i ", want: targetPath,
		},
		{
			// -p takes a port. Nothing here can guess one, and offering the
			// files in the current directory would be an answer from the wrong
			// universe -- worse than no answer.
			name: "an option whose argument is neither", prefix: "ssh -p ", want: targetUnknown,
		},
		{
			// The value belongs to -i and is not the host, so the host is still
			// wanted. Counting it as an operand made `ssh -i key ` stop offering
			// hosts, which is how this case got written.
			name: "after an option and its value", prefix: "ssh -i key ", want: targetHost,
		},
		{
			// Attached, so the word after it is an operand again.
			name: "an attached option value", prefix: "ssh -ikey ", want: targetHost,
		},
		{
			// `ssh host command` runs command on the far side. bash-completion
			// answers it by opening a connection; this shell will not.
			name: "the second operand is a remote command", prefix: "ssh myhost ", want: targetUnknown,
		},
		{name: "an ordinary command is unaffected", prefix: "ls ", want: targetPath},
		{name: "cd is unaffected", prefix: "cd ", want: targetPath},
		{name: "a command with no row at all", prefix: "nosuchcommand ", want: targetPath},
		{
			// From completions/adb.toml, which is data rather than a compiled-in
			// row -- so this is the loader as much as the rule.
			name: "a command whose first operand is a subcommand", prefix: "adb ", want: targetSubcommand,
		},
		{name: "and after the subcommand, its own operand kind", prefix: "adb install ", want: targetPath},
		{name: "a subcommand that takes nothing", prefix: "adb devices ", want: targetUnknown},
		{name: "after a pipe, the new command decides", prefix: "ls | ssh ", want: targetHost},
		{name: "nothing typed yet", prefix: "", want: targetPath},
		// The long spellings, and every one of these is chosen so that the right
		// answer differs from the wrong one. The specs have carried value-long
		// and file-long since they were written -- curl declares a hundred and
		// eighty of the first and thirty of the second -- the loader validates
		// them, and nothing read them: detachedValueOption required a word of
		// exactly two characters, so `-o` was recognised and `--output` was not.
		//
		// The first five cases written here were useless and are worth naming as
		// a warning. They used wget, whose operand is a path, so "offer this
		// directory's files" is the right answer whether the option is
		// understood or falls through as an operand. Four of the five passed
		// against the broken code.
		{
			// fastboot takes no operand, so the fallthrough answer is nothing
			// at all -- and --dtb is file-long, so the answer is files.
			name:   "a long option whose argument is a file",
			prefix: "fastboot --dtb ", want: targetPath,
		},
		{
			// value-long but not file-long: it takes a header, and offering
			// files for one is an answer from the wrong universe, exactly as
			// it is for ssh -p.
			name:   "a long option whose argument is neither",
			prefix: "wget --header ", want: targetUnknown,
		},
		{
			// The value belongs to the option and is not an operand. adb's
			// first operand is a subcommand, so miscounting the value costs
			// the subcommand list -- the long spelling of `ssh -i key`.
			name:   "after a long option and its value",
			prefix: "adb --one-device serial ", want: targetSubcommand,
		},
		{
			// `--name=value` carries its own value, so the word after it is an
			// operand again: `install` is the subcommand, and its own operand
			// kind is a path. The long form of `ssh -ikey`.
			name:   "an attached long option value",
			prefix: "adb --one-device=serial install ", want: targetPath,
		},
		{
			// A long option that takes nothing leaves the operand where it was.
			name:   "a long flag with no value",
			prefix: "wget --spider ", want: targetPath,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := operandTargetFor(testSpecs(t), test.prefix)

			// Then
			if got != test.want {
				t.Fatalf("operandTargetFor(%q) = %v, want %v", test.prefix, got, test.want)
			}
		})
	}
}

func TestCompleteHost(t *testing.T) {
	hosts := []string{"build", "build-eu", "nemo-desktop", "prod"}
	tests := []struct {
		name string
		stem string
		want []string
	}{
		{name: "a prefix", stem: "bu", want: []string{"build", "build-eu"}},
		{name: "everything", stem: "", want: hosts},
		{
			// The login name is not something this shell has a list of, so it is
			// carried through and the host after it is what gets completed.
			// fish does the same, through __fish_complete_user_at_hosts.
			name: "after user@", stem: "root@bu", want: []string{"root@build", "root@build-eu"},
		},
		{name: "nothing matches", stem: "zzz", want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := completeHost(hosts, test.stem)

			// Then
			if !slices.Equal(got, test.want) {
				t.Fatalf("completeHost(%q) = %v, want %v", test.stem, got, test.want)
			}
		})
	}
}

// End to end through the editor, which is where the pieces could disagree.
func TestComplete_offersHostsAfterSSH(t *testing.T) {
	// Given
	_, editor := newStyledEditor(t, 80, "", nil)
	// HostName is an address here on purpose: `HostName build.example` would
	// also match `bui`, and then two candidates would be the right answer for a
	// reason that has nothing to do with what this test is about.
	editor.hosts = settledHostIndex(t, writeSSHConfig(t, "Host build\n  HostName 10.0.0.5\nHost prod\n"))
	// A file in the working directory, so a wrong fallback to paths is visible
	// rather than an empty answer that proves nothing.
	if err := os.WriteFile(filepath.Join(editor.workingDirectory, "buildlog.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// When
	for _, r := range "ssh bui" {
		editor.buffer.insert(r)
	}
	editor.complete("$ ")

	// Then: the host, not the file that also starts with `bui`.
	if got := editor.buffer.String(); got != "ssh build " {
		t.Fatalf("buffer = %q, want %q", got, "ssh build ")
	}
}

// And the refusal to fall back. Everywhere else a specific completion that finds
// nothing falls through to paths -- that is what keeps a file named
// `-1.18-windows.xml` reachable. A host is not a path, so here it must not.
func TestComplete_doesNotFallBackToFilesForAHost(t *testing.T) {
	// Given
	_, editor := newStyledEditor(t, 80, "", nil)
	editor.hosts = settledHostIndex(t, writeSSHConfig(t, "Host prod\n"))
	if err := os.WriteFile(filepath.Join(editor.workingDirectory, "zebra.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// When
	for _, r := range "ssh zeb" {
		editor.buffer.insert(r)
	}
	editor.complete("$ ")

	// Then
	if got := editor.buffer.String(); got != "ssh zeb" {
		t.Fatalf("buffer = %q, want it left alone rather than completed to a file", got)
	}
}

// `ssh -i ` is the case the value-option column exists for.
func TestComplete_offersFilesAfterAnOptionThatTakesOne(t *testing.T) {
	// Given
	_, editor := newStyledEditor(t, 80, "", nil)
	editor.hosts = settledHostIndex(t, writeSSHConfig(t, "Host keyserver\n"))
	if err := os.WriteFile(filepath.Join(editor.workingDirectory, "keyfile"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// When
	for _, r := range "ssh -i key" {
		editor.buffer.insert(r)
	}
	editor.complete("$ ")

	// Then: the file, not the host that also starts with `key`.
	if got := editor.buffer.String(); got != "ssh -i keyfile " {
		t.Fatalf("buffer = %q, want %q", got, "ssh -i keyfile ")
	}
}

// The flags themselves, which people do type.
func TestComplete_offersSSHOptions(t *testing.T) {
	// Given
	_, editor := newStyledEditor(t, 80, "", nil)
	editor.hosts = settledHostIndex(t, writeSSHConfig(t, "Host prod\n"))

	// When
	for _, r := range "ssh -J" {
		editor.buffer.insert(r)
	}
	editor.complete("$ ")

	// Then
	if got := editor.buffer.String(); got != "ssh -J " {
		t.Fatalf("buffer = %q, want %q", got, "ssh -J ")
	}
}

func writeSSHConfig(t *testing.T, contents string) []string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".ssh", "config")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return []string{path}
}

func settledHostIndex(t *testing.T, sources []string) *hostIndex {
	t.Helper()
	index := newHostIndex()
	index.refresh(sources)
	deadline := time.Now().Add(10 * time.Second)
	for index.builtFrom() == "" {
		if time.Now().After(deadline) {
			t.Fatal("the host index never finished building")
		}
		time.Sleep(time.Millisecond)
	}
	return index
}

// The grey text has to agree with the Tab key about what the word is. They read
// the same operandTargetFor for exactly that reason -- when the two disagreed
// about case, the result was a suggestion moving while Tab did nothing.
//
// This is zsh-autosuggestions' `completion` strategy in miniature: a suggestion
// for something never typed before, from the one source already in memory.
func TestSuggest_offersAHostAfterSSH(t *testing.T) {
	// The registry has to be here. Since ssh's surface moved out of the compiled
	// table and into completions/ssh.toml, "ssh takes a host" is something only
	// the spec knows -- and a suggester with no registry correctly knows nothing.
	engine := suggester{commands: []string{"ssh"}, hosts: []string{"build", "prod"}, specs: testSpecs(t)}
	tests := []struct {
		name string
		line string
		want string
	}{
		{name: "a host being typed", line: "ssh bu", want: "ild"},
		{name: "after user@", line: "ssh root@pr", want: "od"},
		{name: "no host matches", line: "ssh zz", want: ""},
		{
			// -p takes a port. Tab offers nothing there, so neither may this.
			name: "an option's value is not a host", line: "ssh -p bu", want: "",
		},
		{
			// The second operand is a remote command.
			name: "not the word after the host", line: "ssh prod bu", want: "",
		},
		{name: "not for another command", line: "ls bu", want: ""},
		{
			// Nothing typed toward the word yet: guessing a whole host from a
			// blank is a guess too far, and fish does not do it either.
			name: "not from nothing", line: "ssh ", want: "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := engine.suggest(test.line)

			// Then
			if got != test.want {
				t.Fatalf("suggest(%q) = %q, want %q", test.line, got, test.want)
			}
		})
	}
}

// History still leads. A line already run is a line meant, and it knows things
// the host list cannot -- the flags and the remote command that went with it.
func TestSuggest_prefersHistoryOverTheHostList(t *testing.T) {
	// Given
	engine := suggester{
		history: []string{"ssh build -p 2222"},
		hosts:   []string{"bu", "build"},
		specs:   testSpecs(t),
	}

	// When
	got := engine.suggest("ssh bu")

	// Then
	if got != "ild -p 2222" {
		t.Fatalf("suggest = %q, want the whole line from history", got)
	}
}

// testSpecs is the bundled set with no user directory in front of it, which is
// what a machine with nothing in %APPDATA% has.
func testSpecs(t *testing.T) *completionspec.Registry {
	t.Helper()
	return completionspec.NewRegistry(completions.Files)
}

// The loader, end to end: a spec file is data until something reads it, and this
// is the something. `curl` and `adb` are not commands this shell ships, so
// before completions/ existed neither could complete anything but a filename.
func TestComplete_usesABundledSpec(t *testing.T) {
	for _, test := range []struct {
		name  string
		typed string
		want  string
	}{
		{
			// A long option out of the generated curl file, which was read off
			// curl's own `--help all` rather than transcribed.
			name: "a long option", typed: "curl --resol", want: "curl --resolve ",
		},
		{
			// From adb's subcommand list. Before this, `adb ins` completed the
			// files in the current directory.
			name: "a subcommand", typed: "adb install-mul", want: "adb install-multi",
		},
		{
			// The subcommand's own surface decides the operand: `adb install`
			// takes a path, so the file is offered.
			name: "a path under a subcommand that takes one", typed: "adb install pkg", want: "adb install pkg.apk ",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			_, editor := newStyledEditor(t, 80, "", nil)
			editor.specs = testSpecs(t)
			if err := os.WriteFile(filepath.Join(editor.workingDirectory, "pkg.apk"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}

			// When
			for _, r := range test.typed {
				editor.buffer.insert(r)
			}
			editor.complete("$ ")

			// Then
			if got := editor.buffer.String(); got != test.want {
				t.Fatalf("buffer = %q, want %q", got, test.want)
			}
		})
	}
}

// A user's own file wins. That is the whole answer to a bundled spec being wrong
// for a particular machine -- a curl built without a feature, or one new enough
// to have grown an option since the bundled file was measured.
//
// This used to be written with wget, on the reading that one name can be two
// programs. It cannot be any more: wget is an applet now, so capability.Lookup
// answers for it before any spec is consulted, and the bundled wget.toml was
// removed for the same reason. curl is still external, which is what this test
// needs.
func TestComplete_prefersTheUsersOwnSpec(t *testing.T) {
	// Given: a directory holding a curl spec that claims an option the bundled
	// one does not have
	directory := t.TempDir()
	spec := `[meta]
derived-from = "curl --help all"
tool-version = "curl 8.99.0"
measured-on = "2026-08-16"

[command]
name = "curl"
operand = "path"
long = ["mirror"]
`
	if err := os.WriteFile(filepath.Join(directory, "curl.toml"), []byte(spec), 0o600); err != nil {
		t.Fatal(err)
	}
	_, editor := newStyledEditor(t, 80, "", nil)
	editor.specs = completionspec.NewRegistry(completions.Files, directory)

	// When
	for _, r := range "curl --mir" {
		editor.buffer.insert(r)
	}
	editor.complete("$ ")

	// Then
	if got := editor.buffer.String(); got != "curl --mirror " {
		t.Fatalf("buffer = %q, want the user's spec to have won", got)
	}
}
