package agent

import "time"

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func timestampNow() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
