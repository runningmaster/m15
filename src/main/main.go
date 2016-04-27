package main

import (
	"io/ioutil"
	"log"
	"os"

	"internal/cmd"
	"internal/flag"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

func main() {
	flag.Parse()
	initLogger()

	cmd.Register()
	os.Exit(int(subcommands.Execute(context.Background())))
}

func initLogger() {
	log.SetFlags(0)
	log.SetOutput(ioutil.Discard)
	if flag.Verbose {
		log.SetOutput(os.Stderr)
	}
}
