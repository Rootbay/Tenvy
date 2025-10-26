package providers

import (
	"context"
	"net"
	"strings"
)

// MaxMind returns a resolver that simulates the MaxMind provider.
func MaxMind() Resolver {
	return ResolverFunc(func(ctx context.Context, ip net.IP, cfg Config) (Result, error) {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if strings.TrimSpace(cfg.APIKey) == "" {
			return Result{}, ErrMissingAPIKey
		}
		return syntheticLookup(ip, maxmindDataset), nil
	})
}

var maxmindDataset = []Result{
	{
		City:        "Seattle",
		Region:      "Washington",
		Country:     "United States",
		CountryCode: "US",
		Latitude:    47.6062,
		Longitude:   -122.3321,
		ISP:         "MaxMind Transit",
		ASN:         "AS56001",
		Timezone:    &Timezone{ID: "America/Los_Angeles", Offset: "-08:00", Abbreviation: "PST"},
	},
	{
		City:        "Stockholm",
		Region:      "Stockholm",
		Country:     "Sweden",
		CountryCode: "SE",
		Latitude:    59.3293,
		Longitude:   18.0686,
		ISP:         "MaxMind Nordic",
		ASN:         "AS56002",
		Timezone:    &Timezone{ID: "Europe/Stockholm", Offset: "+01:00", Abbreviation: "CET"},
	},
	{
		City:        "Sydney",
		Region:      "New South Wales",
		Country:     "Australia",
		CountryCode: "AU",
		Latitude:    -33.8688,
		Longitude:   151.2093,
		ISP:         "MaxMind Pacific",
		ASN:         "AS56003",
		Timezone:    &Timezone{ID: "Australia/Sydney", Offset: "+10:00", Abbreviation: "AEST"},
	},
	{
		City:        "Tokyo",
		Region:      "Tokyo",
		Country:     "Japan",
		CountryCode: "JP",
		Latitude:    35.6762,
		Longitude:   139.6503,
		ISP:         "MaxMind East",
		ASN:         "AS56004",
		Timezone:    &Timezone{ID: "Asia/Tokyo", Offset: "+09:00", Abbreviation: "JST"},
	},
}
