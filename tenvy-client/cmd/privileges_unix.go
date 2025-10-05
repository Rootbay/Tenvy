//go:build !windows

package main

import "os"

func currentUserIsElevated() bool {
	return os.Geteuid() == 0
}
