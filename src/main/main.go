package main

import (
	"flag"
	"log"
	"os"

	"internal/cmd"
)

func main() {
	flag.Parse()

	log.SetFlags(0)
	log.SetOutput(os.Stderr)

	os.Exit(cmd.Run())
}
