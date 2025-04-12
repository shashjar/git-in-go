package main

import "flag"

var CopyRunSh = flag.Bool("copy-run-sh", true, "Copy the mygit run.sh script into the root of repositories as soon as they are cloned")
