package server_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
	"github.com/terrabitz/rpg-audio-streamer/internal/musicprocessing"
	"github.com/terrabitz/rpg-audio-streamer/internal/playback"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore"
	"github.com/terrabitz/rpg-audio-streamer/internal/youtube"
)

type testWSRegisterer struct {
	err error
}

func (testWSRegisterer) Register(*websocket.Conn, *auth.Token)        {}
func (h testWSRegisterer) SendEvent(string, any) error                { return h.err }
func (testWSRegisterer) RevokePlayerAccess(string)                    {}
func (testWSRegisterer) On(string, events.Handler) func()             { return func() {} }
func (h testWSRegisterer) Publish(string, any, events.Metadata) error { return h.err }

func TestTableHTTP(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := sqlitedatastore.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrations, err := sqlitedatastore.NewMigration(os.DirFS("../../sql/migrations"), db)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authenticator, _, err := auth.New(context.Background(), auth.Config{
		TokenSecret:   "public-test-secret",
		TokenDuration: time.Hour,
		TokenIssuer:   "test",
		TokenAudience: "test",
	}, auth.BootstrapConfig{}, db, logger)
	if err != nil {
		t.Fatal(err)
	}
	gm, err := authenticator.NewToken("gm", auth.RoleGM)
	if err != nil {
		t.Fatal(err)
	}
	videos := youtube.New(logger)
	hub := &testWSRegisterer{}
	processing := musicprocessing.New(dir, logger, db, videos, hub)
	srv, err := server.New(server.Config{UploadDir: dir, DevMode: true}, logger, authenticator, db, hub, playback.New(logger, db, hub), processing, videos)
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body, _ string, asGM bool, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if asGM {
			req.AddCookie(&http.Cookie{Name: "auth_token", Value: gm.String()})
		}
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s = %d: %s; want %d", method, path, rec.Code, rec.Body, status)
		}
		return rec
	}
	decodeTable := func(rec *httptest.ResponseRecorder) server.Table {
		t.Helper()
		var table server.Table
		if err := json.Unmarshal(rec.Body.Bytes(), &table); err != nil {
			t.Fatal(err)
		}
		return table
	}
	request("GET", "/api/v1/tables", "", "", false, 401)
	request("GET", "/api/v1/tables", "", "legacy-invite", false, 401)
	for _, body := range []string{`{`, `{}`, `{"name":"   "}`, `{"name":"` + strings.Repeat("x", 201) + `"}`} {
		request("POST", "/api/v1/tables", body, "", true, 400)
	}
	first := decodeTable(request("POST", "/api/v1/tables", `{"name":" Friday "}`, "", true, 201))
	decodedExternalID, decodeErr := base64.RawURLEncoding.DecodeString(first.ExternalID)
	if first.Name != "Friday" || decodeErr != nil || len(decodedExternalID) != 32 {
		t.Fatalf("created table = %+v", first)
	}
	second := decodeTable(request("POST", "/api/v1/tables", `{"name":"Sunday"}`, "", true, 201))
	firstPath := "/api/v1/tables/" + first.ExternalID
	secondPath := "/api/v1/tables/" + second.ExternalID
	settingsPath := firstPath + "/settings"
	request("GET", settingsPath, "", "", false, 401)
	request("GET", settingsPath, "", "", false, 401)
	request("PATCH", settingsPath, `{"masterVolume":0}`, "", false, 401)
	request("POST", settingsPath, `{}`, "", true, 405)
	request("GET", "/api/v1/tables/not-a-table/settings", "", "", true, 404)
	for _, body := range []string{
		`{`, `null`, `{"currentTime":12}`, `{"isPlaying":true}`,
		`{"masterVolume":-1}`, `{"masterVolume":201}`, `{"masterVolume":"50"}`,
		`{"typeVolumes":{"Music":101}}`, `{"typeVolumes":{"Music":-1}}`,
		`{"typeVolumes":{" ":30}}`, `{"trackVolumes":{"` + uuid.NewString() + `":50}}`,
		`{"trackOrder":["bad"]}`, `{"trackOrder":["` + uuid.Nil.String() + `"]}`,
		`{"trackOrder":["` + first.ID.String() + `","` + first.ID.String() + `"]}`,
		`{} {}`,
	} {
		request("PATCH", settingsPath, body, "", true, 400)
	}
	readSettings := func(rec *httptest.ResponseRecorder) server.TableSettings {
		t.Helper()
		var settings server.TableSettings
		if err := json.Unmarshal(rec.Body.Bytes(), &settings); err != nil {
			t.Fatal(err)
		}
		return settings
	}
	initial := readSettings(request("GET", settingsPath, "", "", true, 200))
	if initial.MasterVolume != 100 || initial.TrackOrder == nil || initial.TypeVolumes == nil {
		t.Fatalf("default settings = %+v", initial)
	}
	hub.err = errors.New("event hub is closed")
	rejected := request("PATCH", settingsPath, `{"masterVolume":0}`, "", true, http.StatusServiceUnavailable)
	if !strings.Contains(rejected.Body.String(), "live synchronization failed") {
		t.Fatalf("event failure was hidden: %s", rejected.Body)
	}
	if persisted := readSettings(request("GET", settingsPath, "", "", true, 200)); persisted.MasterVolume != 0 {
		t.Fatal("event failure changed the outcome of the saved settings")
	}
	hub.err = nil
	readSettings(request("PATCH", settingsPath, `{"masterVolume":0,"typeVolumes":{"Music":0,"Ambiance":60}}`, "", true, 200))
	readSettings(request("PATCH", settingsPath, `{"typeVolumes":{"Ambiance":40}}`, "", true, 200))
	saved := readSettings(request("GET", settingsPath, "", "", true, 200))
	if saved.MasterVolume != 0 || saved.TypeVolumes["Music"] != 0 || saved.TypeVolumes["Ambiance"] != 40 {
		t.Fatalf("partial settings lost saved mix = %+v", saved)
	}
	if other := readSettings(request("GET", secondPath+"/settings", "", "", true, 200)); other.MasterVolume != 100 || len(other.TypeVolumes) != 0 {
		t.Fatalf("settings leaked to other table = %+v", other)
	}
	renamed := decodeTable(request("PUT", firstPath, `{"name":"Friday group"}`, "", true, 200))
	if renamed.Name != "Friday group" {
		t.Fatalf("renamed table = %+v", renamed)
	}
	playerTable := decodeTable(request("GET", firstPath, "", "", false, 200))
	if playerTable.ID != first.ID || playerTable.Name != renamed.Name {
		t.Fatalf("table URL resolved wrong table: %+v", playerTable)
	}
	request("GET", secondPath, "", "", false, 200)
	request("PUT", firstPath, `{"name":"Forbidden"}`, "", false, 403)
	request("DELETE", firstPath, "", "", false, 403)
	request("GET", firstPath+"/tracks", "", "", false, 200)
	request("GET", "/api/v1/tables/invalid", "", "", true, 404)
	request("GET", "/api/v1/tables/not-a-table", "", "", true, 404)
	request("PATCH", firstPath, "", "", true, 405)
	request("PATCH", "/api/v1/tables", "", "", true, 405)
	request("POST", "/api/v1/tables/current", "", "", true, 404)
	request("POST", firstPath+"/tracks", "", "", true, 405)
	types, err := db.GetTrackTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	track := server.Track{ID: uuid.New(), Name: "Forest", TypeID: types[0].ID, CreatedAt: time.Now()}
	track.Path = filepath.Join(dir, track.ID.String())
	if err := os.Mkdir(track.Path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(track.Path, "index.m3u8"), []byte("#EXTM3U\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveTrack(ctx, &track); err != nil {
		t.Fatal(err)
	}
	usagePath := "/api/v1/files/" + track.ID.String() + "/usage"
	filePath := "/api/v1/files/" + track.ID.String()
	assertUsage := func(want int64) {
		t.Helper()
		rec := request("GET", usagePath, "", "", true, 200)
		var usage map[string]int64
		err := json.Unmarshal(rec.Body.Bytes(), &usage)
		got, ok := usage["tableCount"]
		if err != nil || !ok || len(usage) != 1 || got != want {
			t.Fatalf("track usage = %s, %v; want %d", rec.Body, err, want)
		}
	}
	assertUsage(0)
	request("GET", usagePath, "", "", false, 401)
	request("GET", usagePath, "", "", false, 401)
	request("GET", "/api/v1/files/invalid/usage", "", "", true, 400)
	request("GET", "/api/v1/files/"+uuid.NewString()+"/usage", "", "", true, 404)
	request("POST", usagePath, "", "", true, 405)
	for _, query := range []string{
		"?expectedTableCount=-1", "?expectedTableCount=", "?expectedTableCount=bad",
		"?expectedTableCount=0&expectedTableCount=0", "?expectedTableCount=9223372036854775808",
	} {
		request("DELETE", filePath+query, "", "", true, 400)
	}
	association := firstPath + "/tracks/" + track.ID.String()
	request("PUT", association, "", "", true, 204)
	request("PUT", association, "", "", true, 204)
	t.Run("live playback and mix commands", func(t *testing.T) {
		path := firstPath + "/playback"
		command := path + "/" + track.ID.String()
		request("PATCH", command, `{"isPlaying":true}`, "", false, 401)
		request("PATCH", path+"/invalid", `{}`, "", true, 400)
		request("PATCH", secondPath+"/playback/"+track.ID.String(), `{"isPlaying":true}`, "", true, 404)
		for _, invalid := range []string{`null`, `{"volume":"0"}`, `{"fileID":"forged"}`, `{} {}`, `{"volume":-1}`} {
			request("PATCH", command, invalid, "", true, 400)
		}
		request("PATCH", command, `{"isPlaying":false,"currentTime":12,"volume":50,"duration":30}`, "", true, 204)
		read := func(asGM bool) events.Snapshot {
			res := request("GET", path, "", "", asGM, 200)
			var snapshot events.Snapshot
			if err := json.Unmarshal(res.Body.Bytes(), &snapshot); err != nil {
				t.Fatal(err)
			}
			if res.Header().Get("Cache-Control") != "no-store" || len(snapshot.Tracks) != 1 {
				t.Fatalf("snapshot = %+v, headers = %v", snapshot, res.Header())
			}
			return snapshot
		}
		gm, player := read(true), read(false)
		if gm.Settings == nil || player.Settings != nil || player.Revision <= gm.Revision ||
			gm.Tracks[0].Volume != 50 || player.Tracks[0].Volume != 0 ||
			player.Tracks[0].IsPlaying || player.Tracks[0].CurrentTime != 12 {
			t.Fatalf("unexpected playback projection: GM=%+v player=%+v", gm, player)
		}
		request("GET", "/api/v1/tables/no-table/playback", "", "", false, 404)
		mix := firstPath + "/mix"
		request("PUT", mix, `{"tracks":[]}`, "", false, 401)
		request("PATCH", mix, `{"typeVolumes":{"Music":0}}`, "", false, 401)
		request("PUT", mix, `{}`, "", true, 400)
		request("PATCH", mix, `{"typeVolumes":[]}`, "", true, 400)
		request("PATCH", mix, `{}`, "", true, 400)
		request("PUT", mix, `{"tracks":[]}`, "", true, 202)
		request("PATCH", mix, `{"typeVolumes":{"Music":0}}`, "", true, 202)
		hub.err = errors.New("event hub is closed")
		defer func() { hub.err = nil }()
		request("PUT", mix, `{"tracks":[]}`, "", true, 503)
	})
	assertUsage(1)
	request("DELETE", filePath+"?expectedTableCount=0", "", "", true, 409)
	assertUsage(1)
	secondAssociation := secondPath + "/tracks/" + track.ID.String()
	request("PUT", secondAssociation, "", "", true, 204)
	assertUsage(2)
	request("DELETE", filePath+"?expectedTableCount=1", "", "", true, 409)
	assertUsage(2)
	request("DELETE", secondAssociation, "", "", true, 204)
	assertUsage(1)
	request("PUT", association, "", "", false, 401)
	request("PUT", firstPath+"/tracks/"+uuid.NewString(), "", "", true, 404)
	request("PUT", firstPath+"/tracks/invalid", "", "", true, 400)
	request("PATCH", association, "", "", true, 405)
	list := request("GET", firstPath+"/tracks", "", "", true, 200)
	var tracks []server.Track
	if err := json.Unmarshal(list.Body.Bytes(), &tracks); err != nil || len(tracks) != 1 || tracks[0].ID != track.ID {
		t.Fatalf("associated tracks = %v, %v", tracks, err)
	}
	const diagnostics = "Command:\nyt-dlp -- source\n\nLogs:\nERROR: blocked\n\nError:\nexit status 1"
	failedTrack := server.Track{ID: uuid.New(), Name: "Failed import", TypeID: types[0].ID, CreatedAt: time.Now(),
		Status: "failed", ProcessingError: diagnostics}
	if err := db.SaveTrackToTable(ctx, &failedTrack, first.ID); err != nil {
		t.Fatal(err)
	}
	for _, asGM := range []bool{false, true} {
		rec := request("GET", firstPath+"/tracks", "", "", asGM, 200)
		var listed []server.Track
		if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil || len(listed) != 2 {
			t.Fatalf("failed track response = %s, %v", rec.Body, err)
		}
		var failed server.Track
		for _, item := range listed {
			if item.ID == failedTrack.ID {
				failed = item
			}
		}
		if failed.Status != "failed" || (asGM && failed.ProcessingError != diagnostics) || (!asGM && failed.ProcessingError != "") {
			t.Fatalf("diagnostics visibility for GM=%v: %s", asGM, rec.Body)
		}
	}
	if err := db.DeleteTrack(ctx, failedTrack.ID); err != nil {
		t.Fatal(err)
	}
	stream := firstPath + "/stream/" + track.ID.String() + "/index.m3u8"
	request("GET", stream, "", "", false, 200)
	request("GET", secondPath+"/stream/"+track.ID.String()+"/index.m3u8", "", "", false, 404)
	request("GET", "/api/v1/tables/invalid/stream/"+track.ID.String()+"/index.m3u8", "", "", false, 404)
	request("POST", firstPath+"/external-id", "", "", false, 401)
	rotated := decodeTable(request("POST", firstPath+"/external-id", "", "", true, 200))
	if rotated.ID != first.ID || rotated.ExternalID == first.ExternalID {
		t.Fatalf("rotated table = %+v; previous = %+v", rotated, first)
	}
	decodedRotatedID, decodeErr := base64.RawURLEncoding.DecodeString(rotated.ExternalID)
	if decodeErr != nil || len(decodedRotatedID) != 32 {
		t.Fatalf("rotated external ID is not a 32-byte base64url value: %q, %v", rotated.ExternalID, decodeErr)
	}
	request("GET", firstPath, "", "", false, 404)
	firstPath = "/api/v1/tables/" + rotated.ExternalID
	settingsPath = firstPath + "/settings"
	association = firstPath + "/tracks/" + track.ID.String()
	stream = firstPath + "/stream/" + track.ID.String() + "/index.m3u8"
	request("GET", firstPath, "", "", false, 200)
	request("GET", stream, "", "", false, 200)
	request("DELETE", association, "", "", true, 204)
	assertUsage(0)
	request("GET", stream, "", "", false, 404)
	request("GET", stream, "", "", true, 200)
	request("PUT", association, "", "", true, 204)
	request("DELETE", firstPath, "", "", true, 204)
	request("GET", firstPath, "", "", true, 404)
	request("GET", "/api/v1/stream/"+track.ID.String()+"/index.m3u8", "", "", true, 200)
	request("DELETE", filePath+"?expectedTableCount=0", "", "", true, 200)
	request("GET", usagePath, "", "", true, 404)
	defaultTable, err := db.GetTable(ctx, server.DefaultTableID)
	if err != nil {
		t.Fatal(err)
	}
	request("DELETE", "/api/v1/tables/"+defaultTable.ExternalID, "", "", true, 204)
	request("GET", "/api/v1/joinToken", "", "", true, 404)
	request("DELETE", secondPath, "", "", true, 204)
	if rec := request("GET", "/api/v1/tables", "", "", true, 200); strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty tables must be an array: %s", rec.Body)
	}
}

