package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

type temporaryError interface {
	error
	Temporary() bool
}

type registrationError struct {
	err       error
	temporary bool
}

func (e *registrationError) Error() string {
	if e == nil || e.err == nil {
		return "registration error"
	}
	return e.err.Error()
}

func (e *registrationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *registrationError) Temporary() bool {
	if e == nil {
		return false
	}
	return e.temporary
}

func registerAgentWithRetry(ctx context.Context, logger *log.Logger, client *http.Client, serverURL, token string, metadata protocol.AgentMetadata, maxBackoff time.Duration) (*protocol.AgentRegistrationResponse, error) {
	if maxBackoff <= 0 {
		maxBackoff = defaultBackoff
	}

	backoff := time.Second
	if backoff <= 0 {
		backoff = time.Second
	}

	attempt := 1
	for {
		registration, err := registerAgent(ctx, client, serverURL, token, metadata)
		if err == nil {
			if attempt > 1 {
				logger.Printf("registration succeeded after %d attempts", attempt)
			}
			return registration, nil
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		if tempErr, ok := err.(temporaryError); ok && !tempErr.Temporary() {
			logger.Printf("registration aborted after %d attempts: %v", attempt, err)
			return nil, err
		}

		logger.Printf("registration attempt %d failed: %v", attempt, err)

		wait := jitterDuration(backoff)
		if wait > maxBackoff {
			wait = maxBackoff
		}

		logger.Printf("retrying registration in %s", wait)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}

		attempt++
		if backoff < maxBackoff {
			backoff = minDuration(backoff*2, maxBackoff)
		}
	}
}

func registerAgent(ctx context.Context, client *http.Client, serverURL, token string, metadata protocol.AgentMetadata) (*protocol.AgentRegistrationResponse, error) {
	request := protocol.AgentRegistrationRequest{Metadata: metadata}
	if strings.TrimSpace(token) != "" {
		request.Token = token
	}

	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/api/agents/register", serverURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("tenvy-client/%s", metadata.Version))

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		temporary := false
		var netErr net.Error
		if errors.As(err, &netErr) {
			temporary = netErr.Timeout() || netErr.Temporary()
		}
		if !temporary {
			var opErr *net.OpError
			if errors.As(err, &opErr) {
				temporary = true
			}
		}
		if !temporary {
			var urlErr *url.Error
			if errors.As(err, &urlErr) {
				if urlErr.Timeout() {
					temporary = true
				}
				if !temporary {
					if _, ok := urlErr.Err.(*net.OpError); ok {
						temporary = true
					}
				}
			}
		}

		return nil, &registrationError{err: err, temporary: temporary}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return nil, &registrationError{
			err:       fmt.Errorf("registration failed: %s", message),
			temporary: isTemporaryStatus(resp.StatusCode),
		}
	}

	var payload protocol.AgentRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, &registrationError{err: err, temporary: true}
	}

	if strings.TrimSpace(payload.AgentID) == "" {
		return nil, &registrationError{err: errors.New("missing agent identifier in response"), temporary: true}
	}
	if strings.TrimSpace(payload.AgentKey) == "" {
		return nil, &registrationError{err: errors.New("missing agent key in response"), temporary: true}
	}

	return &payload, nil
}

func isTemporaryStatus(status int) bool {
	switch {
	case status >= 500:
		return true
	case status == http.StatusTooManyRequests:
		return true
	default:
		return false
	}
}

func jitterDuration(base time.Duration) time.Duration {
	if base <= 0 {
		return time.Second
	}

	const (
		minFactor = 0.8
		maxFactor = 1.3
	)

	factor := minFactor + rand.Float64()*(maxFactor-minFactor)
	wait := time.Duration(float64(base) * factor)
	if wait <= 0 {
		return base
	}

	return wait
}
