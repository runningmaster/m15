package main

import (
	"flag"
	"log"
	"os"

	"internal/cmd"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

func main() {
	flag.Parse()
	log.SetFlags(0)
	log.SetOutput(os.Stderr)

	cmd.Register()
	os.Exit(int(subcommands.Execute(context.Background())))
}
