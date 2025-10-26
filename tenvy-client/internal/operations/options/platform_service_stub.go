//go:build !windows

package options

import "context"

type noopPlatformService struct{}

func newPlatformService() PlatformService {
	return noopPlatformService{}
}

func (noopPlatformService) Execute(context.Context, string, map[string]any, State) (string, error) {
	return "", nil
}
