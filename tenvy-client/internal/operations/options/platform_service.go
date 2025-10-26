package options

import "context"

type PlatformService interface {
	Execute(ctx context.Context, operation string, metadata map[string]any, state State) (string, error)
}
