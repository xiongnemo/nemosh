package applets

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/xiongnemo/nemosh/internal/netio"
)

// nc -z, netcat-openbsd's, which is Debian's nc and busybox's has not got: whether each port
// takes a connection, and nothing sent. `nc -zv host 22` is how a port is checked. Each PORT is a
// number or a range, `20-25`, tried in turn; with -v each is said of on stderr in openbsd's words,
// and the status is 0 when any took a connection, as openbsd's is.

func scanNetcat(ctx context.Context, host string, ports []string, timeout time.Duration, verbose bool, stderr io.Writer) error {
	connected := false
	for _, spec := range ports {
		first, last, err := netcatPortRange(spec)
		if err != nil {
			return err
		}
		for port := first; port <= last; port++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			text := strconv.Itoa(port)
			connection, err := netio.Dial(ctx, host, text, timeout)
			if err != nil {
				if verbose {
					fmt.Fprintf(stderr, "nc: connect to %s port %s (tcp) failed: %v\n", host, text, err)
				}
				continue
			}
			connection.Close()
			connected = true
			if verbose {
				// openbsd names the port's service where it knows one; this knows none, and
				// says `*` as openbsd does for a port it has no name for.
				fmt.Fprintf(stderr, "Connection to %s %s port [tcp/*] succeeded!\n", host, text)
			}
		}
	}
	if !connected {
		return ExitStatus(1)
	}
	return nil
}

// netcatPortRange is a PORT or a range of them, LOW-HIGH, each from 1 to 65535.
func netcatPortRange(spec string) (int, int, error) {
	low, high, ranged := strings.Cut(spec, "-")
	if !ranged {
		high = low
	}
	first, firstErr := strconv.Atoi(low)
	last, lastErr := strconv.Atoi(high)
	if firstErr != nil || lastErr != nil || first < 1 || last > 65535 || first > last {
		return 0, 0, fmt.Errorf("port range not valid: %s", spec)
	}
	return first, last, nil
}
