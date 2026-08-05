package pathmodel_test

import (
	"errors"
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

func TestP05WaveA_ModelResolve_rejectsWindowsDriveRelativePaths(t *testing.T) {
	// Given
	model := pathmodel.New(pathmodel.DefaultConfig(), "/d/work")
	tests := []string{
		"C:",
		"c:",
		"C:foo",
		"C:foo/bar",
		"C:.",
		"C:..",
		`C:.\child`,
		`C:..\child`,
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			// When
			_, err := model.Resolve(input)

			// Then
			if err == nil {
				t.Fatalf("expected drive-relative path %q to be rejected", input)
			}
			if !errors.Is(err, pathmodel.ErrDriveRelativePath) {
				t.Fatalf("expected ErrDriveRelativePath for %q, got %v", input, err)
			}
		})
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
