package version

import (
	"strings"
	"testing"
)

// The banner is the kernel's shape and answers what a version number cannot:
// two binaries claiming one commit can still differ, and the first thing worth
// knowing about a misbehaving one is whether it came off CI or a laptop.
func TestDescribeBuild(t *testing.T) {
	tests := []struct {
		name             string
		time, user, host string
		want             string
	}{
		{
			name: "all three", time: "2026-08-16T19:14:44Z", user: "nemo", host: "nemo-g15-5511",
			want: "built by nemo@nemo-g15-5511, 2026-08-16T19:14:44Z",
		},
		{
			// A build that recorded nothing says nothing. `go build ./cmd/nemosh`
			// with no ldflags must not invent a time: a timestamp baked in by
			// default would make two builds of one source differ, and that
			// property is worth more than a line of output.
			name: "nothing recorded", want: "",
		},
		{name: "only a time", time: "2026-08-16T19:14:44Z", want: "built by 2026-08-16T19:14:44Z"},
		{name: "only a user", user: "nemo", want: "built by nemo"},
		{name: "user with no host", time: "t", user: "nemo", want: "built by nemo, t"},
		{name: "host with no user", time: "t", host: "ci", want: "built by ci, t"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			buildTime, buildUser, buildHost = test.time, test.user, test.host
			t.Cleanup(func() { buildTime, buildUser, buildHost = "", "", "" })

			// When
			got := DescribeBuild()

			// Then
			if got != test.want {
				t.Fatalf("DescribeBuild() = %q, want %q", got, test.want)
			}
		})
	}
}

// The first line is what the release workflow and a package manager parse, so
// nothing about how the binary was produced may leak into it.
func TestDescribe_doesNotCarryTheBuildStamp(t *testing.T) {
	// Given
	buildTime, buildUser, buildHost = "2026-08-16T19:14:44Z", "nemo", "laptop"
	t.Cleanup(func() { buildTime, buildUser, buildHost = "", "", "" })

	// When
	line := Describe()

	// Then. Not the user name: it is "nemo" here and the product is "nemosh",
	// so that fragment finds itself. The host and the date are unambiguous.
	for _, fragment := range []string{"laptop", "2026-08-16", "built by"} {
		if strings.Contains(line, fragment) {
			t.Fatalf("Describe() = %q, want it free of %q", line, fragment)
		}
	}
}
