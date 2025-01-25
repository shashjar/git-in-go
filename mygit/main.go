package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const REPO_DIR = "repo/"

func configureLogger() {
	log.SetFlags(0)
}

// Usage: ./run.sh <command> [<args>...]
func main() {
	configureLogger()

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: ./run.sh <command> [<args>...]\n")
		os.Exit(1)
	}

	switch command := os.Args[1]; command {
	case "init":
		for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
			if err := os.MkdirAll(REPO_DIR+dir, 0755); err != nil {
				log.Fatalf("Error creating directory: %s\n", err)
			}
		}

		headFileContents := []byte("ref: refs/heads/main\n")
		if err := os.WriteFile(REPO_DIR+".git/HEAD", headFileContents, 0644); err != nil {
			log.Fatalf("Error writing HEAD file: %s\n", err)
		}

		absPath, err := filepath.Abs(REPO_DIR + ".git")
		if err != nil {
			log.Fatalf("Error getting absolute path of Git repository: %s\n", err)
		}
		log.Printf("Initialized empty Git repository in %s\n", absPath)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		os.Exit(1)
	}
}
