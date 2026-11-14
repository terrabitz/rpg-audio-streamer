package e2e

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/disgoorg/godave"
)

func TestJoinerSessionLeavesInitialCommitToExistingMembers(t *testing.T) {
	gateway := &recordingCallbacks{}
	inner := &fakeSession{}
	create := joinerSession(func(_ *slog.Logger, _ godave.UserID, callbacks godave.Callbacks) godave.Session {
		inner.callbacks = callbacks
		return inner
	})
	s := create(slog.Default(), "1", gateway)

	_ = inner.callbacks.SendMLSKeyPackage([]byte("key"))
	_ = inner.callbacks.SendMLSCommitWelcome([]byte("racing commit"))
	if s.Ready() {
		t.Fatal("ready before joining an epoch")
	}
	_ = inner.callbacks.SendMLSCommitWelcome([]byte("still joining"))

	inner.ready = true
	if !s.Ready() {
		t.Fatal("not ready after joining an epoch")
	}
	_ = inner.callbacks.SendMLSCommitWelcome([]byte("member commit"))
	_ = inner.callbacks.SendReadyForTransition(3)
	_ = inner.callbacks.SendInvalidCommitWelcome(4)

	want := []string{"key", "member commit", "ready 3", "invalid 4"}
	if len(gateway.sent) != len(want) {
		t.Fatalf("sent %q, want %q", gateway.sent, want)
	}
	for i := range want {
		if gateway.sent[i] != want[i] {
			t.Fatalf("sent %q, want %q", gateway.sent, want)
		}
	}
}

type fakeSession struct {
	godave.Session
	callbacks godave.Callbacks
	ready     bool
}

func (f *fakeSession) Ready() bool { return f.ready }

type recordingCallbacks struct {
	sent []string
}

func (r *recordingCallbacks) SendMLSKeyPackage(p []byte) error {
	r.sent = append(r.sent, string(p))
	return nil
}

func (r *recordingCallbacks) SendMLSCommitWelcome(p []byte) error {
	r.sent = append(r.sent, string(p))
	return nil
}

func (r *recordingCallbacks) SendReadyForTransition(id uint16) error {
	r.sent = append(r.sent, fmt.Sprint("ready ", id))
	return nil
}

func (r *recordingCallbacks) SendInvalidCommitWelcome(id uint16) error {
	r.sent = append(r.sent, fmt.Sprint("invalid ", id))
	return nil
}
