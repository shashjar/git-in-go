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
./run.sh commit-tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904 -m "Initial commit"
```
