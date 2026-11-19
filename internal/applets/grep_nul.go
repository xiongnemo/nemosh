package applets

import "bytes"

// grep -z and -b, GNU's --null-data and --byte-offset, which busybox's grep has not got.
//
// -z ends a line with a NUL rather than a newline, read and written, so `find -print0 | grep -z x
// | xargs -0` keeps a name with a newline in it whole. A NUL there is the end of a line, so it
// does not make the input a binary file, as GNU's does not take it for one.
//
// -b puts before each line the offset of its first byte in the input, counted from 0, and with -o
// the offset of the match itself. A line ended CRLF counts both. The offset is of the text grep
// reads, which for UTF-16 decoded on the way in is its UTF-8.

// grepSplit cuts the input into lines at each separator, the token holding what was before it.
// A line's carriage return is left in the token, for its offset; grepLine is what is matched.
func grepSplit(separator byte) func(data []byte, atEOF bool) (int, []byte, error) {
	return func(data []byte, atEOF bool) (int, []byte, error) {
		if index := bytes.IndexByte(data, separator); index >= 0 {
			return index + 1, data[:index], nil
		}
		if atEOF && len(data) > 0 {
			return len(data), data, nil
		}
		return 0, nil, nil
	}
}

// separator is what ends a line, read and written.
func (f grepFlags) separator() byte {
	if f.nullData {
		return 0
	}
	return '\n'
}

// grepLine is a token as it is matched and printed: without the carriage return of a CRLF, as
// bufio.ScanLines drops it, unless lines end with a NUL.
func (f grepFlags) grepLine(token []byte) []byte {
	if !f.nullData && len(token) > 0 && token[len(token)-1] == '\r' {
		return token[:len(token)-1]
	}
	return token
}

// binaryMode is what a NUL in the input means: under -z the end of a line, so nothing is binary.
func (f grepFlags) binaryMode() grepBinaryMode {
	if f.nullData && f.binary == grepBinaryDetect {
		return grepBinaryText
	}
	return f.binary
}
