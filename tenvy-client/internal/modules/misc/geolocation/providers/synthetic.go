package providers

import "net"

func syntheticLookup(ip net.IP, dataset []Result) Result {
	if len(dataset) == 0 {
		return Result{}
	}

	sum := 0
	for _, b := range ip {
		sum += int(b)
	}

	index := sum % len(dataset)
	return dataset[index]
}
