//go:build !windows

package remotedesktop

func processRemoteInput(monitors []remoteMonitor, settings RemoteDesktopSettings, events []RemoteDesktopInputEvent) error {
	return nil
}
