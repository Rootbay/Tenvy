package keylogger

import (
	"context"
	"errors"
)

var ErrProviderUnavailable = errors.New("keylogger provider not supported on this platform")

type stubProvider struct{}

func (stubProvider) Start(ctx context.Context, cfg StartConfig) (EventStream, error) {
	return nil, ErrProviderUnavailable
}

func defaultProviderFactory() func() Provider {
	return func() Provider {
		return stubProvider{}
	}
}
