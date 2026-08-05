//go:build !windows

package main

import "os"

func promptSymbol() string {
	if os.Geteuid() == 0 {
		return "#"
	}
	return "$"
}
