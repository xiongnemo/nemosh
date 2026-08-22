package applets

import (
	"bytes"
	"strings"
	"testing"
)

// Archive headers are the second surface fuzzing is for, after parsing.
//
// The shape is the same as FuzzParseScript's argument: arbitrary bytes from
// somewhere else, read before anything is created, by code that does its own
// fixed-width field arithmetic. cpio and ar have no library behind them here --
// `readCpioHeader` and `readArHeader` slice a byte array at hand-written offsets
// and parse each field with an explicitly named base -- which is exactly the code
// where an off-by-one becomes an index panic or a huge allocation.
//
// Both readers are pure: they consume a reader and build a struct, creating
// nothing. So a corpus costs only time.
//
// What is asserted is narrow and is the whole point: **no panic, and no absurd
// allocation.** A malformed archive is *expected* to be an error. What must never
// happen is a slice bounds panic, or a header whose length field says four
// gigabytes being believed.

func FuzzReadCpioHeader(f *testing.F) {
	// A well-formed header, so the fuzzer starts from something that parses.
	f.Add(buildFuzzCpio("a.txt", "payload"))
	f.Add(buildFuzzCpio(cpioTrailer, ""))
	f.Add(buildFuzzCpio("sub/deep/name.txt", ""))
	// The shapes that broke it or nearly did.
	f.Add([]byte("070701"))                                                     // the magic and nothing else
	f.Add([]byte("070707" + strings.Repeat("0", 70)))                           // an odc header: refused by magic
	f.Add([]byte(strings.Repeat("0", 110)))                                     // 110 zero bytes: no magic
	f.Add([]byte("070701" + strings.Repeat("F", 104)))                          // every field at its maximum
	f.Add([]byte("070701" + strings.Repeat("0", 96) + "FFFFFFFF" + "00000000")) // a 4 GiB name
	f.Add([]byte("070701" + strings.Repeat("0", 104)))                          // a zero-length name
	f.Add([]byte("070701" + strings.Repeat(" ", 104)))                          // blank fields
	f.Add([]byte("070701" + strings.Repeat("z", 104)))                          // fields that are not hex
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Bounded, so the corpus stays inside what the reader is designed for and
		// the fuzzer does not spend its time on length alone.
		if len(data) > 8192 {
			t.Skip()
		}
		entry, err := readCpioHeader(bytes.NewReader(data))
		if err != nil {
			return
		}
		if entry == nil {
			// The trailer, which is the only clean end.
			return
		}
		// A header that parsed must be *usable*, because every caller acts on these
		// fields immediately: the size is passed to io.CopyN and the name to the
		// containment check.
		if entry.size < 0 {
			t.Fatalf("a parsed entry claims size %d", entry.size)
		}
		if entry.name == "" {
			t.Fatal("a parsed entry has no name, but the name size was checked")
		}
		if strings.Contains(entry.name, "\x00") {
			// The stored length includes the terminating NUL and the name must not:
			// a name with an embedded NUL reaches Windows APIs that stop there, so
			// the checked name and the created name would differ.
			t.Fatalf("a parsed name holds a NUL: %q", entry.name)
		}
		// And the containment check must reach a decision about it rather than
		// panicking, which is the property that matters for a hostile archive.
		if _, err := safeArchivePath(entry.name); err != nil {
			return
		}
	})
}

func FuzzReadArHeader(f *testing.F) {
	f.Add(buildFuzzAr("a.txt/", 7))
	f.Add(buildFuzzAr("//", 20))
	f.Add(buildFuzzAr("/0", 5))
	f.Add([]byte(strings.Repeat(" ", 60)))
	// The end marker is the only structural check, so its absence is the first
	// thing to try.
	f.Add(buildFuzzArRaw(strings.Repeat("x", 58) + "??"))
	// A negative size, which decimal parsing will accept and io.CopyN will not.
	f.Add(buildFuzzArRaw("name/           0     0     0     0       -1        `\n"))
	// A mode in decimal where octal is expected: plausible nonsense rather than an
	// error, which is why the base is named at every call site.
	f.Add(buildFuzzArRaw("name/           0     0     0     99999999 1         `\n"))
	f.Add([]byte("!<arch>\n"))
	f.Add([]byte("!<arch>\n\n"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 8192 {
			t.Skip()
		}
		member, err := readArHeader(bytes.NewReader(data))
		if err != nil || member == nil {
			return
		}
		if member.size < 0 {
			t.Fatalf("a parsed member claims size %d", member.size)
		}
		// resolveArName does arithmetic on a stored `/OFFSET` against a table that
		// may be shorter than the offset, which is the indexing this guards.
		for _, table := range []string{"", "short/\n", strings.Repeat("x", 64)} {
			if _, err := resolveArName(member.name, table); err != nil {
				continue
			}
		}
	})
}

