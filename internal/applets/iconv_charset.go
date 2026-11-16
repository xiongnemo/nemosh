package applets

import (
	"errors"
	"strconv"
	"strings"
)

// An iconv character set, win_iconv's csconv_t (busybox-w32's miscutils/iconv.c): its code
// page, how one character of it becomes UTF-16 and back, how long a character is, what ends a
// conversion into it, and the state of a conversion under way.

var (
	errIconvIllegal    = errors.New("Illegal byte sequence") // EILSEQ
	errIconvIncomplete = errors.New("Invalid argument")      // EINVAL: a character the input stops inside
)

const (
	iconvUseBOM = 1 << iota
	iconvTranslit
	iconvIgnore
)

const (
	unicodeBOMDone = 1 << iota
	unicodeSwapped
)

type iconvCharset struct {
	codePage int
	flags    int
	mode     uint32
	// toWide is one character of in as UTF-16: how many bytes it took and how many units it
	// made, 0 for one, like a byte-order mark, that stands for nothing.
	toWide   func(cs *iconvCharset, in []byte, wide []uint16) (int, int, error)
	fromWide func(cs *iconvCharset, wide []uint16, out []byte) (int, error)
	length   func(cs *iconvCharset, in []byte) (int, error)
	flush    func(cs *iconvCharset, out []byte) (int, error)
	compat   []iconvCompat
	// leads are a double-byte code page's lead bytes, IsDBCSLeadByteEx's answer for each, asked
	// of GetCPInfo once.
	leads [256]bool
}

// newIconvCharset is win_iconv's make_csconv: a name, with any //NOCOMPAT, //TRANSLIT and
// //IGNORE after it, as the character set it names, or false for none Windows has.
func newIconvCharset(name string) (iconvCharset, bool) {
	cs := iconvCharset{}
	useCompat := true
	for {
		at := strings.LastIndex(name, "//")
		if at < 0 {
			break
		}
		switch strings.ToLower(name[at+2:]) {
		case "nocompat":
			useCompat = false
		case "translit":
			cs.flags |= iconvTranslit
		case "ignore":
			cs.flags |= iconvIgnore
		}
		name = name[:at]
	}
	cs.codePage = iconvCodePage(name)
	switch page := cs.codePage; {
	case page == 1200 || page == 1201:
		cs.toWide, cs.fromWide = utf16ToWide, utf16FromWide
		if equalsAnyFold(name, "UTF-16", "UTF16", "UCS-2", "UCS2", "UCS-2-INTERNAL") {
			cs.flags |= iconvUseBOM
		}
	case page == 12000 || page == 12001:
		cs.toWide, cs.fromWide = utf32ToWide, utf32FromWide
		if equalsAnyFold(name, "UTF-32", "UTF32", "UCS-4", "UCS4") {
			cs.flags |= iconvUseBOM
		}
	case page == 65001:
		cs.toWide, cs.fromWide, cs.length = kernelToWide, kernelFromWide, utf8CharLength
	case (page == 50220 || page == 50221 || page == 50222) && iconvAPI().mlangLoaded:
		cs.toWide, cs.fromWide, cs.flush = iso2022jpToWide, iso2022jpFromWide, iso2022jpFlush
	case page == 51932 && iconvAPI().mlangLoaded:
		cs.toWide, cs.fromWide, cs.length = mlangToWideChar, mlangFromWideChar, eucjpCharLength
	case page > 0 && page <= 0xffff && codePageMaxCharSize(page, nil) > 0:
		cs.toWide, cs.fromWide = kernelToWide, kernelFromWide
		switch codePageMaxCharSize(page, &cs.leads) {
		case 1:
			cs.length = singleByteLength
		case 2:
			cs.length = doubleByteLength
		default:
			cs.length = multiByteLength
		}
	default:
		return cs, false
	}
	if useCompat {
		switch cs.codePage {
		case 932, 51932, 50220, 50221, 50222:
			cs.compat = cp932Compat[:]
		case 20932:
			cs.compat = cp20932Compat[:]
		}
	}
	return cs, true
}

