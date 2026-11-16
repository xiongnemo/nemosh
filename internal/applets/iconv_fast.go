package applets

// A run of characters converted in two calls rather than two a character, which is what
// win_iconv does and what made iconv take 2.0 s over a 27 MB file where busybox-w32 takes 0.5:
// MultiByteToWideChar of every whole character the block holds, then WideCharToMultiByte of
// all of it, or UTF-16 by hand on either side. Anything either call refuses, and a character
// set whose characters a run cannot hold -- one that keeps state, one with a compatibility
// table, one whose calls take no MB_ERR_INVALID_CHARS -- goes the way win_iconv goes, a
// character at a time, so what is written and where it stops are win_iconv's.

// iconvRun converts as much of in as it can as one run: how much it took, what it wrote, and
// whether it did any of it. Nothing taken means the caller goes a character at a time.
func (c *iconvConverter) iconvRun(in []byte, out []byte) (int, []byte, bool) {
	if c.from.compat != nil || c.to.compat != nil || c.from.mode&unicodeBOMDone == 0 && c.from.flags&iconvUseBOM != 0 ||
		c.to.mode&unicodeBOMDone == 0 && c.to.flags&iconvUseBOM != 0 {
		return 0, out, false
	}
	whole, wide := c.wholeCharacters(in), c.wide[:0]
	if whole == 0 {
		return 0, out, false
	}
	switch {
	case c.fastKernel(&c.from):
		if cap(c.wide) < whole {
			c.wide = make([]uint16, whole)
		}
		made := multiToWide(c.from.codePage, mbErrInvalidChars, in[:whole], c.wide[:whole])
		if made == 0 {
			return 0, out, false
		}
		wide = c.wide[:made]
	case c.from.codePage == 1200 || c.from.codePage == 1201:
		for at := 0; at < whole; at += 2 {
			wide = append(wide, uint16(unicodeUnit(&c.from, 1200, in[at:], 2)))
		}
		c.wide = wide
	default:
		return 0, out, false
	}
	switch {
	case c.fastKernel(&c.to):
		flags, askDefault := uint32(0), c.to.codePage != 65001
		if askDefault && c.to.flags&iconvTranslit == 0 {
			flags = wcNoBestFitChars
		}
		start := len(out)
		out = append(out, make([]byte, 4*len(wide)+4)...)
		written, usedDefault, _ := wideToMulti(c.to.codePage, flags, wide, out[start:], askDefault)
		if written == 0 || usedDefault && c.to.flags&iconvTranslit == 0 {
			if c.to.flags&iconvIgnore == 0 {
				return 0, out[:start], false
			}
			// -c: the run in halves, and halves of those, down to the characters the target has
			// not got, which are passed over, as a character at a time would pass them over.
			return whole, c.keepConvertible(wide, out[:start], flags, askDefault), true
		}
		return whole, out[:start+written], true
	case c.to.codePage == 1200 || c.to.codePage == 1201:
		for _, unit := range wide {
			out = append(out, 0, 0)
			putUnicodeUnit(c.to.codePage == 1200, out[len(out)-2:], uint32(unit), 2)
		}
		return whole, out, true
	}
	return 0, out, false
}

// fastKernel is whether a character set's conversions are kernel32's and keep no state, and
// refuse what is no character when asked to.
func (c *iconvConverter) fastKernel(cs *iconvCharset) bool {
	page := cs.codePage
	if page == 1200 || page == 1201 || page == 12000 || page == 12001 || page == 20127 || page == 51932 {
		return false
	}
	if page >= 50220 && page <= 50222 || cs.length == nil {
		return false
	}
	return page == 65001 || !nullDefaultCharPage(page)
}

// wholeCharacters is how many bytes of in are whole characters of the input's set, read in
// Go; it stops at the first that is not one, which a character at a time then reports.
func (c *iconvConverter) wholeCharacters(in []byte) int {
	from := &c.from
	if from.codePage == 1200 || from.codePage == 1201 {
		at := 0
		for at+2 <= len(in) {
			unit := unicodeUnit(from, 1200, in[at:], 2)
			switch {
			case unit >= 0xDC00 && unit <= 0xDFFF:
				return at
			case unit >= 0xD800 && unit <= 0xDBFF:
				if at+4 > len(in) {
					return at
				}
				if second := unicodeUnit(from, 1200, in[at+2:], 2); second < 0xDC00 || second > 0xDFFF {
					return at
				}
				at += 4
			default:
				at += 2
			}
		}
		return at
	}
	if !c.fastKernel(from) {
		return 0
	}
	at := 0
	for at < len(in) {
		if in[at] < 0x80 {
			at++
			continue
		}
		length, err := from.length(from, in[at:])
		if err != nil || length > 4 {
			return at
		}
		at += length
	}
	return at
}

// keepConvertible is wide in the output's set without the characters it has not got.
func (c *iconvConverter) keepConvertible(wide []uint16, out []byte, flags uint32, askDefault bool) []byte {
	start := len(out)
	out = append(out, make([]byte, 4*len(wide)+4)...)
	written, usedDefault, _ := wideToMulti(c.to.codePage, flags, wide, out[start:], askDefault)
	if written > 0 && !(usedDefault && c.to.flags&iconvTranslit == 0) {
		return out[:start+written]
	}
	out = out[:start]
	if len(wide) == 1 || len(wide) == 2 && wide[0] >= 0xD800 && wide[0] <= 0xDBFF {
		return out
	}
	middle := len(wide) / 2
	if wide[middle] >= 0xDC00 && wide[middle] <= 0xDFFF {
		middle++
	}
	out = c.keepConvertible(wide[:middle], out, flags, askDefault)
	return c.keepConvertible(wide[middle:], out, flags, askDefault)
}
