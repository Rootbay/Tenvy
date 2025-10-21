//go:build !windows && !darwin && !linux

package screen

func defaultPlatformCaptureCandidates() []backendCandidate { return nil }
