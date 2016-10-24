package cli

import (
	"context"
	"internal/run"

	"github.com/google/subcommands"
)

func init() {
	subcommands.Register(run.NewCmdAve(), "")
	subcommands.Register(run.NewCmdFoz(), "")
	subcommands.Register(run.NewCmdBel(), "")
	subcommands.Register(run.NewCmdA24(), "")
	subcommands.Register(run.NewCmdStl(), "")
	subcommands.Register(run.NewCmdA55(), "")
	subcommands.Register(run.NewCmdTst(), "")
}

// Run registers commands in subcommands and execute it
func Run() int {
	return int(subcommands.Execute(context.Background()))
}
