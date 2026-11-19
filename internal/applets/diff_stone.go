package applets

import (
	"cmp"
	"math"
	"slices"
	"strings"
)

// The edit script is found as busybox-w32's diff finds it, by the Unix diff of Hunt and
// McIlroy: each line is hashed, the lines of the two files are put in classes by hash, stone
// finds the longest run of matched lines it can through those classes, and a second reading
// breaks any match two different lines made by sharing a hash. Its memory is the lines'. It
// was a table of every line of one file against every line of the other, which for Go's
// ssa/opGen.go against rewriteAMD64.go wanted 103 GB.
//
// This is editors/diff.c's create_J, stone, search, equiv and unsort, and diff's walk of J,
// in its one-based arrays, so that each step can be read against it. It is ported rather than
// replaced by another algorithm because a diff has many right answers, and busybox's choice
// among them -- including where stone stops looking, after max(256, √n) tries a line unless -d
// asks for the least diff -- is the one this one prints.

// diffEOF is the token read_token makes of the end of a file with no newline before it.
const diffEOF = 0x1ff

// diffKey is a line as read_token reads it under -i, -w and -b, as a string two lines share
// only when it reads the same of both, and the hash create_J takes of it, Sedgewick's: the
// newline that ends the line is read, or EOF where the last line has none; -i lowers A to Z
// alone; -w passes over all white space, and -b reads a run of it as one space, the newline
// or EOF among it. The key spells EOF as two 0xff bytes. A line ends in a newline or an EOF
// whenever EOF is read at all, so no line's key is another's.
func (r diffRequest) diffKey(line string) (string, int) {
	ended := strings.HasSuffix(line, "\n")
	plain := !r.ignoreCase && !r.ignoreAll && !r.ignoreSpace
	var kept []byte
	if !plain {
		kept = make([]byte, 0, len(line)+2)
	}
	hash, space := uint32(0), false
	for index := 0; index < len(line) || index == len(line) && !ended; index++ {
		token := diffEOF
		if index < len(line) {
			token = int(line[index])
		}
		blank := token == diffEOF || diffSpace(byte(token))
		if r.ignoreCase && 'A' <= token && token <= 'Z' {
			token += 'a' - 'A'
		}
		if r.ignoreAll && blank {
			continue
		}
		if r.ignoreSpace {
			if space && blank {
				continue
			}
			space = blank
			if blank {
				token = ' '
			}
		}
		hash = hash*127 + uint32(token)
		if !plain {
			if token == diffEOF {
				kept = append(kept, 0xff, 0xff)
			} else {
				kept = append(kept, byte(token))
			}
		}
	}
	key := line
	switch {
	case !plain:
		key = string(kept)
	case !ended:
		key = line + "\xff\xff"
	}
	return key, int(hash & math.MaxInt32)
}

// diffLine is create_J's struct line: a line's place among the pruned lines, and its hash,
// which equiv replaces with its class.
type diffLine struct{ serial, value int }

// diffMatches is create_J's J: for each line of the left, one-based, the line of the right it
// is matched with, or 0, with J[0] = 0 and J[n+1] = m+1 at either end.
func diffMatches(keys [2][]string, hashes [2][]int, minimal bool) []int {
	n, m := len(hashes[0]), len(hashes[1])
	prefix, suffix := 0, 0
	for prefix < n && prefix < m && hashes[0][prefix] == hashes[1][prefix] {
		prefix++
	}
	for suffix < n-prefix && suffix < m-prefix && hashes[0][n-1-suffix] == hashes[1][m-1-suffix] {
		suffix++
	}
	var sorted [2][]diffLine
	for side := range sorted {
		count := len(hashes[side]) - prefix - suffix
		lines := make([]diffLine, count+1)
		for index := 1; index <= count; index++ {
			lines[index] = diffLine{serial: index, value: hashes[side][prefix+index-1]}
		}
		slices.SortFunc(lines[1:], func(a, b diffLine) int {
			return cmp.Or(cmp.Compare(a.value, b.value), cmp.Compare(a.serial, b.serial))
		})
		sorted[side] = lines
	}
	member := diffEquiv(sorted[0], sorted[1])
	class := make([]int, len(sorted[0]))
	for _, line := range sorted[0][1:] {
		class[line.serial] = line.value
	}
	matches := make([]int, n+2)
	for index := 0; index <= n; index++ {
		switch {
		case index <= prefix:
			matches[index] = index
		case index > n-suffix:
			matches[index] = index + m - n
		}
	}
	diffStone(class, member, matches, prefix, minimal)
	matches[n+1] = m + 1
	// The lines are read again, a run of matched ones at a time, and a pair whose lines differ
	// though their hashes agree is matched no more. busybox's loop steps past the line that
	// ends each run without reading it, and so does this one.
	for index := 1; index <= n; index++ {
		if matches[index] == 0 {
			continue
		}
		for other := matches[index]; index <= n && matches[index] == other; index, other = index+1, other+1 {
			if keys[0][index-1] != keys[1][other-1] {
				matches[index] = 0
			}
		}
	}
	return matches
}

