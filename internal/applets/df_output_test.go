package applets

import (
	"bytes"
	"fmt"
	"testing"
)

// df takes GNU's -x TYPE, more than one -t, --output[=FIELDS] and -i: `df -hT -x tmpfs`, `df
// --output=source,size,pcent,target`. --output and -i print GNU's columns, as wide as what is in
// them; Windows keeps no inode count an unelevated session may read, so the inode fields are `-`,
// as GNU's df prints them for a filesystem that has none, vfat's.
func TestDf_gnuOutputAndSelections(t *testing.T) {
	rows := []filesystemUsage{
		{device: "C:", mount: "C:/", fsType: "NTFS", used: 600 << 30, available: 400 << 30, file: "."},
		{device: "D:", mount: "D:/", fsType: "FAT32", used: 1 << 30, available: 3 << 30},
	}
	kib := func(bytes uint64) string { return fmt.Sprint(bytes / 1024) }
	for _, test := range []struct {
		name   string
		layout diskFreeLayout
		fields []string
		want   string
	}{
		{name: "source size pcent target", layout: diskFreeLayout{unit: 1024}, fields: []string{"source", "size", "pcent", "target"},
			want: fmt.Sprintf("%-14s %10s %4s %s\n%-14s %10s %4s %s\n%-14s %10s %4s %s\n",
				"Filesystem", "1K-blocks", "Use%", "Mounted on",
				"C:", kib(1000<<30), "60%", "C:/",
				"D:", kib(4<<30), "25%", "D:/")},
		{name: "fstype used avail file, -h", layout: diskFreeLayout{unit: 0}, fields: []string{"fstype", "used", "avail", "file"},
			want: fmt.Sprintf("%-5s %6s %6s %s\n%-5s %6s %6s %s\n%-5s %6s %6s %s\n",
				"Type", "Used", "Avail", "File",
				"NTFS", "600.0G", "400.0G", ".",
				"FAT32", "1.0G", "3.0G", "-")},
		{name: "-i", layout: diskFreeLayout{unit: 1024}, fields: dfInodeFields,
			want: fmt.Sprintf("%-14s %6s %5s %5s %5s %s\n%-14s %6s %5s %5s %5s %s\n%-14s %6s %5s %5s %5s %s\n",
				"Filesystem", "Inodes", "IUsed", "IFree", "IUse%", "Mounted on",
				"C:", "-", "-", "-", "-", "C:/",
				"D:", "-", "-", "-", "-", "D:/")},
	} {
		var stdout bytes.Buffer
		if err := writeDiskFreeFields(&stdout, rows, test.layout, test.fields); err != nil || stdout.String() != test.want {
			t.Errorf("%s:\n%s\nwant:\n%s", test.name, stdout.String(), test.want)
		}
	}
	// -t chooses types and -x leaves them out, without regard to case, as many of each as given.
	for _, test := range []struct {
		only, exclude []string
		want          int
	}{
		{only: []string{"ntfs"}, want: 1},
		{only: []string{"NTFS", "fat32"}, want: 2},
		{exclude: []string{"FAT32"}, want: 1},
		{exclude: []string{"tmpfs"}, want: 2},
	} {
		if got := len(selectFilesystems(rows, diskFreeLayout{only: test.only, exclude: test.exclude})); got != test.want {
			t.Errorf("-t %v -x %v kept %d, want %d", test.only, test.exclude, got, test.want)
		}
	}
}

func TestDf_readsOutputFields(t *testing.T) {
	for _, test := range []struct {
		args   []string
		fields []string
		rest   []string
		says   string
	}{
		{args: []string{"--output", "."}, fields: dfAllFields, rest: []string{"."}},
		{args: []string{"--output=source,target", "-h"}, fields: []string{"source", "target"}, rest: []string{"-h"}},
		{args: []string{"-hi", "C:"}, rest: []string{"-hi", "C:"}},
		{args: []string{"--output=bogus"}, says: "--output: field 'bogus' unknown"},
		{args: []string{"--output=source,source"}, says: "--output: field 'source' used more than once"},
	} {
		fields, rest, err := dfOutputFields(test.args)
		switch {
		case test.says != "":
			if err == nil || err.Error() != test.says {
				t.Errorf("%q: %v, want %q", test.args, err, test.says)
			}
		case err != nil || fmt.Sprint(fields) != fmt.Sprint(test.fields) || fmt.Sprint(rest) != fmt.Sprint(test.rest):
			t.Errorf("%q: %v %v %v", test.args, fields, rest, err)
		}
	}
}
