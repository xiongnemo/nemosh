package applets_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// 中文测试 in CP936, with a CRLF line ending -- a file straight off a Chinese Windows.
var gbkLine = []byte{0xd6, 0xd0, 0xce, 0xc4, 0xb2, 0xe2, 0xca, 0xd4, 0x0d, 0x0a}

// The same content ending in LF. The round-trip property is stated over this one because a
// text filter here normalizes CRLF to LF: bufio.ScanLines strips the carriage return and
// eachLine re-emits a newline. That is deliberate, pre-existing, and separate from anything
// about encoding -- but it does mean `rev | rev` returns a CRLF file with LF endings.
var gbkLineLF = []byte{0xd6, 0xd0, 0xce, 0xc4, 0xb2, 0xe2, 0xca, 0xd4, 0x0a}

// Every filter that has to understand characters, given bytes that are not characters.
//
// These are not hypothetical inputs. A .txt from Notepad or a .bat on a Chinese Windows is
// CP936, and `tr -d '\r'` on such a file is the most ordinary thing anyone does with it. Before
// this, that command destroyed the file: every GBK byte became U+FFFD, three bytes for one.
//
// The corpus cannot hold these cases -- a TOML file has to be valid UTF-8 -- so they live here.
func TestFilters_leaveNonUTF8BytesAlone(t *testing.T) {
	tests := []struct {
		name    string
		applet  string
		args    []string
		in      []byte
		want    []byte
		comment string
	}{
		{
			name: "tr with no substitution to make", applet: "tr", args: []string{"q", "q"},
			in: gbkLine, want: gbkLine,
			comment: "a transformation that changes nothing must change nothing",
		},
		{
			// The canonical Windows use of tr, and the one that mattered most.
			name: "tr -d carriage return", applet: "tr", args: []string{"-d", "\\r"},
			in:   gbkLine,
			want: []byte{0xd6, 0xd0, 0xce, 0xc4, 0xb2, 0xe2, 0xca, 0xd4, 0x0a},
		},
		{
			name: "tr -s on a character not present", applet: "tr", args: []string{"-s", "q"},
			in: gbkLine, want: gbkLine,
		},
		{
			name: "tr translating ascii around the bytes", applet: "tr", args: []string{"a", "b"},
			in:   []byte{0xd6, 0xd0, 'a', 0x0a},
			want: []byte{0xd6, 0xd0, 'b', 0x0a},
		},
		{
			name: "rev reverses the bytes", applet: "rev", args: nil,
			in:   []byte{0xd6, 0xd0, 0xce, 0xc4, 0x0a},
			want: []byte{0xc4, 0xce, 0xd0, 0xd6, 0x0a},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			out, stderr, err := runFilter(t, test.applet, test.args, string(test.in))
			if err != nil {
				t.Fatalf("%s %v: %v (%s)", test.applet, test.args, err, stderr)
			}
			got := []byte(out)

			// Then
			if !bytes.Equal(got, test.want) {
				t.Fatalf("%s %v\n in : % x\n got: % x\nwant: % x",
					test.applet, test.args, test.in, got, test.want)
			}
			if bytes.Contains(got, []byte{0xef, 0xbf, 0xbd}) {
				t.Fatalf("%s invented a replacement character: % x", test.applet, got)
			}
		})
	}
}

// rev twice returns the file, whatever the file is. This is the property the fix is really
// about: a filter may not understand the bytes, and it may not damage them either.
func TestRev_roundTripsACP936File(t *testing.T) {
	// Given
	original := append([]byte{}, gbkLineLF...)

	// When
	once, _, err := runFilter(t, "rev", nil, string(original))
	if err != nil {
		t.Fatalf("rev: %v", err)
	}
	twiceText, _, err := runFilter(t, "rev", nil, once)
	if err != nil {
		t.Fatalf("rev: %v", err)
	}
	twice := []byte(twiceText)

	// Then
	if !bytes.Equal(twice, original) {
		t.Fatalf("rev twice\n in : % x\n got: % x", original, twice)
	}
}

// The rune path is the reason not to simply work in bytes everywhere: it is right, and both
// reference implementations are wrong here. busybox reverses the bytes of 中文 and produces
// mojibake; this produces 文中.
func TestRev_stillReversesUTF8ByCharacter(t *testing.T) {
	// When
	got, _, err := runFilter(t, "rev", nil, "中文abc\n")
	if err != nil {
		t.Fatalf("rev: %v", err)
	}

	// Then
	if want := "cba文中\n"; got != want {
		t.Fatalf("rev = %q, want %q", got, want)
	}
}

// `expr substr` on a value that came out of a file rather than out of argv. Windows hands
// arguments over as UTF-16 so they are always valid UTF-8, but `x=$(cat gbk.txt)` is not.
func TestExprSubstr_onNonUTF8(t *testing.T) {
	applet, ok := applets.DefaultRegistry.Lookup("expr")
	if !ok {
		t.Fatal("expr is not registered")
	}
	value := string([]byte{0xd6, 0xd0, 0xce, 0xc4})
	var stdout, stderr bytes.Buffer
	if err := applet.Run(context.Background(), []string{"substr", value, "1", "2"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("expr substr: %v (%s)", err, stderr.String())
	}

	// Then -- the first two bytes, which is one CP936 character, and reversible.
	if want := string([]byte{0xd6, 0xd0}) + "\n"; stdout.String() != want {
		t.Fatalf("expr substr = % x, want % x", stdout.String(), want)
	}
}
