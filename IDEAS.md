# Ideas for Extensions/Improvements to my Git Implementation

This is a living document storing some ideas for extensions & improvements that I might make to this implementation of Git.

## Functionality

- [x] Reach parity/same behavior with actual Git for basic commands that are being implemented
  - [x] `git init`
  - [x] `git cat-file`
  - [x] `git hash-object`
  - [x] `git ls-tree`
  - [x] `git write-tree`
  - [x] `git commit-tree`
  - [x] `git clone`
- [ ] Implement the index/staging area
  - [x] Add functionality for reading & writing the index file
  - [x] `git ls-files`
  - [x] `git add` / `git reset`
  - [ ] `git status`
- [ ] Implement `git commit`
  - [ ] Use the `commit-tree` plumbing command to produce commit objects
- [ ] Implement `git push`
- [ ] Implement creation and checking out of branches
- [ ] Implement `git pull`

## Aesthetics/Usability

- [x] Use the directory in which the `run.sh` script is run as the repo directory to execute the command with
- [ ] Add a progress bar/percentage completion for cloning (like actual Git)

## Housekeeping/Tech Debt

- [ ] Add documentation to each of the available commands in `commands.go`
- [ ] Update [README](README.md) with information about implementation and usage
- [ ] Update `Git in Go` project document in Obsidian
- [ ] Write a blog post or something that has better/clearer documentation of packfile object schemas than what's currently out there
