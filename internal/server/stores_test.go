package server

import (
	"context"
	"testing"
)

type MockTrackStore struct{}

func (m *MockTrackStore) SaveTrack(ctx context.Context, track *Track) error {
	return nil
}

func NewMockTrackStore(t *testing.T) *MockTrackStore {
	t.Helper()

	return &MockTrackStore{}
}
