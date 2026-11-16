package applets

import (
	"crypto/sha3"
	"fmt"
	"hash"
	"strconv"

	"github.com/xiongnemo/nemosh/internal/digest"
)

// The rest of the hash family. nemosh had md5sum and sha256sum; busybox-w32 also
// has these four, and a clean Windows machine has none of them -- which is most
// of why anyone reaches for a checksum tool after a download.
//
// Each is the same applet with a different constructor, so -c, -b, -t and -w and
// the `<hex>  <name>` format come along for free rather than being reimplemented
// four times.
//
// MD5, SHA-1 and SHA-2 are CNG's, internal/digest, as busybox-w32's are; SHA-3 is crypto's,
// as busybox's is its own, CNG having no SHA3-224.

func newSha1sumApplet() Applet { return newChecksumApplet("sha1sum", digest.SHA1) }

func newSha384sumApplet() Applet { return newChecksumApplet("sha384sum", digest.SHA384) }

func newSha512sumApplet() Applet { return newChecksumApplet("sha512sum", digest.SHA512) }

// sha3sumDefaultBits is 224, which is what busybox defaults to -- measured, and
// worth stating because 512 is the obvious guess and would be silently wrong.
const sha3sumDefaultBits = 224

// newSha3sumApplet is sha3sum, whose width is chosen with `-a BITS`.
//
// SHA-3 is not SHA-2 truncated: SHA3-224 and SHA-224 are different functions
// over different permutations, so this cannot share a constructor with the four
// above. All four widths were cross-checked against both busybox and Go's
// crypto/sha3, because a wrong constant here yields a digest that looks
// plausible and is silently useless.
func newSha3sumApplet() Applet {
	return newChecksumAppletWith("sha3sum", "a", func(options appletOptions) (func() hash.Hash, error) {
		bits := sha3sumDefaultBits
		if options.has('a') {
			parsed, err := strconv.Atoi(options.value('a'))
			if err != nil {
				return nil, fmt.Errorf("invalid number '%s'", options.value('a'))
			}
			bits = parsed
		}
		switch bits {
		case 224:
			return func() hash.Hash { return sha3.New224() }, nil
		case 256:
			return func() hash.Hash { return sha3.New256() }, nil
		case 384:
			return func() hash.Hash { return sha3.New384() }, nil
		case 512:
			return func() hash.Hash { return sha3.New512() }, nil
		}
		// Refused rather than rounded to a width SHA-3 does have: a digest of
		// the wrong length compared against a published one fails confusingly,
		// where a refusal names the problem.
		return nil, fmt.Errorf("unsupported SHA-3 width %d; this build implements 224, 256, 384 and 512", bits)
	})
}
