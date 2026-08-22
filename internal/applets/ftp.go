package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// ftpget and ftpput: one file each way over FTP.
//
// Go has no FTP client in its standard library, so this is the protocol directly.
// It is a small one: a text control connection, and a second connection for the
// data. Passive mode only -- active mode asks the *server* to connect back to us,
// which no machine behind a router or a Windows firewall can accept, so offering
// it would be offering something that mostly fails.

func newFtpgetApplet() Applet {
	return newFtpApplet("ftpget")
}

func newFtpputApplet() Applet {
	return newFtpApplet("ftpput")
}

func newFtpApplet(name string) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(args, "cv", "upP")
		if err != nil {
			return err
		}
		if len(operands) < 2 {
			return fmt.Errorf("a host and a file are required")
		}
		port := options.value('P')
		if port == "" {
			port = "21"
		}
		session := &ftpSession{
			host:    net.JoinHostPort(operands[0], port),
			user:    valueOrDefault(options.value('u'), "anonymous"),
			pass:    valueOrDefault(options.value('p'), "nemosh@"),
			verbose: options.has('v'),
			log:     stderr,
		}
		// The two argument orders are *mirrors* of each other, which is the trap:
		//
		//   ftpget HOST [LOCAL]  REMOTE
		//   ftpput HOST [REMOTE] LOCAL
		//
		// so in both cases the file being *written* is named first. With one file
		// operand the two collapse to the same name, which is busybox's shape and
		// also why having them reversed for ftpput went unnoticed -- every test that
		// existed passed either one operand or none.
		first := operands[1]
		second := first
		if len(operands) > 2 {
			second = operands[2]
		}
		if err := session.connect(ctx); err != nil {
			return err
		}
		defer session.close()
		if name == "ftpget" {
			// first is local, second is remote.
			return session.retrieve(ctx, second, first)
		}
		// first is remote, second is local.
		return session.store(ctx, first, second)
	}}
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

type ftpSession struct {
	host       string
	user, pass string
	verbose    bool
	log        io.Writer
	control    net.Conn
	reader     *bufio.Reader
}

func (s *ftpSession) connect(ctx context.Context) error {
	dialer := net.Dialer{Timeout: 30 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", s.host)
	if err != nil {
		return fmt.Errorf("cannot connect to %s: %v", s.host, err)
	}
	s.control, s.reader = connection, bufio.NewReader(connection)
	if _, err := s.expect(2); err != nil {
		return err
	}
	if _, err := s.command(fmt.Sprintf("USER %s", s.user), 2, 3); err != nil {
		return err
	}
	// A 230 to USER means no password was wanted, which anonymous servers do.
	if _, err := s.command(fmt.Sprintf("PASS %s", s.pass), 2); err != nil {
		return err
	}
	_, err = s.command("TYPE I", 2)
	return err
}

func (s *ftpSession) close() {
	if s.control != nil {
		fmt.Fprintf(s.control, "QUIT\r\n")
		s.control.Close()
	}
}

// command sends one line and reads the reply, checking its leading digit.
func (s *ftpSession) command(line string, accept ...int) (string, error) {
	if s.verbose {
		fmt.Fprintf(s.log, "> %s\n", line)
	}
	if _, err := fmt.Fprintf(s.control, "%s\r\n", line); err != nil {
		return "", err
	}
	return s.expect(accept...)
}

// expect reads a reply, skipping the continuation lines of a multi-line one.
//
// A multi-line reply is `250-first`, more lines, then `250 last`: the same code
// with a space rather than a hyphen. Reading only the first line leaves the rest
// in the buffer to be mistaken for the *next* reply, which is the classic way an
// FTP client desynchronises.
func (s *ftpSession) expect(accept ...int) (string, error) {
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("the server closed the connection")
		}
		line = strings.TrimRight(line, "\r\n")
		if s.verbose {
			fmt.Fprintf(s.log, "< %s\n", line)
		}
		if len(line) < 4 || line[3] == '-' {
			continue
		}
		code, err := strconv.Atoi(line[:3])
		if err != nil {
			continue
		}
		for _, wanted := range accept {
			if code/100 == wanted {
				return line, nil
			}
		}
		return "", fmt.Errorf("server said: %s", line)
	}
}

// openData asks for passive mode and connects to the address it answers with.
func (s *ftpSession) openData(ctx context.Context) (net.Conn, error) {
	reply, err := s.command("PASV", 2)
	if err != nil {
		return nil, err
	}
	address, err := parsePassiveReply(reply)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: 30 * time.Second}
	return dialer.DialContext(ctx, "tcp", address)
}

// parsePassiveReply reads `227 Entering Passive Mode (h1,h2,h3,h4,p1,p2)`.
//
// The port is two bytes, high first, which is the part everyone gets wrong: it is
// p1*256+p2 and not a decimal number spelled across two fields.
func parsePassiveReply(reply string) (string, error) {
	open := strings.LastIndex(reply, "(")
	closing := strings.LastIndex(reply, ")")
	if open < 0 || closing < open {
		return "", fmt.Errorf("cannot read the passive-mode reply: %s", reply)
	}
	fields := strings.Split(reply[open+1:closing], ",")
	if len(fields) != 6 {
		return "", fmt.Errorf("cannot read the passive-mode reply: %s", reply)
	}
	numbers := make([]int, 6)
	for index, field := range fields {
		value, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || value < 0 || value > 255 {
			return "", fmt.Errorf("cannot read the passive-mode reply: %s", reply)
		}
		numbers[index] = value
	}
	host := fmt.Sprintf("%d.%d.%d.%d", numbers[0], numbers[1], numbers[2], numbers[3])
	return net.JoinHostPort(host, strconv.Itoa(numbers[4]<<8|numbers[5])), nil
}

// retrieve downloads remote into local. The local name is checked the way an
// archive entry is, because it may have come from the server's own listing.
func (s *ftpSession) retrieve(ctx context.Context, remote, local string) error {
	data, err := s.openData(ctx)
	if err != nil {
		return err
	}
	defer data.Close()
	if _, err := s.command(fmt.Sprintf("RETR %s", remote), 1); err != nil {
		return err
	}
	safe, err := safeArchivePath(local)
	if err != nil {
		return err
	}
	native, err := resolveHostPath(ProcessViewFromContext(ctx), safe)
	if err != nil {
		return operandFailure(safe, err)
	}
	file, err := os.Create(native)
	if err != nil {
		return operandFailure(safe, err)
	}
	_, copyErr := io.Copy(file, data)
	closeErr := file.Close()
	if copyErr != nil {
		os.Remove(native)
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	data.Close()
	_, err = s.expect(2)
	return err
}

func (s *ftpSession) store(ctx context.Context, remote, local string) error {
	native, err := resolveHostPath(ProcessViewFromContext(ctx), local)
	if err != nil {
		return operandFailure(local, err)
	}
	file, err := os.Open(native)
	if err != nil {
		return operandFailure(local, err)
	}
	defer file.Close()
	data, err := s.openData(ctx)
	if err != nil {
		return err
	}
	if _, err := s.command(fmt.Sprintf("STOR %s", remote), 1); err != nil {
		data.Close()
		return err
	}
	_, copyErr := io.Copy(data, file)
	// Closed before waiting: the server's completion reply does not arrive until
	// it has seen the end of the data connection.
	data.Close()
	if copyErr != nil {
		return copyErr
	}
	_, err = s.expect(2)
	return err
}
