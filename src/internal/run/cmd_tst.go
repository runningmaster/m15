package run

import (
	"flag"
	"log"
)

type cmdTst struct {
	cmdBase
}

func NewCmdTst() *cmdTst {
	cmd := &cmdTst{}
	cmd.mustInitBase(cmd, "tst", "test command")
	return cmd
}

func (c *cmdTst) setFlags(f *flag.FlagSet) {
	log.Println("test setFlag()")
}

func (c *cmdTst) exec() error {
	log.Println("test exec()")
	return nil
}
