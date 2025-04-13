package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	username := os.Getenv("GIT_USERNAME")
	token := os.Getenv("GIT_TOKEN")

	refsMap, err := refDiscovery(repoURL, username, token)
	if err != nil {
		log.Fatalf("Failed to perform reference discovery on the remote repository: %s\n", err)
	}

	packfile, err := uploadPackRequest(repoURL, refsMap, []string{"HEAD"}, username, token)
	if err != nil {
		log.Fatalf("Failed to perform git-upload-pack request: %s\n", err)
	}

	headHash := refsMap["HEAD"]

	err = ReadPackfile(packfile, repoDir)
	if err != nil {
		log.Fatalf("Failed to read packfile: %s\n", err)
	}

	err = createRefs(headHash, repoDir)
	if err != nil {
		log.Fatalf("Failed to create refs: %s\n", err)
	}

	err = CheckoutCommit(headHash, repoDir)
	if err != nil {
		log.Fatalf("Failed to check out HEAD commit: %s\n", err)
	}

	err = copyRunSh(repoDir)
	if err != nil {
		log.Fatalf("Failed to copy mygit run.sh script into cloned repository: %s\n", err)
	}
}

func createRefs(headHash string, repoDir string) error {
	masterRefPathLocal := filepath.Join(repoDir, ".git", "refs", "heads", "master")
	err := os.WriteFile(masterRefPathLocal, []byte(headHash+"\n"), 0644)
	if err != nil {
		return fmt.Errorf("failed to write master branch local reference: %s", err)
	}

	masterRefPathRemote := filepath.Join(repoDir, ".git", "refs", "remotes", "origin", "master")
	err = os.WriteFile(masterRefPathRemote, []byte(headHash+"\n"), 0644)
	if err != nil {
		return fmt.Errorf("failed to write master branch remote reference: %s", err)
	}

	return nil
}
