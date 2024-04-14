package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"path"

	"github.com/efmrl/api2"
)

type UserCmd struct {
	Get GetUser `cmd:"" help:"get current user"`
}

type GetUser struct {
	ts *httptest.Server
}

func (gu *GetUser) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = gu.ts

	url, err := getUserPath(cfg)
	if err != nil {
		return err
	}

	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	res := &api2.NewUser{}
	nuRes := api2.NewResult(res)
	err = httpGetJSON(client, url, nuRes)
	if err != nil {
		err = fmt.Errorf("cannot get user data: %w", err)
		return err
	}

	out, err := json.MarshalIndent(res, "", "    ")
	if err != nil {
		return err
	}

	fmt.Println(string(out))

	return nil
}

func getUserPath(cfg *Config) (*url.URL, error) {
	url := cfg.pathToAPIurl("session")
	client, err := cfg.getClient()
	if err != nil {
		return nil, err
	}

	res := &api2.NewSessionRes{}
	nsRes := api2.NewResult(res)
	err = httpGetJSON(client, url, nsRes)
	if err != nil {
		err = fmt.Errorf("cannot get session: %w", err)
		return nil, err
	}

	if res.UserID == "" {
		return nil, fmt.Errorf("not logged in")
	}

	path := path.Join("users", res.UserID, "data")
	url = cfg.pathToAPIurl(path)

	return url, nil
}
