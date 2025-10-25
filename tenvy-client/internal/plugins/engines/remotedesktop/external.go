package remotedesktopengine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/rootbay/tenvy-client/internal/plugins"
	manifest "github.com/rootbay/tenvy-client/shared/pluginmanifest"
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
	ID     uint64          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ipcError       `json:"error,omitempty"`
}

type ipcError struct {
	Message string `json:"message"`
}

type stopSessionRequest struct {
	SessionID string `json:"sessionId"`
}

type configEnvelope struct {
	AgentID          string                         `json:"agentId"`
	BaseURL          string                         `json:"baseUrl"`
	AuthKey          string                         `json:"authKey"`
	PluginVersion    string                         `json:"pluginVersion,omitempty"`
	UserAgent        string                         `json:"userAgent,omitempty"`
	RequestTimeout   time.Duration                  `json:"requestTimeout,omitempty"`
	WebRTCICEServers []RemoteDesktopWebRTCICEServer `json:"webrtcIceServers,omitempty"`
	QUICInput        QUICInputConfig                `json:"quicInput"`
}

func newConfigEnvelope(cfg Config) configEnvelope {
	return configEnvelope{
		AgentID:          cfg.AgentID,
		BaseURL:          cfg.BaseURL,
		AuthKey:          cfg.AuthKey,
		PluginVersion:    cfg.PluginVersion,
		UserAgent:        cfg.UserAgent,
		RequestTimeout:   cfg.RequestTimeout,
		WebRTCICEServers: append([]RemoteDesktopWebRTCICEServer(nil), cfg.WebRTCICEServers...),
		QUICInput:        cfg.QUICInput,
	}
}

func (e configEnvelope) toConfig(logger Logger) Config {
	cfg := Config{
		AgentID:          e.AgentID,
		BaseURL:          e.BaseURL,
		AuthKey:          e.AuthKey,
		PluginVersion:    e.PluginVersion,
		UserAgent:        e.UserAgent,
		RequestTimeout:   e.RequestTimeout,
		WebRTCICEServers: append([]RemoteDesktopWebRTCICEServer(nil), e.WebRTCICEServers...),
		QUICInput:        e.QUICInput,
		Logger:           logger,
	}
	if cfg.RequestTimeout < 0 {
		cfg.RequestTimeout = 0
	}
	return cfg
}

type remoteDesktopIPCClient struct {
	process *engineProcess
}

// NewManagedRemoteDesktopEngine returns an Engine implementation backed by the
// external remote desktop plugin. The returned engine communicates with the
// plugin over the IPC channel exposed by the plugin binary. If the entry path is
// empty, the returned engine will emit initialization errors when invoked.
func NewManagedRemoteDesktopEngine(entryPath, version string, manager *plugins.Manager, logger Logger) Engine {
	process := newEngineProcess(entryPath, version, manager, logger)
	return &remoteDesktopIPCClient{process: process}
}

func (e *remoteDesktopIPCClient) Configure(cfg Config) error {
	if e == nil || e.process == nil {
		return errors.New("remote desktop subsystem not initialized")
	}
	envelope := newConfigEnvelope(cfg)
	return e.process.call(context.Background(), methodConfigure, envelope, nil)
}

func (e *remoteDesktopIPCClient) StartSession(ctx context.Context, payload RemoteDesktopCommandPayload) error {
	if e == nil || e.process == nil {
		return errors.New("remote desktop subsystem not initialized")
	}
	return e.process.call(ctx, methodStartSession, payload, nil)
}

func (e *remoteDesktopIPCClient) StopSession(sessionID string) error {
	if e == nil || e.process == nil {
		return errors.New("remote desktop subsystem not initialized")
	}
	request := stopSessionRequest{SessionID: strings.TrimSpace(sessionID)}
	return e.process.call(context.Background(), methodStopSession, request, nil)
}

func (e *remoteDesktopIPCClient) UpdateSession(payload RemoteDesktopCommandPayload) error {
	if e == nil || e.process == nil {
		return errors.New("remote desktop subsystem not initialized")
	}
	return e.process.call(context.Background(), methodUpdateSession, payload, nil)
}

