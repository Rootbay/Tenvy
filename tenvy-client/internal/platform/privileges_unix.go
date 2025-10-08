//go:build !windows

package platform

import "os"

func CurrentUserIsElevated() bool {
	return os.Geteuid() == 0
}
