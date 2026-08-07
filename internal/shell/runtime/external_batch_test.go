package runtime

import (
	"errors"
	"testing"
)

func TestHasBatchSuffix_matchesBatAndCmdCaseInsensitively(t *testing.T) {
	for _, testCase := range []struct {
		name string
		want bool
	}{
		{name: `C:\tools\build.bat`, want: true},
		{name: `C:\tools\build.CMD`, want: true},
		{name: `C:\tools\build.Bat`, want: true},
		{name: `C:\tools\build.exe`, want: false},
		{name: `C:\tools\build.sh`, want: false},
		{name: `C:\tools\build`, want: false},
		{name: `C:\bat\build`, want: false},
		{name: `C:\tools\build.batch`, want: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := hasBatchSuffix(testCase.name); got != testCase.want {
				t.Fatalf("hasBatchSuffix(%q) = %v, want %v", testCase.name, got, testCase.want)
			}
		})
	}
}

func TestComSpecPath_prefersComspecOverTheSystemRootFallback(t *testing.T) {
	vars := map[string]string{
		"COMSPEC":    `C:\alt\cmd.exe`,
		"SYSTEMROOT": `C:\Windows`,
	}

	got, err := comSpecPath(vars)

	if err != nil {
		t.Fatalf("comSpecPath: %v", err)
	}
	if want := `C:\alt\cmd.exe`; got != want {
		t.Fatalf("comSpecPath = %q, want %q", got, want)
	}
}

func TestComSpecPath_fallsBackToSystemRootWhenComspecIsAbsentOrEmpty(t *testing.T) {
	for name, vars := range map[string]map[string]string{
		"absent": {"SYSTEMROOT": `C:\Windows`},
		"empty":  {"COMSPEC": "", "SYSTEMROOT": `C:\Windows`},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := comSpecPath(vars)

			if err != nil {
				t.Fatalf("comSpecPath: %v", err)
			}
			if want := `C:\Windows\System32\cmd.exe`; got != want {
				t.Fatalf("comSpecPath = %q, want %q", got, want)
			}
		})
	}
}

func TestComSpecPath_reportsAMissingInterpreterRatherThanGuessing(t *testing.T) {
	_, err := comSpecPath(map[string]string{})

	if !errors.Is(err, errComSpecMissing) {
		t.Fatalf("comSpecPath error = %v, want %v", err, errComSpecMissing)
	}
}

func TestComSpecCommandLine_wrapsTheWholeCommandForTheSlashSStripRule(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		script string
		args   []string
		want   string
	}{
		// An operand that needs no quotes gets none, so `%1` is what the caller
		// wrote rather than a quoted copy of it. Quoting everything is what
		// made `if "%1"=="release"` compare ""release"" against "release".
		{
			name:   "no arguments",
			script: `C:\work\build.bat`,
			want:   `"C:\Windows\System32\cmd.exe" /d /s /c "C:\work\build.bat"`,
		},
		{
			name:   "plain argument",
			script: `C:\work\build.bat`,
			args:   []string{"release"},
			want:   `"C:\Windows\System32\cmd.exe" /d /s /c "C:\work\build.bat release"`,
		},
		{
			name:   "a script path with spaces is quoted",
			script: `C:\my work\build.bat`,
			args:   []string{"release"},
			want:   `"C:\Windows\System32\cmd.exe" /d /s /c ""C:\my work\build.bat" release"`,
		},
		{
			name:   "argument containing spaces",
			script: `C:\work\build.bat`,
			args:   []string{"two words"},
			want:   `"C:\Windows\System32\cmd.exe" /d /s /c "C:\work\build.bat "two words""`,
		},
		{
			name:   "argument containing a quote is doubled inside quotes",
			script: `C:\work\build.bat`,
			args:   []string{`say "hi"`},
			want:   `"C:\Windows\System32\cmd.exe" /d /s /c "C:\work\build.bat "say ""hi""""`,
		},
		{
			name:   "empty argument stays a distinct empty field",
			script: `C:\work\build.bat`,
			args:   []string{"", "after"},
			want:   `"C:\Windows\System32\cmd.exe" /d /s /c "C:\work\build.bat "" after"`,
		},
		{
			name:   "cmd metacharacters are quoted so they reach the script whole",
			script: `C:\work\build.bat`,
			args:   []string{"a&b|c>d"},
			want:   `"C:\Windows\System32\cmd.exe" /d /s /c "C:\work\build.bat "a&b|c>d""`,
		},
		{
			name:   "a lone percent cannot open a variable reference",
			script: `C:\work\build.bat`,
			args:   []string{"50%", "a%b"},
			want:   `"C:\Windows\System32\cmd.exe" /d /s /c "C:\work\build.bat 50% a%b"`,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := comSpecCommandLine(`C:\Windows\System32\cmd.exe`, testCase.script, testCase.args)

			if err != nil {
				t.Fatalf("comSpecCommandLine: %v", err)
			}
			if got != testCase.want {
				t.Fatalf("comSpecCommandLine:\n got %s\nwant %s", got, testCase.want)
			}
		})
	}
}

func TestComSpecCommandLine_rejectsOperandsCmdCannotCarryUnchanged(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		script string
		args   []string
	}{
		{name: "variable reference in argument", script: `C:\work\build.bat`, args: []string{"%PATH%"}},
		{name: "undefined variable reference is still refused", script: `C:\work\build.bat`, args: []string{"%a%"}},
		{name: "doubled percent in argument", script: `C:\work\build.bat`, args: []string{"a%%b"}},
		{name: "carriage return in argument", script: `C:\work\build.bat`, args: []string{"a\rb"}},
		{name: "newline in argument", script: `C:\work\build.bat`, args: []string{"a\nb"}},
		{name: "variable reference in script path", script: `C:\%work%\build.bat`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := comSpecCommandLine(`C:\Windows\System32\cmd.exe`, testCase.script, testCase.args)

			if !errors.Is(err, errBatchOperandUnsupported) {
				t.Fatalf("comSpecCommandLine error = %v, want %v", err, errBatchOperandUnsupported)
			}
		})
	}
}
