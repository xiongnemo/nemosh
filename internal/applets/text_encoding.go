package applets

import "unicode/utf8"

// Text that is not UTF-8 has to survive a filter that does not understand it.
//
// This shell reads text as UTF-8 everywhere, and that is right: `rev 中文` answers `文中`,
// `wc -m` counts characters, `expr length` counts characters, and both reference
// implementations get all three wrong by counting bytes. The mistake was doing it
// *unconditionally*.
//
// Go's `[]rune(string)` conversion is lossy on input that is not UTF-8: every invalid byte
// becomes U+FFFD, and converting back writes three bytes where one came in. So a GBK file --
// which is what Notepad still produces on a Chinese Windows, and what most .bat files on such a
// machine are -- came out of `rev` and `tr` destroyed:
//
//	in : d6d0 cec4 b2e2 cad4     (中文测试 in CP936)
//	out: efbfbd efbfbd efbfbd ...
//
//	$ tr q q < gbk.txt           a transformation that changes nothing
//	  ... and the file is ruined
//
// The fix is not to stop understanding UTF-8. It is to notice when the input is not UTF-8 and
// then touch nothing: a filter that cannot read the bytes has no business rewriting them. That
// covers GBK, Big5, Shift-JIS and EUC-KR alike, without knowing one of them from another --
// which is the only approach that scales, since we cannot know which one a file is in.
//
// What stays byte-exact either way, measured over a GBK file: cat, head, tail, sort, uniq, tac,
// sed, grep, cut, paste, nl and base64. Only rev and tr rewrote what they had decoded.

// reverseText reverses a line, by character where it can and by byte where it cannot.
func reverseText(line string) string {
	if utf8.ValidString(line) {
		runes := []rune(line)
		for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
			runes[left], runes[right] = runes[right], runes[left]
		}
		return string(runes)
	}
	// Not UTF-8: reverse the bytes, which is what busybox does for every input and is
	// lossless. `rev | rev` restores the file exactly, which is the property worth having.
	bytes := []byte(line)
	for left, right := 0, len(bytes)-1; left < right; left, right = left+1, right-1 {
		bytes[left], bytes[right] = bytes[right], bytes[left]
	}
	return string(bytes)
}

// substringText is `expr substr`: characters where the value is UTF-8, bytes where it is not.
//
// Reachable with non-UTF-8 input through a variable rather than through argv -- Windows hands
// arguments over as UTF-16, so they are always valid, but `x=$(cat gbk.txt)` is not.
func substringText(value string, start, count int) string {
	if !utf8.ValidString(value) {
		if start > len(value) {
			return ""
		}
		return value[start:min(start+count, len(value))]
	}
	runes := []rune(value)
	if start > len(runes) {
		return ""
	}
	return string(runes[start:min(start+count, len(runes))])
}
