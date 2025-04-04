package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	repoDir = filepath.Clean(repoDir) + string(filepath.Separator)

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
		InitHandler(repoDir)
	case "cat-file":
		CatFileHandler(repoDir)
	case "hash-object":
		HashObjectHandler(repoDir)
	case "ls-tree":
		LsTreeHandler(repoDir)
	case "write-tree":
		WriteTreeHandler(repoDir)
	case "write-working-tree":
		WriteWorkingTreeHandler(repoDir)
	case "commit-tree":
		CommitTreeHandler(repoDir)
	case "clone":
		CloneHandler()
	case "ls-files":
		LSFilesHandler(repoDir)
	case "add":
		AddHandler(repoDir)
	case "reset":
		ResetHandler(repoDir)
	case "status":
		StatusHandler(repoDir)
	case "commit":
		CommitHandler(repoDir)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		os.Exit(1)
	}
}
