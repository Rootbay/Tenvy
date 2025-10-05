//go:build !windows

package main

import "syscall"

func processExists(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	err := syscall.Kill(pid, 0)
	switch err {
	case nil:
		return true, nil
	case syscall.ESRCH:
		return false, nil
	case syscall.EPERM:
		return true, nil
	default:
		return false, err
	}
}
