package keylogger

import "errors"

var ErrProviderUnavailable = errors.New("keylogger provider not supported on this platform")
