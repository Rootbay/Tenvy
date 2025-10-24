package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rootbay/tenvy-client/internal/agent"
)

var buildVersion = "dev"

func main() {
	logger := log.New(os.Stdout, "[tenvy-client] ", log.LstdFlags|log.Lmsgprefix)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	opts, err := loadRuntimeOptions(logger)
	if err != nil {
		logger.Fatalf("configuration error: %v", err)
	}

	if err := agent.Run(ctx, opts); err != nil {
		logger.Fatalf("agent terminated: %v", err)
	}
}