func (e *remoteDesktopIPCClient) HandleInput(ctx context.Context, payload RemoteDesktopCommandPayload) error {
	if e == nil || e.process == nil {
		return errors.New("remote desktop subsystem not initialized")
	}
	return e.process.call(ctx, methodHandleInput, payload, nil)
}

func (e *remoteDesktopIPCClient) DeliverFrame(ctx context.Context, frame RemoteDesktopFramePacket) error {
	if e == nil || e.process == nil {
		return errors.New("remote desktop subsystem not initialized")
	}
	return e.process.call(ctx, methodDeliverFrame, frame, nil)
}

func (e *remoteDesktopIPCClient) Shutdown() {
	if e == nil || e.process == nil {
		return
	}
	e.process.shutdown()
}

type engineProcess struct {
	path    string
	version string
	manager *plugins.Manager
	logger  Logger

	mu       sync.Mutex
	cmd      *exec.Cmd
	cancel   context.CancelFunc
	done     chan processExit
	stopping bool
	output   *processOutputBuffer

	stdin   io.WriteCloser
	stdout  io.ReadCloser
	writer  *bufio.Writer
	encoder *json.Encoder
	decoder *json.Decoder
	nextID  uint64
}

type processExit struct {
	err      error
	stopping bool
}

func newEngineProcess(path, version string, manager *plugins.Manager, logger Logger) *engineProcess {
	return &engineProcess{
		path:    strings.TrimSpace(path),
		version: strings.TrimSpace(version),
		manager: manager,
		logger:  logger,
	}
}

func (p *engineProcess) call(ctx context.Context, method string, payload interface{}, result interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if method == methodShutdown {
		if p.cmd == nil {
			return nil
		}
	} else {
		if err := p.ensureStartedLocked(); err != nil {
			return err
		}
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	p.nextID++
	req := ipcRequest{ID: p.nextID, Method: method}
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode %s payload: %w", method, err)
		}
		req.Params = data
	}

	if p.encoder == nil || p.decoder == nil {
		return errors.New("remote desktop engine ipc channel unavailable")
	}

	if err := p.encoder.Encode(&req); err != nil {
		p.resetLocked()
		return fmt.Errorf("send %s request: %w", method, err)
	}
	if p.writer != nil {
		if err := p.writer.Flush(); err != nil {
			p.resetLocked()
			return fmt.Errorf("flush %s request: %w", method, err)
		}
	}

	var resp ipcResponse
	if err := p.decoder.Decode(&resp); err != nil {
		p.resetLocked()
		return fmt.Errorf("receive %s response: %w", method, err)
	}

	if resp.ID != req.ID {
		return fmt.Errorf("unexpected response id: got %d, want %d", resp.ID, req.ID)
	}
	if resp.Error != nil {
		return errors.New(resp.Error.Message)
	}
	if result != nil && len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return fmt.Errorf("decode %s result: %w", method, err)
		}
	}
	return nil
}

func (p *engineProcess) ensureStartedLocked() error {
	if p.cmd != nil {
		if p.done != nil {
			select {
			case exit := <-p.done:
				if !exit.stopping {
					if exit.err != nil {
						p.logf("engine exited: %v", exit.err)
					} else {
						p.logf("engine exited unexpectedly")
					}
				}
				p.resetLocked()
			default:
				return nil
			}
		} else {
			return nil
		}
	}

	if strings.TrimSpace(p.path) == "" {
		message := "engine entry path not configured"
		plugins.RecordInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID, p.version, manifest.InstallError, message)
		return errors.New(message)
	}

	info, err := os.Stat(p.path)
	if err != nil {
		message := fmt.Sprintf("engine binary unavailable: %v", err)
		plugins.RecordInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID, p.version, manifest.InstallError, message)
		return errors.New(message)
	}
	if info.IsDir() {
		message := "engine entry path resolves to a directory"
		plugins.RecordInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID, p.version, manifest.InstallError, message)
		return errors.New(message)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, p.path)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		message := fmt.Sprintf("engine stdin pipe: %v", err)
		plugins.RecordInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID, p.version, manifest.InstallError, message)
		return errors.New(message)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		cancel()
		message := fmt.Sprintf("engine stdout pipe: %v", err)
		plugins.RecordInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID, p.version, manifest.InstallError, message)
		return errors.New(message)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		cancel()
		message := fmt.Sprintf("engine stderr pipe: %v", err)
		plugins.RecordInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID, p.version, manifest.InstallError, message)
		return errors.New(message)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		cancel()
		message := fmt.Sprintf("engine launch failed: %v", err)
		plugins.RecordInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID, p.version, manifest.InstallError, message)
		return fmt.Errorf("remote desktop engine launch: %w", err)
	}

	plugins.ClearInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID)

	p.cmd = cmd
	p.cancel = cancel
	p.done = make(chan processExit, 1)
	p.stopping = false
	p.output = newProcessOutputBuffer(4096)
	p.stdin = stdin
	p.stdout = stdout
	p.writer = bufio.NewWriter(stdin)
	p.encoder = json.NewEncoder(p.writer)
	p.decoder = json.NewDecoder(stdout)
	p.nextID = 0

	go p.captureStream("stderr", stderr)
	go p.wait(cmd)
	return nil
}

