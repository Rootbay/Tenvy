package appvnc

import (
	"context"
	"testing"

	"github.com/rootbay/tenvy-client/internal/protocol"
)

func TestHandleInputBurstValidatesSession(t *testing.T) {
	controller := NewController()

	err := controller.HandleInputBurst(context.Background(), protocol.AppVncInputBurst{
		SessionID: "session-1",
		Events: []protocol.AppVncInputEvent{{
			Type:       protocol.AppVncInputPointerMove,
			CapturedAt: 1,
			X:          0.1,
			Y:          0.2,
		}},
	})
	if err == nil || err.Error() != "no active session" {
		t.Fatalf("expected no active session error, got: %v", err)
	}

	controller.session = &sessionState{id: "session-1"}

	err = controller.HandleInputBurst(context.Background(), protocol.AppVncInputBurst{
		SessionID: " ",
		Events: []protocol.AppVncInputEvent{{
			Type:       protocol.AppVncInputPointerMove,
			CapturedAt: 2,
			X:          0.3,
			Y:          0.4,
		}},
	})
	if err == nil || err.Error() != "missing session identifier" {
		t.Fatalf("expected missing session identifier error, got: %v", err)
	}

	err = controller.HandleInputBurst(context.Background(), protocol.AppVncInputBurst{
		SessionID: "session-2",
		Events: []protocol.AppVncInputEvent{{
			Type:       protocol.AppVncInputPointerMove,
			CapturedAt: 3,
			X:          0.5,
			Y:          0.6,
		}},
	})
	if err == nil || err.Error() != "session identifier mismatch" {
		t.Fatalf("expected session identifier mismatch error, got: %v", err)
	}
}

func TestHandleInputBurstQueuesEvents(t *testing.T) {
	controller := NewController()
	controller.session = &sessionState{id: "session-1"}

	burst := protocol.AppVncInputBurst{
		SessionID: "session-1",
		Sequence:  7,
		Events: []protocol.AppVncInputEvent{{
			Type:       protocol.AppVncInputPointerButton,
			CapturedAt: 99,
			Button:     protocol.AppVncPointerButtonLeft,
			Pressed:    true,
		}},
	}

	if err := controller.HandleInputBurst(context.Background(), burst); err != nil {
		t.Fatalf("HandleInputBurst returned error: %v", err)
	}

	burst.Events[0].Pressed = false
	burst.Events[0].Button = protocol.AppVncPointerButtonRight

	controller.mu.Lock()
	defer controller.mu.Unlock()

	if controller.session == nil {
		t.Fatalf("session cleared unexpectedly")
	}
	if len(controller.session.inputQueue) != 1 {
		t.Fatalf("expected 1 queued burst, got %d", len(controller.session.inputQueue))
	}

	stored := controller.session.inputQueue[0]
	if stored.SessionID != "session-1" {
		t.Fatalf("unexpected stored session id: %s", stored.SessionID)
	}
	if stored.Sequence != 7 {
		t.Fatalf("unexpected stored sequence: %d", stored.Sequence)
	}
	if len(stored.Events) != 1 {
		t.Fatalf("unexpected stored event count: %d", len(stored.Events))
	}
	if !stored.Events[0].Pressed {
		t.Fatalf("expected stored event to remain pressed")
	}
	if stored.Events[0].Button != protocol.AppVncPointerButtonLeft {
		t.Fatalf("expected stored event button to remain left, got %s", stored.Events[0].Button)
	}
	if controller.session.lastSequence != 7 {
		t.Fatalf("unexpected last sequence: %d", controller.session.lastSequence)
	}
}
