package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func CreateBranch(branchName string, repoDir string) error {
	err := CreateIndexFromWorkingTree(repoDir)
	if err != nil {
		return fmt.Errorf("failed to create Git index from working tree: %s", err)
	}

	treeObj, err := CreateTreeObjectFromIndex(repoDir)
	if err != nil {
		return fmt.Errorf("failed to create tree object from Git index: %s", err)
	}

	commitObj, err := CreateCommitObjectFromTree(treeObj.hash, []string{}, fmt.Sprintf("Create branch %s", branchName), repoDir)
	if err != nil {
		return fmt.Errorf("failed to create commit object from tree: %s", err)
	}

	branchRefPath := filepath.Join(repoDir, ".git", "refs", "heads", branchName)
	_, err = os.Stat(branchRefPath)
	if !os.IsNotExist(err) {
		return fmt.Errorf("branch %s already exists", branchName)
	}

	err = UpdateBranchRef(branchName, commitObj.hash, false, repoDir)
	if err != nil {
		return fmt.Errorf("failed to create reference for new branch %s: %s", branchName, err)
	}

	return nil
}

func CheckoutBranch(branchName string, repoDir string) error {
	headCommitHash, commitsExist, err := ResolveBranchRef(branchName, false, repoDir)
	if err != nil || !commitsExist {
		return fmt.Errorf("no branch named %s found", branchName)
	}

	err = CheckoutCommit(headCommitHash, repoDir)
	if err != nil {
		return fmt.Errorf("failed to checkout commit %s: %s", headCommitHash, err)
	}

	err = updateRefsAfterCheckout(branchName, repoDir)
	if err != nil {
		return err
	}

	return nil
}

func updateRefsAfterCheckout(branchName string, repoDir string) error {
	err := UpdateHeadWithBranchRef(branchName, false, repoDir)
	if err != nil {
		return fmt.Errorf("failed to update local HEAD reference with branch %s: %s", branchName, err)
	}

	err = UpdateHeadWithBranchRef(branchName, true, repoDir)
	if err != nil {
		return fmt.Errorf("failed to update remote HEAD reference with branch %s: %s", branchName, err)
	}

	return nil
}
