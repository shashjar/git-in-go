package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func initHandler(repoDir string) {
	absPath, err := initRepo(repoDir)
	if err != nil {
		log.Fatalf("Error initializing Git repository: %s\n", err)
	}
	log.Printf("Initialized empty Git repository in %s\n", absPath)
}

func catFileHandler(repoDir string) {
	flag := os.Args[2]
	if len(os.Args) != 4 || (flag != "-t" && flag != "-s" && flag != "-p") {
		log.Fatal("Usage: cat-file (-t | -s | -p) <object_sha>")
	}

	objHash := os.Args[3]
	if !isValidObjectHash(objHash) {
		log.Fatalf("Invalid object hash: %s\n", objHash)
	}

	obj, err := getObject(objHash, repoDir)
	if err != nil {
		log.Fatalf("Could not read object file: %s\n", err)
	}

	switch flag {
	case "-t":
		t := obj.GetObjectType()
		fmt.Println(t.toString())
	case "-s":
		s := obj.GetSizeBytes()
		fmt.Println(s)
	case "-p":
		p := obj.PrettyPrint()
		fmt.Print(p)
	}
}

func hashObjectHandler(repoDir string) {
	if len(os.Args) != 4 || os.Args[2] != "-w" {
		log.Fatal("Usage: hash-object -w <file>")
	}

	filePath := os.Args[3]
	blobObj, err := createBlobObjectFromFile(repoDir+filePath, repoDir)
	if err != nil {
		log.Fatalf("Could not create blob object from file: %s\n", err)
	}

	fmt.Println(blobObj.hash)
}

func lsTreeHandler(repoDir string) {
	var nameOnly bool
	if len(os.Args) == 3 {
		nameOnly = false
	} else if len(os.Args) == 4 && os.Args[2] == "--name-only" {
		nameOnly = true
	} else {
		log.Fatal("Usage: ls-tree [--name-only] <tree_sha>")
	}

	treeHash := os.Args[len(os.Args)-1]
	if !isValidObjectHash(treeHash) {
		log.Fatalf("Invalid object hash: %s\n", treeHash)
	}

	treeObj, err := readTreeObjectFile(treeHash, repoDir)
	if err != nil {
		log.Fatalf("Could not read tree object file: %s\n", err)
	}

	for _, entry := range treeObj.entries {
		entryString := entry.toString(nameOnly)
		fmt.Println(entryString)
	}
}

func writeTreeHandler(repoDir string) {
	if len(os.Args) != 2 {
		log.Fatal("Usage: write-tree")
	}

	treeObj, err := createTreeObjectFromDirectory(repoDir, repoDir)
	if err != nil {
		log.Fatalf("Could not create tree object for directory: %s\n", err)
	}

	fmt.Println(treeObj.hash)
}

func commitTreeHandler(repoDir string) {
	if len(os.Args) < 3 || len(os.Args) > 7 {
		log.Fatal("Usage: commit-tree <tree_sha> [-p <parent_commit_sha>] [-m <commit_message>]")
	}

	treeHash := os.Args[2]
	if !isValidObjectHash(treeHash) {
		log.Fatalf("Invalid object hash: %s\n", treeHash)
	}

	os.Args = append(os.Args[0:1], os.Args[3:]...)
	parentCommitHashPtr := flag.String("p", "", "Parent commit")
	commitMessagePtr := flag.String("m", "Made a commit!", "Commit message")
	flag.Parse()

	if *parentCommitHashPtr != "" && !isValidObjectHash(*parentCommitHashPtr) {
		log.Fatalf("Invalid parent commit hash: %s\n", *parentCommitHashPtr)
	}

	var parentCommitHashes []string
	if *parentCommitHashPtr != "" {
		parentCommitHashes = append(parentCommitHashes, *parentCommitHashPtr)
	}

	commitObj, err := createCommitObjectFromTree(treeHash, parentCommitHashes, *commitMessagePtr, repoDir)
	if err != nil {
		log.Fatalf("Could not create commit object from tree: %s\n", err)
	}

	fmt.Println(commitObj.hash)
}

func cloneHandler() {
	if len(os.Args) != 3 && len(os.Args) != 4 {
		log.Fatal("Usage: clone <repo_url> [some_dir]")
	}

	repoURL := os.Args[2]
	err := validateRepoURL(repoURL)
	if err != nil {
		log.Fatalf("Failed to validate structure of repository URL: %s\n", err)
	}

	var repoDir string
	if len(os.Args) == 4 {
		repoDir = os.Args[3]
	} else {
		repoURLParts := strings.Split(repoURL, "/")
		repoDir = repoURLParts[len(repoURLParts)-1]
	}
	repoDir = filepath.Clean(repoDir) + string(filepath.Separator)

	cloneRepo(repoURL, repoDir)
}

func lsFilesHandler(repoDir string) {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		log.Fatal("Usage: ls-files [-s]")
	}

	os.Args = append(os.Args[0:1], os.Args[2:]...)
	showDetailsPtr := flag.Bool("s", false, "Show entries' mode bits and object hash in the output")
	flag.Parse()

	entries, err := readIndex(repoDir)
	if err != nil {
		log.Fatalf("Failed to read entries within Git index file: %s\n", err)
	}

	for _, entry := range entries {
		if *showDetailsPtr {
			fmt.Printf("%06d %s %s\n", entry.mode, hex.EncodeToString(entry.sha1[:]), entry.path)
		} else {
			fmt.Println(entry.path)
		}
	}
}

func addHandler(repoDir string) {
	if len(os.Args) < 3 {
		log.Fatal("Usage: `add <file> <file> ...` or `add .`")
	}

	addAll := len(os.Args) == 3 && os.Args[2] == "."

	var filesToAdd []string
	if addAll {
		err := filepath.WalkDir(repoDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(repoDir, path)
			if err != nil {
				return err
			}

			if relPath == "." || d.IsDir() || strings.HasPrefix(relPath, ".git") {
				return nil
			}

			filesToAdd = append(filesToAdd, relPath)
			return nil
		})

		if err != nil {
			log.Fatalf("Failed to scan repository files: %s\n", err)
		}
	} else {
		for _, file := range os.Args[2:] {
			if _, err := os.Stat(filepath.Join(repoDir, file)); err != nil {
				log.Fatalf("File does not exist: %s\n", file)
			}

			filesToAdd = append(filesToAdd, file)
		}
	}

	err := addFilesToIndex(filesToAdd, repoDir)
	if err != nil {
		log.Fatalf("Failed to add files to index: %s\n", err)
	}
}

func resetHandler(repoDir string) {
	if len(os.Args) < 3 {
		log.Fatal("Usage: reset <file> <file> ...")
	}

	var filesToRemove []string
	for _, file := range os.Args[2:] {
		if _, err := os.Stat(filepath.Join(repoDir, file)); err != nil {
			log.Fatalf("File does not exist: %s\n", file)
		}

		filesToRemove = append(filesToRemove, file)
	}

	err := removeFilesFromIndex(filesToRemove, repoDir)
	if err != nil {
		log.Fatalf("Failed to remove files from index: %s\n", err)
	}
}
