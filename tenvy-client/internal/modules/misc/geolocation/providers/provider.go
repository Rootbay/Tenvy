package providers

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"
)

// Result represents the geographic data returned by a provider.
type Result struct {
	City        string
	Region      string
	Country     string
	CountryCode string
	Latitude    float64
	Longitude   float64
	ISP         string
	ASN         string
	Timezone    *Timezone
}

// Timezone describes timezone metadata returned by a provider.
type Timezone struct {
	ID           string
	Offset       string
	Abbreviation string
}

// Config contains provider specific runtime options.
type Config struct {
	APIKey  string
	Timeout time.Duration
}

// Normalize returns a sanitized copy of the provider configuration.
func (c Config) Normalize() Config {
	copy := Config{APIKey: strings.TrimSpace(c.APIKey), Timeout: c.Timeout}
	if copy.Timeout <= 0 {
		copy.Timeout = 5 * time.Second
	}
	return copy
}

// Resolver resolves an IP into geolocation data.
type Resolver interface {
	Lookup(ctx context.Context, ip net.IP, cfg Config) (Result, error)
}

// ResolverFunc adapts a function into a Resolver implementation.
type ResolverFunc func(context.Context, net.IP, Config) (Result, error)

// Lookup implements Resolver.
func (f ResolverFunc) Lookup(ctx context.Context, ip net.IP, cfg Config) (Result, error) {
	if f == nil {
		return Result{}, errors.New("resolver not defined")
	}
	return f(ctx, ip, cfg)
}

// ErrMissingAPIKey indicates that the provider requires an API key.
var ErrMissingAPIKey = errors.New("provider api key required")
