package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/terrabitz/rpg-audio-streamer/internal/types"
)

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(s.cfg.UploadDir)
	if err != nil {
		s.logger.Error("failed to read directory", "error", err)
		http.Error(w, "Failed to list files", http.StatusInternalServerError)
		return
	}

	var fileList []types.FileInfo
	for _, file := range files {
		hlsPath := filepath.Join(s.cfg.UploadDir, file.Name(), "index.m3u8")
		if _, err := os.Stat(hlsPath); err == nil {
			fileList = append(fileList, types.FileInfo{
				Name: file.Name(),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(fileList); err != nil {
		s.logger.Error("failed to encode response", "error", err)
	}
}
