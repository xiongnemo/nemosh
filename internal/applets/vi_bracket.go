package applets

import (
	"strings"
	"time"
)

// Brackets: %, and showmatch's glimpse of a pair as one is typed.

// matchBracket is busybox's %: the first of ()[]{} from the cursor to the line's end, then its
// pair, nested pairs counted.
func (e *viEditor) matchBracket() {
	line := e.line()
	for col := e.col; col < len(line); col++ {
		if strings.ContainsRune("()[]{}", line[col]) {
			if pair, ok := e.pairOf(viPos{e.row, col}); ok {
				e.setCursor(pair)
				return
			}
			break
		}
	}
	e.beep()
}

// pairOf is busybox's find_pair: where the bracket at p is closed or opened, pairs inside it
// counted.
func (e *viEditor) pairOf(p viPos) (viPos, bool) {
	at := strings.IndexRune("()[]{}", e.charAt(p))
	if at < 0 {
		return p, false
	}
	found, pair, dir, level := e.charAt(p), rune("()[]{}"[at^1]), 1, 1
	if at%2 == 1 {
		dir = -1
	}
	for {
		next, ok := e.step(p, dir)
		if !ok {
			return p, false
		}
		p = next
		switch e.charAt(p) {
		case found:
			level++
		case pair:
			if level--; level == 0 {
				return p, true
			}
		}
	}
}

// showMatching is showmatch, busybox's showmatching: the cursor goes to the pair of the bracket
// just typed for a moment, or it beeps when there is none.
func (e *viEditor) showMatching() {
	pair, ok := e.pairOf(viPos{e.row, e.col - 1})
	if !ok {
		e.beep()
		return
	}
	if e.screen == nil {
		return
	}
	at := e.cursor()
	e.setCursor(pair)
	e.draw()
	time.Sleep(e.matchPause)
	e.setCursor(at)
}