type sentEvent struct {
	name    string
	payload any
}

type recordingWSRegisterer struct {
	testWSRegisterer
	sent []sentEvent
}

func (h *recordingWSRegisterer) SendEvent(name string, payload any) error {
	h.sent = append(h.sent, sentEvent{
		name:    name,
		payload: payload,
	})
	return h.err
}

func TestTagHTTP(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := sqlitedatastore.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrations, err := sqlitedatastore.NewMigration(os.DirFS("../../sql/migrations"), db)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authenticator, _, err := auth.New(context.Background(), auth.Config{
		TokenSecret:   "public-test-secret",
		TokenDuration: time.Hour,
		TokenIssuer:   "test",
		TokenAudience: "test",
	}, auth.BootstrapConfig{}, db, logger)
	if err != nil {
		t.Fatal(err)
	}
	gm, err := authenticator.NewToken("gm", auth.RoleGM)
	if err != nil {
		t.Fatal(err)
	}
	player, err := authenticator.NewToken("player", auth.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	videos := youtube.New(logger)
	hub := &recordingWSRegisterer{}
	processing := musicprocessing.New(dir, logger, db, videos, hub)
	srv, err := server.New(server.Config{UploadDir: dir, DevMode: true}, logger, authenticator, db, hub, playback.New(logger, db, hub), processing, videos)
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body string, token *auth.Token, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != nil {
			req.AddCookie(&http.Cookie{Name: "auth_token", Value: token.String()})
		}
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s = %d: %s; want %d", method, path, rec.Code, rec.Body, status)
		}
		return rec
	}
	decodeTag := func(rec *httptest.ResponseRecorder) domain.Tag {
		t.Helper()
		var tag domain.Tag
		if err := json.Unmarshal(rec.Body.Bytes(), &tag); err != nil {
			t.Fatal(err)
		}
		return tag
	}
	decodeTracks := func(rec *httptest.ResponseRecorder) []server.Track {
		t.Helper()
		var tracks []server.Track
		if err := json.Unmarshal(rec.Body.Bytes(), &tracks); err != nil {
			t.Fatal(err)
		}
		return tracks
	}
	lastEvent := func() sentEvent {
		t.Helper()
		if len(hub.sent) == 0 {
			t.Fatal("no event sent")
		}
		return hub.sent[len(hub.sent)-1]
	}
	types, err := db.GetTrackTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	track := server.Track{
		ID:        uuid.New(),
		Name:      "Battle",
		TypeID:    types[0].ID,
		CreatedAt: time.Now(),
	}
	if err := db.SaveTrack(ctx, &track); err != nil {
		t.Fatal(err)
	}

	request("GET", "/api/v1/tags", "", nil, 401)
	request("GET", "/api/v1/tags", "", player, 401)
	if rec := request("GET", "/api/v1/tags", "", gm, 200); strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty tags must be an array: %s", rec.Body)
	}
	for _, body := range []string{
		`{`,
		`{"name":"   ","color":"#FFFFFF"}`,
		`{"name":"` + strings.Repeat("x", 41) + `","color":"#FFFFFF"}`,
		`{"name":"Combat","color":"red"}`,
		`{"name":"Combat","color":"#FFF"}`,
	} {
		request("POST", "/api/v1/tags", body, gm, 400)
	}
	combat := decodeTag(request("POST", "/api/v1/tags", `{"name":" Combat ","color":"#e53935"}`, gm, 201))
	if combat.Name != "Combat" || combat.Color != "#E53935" {
		t.Fatalf("created tag = %+v", combat)
	}
	if event := lastEvent(); event.name != events.TagUpdated || event.payload != combat {
		t.Fatalf("create event = %+v", event)
	}
	request("POST", "/api/v1/tags", `{"name":"combat","color":"#000000"}`, gm, 409)
	calm := decodeTag(request("POST", "/api/v1/tags", `{"name":"Calm","color":"#43A047"}`, gm, 201))

	tagPath := "/api/v1/tags/" + calm.ID.String()
	request("PUT", tagPath, `{"name":"Peaceful","color":"#1E88E5"}`, player, 401)
	request("PUT", tagPath, `{"name":"COMBAT","color":"#1E88E5"}`, gm, 409)
	request("PUT", tagPath, `{"name":"","color":"#1E88E5"}`, gm, 400)
	request("PUT", "/api/v1/tags/invalid", `{"name":"Peaceful","color":"#1E88E5"}`, gm, 400)
	request("PUT", "/api/v1/tags/"+uuid.NewString(), `{"name":"Peaceful","color":"#1E88E5"}`, gm, 404)
	peaceful := decodeTag(request("PUT", tagPath, `{"name":"Peaceful","color":"#1E88E5"}`, gm, 200))
	if peaceful.ID != calm.ID || peaceful.Name != "Peaceful" || peaceful.Color != "#1E88E5" {
		t.Fatalf("updated tag = %+v", peaceful)
	}
	if event := lastEvent(); event.name != events.TagUpdated || event.payload != peaceful {
		t.Fatalf("update event = %+v", event)
	}

	editBody := func(fields string) string {
		return `{"trackIDs":["` + track.ID.String() + `"],` + fields + `}`
	}
	addCombat := editBody(`"addTagIDs":["` + combat.ID.String() + `"]`)
	request("PATCH", "/api/v1/files", addCombat, player, 401)
	for _, body := range []string{
		`{`,
		`{"trackIDs":[],"addTagIDs":["` + combat.ID.String() + `"]}`,
		editBody(`"addTagIDs":[]`),
		editBody(`"addTagIDs":["invalid"]`),
		editBody(`"addTagIDs":["` + combat.ID.String() + `"],"removeTagIDs":["` + combat.ID.String() + `"]`),
	} {
		request("PATCH", "/api/v1/files", body, gm, 400)
	}
	request("PATCH", "/api/v1/files", editBody(`"addTagIDs":["`+uuid.NewString()+`"]`), gm, 404)
	request("PATCH", "/api/v1/files", `{"trackIDs":["`+uuid.NewString()+`"],"addTagIDs":["`+combat.ID.String()+`"]}`, gm, 404)
	for range 2 {
		request("PATCH", "/api/v1/files", addCombat, gm, 200)
	}
	tagged := decodeTracks(request("PATCH", "/api/v1/files", editBody(`"typeID":"`+types[1].ID.String()+`","addTagIDs":["`+peaceful.ID.String()+`"]`), gm, 200))
	if len(tagged) != 1 || tagged[0].TypeID != types[1].ID || len(tagged[0].TagIDs) != 2 || tagged[0].TagIDs[0] != combat.ID || tagged[0].TagIDs[1] != peaceful.ID {
		t.Fatalf("tagged tracks = %+v", tagged)
	}
	if event := lastEvent(); event.name != events.TrackUpdated || event.payload != track.ID.String() {
		t.Fatalf("track edit event = %+v", event)
	}
	var library []server.Track
	if err := json.Unmarshal(request("GET", "/api/v1/files", "", gm, 200).Body.Bytes(), &library); err != nil || len(library) != 1 || len(library[0].TagIDs) != 2 {
		t.Fatalf("library = %+v, %v", library, err)
	}
	untagged := decodeTracks(request("PATCH", "/api/v1/files", editBody(`"removeTagIDs":["`+combat.ID.String()+`"]`), gm, 200))
	if len(untagged) != 1 || len(untagged[0].TagIDs) != 1 || untagged[0].TagIDs[0] != peaceful.ID {
		t.Fatalf("untagged tracks = %+v", untagged)
	}
	if event := lastEvent(); event.name != events.TagDeleted || event.payload != (events.TagReference{TagID: combat.ID.String()}) {
		t.Fatalf("unused tag event = %+v", event)
	}
	if tags, err := db.GetTags(ctx); err != nil || len(tags) != 1 || tags[0].ID != peaceful.ID {
		t.Fatalf("tags after removing last use = %+v, %v", tags, err)
	}
	request("DELETE", "/api/v1/files/"+track.ID.String(), "", gm, 200)
	if event := lastEvent(); event.name != events.TagDeleted || event.payload != (events.TagReference{TagID: peaceful.ID.String()}) {
		t.Fatalf("deleted track tag event = %+v", event)
	}
	if tags, err := db.GetTags(ctx); err != nil || len(tags) != 0 {
		t.Fatalf("tags after deleting last track = %+v, %v", tags, err)
	}
	if err := db.SaveTrack(ctx, &track); err != nil {
		t.Fatal(err)
	}

	hub.err = errors.New("hub closed")
	request("PATCH", "/api/v1/files", editBody(`"typeID":"`+types[0].ID.String()+`"`), gm, 503)
	request("POST", "/api/v1/tags", `{"name":"Stealth","color":"#8E24AA"}`, gm, 503)
}
