package textgrid

import (
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/text/width"
)

// The generated table says of every rune what x/text/width says, wide or fullwidth or neither:
// the table is that package's answer, in a few hundred bytes where the package linked 357 KB.
//
// x/text answers for the Unicode of the Go that builds it, 15.0.0 below go1.27 and 17.0.0 from
// it, and the table is the one the Go in go.mod gives, which CI and the release build with, so
// that the release says of each rune what v1.5.0, which linked x/text, said. It was generated
// once under go1.27, and so said U+2630 was wide where every runner's x/text said it was not.
// Under the Go in go.mod the two agree, version and all; under a newer Go x/text answers for
// another Unicode, and the comparison is not made. Regenerate as tools/widthtable says.
func TestWideTable_agreesWithXTextWidthForEveryRune(t *testing.T) {
	if width.UnicodeVersion != wideTableUnicode {
		release := goModRelease(t)
		if version := runtime.Version(); version == "go"+release || strings.HasPrefix(version, "go"+release+".") {
			t.Fatalf("the table is Unicode %s, and x/text/width under %s, the Go in go.mod, is %s: regenerate it", wideTableUnicode, version, width.UnicodeVersion)
		}
		t.Skipf("x/text/width under %s is Unicode %s; the table is %s, which go%s gives", runtime.Version(), width.UnicodeVersion, wideTableUnicode, release)
	}
	for r := rune(0); r <= 0x10FFFF; r++ {
		kind := width.LookupRune(r).Kind()
		want := kind == width.EastAsianWide || kind == width.EastAsianFullwidth
		if got := isWide(r); got != want {
			t.Fatalf("U+%04X: the table says wide %v, x/text/width %v", r, got, want)
		}
	}
	for index := 1; index < len(wideRanges); index++ {
		if wideRanges[index][0] <= wideRanges[index-1][1]+1 {
			t.Fatalf("ranges %d and %d touch or overlap: %X, %X", index-1, index, wideRanges[index-1], wideRanges[index])
		}
	}
}

// goModRelease is the Go release go.mod names, 1.26 of `go 1.26`.
func goModRelease(t *testing.T) string {
	t.Helper()
	text, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	found := regexp.MustCompile(`(?m)^go (\d+\.\d+)`).FindSubmatch(text)
	if found == nil {
		t.Fatal("go.mod names no Go release")
	}
	return string(found[1])
}
