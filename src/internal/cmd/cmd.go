package cmd

import (
	"github.com/google/subcommands"
)

// Register registers commands in subcommands
func Register() {
	subcommands.Register(ave, "")
}
