package pathmodel_test

import (
	"errors"
	"testing"

	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

func TestModel_resolvesDriveForms_whenDefaultsUsed(t *testing.T) {
	// Given
	model := pathmodel.New(pathmodel.DefaultConfig(), "/d/work")
	tests := []struct {
		name  string
		input string
		want  pathmodel.Path
	}{
		{name: "windows forward", input: "C:/Users/nemo", want: "/c/Users/nemo"},
		{name: "windows backslash", input: `C:\Users\nemo`, want: "/c/Users/nemo"},
		{name: "drive short", input: "/c/Users/nemo", want: "/c/Users/nemo"},
		{name: "upper drive short", input: "/C/Users/nemo", want: "/c/Users/nemo"},
		{name: "mount prefix", input: "/mnt/c/Users/nemo", want: "/c/Users/nemo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got, err := model.Resolve(tt.input)

			// Then
			if err != nil {
				t.Fatalf("expected resolve to succeed, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestModel_resolvesSlashUnderDriveRoot_whenCwdIsDrive(t *testing.T) {
	// Given
	model := pathmodel.New(pathmodel.DefaultConfig(), "/d/work/project")

	// When
	root, rootErr := model.Resolve("/")
	child, childErr := model.Resolve("/tmp-file")

	// Then
	if rootErr != nil {
		t.Fatalf("expected root resolve to succeed, got %v", rootErr)
	}
	if childErr != nil {
		t.Fatalf("expected child resolve to succeed, got %v", childErr)
	}
	if root != "/d" {
		t.Fatalf("expected root %q, got %q", "/d", root)
	}
	if child != "/d/tmp-file" {
		t.Fatalf("expected child %q, got %q", "/d/tmp-file", child)
	}
}

func TestModel_resolvesSlashUnderUNCRoot_whenCwdIsUNC(t *testing.T) {
	// Given
	model := pathmodel.New(pathmodel.DefaultConfig(), "//server/share/work")

	// When
	root, rootErr := model.Resolve("/")
	child, childErr := model.Resolve("/tmp-file")

	// Then
	if rootErr != nil {
		t.Fatalf("expected root resolve to succeed, got %v", rootErr)
	}
	if childErr != nil {
		t.Fatalf("expected child resolve to succeed, got %v", childErr)
	}
	if root != "//server/share" {
		t.Fatalf("expected root %q, got %q", "//server/share", root)
	}
	if child != "//server/share/tmp-file" {
		t.Fatalf("expected child %q, got %q", "//server/share/tmp-file", child)
	}
}

func TestModel_preservesVirtualRoots_whenCwdIsDrive(t *testing.T) {
	// Given
	model := pathmodel.New(pathmodel.DefaultConfig(), "/d/work")

	// When
	dev, devErr := model.Resolve("/dev/null")
	tmp, tmpErr := model.Resolve("/tmp/file")

	// Then
	if devErr != nil {
		t.Fatalf("expected dev resolve to succeed, got %v", devErr)
	}
	if tmpErr != nil {
		t.Fatalf("expected tmp resolve to succeed, got %v", tmpErr)
	}
	if dev != "/dev/null" {
		t.Fatalf("expected dev %q, got %q", "/dev/null", dev)
	}
	if tmp != "/tmp/file" {
		t.Fatalf("expected tmp %q, got %q", "/tmp/file", tmp)
	}
}

func TestModel_returnsHostOnlyUNCError_whenUNCShareMissing(t *testing.T) {
	// Given
	model := pathmodel.New(pathmodel.DefaultConfig(), "/c")

	// When
	_, err := model.Resolve("//server")

	// Then
	var hostErr pathmodel.HostOnlyUNCError
	if !errors.As(err, &hostErr) {
		t.Fatalf("expected HostOnlyUNCError, got %v", err)
	}
}

func TestModel_rejectsCygdrive_whenDefaultConfigUsed(t *testing.T) {
	// Given
	model := pathmodel.New(pathmodel.DefaultConfig(), "/c")

	// When
	_, err := model.Resolve("/cygdrive/c/tmp")

	// Then
	if !errors.Is(err, pathmodel.ErrCygdriveDisabled) {
		t.Fatalf("expected ErrCygdriveDisabled, got %v", err)
	}
}

func TestWindowsPath_returnsWindowsSpelling_whenCanonicalPathHasWindowsForm(t *testing.T) {
	// Given
	tests := []struct {
		name string
		path pathmodel.Path
		want string
	}{
		{name: "drive path", path: "/c/Users/nemo", want: "C:/Users/nemo"},
		{name: "drive root", path: "/c", want: "C:/"},
		{name: "UNC share path", path: "//server/share/dir", want: "//server/share/dir"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got, err := pathmodel.WindowsPath(tt.path)

			// Then
			if err != nil {
				t.Fatalf("expected WindowsPath to succeed, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestWindowsPath_returnsNoWindowsPath_whenPathUsesVirtualRoot(t *testing.T) {
	// Given
	tests := []struct {
		name string
		path pathmodel.Path
	}{
		{name: "tmp", path: "/tmp/file"},
		{name: "dev", path: "/dev/null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			_, err := pathmodel.WindowsPath(tt.path)

			// Then
			if !errors.Is(err, pathmodel.ErrNoWindowsPath) {
				t.Fatalf("expected ErrNoWindowsPath, got %v", err)
			}
		})
	}
}

func TestP05WaveA_ModelCWD_returnsCanonicalValue(t *testing.T) {
	// Given
	model := pathmodel.New(pathmodel.DefaultConfig(), "/c/work/./project/..")
	accessor, ok := any(model).(interface {
		CWD() pathmodel.Path
	})
	if !ok {
		t.Fatalf("expected Model to implement CWD() pathmodel.Path")
	}

	// When
	got := accessor.CWD()

	// Then
	if got != "/c/work" {
		t.Fatalf("expected CWD %q, got %q", "/c/work", got)
	}
}

func TestP05WaveA_ModelWithCWD_returnsValueCopyWithNewRoot(t *testing.T) {
	// Given
	originalCWD := pathmodel.Path("/d/original/work")
	original := pathmodel.New(pathmodel.DefaultConfig(), originalCWD)
	withCWD, ok := any(original).(interface {
		WithCWD(pathmodel.Path) pathmodel.Model
	})
	if !ok {
		t.Fatalf("expected Model to implement WithCWD(pathmodel.Path) pathmodel.Model")
	}
	tests := []struct {
		name      string
		cwd       pathmodel.Path
		wantRoot  pathmodel.Path
		wantChild pathmodel.Path
	}{
		{name: "drive", cwd: "/c/new/work", wantRoot: "/c", wantChild: "/c/root-relative-child"},
		{name: "UNC", cwd: "//server/share/new/work", wantRoot: "//server/share", wantChild: "//server/share/root-relative-child"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			updated := withCWD.WithCWD(tt.cwd)
			root, rootErr := updated.Resolve("/")
			child, childErr := updated.Resolve("/root-relative-child")
			originalAfter, originalErr := original.Resolve("")

			// Then
			if rootErr != nil {
				t.Fatalf("expected updated root resolve to succeed, got %v", rootErr)
			}
			if childErr != nil {
				t.Fatalf("expected updated child resolve to succeed, got %v", childErr)
			}
			if originalErr != nil {
				t.Fatalf("expected original CWD resolve to succeed, got %v", originalErr)
			}
			if root != tt.wantRoot {
				t.Fatalf("expected updated root %q, got %q", tt.wantRoot, root)
			}
			if child != tt.wantChild {
				t.Fatalf("expected updated child %q, got %q", tt.wantChild, child)
			}
			if originalAfter != originalCWD {
				t.Fatalf("expected original CWD to remain %q, got %q", originalCWD, originalAfter)
			}
		})
	}
}

func TestP05WaveA_ModelResolve_convertsCygdriveWhenEnabled(t *testing.T) {
	// Given
	config := pathmodel.DefaultConfig()
	config.AcceptCygdrive = true
	model := pathmodel.New(config, "/d/work")

	// When
	got, err := model.Resolve("/cygdrive/c/Users/nemo")

	// Then
	if err != nil {
		t.Fatalf("expected enabled cygdrive resolve to succeed, got %v", err)
	}
	if got != "/c/Users/nemo" {
		t.Fatalf("expected canonical cygdrive path %q, got %q", "/c/Users/nemo", got)
	}
}

func TestP05WaveA_ModelWithCWD_preservesUNCCurrentRootWhenEnteringVirtualRoot(t *testing.T) {
	// Given
	model := pathmodel.New(pathmodel.DefaultConfig(), "//server/share/work").WithCWD("/tmp")

	// When
	got, err := model.Resolve("/")

	// Then
	if err != nil {
		t.Fatalf("expected current root resolve to succeed, got %v", err)
	}
	if got != "//server/share" {
		t.Fatalf("expected preserved current root %q, got %q", "//server/share", got)
	}
}

func TestP05WaveA_ModelWithCWD_preservesCurrentRootWhenEnteringVirtualRoot(t *testing.T) {
	// Given
	model := pathmodel.New(pathmodel.DefaultConfig(), "/d/work").WithCWD("/tmp")

	// When
	root, rootErr := model.Resolve("/")
	child, childErr := model.Resolve("child.txt")

	// Then
	if rootErr != nil {
		t.Fatalf("expected current root resolve to succeed, got %v", rootErr)
	}
	if childErr != nil {
		t.Fatalf("expected virtual-root child resolve to succeed, got %v", childErr)
	}
	if root != "/d" {
		t.Fatalf("expected preserved current root %q, got %q", "/d", root)
	}
	if child != "/tmp/child.txt" {
		t.Fatalf("expected virtual-root child %q, got %q", "/tmp/child.txt", child)
	}
}
