package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// uuencode and uudecode: how a binary travelled through a text-only channel
// before base64 was everywhere, and still how a few tools ship attachments.
//
// The wire format is fixed and old:
//
//	begin 644 name.txt
//	&:&5L;&\*
//	`
//	end
//
// Each line encodes up to 45 bytes as groups of three into four characters, with
// a leading length character. The lone backtick is a zero-length line, which is
// what marks the end -- and it is a backtick rather than a space because trailing
// spaces do not survive mail.

func newUuencodeApplet() Applet {
	return simpleApplet{name: "uuencode", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "m", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return fmt.Errorf("a name for the decoded file is required")
		}
		// The last operand is the name to record; anything before it is the file
		// to read. That ordering is the format's, not a choice: the name in the
		// header is where uudecode will write.
		source := operands[:len(operands)-1]
		recorded := operands[len(operands)-1]
		if options.has('m') {
			return fmt.Errorf("base64 output is not implemented; use `base64` instead")
		}
		return eachTextInput(ctx, source, stdin, func(reader io.Reader) error {
			return writeUuencoded(stdout, reader, recorded)
		})
	}}
}

func writeUuencoded(stdout io.Writer, reader io.Reader, name string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "begin 644 %s\n", name); err != nil {
		return err
	}
	for offset := 0; offset < len(data); offset += 45 {
		chunk := data[offset:min(offset+45, len(data))]
		if _, err := fmt.Fprintln(stdout, uuencodeLine(chunk)); err != nil {
			return err
		}
	}
	_, err = fmt.Fprint(stdout, "`\nend\n")
	return err
}

// uuencodeLine encodes up to 45 bytes.
//
// Every six bits become one character at 0x20 plus the value, except that zero
// becomes a backtick rather than a space -- trailing spaces do not survive mail,
// which is the whole reason the substitution exists.
func uuencodeLine(chunk []byte) string {
	var out strings.Builder
	out.WriteByte(uuencodeByte(byte(len(chunk))))
	for index := 0; index < len(chunk); index += 3 {
		var group [3]byte
		copy(group[:], chunk[index:min(index+3, len(chunk))])
		value := uint32(group[0])<<16 | uint32(group[1])<<8 | uint32(group[2])
		for shift := 18; shift >= 0; shift -= 6 {
			out.WriteByte(uuencodeByte(byte(value >> shift & 0o77)))
		}
	}
	return out.String()
}

func uuencodeByte(value byte) byte {
	if value == 0 {
		return '`'
	}
	return value + 0x20
}

func newUudecodeApplet() Applet {
	return simpleApplet{name: "uudecode", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(args, "", "o")
		if err != nil {
			return err
		}
		return eachTextInput(ctx, paths, stdin, func(reader io.Reader) error {
			return decodeUuencoded(ctx, stdout, reader, options)
		})
	}}
}

// decodeUuencoded reads the header, the body and the terminator.
//
// The destination is the name in the header unless -o overrides it. `-o -` means
// stdout, which is how the decoded bytes are piped somewhere rather than landing
// in whatever file the sender happened to name -- and that matters, because the
// sender chose that name.
func decodeUuencoded(ctx context.Context, stdout io.Writer, reader io.Reader, options appletOptions) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	name := ""
	started := false
	var decoded []byte
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if !started {
			if fields := strings.Fields(line); len(fields) == 3 && fields[0] == "begin" {
				name, started = fields[2], true
			}
			continue
		}
		if line == "end" {
			break
		}
		if line == "" || line == "`" {
			continue
		}
		chunk, err := uudecodeLine(line)
		if err != nil {
			return err
		}
		decoded = append(decoded, chunk...)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !started {
		return fmt.Errorf("no `begin' line found")
	}
	return writeDecodedUu(ctx, stdout, decoded, name, options)
}

func writeDecodedUu(ctx context.Context, stdout io.Writer, decoded []byte, name string, options appletOptions) error {
	target := name
	if options.has('o') {
		target = options.value('o')
	}
	if target == "-" || target == "" {
		_, err := stdout.Write(decoded)
		return err
	}
	// The name came from the sender, so it is checked the way an archive entry is
	// -- `begin 644 ../../evil` is the same attack a tar entry would be.
	safe, err := safeArchivePath(target)
	if err != nil {
		return err
	}
	native, err := resolveHostPath(ProcessViewFromContext(ctx), safe)
	if err != nil {
		return operandFailure(safe, err)
	}
	return os.WriteFile(native, decoded, 0o644)
}

func uudecodeLine(line string) ([]byte, error) {
	length := int(uudecodeByte(line[0]))
	if length > 45 {
		return nil, fmt.Errorf("invalid length character in uuencoded data")
	}
	body := line[1:]
	var out []byte
	for index := 0; index+3 < len(body)+3 && len(out) < length; index += 4 {
		var group [4]byte
		for offset := range 4 {
			if index+offset < len(body) {
				group[offset] = uudecodeByte(body[index+offset])
			}
		}
		value := uint32(group[0])<<18 | uint32(group[1])<<12 | uint32(group[2])<<6 | uint32(group[3])
		for shift := 16; shift >= 0 && len(out) < length; shift -= 8 {
			out = append(out, byte(value>>shift))
		}
	}
	if len(out) != length {
		return nil, fmt.Errorf("truncated uuencoded line")
	}
	return out, nil
}

func uudecodeByte(character byte) byte {
	if character == '`' {
		return 0
	}
	return (character - 0x20) & 0o77
}
