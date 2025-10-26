package providers

import (
	"context"
	"net"
	"strings"
)

// DBIP returns a resolver that simulates the DB-IP provider.
func DBIP() Resolver {
	return ResolverFunc(func(ctx context.Context, ip net.IP, cfg Config) (Result, error) {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if strings.TrimSpace(cfg.APIKey) == "" {
			return Result{}, ErrMissingAPIKey
		}
		return syntheticLookup(ip, dbipDataset), nil
	})
}

var dbipDataset = []Result{
	{
		City:        "Dublin",
		Region:      "Leinster",
		Country:     "Ireland",
		CountryCode: "IE",
		Latitude:    53.3498,
		Longitude:   -6.2603,
		ISP:         "DB-IP Transit",
		ASN:         "AS57001",
		Timezone:    &Timezone{ID: "Europe/Dublin", Offset: "+00:00", Abbreviation: "GMT"},
	},
	{
		City:        "Madrid",
		Region:      "Community of Madrid",
		Country:     "Spain",
		CountryCode: "ES",
		Latitude:    40.4168,
		Longitude:   -3.7038,
		ISP:         "DB-IP Iberia",
		ASN:         "AS57002",
		Timezone:    &Timezone{ID: "Europe/Madrid", Offset: "+01:00", Abbreviation: "CET"},
	},
	{
		City:        "São Paulo",
		Region:      "São Paulo",
		Country:     "Brazil",
		CountryCode: "BR",
		Latitude:    -23.5505,
		Longitude:   -46.6333,
		ISP:         "DB-IP LATAM",
		ASN:         "AS57003",
		Timezone:    &Timezone{ID: "America/Sao_Paulo", Offset: "-03:00", Abbreviation: "BRT"},
	},
	{
		City:        "Cape Town",
		Region:      "Western Cape",
		Country:     "South Africa",
		CountryCode: "ZA",
		Latitude:    -33.9249,
		Longitude:   18.4241,
		ISP:         "DB-IP Africa",
		ASN:         "AS57004",
		Timezone:    &Timezone{ID: "Africa/Johannesburg", Offset: "+02:00", Abbreviation: "SAST"},
	},
}