// diffEquiv is equiv: each left line's value becomes the place among the sorted right lines
// of the first with its hash, or 0 where none has it, and member lists the right lines of each
// class in order, the first of each negated and -1 after the last. busybox ends a class at a
// sentinel of 0, which a line of hash 0 under -w would run past; this stops at the last line.
func diffEquiv(a, b []diffLine) []int {
	n, m := len(a)-1, len(b)-1
	i, j := 1, 1
	for i <= n && j <= m {
		switch {
		case a[i].value < b[j].value:
			a[i].value = 0
			i++
		case a[i].value == b[j].value:
			a[i].value = j
			i++
		default:
			j++
		}
	}
	for ; i <= n; i++ {
		a[i].value = 0
	}
	member := make([]int, m+2)
	for j = 1; j <= m; j++ {
		member[j] = -b[j].serial
		for j < m && b[j+1].value == b[j].value {
			j++
			member[j] = b[j].serial
		}
	}
	member[j] = -1
	return member
}

// diffCandidate is stone's struct cand.
type diffCandidate struct{ x, y, pred int }

// diffStone is stone: the k-candidates of the left lines' classes, unravelled into J past the
// prefix. Each line tries at most max(256, √n) of its class's lines unless minimal.
func diffStone(class, member, matches []int, prefix int, minimal bool) {
	n := len(class) - 1
	bound := math.MaxInt
	if !minimal {
		bound = max(256, diffSquareRoot(n))
	}
	candidates := make([]diffCandidate, 1, 100)
	klist := make([]int, n+2)
	k := 0
	for x := 1; x <= n; x++ {
		j := class[x]
		if j == 0 {
			continue
		}
		previous, tries := 0, 0
		candidate := diffCandidate{x: x, y: -member[j], pred: klist[0]}
		for {
			if candidate.y > candidates[candidate.pred].y {
				l := diffSearch(klist, k, candidate.y, candidates)
				if l != previous+1 {
					candidate.pred = klist[l-1]
				}
				if l > k || candidates[klist[l]].y > candidate.y {
					candidates = append(candidates, candidate)
					displaced := klist[l]
					klist[l] = len(candidates) - 1
					if l > k {
						k++
						break
					}
					candidate.pred, previous = displaced, l
					tries++
				}
			}
			j++
			if candidate.y = member[j]; candidate.y <= 0 || tries >= bound {
				break
			}
		}
	}
	for q := candidates[klist[k]]; q.y != 0; q = candidates[q.pred] {
		matches[q.x+prefix] = q.y + prefix
	}
}

// diffSearch is search: where among the k-candidates one ending at y belongs.
func diffSearch(c []int, k, y int, list []diffCandidate) int {
	if list[c[k]].y < y {
		return k + 1
	}
	i, j := 0, k+1
	for {
		l := (i + j) >> 1
		if l <= i {
			return l + 1
		}
		switch t := list[c[l]].y; {
		case t > y:
			j = l
		case t < y:
			i = l
		default:
			return l
		}
	}
}

// diffSquareRoot is libbb's isqrt, the floor of the square root.
func diffSquareRoot(n int) int {
	root := int(math.Sqrt(float64(n)))
	for root*root > n {
		root--
	}
	for (root+1)*(root+1) <= n {
		root++
	}
	return root
}

// diffEditsOf walks J as diff does, into the blocks of lines one file has and the other has
// not, and writes them as the edit script: the left's lines of a block removed, then the
// right's added, between the lines both keep.
func diffEditsOf(matches []int, left, right []string) []diffEdit {
	edits := make([]diffEdit, 0, len(left)+len(right))
	next := [2]int{1, 1}
	keep := func(to int) {
		for ; next[0] < to; next[0], next[1] = next[0]+1, next[1]+1 {
			edits = append(edits, diffEdit{kind: editKeep, text: left[next[0]-1], leftLine: next[0], rightLine: next[1]})
		}
	}
	for at := 1; ; {
		a := at
		for a <= len(left) && matches[a] == matches[a-1]+1 {
			a++
		}
		c := matches[a-1] + 1
		b := a - 1
		for b < len(left) && matches[b+1] == 0 {
			b++
		}
		d := matches[b+1] - 1
		if a <= b || c <= d {
			keep(a)
			for line := a; line <= b; line++ {
				edits = append(edits, diffEdit{kind: editRemove, text: left[line-1], leftLine: line, rightLine: c - 1})
			}
			for line := c; line <= d; line++ {
				edits = append(edits, diffEdit{kind: editAdd, text: right[line-1], leftLine: b, rightLine: line})
			}
			next = [2]int{b + 1, d + 1}
		}
		if at = b + 1; at > len(left) {
			break
		}
		matches[b] = d
	}
	keep(len(left) + 1)
	return edits
}
