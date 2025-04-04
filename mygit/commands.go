package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	COLOR_RESET = "\033[0m"
	COLOR_RED   = "\033[31m"
	COLOR_GREEN = "\033[32m"
)

// Initializes the given directory as a Git repository by creating the .git directory and
// any necessary Git metadata.
func InitHandler(repoDir string) {
	if len(os.Args) != 2 {
		log.Fatal("Usage: init")
	}

	absPath, err := initRepo(repoDir)
	if err != nil {
		log.Fatalf("Error initializing Git repository: %s\n", err)
	}
	fmt.Printf("Initialized empty Git repository in %s\n", absPath)
}

// Prints the information associated with the given object, identified by hash.
// -t --> Prints the type of the object.
// -s --> Prints the size in bytes of the object's content.
// -p --> Pretty-prints the object file, including header and content.
func CatFileHandler(repoDir string) {
	flag := os.Args[2]
	if len(os.Args) != 4 || (flag != "-t" && flag != "-s" && flag != "-p") {
		log.Fatal("Usage: cat-file (-t | -s | -p) <object_sha>")
	}

	objHash := os.Args[3]
	if !isValidObjectHash(objHash) {
		log.Fatalf("Invalid object hash: %s\n", objHash)
	}

	obj, err := GetObject(objHash, repoDir)
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

// Creates a Git blob object for the repository file provided and prints the resulting object hash.
// Must be executed with the -w flag for actually writing the object into the object database.
func HashObjectHandler(repoDir string) {
	if len(os.Args) != 4 || os.Args[2] != "-w" {
		log.Fatal("Usage: hash-object -w <file>")
	}

	filePath := os.Args[3]
	blobObj, err := CreateBlobObjectFromFile(repoDir+filePath, repoDir)
	if err != nil {
		log.Fatalf("Could not create blob object from file: %s\n", err)
	}

	fmt.Println(blobObj.hash)
}

// Prints information on the entries in the given tree object, identified by hash.
// --name-only --> Prints only the names of the entries in the given tree object.
func LsTreeHandler(repoDir string) {
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

	treeObj, err := ReadTreeObjectFile(treeHash, repoDir)
	if err != nil {
		log.Fatalf("Could not read tree object file: %s\n", err)
	}

	for _, entry := range treeObj.entries {
		entryString := entry.toString(nameOnly)
		fmt.Println(entryString)
	}
}

// Creates a new Git tree object from the current Git index file. Prints the hash of the resulting tree object.
func WriteTreeHandler(repoDir string) {
	if len(os.Args) != 2 {
		log.Fatal("Usage: write-tree")
	}

	treeObj, err := CreateTreeObjectFromIndex(repoDir)
	if err != nil {
		log.Fatalf("Could not create tree object from Git index: %s\n", err)
	}

	fmt.Println(treeObj.hash)
}

// Creates a new Git tree object for the working tree of the given directory. Prints the hash of the resulting tree object.
func WriteWorkingTreeHandler(repoDir string) {
	if len(os.Args) != 2 {
		log.Fatal("Usage: write-working-tree")
	}

	treeObj, err := CreateTreeObjectFromDirectory(repoDir, repoDir)
	if err != nil {
		log.Fatalf("Could not create tree object from working tree directory: %s\n", err)
	}

	fmt.Println(treeObj.hash)
}

// Creates a new Git commit object from the tree object provided, identified by hash. Prints the
// hash of the resulting commit object.
// -p --> Identifies an optional parent commit hash for the new commit.
// -m --> Identifies an optional message for the new commit.
func CommitTreeHandler(repoDir string) {
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

	commitObj, err := CreateCommitObjectFromTree(treeHash, parentCommitHashes, *commitMessagePtr, repoDir)
	if err != nil {
		log.Fatalf("Could not create commit object from tree: %s\n", err)
	}

	fmt.Println(commitObj.hash)
}

// Clones the Git repository at the given URL into some local directory. The directory to clone into may be
// specified by the user. If not specified, it will default to the basename of the remote repository.
func CloneHandler() {
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

	CloneRepo(repoURL, repoDir)
}

// Prints information about the entries (representing repository files) in the Git index file. By default,
// prints only the filepath of each entry.
// -s --> Prints the mode and object hash for each entry, in addition to the path.
func LSFilesHandler(repoDir string) {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		log.Fatal("Usage: ls-files [-s]")
	}

	os.Args = append(os.Args[0:1], os.Args[2:]...)
	showDetailsPtr := flag.Bool("s", false, "Show entries' mode bits and object hash in the output")
	flag.Parse()

	entries, err := ReadIndex(repoDir)
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

// Adds the list of provided files (identified by relative paths from the repository root) to the Git index.
// If executed with ., adds all files in the repository to the Git index.
func AddHandler(repoDir string) {
	if len(os.Args) < 3 {
		log.Fatal("Usage: `add <file> <file> ...` or `add .`")
	}

	addAll := len(os.Args) == 3 && os.Args[2] == "."

	var filesToAdd []string
	if addAll {
		var err error
		filesToAdd, err = getWorkingTreeFilePaths(repoDir)
		if err != nil {
			log.Fatalf("Failed to scan repository for all files in working tree: %s\n", err)
		}
	} else {
		for _, file := range os.Args[2:] {
			if _, err := os.Stat(filepath.Join(repoDir, file)); err != nil {
				log.Fatalf("File does not exist: %s\n", file)
			}

			filesToAdd = append(filesToAdd, file)
		}
	}

	err := AddFilesToIndex(filesToAdd, repoDir)
	if err != nil {
		log.Fatalf("Failed to add files to index: %s\n", err)
	}
}

// Removes the list of provided files (identified by relative paths from the repository root)
// from the Git index.
func ResetHandler(repoDir string) {
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

	err := RemoveFilesFromIndex(filesToRemove, repoDir)
	if err != nil {
		log.Fatalf("Failed to remove files from index: %s\n", err)
	}
}

// Shows the status of the working tree to the user, including modified, deleted, and created/untracked files.
func StatusHandler(repoDir string) {
	if len(os.Args) != 2 {
		log.Fatal("Usage: status")
	}

	status, err := GetRepoStatus(repoDir)
	if err != nil {
		log.Fatalf("Failed to determine status of repository: %s\n", err)
	}

	hasChanges := len(status.stagedFiles) > 0 || len(status.notStagedFiles) > 0 || len(status.untrackedFiles) > 0

	fmt.Printf("On branch %s\n", status.branch)

	if !hasChanges {
		fmt.Printf("Your branch is up to date with 'origin/%s'.\n", status.branch)
		fmt.Println("\nnothing to commit, working tree clean")
		return
	}

	// Print staged changes
	if len(status.stagedFiles) > 0 {
		fmt.Println("\nChanges to be committed:")
		fmt.Println("  (use \"git reset <file>...\" to unstage)")

		for _, fs := range status.stagedFiles {
			var statusStr string
			switch fs.status {
			case ModifiedStaged:
				statusStr = "modified:"
			case AddedStaged:
				statusStr = "new file:"
			case DeletedStaged:
				statusStr = "deleted:"
			default:
				log.Fatalf("Unexpected status for staged file %s: %d\n", fs.path, fs.status)
			}
			fmt.Printf("\t%s%s\t%s%s\n", COLOR_GREEN, statusStr, fs.path, COLOR_RESET)
		}
	}

	// Print not staged changes
	if len(status.notStagedFiles) > 0 {
		fmt.Println("\nChanges not staged for commit:")
		fmt.Println("  (use \"git add/reset <file>...\" to update what will be committed)")

		for _, fs := range status.notStagedFiles {
			var statusStr string
			switch fs.status {
			case ModifiedNotStaged:
				statusStr = "modified:"
			case DeletedNotStaged:
				statusStr = "deleted:"
			default:
				log.Fatalf("Unexpected status for unstaged file %s: %d\n", fs.path, fs.status)
			}
			fmt.Printf("\t%s%s\t%s%s\n", COLOR_RED, statusStr, fs.path, COLOR_RESET)
		}
	}

	// Print untracked files
	if len(status.untrackedFiles) > 0 {
		fmt.Println("\nUntracked files:")
		fmt.Println("  (use \"git add <file>...\" to include in what will be committed)")

		for _, fs := range status.untrackedFiles {
			fmt.Printf("\t%s%s%s\n", COLOR_RED, fs.path, COLOR_RESET)
		}
	}

	if len(status.stagedFiles) == 0 {
		fmt.Println("\nno changes added to commit (use \"git add\" and/or \"git commit -a\")")
	}
}

// Creates a new Git commit from the current contents of the index and with the optional commit message specified.
// -m --> Identifies an optional message for the new commit.
func CommitHandler(repoDir string) {
	if len(os.Args) < 2 || len(os.Args) > 4 {
		log.Fatal("Usage: commit [-m <commit_message>]")
	}

	os.Args = append(os.Args[0:1], os.Args[2:]...)
	commitMessagePtr := flag.String("m", "Made a commit!", "Commit message")
	flag.Parse()

	headCommitHash, commitsExist, err := ResolveRef("HEAD", repoDir)
	if err != nil {
		log.Fatalf("Failed to resolve HEAD reference: %s\n", err)
	}

	parentCommitHashes := []string{}
	if commitsExist {
		parentCommitHashes = append(parentCommitHashes, headCommitHash)
	}

	treeObj, err := CreateTreeObjectFromIndex(repoDir)
	if err != nil {
		log.Fatalf("Could not create tree object from Git index: %s\n", err)
	}

	commitObj, err := CreateCommitObjectFromTree(treeObj.hash, parentCommitHashes, *commitMessagePtr, repoDir)
	if err != nil {
		log.Fatalf("Could not create commit object from tree: %s\n", err)
	}

	err = UpdateRef("HEAD", commitObj.hash, repoDir)
	if err != nil {
		log.Fatalf("Failed to update HEAD reference: %s\n", err)
	}

	currBranch, err := getCurrentBranch(repoDir)
	if err != nil {
		log.Fatalf("Failed to determine the current branch: %s\n", err)
	}

	fmt.Printf("Committed: [%s %s] %s\n", currBranch, commitObj.hash, *commitMessagePtr)
}
