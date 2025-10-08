//go:build windows

package platform

import "golang.org/x/sys/windows"

func ProcessExists(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		switch err {
		case windows.ERROR_ACCESS_DENIED:
			return true, nil
		case windows.ERROR_INVALID_PARAMETER:
			return false, nil
		default:
			return false, err
		}
	}
	windows.CloseHandle(handle)
	return true, nil
}
