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
	if name, ok := filesystemTypes(uint32(kind)); ok {
		return name
	}
	return "UNKNOWN"
}

func filesystemTypes(key uint32) (string, bool) {
	switch key {
	case 0xADFF:
		return "affs", true
	case 0x1CD1:
		return "devpts", true
	case 0x137D:
		return "ext", true
	case 0xEF51:
		return "ext2", true
	case 0xEF53:
		return "ext2/ext3", true
	case 0x3153464a:
		return "jfs", true
	case 0x58465342:
		return "xfs", true
	case 0xF995E849:
		return "hpfs", true
	case 0x9660, 0x4000, 0x4004:
		return "isofs", true
	case 0x137F:
		return "minix", true
	case 0x138F:
		return "minix (30 char.)", true
	case 0x2468:
		return "minix v2", true
	case 0x2478:
		return "minix v2 (30 char.)", true
	case 0x4d44:
		return "msdos", true
	case 0x4006:
		return "fat", true
	case 0x564c:
		return "novell", true
	case 0x6969:
		return "nfs", true
	case 0x9fa0:
		return "proc", true
	case 0x517B:
		return "smb", true
	case 0x012FF7B4:
		return "xenix", true
	case 0x012FF7B5:
		return "sysv4", true
	case 0x012FF7B6:
		return "sysv2", true
	case 0x012FF7B7:
		return "coh", true
	case 0x15013346:
		return "udf", true
	case 0x00011954:
		return "ufs", true
	case 0x012FD16D:
		return "xia", true
	case 0x5346544e:
		return "ntfs", true
	case 0x1021994:
		return "tmpfs", true
	case 0x52654973:
		return "reiserfs", true
	case 0x28cd3d45:
		return "cramfs", true
	case 0x7275:
		return "romfs", true
	case 0x858458f6:
		return "ramfs", true
	case 0x73717368:
		return "squashfs", true
	case 0x62656572:
		return "sysfs", true
	}
	var zero string
	return zero, false
}
