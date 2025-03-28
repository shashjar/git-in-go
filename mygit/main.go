package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

func configureLogger() {
	log.SetFlags(0)
}

func getRepoDir() string {
	repoDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to retrieve current working directory as repository: %s\n", err)
		os.Exit(1)
	}

	if !strings.HasSuffix(repoDir, "/") {
		repoDir = repoDir + "/"
	}

	return repoDir
}

// Usage: ./run.sh <command> [<args>...]
func main() {
	configureLogger()

	repoDir := getRepoDir()

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: ./run.sh <command> [<args>...]\n")
		os.Exit(1)
	}

	switch command := os.Args[1]; command {
	case "init":
		initHandler(repoDir)
	case "cat-file":
		catFileHandler(repoDir)
	case "hash-object":
		hashObjectHandler(repoDir)
	case "ls-tree":
		lsTreeHandler(repoDir)
	case "write-tree":
		writeTreeHandler(repoDir)
	case "commit-tree":
		commitTreeHandler(repoDir)
	case "clone":
		cloneHandler()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		os.Exit(1)
	}
}
