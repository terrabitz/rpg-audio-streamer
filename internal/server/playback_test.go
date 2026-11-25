package server

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

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