// iconvCodePage is win_iconv's name_to_codepage: nothing or "char" is the code page this
// process uses, which busybox-w32 builds UTF-8 into its manifest for, and nemosh's is UTF-8
// too; CP123, XX123 and 123 are that number; anything else is the first alias it matches,
// whatever its case; -1 for none.
func iconvCodePage(name string) int {
	switch {
	case name == "" || name == "char":
		return 65001
	case name == "wchar_t":
		return 1200
	case len(name) >= 2 && (strings.EqualFold(name[:2], "cp") || strings.EqualFold(name[:2], "xx")):
		return atoiPrefix(name[2:])
	case name[0] >= '0' && name[0] <= '9':
		return atoiPrefix(name)
	}
	rest := iconvAliasNames
	for index := 0; rest != ""; index++ {
		alias, after, _ := strings.Cut(rest, "\x00")
		if strings.EqualFold(alias, name) {
			return int(iconvAliasPages[index])
		}
		rest = after
	}
	return -1
}

// atoiPrefix is C's atoi: the digits text begins with, 0 for none.
func atoiPrefix(text string) int {
	end := 0
	for end < len(text) && end < 9 && text[end] >= '0' && text[end] <= '9' {
		end++
	}
	value, _ := strconv.Atoi(text[:end])
	return value
}

func equalsAnyFold(name string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.EqualFold(name, candidate) {
			return true
		}
	}
	return false
}

// iconvCompat maps a character to the one another table has for it: in to out on the way out,
// out to in on the way in where the flag says so.
type iconvCompat struct {
	in, out uint32
	inward  bool
}

// CP932's table differs from SJIS's in ten places, and EUC-JP and ISO-2022-JP should agree with
// CP932; see win_iconv's comment over cp932_compat.
var cp932Compat = [...]iconvCompat{
	{0x00A5, 0x005C, false}, {0x203E, 0x007E, false}, {0x2014, 0x2015, false},
	{0x301C, 0xFF5E, false}, {0x2016, 0x2225, false}, {0x2212, 0xFF0D, false},
	{0x00A2, 0xFFE0, false}, {0x00A3, 0xFFE1, false}, {0x00AC, 0xFFE2, false},
}

var cp20932Compat = [...]iconvCompat{
	{0x00A5, 0x005C, false}, {0x203E, 0x007E, false}, {0x2014, 0x2015, false},
	{0xFF5E, 0x301C, true}, {0x2225, 0x2016, true}, {0xFF0D, 0x2212, true},
	{0xFFE0, 0x00A2, true}, {0xFFE1, 0x00A3, true}, {0xFFE2, 0x00AC, true},
}

func utf16ToCodePoint(wide []uint16) uint32 {
	if len(wide) >= 2 && wide[0] >= 0xD800 && wide[0] <= 0xDBFF {
		return (uint32(wide[0]&0x3FF) << 10) + uint32(wide[1]&0x3FF) + 0x10000
	}
	return uint32(wide[0])
}

func codePointToUTF16(point uint32, wide []uint16) int {
	if point < 0x10000 {
		wide[0] = uint16(point)
		return 1
	}
	point -= 0x10000
	wide[0], wide[1] = 0xD800|uint16(point>>10&0x3FF), 0xDC00|uint16(point&0x3FF)
	return 2
}

// mbToWideFlags is win_iconv's mbtowc_flags: the code pages MultiByteToWideChar refuses
// MB_ERR_INVALID_CHARS for.
func mbToWideFlags(page int) uint32 {
	if nullDefaultCharPage(page) && page != 65001 {
		return 0
	}
	return mbErrInvalidChars
}

// nullDefaultCharPage is win_iconv's must_use_null_useddefaultchar.
func nullDefaultCharPage(page int) bool {
	switch page {
	case 65000, 65001, 50220, 50221, 50222, 50225, 50227, 50229, 52936, 54936, 42:
		return true
	}
	return page >= 57002 && page <= 57011
}
