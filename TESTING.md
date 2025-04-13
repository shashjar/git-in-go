After moving `run.sh` into the repository directory, the below commands can be used (the script directory will be used as the repository directory).

# `git init`

```
./run.sh init
```

# `git cat-file`

```
echo "hello world" > test.txt
git hash-object -w test.txt
./run.sh cat-file -p 3b18e512dba79e4c8300dd08aeb37f8e728b8dad
```

# `git hash-object`

```
echo "hello world" > test.txt
./run.sh hash-object -w test.txt
file repo/.git/objects/3b/18e512dba79e4c8300dd08aeb37f8e728b8dad
```

# `git write-tree`, `git write-working-tree`, & `git ls-tree`

```
./run.sh write-tree
./run.sh write-working-tree
```

Then use the outputted tree hash with

```
./run.sh ls-tree [--name-only] <tree_hash>
```

# `git commit-tree`

```
echo "hello world" > test.txt
./run.sh write-tree
./run.sh commit-tree f5e9585a3f08476bd248b12e64230900c21baace -m "Initial commit"
```

# `git clone`

Run from the project root.

```
./run.sh clone https://github.com/shashjar/redis-in-go cloned-redis-in-go
```

# `git ls-files`

```
./run.sh ls-files
./run.sh ls-files -s
```

# `git add`

```
./run.sh add test.txt
./run.sh add test_file_1.txt
./run.sh add test_dir_1/test_file_2.txt
./run.sh ls-files
```

```
./run.sh add .
./run.sh ls-files
```

# `git reset`

```
./run.sh add test.txt
./run.sh add test_file_1.txt
./run.sh ls-files
./run.sh reset test_file_1.txt
./run.sh ls-files
```

# `git status`

```
./run.sh status
```

# `git commit`

```
./run.sh commit
./run.sh commit -m "I'm making a commit"
```

# `git push`

```
./run.sh commit -m "Making a new commit to test my `push` command`
./run.sh push <remote_repo_url>
```

# `git pull`

```
./run.sh pull <remote_repo_url>
```

# Reading a zlib-compressed file

```
python3 -c "import sys, zlib; print(zlib.decompress(sys.stdin.buffer.read()).decode())" < <file_name>
```
