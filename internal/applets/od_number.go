package applets

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// odNumber is busybox's xstrtou_range_sfx as -N -j and -S read it, a number of 0 to most: C's
// strtoull with base 0, so 0x10 and 010 too, then nothing or one of b k m for 512, 1024 and
// 1048576. A sign or a blank before it, a letter after it, and a number the type cannot hold
// are invalid; one that the suffix takes past most is out of range.
func odNumber(text string, most uint64) (uint64, error) {
	invalid := fmt.Errorf("invalid number '%s'", text)
	if text == "" || strings.IndexByte("+- \t\n\v\f\r", text[0]) >= 0 {
		return 0, invalid
	}
	value, rest, ok := cNumberPrefix(text, 0)
	multiplier, known := map[string]uint64{"": 1, "b": 512, "k": 1024, "m": 1024 * 1024}[rest]
	switch {
	case !ok || !known || value > most:
		return 0, invalid
	case value > most/multiplier:
		return 0, fmt.Errorf("number %s is not in 0..%d range", text, most)
	}
	return value * multiplier, nil
}

// cNumberPrefix is what strtoull reads of text in base, or with base 0 in the base text names:
// hex after 0x, octal after a 0, decimal otherwise. ok is false when it read no digit, or more
// than 64 bits of one.
func cNumberPrefix(text string, base int) (uint64, string, bool) {
	body := text
	hex := len(text) > 2 && (text[:2] == "0x" || text[:2] == "0X") && isHexDigit(text[2])
	switch {
	case hex && (base == 0 || base == 16):
		base, body = 16, text[2:]
	case base == 0 && strings.HasPrefix(text, "0"):
		base = 8
	case base == 0:
		base = 10
	}
	end := 0
	for end < len(body) {
		digit, err := strconv.ParseUint(body[end:end+1], 16, 8)
		if err != nil || int(digit) >= base {
			break
		}
		end++
	}
	if end == 0 {
		return 0, text, false
	}
	value, err := strconv.ParseUint(body[:end], base, 64)
	return value, body[end:], err == nil
}

// odOldOffset is --traditional's [+]OFFSET[.][b], busybox's parse_old_offset: octal, hex after
// 0x, decimal when a . is in it, which ends it, and b or B after it for 512 or 1024 bytes. ok
// is false for what is no offset, which a digit does not begin.
func odOldOffset(text string) (int64, bool, error) {
	text = strings.TrimPrefix(text, "+")
	if text == "" || !isASCIIDigit(text[0]) {
		return 0, false, nil
	}
	base := 8
	if dot := strings.IndexByte(text, '.'); dot >= 0 {
		text, base = text[:dot], 10
	} else if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		base = 16
	}
	value, rest, ok := cNumberPrefix(text, base)
	multiplier, known := map[string]uint64{"": 1, "b": 512, "B": 1024}[rest]
	switch {
	case !ok || !known:
		return 0, false, fmt.Errorf("invalid number '%s'", text)
	case value > math.MaxUint64/multiplier:
		return 0, false, fmt.Errorf("number %s is not in 0..%d range", text, uint64(math.MaxUint64))
	}
	offset := int64(value * multiplier)
	return offset, offset >= 0, nil
}

// traditional is --traditional's operands, od_bloaty.c's `[FILE] [[+]OFFSET[.][b]
// [[+]LABEL[.][b]]]`: OFFSET is where to begin, as -j's SKIP, and LABEL the offset to call the
// first byte, printed in parentheses after each address, or alone in them with -A n.
func (r *odRun) traditional(paths []string) ([]string, error) {
	label := int64(-1)
	switch len(paths) {
	case 0:
	case 1:
		offset, ok, err := odOldOffset(paths[0])
		if err != nil {
			return nil, err
		}
		if ok {
			r.skip, paths = offset, nil
		}
	case 2:
		first, firstOK, err := odOldOffset(paths[0])
		if err != nil {
			return nil, err
		}
		second, secondOK, err := odOldOffset(paths[1])
		switch {
		case err != nil:
			return nil, err
		case firstOK && secondOK:
			r.skip, label, paths = first, second, nil
		case secondOK:
			r.skip, paths = second, paths[:1]
		default:
			return nil, fmt.Errorf("invalid second argument '%s'", paths[1])
		}
	case 3:
		offset, ok, err := odOldOffset(paths[1])
		if err != nil {
			return nil, err
		}
		if ok {
			if label, ok, err = odOldOffset(paths[2]); err != nil {
				return nil, err
			}
		}
		if !ok {
			return nil, errors.New("the last two arguments must be offsets")
		}
		r.skip, paths = offset, paths[:1]
	default:
		return nil, errors.New("too many arguments")
	}
	if label >= 0 {
		r.addressing = odAddressLabel
		if r.radix == 'n' {
			r.radix, r.pad, r.addressing = 'o', 7, odAddressParen
		}
		r.pseudo = label - r.skip
	}
	return paths, nil
}
