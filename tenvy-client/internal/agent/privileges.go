package agent

import (
	"errors"

	"github.com/rootbay/tenvy-client/internal/platform"
)

func enforcePrivilegeRequirement(required bool) error {
	if !required {
		return nil
	}
	if platform.CurrentUserIsElevated() {
		return nil
	}
	return errors.New("administrator privileges are required")
}
