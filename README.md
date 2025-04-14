# git-in-go

[![Go 1.22](https://img.shields.io/badge/go-1.22-9cf.svg)](https://golang.org/dl/)

An implementation of Git using Go. Inspired by the [CodeCrafters Git challenge](https://app.codecrafters.io/courses/git/overview). This Git implementation is capable of initializing a new Git repository, cloning a repository, maintaining a set of files in the index/staging area, determining the status of the repository's working tree, creating new commits, pushing commits to the remote origin, pulling commits from the remote origin, creating new branches, and checking out branches.

The `mygit` program entrypoint is written in [main.go](mygit/main.go), which then relies on the command handlers in [commands.go](mygit/commands.go).

<div align="center">
  <img src="./docs/assets/git-logo.png" alt="Git Logo">
</div>

## Git Object Representation

### Supported Objects

The object representation in [objects.go](mygit/objects.go) supports Git blobs, trees, and commits. Each object has an associated SHA-1 hash determined by its contents, and specifying where to store the object within the repository's `.git/` directory.

A blob object stores the contents of a tracked file in the Git repository. A tree object stores the structure of a directory in the repository, so its entries can be either blobs (files) or other trees (subdirectories). A commit object represents a Git commit made by a user for the repository. A commit references a tree, representing the state of the repository at the time the commit was made.

The contents of an object file, consisting of a header containing metadata and the actual object contents, are compressed with `zlib` when written to disk.

### Plumbing Commands

This implementation supports various Git plumbing commands allowing low-level interaction with objects. Documentation for these commands can be found in [commands.go](mygit/commands.go):
- `cat-file`
- `hash-object`
- `ls-tree`
- `write-tree`
- `write-working-tree`
- `commit-tree`

## Cloning a Repository

Cloning a repository requires two stages of interaction with the remote Git server. First, reference discovery is performed to retrieve the remote `HEAD` of the repository and its various branches, identified by both branch name and `HEAD` commit hash. Second, the client performs a `git-upload-pack` request, requesting for the remote server to send all Git objects associated with the desired references (refs).

A successful response to the client's `git-upload-pack` request is a packfile containing all of the desired objects, constructed according to Git's [format for packfiles](https://git-scm.com/docs/pack-format). This implementation parses the packfile, decompresses each individual object's contents, and creates each object on the local disk. At this point, the `HEAD` commit specified by the reference discovery request can be checked out by traversing its directory structure and creating the corresponding files and directory structure. Finally, the local repository's refs are updated to indicate that the local and remote `HEAD`s reflect the information most recently pulled from the remote source.

Finally, this implementation copies [run.sh](run.sh) into the root of any cloned repository, so that subsequent commands can be run with `mygit`.

## The Index/Staging Area

The Git index file, stored at the root of the `.git/` directory, contains a list of files in the repository's working tree which are currently being tracked. If the latest version of a file is stored in the index, it is either already up-to-date in the latest commit or staged for the next commit. The Git index can be managed via commands `ls-files`, `add`, and `reset`.

The `status` command takes into account the repository working tree, the index, the local `HEAD`, and the remote `HEAD`. Each file is assigned one of the following statuses: `Untracked`, `ModifiedNotStaged`, `DeletedNotStaged`, `ModifiedStaged`, `AddedStaged`, `DeletedStaged`, or `Unmodified`. Subsequently, staged changes, unstaged changes, and untracked files are displayed to the user. 

## Committing, Pushing, & Pulling

Committing is implemented by producing a tree from the current state of the index, creating a commit object from that tree, and updating the ref for the current branch to point to the new commit.

Pushing is implemented by determining which objects are present in the local `HEAD` but missing in the remote `HEAD`, creating a packfile out of those objects, and making a `git-receive-pack` request to the remote Git server to send the encoded objects.

Pulling is implemented via roughly the same process as cloning. A `git-upload-pack` request is made to fetch the most up-to-date objects in the remote source, and then the packfile is read and applied in order to update the local repository.

## Checking Out Branches

Checking out a branch by name requires looking up the `HEAD` commit for that branch (via its ref) and checking it out. Creating a new branch locally and then publishing it to the remote source is also supported.

## Using `mygit`

The `./run.sh` script is used as an entrypoint into `mygit`'s commands, in the same way that `git` is used preceding specific commands. For example, the command `./run.sh clone https://github.com/shashjar/git-in-go cloned-git-in-go` will produce a local directory `cloned-git-in-go/` into which this repository will be cloned.

`GIT_USERNAME` and `GIT_TOKEN` can be set as environment variables in a `.env` file for usage with private repositories. The token must be a [Personal Access Token (PAT)](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens) scoped with `repo` access at minimum. For usage solely with public repositories, these environment variables can be set to dummy values.
