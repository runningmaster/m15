package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"

	"internal/cli"
)

var (
	flagVerbose  = flag.Bool("verbose", false, "make logging visible")
	flagHideTime = flag.Bool("hidetime", false, "show time when verbose")
)

func main() {
	flag.Parse()
	initLogger(*flagVerbose, *flagHideTime)
	os.Exit(cli.Run())
}

func initLogger(v, ht bool) {
	log.SetOutput(ioutil.Discard)
	if v {
		log.SetOutput(os.Stderr)
	}
	if ht {
		log.SetFlags(0)
	}
}
