package main

import (
	"fmt"
	"net/http/httptest"

	"github.com/efmrl/api2"
)

type PermsCmd struct {
	List PermsListCmd `cmd:"" help:"list all perms for efmrl"`
}

type PermsListCmd struct {
	ts *httptest.Server
}

func (pl *PermsListCmd) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = pl.ts

	url := cfg.pathToAPIurl("settings/data")

	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	settings := &api2.EfmrlSettings{}
	apiRes := api2.NewResult(settings)
	err = httpGetJSON(client, url, apiRes)
	if err != nil {
		return err
	}

	fmt.Printf("%v\n", settings.Perms.Everyone)
	fmt.Printf("%v\n", settings.Perms.Sessioned)
	fmt.Printf("%v\n", settings.Perms.Authenticated)

	return nil
}
