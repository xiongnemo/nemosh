package applets

import "math"

// diffCandidate is stone's struct cand, in its three ints: 12 bytes, where Go's ints made 24.
type diffCandidate struct{ x, y, pred int32 }

// diffCandidates is stone's clist, kept in blocks that no candidate is copied out of to grow.
// busybox's grows by a tenth with realloc; a slice appended to was the old array and the new at
// once, of 24-byte candidates, and a 19 MB file against its sorted copy peaked at 2,380 MiB,
// where busybox's peaked at 701. A candidate's index is an int32, as busybox's is an int.
type diffCandidates struct {
	blocks [][]diffCandidate
	count  int
}

// diffCandidateBlock is the candidates of a block, 48 KB of them; the first grows from busybox's
// 100 to it. Blocks of 65,536 took 29 bytes a candidate: append grows a large slice by a quarter
// at a time, so the first block was made five times over, and the last stood mostly empty.
const diffCandidateBlock = 1 << 12

func (c *diffCandidates) at(index int32) diffCandidate {
	place := uint32(index)
	return c.blocks[place/diffCandidateBlock][place%diffCandidateBlock]
}

// add keeps a candidate and answers its index.
func (c *diffCandidates) add(candidate diffCandidate) int32 {
	last := len(c.blocks) - 1
	if last < 0 || len(c.blocks[last]) == diffCandidateBlock {
		capacity := diffCandidateBlock
		if last < 0 {
			capacity = 100
		}
		c.blocks = append(c.blocks, make([]diffCandidate, 0, capacity))
		last++
	}
	c.blocks[last] = append(c.blocks[last], candidate)
	c.count++
	return int32(c.count - 1)
}

// diffStone is stone: the k-candidates of the left lines' classes, unravelled into J past the
// prefix, and how many candidates it made. Each line tries at most max(256, √n) of its class's
// lines unless minimal.
func diffStone(class, member []int32, matches []int, prefix int, minimal bool) int {
	n := len(class) - 1
	bound := math.MaxInt
	if !minimal {
		bound = max(256, diffSquareRoot(n))
	}
	var candidates diffCandidates
	candidates.add(diffCandidate{})
	// klist is busybox's, and ends the y of each of its candidates, which search reads and
	// reads again: kept beside it, the search does not go through the blocks, where it took 45%
	// longer than the slice the blocks replaced.
	klist, ends := make([]int32, n+2), make([]int32, n+2)
	k := 0
	for x := 1; x <= n; x++ {
		j := class[x]
		if j == 0 {
			continue
		}
		previous, tries := 0, 0
		candidate := diffCandidate{x: int32(x), y: -member[j], pred: klist[0]}
		for {
			if candidate.y > candidates.at(candidate.pred).y {
				l := diffSearch(ends, k, candidate.y)
				if l != previous+1 {
					candidate.pred = klist[l-1]
				}
				if l > k || ends[l] > candidate.y {
					displaced := klist[l]
					klist[l], ends[l] = candidates.add(candidate), candidate.y
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
	for q := candidates.at(klist[k]); q.y != 0; q = candidates.at(q.pred) {
		matches[int(q.x)+prefix] = int(q.y) + prefix
	}
	return candidates.count
}

// diffSearch is search: where among the k-candidates, which end at ends, one ending at y belongs.
func diffSearch(ends []int32, k int, y int32) int {
	if ends[k] < y {
		return k + 1
	}
	i, j := 0, k+1
	for {
		l := (i + j) >> 1
		if l <= i {
			return l + 1
		}
		switch t := ends[l]; {
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
