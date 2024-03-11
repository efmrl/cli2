package main

import (
	"encoding/json"
	"fmt"
	"net/http"
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

	url := cfg.pathToAPIurl("names/data")

	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	res, err := client.Get(url.String())
	if err != nil {
		err = fmt.Errorf("cannot connect to server: %w", err)
		return err
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("get list failed: %v", res.Status)
	}

	dec := json.NewDecoder(res.Body)
	names := &api2.GetNamesRes{}
	lnRes := api2.NewResult(names)
	err = dec.Decode(lnRes)
	if err != nil {
		return err
	}
	if lnRes.Status != api2.StatusSuccess {
		err = fmt.Errorf("%v: %v", lnRes.Status, lnRes.Message)
		return err
	}

	for _, name := range names.Names {
		fmt.Printf("%v\n", name.Name)
	}

	return nil
}
