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
	authenticator := auth.New(auth.Config{TokenSecret: "public-test-secret", JoinToken: "legacy-invite",
		TokenDuration: time.Hour, TokenIssuer: "test", TokenAudience: "test"}, logger)
	gm, err := authenticator.NewToken("gm", auth.RoleGM)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := server.New(server.Config{UploadDir: dir, DevMode: true}, logger, authenticator, db, websocket.NewHub(logger))
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body, invite string, asGM bool, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if asGM {
			req.AddCookie(&http.Cookie{Name: "auth_token", Value: gm.String()})
		}
		if invite != "" {
			req.Header.Set("Authorization", "Bearer "+invite)
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
	if first.Name != "Friday" || first.InviteToken == "" {
		t.Fatalf("created table = %+v", first)
	}
	second := decodeTable(request("POST", "/api/v1/tables", `{"name":"Sunday"}`, "", true, 201))
	firstPath := "/api/v1/tables/" + first.ID.String()
	secondPath := "/api/v1/tables/" + second.ID.String()
	renamed := decodeTable(request("PUT", firstPath, `{"name":"Friday group"}`, "", true, 200))
	if renamed.Name != "Friday group" || renamed.InviteToken != first.InviteToken {
		t.Fatalf("renamed table = %+v", renamed)
	}
	playerTable := decodeTable(request("GET", "/api/v1/tables/current", "", first.InviteToken, false, 200))
	if playerTable.ID != first.ID || playerTable.Name != renamed.Name || playerTable.InviteToken != "" {
		t.Fatalf("player table leaked invite or used wrong table: %+v", playerTable)
	}
	gmInvite := decodeTable(request("GET", "/api/v1/tables/current", "", second.InviteToken, true, 200))
	if gmInvite.ID != second.ID {
		t.Fatalf("GM opening invite resolved wrong table: %+v", gmInvite)
	}
	legacy := decodeTable(request("GET", "/api/v1/tables/current", "", "LEGACY-INVITE", false, 200))
	if legacy.ID != server.DefaultTableID {
		t.Fatalf("legacy invite = %+v", legacy)
	}
	request("GET", firstPath, "", first.InviteToken, false, 200)
	request("GET", secondPath, "", first.InviteToken, false, 403)
	request("PUT", firstPath, `{"name":"Forbidden"}`, first.InviteToken, false, 403)
	request("DELETE", firstPath, "", first.InviteToken, false, 403)
	request("GET", firstPath+"/tracks", "", first.InviteToken, false, 401)
	request("GET", "/api/v1/tables/invalid", "", "", true, 400)
	request("GET", "/api/v1/tables/"+uuid.NewString(), "", "", true, 404)
	request("PATCH", firstPath, "", "", true, 405)
	request("PATCH", "/api/v1/tables", "", "", true, 405)
	request("POST", "/api/v1/tables/current", "", "", true, 405)
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
	association := firstPath + "/tracks/" + track.ID.String()
	request("PUT", association, "", "", true, 204)
	request("PUT", association, "", "", true, 204)
	request("PUT", association, "", second.InviteToken, false, 401)
	request("PUT", firstPath+"/tracks/"+uuid.NewString(), "", "", true, 404)
	request("PUT", firstPath+"/tracks/invalid", "", "", true, 400)
	request("PATCH", association, "", "", true, 405)
	list := request("GET", firstPath+"/tracks", "", "", true, 200)
	var tracks []server.Track
	if err := json.Unmarshal(list.Body.Bytes(), &tracks); err != nil || len(tracks) != 1 || tracks[0].ID != track.ID {
		t.Fatalf("associated tracks = %v, %v", tracks, err)
	}
	stream := "/api/v1/stream/" + track.ID.String() + "/index.m3u8"
	request("GET", stream, "", first.InviteToken, false, 200)
	request("GET", stream+"?token="+first.InviteToken, "", "", false, 200)
	request("GET", stream, "", second.InviteToken, false, 404)
	request("GET", stream, "", "invalid-invite", false, 401)
	request("GET", "/api/v1/stream/invalid/index.m3u8", "", first.InviteToken, false, 400)
	request("DELETE", association, "", "", true, 204)
	request("GET", stream, "", first.InviteToken, false, 404)
	request("GET", stream, "", "", true, 200)
	request("PUT", association, "", "", true, 204)
	request("DELETE", firstPath, "", "", true, 204)
	request("GET", "/api/v1/tables/current", "", first.InviteToken, false, 401)
	request("GET", firstPath, "", "", true, 404)
	request("GET", stream, "", "", true, 200)
	request("DELETE", "/api/v1/files/"+track.ID.String(), "", "", true, 200)
	request("DELETE", "/api/v1/tables/"+server.DefaultTableID.String(), "", "", true, 204)
	request("GET", "/api/v1/tables/current", "", "legacy-invite", false, 401)
	request("GET", "/api/v1/tables/current", "", "", true, 404)
	request("GET", "/api/v1/joinToken", "", "", true, 404)
	request("DELETE", secondPath, "", "", true, 204)
	if rec := request("GET", "/api/v1/tables", "", "", true, 200); strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty tables must be an array: %s", rec.Body)
	}
}
