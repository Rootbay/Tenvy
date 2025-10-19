//go:build cgo && !tenvy_no_audio
// +build cgo,!tenvy_no_audio

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/rootbay/tenvy-client/internal/modules/control/audio"
)

func main() {
	log.SetFlags(0)

	captureWindow := 2 * time.Second
	if len(os.Args) > 1 {
		if parsed, err := time.ParseDuration(os.Args[1]); err == nil && parsed > 0 {
			captureWindow = parsed
		}
	}

	timeout := captureWindow + 5*time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result, err := audio.RunCaptureDiagnostic(ctx, captureWindow)

	if result != nil && result.Inventory != nil {
		payload, marshalErr := json.MarshalIndent(result.Inventory, "", "  ")
		if marshalErr == nil {
			fmt.Printf("Discovered audio devices:\n%s\n", string(payload))
		} else {
			fmt.Println("Discovered audio devices (marshal error):", marshalErr)
		}
	} else {
		fmt.Println("No audio inventory data returned.")
	}

	if result != nil {
		fmt.Printf("Bytes captured: %d over %s\n", result.BytesCaptured, result.Duration)
	}

	if err != nil {
		log.Fatalf("audio diagnostic failed: %v", err)
	}

	if result != nil && result.BytesCaptured == 0 {
		log.Println("warning: diagnostic capture yielded zero bytes; verify microphone availability")
	}
}
