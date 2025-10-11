//go:build cgo

package audio

import (
	"runtime"

	"github.com/gen2brain/malgo"
)

func fallbackAudioBackendAttempts() [][]malgo.Backend {
	switch runtime.GOOS {
	case "windows":
		return [][]malgo.Backend{
			{malgo.BackendWasapi},
			{malgo.BackendDsound},
			{malgo.BackendWinmm},
		}
	case "darwin":
		return [][]malgo.Backend{
			{malgo.BackendCoreAudio},
		}
	case "android":
		return [][]malgo.Backend{
			{malgo.BackendAAudio},
			{malgo.BackendOpenSLES},
		}
	case "linux":
		fallthrough
	case "freebsd":
		fallthrough
	case "openbsd":
		fallthrough
	case "netbsd":
		return [][]malgo.Backend{
			{malgo.BackendPulseAudio},
			{malgo.BackendAlsa},
			{malgo.BackendJack},
		}
	default:
		return nil
	}
}
