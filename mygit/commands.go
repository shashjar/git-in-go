package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func initHandler() {
	for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
		if err := os.MkdirAll(REPO_DIR+dir, 0755); err != nil {
			log.Fatalf("Error creating directory: %s\n", err)
		}
	}

	headFileContents := []byte("ref: refs/heads/master\n")
	if err := os.WriteFile(REPO_DIR+".git/HEAD", headFileContents, 0644); err != nil {
		log.Fatalf("Error writing HEAD file: %s\n", err)
	}

	absPath, err := filepath.Abs(REPO_DIR + ".git")
	if err != nil {
		log.Fatalf("Error getting absolute path of Git repository: %s\n", err)
	}
	log.Printf("Initialized empty Git repository in %s\n", absPath)
}

func catFileHandler() {
	if len(os.Args) != 4 || os.Args[2] != "-p" {
		log.Fatal("Usage: cat-file -p <blob_sha>")
	}

	objHash := os.Args[3]
	if !isValidObjectHash(objHash) {
		log.Fatalf("Invalid object hash: %s\n", objHash)
	}

	blobObj, err := readBlobObjectFile(objHash)
	if err != nil {
		log.Fatalf("Could not read object file: %s\n", err)
	}

	fmt.Printf(blobObj.content)
}

func hashObjectHandler() {
	if len(os.Args) != 4 || os.Args[2] != "-w" {
		log.Fatal("Usage: hash-object -w <file>")
	}

	filePath := os.Args[3]
	blobObj, err := createBlobObjectFromFile(REPO_DIR + filePath)
	if err != nil {
		log.Fatalf("Could not create blob object from file: %s\n", err)
	}

	fmt.Println(blobObj.hash)
}

func lsTreeHandler() {
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

	treeObj, err := readTreeObjectFile(treeHash)
	if err != nil {
		log.Fatalf("Could not read tree object file: %s\n", err)
	}

	for _, entry := range treeObj.entries {
		entryString := entry.toString(nameOnly)
		fmt.Println(entryString)
	}
}

func writeTreeHandler() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: write-tree")
	}

	treeObj, err := createTreeObjectFromDirectory(REPO_DIR)
	if err != nil {
		log.Fatalf("Could not create tree object for directory: %s\n", err)
	}

	fmt.Println(treeObj.hash)
}
