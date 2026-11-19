package applets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"io"
	"math/rand/v2"
	"runtime"
	"strings"
	"testing"
)

// base64 and base32 stream, as busybox's do: `base64 -d` of 26 MB peaked at 155 MiB of commit,
// where busybox's peaked at 1.1, as the whole input was read, cleaned into a copy and decoded into
// a third. Here 3 MB each way, which allocated some 20 MB, held to 1 MiB; and the output is the
// same as the whole-input encoding's, wrapped at 76.
func TestBase64AndBase32_streamInBoundedMemory(t *testing.T) {
	data := make([]byte, 3<<20)
	rand.NewChaCha8([32]byte{20, 26, 11, 20}).Read(data)
	for _, codec := range []struct {
		name    string
		applet  func() Applet
		encoded string
	}{
		{"base64", newBase64Applet, base64.StdEncoding.EncodeToString(data)},
		{"base32", newBase32Applet, base32.StdEncoding.EncodeToString(data)},
	} {
		var wrapped strings.Builder
		for start := 0; start < len(codec.encoded); start += 76 {
			wrapped.WriteString(codec.encoded[start:min(start+76, len(codec.encoded))] + "\n")
		}
		for _, direction := range []struct {
			args  []string
			input string
			want  [32]byte
		}{
			{nil, string(data), sha256.Sum256([]byte(wrapped.String()))},
			{[]string{"-d"}, wrapped.String(), sha256.Sum256(data)},
		} {
			// When
			digest := sha256.New()
			input := strings.NewReader(direction.input)
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			err := codec.applet().Run(context.Background(), direction.args, input, digest, io.Discard)
			runtime.ReadMemStats(&after)

			// Then
			if err != nil || !bytes.Equal(digest.Sum(nil), direction.want[:]) {
				t.Fatalf("%s %v: error %v, or not the output of the whole-input encoding", codec.name, direction.args, err)
			}
			if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 1<<20 {
				t.Errorf("%s %v of 3 MB allocated %d bytes, want at most 1 MiB", codec.name, direction.args, allocated)
			}
		}
	}
}
