package applets

// statfsRecord is a struct statfs as busybox's stat -f prints one.
type statfsRecord struct {
	// id is f_fsid as coreutils reads it, an array of ints with the most significant first.
	id, nameLength, kind, blockSize           uint64
	blocks, free, available, files, freeFiles uint64
}

// statfsFormat is busybox's layout of a filesystem (coreutils/stat.c:500-506).
func statfsFormat(terse bool) string {
	if terse {
		return "%n %i %l %t %s %b %f %a %c %d"
	}
	return "  File: \"%n\"\n    ID: %-8i Namelen: %-7l Type: %T\nBlock size: %-10s\n" +
		"Blocks: Total: %-10b Free: %-10f Available: %a\nInodes: Total: %-10c Free: %d"
}

// conversion is busybox's print_statfs (coreutils/stat.c:260-304).
func (s statfsRecord) conversion(spec cSpec, verb byte, name string) string {
	number := func(base byte, value uint64) string { return cInteger(spec, base, false, value) }
	switch verb {
	case 'n':
		return statString(spec, name)
	case 'i':
		return number('x', s.id)
	case 'l':
		return number('u', s.nameLength)
	case 't':
		return number('x', s.kind)
	case 'T':
		return statString(spec, filesystemTypeName(s.kind))
	case 'b':
		return number('u', s.blocks)
	case 'f':
		return number('u', s.free)
	case 'a':
		return number('u', s.available)
	case 's', 'S':
		return number('u', s.blockSize)
	case 'c':
		return number('u', s.files)
	case 'd':
		return number('u', s.freeFiles)
	}
	return cPad(spec, "", string(verb), true)
}

// filesystemTypeName is busybox's human_fstype (coreutils/stat.c:167-227): the name of a magic
// number Linux's statfs answers, or of one busybox-w32 makes of what Windows calls a volume's
// filesystem, as win32/statfs.c does.
func filesystemTypeName(kind uint64) string {
	if name, ok := filesystemTypes[uint32(kind)]; ok {
		return name
	}
	return "UNKNOWN"
}

var filesystemTypes = map[uint32]string{
	0xADFF: "affs", 0x1CD1: "devpts", 0x137D: "ext", 0xEF51: "ext2", 0xEF53: "ext2/ext3",
	0x3153464a: "jfs", 0x58465342: "xfs", 0xF995E849: "hpfs", 0x9660: "isofs", 0x4000: "isofs",
	0x4004: "isofs", 0x137F: "minix", 0x138F: "minix (30 char.)", 0x2468: "minix v2",
	0x2478: "minix v2 (30 char.)", 0x4d44: "msdos", 0x4006: "fat", 0x564c: "novell", 0x6969: "nfs",
	0x9fa0: "proc", 0x517B: "smb", 0x012FF7B4: "xenix", 0x012FF7B5: "sysv4", 0x012FF7B6: "sysv2",
	0x012FF7B7: "coh", 0x15013346: "udf", 0x00011954: "ufs", 0x012FD16D: "xia", 0x5346544e: "ntfs",
	0x1021994: "tmpfs", 0x52654973: "reiserfs", 0x28cd3d45: "cramfs", 0x7275: "romfs",
	0x858458f6: "ramfs", 0x73717368: "squashfs", 0x62656572: "sysfs",
}
