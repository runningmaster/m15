package run

import (
	"flag"
	"fmt"
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
	fmt.Println("test setFlag()")
}

func (c *cmdTst) exec() error {
	fmt.Println("test exec()")
	return nil
}
