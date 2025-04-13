package main

import (
	"fmt"
	"log"
	"os"
)

func CloneRepo(repoURL string, repoDir string) {
	info, err := os.Stat(repoDir)
	if !os.IsNotExist(err) && info.IsDir() {
		log.Fatalf("Destination path '%s' already exists", repoDir)
	}

	err = os.MkdirAll(repoDir, 0755)
	if err != nil {
		log.Fatalf("Failed to create repository directory: %s\n", err)
	}

	fmt.Printf("Cloning into '%s'...\n", repoDir)

	_, err = initRepo(repoDir)
	if err != nil {
		log.Fatalf("Failed to initialize repository: %s\n", err)
	}

	refsMap, err := refDiscovery(repoURL)
	if err != nil {
		log.Fatalf("Failed to perform reference discovery on the remote repository: %s\n", err)
	}

	packfile, err := uploadPackRequest(repoURL, refsMap)
	if err != nil {
		log.Fatalf("Failed to perform git-upload-pack request: %s\n", err)
	}

	headHash, ok := refsMap["HEAD"]
	if !ok {
		log.Fatalf("No HEAD reference found in remote repository")
	}

	err = ReadPackfile(packfile, repoDir)
	if err != nil {
		log.Fatalf("Failed to read packfile: %s\n", err)
	}

	err = CheckoutCommit(headHash, repoDir)
	if err != nil {
		log.Fatalf("Failed to check out HEAD commit: %s\n", err)
	}

	err = copyRunSh(repoDir)
	if err != nil {
		log.Fatalf("Failed to copy mygit run.sh script into cloned repository: %s\n", err)
	}

	err = updateRefsAfterPull(refsMap, repoDir)
	if err != nil {
		log.Fatalf("Failed to create refs: %s\n", err)
	}
}
