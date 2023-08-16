package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/efmrl/api2"
)

// LoginCmd holds the args for logging in
type LoginCmd struct {
	Arg string `kong:"arg,help='give a string to use as a credential'"`

	ts *httptest.Server
}

// Run the "login" subcommand
func (login *LoginCmd) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = login.ts

	gecfg, err := cfg.getGlobalConfig()
	if err != nil {
		return err
	}

	url := cfg.pathToAPIurl("session")

	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	req := &api2.SessionReq{
		CookieOK: true,
		Input:    login.Arg,
	}

	message, err := json.Marshal(&req)
	if err != nil {
		return fmt.Errorf("marshalling login object: %w", err)
	}
	res, err := client.Post(
		url.String(),
		"application/json",
		bytes.NewReader(message),
	)
	if err != nil {
		return fmt.Errorf("cannot connect to server: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed: %q was incorrect", login.Arg)
	}

	dec := json.NewDecoder(res.Body)
	authRes := &api2.SessionRes{}
	err = dec.Decode(authRes)
	if err != nil {
		return fmt.Errorf("login failed: cannot decode response: %w", err)
	}
	fmt.Println(authRes.Message)

	url.Path = ""
	var success bool
	for _, cookie := range client.Jar.Cookies(url) {
		if gecfg.eatCookie(cookie) {
			success = true
		}
	}
	if !success {
		return fmt.Errorf("no cookie received from the server")
	}

	err = cfg.save()
	return err
}

// LogoutCmd logs you out
type LogoutCmd struct {
}

// Run the "logout" command
func (logout *LogoutCmd) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	gecfg, err := cfg.getGlobalConfig()
	if err != nil {
		return err
	}

	gecfg.Cookie = ""
	gecfg.StrictCookie = ""

	err = cfg.save()
	if err != nil {
		return fmt.Errorf("cannot save config: %w", err)
	}

	return nil
}
