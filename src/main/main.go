package main

import (
	"os"

	"internal/cmd"
	"internal/flag"
	"internal/log"
	"internal/version"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

func main() {
	flag.Parse()
	log.Printf("main: start version %s", version.Stamp)
	cmd.Register()
	os.Exit(int(subcommands.Execute(context.Background())))
}
