package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

func (a *Agent) run(ctx context.Context) {
	pollInterval := a.pollInterval()
	backoff := pollInterval

	for {
		if err := sleepContext(ctx, a.withJitter(pollInterval)); err != nil {
			return
		}

		if err := a.sync(ctx, statusOnline); err != nil {
			if errors.Is(err, protocol.ErrUnauthorized) {
				a.logger.Printf("sync unauthorized: %v", err)
				if err := a.reRegister(ctx); err != nil {
					if ctx.Err() != nil {
						return
					}
					a.logger.Printf("re-registration failed: %v", err)
				} else {
					pollInterval = a.pollInterval()
					backoff = pollInterval
					continue
				}
			} else {
				a.logger.Printf("sync error: %v", err)
			}
			backoff = minDuration(backoff*2, a.maxBackoff())
			if err := sleepContext(ctx, backoff); err != nil {
				return
			}
			continue
		}

		pollInterval = a.pollInterval()
		backoff = pollInterval
	}
}

func (a *Agent) sync(ctx context.Context, status string) error {
	results := a.consumeResults()
	payload, err := a.performSync(ctx, status, results)
	if err != nil {
		if len(results) > 0 && !errors.Is(err, protocol.ErrUnauthorized) {
			a.enqueueResults(results)
		}
		return err
	}

	a.config = payload.Config
	a.processCommands(ctx, payload.Commands)

	if a.notes != nil {
		if err := a.notes.SyncShared(ctx, a.client, a.baseURL, a.id, a.key, a.userAgent()); err != nil {
			if errors.Is(err, protocol.ErrUnauthorized) {
				return err
			}
			a.logger.Printf("notes sync failed: %v", err)
		}
	}

	return nil
}

func (a *Agent) performSync(ctx context.Context, status string, results []protocol.CommandResult) (*protocol.AgentSyncResponse, error) {
	request := protocol.AgentSyncRequest{
		Status:    status,
		Timestamp: timestampNow(),
		Metrics:   a.collectMetrics(),
	}
	if len(results) > 0 {
		request.Results = results
	}

	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/api/agents/%s/sync", a.baseURL, url.PathEscape(a.id))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", a.userAgent())
	if strings.TrimSpace(a.key) != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.key))
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		message := strings.TrimSpace(string(body))
		if resp.StatusCode == http.StatusUnauthorized {
			if message == "" {
				return nil, protocol.ErrUnauthorized
			}
			return nil, fmt.Errorf("%w: %s", protocol.ErrUnauthorized, message)
		}
		if message == "" {
			message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("sync failed: %s", message)
	}

	var payload protocol.AgentSyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return &payload, nil
}

func (a *Agent) reRegister(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	metadata := CollectMetadata(a.buildVersion)
	registration, err := registerAgentWithRetry(ctx, a.logger, a.client, a.baseURL, a.sharedSecret, metadata, a.maxBackoff())
	if err != nil {
		return err
	}

	a.metadata = metadata
	a.id = registration.AgentID
	a.key = registration.AgentKey
	a.config = registration.Config
	a.startTime = time.Now()
	a.resultMu.Lock()
	a.pendingResults = a.pendingResults[:0]
	a.resultMu.Unlock()

	if a.modules != nil {
		if err := a.modules.Update(a.moduleRuntime()); err != nil {
			return fmt.Errorf("update modules: %w", err)
		}
	}

	a.logger.Printf("re-registered as %s", a.id)
	a.processCommands(ctx, registration.Commands)
	return nil
}

func (a *Agent) enqueueResult(result protocol.CommandResult) {
	a.resultMu.Lock()
	defer a.resultMu.Unlock()
	a.pendingResults = append(a.pendingResults, result)
	a.trimPendingResultsLocked()
}

func (a *Agent) enqueueResults(results []protocol.CommandResult) {
	if len(results) == 0 {
		return
	}

	a.resultMu.Lock()
	defer a.resultMu.Unlock()

	trimmed := limitResults(results, maxBufferedResults)
	if len(trimmed) == 0 {
		return
	}

	a.pendingResults = append(a.pendingResults, trimmed...)
	a.trimPendingResultsLocked()
}

func (a *Agent) trimPendingResultsLocked() {
	if len(a.pendingResults) == 0 {
		return
	}
	if maxBufferedResults <= 0 {
		a.pendingResults = a.pendingResults[:0]
		return
	}
	if len(a.pendingResults) <= maxBufferedResults {
		return
	}

	keep := a.pendingResults[len(a.pendingResults)-maxBufferedResults:]
	a.pendingResults = slices.Clone(keep)
}

func (a *Agent) consumeResults() []protocol.CommandResult {
	a.resultMu.Lock()
	defer a.resultMu.Unlock()
	if len(a.pendingResults) == 0 {
		return nil
	}
	results := slices.Clone(a.pendingResults)
	a.pendingResults = a.pendingResults[:0]
	return results
}

func (a *Agent) collectMetrics() *protocol.AgentMetrics {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return &protocol.AgentMetrics{
		MemoryBytes:   stats.Alloc,
		Goroutines:    runtime.NumGoroutine(),
		UptimeSeconds: uint64(time.Since(a.startTime).Seconds()),
	}
}

func (a *Agent) pollInterval() time.Duration {
	if a.config.PollIntervalMs > 0 {
		return time.Duration(a.config.PollIntervalMs) * time.Millisecond
	}
	if a.timing.PollInterval > 0 {
		return a.timing.PollInterval
	}
	return defaultPollInterval
}

func (a *Agent) maxBackoff() time.Duration {
	if a.config.MaxBackoffMs > 0 {
		return time.Duration(a.config.MaxBackoffMs) * time.Millisecond
	}
	if a.timing.MaxBackoff > 0 {
		return a.timing.MaxBackoff
	}
	return defaultBackoff
}

func (a *Agent) withJitter(base time.Duration) time.Duration {
	ratio := a.config.JitterRatio
	if ratio <= 0 {
		return base
	}
	jitter := (rand.Float64()*2 - 1) * ratio * float64(base)
	value := time.Duration(float64(base) + jitter)
	if value <= 0 {
		return base
	}
	return value
}

func (a *Agent) userAgent() string {
	return fmt.Sprintf("tenvy-client/%s", a.buildVersion)
}

func (a *Agent) shutdown(ctx context.Context) {
	if a.modules != nil {
		a.modules.Shutdown(ctx)
	}
	if err := a.sync(ctx, statusOffline); err != nil {
		a.logger.Printf("failed to send offline heartbeat: %v", err)
	}
}

func limitResults(results []protocol.CommandResult, limit int) []protocol.CommandResult {
	if limit <= 0 || len(results) == 0 {
		return nil
	}
	if len(results) <= limit {
		return results
	}
	return results[len(results)-limit:]
}
