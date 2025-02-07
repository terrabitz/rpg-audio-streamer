package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var uploadDir = "./uploads"

//go:embed ui/dist
var frontend embed.FS

type FileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func main() {
	if err := run(); err != nil {
		log.Printf("error: %s", err)
		os.Exit(1)
	}
}

func run() error {
	stripped, err := fs.Sub(frontend, "ui/dist")
	if err != nil {
		log.Fatalln(err)
	}

	var port int
	flag.IntVar(&port, "port", 8080, "The port to listen on")
	flag.Parse()

	frontendFS := http.FileServer(http.FS(stripped))

	mux := http.NewServeMux()
	mux.Handle("/", frontendFS)
	mux.HandleFunc("/upload", uploadFile)
	mux.HandleFunc("/files", listFiles)
	mux.HandleFunc("/stream/{fileName}", streamFile)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	fmt.Printf("Listening on port %d\n", port)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("error while running server: %w", err)
	}

	return nil
}

func uploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(10 << 20) // Limit upload size to 10MB
	if err != nil {
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("files")
	if err != nil {
		http.Error(w, "Failed to retrieve file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	err = os.MkdirAll(uploadDir, os.ModePerm)
	if err != nil {
		http.Error(w, "Failed to create upload directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	dstFile, err := os.Create(filepath.Join(uploadDir, handler.Filename))
	if err != nil {
		http.Error(w, "Failed to save file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, file)
	if err != nil {
		http.Error(w, "Failed to write file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("File uploaded successfully"))
}

func listFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	files, err := os.ReadDir(uploadDir)
	if err != nil {
		http.Error(w, "Failed to read directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var fileList []FileInfo
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			continue
		}
		fileList = append(fileList, FileInfo{
			Name: info.Name(),
			Size: info.Size(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileList)
}

func streamFile(w http.ResponseWriter, r *http.Request) {
	fileName := r.PathValue("fileName")

	filePath := filepath.Join(uploadDir, fileName)

	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "Failed to open file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "audio/mp3")
	http.ServeContent(w, r, fileName, time.Time{}, file)
}
