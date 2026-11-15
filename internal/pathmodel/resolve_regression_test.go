package pathmodel_test

import (
	"testing"

	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

func TestP05WaveA_ModelResolve_preservesDriveCurrentRootWhenTraversingAboveVirtualRoot(t *testing.T) {
	// Given
	model := pathmodel.New(pathmodel.DefaultConfig(), "/d/work").WithCWD("/tmp")

	// When
	got, err := model.Resolve("..")

	// Then
	if err != nil {
		t.Fatalf("expected virtual-root traversal to resolve, got %v", err)
	}
	if got != "/d" {
		t.Fatalf("expected retained drive root %q, got %q", "/d", got)
	}
	native, err := pathmodel.WindowsPath(got)
	if err != nil {
		t.Fatalf("expected retained drive root to have Windows spelling, got %v", err)
	}
	if native != "D:/" {
		t.Fatalf("expected Windows drive root %q, got %q", "D:/", native)
	}
}

// A drive-relative path is relative to the directory its drive was last at, as busybox-w32 has
// it, measured with FRP-6354: after `cd D:/BT; cd C:/Windows`, `cd D:` is D:/BT, `cd C:` stays,
// and `D:..` is D:/. The drive the shell is on is at its current directory, and a drive the
// shell has not been to is at its root. They were refused: `cd D:` said "drive-relative Windows
// paths are not supported".
func TestModelResolve_aDriveRelativePathIsRelativeToWhereItsDriveWasLeft(t *testing.T) {
	// Given: at D:/work, having been at C:/Windows before it
	model := pathmodel.New(pathmodel.DefaultConfig(), "/c/Windows").WithCWD("/c/Users").WithCWD("/c/Windows").WithCWD("/d/work")
	for _, test := range []struct{ input, want string }{
		{"D:", "/d/work"},
		{"d:", "/d/work"},
		{"D:sub", "/d/work/sub"},
		{`D:sub\deeper`, "/d/work/sub/deeper"},
		{"D:..", "/d"},
		{"D:.", "/d/work"},
		{"C:", "/c/Windows"},
		{"C:System32", "/c/Windows/System32"},
		{`C:..\Users`, "/c/Users"},
		{"E:", "/e"},
		{"E:x", "/e/x"},
	} {
		t.Run(test.input, func(t *testing.T) {
			// When
			got, err := model.Resolve(test.input)

			// Then
			if err != nil || string(got) != test.want {
				t.Fatalf("Resolve(%q) = %q, %v; want %q", test.input, got, err, test.want)
			}
		})
	}
}

// Where each drive was left belongs to the model it was recorded in: a copy that goes on
// elsewhere does not move the drives of the one it was copied from.
func TestModelResolve_eachCopyKeepsItsOwnDrives(t *testing.T) {
	// Given
	original := pathmodel.New(pathmodel.DefaultConfig(), "/c/one").WithCWD("/d/x")
	copied := original.WithCWD("/c/two").WithCWD("/d/x")

	// When
	fromOriginal, _ := original.Resolve("C:")
	fromCopy, _ := copied.Resolve("C:")

	// Then
	if fromOriginal != "/c/one" || fromCopy != "/c/two" {
		t.Fatalf("C: is %q in the original and %q in the copy; want /c/one and /c/two", fromOriginal, fromCopy)
	}
	if drives := copied.DriveDirectories(); len(drives) != 2 || drives["c"] != "/c/two" || drives["d"] != "/d/x" {
		t.Fatalf("the copy's drives are %v", drives)
	}
}

func TestP05WaveA_ModelResolve_preservesUNCCurrentRootWhenTraversingAboveVirtualRoot(t *testing.T) {
	// Given
	model := pathmodel.New(pathmodel.DefaultConfig(), "//server/share/work").WithCWD("/tmp")

	// When
	got, err := model.Resolve("..")

	// Then
	if err != nil {
		t.Fatalf("expected virtual-root traversal to resolve, got %v", err)
	}
	if got != "//server/share" {
		t.Fatalf("expected retained UNC root %q, got %q", "//server/share", got)
	}
	native, err := pathmodel.WindowsPath(got)
	if err != nil {
		t.Fatalf("expected retained UNC root to have Windows spelling, got %v", err)
	}
	if native != "//server/share" {
		t.Fatalf("expected Windows UNC root %q, got %q", "//server/share", native)
	}
}

func TestP05WaveA_ModelResolve_preservesDriveCurrentRootWhenDirectVirtualPathEscapes(t *testing.T) {
	// Given
	model := pathmodel.New(pathmodel.DefaultConfig(), "/d/work")
	tests := []struct {
		name  string
		input string
		want  pathmodel.Path
	}{
		{name: "parent", input: "/tmp/..", want: "/d"},
		{name: "parent child", input: "/tmp/../child", want: "/d/child"},
		{name: "repeated parents", input: "/tmp/../../../child", want: "/d/child"},
		{name: "dev parent child", input: "/dev/../child", want: "/d/child"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got, err := model.Resolve(tt.input)

			// Then
			if err != nil {
				t.Fatalf("expected direct virtual traversal to resolve, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected retained drive path %q, got %q", tt.want, got)
			}
		})
	}
}

func TestP05WaveA_ModelResolve_preservesUNCCurrentRootWhenDirectVirtualPathEscapes(t *testing.T) {
	// Given
	model := pathmodel.New(pathmodel.DefaultConfig(), "//server/share/work")
	tests := []struct {
		name  string
		input string
		want  pathmodel.Path
	}{
		{name: "parent", input: "/tmp/..", want: "//server/share"},
		{name: "parent child", input: "/tmp/../child", want: "//server/share/child"},
		{name: "repeated parents", input: "/tmp/../../../child", want: "//server/share/child"},
		{name: "dev parent child", input: "/dev/../child", want: "//server/share/child"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got, err := model.Resolve(tt.input)

			// Then
			if err != nil {
				t.Fatalf("expected direct virtual traversal to resolve, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected retained UNC path %q, got %q", tt.want, got)
			}
		})
	}
}

func TestP05WaveA_ModelResolve_preservesVirtualIdentityWhenDirectPathRemainsInsideVirtualRoot(t *testing.T) {
	// Given
	model := pathmodel.New(pathmodel.DefaultConfig(), "/d/work")
	tests := []struct {
		input string
		want  pathmodel.Path
	}{
		{input: "/tmp", want: "/tmp"},
		{input: "/tmp/file", want: "/tmp/file"},
		{input: "/dev", want: "/dev"},
		{input: "/dev/null", want: "/dev/null"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// When
			got, err := model.Resolve(tt.input)

			// Then
			if err != nil {
				t.Fatalf("expected virtual path to resolve, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected virtual path %q, got %q", tt.want, got)
			}
		})
	}
}
