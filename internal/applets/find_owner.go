package applets

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
)

// find -user and -group, busybox's and GNU's, and GNU's -uid and -gid.
//
// A NAME is what ls -l prints in its owner column, which is the group column too, as
// busybox-w32 prints it: the account that owns the file, a number for one this machine cannot
// name, or root for an owner that is no user's account -- SYSTEM, Administrators. It is matched
// in any case, as Windows matches an account's name. A number is the uid ls -n prints, which is
// busybox-w32's: its default uid for what this account owns. busybox's own -user looks the NAME up
// with a getpwnam that knows only this account and root, so `find -user Administrators` found
// nothing it could explain; a NAME that is no account at all is GNU's error, before the walk.

// findOwner matches an entry's owner by name, by number, or by either.
type findOwner struct {
	name       string
	byID       bool
	comparison byte
	id         uint64
}

func (p *findParser) ownerPredicate(operand string) (findNode, error) {
	value, err := p.argument(operand)
	if err != nil {
		return nil, err
	}
	if operand == "-uid" || operand == "-gid" {
		comparison, digits := splitFindComparison(value)
		id, err := strconv.ParseUint(digits, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid number '%s'", value)
		}
		return findOwner{byID: true, comparison: comparison, id: id}, nil
	}
	if id, err := strconv.ParseUint(value, 10, 32); err == nil {
		return findOwner{name: value, byID: true, comparison: '=', id: id}, nil
	}
	if !strings.EqualFold(value, "root") {
		if _, _, _, err := windows.LookupSID("", value); err != nil {
			return nil, fmt.Errorf("'%s' is not the name of a known %s", value, strings.TrimPrefix(operand, "-"))
		}
	}
	return findOwner{name: value}, nil
}

func (n findOwner) eval(c findCandidate, _ *findRun) bool {
	record, ok := c.statRecord(0)
	switch {
	case !ok:
		return false
	case n.name != "" && strings.EqualFold(record.account, n.name):
		return true
	case !n.byID:
		return false
	case n.comparison == '+':
		return uint64(record.uid) > n.id
	case n.comparison == '-':
		return uint64(record.uid) < n.id
	}
	return uint64(record.uid) == n.id
}
