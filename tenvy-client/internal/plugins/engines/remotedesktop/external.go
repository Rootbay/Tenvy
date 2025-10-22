package remotedesktopengine

import (
	"bufio"
	"context"
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

type managedEngine struct {
	inner   Engine
	process *engineProcess
}

// NewManagedRemoteDesktopEngine wraps the provided engine implementation with a
// lifecycle manager that ensures the external remote desktop engine binary is
// launched when sessions start and torn down when they conclude. If the plugin
// entry path is empty or the manager is nil, the underlying engine is returned
// unchanged.
func NewManagedRemoteDesktopEngine(inner Engine, entryPath, version string, manager *plugins.Manager, logger Logger) Engine {
	entryPath = strings.TrimSpace(entryPath)
	if entryPath == "" || manager == nil {
		return inner
	}
	process := newEngineProcess(entryPath, version, manager, logger)
	return &managedEngine{inner: inner, process: process}
}

func (e *managedEngine) Configure(cfg Config) error {
	if e == nil || e.inner == nil {
		return errors.New("remote desktop subsystem not initialized")
	}
	return e.inner.Configure(cfg)
}

func (e *managedEngine) StartSession(ctx context.Context, payload RemoteDesktopCommandPayload) error {
	if e == nil || e.inner == nil {
		return errors.New("remote desktop subsystem not initialized")
	}
	if err := e.process.start(payload.SessionID); err != nil {
		return err
	}
	if err := e.inner.StartSession(ctx, payload); err != nil {
		_ = e.process.stop()
		return err
	}
	return nil
}

func (e *managedEngine) StopSession(sessionID string) error {
	if e == nil || e.inner == nil {
		return errors.New("remote desktop subsystem not initialized")
	}
	var stopErr error
	if err := e.inner.StopSession(sessionID); err != nil {
		stopErr = err
	}
	if err := e.process.stop(); err != nil && stopErr == nil {
		stopErr = err
	}
	return stopErr
}

func (e *managedEngine) UpdateSession(payload RemoteDesktopCommandPayload) error {
	if e == nil || e.inner == nil {
		return errors.New("remote desktop subsystem not initialized")
	}
	return e.inner.UpdateSession(payload)
}

func (e *managedEngine) HandleInput(ctx context.Context, payload RemoteDesktopCommandPayload) error {
	if e == nil || e.inner == nil {
		return errors.New("remote desktop subsystem not initialized")
	}
	return e.inner.HandleInput(ctx, payload)
}

func (e *managedEngine) DeliverFrame(ctx context.Context, frame RemoteDesktopFramePacket) error {
	if e == nil || e.inner == nil {
		return errors.New("remote desktop subsystem not initialized")
	}
	return e.inner.DeliverFrame(ctx, frame)
}

func (e *managedEngine) Shutdown() {
	if e == nil {
		return
	}
	if e.inner != nil {
		e.inner.Shutdown()
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

func (p *engineProcess) start(sessionID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd != nil {
		if p.done != nil {
			select {
			case <-p.done:
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
		plugins.RecordInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID, p.version, manifest.InstallFailed, message)
		return errors.New(message)
	}

	if info, err := os.Stat(p.path); err != nil {
		message := fmt.Sprintf("engine binary unavailable: %v", err)
		plugins.RecordInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID, p.version, manifest.InstallFailed, message)
		return errors.New(message)
	} else if info.IsDir() {
		message := "engine entry path resolves to a directory"
		plugins.RecordInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID, p.version, manifest.InstallFailed, message)
		return errors.New(message)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, p.path)
	cmd.Env = append(os.Environ(), fmt.Sprintf("TENVY_REMOTE_DESKTOP_SESSION_ID=%s", strings.TrimSpace(sessionID)))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		message := fmt.Sprintf("engine stdout pipe: %v", err)
		plugins.RecordInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID, p.version, manifest.InstallFailed, message)
		return errors.New(message)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdout.Close()
		cancel()
		message := fmt.Sprintf("engine stderr pipe: %v", err)
		plugins.RecordInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID, p.version, manifest.InstallFailed, message)
		return errors.New(message)
	}

	if err := cmd.Start(); err != nil {
		stdout.Close()
		stderr.Close()
		cancel()
		message := fmt.Sprintf("engine launch failed: %v", err)
		plugins.RecordInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID, p.version, manifest.InstallFailed, message)
		return fmt.Errorf("remote desktop engine launch: %w", err)
	}

	plugins.ClearInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID)

	p.cmd = cmd
	p.cancel = cancel
	p.done = make(chan processExit, 1)
	p.stopping = false
	p.output = newProcessOutputBuffer(4096)

	go p.captureStream("stdout", stdout)
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
	var stopping bool
	if done != nil {
		select {
		case exit := <-done:
			waitErr = exit.err
			stopping = exit.stopping
		case <-time.After(5 * time.Second):
			p.mu.Lock()
			if p.cmd != nil && p.cmd.Process != nil {
				_ = p.cmd.Process.Kill()
			}
			p.mu.Unlock()
			if done != nil {
				exit := <-done
				waitErr = exit.err
				stopping = exit.stopping
			}
		}
	}

	plugins.ClearInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID)
	if stopping {
		return nil
	}
	return waitErr
}

func (p *engineProcess) shutdown() {
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

	plugins.RecordInstallStatus(p.manager, plugins.RemoteDesktopEnginePluginID, version, manifest.InstallFailed, message)
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
	p.cmd = nil
	p.cancel = nil
	p.done = nil
	p.output = nil
	p.stopping = false
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
