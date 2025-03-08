package main

import (
	"fmt"
	"log"
	"os"
)

func utilPrintObjectHandler() {
	if len(os.Args) != 3 {
		log.Fatal("Usage: print-object <object_sha>")
	}

	objHash := os.Args[2]
	if !isValidObjectHash(objHash) {
		log.Fatalf("Invalid object hash: %s\n", objHash)
	}

	data, err := readObjectFile(objHash)
	if err != nil {
		log.Fatalf("Unable to read object file: %s\n", err)
	}

	fmt.Println(string(data))
}