func (p *engineProcess) stop() error {
	p.mu.Lock()
	cmd := p.cmd
	cancel := p.cancel
	done := p.done
	if cmd == nil {
		p.mu.Unlock()
		return nil
	}
	p.stopping = true
	p.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	var waitErr error
	if done != nil {
		select {
		case exit := <-done:
			waitErr = exit.err
		case <-time.After(5 * time.Second):
			p.mu.Lock()
			if p.cmd != nil && p.cmd.Process != nil {
				_ = p.cmd.Process.Kill()
			}
			p.mu.Unlock()
			if exit := <-done; exit.err != nil {
				waitErr = exit.err
			}
		}
	}

	plugins.ClearInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID)
	return waitErr
}

func (p *engineProcess) shutdown() {
	_ = p.call(context.Background(), methodShutdown, nil, nil)
	_ = p.stop()
}

func (p *engineProcess) wait(cmd *exec.Cmd) {
	err := cmd.Wait()

	p.mu.Lock()
	done := p.done
	stopping := p.stopping
	buffer := p.output
	logger := p.logger
	version := p.version
	p.resetLocked()
	p.mu.Unlock()

	if done != nil {
		done <- processExit{err: err, stopping: stopping}
	}

	if stopping {
		return
	}

	output := ""
	if buffer != nil {
		output = buffer.String()
	}

	var message string
	if err != nil {
		message = fmt.Sprintf("engine process exited: %v", err)
	} else {
		message = "engine process exited unexpectedly"
	}
	if output != "" {
		message = fmt.Sprintf("%s; output: %s", message, output)
	}

	plugins.RecordInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID, version, manifest.InstallError, message)
	if logger != nil {
		logger.Printf("remote desktop engine: %s", message)
	}
}

func (p *engineProcess) captureStream(stream string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 4096)
	scanner.Buffer(buf, 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if p.output != nil {
			p.output.append(line)
		}
		if p.logger != nil {
			p.logger.Printf("remote desktop engine %s: %s", stream, line)
		}
	}
}

func (p *engineProcess) resetLocked() {
	if p.stdin != nil {
		p.stdin.Close()
	}
	if p.stdout != nil {
		p.stdout.Close()
	}
	p.cmd = nil
	p.cancel = nil
	p.done = nil
	p.output = nil
	p.stopping = false
	p.stdin = nil
	p.stdout = nil
	p.writer = nil
	p.encoder = nil
	p.decoder = nil
	p.nextID = 0
}

func (p *engineProcess) logf(format string, args ...interface{}) {
	if p.logger == nil {
		return
	}
	p.logger.Printf(format, args...)
}

type processOutputBuffer struct {
	mu    sync.Mutex
	data  []byte
	limit int
}

func newProcessOutputBuffer(limit int) *processOutputBuffer {
	return &processOutputBuffer{limit: limit}
}

func (b *processOutputBuffer) append(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.limit <= 0 {
		return
	}
	payload := append([]byte(line), '\n')
	if len(payload) >= b.limit {
		b.data = append([]byte(nil), payload[len(payload)-b.limit:]...)
		return
	}
	if len(b.data)+len(payload) > b.limit {
		overflow := len(b.data) + len(payload) - b.limit
		b.data = b.data[overflow:]
	}
	b.data = append(b.data, payload...)
}

func (b *processOutputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(string(b.data))
}
