package main

import (
	"fmt"
	"strings"
	"testing"
)

// Column-major, as busybox lays it out (libbb/lineedit.c:1279), so reading down
// a column is alphabetical.
func TestCandidateListing_fillsColumnsDownwards(t *testing.T) {
	// Given: six names, each 3 cells wide, at a width that holds four per row --
	// so two rows, and therefore three columns actually used.
	matches := []string{"aaa", "bbb", "ccc", "ddd", "eee", "fff"}

	// When
	listing := candidateListing(matches, 20)

	// Then: down each column, not across each row. Reading a column is then
	// alphabetical, which is the property the layout exists for.
	want := "aaa  ccc  eee\nbbb  ddd  fff\n"
	if listing != want {
		t.Fatalf("listing =\n%q\nwant\n%q", listing, want)
	}
}

// The column is as wide as the widest name, or names would run together.
func TestCandidateListing_sizesColumnsToTheLongestName(t *testing.T) {
	// Given
	matches := []string{"a", "bbbbbbbb", "c", "d"}

	// When
	listing := candidateListing(matches, 40)

	// Then: every row must keep the names apart by at least two blanks.
	for _, row := range strings.Split(strings.TrimRight(listing, "\n"), "\n") {
		if strings.Contains(row, "bbbbbbbb c") {
			t.Fatalf("names ran together: %q", row)
		}
	}
	if !strings.Contains(listing, "bbbbbbbb") {
		t.Fatalf("listing lost a name:\n%s", listing)
	}
}

// A listing that would fill the screen is not printed. Command completion
// reaches PATH, so a bare Tab has thousands of answers, and printing them
// scrolls away the session to say nothing the reader can use.
func TestCandidateListing_reportsACountRatherThanFillingTheScreen(t *testing.T) {
	// Given: more names than the row budget can hold at this width
	var many []string
	for index := range 4000 {
		many = append(many, fmt.Sprintf("name%04d", index))
	}

	// When
	listing := candidateListing(many, 80)

	// Then
	if listing != "4000 matches; type more to narrow\n" {
		t.Fatalf("listing = %q, want a count", listing)
	}
}

// And a listing that fits is printed, however close to the budget it comes.
func TestCandidateListing_printsWhatFits(t *testing.T) {
	// Given: at width 80 with 8-column names, nine per row -- 118 names is 14
	// rows, just inside the budget. This is the case that prompted the layout:
	// `w` has that many answers on an ordinary Windows machine.
	var many []string
	for index := range 118 {
		many = append(many, fmt.Sprintf("name%03d", index))
	}

	// When
	listing := candidateListing(many, 80)

	// Then
	rows := strings.Count(listing, "\n")
	if rows > listedRowLimit {
		t.Fatalf("listing took %d rows, over the %d budget", rows, listedRowLimit)
	}
	if !strings.Contains(listing, "name000") || !strings.Contains(listing, "name117") {
		t.Fatalf("listing lost a name:\n%s", listing)
	}
}

// A row must not be padded past the last column, or it wraps into an empty one.
func TestCandidateListing_doesNotPadPastTheLastColumn(t *testing.T) {
	// Given: names that exactly fill a row
	matches := []string{"aaaa", "bbbb", "cccc", "dddd"}

	// When: a width taking two per row
	listing := candidateListing(matches, 12)

	// Then
	for _, row := range strings.Split(strings.TrimRight(listing, "\n"), "\n") {
		if strings.HasSuffix(row, " ") {
			t.Fatalf("row %q is padded past its last name", row)
		}
	}
}

// A wide character costs two cells, and a column measured in characters would
// be too narrow for it.
func TestCandidateListing_measuresWideCharactersInCells(t *testing.T) {
	// Given
	matches := []string{"文档", "ab", "cd", "ef"}

	// When
	listing := candidateListing(matches, 40)

	// Then: the widest is 4 cells, so columns are 6 wide; `ab` in the next column
	// must start 6 cells after the start of `文档`.
	first := strings.SplitN(listing, "\n", 2)[0]
	if textColumns(first) != textColumns(strings.TrimRight(first, " ")) {
		t.Fatalf("trailing padding on %q", first)
	}
	if !strings.HasPrefix(first, "文档  ") {
		t.Fatalf("first row = %q, want the wide name padded to its cell width", first)
	}
}

func TestCandidateListing_isEmptyWithNothingToShow(t *testing.T) {
	if got := candidateListing(nil, 80); got != "" {
		t.Fatalf("listing = %q, want empty", got)
	}
}
