package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func configureLogger() {
	log.SetFlags(0)
}

func initEnvironmentVariables() {
	if err := godotenv.Load(".env"); err == nil {
		return
	}

	if err := godotenv.Load("../.env"); err == nil {
		return
	}

	log.Fatal("Error: no .env file found. Please create one with GIT_USERNAME and GIT_PASSWORD set in either the current directory or parent directory.")
}

func copyRunSh(repoDir string) error {
	if *CopyRunSh {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %s", err)
		}

		srcPath := filepath.Join(cwd, "run.sh")
		srcContent, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("failed to read run.sh from current directory: %s", err)
		}

		dstPath := filepath.Join(repoDir, "run.sh")
		err = os.WriteFile(dstPath, srcContent, 0755)
		if err != nil {
			return fmt.Errorf("failed to write run.sh to repository directory: %s", err)
		}

		err = os.Chmod(dstPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to make run.sh executable: %s", err)
		}
	}

	return nil
}

func getRepoDir() string {
	repoDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Unable to retrieve current working directory as repository: %s\n", err)
	}

	repoDir = filepath.Clean(repoDir) + string(filepath.Separator)

	return repoDir
}

// Usage: ./run.sh <command> [<args>...]
func main() {
	configureLogger()
	initEnvironmentVariables()
	flag.Parse()

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
