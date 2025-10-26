package providers

import (
	"context"
	"net"
	"strings"
)

// IPInfo returns a resolver that simulates the ipinfo provider.
func IPInfo() Resolver {
	return ResolverFunc(func(ctx context.Context, ip net.IP, cfg Config) (Result, error) {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if strings.TrimSpace(cfg.APIKey) == "" {
			return Result{}, ErrMissingAPIKey
		}
		return syntheticLookup(ip, ipinfoDataset), nil
	})
}

var ipinfoDataset = []Result{
	{
		City:        "Lisbon",
		Region:      "Lisboa",
		Country:     "Portugal",
		CountryCode: "PT",
		Latitude:    38.7223,
		Longitude:   -9.1393,
		ISP:         "IPInfo Communications",
		ASN:         "AS55001",
		Timezone:    &Timezone{ID: "Europe/Lisbon", Offset: "+01:00", Abbreviation: "WET"},
	},
	{
		City:        "Berlin",
		Region:      "Berlin",
		Country:     "Germany",
		CountryCode: "DE",
		Latitude:    52.52,
		Longitude:   13.405,
		ISP:         "IPInfo Fiber",
		ASN:         "AS55002",
		Timezone:    &Timezone{ID: "Europe/Berlin", Offset: "+01:00", Abbreviation: "CET"},
	},
	{
		City:        "Toronto",
		Region:      "Ontario",
		Country:     "Canada",
		CountryCode: "CA",
		Latitude:    43.6518,
		Longitude:   -79.3832,
		ISP:         "IPInfo North",
		ASN:         "AS55003",
		Timezone:    &Timezone{ID: "America/Toronto", Offset: "-05:00", Abbreviation: "EST"},
	},
	{
		City:        "Singapore",
		Region:      "Central",
		Country:     "Singapore",
		CountryCode: "SG",
		Latitude:    1.3521,
		Longitude:   103.8198,
		ISP:         "IPInfo Asia",
		ASN:         "AS55004",
		Timezone:    &Timezone{ID: "Asia/Singapore", Offset: "+08:00", Abbreviation: "SGT"},
	},
}
