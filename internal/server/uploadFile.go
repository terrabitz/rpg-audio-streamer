package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20) // 10MB max
	if err != nil {
		s.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("files")
	if err != nil {
		s.logger.Error("failed to get file", "error", err)
		http.Error(w, "Failed to retrieve file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Retrieve additional metadata
	name := r.FormValue("name")
	typeIDStr := r.FormValue("typeId")

	typeID, err := uuid.Parse(typeIDStr)
	if err != nil {
		s.logger.Error("invalid type ID", "error", err)
		http.Error(w, "Invalid track type ID", http.StatusBadRequest)
		return
	}

	// Validate track type exists
	if _, err := s.store.GetTrackTypeByID(r.Context(), typeID); err != nil {
		s.logger.Error("track type not found", "error", err)
		http.Error(w, "Invalid track type", http.StatusBadRequest)
		return
	}

	id, err := uuid.NewV7()
	if err != nil {
		s.logger.Error("failed to generate UUID", "error", err)
		http.Error(w, "Failed to save track information", http.StatusInternalServerError)
	}

	tempDir := os.TempDir()
	dstPath := filepath.Join(tempDir, id.String())
	dstFile, err := os.Create(dstPath)
	if err != nil {
		s.logger.Error("failed to create file", "error", err, "path", dstPath)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, file); err != nil {
		s.logger.Error("failed to write file", "error", err, "path", dstPath)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	hlsDir := filepath.Join(s.cfg.UploadDir, id.String())
	if err := os.MkdirAll(hlsDir, os.ModePerm); err != nil {
		s.logger.Error("failed to create HLS directory", "error", err, "path", hlsDir)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	cmd := exec.Command("ffmpeg",
		"-i", dstPath,
		"-v", "verbose",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ac", "2",
		"-ar", "44100",
		"-hls_time", "6",
		"-hls_playlist_type", "event",
		"-hls_segment_filename", hlsDir+"/segment_%03d.ts",
		"-vn",
		"-f", "hls",
		filepath.Join(hlsDir, "index.m3u8"))
	s.logger.Info("executing ffmpeg command", "command", cmd.String())
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		s.logger.Error("failed to convert file to HLS", "error", err, "path", dstPath)
		fmt.Println(out.String())
		fmt.Println(stderr.String())
		http.Error(w, "Failed to convert file", http.StatusInternalServerError)
		return
	}

	fmt.Println(out.String())
	fmt.Println(stderr.String())

	if err := os.Remove(dstPath); err != nil {
		s.logger.Warn("failed to remove original file", "error", err, "path", dstPath)
	}

	// Save track information to the datastore
	track := Track{
		ID:        id,
		CreatedAt: time.Now(),
		Name:      name,
		Path:      hlsDir,
		TypeID:    typeID,
	}

	if err := s.store.SaveTrack(r.Context(), &track); err != nil {
		s.logger.Error("failed to save track information", "error", err)
		http.Error(w, "Failed to save track information", http.StatusInternalServerError)
		return
	}

	s.logger.Info("file uploaded and converted to HLS", "filename", handler.Filename)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("File uploaded and converted to HLS successfully"))
}
