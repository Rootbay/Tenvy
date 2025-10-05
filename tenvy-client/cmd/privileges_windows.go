//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type tokenElevation struct {
	TokenIsElevated uint32
}

func currentUserIsElevated() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()

	var elevation tokenElevation
	var returned uint32
	err := windows.GetTokenInformation(
		token,
		windows.TokenElevation,
		(*byte)(unsafe.Pointer(&elevation)),
		uint32(unsafe.Sizeof(elevation)),
		&returned,
	)
	if err != nil {
		return false
	}

	return elevation.TokenIsElevated != 0
}
