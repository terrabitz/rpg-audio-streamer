package main

import (
	"fmt"
	"os"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"golang.org/x/term"
)

func main() {
	fmt.Print("Enter password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read password: %v\n", err)
		os.Exit(1)
	}
	fmt.Println() // Add newline after password input

	hash, err := auth.HashPassword(string(password))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to hash password: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Hashed password:")
	fmt.Println(hash)
}
