package main

import (
	"os"

	"internal/cmd"
	"internal/flag"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

func main() {
	flag.Parse()
	cmd.Register()
	os.Exit(int(subcommands.Execute(context.Background())))
}
