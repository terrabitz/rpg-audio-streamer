package server_test

import (
	"context"
	"encoding/json"
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
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore"
	"github.com/terrabitz/rpg-audio-streamer/internal/websocket"
)

func TestTagHTTP(t *testing.T) {
	dir := t.TempDir()
	db, err := sqlitedatastore.New(filepath.Join(dir, "tags.db"))
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
	authenticator := auth.New(auth.Config{TokenSecret: "public-test-secret", JoinToken: "test-invite",
		TokenDuration: time.Hour, TokenIssuer: "test", TokenAudience: "test"}, logger)
	gm, err := authenticator.NewToken("gm", auth.RoleGM)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := server.New(server.Config{UploadDir: dir, DevMode: true}, logger, authenticator, db, websocket.NewHub(logger))
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body, role string, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if role == "gm" {
			req.AddCookie(&http.Cookie{Name: "auth_token", Value: gm.String()})
		}
		if role == "player" {
			req.Header.Set("Authorization", "Bearer test-invite")
		}
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("%s %s = %d: %s; want %d", method, path, rec.Code, rec.Body, status)
		}
		return rec
	}
	for _, role := range []string{"", "player"} {
		for _, method := range []string{"GET", "POST"} {
			request(method, "/api/v1/tags", `{}`, role, 401)
		}
		for _, method := range []string{"PUT", "DELETE"} {
			request(method, "/api/v1/tags/"+uuid.NewString(), `{}`, role, 401)
		}
	}
	for _, body := range []string{`{`, `null`, `{}`, `{"name":" ","color":"#123456"}`,
		`{"name":"ok","color":"red"}`, `{"name":"` + strings.Repeat("a", 51) + `","color":"#123456"}`} {
		request("POST", "/api/v1/tags", body, "gm", 400)
	}
	var tag server.Tag
	rec := request("POST", "/api/v1/tags", `{"name":" Intense ","color":"#123456"}`, "gm", 201)
	if err := json.Unmarshal(rec.Body.Bytes(), &tag); err != nil {
		t.Fatal(err)
	}
	if tag.Name != "Intense" || tag.ID == uuid.Nil {
		t.Fatalf("created tag = %v", tag)
	}
	request("POST", "/api/v1/tags", `{"name":"INTENSE","color":"#abcdef"}`, "gm", 409)
	tagPath := "/api/v1/tags/" + tag.ID.String()
	request("PUT", tagPath, `{"name":"High intensity","color":"#abcdef"}`, "gm", 200)
	request("PUT", tagPath, `{"name":"Bad","color":"#zzz"}`, "gm", 400)
	request("PUT", "/api/v1/tags/"+uuid.NewString(), `{"name":"Missing","color":"#abcdef"}`, "gm", 404)
	request("DELETE", "/api/v1/tags/bad", "", "gm", 400)
	request("PATCH", tagPath, "", "gm", 405)
	request("DELETE", "/api/v1/tags", "", "gm", 405)
	types, err := db.GetTrackTypes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	track := server.Track{ID: uuid.New(), Name: "Battle", TypeID: types[0].ID,
		Path: "/battle", CreatedAt: time.Now().UTC().Truncate(time.Second)}
	if err := db.SaveTrackToTable(context.Background(), &track, server.DefaultTableID); err != nil {
		t.Fatal(err)
	}
	trackPath := "/api/v1/files/" + track.ID.String()
	request("PUT", trackPath, `{"tagIDs":["`+uuid.NewString()+`"],"name":"Unsaved"}`, "gm", 400)
	request("PUT", trackPath, `{"tagIDs":["`+tag.ID.String()+`"]}`, "gm", 200)
	var tracks []server.Track
	rec = request("GET", "/api/v1/tables/"+server.DefaultTableID.String()+"/tracks", "", "gm", 200)
	if err := json.Unmarshal(rec.Body.Bytes(), &tracks); err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 1 || tracks[0].Name != "Battle" || len(tracks[0].Tags) != 1 || tracks[0].Tags[0].Color != "#abcdef" {
		t.Fatalf("shared track tags = %v", tracks)
	}
	request("GET", "/api/v1/tags", "", "gm", 200)
	request("DELETE", tagPath, "", "gm", 204)
	request("DELETE", tagPath, "", "gm", 404)
	got, err := db.GetTrackByID(context.Background(), track.ID)
	if err != nil || got.Name != "Battle" || len(got.Tags) != 0 {
		t.Fatalf("tag deletion affected audio = %v, %v", got, err)
	}
}
