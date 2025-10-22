package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

const (
	methodConfigure     = "configure"
	methodStartSession  = "startSession"
	methodStopSession   = "stopSession"
	methodUpdateSession = "updateSession"
	methodHandleInput   = "handleInput"
	methodDeliverFrame  = "deliverFrame"
	methodShutdown      = "shutdown"
)

type ipcRequest struct {
	ID     uint64          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type ipcResponse struct {
	ID     uint64                 `json:"id"`
	Result map[string]interface{} `json:"result,omitempty"`
	Error  *ipcError              `json:"error,omitempty"`
}

type ipcError struct {
	Message string `json:"message"`
}

type logEntry struct {
	Method    string          `json:"method"`
	Timestamp string          `json:"timestamp"`
	Params    json.RawMessage `json:"params,omitempty"`
}

func main() {
	logPath := os.Getenv("FAKE_REMOTE_DESKTOP_PLUGIN_LOG")
	var logEncoder *json.Encoder
	if logPath != "" {
		file, err := os.Create(logPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fake plugin: open log: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		logEncoder = json.NewEncoder(file)
	}

	decoder := json.NewDecoder(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	encoder := json.NewEncoder(writer)

	for {
		var req ipcRequest
		if err := decoder.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			fmt.Fprintf(os.Stderr, "fake plugin: decode request: %v\n", err)
			return
		}

		if logEncoder != nil {
			entry := logEntry{
				Method:    req.Method,
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
				Params:    append(json.RawMessage(nil), req.Params...),
			}
			_ = logEncoder.Encode(entry)
		}

		resp := ipcResponse{ID: req.ID, Result: map[string]interface{}{"status": "ok"}}
		if err := encoder.Encode(resp); err != nil {
			fmt.Fprintf(os.Stderr, "fake plugin: encode response: %v\n", err)
			return
		}
		if err := writer.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "fake plugin: flush response: %v\n", err)
			return
		}

		if req.Method == methodShutdown {
			return
		}
	}
}
