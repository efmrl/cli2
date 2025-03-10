package main

import (
	"fmt"
	"net/http/httptest"

	"github.com/efmrl/api2"
)

type NamesCmd struct {
	List NamesList `cmd:"" help:"list efmrl's names"`
}

type NamesList struct {
	ts *httptest.Server
}

func (nl *NamesList) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = nl.ts

	url := cfg.pathToAPIurl("names")

	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	names := &api2.GetNamesRes{}
	lnRes := api2.NewResult(names)
	err = getJSON(client, url, lnRes)
	if err != nil {
		return err
	}

	for _, name := range names.Names {
		fmt.Printf("%v\n", name.Name)
	}

	return nil
}