// The two writers, fuzzed on the *names* they are given rather than on bytes:
// writeArHeader builds a fixed-width header with %-16s and friends, so a name or a
// number that overflows its column silently shifts every field after it -- which
// is why the function asserts its own output length. That assertion is what this
// checks holds for every input.
func FuzzWriteArHeader(f *testing.F) {
	for _, seed := range []string{"a.txt", "", "//", "/0", strings.Repeat("x", 15), strings.Repeat("x", 16), "a b", "名前"} {
		f.Add(seed, int64(0), int64(0o644))
	}
	f.Add("a.txt", int64(1<<62), int64(0o777))
	f.Add("a.txt", int64(-1), int64(-1))

	f.Fuzz(func(t *testing.T, name string, size int64, mode int64) {
		if len(name) > 256 {
			t.Skip()
		}
		var buffer bytes.Buffer
		err := writeArHeader(&buffer, arMember{name: name, size: size, mode: mode})
		if err != nil {
			// Refusing is fine and is what a name too long for the column gets.
			return
		}
		// Having written one, it must be exactly 60 bytes, or the member after it
		// starts at the wrong offset and the archive is quietly corrupt.
		if buffer.Len() != arHeaderSize {
			t.Fatalf("writeArHeader(%q, %d, %o) wrote %d bytes, want %d",
				name, size, mode, buffer.Len(), arHeaderSize)
		}
		// And it must read back: a header this build writes and cannot parse is
		// the worst of the failures, because nothing reports it.
		member, err := readArHeader(bytes.NewReader(buffer.Bytes()))
		if err != nil {
			t.Fatalf("writeArHeader(%q, %d, %o) wrote a header readArHeader rejects: %v",
				name, size, mode, err)
		}
		if member == nil {
			t.Fatal("a written header read back as the end of the archive")
		}
		if member.size != size {
			t.Fatalf("the size round-tripped as %d, want %d", member.size, size)
		}
	})
}

// buildFuzzCpio is a minimal well-formed newc entry, used only as a seed. The
// hostile fixtures in cpio_ar_test.go build the same thing for assertions; this
// one stays here so the seeds do not depend on a helper in the external test
// package, which a fuzz target in package applets cannot reach.
func buildFuzzCpio(name, content string) []byte {
	var buffer bytes.Buffer
	stored := name + "\x00"
	buffer.WriteString(cpioMagicNewc)
	fields := []int{1, 0o100644, 0, 0, 1, 0, len(content), 0, 0, 0, 0, len(stored), 0}
	for _, value := range fields {
		buffer.WriteString(fuzzHex(value))
	}
	buffer.WriteString(stored)
	for (buffer.Len())%4 != 0 {
		buffer.WriteByte(0)
	}
	buffer.WriteString(content)
	for (buffer.Len())%4 != 0 {
		buffer.WriteByte(0)
	}
	return buffer.Bytes()
}

func fuzzHex(value int) string {
	const digits = "0123456789ABCDEF"
	out := make([]byte, 8)
	for index := 7; index >= 0; index-- {
		out[index] = digits[value&0xf]
		value >>= 4
	}
	return string(out)
}

func buildFuzzAr(stored string, size int) []byte {
	header := stored + strings.Repeat(" ", arNameColumns-len(stored))
	header += "0           " + "0     " + "0     " + "644     "
	header += "0000000000"[:10-len(fuzzDecimal(size))] + fuzzDecimal(size)
	return buildFuzzArRaw(header + arEndMarker)
}

func fuzzDecimal(value int) string {
	if value == 0 {
		return "0"
	}
	out := ""
	for value > 0 {
		out = string(rune('0'+value%10)) + out
		value /= 10
	}
	return out
}

func buildFuzzArRaw(header string) []byte {
	if len(header) < arHeaderSize {
		header += strings.Repeat(" ", arHeaderSize-len(header))
	}
	return []byte(header[:arHeaderSize])
}
