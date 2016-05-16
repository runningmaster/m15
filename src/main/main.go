package main

import (
	"io/ioutil"
	"log"
	"os"

	"internal/conf"
	sc "internal/subcmd"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

func init() {
	sc.Register()
	os.Exit(int(subcommands.Execute(context.Background())))
}

func main() {
	initConfig()
	initLogger()
}

func initConfig() {
	conf.Parse()
}

func initLogger() {
	log.SetFlags(0)
	log.SetOutput(ioutil.Discard)
	if conf.Verbose {
		log.SetOutput(os.Stderr)
	}
}
