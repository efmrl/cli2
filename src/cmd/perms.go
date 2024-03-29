package main

import (
	"fmt"
	"net/http/httptest"
	"strings"

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

	fmt.Printf("     everyone: %v\n",
		strings.Join(settings.Perms.Everyone.SimpleNames(), " "),
	)
	fmt.Printf("    sessioned: %v\n",
		strings.Join(settings.Perms.Sessioned.SimpleNames(), " "),
	)
	fmt.Printf("authenticated: %v\n",
		strings.Join(settings.Perms.Authenticated.SimpleNames(), " "),
	)

	return nil
}
