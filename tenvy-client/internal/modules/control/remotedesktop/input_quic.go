package remotedesktop

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	quic "github.com/quic-go/quic-go"
)

type quicInputBridge struct {
	cfg     sanitizedQUICInput
	agentID string
	logger  Logger
	handler func([]RemoteDesktopInputEvent) error

	mu     sync.Mutex
	conn   quic.Connection
	stream quic.Stream
	active atomic.Bool
	closed chan struct{}
}

type quicInputMessage struct {
	Type      string                    `json:"type"`
	SessionID string                    `json:"sessionId"`
	Sequence  uint64                    `json:"sequence"`
	Events    []RemoteDesktopInputEvent `json:"events"`
	Reason    string                    `json:"reason,omitempty"`
}

func newQuicInputBridge(cfg sanitizedQUICInput, agentID string, logger Logger, handler func([]RemoteDesktopInputEvent) error) *quicInputBridge {
	if !cfg.enabled || agentID == "" || handler == nil {
		return nil
	}
	return &quicInputBridge{
		cfg:     cfg,
		agentID: agentID,
		logger:  logger,
		handler: handler,
		closed:  make(chan struct{}),
	}
}

func (b *quicInputBridge) Start(ctx context.Context, sessionID string) {
	if b == nil || !b.cfg.enabled || sessionID == "" {
		return
	}
	if !b.active.CompareAndSwap(false, true) {
		return
	}
	go b.run(ctx, sessionID)
}

func (b *quicInputBridge) Close() {
	if b == nil {
		return
	}
	if !b.active.CompareAndSwap(true, false) {
		closeOnce(b.closed)
		return
	}
	b.mu.Lock()
	if b.stream != nil {
		_ = b.stream.Close()
		b.stream = nil
	}
	if b.conn != nil {
		_ = b.conn.CloseWithError(0, "session closed")
		b.conn = nil
	}
	b.mu.Unlock()
	closeOnce(b.closed)
}

func (b *quicInputBridge) run(ctx context.Context, sessionID string) {
	defer closeOnce(b.closed)
	defer b.active.Store(false)

	retry := b.cfg.retryInterval
	if retry <= 0 {
		retry = defaultQuicRetryInterval
	}

	for {
		if ctx.Err() != nil {
			return
		}
		if err := b.connectAndServe(ctx, sessionID); err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				b.logf("remote desktop: quic input bridge error: %v", err)
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(retry):
		}
	}
}

func (b *quicInputBridge) connectAndServe(ctx context.Context, sessionID string) error {
	dialTimeout := b.cfg.connectTimeout
	if dialTimeout <= 0 {
		dialTimeout = defaultQuicConnectTimeout
	}

	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS13,
		NextProtos: []string{b.cfg.alpn},
		ServerName: b.cfg.serverName,
		RootCAs:    b.cfg.rootCAs,
	}

	if len(b.cfg.spkiPins) > 0 {
		pins := make([][]byte, len(b.cfg.spkiPins))
		for i, pin := range b.cfg.spkiPins {
			pins[i] = append([]byte(nil), pin...)
		}

		tlsCfg.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("remote desktop: peer certificate missing")
			}
			cert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return fmt.Errorf("remote desktop: parse peer certificate: %w", err)
			}
			fingerprint := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
			for _, expected := range pins {
				if bytes.Equal(fingerprint[:], expected) {
					return nil
				}
			}
			return errors.New("remote desktop: peer certificate pin mismatch")
		}
	}

	quicCfg := &quic.Config{KeepAlivePeriod: 30 * time.Second}

	conn, err := quic.DialAddr(dialCtx, b.cfg.address, tlsCfg, quicCfg)
	if err != nil {
		return fmt.Errorf("dial quic: %w", err)
	}

	stream, err := conn.OpenStreamSync(dialCtx)
	if err != nil {
		conn.CloseWithError(0, "open stream failed")
		return fmt.Errorf("open stream: %w", err)
	}

	b.mu.Lock()
	b.conn = conn
	b.stream = stream
	b.mu.Unlock()

	if err := b.sendRegister(stream, sessionID); err != nil {
		b.closeConnection("register failed")
		return err
	}

	return b.readLoop(ctx, stream, sessionID)
}

func (b *quicInputBridge) readLoop(ctx context.Context, stream quic.Stream, sessionID string) error {
	reader := bufio.NewReader(stream)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read message: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var message quicInputMessage
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			b.logf("remote desktop: invalid quic input message: %v", err)
			continue
		}

		switch strings.ToLower(strings.TrimSpace(message.Type)) {
		case "input":
			if message.SessionID != "" && message.SessionID != sessionID {
				continue
			}
			if len(message.Events) == 0 {
				continue
			}
			if err := b.handler(message.Events); err != nil {
				b.logf("remote desktop: failed to process quic input: %v", err)
			}
			if message.Sequence > 0 {
				if err := b.sendAck(stream, sessionID, message.Sequence); err != nil {
					b.logf("remote desktop: failed to ack quic input: %v", err)
				}
			}
		case "close":
			return nil
		case "registered":
			continue
		case "pong":
			continue
		default:
			if err := b.handleControlMessage(stream, &message, sessionID); err != nil {
				b.logf("remote desktop: quic control message error: %v", err)
			}
		}
	}
}

func (b *quicInputBridge) handleControlMessage(stream quic.Stream, message *quicInputMessage, sessionID string) error {
	switch strings.ToLower(strings.TrimSpace(message.Type)) {
	case "ping":
		return b.sendPong(stream)
	default:
		return nil
	}
}

func (b *quicInputBridge) sendRegister(stream quic.Stream, sessionID string) error {
	payload := map[string]any{
		"type":      "register",
		"agentId":   b.agentID,
		"sessionId": sessionID,
	}
	if b.cfg.token != "" {
		payload["token"] = b.cfg.token
	}
	return b.sendMessage(stream, payload)
}

func (b *quicInputBridge) sendAck(stream quic.Stream, sessionID string, sequence uint64) error {
	payload := map[string]any{
		"type":      "ack",
		"sessionId": sessionID,
		"sequence":  sequence,
	}
	return b.sendMessage(stream, payload)
}

func (b *quicInputBridge) sendPong(stream quic.Stream) error {
	payload := map[string]any{
		"type":      "pong",
		"timestamp": time.Now().UnixMilli(),
	}
	return b.sendMessage(stream, payload)
}

func (b *quicInputBridge) sendMessage(stream quic.Stream, payload any) error {
	if stream == nil {
		return errors.New("quic stream unavailable")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = stream.Write(data)
	return err
}

func (b *quicInputBridge) closeConnection(reason string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stream != nil {
		_ = b.stream.Close()
		b.stream = nil
	}
	if b.conn != nil {
		_ = b.conn.CloseWithError(0, reason)
		b.conn = nil
	}
}

func (b *quicInputBridge) logf(format string, args ...interface{}) {
	if b == nil || b.logger == nil {
		return
	}
	b.logger.Printf(format, args...)
}

func closeOnce(ch chan struct{}) {
	select {
	case <-ch:
	default:
		close(ch)
	}
}
