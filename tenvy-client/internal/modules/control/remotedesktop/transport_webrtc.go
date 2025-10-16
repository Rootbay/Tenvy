package remotedesktop

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
)

const remoteDesktopDataChannelLabel = "remote-desktop-frames"

type webrtcOfferHandle struct {
	pc     *webrtc.PeerConnection
	dc     *webrtc.DataChannel
	offer  string
	label  string
	ready  chan struct{}
	closed chan struct{}
	mu     sync.Mutex
	used   bool
}

type webrtcFrameTransport struct {
	pc        *webrtc.PeerConnection
	dc        *webrtc.DataChannel
	ready     <-chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
	mu        sync.Mutex
}

func prepareWebRTCOffer(ctx context.Context) (*webrtcOfferHandle, error) {
	api := webrtc.NewAPI()
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, err
	}

	ordered := false
	maxRetransmits := uint16(1)
	dc, err := pc.CreateDataChannel(remoteDesktopDataChannelLabel, &webrtc.DataChannelInit{
		Ordered:        &ordered,
		MaxRetransmits: &maxRetransmits,
	})
	if err != nil {
		pc.Close()
		return nil, err
	}

	ready := make(chan struct{})
	closed := make(chan struct{})
	dc.OnOpen(func() {
		select {
		case <-ready:
		default:
			close(ready)
		}
	})
	dc.OnClose(func() {
		select {
		case <-closed:
		default:
			close(closed)
		}
	})
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed || state == webrtc.PeerConnectionStateDisconnected {
			select {
			case <-closed:
			default:
				close(closed)
			}
		}
	})

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		dc.Close()
		pc.Close()
		return nil, err
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		dc.Close()
		pc.Close()
		return nil, err
	}

	gather := webrtc.GatheringCompletePromise(pc)
	select {
	case <-gather:
	case <-ctx.Done():
		dc.Close()
		pc.Close()
		return nil, ctx.Err()
	case <-time.After(15 * time.Second):
		dc.Close()
		pc.Close()
		return nil, errors.New("remote desktop: webrtc ice gathering timeout")
	}

	local := pc.LocalDescription()
	if local == nil {
		dc.Close()
		pc.Close()
		return nil, errors.New("remote desktop: missing local description")
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(local.SDP))
	return &webrtcOfferHandle{
		pc:     pc,
		dc:     dc,
		offer:  encoded,
		label:  remoteDesktopDataChannelLabel,
		ready:  ready,
		closed: closed,
	}, nil
}

func (h *webrtcOfferHandle) Offer() string {
	if h == nil {
		return ""
	}
	return h.offer
}

func (h *webrtcOfferHandle) Label() string {
	if h == nil {
		return ""
	}
	return h.label
}

func (h *webrtcOfferHandle) Accept(ctx context.Context, answer string) (frameTransport, error) {
	if h == nil {
		return nil, errors.New("remote desktop: missing webrtc offer handle")
	}
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" {
		return nil, errors.New("remote desktop: missing webrtc answer")
	}

	h.mu.Lock()
	if h.used {
		h.mu.Unlock()
		return nil, errors.New("remote desktop: webrtc offer already accepted")
	}
	h.used = true
	pc := h.pc
	dc := h.dc
	ready := h.ready
	closed := h.closed
	h.pc = nil
	h.dc = nil
	h.mu.Unlock()

	if pc == nil || dc == nil {
		return nil, errors.New("remote desktop: invalid webrtc handle state")
	}

	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		pc.Close()
		return nil, err
	}

	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: string(decoded)}); err != nil {
		pc.Close()
		return nil, err
	}

	transport := &webrtcFrameTransport{
		pc:     pc,
		dc:     dc,
		ready:  ready,
		closed: closed,
	}

	dc.OnClose(func() {
		transport.Close()
	})
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed || state == webrtc.PeerConnectionStateDisconnected {
			transport.Close()
		}
	})

	return transport, nil
}

func (h *webrtcOfferHandle) Close() {
	if h == nil {
		return
	}
	h.mu.Lock()
	pc := h.pc
	dc := h.dc
	h.pc = nil
	h.dc = nil
	h.mu.Unlock()

	if dc != nil {
		_ = dc.Close()
	}
	if pc != nil {
		_ = pc.Close()
	}
}

func (t *webrtcFrameTransport) Ready() bool {
	if t == nil {
		return false
	}
	select {
	case <-t.ready:
		return true
	default:
		return false
	}
}

func (t *webrtcFrameTransport) Send(ctx context.Context, frame RemoteDesktopFramePacket) error {
	if t == nil {
		return errors.New("remote desktop: webrtc transport unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	payload, err := json.Marshal(frame)
	if err != nil {
		return err
	}

	select {
	case <-t.closed:
		return errors.New("remote desktop: webrtc transport closed")
	case <-t.ready:
	case <-ctx.Done():
		return ctx.Err()
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.dc == nil {
		return errors.New("remote desktop: webrtc channel closed")
	}
	return t.dc.SendText(string(payload))
}

func (t *webrtcFrameTransport) Close() error {
	if t == nil {
		return nil
	}
	var err error
	t.closeOnce.Do(func() {
		t.mu.Lock()
		dc := t.dc
		pc := t.pc
		t.dc = nil
		t.pc = nil
		t.mu.Unlock()

		if dc != nil {
			if closeErr := dc.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}
		if pc != nil {
			if closeErr := pc.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}
		select {
		case <-t.closed:
		default:
			close(t.closed)
		}
	})
	return err
}
