package main

import (
	"flag"
	"log"
	"os"

	"internal/cli"
)

func main() {
	flag.Parse()

	log.SetFlags(0)
	log.SetOutput(os.Stderr)

	os.Exit(cli.Run())
}
