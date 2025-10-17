package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rootbay/tenvy-client/internal/protocol"
	"nhooyr.io/websocket"
)

const (
	commandStreamDialTimeout    = 15 * time.Second
	remoteDesktopInputQueueSize = 32
)

type remoteDesktopInputTask struct {
	ctx    context.Context
	module *remoteDesktopModule
	burst  protocol.RemoteDesktopInputBurst
}

func (a *Agent) commandStreamURL() (string, error) {
	if a == nil {
		return "", errors.New("agent not initialized")
	}
	base := strings.TrimSpace(a.baseURL)
	if base == "" {
		return "", errors.New("missing base url")
	}
	if strings.TrimSpace(a.id) == "" {
		return "", errors.New("missing agent identifier")
	}

	joined, err := url.JoinPath(base, "api", "agents", url.PathEscape(a.id), "session")
	if err != nil {
		return "", err
	}

	parsed, err := url.Parse(joined)
	if err != nil {
		return "", err
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
		// already websocket
	default:
		return "", fmt.Errorf("unsupported command stream scheme: %s", parsed.Scheme)
	}

	return parsed.String(), nil
}

func (a *Agent) runCommandStream(ctx context.Context) {
	if a == nil {
		return
	}

	backoff := a.pollInterval()

	for {
		if ctx.Err() != nil {
			return
		}

		streamURL, err := a.commandStreamURL()
		if err != nil {
			if a.logger != nil {
				a.logger.Printf("command stream unavailable: %v", err)
			}
			return
		}

		headers := http.Header{}
		headers.Set("User-Agent", a.userAgent())
		if trimmed := strings.TrimSpace(a.key); trimmed != "" {
			headers.Set("Authorization", fmt.Sprintf("Bearer %s", trimmed))
		}

		dialCtx, cancel := context.WithTimeout(ctx, commandStreamDialTimeout)
		conn, resp, err := websocket.Dial(dialCtx, streamURL, &websocket.DialOptions{
			HTTPHeader:      headers,
			Subprotocols:    []string{protocol.CommandStreamSubprotocol},
			CompressionMode: websocket.CompressionDisabled,
		})
		cancel()
		if err != nil {
			if resp != nil {
				resp.Body.Close()
				if shouldReRegisterStatus(resp.StatusCode) {
					if a.logger != nil {
						a.logger.Printf("command stream rejected with status %d; scheduling re-registration", resp.StatusCode)
					}
					a.requestReconnect()
				}
			}
			if a.logger != nil && !errors.Is(err, context.Canceled) {
				a.logger.Printf("command stream connection failed: %v", err)
			}
			if err := sleepContext(ctx, backoff); err != nil {
				return
			}
			backoff = minDuration(backoff*2, a.maxBackoff())
			continue
		}

		if selected := conn.Subprotocol(); selected != protocol.CommandStreamSubprotocol {
			if a.logger != nil {
				if selected == "" {
					a.logger.Printf("command stream rejected connection without subprotocol")
				} else {
					a.logger.Printf("command stream negotiated unexpected subprotocol %q", selected)
				}
			}
			_ = conn.Close(websocket.StatusPolicyViolation, "unsupported protocol")
			if err := sleepContext(ctx, backoff); err != nil {
				return
			}
			backoff = minDuration(backoff*2, a.maxBackoff())
			continue
		}

		conn.SetReadLimit(protocol.CommandStreamMaxMessageSize)
		backoff = a.pollInterval()
		if a.logger != nil {
			a.logger.Printf("command stream connected")
		}

		a.consumeCommandStream(ctx, conn)

		if a.logger != nil {
			a.logger.Printf("command stream disconnected")
		}

		_ = conn.Close(websocket.StatusNormalClosure, "stream terminated")
	}
}

func (a *Agent) consumeCommandStream(ctx context.Context, conn *websocket.Conn) {
	if conn == nil {
		return
	}

	for {
		messageType, data, err := conn.Read(ctx)
		if err != nil {
			status := websocket.CloseStatus(err)
			if status != websocket.StatusNormalClosure && status != websocket.StatusGoingAway && !errors.Is(err, context.Canceled) {
				if a.logger != nil {
					a.logger.Printf("command stream read error: %v", err)
				}
			}
			return
		}

		if messageType != websocket.MessageText {
			continue
		}

		var envelope protocol.CommandEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			if a.logger != nil {
				a.logger.Printf("invalid command envelope: %v", err)
			}
			continue
		}

		switch strings.ToLower(strings.TrimSpace(envelope.Type)) {
		case "command":
			if envelope.Command == nil {
				continue
			}
			a.processCommands(ctx, []protocol.Command{*envelope.Command})
		case "remote-desktop-input":
			if envelope.Input == nil {
				continue
			}
			a.handleRemoteDesktopInput(ctx, *envelope.Input)
		default:
			continue
		}
	}
}

func (a *Agent) handleRemoteDesktopInput(ctx context.Context, burst protocol.RemoteDesktopInputBurst) {
	if a == nil || len(burst.Events) == 0 {
		return
	}
	if a.modules == nil {
		return
	}

	module := a.modules.remoteDesktopModule()
	if module == nil {
		if a.logger != nil {
			a.logger.Printf("remote desktop input ignored: module unavailable")
		}
		return
	}

	queue := a.ensureRemoteDesktopInputWorker()
	if queue == nil {
		return
	}

	task := remoteDesktopInputTask{ctx: ctx, module: module, burst: burst}

	select {
	case queue <- task:
	case <-ctx.Done():
		if a.logger != nil && !errors.Is(ctx.Err(), context.Canceled) {
			a.logger.Printf("remote desktop input dropped: %v", ctx.Err())
		}
	}
}

func (a *Agent) ensureRemoteDesktopInputWorker() chan remoteDesktopInputTask {
	if a == nil {
		return nil
	}

	a.remoteDesktopInputOnce.Do(func() {
		a.remoteDesktopInputQueue = make(chan remoteDesktopInputTask, remoteDesktopInputQueueSize)
		go a.remoteDesktopInputWorker()
	})

	return a.remoteDesktopInputQueue
}

func (a *Agent) remoteDesktopInputWorker() {
	for task := range a.remoteDesktopInputQueue {
		if task.module == nil {
			continue
		}
		if err := task.module.HandleInputBurst(task.ctx, task.burst); err != nil {
			if a.logger != nil && !errors.Is(err, context.Canceled) {
				a.logger.Printf("remote desktop input error: %v", err)
			}
		}
	}
}
