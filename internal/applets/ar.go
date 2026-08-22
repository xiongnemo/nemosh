package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ar: the oldest archive format still in use, and the one a `.deb` is made of.
//
// A `.deb` is an ar archive holding `debian-binary`, `control.tar.*` and
// `data.tar.*`, so `ar` plus this build's `tar` and `gzip` is enough to take one
// apart. That is the reason to have it; nobody has used ar for static libraries in
// a long time, and Windows' own `lib.exe` writes a different variant.
//
// The interface is a *verb* rather than an option, which is unusual and is both
// references' shape: `ar x|p|t|r [-ov] ARCHIVE [FILE]...`. The dash spelling
// (`ar -t`) is accepted too, because busybox accepts it and the muscle memory is
// hard to argue with.

func newArApplet() Applet {
	return simpleApplet{name: "ar", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		verb, rest, err := arVerb(args)
		if err != nil {
			return err
		}
		options, operands, err := parseAppletOptions(rest, "xptrov", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return fmt.Errorf("an archive operand is required")
		}
		request := arRequest{
			verb:     verb,
			verbose:  options.has('v'),
			keepTime: options.has('o'),
			archive:  operands[0],
			members:  operands[1:],
			view:     ProcessViewFromContext(ctx),
		}
		return request.run(stdout, stderr)
	}}
}

// arVerb takes the operation off the front.
//
// **Only the first word is looked at**, which is both references' rule -- GNU's
// synopsis is `ar [-]{dmpqrstx}[...] archive` and busybox's is
// `ar x|p|t|r [-ov] ARCHIVE`. Scanning every argument instead looked more
// forgiving and was a bug: a Windows temporary path contains a `p` in `AppData`,
// so `ar -o C:\Users\...\AppData\...` read the *operand* as the verb, removed a
// letter from the middle of it, and handed the mangled remainder back to the
// option parser as `-C:\Users\...`.
func arVerb(args []string) (byte, []string, error) {
	if len(args) == 0 {
		return 0, nil, fmt.Errorf("one of x, p, t or r is required")
	}
	first := strings.TrimPrefix(args[0], "-")
	// Every letter of the first word has to be one this applet knows, checked
	// before the verb is looked for. That is what makes `ar -Z lib.a` name the bad
	// letter rather than the missing verb, and it is also what stops
	// `ar libtest.a` -- a real mistake, with no verb at all -- from finding the `t`
	// in the middle of the file name and listing the archive. GNU refuses that
	// invocation on its `l`, which is the same answer.
	for index := 0; index < len(first); index++ {
		if !containsByte("xptrov", first[index]) {
			return 0, nil, invalidOption(first[index])
		}
	}
	for _, letter := range []byte("xptr") {
		if strings.IndexByte(first, letter) < 0 {
			continue
		}
		// The verb may be clustered with the options -- `ar tv lib.a` and
		// `ar -tv lib.a` are both real -- so only the verb letter is removed and
		// the rest goes back for the option parser.
		rest := []string{}
		if remainder := strings.Replace(first, string(letter), "", 1); remainder != "" {
			rest = append(rest, "-"+remainder)
		}
		return letter, append(rest, args[1:]...), nil
	}
	return 0, nil, fmt.Errorf("one of x, p, t or r is required, as the first argument")
}

type arRequest struct {
	verb     byte
	verbose  bool
	keepTime bool
	archive  string
	members  []string
	view     ProcessView
}

func (r arRequest) run(stdout, stderr io.Writer) error {
	native, err := resolveHostPath(r.view, r.archive)
	if err != nil {
		return operandFailure(r.archive, err)
	}
	if r.verb == 'r' {
		return r.create(native, stderr)
	}
	file, err := os.Open(native)
	if err != nil {
		return operandFailure(r.archive, err)
	}
	defer file.Close()
	return r.read(bufio.NewReader(file), stdout, stderr)
}

func (r arRequest) read(reader io.Reader, stdout, stderr io.Writer) error {
	if err := readArMagic(reader); err != nil {
		return operandFailure(r.archive, err)
	}
	collisions := newArchiveCollisions()
	longNames := ""
	for {
		member, err := readArHeader(reader)
		if err != nil {
			return err
		}
		if member == nil {
			return nil
		}
		// The GNU long-name table is a member in its own right, named `//`, and it
		// has to be *read* before the entries that refer into it. It is data, not
		// content, so it is never listed or written out.
		if member.name == "//" {
			table, err := readArBody(reader, *member)
			if err != nil {
				return err
			}
			longNames = string(table)
			continue
		}
		name, err := resolveArName(member.name, longNames)
		if err != nil {
			// Skipped rather than fatal, which is the same rule every archiver
			// here follows: one bad member must not cost the honest ones. The
			// header gave the size, so the stream stays aligned across the skip --
			// and a member whose name cannot be resolved is precisely one that
			// must not be written, since there is no name to write it under.
			//
			// This is also how a stored `/absolute.txt` lands here: a leading
			// slash means "offset into the long-name table", so an absolute name
			// is not a path at all but an unreadable reference.
			fmt.Fprintf(stderr, "ar: skipping %v\n", err)
			if err := skipArBody(reader, *member); err != nil {
				return err
			}
			continue
		}
		// The `/` member is the symbol table, which is an index of the archive
		// rather than a file in it.
		if name == "/" || name == "" {
			if err := skipArBody(reader, *member); err != nil {
				return err
			}
			continue
		}
		if !r.selects(name) {
			if err := skipArBody(reader, *member); err != nil {
				return err
			}
			continue
		}
		if err := r.oneMember(reader, *member, name, collisions, stdout, stderr); err != nil {
			return err
		}
	}
}

func (r arRequest) selects(name string) bool {
	if len(r.members) == 0 {
		return true
	}
	for _, wanted := range r.members {
		if wanted == name || filepath.Base(wanted) == name {
			return true
		}
	}
	return false
}

func (r arRequest) oneMember(reader io.Reader, member arMember, name string,
	collisions *archiveCollisions, stdout, stderr io.Writer) error {
	switch r.verb {
	case 't':
		// Unchecked, like every other listing here: inspecting an archive one does
		// not trust is exactly when the hostile name must still be visible.
		if err := r.writeListing(member, name, stdout); err != nil {
			return err
		}
		return skipArBody(reader, member)
	case 'p':
		if _, err := io.CopyN(stdout, reader, member.size); err != nil {
			return err
		}
		return skipArPadding(reader, member.size)
	}
	return r.extract(reader, member, name, collisions, stderr)
}

func (r arRequest) writeListing(member arMember, name string, stdout io.Writer) error {
	if !r.verbose {
		_, err := fmt.Fprintln(stdout, name)
		return err
	}
	// Six columns for the size and no type character, both measured against
	// busybox rather than guessed: ar has no directories, so there is nothing for a
	// leading `d` or `-` to distinguish.
	_, err := fmt.Fprintf(stdout, "%s %d/%d %6d %s %s\n",
		arModeString(member.mode), member.uid, member.gid, member.size,
		arListingTime(member.mtime), name)
	return err
}
