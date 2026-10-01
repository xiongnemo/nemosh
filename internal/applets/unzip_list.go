package applets

import (
	"archive/zip"
	"fmt"
	"io"
)

// writeListing prints -l's table as busybox's unzip prints it, or -v's, which adds each
// entry's method, compressed size, ratio and CRC. The date is the DOS date the archive
// holds, month first, as busybox prints it. The total counts the entries listed, where
// busybox's counts every entry in the archive, so `unzip -l a.zip one.txt` says "2 files"
// there.
//
// Names are listed as the archive holds them, unchecked, because listing is how
// somebody inspects an archive they do not trust -- refusing here would hide the
// very entry they are looking for.
func (r unzipRequest) writeListing(entries []*zip.File, stdout io.Writer) error {
	var size, compressed uint64
	for _, entry := range entries {
		size += entry.UncompressedSize64
		compressed += entry.CompressedSize64
		when := unzipDOSTime(entry.ModifiedDate, entry.ModifiedTime)
		if r.verbose {
			fmt.Fprintf(stdout, "%8d  %s%9d%4d%% %s %08x  %s\n", entry.UncompressedSize64, unzipMethod(entry),
				entry.CompressedSize64, unzipRatio(entry.UncompressedSize64, entry.CompressedSize64), when, entry.CRC32, entry.Name)
		} else {
			fmt.Fprintf(stdout, "%9d  %s   %s\n", entry.UncompressedSize64, when, entry.Name)
		}
	}
	if r.quiet > 1 {
		return nil
	}
	if r.verbose {
		_, err := fmt.Fprintf(stdout, "--------          ------- ----%28s----\n%8d%17d%4d%%%28s%d files\n",
			"", size, compressed, unzipRatio(size, compressed), "", len(entries))
		return err
	}
	_, err := fmt.Fprintf(stdout, " --------%21s-------\n%9d%21s%d files\n", "", size, "", len(entries))
	return err
}

// unzipDOSTime is mm-dd-yyyy hh:mm of an MS-DOS date and time, field by field.
func unzipDOSTime(date, clock uint16) string {
	return fmt.Sprintf("%02d-%02d-%04d %02d:%02d", date>>5&0xf, date&0x1f, int(date>>9)+1980, clock>>11, clock>>5&0x3f)
}

// unzipMethod is -v's method column: Stored, Defl: and the level the flags hold, normal,
// maximum, fast or superfast, or the method's number.
func unzipMethod(entry *zip.File) string {
	switch entry.Method {
	case zip.Store:
		return "Stored"
	case zip.Deflate:
		return "Defl:" + string("NXFS"[entry.Flags>>1&3])
	}
	return fmt.Sprintf("%6d", entry.Method)
}

// unzipRatio is how much smaller compressing made the data, in whole percent, as busybox
// counts it: none where it grew.
func unzipRatio(size, compressed uint64) uint64 {
	if size == 0 || compressed > size {
		return 0
	}
	return (size - compressed) * 100 / size
}
