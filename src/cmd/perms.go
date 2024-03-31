package main

import (
	"fmt"
	"net/http/httptest"
	"strings"

	"github.com/efmrl/api2"
)

type PermsCmd struct {
	List   PermsListCmd   `cmd:"" help:"list all perms for efmrl"`
	Define PermsDefineCmd `cmd:"" help:"define permissions"`
	Efmrl  PermsEfmrlSubs `cmd:"" help:"commands for efmrl's permissions"`
}

type PermsEfmrlSubs struct {
	Everyone PermsEfmrlEveryone `cmd:"" help:"permissions for everyone"`
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

	url := cfg.pathToAPIurl("perms/data")

	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	settings := &api2.AllPerms{}
	apiRes := api2.NewResult(settings)
	err = httpGetJSON(client, url, apiRes)
	if err != nil {
		return err
	}

	fmt.Printf("     everyone: %v\n",
		strings.Join(settings.Efmrl.Everyone.SimpleNames(), " "),
	)
	fmt.Printf("    sessioned: %v\n",
		strings.Join(settings.Efmrl.Sessioned.SimpleNames(), " "),
	)
	fmt.Printf("authenticated: %v\n",
		strings.Join(settings.Efmrl.Authenticated.SimpleNames(), " "),
	)

	return nil
}

type PermsDefineCmd struct {
}

func (pd *PermsDefineCmd) Run() error {
	for _, perm := range api2.PermSimplePerms() {
		val := api2.PermNameValue[perm]
		perm = strings.TrimPrefix(perm, "Perm")
		fmt.Printf("%14v - %v\n", perm, api2.PermShortDefinitions[val])
	}

	return nil
}

type PermsEfmrlEveryone struct {
	Set PermsEfmrlEveryoneSet `cmd:"" help:"set permissions for everyone"`
}

type PermsEfmrlEveryoneSet struct {
	ts *httptest.Server

	Perms []string `arg:"" optional:"" help:"permissions to set"`
}

func (pees *PermsEfmrlEveryoneSet) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = pees.ts

	url := cfg.pathToAPIurl("perms/data")

	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	perms, err := api2.SimpleNamesToPerm(pees.Perms)
	if err != nil {
		return err
	}

	settings := &api2.AllPerms{
		Efmrl: &api2.SpecialPerms{
			Everyone: perms,
		},
	}
	res, err := putJSON(ctx.Context, client, url, settings, nil)
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("failed: %v", res.Status)
	}

	fmt.Println("done")

	return nil
}
