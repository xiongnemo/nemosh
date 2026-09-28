package runtime_test

import "testing"

// A pattern's and a replacement's leading tilde-prefix is the directory it names, inside
// double quotes too, as busybox-w32 and bash both expand it; a value's in double quotes stays
// a tilde there. Only a value's was expanded, so `${path//~/z}` left the path as it was.
func TestRuntime_operatorPatternTakesTilde(t *testing.T) {
	script := "HOME=/h; x=/h/a\n" +
		"echo ${x//~/z} ${x/~/z} ${x#~} ${x%~/a}X \"${x/~/z}\" \"${x#~}\"\n" +
		"y=a; echo ${y/a/~} \"${y/a/~}\"\n" +
		"unset u; echo ${u:-~} \"${u:-~}\"\n" +
		"HOME='/h*'; z='/hx/a'; echo \"${z#~}\"\n"
	want := "z/a z/a /a X z/a /a\n/h /h\n/h ~\n/hx/a\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Errorf("got %q/%d, want %q/0, as busybox-w32 and bash answer", stdout, status, want)
	}
}
