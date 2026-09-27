package runtime_test

import "testing"

// `${x~}` and `${x~~}` toggle case, the first character and every one, as `^` and `,` raise
// and lower it: a pattern narrows which characters are touched, and an array is toggled
// element by element. Both were "bad substitution". The answers are bash 5.3's; busybox has
// no case operators.
func TestRuntime_tildeTogglesCase(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`x="Hello World"; echo "${x~}|${x~~}|${x~~[lo]}|${x~[hH]}"`, "hello World|hELLO wORLD|HeLLO WOrLd|hello World\n"},
		{`e=; y=12a; echo "[${e~~}]|${y~}|${y~~}"`, "[]|12a|12A\n"},
		{`u="éÉ straße"; echo "${u~~}|${u~}"`, "Éé STRAßE|ÉÉ straße\n"},
		{`a=(ab Cd); echo "${a[@]~}|${a[@]~~}|${a[*]~~}"`, "Ab cd|AB cD|AB cD\n"},
		{`set -- aB Cd; echo "${@~}|${*~~}"`, "AB cd|Ab cD\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}

// An operator on a `*` list maps every element and joins them, as `${a[*]}` joins them. Only
// the slice did; the rest kept the first element and dropped the others. bash 5.3's answers.
func TestRuntime_aStarListKeepsEveryElementUnderAnOperator(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`a=(ab cd); echo "${a[*]/b/X}|${a[*]#a}|${a[*]%d}|${a[*]^^}"`, "aX cd|b cd|ab c|AB CD\n"},
		{`set -- ab cd; echo "${*/b/X}|${*^}"`, "aX cd|Ab Cd\n"},
		{`a=(ab cd); IFS=:; echo "${a[*]^^}"; for w in ${a[*]^^}; do echo "<$w>"; done`, "AB:CD\n<AB>\n<CD>\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
