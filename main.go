package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
)

//go:embed ui/dist
var frontend embed.FS

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
	http.Handle("/", frontendFS)

	fmt.Printf("Listening on port %d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("error while running server: %w", err)
	}

	return nil
}
