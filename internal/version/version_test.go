package version

import (
	"strings"
	"testing"
)

func TestSanitizeBranch_replacesWhatAVersionStringCannotCarry(t *testing.T) {
	for _, test := range []struct {
		name   string
		branch string
		want   string
	}{
		{name: "plain", branch: "master", want: "master"},
		{name: "slashes become dashes", branch: "feature/v1a-license", want: "feature-v1a-license"},
		{name: "spaces become dashes", branch: "my branch", want: "my-branch"},
		{name: "path separators on either platform", branch: `a\b/c`, want: "a-b-c"},
		{name: "characters an artifact name cannot hold", branch: "v1:*?\"<>|x", want: "v1-------x"},
		{name: "runs are not collapsed, so the mapping stays reversible by eye", branch: "a//b", want: "a--b"},
		{name: "empty stays empty", branch: "", want: ""},
		{name: "unicode is kept", branch: "分支", want: "分支"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := sanitizeBranch(test.branch)

			// Then
			if got != test.want {
				t.Fatalf("sanitizeBranch(%q) = %q, want %q", test.branch, got, test.want)
			}
		})
	}
}

func TestShortCommit_takesTwelveCharacters(t *testing.T) {
	for _, test := range []struct {
		name   string
		commit string
		want   string
	}{
		{name: "a full sha is cut to twelve", commit: "38e8ec9a1b2c3d4e5f60718293a4b5c6d7e8f900", want: "38e8ec9a1b2c"},
		{name: "exactly twelve is unchanged", commit: "38e8ec9a1b2c", want: "38e8ec9a1b2c"},
		{name: "shorter is left alone rather than padded", commit: "38e8ec9", want: "38e8ec9"},
		{name: "empty stays empty", commit: "", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := shortCommit(test.commit)

			// Then
			if got != test.want {
				t.Fatalf("shortCommit(%q) = %q, want %q", test.commit, got, test.want)
			}
		})
	}
}

// The format is `v{major}.{minor}.{patch}-{branch}-{commit12}[-dirty]`, and the
// patch number is the tag's patch plus the commits since that tag, so it
// advances by itself and is never hand-maintained (AGENTS.md, Versioning).
func TestFormat_buildsTheDocumentedVersionString(t *testing.T) {
	for _, test := range []struct {
		name  string
		build Build
		want  string
	}{
		{
			name:  "on the tagged commit itself",
			build: Build{Tag: "v0.1.0", CommitsSinceTag: 0, Branch: "master", Commit: "38e8ec9a1b2c3d4e"},
			want:  "v0.1.0-master-38e8ec9a1b2c",
		},
		{
			name:  "one commit past the tag advances the patch",
			build: Build{Tag: "v0.1.0", CommitsSinceTag: 1, Branch: "master", Commit: "38e8ec9a1b2c3d4e"},
			want:  "v0.1.1-master-38e8ec9a1b2c",
		},
		{
			name:  "many commits past the tag",
			build: Build{Tag: "v0.1.0", CommitsSinceTag: 27, Branch: "master", Commit: "38e8ec9a1b2c3d4e"},
			want:  "v0.1.27-master-38e8ec9a1b2c",
		},
		{
			name:  "the tag's own patch is the base, not zero",
			build: Build{Tag: "v1.2.3", CommitsSinceTag: 4, Branch: "master", Commit: "38e8ec9a1b2c3d4e"},
			want:  "v1.2.7-master-38e8ec9a1b2c",
		},
		{
			name:  "a dirty tree is marked",
			build: Build{Tag: "v0.1.0", CommitsSinceTag: 2, Branch: "master", Commit: "38e8ec9a1b2c3d4e", Dirty: true},
			want:  "v0.1.2-master-38e8ec9a1b2c-dirty",
		},
		{
			name:  "the branch is sanitized in place",
			build: Build{Tag: "v0.1.0", CommitsSinceTag: 1, Branch: "feature/x y", Commit: "38e8ec9a1b2c3d4e"},
			want:  "v0.1.1-feature-x-y-38e8ec9a1b2c",
		},
		{
			name:  "no tag at all falls back to v0.0.1 as the base",
			build: Build{Tag: "", CommitsSinceTag: 5, Branch: "master", Commit: "38e8ec9a1b2c3d4e"},
			want:  "v0.0.6-master-38e8ec9a1b2c",
		},
		{
			name:  "a prerelease tag is not a base, so the fallback is used",
			build: Build{Tag: "v0.1.2-dev-abcdef123456", CommitsSinceTag: 3, Branch: "master", Commit: "38e8ec9a1b2c3d4e"},
			want:  "v0.0.4-master-38e8ec9a1b2c",
		},
		{
			name:  "an unknown commit still produces a usable string",
			build: Build{Tag: "v0.1.0", CommitsSinceTag: 0, Branch: "master"},
			want:  "v0.1.0-master-unknown",
		},
		{
			name:  "an unknown branch still produces a usable string",
			build: Build{Tag: "v0.1.0", CommitsSinceTag: 0, Commit: "38e8ec9a1b2c3d4e"},
			want:  "v0.1.0-unknown-38e8ec9a1b2c",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := test.build.String()

			// Then
			if got != test.want {
				t.Fatalf("Build.String() = %q, want %q", got, test.want)
			}
		})
	}
}

// A version string must never be empty, whatever the build knew, because it is
// what a bug report quotes.
func TestString_isNeverEmpty_whenNothingIsKnown(t *testing.T) {
	// When
	got := Build{}.String()

	// Then
	if got == "" {
		t.Fatal("Build{}.String() is empty")
	}
	if !strings.HasPrefix(got, "v0.0.1-") {
		t.Fatalf("Build{}.String() = %q, want the v0.0.1 fallback base", got)
	}
}

// Current() must work without ldflags, which is what `go run` and a plain
// `go build` do, and without a terminal.
func TestCurrent_reportsSomethingUsable_withoutLdflags(t *testing.T) {
	// When
	got := Current()

	// Then
	if got.String() == "" {
		t.Fatal("Current().String() is empty")
	}
	if !strings.HasPrefix(got.String(), "v") {
		t.Fatalf("Current().String() = %q, want a leading v", got.String())
	}
}

func TestDescribe_namesTheRuntimeItWasBuiltFor(t *testing.T) {
	// When
	got := Describe()

	// Then
	for _, want := range []string{"nemosh", runtimeGOOS, runtimeGOARCH} {
		if !strings.Contains(got, want) {
			t.Fatalf("Describe() = %q, want it to contain %q", got, want)
		}
	}
}
