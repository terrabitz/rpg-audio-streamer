package e2e

import (
	"log/slog"
	"sync/atomic"

	"github.com/disgoorg/godave"
)

// joinerSession keeps the listener from committing until it has joined an
// epoch. When dave-go's commit wins the join race, the streamer's libdave
// session rejects dave-go's Welcome and the group never settles; leaving the
// commit to the streamer makes the listener join through the streamer's
// Welcome instead.
func joinerSession(create godave.SessionCreateFunc) godave.SessionCreateFunc {
	return func(logger *slog.Logger, userID godave.UserID, callbacks godave.Callbacks) godave.Session {
		s := &joiner{gateway: callbacks}
		s.Session = create(logger, userID, s)
		return s
	}
}

type joiner struct {
	godave.Session
	gateway godave.Callbacks
	joined  atomic.Bool
}

// Ready is polled by the audio receiver outside the session's lock, which is
// held while callbacks run, so readiness is recorded here.
func (j *joiner) Ready() bool {
	ready := j.Session.Ready()
	if ready {
		j.joined.Store(true)
	}
	return ready
}

func (j *joiner) SendMLSKeyPackage(keyPackage []byte) error {
	return j.gateway.SendMLSKeyPackage(keyPackage)
}

func (j *joiner) SendMLSCommitWelcome(commitWelcome []byte) error {
	if !j.joined.Load() {
		return nil
	}
	return j.gateway.SendMLSCommitWelcome(commitWelcome)
}

func (j *joiner) SendReadyForTransition(transitionID uint16) error {
	return j.gateway.SendReadyForTransition(transitionID)
}

func (j *joiner) SendInvalidCommitWelcome(transitionID uint16) error {
	return j.gateway.SendInvalidCommitWelcome(transitionID)
}
