package applets

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Read back from its end, an operand's last lines and bytes are what reading it through gives:
// ends and none, CRLF, empty lines, lines longer than a block, more lines asked for than there
// are, none asked for, and a file already part read, whose input is what is ahead.
func TestTail_fromTheEndIsWhatReadingThroughGives(t *testing.T) {
	saved := tailBlock
	t.Cleanup(func() { tailBlock = saved })
	dir := t.TempDir()
	contents := []string{"", "a", "a\n", "a\nb", "a\nb\n", "\n\n\n", "x\r\ny\r\n", "one\ntwo\nthree\nfour\n",
		strings.Repeat("long", 9) + "\n" + strings.Repeat("z", 13), "a\n\nb\n\n"}
	for _, block := range []int{1, 2, 3, 7, 64 << 10} {
		tailBlock = block
		for index, content := range contents {
			path := filepath.Join(dir, "f")
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, skip := range []int64{0, 2} {
				if skip > int64(len(content)) {
					continue
				}
				for count := int64(0); count <= 6; count++ {
					for _, inBytes := range []bool{false, true} {
						spec := countSpec{count: count, bytes: inBytes}
						var want, got bytes.Buffer
						if err := copyTailOf(&want, strings.NewReader(content[skip:]), spec); err != nil {
							t.Fatal(err)
						}
						file, err := os.Open(path)
						if err != nil {
							t.Fatal(err)
						}
						file.Seek(skip, io.SeekStart)
						err = copyTailOf(&got, &contextReadCloser{ctx: context.Background(), input: file}, spec)
						at, _ := file.Seek(0, io.SeekCurrent)
						file.Close()
						if err != nil || got.String() != want.String() || at != int64(len(content)) {
							t.Fatalf("block %d, content %d from %d, %+v: %q, %v, left at %d; want %q", block, index, skip, spec, got.String(), err, at, want.String())
						}
					}
				}
			}
		}
	}
}

// The last line of a file of a terabyte, nearly all of it a hole, comes back at once: tail reads
// back from the end. Reading it through would read a terabyte of zeros.
func TestTail_readsBackFromTheEnd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "huge")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	var returned uint32
	const setSparse = 0x900C4 // FSCTL_SET_SPARSE
	if err := windows.DeviceIoControl(windows.Handle(file.Fd()), setSparse, nil, 0, nil, 0, &returned, nil); err != nil {
		file.Close()
		t.Skipf("no sparse files here: %v", err)
	}
	if _, err := file.WriteAt([]byte("first\nthe last line\n"), 1<<40); err != nil {
		file.Close()
		t.Skipf("no terabyte file here: %v", err)
	}
	file.Close()
	done := make(chan string, 1)
	go func() {
		input, err := os.Open(path)
		if err != nil {
			done <- err.Error()
			return
		}
		defer input.Close()
		var out bytes.Buffer
		if err := copyTailOf(&out, &contextReadCloser{ctx: context.Background(), input: input}, countSpec{count: 1}); err != nil {
			done <- err.Error()
			return
		}
		done <- out.String()
	}()
	select {
	case got := <-done:
		if got != "the last line\n" {
			t.Errorf("tail -n1 of the terabyte said %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("tail -n1 of a terabyte file was still reading after five seconds: it read from the start")
	}
}
