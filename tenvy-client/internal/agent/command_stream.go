package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rootbay/tenvy-client/internal/protocol"
	"nhooyr.io/websocket"
)

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

		conn, _, err := websocket.Dial(ctx, streamURL, &websocket.DialOptions{HTTPHeader: headers})
		if err != nil {
			if a.logger != nil && !errors.Is(err, context.Canceled) {
				a.logger.Printf("command stream connection failed: %v", err)
			}
			if err := sleepContext(ctx, backoff); err != nil {
				return
			}
			backoff = minDuration(backoff*2, a.maxBackoff())
			continue
		}

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

		if !strings.EqualFold(envelope.Type, "command") || envelope.Command == nil {
			continue
		}

		a.processCommands(ctx, []protocol.Command{*envelope.Command})
	}
}
