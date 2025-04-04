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
- [x] Implement the index/staging area
  - [x] Add functionality for reading & writing the index file
  - [x] `git ls-files`
  - [x] `git add` / `git reset`
  - [x] `git status`
  - [x] Update `git write-tree` to write a tree object from the index file instead of the working tree directory (preserve old functionality as `git write-working-tree`)
- [x] Implement `git commit`
  - [x] Use the `commit-tree` plumbing command to produce commit objects
- [ ] Implement `git push`
  - [ ] Implement the functionality for encoding a list of Git objects as a packfile
  - [ ] Implement the functionality for comparing the remote HEAD with the local HEAD and determining which objects are missing from the remote commit (and therefore need to be included in the packfile when `push`ing)
  - [ ] Write the main `push` handler, making the HTTP request to the remote repo with the user's username and password and the packfile
- [ ] Implement creation and checking out of branches
- [ ] Implement `git pull`

## Aesthetics/Usability

- [x] Use the directory in which the `run.sh` script is run as the repo directory to execute the command with
- [ ] Add a progress bar/percentage completion for cloning (like actual Git)

## Housekeeping/Tech Debt

- [x] Add documentation to each of the available commands in `commands.go`
- [ ] Update [README](README.md) with information about implementation and usage
- [ ] Update `Git in Go` project document in Obsidian
- [ ] Write a blog post or something that has better/clearer documentation of packfile object schemas than what's currently out there
