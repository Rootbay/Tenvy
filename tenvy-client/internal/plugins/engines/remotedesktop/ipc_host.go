package remotedesktopengine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// HTTPClientFactory constructs HTTP clients for the hosted engine based on the
// request timeout communicated by the agent. The factory enables the plugin to
// remain decoupled from the agent's HTTP stack while still honoring the
// controller's timing expectations.
type HTTPClientFactory func(timeout time.Duration) *http.Client

// ServeEngineIPC hosts an Engine over a JSON message channel. Requests and
// responses are newline delimited JSON objects written to the supplied reader
// and writer, respectively. The server exits when the context is cancelled, the
// underlying stream is closed, or a shutdown request is processed.
func ServeEngineIPC(ctx context.Context, engine Engine, reader io.Reader, writer io.Writer, logger Logger, clients HTTPClientFactory) error {
	if engine == nil {
		return errors.New("remote desktop engine not provided")
	}
	if reader == nil || writer == nil {
		return errors.New("ipc transport not configured")
	}
	if clients == nil {
		clients = func(timeout time.Duration) *http.Client {
			client := &http.Client{}
			if timeout > 0 {
				client.Timeout = timeout
			}
			return client
		}
	}

	dec := json.NewDecoder(reader)
	bufWriter := bufio.NewWriter(writer)
	enc := json.NewEncoder(bufWriter)

	type pendingResponse struct {
		id  uint64
		err error
	}

	var mu sync.Mutex
	handle := func(req ipcRequest) ipcResponse {
		mu.Lock()
		defer mu.Unlock()

		respond := ipcResponse{ID: req.ID}

		switch req.Method {
		case methodConfigure:
			var payload configEnvelope
			if err := json.Unmarshal(req.Params, &payload); err != nil {
				respond.Error = &ipcError{Message: fmt.Sprintf("decode configure payload: %v", err)}
				return respond
			}
			cfg := payload.toConfig(logger)
			cfg.Client = clients(cfg.RequestTimeout)
			if err := engine.Configure(cfg); err != nil {
				respond.Error = &ipcError{Message: err.Error()}
			}
		case methodStartSession:
			var payload RemoteDesktopCommandPayload
			if err := json.Unmarshal(req.Params, &payload); err != nil {
				respond.Error = &ipcError{Message: fmt.Sprintf("decode start payload: %v", err)}
				return respond
			}
			if err := engine.StartSession(ctx, payload); err != nil {
				respond.Error = &ipcError{Message: err.Error()}
			}
		case methodStopSession:
			var payload stopSessionRequest
			if err := json.Unmarshal(req.Params, &payload); err != nil {
				respond.Error = &ipcError{Message: fmt.Sprintf("decode stop payload: %v", err)}
				return respond
			}
			if err := engine.StopSession(payload.SessionID); err != nil {
				respond.Error = &ipcError{Message: err.Error()}
			}
		case methodUpdateSession:
			var payload RemoteDesktopCommandPayload
			if err := json.Unmarshal(req.Params, &payload); err != nil {
				respond.Error = &ipcError{Message: fmt.Sprintf("decode update payload: %v", err)}
				return respond
			}
			if err := engine.UpdateSession(payload); err != nil {
				respond.Error = &ipcError{Message: err.Error()}
			}
		case methodHandleInput:
			var payload RemoteDesktopCommandPayload
			if err := json.Unmarshal(req.Params, &payload); err != nil {
				respond.Error = &ipcError{Message: fmt.Sprintf("decode input payload: %v", err)}
				return respond
			}
			if err := engine.HandleInput(ctx, payload); err != nil {
				respond.Error = &ipcError{Message: err.Error()}
			}
		case methodDeliverFrame:
			var payload RemoteDesktopFramePacket
			if err := json.Unmarshal(req.Params, &payload); err != nil {
				respond.Error = &ipcError{Message: fmt.Sprintf("decode frame payload: %v", err)}
				return respond
			}
			if err := engine.DeliverFrame(ctx, payload); err != nil {
				respond.Error = &ipcError{Message: err.Error()}
			}
		case methodShutdown:
			engine.Shutdown()
			respond.Result = json.RawMessage(`{"status":"ok"}`)
			return respond
		default:
			respond.Error = &ipcError{Message: fmt.Sprintf("unknown method: %s", req.Method)}
		}

		if respond.Error == nil && respond.Result == nil {
			respond.Result = json.RawMessage(`{"status":"ok"}`)
		}
		return respond
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		var req ipcRequest
		if err := dec.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("decode ipc request: %w", err)
		}

		resp := handle(req)
		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("encode ipc response: %w", err)
		}
		if err := bufWriter.Flush(); err != nil {
			return fmt.Errorf("flush ipc response: %w", err)
		}

		if req.Method == methodShutdown {
			return nil
		}
	}
}
