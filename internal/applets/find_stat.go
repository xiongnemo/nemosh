package applets

import (
	"fmt"
	"strconv"
)

// The predicates that ask of an entry what stat answers: its permissions, its identity and its
// links, and whether it can be run. Each is busybox's, and asks what busybox-w32's stat reports,
// as `stat` here does: the permissions made up from the attributes and the umask, the volume
// serial number and the file index for the device and the inode, and the count of hard links.

// findPerm is -perm MODE: exactly MODE's bits, -MODE all of them, and /MODE or +MODE any of
// them, as busybox's func_perm has it. MODE is octal or chmod's symbolic form, read as
// bb_parse_mode reads it starting from no bits, so `-perm -u+x` asks for the owner's x.
type findPerm struct {
	kind  byte
	mask  uint32
	umask uint32
}

func (p *findParser) permPredicate(operand string) (findNode, error) {
	value, err := p.argument(operand)
	if err != nil {
		return nil, err
	}
	kind, spec := byte('='), value
	if spec != "" && (spec[0] == '-' || spec[0] == '+' || spec[0] == '/') {
		kind, spec = spec[0], spec[1:]
	}
	umask := processFileModeMask(p.view)
	mask, ok := applyChmodMode(spec, 0, umask, false)
	if !ok {
		return nil, fmt.Errorf("invalid mode '%s'", spec)
	}
	return findPerm{kind: kind, mask: mask, umask: umask}, nil
}

func (n findPerm) eval(c findCandidate, _ *findRun) bool {
	record, ok := c.statRecord(n.umask)
	if !ok {
		return false
	}
	mode := record.mode & 0o7777
	switch n.kind {
	case '+', '/':
		return mode&n.mask != 0
	case '-':
		return mode&n.mask == n.mask
	}
	return mode == n.mask
}

// findIdentity is -inum N, the file index, and -samefile FILE, the same index on the same
// volume as FILE's.
type findIdentity struct {
	device, inode uint64
	sameVolume    bool
}

func (p *findParser) inumPredicate(operand string) (findNode, error) {
	value, err := p.argument(operand)
	if err != nil {
		return nil, err
	}
	inode, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number '%s'", value)
	}
	return findIdentity{inode: inode}, nil
}

// samefilePredicate stats FILE when the expression is read, as -newer does: a FILE that is not
// there is an error about the expression, before the walk.
func (p *findParser) samefilePredicate(operand string) (findNode, error) {
	name, err := p.argument(operand)
	if err != nil {
		return nil, err
	}
	host, err := resolveHostPath(p.view, name)
	if err != nil {
		return nil, operandFailure(name, err)
	}
	record, err := hostStatRecord(host, false, 0)
	if err != nil {
		// busybox's words: `find: can't stat 'nope': No such file or directory`.
		return nil, cannotStat(name, err)
	}
	return findIdentity{device: record.device, inode: record.inode, sameVolume: true}, nil
}

func (n findIdentity) eval(c findCandidate, _ *findRun) bool {
	record, ok := c.statRecord(0)
	return ok && record.inode == n.inode && (!n.sameVolume || record.device == n.device)
}

// findLinks is -links N, +N or -N: the count of hard links.
type findLinks struct {
	comparison byte
	count      uint64
}

func (p *findParser) linksPredicate(operand string) (findNode, error) {
	value, err := p.argument(operand)
	if err != nil {
		return nil, err
	}
	comparison, digits := splitFindComparison(value)
	count, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number '%s'", value)
	}
	return findLinks{comparison: comparison, count: count}, nil
}

func (n findLinks) eval(c findCandidate, _ *findRun) bool {
	record, ok := c.statRecord(0)
	if !ok {
		return false
	}
	switch n.comparison {
	case '+':
		return record.links > n.count
	case '-':
		return record.links < n.count
	}
	return record.links == n.count
}

// findExecutable is -executable: an entry that could be run, as `test -x` judges one.
type findExecutable struct{}

func (findExecutable) eval(c findCandidate, _ *findRun) bool {
	info, err := c.info()
	return err == nil && c.host != "" && isExecutableFile(c.host, info)
}

// statRecord is what stat says of the candidate, without following a link, as busybox's find
// asks lstat. A synthetic device entry has nothing to ask.
func (c findCandidate) statRecord(umask uint32) (statRecord, bool) {
	if c.host == "" {
		return statRecord{}, false
	}
	record, err := hostStatRecord(c.host, false, umask)
	return record, err == nil
}
