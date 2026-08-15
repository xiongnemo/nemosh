package main

import "strings"

// hostNamesInLine pulls the names out of one line of either source.
//
// Two grammars, one function, because the callers are the same loop and the
// difference is three lines. Split out from hostindex.go for the file-size
// ceiling, not because it is a separate idea.
func hostNamesInLine(line string, sshConfig bool) []string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}
	if sshConfig {
		return sshConfigHostNames(line)
	}
	return hostsFileNames(line)
}

// sshConfigHostNames reads `Host` and `HostName`.
//
// Both, because they answer different halves of the same question: `Host` is
// the alias a person typed into the file to have something short to type at a
// prompt, and `HostName` is what it resolves to, which is also frequently what
// gets typed. fish reads only `Host`; bash-completion's `_known_hosts_real`
// reads both, and it is right -- an alias whose real name is never offered makes
// the file look half-read.
//
// The keyword is case-insensitive and may be separated by `=`, both of which
// ssh_config permits and neither of which is exotic in a hand-written file.
func sshConfigHostNames(line string) []string {
	keyword, rest, found := strings.Cut(strings.ReplaceAll(line, "=", " "), " ")
	if !found {
		return nil
	}
	switch strings.ToLower(keyword) {
	case "host", "hostname":
	default:
		return nil
	}
	var names []string
	for _, field := range strings.Fields(rest) {
		if !isOfferableHostName(field) {
			continue
		}
		names = append(names, field)
	}
	return names
}

// isOfferableHostName rejects what is a rule rather than a machine.
//
// `Host *` and `Host prod-*` are patterns: they configure a set, and completing
// one would put a name on the line that resolves to nothing. `!name` is a
// negation inside such a pattern. `%h` and friends are ssh's own substitution
// tokens, which appear in `HostName %h.internal`.
func isOfferableHostName(field string) bool {
	if field == "" || strings.HasPrefix(field, "!") {
		return false
	}
	return !strings.ContainsAny(field, "*?%")
}

// hostsFileNames reads `ADDRESS name [alias...]`.
//
// Entries mapped to 0.0.0.0 are skipped. That address is the convention for
// "this name goes nowhere", which is what an ad-blocking hosts file is made of,
// and a name someone installed in order *not* to reach is not a name to offer as
// somewhere to connect. 127.0.0.1 is left alone: that is a real local service as
// often as it is a block.
func hostsFileNames(line string) []string {
	if comment := strings.IndexByte(line, '#'); comment >= 0 {
		line = line[:comment]
	}
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] == "0.0.0.0" {
		return nil
	}
	var names []string
	for _, field := range fields[1:] {
		if !isOfferableHostName(field) {
			continue
		}
		names = append(names, field)
	}
	return names
}
