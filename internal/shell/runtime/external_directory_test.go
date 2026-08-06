package runtime

import (
	"errors"
	"strings"
	"testing"
)

func directoryOfLength(units int) string {
	return `C:\` + strings.Repeat("d", units-3)
}

func rejectingShortener(t *testing.T) func(string) (string, error) {
	t.Helper()
	return func(dir string) (string, error) {
		t.Errorf("shortened %q, wanted the directory handed straight through", dir)
		return "", nil
	}
}

func TestChildWorkingDirectory_handsTheDirectoryThrough_whenItFitsTheCreateProcessBuffer(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		units int
	}{
		{name: "an ordinary directory", units: 30},
		{name: "the longest directory CreateProcess accepts", units: maxChildWorkingDirectory},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			want := directoryOfLength(testCase.units)
			got, err := childWorkingDirectory(want, rejectingShortener(t))
			if err != nil {
				t.Fatalf("childWorkingDirectory: %v", err)
			}
			if got != want {
				t.Errorf("got %q, want the directory unchanged", got)
			}
		})
	}
}

// A path is handed to CreateProcess as UTF-16, so the budget is spent in wide
// characters and not in the UTF-8 bytes Nemosh holds. Measured on Windows 10
// 19045: a 642-byte directory of 258 CJK characters launches, and one more
// character does not.
func TestChildWorkingDirectory_countsWideCharactersRatherThanBytes(t *testing.T) {
	want := `C:\` + strings.Repeat("文", maxChildWorkingDirectory-3)
	if len(want) <= maxChildWorkingDirectory {
		t.Fatalf("directory is %d bytes, wanted a case where bytes overcount", len(want))
	}
	got, err := childWorkingDirectory(want, rejectingShortener(t))
	if err != nil {
		t.Fatalf("childWorkingDirectory: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want the directory unchanged", got)
	}
}

func TestChildWorkingDirectory_fallsBackToTheShortName_whenTheDirectoryIsTooLong(t *testing.T) {
	long := directoryOfLength(maxChildWorkingDirectory + 1)
	short := `C:\DDDDDD~1`
	var asked []string
	got, err := childWorkingDirectory(long, func(dir string) (string, error) {
		asked = append(asked, dir)
		return short, nil
	})
	if err != nil {
		t.Fatalf("childWorkingDirectory: %v", err)
	}
	if got != short {
		t.Errorf("got %q, want the short name %q", got, short)
	}
	if len(asked) != 1 || asked[0] != long {
		t.Errorf("shortened %v, want exactly one request for %q", asked, long)
	}
}

func TestChildWorkingDirectory_reportsTheRealConstraint_whenNoShortNameHelps(t *testing.T) {
	long := directoryOfLength(maxChildWorkingDirectory + 1)
	volumeUnavailable := errors.New("the volume said no")
	for _, testCase := range []struct {
		name    string
		shorten func(string) (string, error)
		wantErr error
		wantMsg string
	}{
		{
			name:    "the volume refuses the query",
			shorten: func(string) (string, error) { return "", volumeUnavailable },
			wantErr: volumeUnavailable,
			wantMsg: "working directory is too long for a child process (259 > 258): the volume said no",
		},
		{
			name:    "8.3 names are disabled, so the long path comes back",
			shorten: func(dir string) (string, error) { return dir, nil },
			wantErr: errNoShortWorkingDirectory,
			wantMsg: "working directory is too long for a child process (259 > 258): no 8.3 short name is available",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := childWorkingDirectory(long, testCase.shorten)
			if got != "" {
				t.Errorf("got %q, want no directory", got)
			}
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("got %v, want it to wrap %v", err, testCase.wantErr)
			}
			if err.Error() != testCase.wantMsg {
				t.Errorf("got %q, want %q", err.Error(), testCase.wantMsg)
			}
		})
	}
}
