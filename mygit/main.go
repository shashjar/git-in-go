package main

import (
	"fmt"
	"log"
	"os"
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
		initHandler()
	case "cat-file":
		catFileHandler()
	case "hash-object":
		hashObjectHandler()
	case "ls-tree":
		lsTreeHandler()
	case "write-tree":
		writeTreeHandler()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		os.Exit(1)
	}
}
