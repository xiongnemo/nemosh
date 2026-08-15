//go:build !windows

package main

// /etc/hosts is read here. It is short on a normal Unix machine and the names in
// it are ones somebody put there, which is the same reason ~/.ssh/config is
// worth reading. See the Windows half for why that platform is the exception.
func hostsFilePath() string { return "/etc/hosts" }
