package remote

import (
	"math"
	"time"
)

type remoteQualityProfile struct {
	width       int
	height      int
	tile        int
	interval    time.Duration
	bitrate     int
	clipQuality int
}

type qualityTemplate struct {
	tile     int
	interval time.Duration
	bitrate  int
	clip     int
}

func selectQualityProfile(quality RemoteDesktopQuality, monitor RemoteDesktopMonitorInfo) (remoteQualityProfile, []remoteQualityProfile, int) {
	ladder := buildQualityLadder(monitor)
	if len(ladder) == 0 {
		fallbackWidth := monitor.Width
		fallbackHeight := monitor.Height
		if fallbackWidth <= 0 {
			fallbackWidth = 1280
		}
		if fallbackHeight <= 0 {
			fallbackHeight = 720
		}
		tmpl := templateForWidth(fallbackWidth)
		profile := remoteQualityProfile{
			width:       alignEven(maxInt(320, fallbackWidth)),
			height:      alignEven(maxInt(240, fallbackHeight)),
			tile:        tmpl.tile,
			interval:    tmpl.interval,
			bitrate:     tmpl.bitrate,
			clipQuality: clampInt(tmpl.clip, minClipQuality, maxClipQuality),
		}
		if profile.clipQuality <= 0 {
			profile.clipQuality = clipQualityBaseline(quality)
		}
		return profile, []remoteQualityProfile{profile}, 0
	}

	index := defaultLadderIndex(quality, len(ladder))
	if index < 0 {
		index = 0
	}
	if index >= len(ladder) {
		index = len(ladder) - 1
	}

	profile := ladder[index]
	if profile.clipQuality <= 0 {
		profile.clipQuality = clipQualityBaseline(quality)
	}
	profile.clipQuality = clampInt(profile.clipQuality, minClipQuality, maxClipQuality)
	ladder[index] = profile
	return profile, ladder, index
}

func buildQualityLadder(monitor RemoteDesktopMonitorInfo) []remoteQualityProfile {
	width := monitor.Width
	height := monitor.Height
	if width <= 0 {
		width = 1280
	}
	if height <= 0 {
		height = 720
	}
	aspect := float64(height) / float64(width)
	if aspect <= 0 {
		aspect = 9.0 / 16.0
	}

	scales := []float64{1.0, 0.88, 0.76, 0.66, 0.56, 0.46, 0.38}
	used := make(map[int]bool)
	ladder := make([]remoteQualityProfile, 0, len(scales))
	for _, scale := range scales {
		candidateWidth := alignEven(int(math.Round(float64(width) * scale)))
		if candidateWidth <= 0 {
			continue
		}
		if candidateWidth > width {
			candidateWidth = width
		}
		if candidateWidth < 320 {
			continue
		}
		if used[candidateWidth] {
			continue
		}
		used[candidateWidth] = true

		candidateHeight := alignEven(int(math.Round(float64(candidateWidth) * aspect)))
		if candidateHeight <= 0 {
			candidateHeight = alignEven(int(math.Round(float64(height) * scale)))
		}
		if candidateHeight <= 0 {
			candidateHeight = height
		}
		tmpl := templateForWidth(candidateWidth)
		profile := remoteQualityProfile{
			width:       candidateWidth,
			height:      maxInt(240, candidateHeight),
			tile:        clampInt(tmpl.tile, 24, 128),
			interval:    tmpl.interval,
			bitrate:     tmpl.bitrate,
			clipQuality: clampInt(tmpl.clip, minClipQuality, maxClipQuality),
		}
		ladder = append(ladder, profile)
	}

	if len(ladder) == 0 {
		return nil
	}

	return ladder
}

func templateForWidth(width int) qualityTemplate {
	switch {
	case width >= 3200:
		return qualityTemplate{tile: 28, interval: 65 * time.Millisecond, bitrate: 11000, clip: 92}
	case width >= 2560:
		return qualityTemplate{tile: 28, interval: 72 * time.Millisecond, bitrate: 9000, clip: 90}
	case width >= 2048:
		return qualityTemplate{tile: 30, interval: 80 * time.Millisecond, bitrate: 7600, clip: 88}
	case width >= 1920:
		return qualityTemplate{tile: 32, interval: 88 * time.Millisecond, bitrate: 6400, clip: 86}
	case width >= 1700:
		return qualityTemplate{tile: 34, interval: 96 * time.Millisecond, bitrate: 5600, clip: 84}
	case width >= 1500:
		return qualityTemplate{tile: 38, interval: 104 * time.Millisecond, bitrate: 5000, clip: 82}
	case width >= 1366:
		return qualityTemplate{tile: 42, interval: 116 * time.Millisecond, bitrate: 4200, clip: 80}
	case width >= 1280:
		return qualityTemplate{tile: 44, interval: 124 * time.Millisecond, bitrate: 3600, clip: 78}
	case width >= 1100:
		return qualityTemplate{tile: 48, interval: 138 * time.Millisecond, bitrate: 3000, clip: 76}
	case width >= 960:
		return qualityTemplate{tile: 52, interval: 150 * time.Millisecond, bitrate: 2300, clip: 74}
	case width >= 820:
		return qualityTemplate{tile: 56, interval: 168 * time.Millisecond, bitrate: 1800, clip: 72}
	case width >= 700:
		return qualityTemplate{tile: 60, interval: 186 * time.Millisecond, bitrate: 1350, clip: 70}
	default:
		return qualityTemplate{tile: 64, interval: 210 * time.Millisecond, bitrate: 950, clip: 68}
	}
}

func defaultLadderIndex(quality RemoteDesktopQuality, length int) int {
	if length <= 0 {
		return 0
	}
	switch quality {
	case RemoteQualityHigh:
		return 0
	case RemoteQualityMedium:
		idx := length / 2
		if idx <= 0 && length > 1 {
			idx = 1
		}
		if idx >= length {
			idx = length - 1
		}
		return idx
	case RemoteQualityLow:
		idx := length - 1
		if idx < 0 {
			idx = 0
		}
		return idx
	default:
		return 0
	}
}

func alignEven(value int) int {
	if value%2 != 0 {
		value++
	}
	return value
}

func clipQualityBaseline(preset RemoteDesktopQuality) int {
	switch preset {
	case RemoteQualityHigh:
		return 88
	case RemoteQualityMedium:
		return 80
	case RemoteQualityLow:
		return 72
	default:
		return defaultClipQuality
	}
}
