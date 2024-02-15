package main

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
)

// CLIContext is for the CLI stuff
type CLIContext struct {
	Context context.Context
	Debug   bool
	Quiet   bool
}

// cli defines the overall CLI
var cli struct {
	Hello      HelloCmd   `cmd:"" help:"say hello world"`
	Init       InitCmd    `cmd:"" help:"init a new working area"`
	Set        SetCmd     `cmd:"" help:"update settings"`
	Sync       SyncCmd    `cmd:"" help:"sync working directory to cloud"`
	NewSession NewSession `cmd:"" help:"newsession stuff"`
}

// HelloCmd is for "hello world"
type HelloCmd struct{}

// Run says "hello world"
func (h *HelloCmd) Run(*CLIContext) error {
	fmt.Println("Hi buddy.")
	return nil
}

func main() {
	ctx := kong.Parse(&cli)

	//cfg, err := config.ParseConfig()
	//ctx.FatalIfErrorf(err, "cannot parse config")
	context := &CLIContext{
		Context: context.Background(),
		//Config:  cfg,
	}

	err := ctx.Run(context)
	ctx.FatalIfErrorf(err)
}
