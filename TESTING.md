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

# `git write-tree` & `git ls-tree`

```
./run.sh write-tree
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

# Reading a zlib-compressed file

```
python3 -c "import sys, zlib; print(zlib.decompress(sys.stdin.buffer.read()).decode())" < <file_name>
```
