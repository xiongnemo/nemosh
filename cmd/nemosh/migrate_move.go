package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/xiongnemo/nemosh/internal/digest"
	"golang.org/x/sys/windows"
)

// moveFile moves one file of an item, never over another: the same content on both sides
// removes the old copy, and a different one keeps both and counts a conflict.
func (m *migrateRun) moveFile(what, from, to string) {
	if _, err := os.Stat(to); err == nil {
		same, err := sameContent(from, to)
		switch {
		case err != nil:
			fmt.Fprintf(m.stderr, "nemosh: --migrate: %s: %v\n", what, err)
			m.conflicts++
		case same:
			m.say("removed the old copy, the same as the new:", "would remove the old copy, the same as the new:", from, to)
			if !m.dryRun {
				_ = os.Remove(from)
			}
		default:
			m.conflicts++
			m.say("conflict, both kept:", "conflict, both would be kept:", from, to)
		}
		return
	}
	m.say("moved", "would move", from, to)
	if m.dryRun {
		return
	}
	if err := m.relocate(from, to); err != nil {
		fmt.Fprintf(m.stderr, "nemosh: --migrate: %s: %v\n", what, err)
		m.conflicts++
	}
}

// relocate renames from to to, and across volumes copies it, checks the copy byte for byte by
// size and hash, and only then removes from. The directory it goes in is made, 0700.
func (m *migrateRun) relocate(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
		return err
	}
	err := m.rename(from, to)
	if err == nil || !errors.Is(err, windows.ERROR_NOT_SAME_DEVICE) {
		return err
	}
	if err := copyKeepingTime(from, to); err != nil {
		_ = os.Remove(to)
		return err
	}
	if same, err := sameContent(from, to); err != nil || !same {
		_ = os.Remove(to)
		return fmt.Errorf("the copy of %s does not match it, so it is left where it was", from)
	}
	return os.Remove(from)
}

// copyKeepingTime copies from to to, which must not be there, with from's mode and time.
func copyKeepingTime(from, to string) error {
	source, err := os.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return err
	}
	target, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		return err
	}
	if err := target.Close(); err != nil {
		return err
	}
	return os.Chtimes(to, info.ModTime(), info.ModTime())
}

// sameContent is whether two files hold the same bytes, by size and then by SHA-256, CNG's.
func sameContent(a, b string) (bool, error) {
	first, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	second, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	if first.Size() != second.Size() {
		return false, nil
	}
	hashA, err := fileHash(a)
	if err != nil {
		return false, err
	}
	hashB, err := fileHash(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(hashA, hashB), nil
}

func fileHash(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	hash := digest.SHA256()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}
