package server

import (
	"math"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestPlaybackValidation(t *testing.T) {
	point := func(n float64) *float64 { return &n }
	for _, tc := range []struct {
		name     string
		settings PlaybackSettings
		valid    bool
	}{
		{"defaults", PlaybackSettings{}, true},
		{"all points", PlaybackSettings{StartTime: 10, EndTime: point(50), LoopBackTime: point(20)}, true},
		{"loop at beginning", PlaybackSettings{StartTime: 10, LoopBackTime: point(10)}, true},
		{"negative beginning", PlaybackSettings{StartTime: -1}, false},
		{"infinite beginning", PlaybackSettings{StartTime: math.Inf(1)}, false},
		{"NaN beginning", PlaybackSettings{StartTime: math.NaN()}, false},
		{"end before beginning", PlaybackSettings{StartTime: 10, EndTime: point(9)}, false},
		{"end at beginning", PlaybackSettings{StartTime: 10, EndTime: point(10)}, false},
		{"infinite end", PlaybackSettings{EndTime: point(math.Inf(1))}, false},
		{"NaN end", PlaybackSettings{EndTime: point(math.NaN())}, false},
		{"loop before beginning", PlaybackSettings{StartTime: 10, LoopBackTime: point(9)}, false},
		{"loop at end", PlaybackSettings{EndTime: point(50), LoopBackTime: point(50)}, false},
		{"NaN loop", PlaybackSettings{LoopBackTime: point(math.NaN())}, false},
		{"infinite loop", PlaybackSettings{LoopBackTime: point(math.Inf(1))}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.settings.Validate(); (err == nil) != tc.valid {
				t.Fatalf("Validate() = %v; valid = %v", err, tc.valid)
			}
		})
	}
}

func TestRoutedPlaybackValidation(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	for _, body := range []string{
		`{"playback":{"startTime":-1}}`,
		`{"playback":{"startTime":10,"endTime":5}}`,
		`{"playback":{"endTime":10,"loopBackTime":10}}`,
		`{"playback":{"startTime":10,"loopBackTime":5}}`,
		`{"playback":{"startTime":"invalid"}}`,
	} {
		rec := routedRequest(ts.Handler(), http.MethodPut, "/api/v1/files/"+uuid.NewString(), body, true)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d: %s", body, rec.Code, rec.Body)
		}
	}
}
