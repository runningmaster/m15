package main

import (
	"internal/flag"
	"internal/log"
	"internal/version"
	"os"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

func main() {
	flag.Parse()
	log.Printf("main: start version %s", version.Stamp)
	os.Exit(int(subcommands.Execute(context.Background())))
}
