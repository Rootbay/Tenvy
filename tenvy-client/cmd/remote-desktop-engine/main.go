package main

import (
	"flag"
	"fmt"
	"os"

	engine "github.com/rootbay/tenvy-client/internal/plugins/engines/remotedesktop"
)

func main() {
	version := flag.Bool("version", false, "print build metadata")
	flag.Parse()

	if *version {
		fmt.Println("remote-desktop-engine plugin")
		return
	}

	// Instantiate an engine instance to ensure all capture and encoding
	// dependencies are linked into the standalone binary. Runtime
	// configuration is provided by the host process when executed.
	_ = engine.NewRemoteDesktopStreamer(engine.Config{})

	fmt.Fprintln(os.Stderr, "remote-desktop-engine plugin build artifact")
}
