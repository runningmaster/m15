package conf

import (
	"expvar"
	"flag"
	"strconv"
	//"github.com/google/subcommands"
)

// NOTE: about priority: default <- key/value store <- config <- env <- flag <-explicit set

var (
	// Verbose is flag for output
	Verbose = *flag.Bool("verbose", true, "Verbose output")

	// Debug mode
	Debug = *flag.Bool("debug", false, "Debug mode")
)

func init() {
	//subcommands.ImportantFlag("verbose")
	//subcommands.ImportantFlag("debug")
}

// Parse is wrapper for std flag.Parse()
func Parse() {
	flag.Parse()
	experimentWithExpVarFIXME()
}

func experimentWithExpVarFIXME() {
	expvar.NewInt("debug").Set(1)
	d, _ := strconv.Atoi(expvar.Get("debug").String())
	Debug = d == 1
}
