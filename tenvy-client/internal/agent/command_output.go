package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

const commandOutputRequestTimeout = 10 * time.Second

type commandOutputMessage struct {
	data   []byte
	result *protocol.CommandResult
}

type commandOutputStreamer struct {
	agent     *Agent
	commandID string
	queue     chan commandOutputMessage
	done      chan struct{}
	sequence  int64
	closeOnce sync.Once
}

func newCommandOutputStreamer(agent *Agent, commandID string) *commandOutputStreamer {
	if agent == nil {
		return nil
	}

	trimmedCommandID := strings.TrimSpace(commandID)
	if trimmedCommandID == "" {
		return nil
	}

	if strings.TrimSpace(agent.baseURL) == "" {
		return nil
	}
	if strings.TrimSpace(agent.id) == "" {
		return nil
	}
	if strings.TrimSpace(agent.key) == "" {
		return nil
	}

	streamer := &commandOutputStreamer{
		agent:     agent,
		commandID: trimmedCommandID,
		queue:     make(chan commandOutputMessage, 8),
		done:      make(chan struct{}),
	}
	go streamer.run()
	return streamer
}

func (s *commandOutputStreamer) Write(p []byte) (int, error) {
	if s == nil {
		return len(p), nil
	}
	if len(p) == 0 {
		return 0, nil
	}

	data := make([]byte, len(p))
	copy(data, p)

	s.queue <- commandOutputMessage{data: data}
	return len(p), nil
}

func (s *commandOutputStreamer) Complete(result protocol.CommandResult) {
	if s == nil {
		return
	}

	copyResult := result
	s.queue <- commandOutputMessage{result: &copyResult}
	s.closeOnce.Do(func() {
		close(s.queue)
	})
	<-s.done
}

func (s *commandOutputStreamer) run() {
	defer close(s.done)

	for message := range s.queue {
		if len(message.data) > 0 {
			s.sequence++
			event := protocol.CommandOutputEvent{
				Type:      "chunk",
				CommandID: s.commandID,
				Sequence:  s.sequence,
				Data:      string(message.data),
				Timestamp: timestampNow(),
			}
			if err := s.agent.sendCommandOutputEvent(event); err != nil && s.agent.logger != nil {
				s.agent.logger.Printf("command output chunk delivery failed: %v", err)
			}
		}

		if message.result != nil {
			event := protocol.CommandOutputEvent{
				Type:      "end",
				CommandID: s.commandID,
				Timestamp: message.result.CompletedAt,
				Result:    message.result,
			}
			if event.Timestamp == "" {
				event.Timestamp = timestampNow()
			}
			if err := s.agent.sendCommandOutputEvent(event); err != nil && s.agent.logger != nil {
				s.agent.logger.Printf("command output completion delivery failed: %v", err)
			}
		}
	}
}

func (a *Agent) commandOutputURL(commandID string) (string, error) {
	if a == nil {
		return "", fmt.Errorf("agent not initialized")
	}

	base := strings.TrimSpace(a.baseURL)
	if base == "" {
		return "", fmt.Errorf("missing base url")
	}

	agentID := strings.TrimSpace(a.id)
	if agentID == "" {
		return "", fmt.Errorf("missing agent identifier")
	}

	trimmedCommandID := strings.TrimSpace(commandID)
	if trimmedCommandID == "" {
		return "", fmt.Errorf("missing command identifier")
	}

	joined, err := url.JoinPath(base, "api", "agents", url.PathEscape(agentID), "commands", url.PathEscape(trimmedCommandID), "output")
	if err != nil {
		return "", err
	}

	parsed, err := url.Parse(joined)
	if err != nil {
		return "", err
	}

	return parsed.String(), nil
}

func (a *Agent) sendCommandOutputEvent(event protocol.CommandOutputEvent) error {
	if a == nil {
		return fmt.Errorf("agent not initialized")
	}

	endpoint, err := a.commandOutputURL(event.CommandID)
	if err != nil {
		return err
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandOutputRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", a.userAgent())

	key := strings.TrimSpace(a.key)
	if key == "" {
		return fmt.Errorf("missing agent key")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))
	applyRequestDecorations(req, a.requestHeaders, a.requestCookies)

	client := a.client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		limited := io.LimitReader(resp.Body, 1024)
		body, _ := io.ReadAll(limited)
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return fmt.Errorf("command output request failed: %s", message)
	}

	return nil
}
