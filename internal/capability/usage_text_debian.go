package capability

// The help of the commands taken from Debian's tools -- iproute2's ip and ss, iputils' ping,
// util-linux's column, psmisc's pstree, tree(1), Debian's hostname -- which busybox-w32 has not
// got, split from usage_text_more.go at the 250-line ceiling AGENTS.md sets and merged with the
// rest by usageText, so nothing that reads the table knows there is a split.
func appletUsageTextDebian() map[string]Usage {
	return map[string]Usage{
		"file": {Summary: "Say what kind of file each FILE is.", Operands: "FILE...",
			Options: map[string]string{"b": "leave the names out", "i": "say the MIME type and charset instead",
				"L": "say what a link leads to rather than that it is one", "N": "do not line the answers up after the names",
				"h": "say that a link is one, which is the default", "brief": "the same as -b", "mime": "the same as -i",
				"dereference": "the same as -L", "no-pad": "the same as -N", "mime-type": "say the MIME type alone"},
			Notes: []string{
				"file(1)'s, in the words of Debian's libmagic: text by its encoding and line terminators, a script by its #!, a Windows executable in full.",
				"An ELF, the archives, the compressions and the images are named with the first fields of their headers; what libmagic says beyond those is left out rather than guessed at. A FILE that cannot be opened is said so of, and the status stays 0.",
			}}, "column": {Summary: "Lay lines out in columns, or as a table.", Operands: "[FILE]...",
			Options: map[string]string{"t": "a table: each line a row, split at runs of spaces and tabs", "x": "fill across and then down, rather than down and then across",
				"L": "keep empty lines", "d": "no line of -N's names", "s": "split at each of these characters instead; an empty field stays one",
				"o": "join a table's columns with this rather than two spaces", "c": "this many columns wide, or unlimited",
				"R": "right-align these columns, by number or -N's name", "N": "name the columns, in a first line", "l": "split into this many fields at most",
				"table": "the same as -t", "fillrows": "the same as -x", "keep-empty-lines": "the same as -L", "table-noheadings": "the same as -d",
				"separator": "the same as -s", "input-separator": "the same as -s", "output-separator": "the same as -o", "output-width": "the same as -c",
				"table-right": "the same as -R", "table-columns": "the same as -N", "table-columns-limit": "the same as -l"},
			Notes: []string{
				"util-linux's column: without -t the columns are tab stops apart, as wide as -c says, or the terminal is, or COLUMNS, or 80.",
			}},
		"ping": {Summary: "Send ICMP echoes to a host and say how they came back.", Operands: "HOST",
			Options: map[string]string{"c": "stop after this many echoes; without it, until interrupted", "i": "seconds between echoes, 1 by default",
				"W": "seconds to wait for each reply, an interval's by default, a second at least", "w": "stop after this many seconds",
				"s": "data bytes in each echo, 56 by default", "t": "the TTL to send with", "q": "only the first line and the statistics",
				"n": "addresses, without looking up their names", "O": "say so of an echo nothing answered", "4": "IPv4", "6": "IPv6"},
			Notes: []string{
				"iputils' ping, by IcmpSendEcho2, which needs no administrator. It takes the place of Windows' PING.EXE, which still runs as ping.exe; a Windows habit, a second word or /t, is pointed to it.",
				"An IPv6 reply has no ttl=: Windows does not say what hop limit it came with. Status 0 when something answered, 1 when nothing did, 2 on an error.",
			}},
		"ss": {Summary: "List the TCP and UDP sockets, and who owns them.", Operands: "",
			Options: map[string]string{"t": "TCP", "u": "UDP", "l": "the listening sockets, which for UDP are all of them",
				"a": "every socket", "n": "ports as numbers rather than names from the services file", "p": "the process that owns each",
				"4": "IPv4 only", "6": "IPv6 only", "H": "no header", "s": "refused: the summary is what Linux's /proc keeps",
				"tcp": "the same as -t", "udp": "the same as -u", "listening": "the same as -l", "all": "the same as -a",
				"numeric": "the same as -n", "processes": "the same as -p", "ipv4": "the same as -4", "ipv6": "the same as -6",
				"no-header": "the same as -H", "summary": "the same as -s"},
			Notes: []string{
				"iproute2's ss, from Windows' socket tables, which say who owns each socket without an administrator: ss -tulpn.",
				"Recv-Q and Send-Q are -: Windows says them of no socket without an administrator's tracing. A filter expression is refused; the lines are there to grep.",
			}},
		"ip": {Summary: "Show the network adapters, their addresses, and the routes.", Operands: "[-4|-6] [-br] [-o] {address|link|route} [show] [[dev] NAME]",
			Options: map[string]string{"4": "IPv4 only", "6": "IPv6 only; with route, the IPv6 routes",
				"o": "each record on one line, its lines joined by \\", "brief": "one line an adapter, as -br is", "oneline": "the same as -o"},
			Notes: []string{
				"iproute2's ip, to read with: address, link and route, and their beginnings, ip a, ip r. A change -- add, del, set, flush -- is refused with the PowerShell command that makes it.",
				"An adapter is named as Windows names it, Ethernet or Wi-Fi or 以太网. Windows has no qdisc, group or queue length, and they are left out; the loopback's state is UP, as Windows says.",
				"The routes are the ones Linux keeps in its main table, and a metric is the route's and the adapter's together, as Windows chooses by.",
			}},
		"tree": {Summary: "Draw the directories under each PATH as a tree.", Operands: "[PATH]...",
			Options: map[string]string{"a": "list names beginning with a dot too", "d": "list directories only", "f": "print each name's whole path",
				"i": "draw no lines", "l": "go into links to directories", "F": "mark directories /, programs *, pipes | and sockets =",
				"L": "go no deeper than this many levels", "I": "leave out names matching this pattern; | separates several",
				"P": "list only the files matching this pattern; | separates several", "p": "show each permission string",
				"u": "show each owner", "g": "show each group", "s": "show each size in bytes", "h": "show each size in K, M and G",
				"D": "show each modification time", "r": "sort in reverse", "U": "leave the names in the order the directory gives them",
				"dirsfirst": "list directories before files", "noreport": "leave out the count of directories and files at the end",
				"charset": "ascii draws with |, |-- and `--, where UTF-8 is the default"},
			Notes: []string{
				"tree(1)'s, which takes the place of Windows' tree.com; that one still runs as tree.com, and /F and /A are pointed to it.",
				"Names are in byte order, and the lines are tree's UTF-8 ones, no-break spaces and all.",
			}},
		"hostname": {Summary: "Print this machine's name, or its addresses.", Operands: "",
			Options: map[string]string{"s": "the name cut at its first dot", "f": "the fully qualified name", "d": "the DNS domain",
				"i": "the addresses the name resolves to", "I": "every address of every adapter that is up, but the loopback's and IPv6's link-local ones",
				"A": "the names -I's addresses have", "a": "the aliases, of which Windows keeps none", "v": "accepted", "b": "accepted",
				"y": "refused: Windows has no NIS domain", "F": "refused: the name is set by Rename-Computer, as an administrator, with a restart",
				"short": "the same as -s", "fqdn": "the same as -f", "long": "the same as -f", "domain": "the same as -d",
				"ip-address": "the same as -i", "all-ip-addresses": "the same as -I", "all-fqdns": "the same as -A", "alias": "the same as -a",
				"verbose": "the same as -v", "boot": "the same as -b", "file": "the same as -F", "yp": "the same as -y", "nis": "the same as -y"},
			Notes: []string{
				"Debian's hostname, which takes the place of Windows' hostname.exe; that one prints the name alone, and runs as hostname.exe.",
				"-i, -I and -A end each address with a space, as Debian's do.",
			}},
		"pstree": {Summary: "Draw the processes as a tree of who started whom.", Operands: "[PID]",
			Options: map[string]string{"p": "show each pid, which stops identical branches being folded", "a": "show each command's arguments, a process a line",
				"c": "fold no identical branches", "A": "draw with ASCII", "U": "draw with UTF-8", "l": "cut no line at the width",
				"n": "order by pid rather than name", "T": "leave out the threads"},
			Notes: []string{
				"psmisc's pstree, parentage as Windows keeps it: a parent must be older than its child, for a pid is reused.",
				"Threads are {name}, folded even under -p, Windows' having no tid to show here; a USER is refused, ps -u lists what a user runs.",
			}}}
}
