//go:build !linux && !windows

package keylogger

import "context"

type stubProvider struct{}

func (stubProvider) Start(ctx context.Context, cfg StartConfig) (EventStream, error) {
	return nil, ErrProviderUnavailable
}

func defaultProviderFactory() func() Provider {
	return func() Provider {
		return stubProvider{}
	}
}
