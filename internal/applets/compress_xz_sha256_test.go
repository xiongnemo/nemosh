package applets_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

// An xz stream checked with SHA-256 is read and its check made, through the system's SHA-256 that
// xz.SHA256New is given: whole it comes out as it went in, and with a byte of the check changed it
// is corrupted data -- where busybox's unxz, which makes no SHA-256 check, writes it out. The
// fixture is liblzma's, Python's lzma.compress with CHECK_SHA256, so the check is another
// program's.
func TestXz_makesTheSHA256CheckAStreamNames(t *testing.T) {
	stream, err := base64.StdEncoding.DecodeString("/Td6WFoAAArh+wyhAgAhARYAAAB0L+WjAQAUY2hlY2tlZCB3aXRoIFNIQS0yNTYKAAAAABznvZL2C7+f2K29eGhLUdKpAI6btT3eyGjl4fvjpkGTAAFFFYDqVcAYm0uaAQAAAAAKWVo=")
	if err != nil || stream[7] != 0x0a {
		t.Fatalf("the fixture is not a stream checked with SHA-256: %v", err)
	}
	const plain = "checked with SHA-256\n"
	dir := t.TempDir()
	if got, stderr, err := runSmall(t, dir, string(stream), "xzcat"); got != plain || err != nil {
		t.Errorf("xzcat = %q, %q, %v; want %q", got, stderr, err, plain)
	}
	sum := sha256.Sum256([]byte(plain))
	at := bytes.Index(stream, sum[:])
	if at < 0 {
		t.Fatal("the fixture's check is not its contents' SHA-256")
	}
	broken := bytes.Clone(stream)
	broken[at+31] ^= 0x01
	if got, stderr, err := runSmall(t, dir, string(broken), "xzcat"); err == nil || !strings.Contains(stderr+err.Error(), "corrupted data") {
		t.Errorf("a stream whose check is wrong = %q, %q, %v; want corrupted data", got, stderr, err)
	}
}
