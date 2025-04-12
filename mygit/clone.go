package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
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

	refsPktLines, err := refDiscovery(repoURL, username, token)
	if err != nil {
		log.Fatalf("Failed to perform reference discovery on the remote repository: %s\n", err)
	}

	packfile, headHash, err := uploadPackRequest(repoURL, refsPktLines, username, token)
	if err != nil {
		log.Fatalf("Failed to perform git-upload-pack request: %s\n", err)
	}

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

func refDiscovery(repoURL string, username string, token string) ([]string, error) {
	refDiscoveryRespBody, err := makeHTTPRequest("GET", repoURL+"/info/refs?service=git-upload-pack", username, token, bytes.Buffer{}, []int{200, 304})
	if err != nil {
		return nil, fmt.Errorf("ref discovery request failed: %s", err)
	}

	validFirstBytes := regexp.MustCompile(`^[0-9a-f]{4}#`).MatchString(string(refDiscoveryRespBody[:5]))
	if !validFirstBytes {
		return []string{}, fmt.Errorf("received invalid response when fetching refs from remote repository")
	}

	refsPktLines, err := readPktLines(bytes.NewReader(refDiscoveryRespBody))
	if err != nil {
		return []string{}, fmt.Errorf("failed to parse response when fetching refs from remote repository: %s", err)
	}

	if len(refsPktLines) == 0 || refsPktLines[0] != "# service=git-upload-pack" {
		return []string{}, fmt.Errorf("received invalid response when fetching refs from remote repository")
	}

	return refsPktLines, nil
}

func uploadPackRequest(repoURL string, refsPktLines []string, username string, token string) ([]byte, string, error) {
	var headHash string
	for _, refPktLine := range refsPktLines {
		if len(refPktLine) > 45 && refPktLine[41:45] == "HEAD" {
			headHash = refPktLine[0:40]
		}
	}
	if headHash == "" {
		return nil, "", fmt.Errorf("refs in remote repository do not contain SHA hash for HEAD")
	}
	if !isValidObjectHash(headHash) {
		return nil, "", fmt.Errorf("refs in remote repository contained invalid SHA hash for HEAD: %s", headHash)
	}

	capabilities := "multi_ack ofs-delta thin-pack include-tag"
	uploadPackPktLine := createPktLine(fmt.Sprintf("want %s %s", headHash, capabilities))
	donePktLine := createPktLine("done")
	uploadPackRequestBody := createPktLineStream([]string{uploadPackPktLine}) + donePktLine

	var uploadPackReqBody bytes.Buffer
	uploadPackReqBody.WriteString(uploadPackRequestBody)
	uploadPackRespBody, err := makeHTTPRequest("POST", repoURL+"/git-upload-pack", username, token, uploadPackReqBody, []int{200})
	if err != nil {
		return nil, "", fmt.Errorf("git-upload-pack request failed: %s", err)
	}

	nakLine, err := readPktLine(bytes.NewReader(uploadPackRespBody))
	if err != nil || nakLine != "NAK" {
		return nil, "", fmt.Errorf("expected NAK in git-upload-pack response")
	}

	return uploadPackRespBody[8:], headHash, nil
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
