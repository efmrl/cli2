package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/efmrl/api2"
)

type Session struct {
	Get     NewSessionGet `cmd:"" help:"get session info"`
	Declare DeclareCmd    `cmd:"" help:"declare a user"`
	Confirm ConfirmCmd    `cmd:"" help:"confirm you're the user"`
}

type NewSessionGet struct {
	ts *httptest.Server
}

func (ns *NewSessionGet) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = ns.ts

	url := cfg.pathToAPIurl("session")
	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	res := &api2.SessionRes{}
	err = getJSON(client, url, api2.NewResult(res))
	if err != nil {
		return err
	}

	out, err := json.MarshalIndent(res, "", "    ")
	if err != nil {
		return err
	}

	fmt.Println(string(out))

	return nil
}

type DeclareCmd struct {
	Who string `arg:"" help:"user identifier (e.g. email)"`

	ts *httptest.Server
}

func (dc *DeclareCmd) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = dc.ts

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
		UserKey:  dc.Who,
	}
	res := api2.NewResult(&api2.SessionRes{})

	message, err := json.Marshal(req)
	if err != nil {
		return err
	}
	pres, err := client.Post(
		url.String(),
		"application/json",
		bytes.NewReader(message),
	)
	if err != nil {
		err = fmt.Errorf("cannot connect to server: %w", err)
		return err
	}

	defer pres.Body.Close()
	dec := json.NewDecoder(pres.Body)

	if pres.StatusCode != http.StatusOK {
		stuff := map[string]any{}
		if dec.Decode(&stuff) == nil {
			out, err := json.MarshalIndent(stuff, "", "    ")
			if err == nil {
				fmt.Println(string(out))
			}
		}

		return fmt.Errorf("declare failed: %v", pres.Status)
	}

	err = dec.Decode(res)
	if err != nil {
		return err
	}
	if res.Status != api2.StatusSuccess {
		err = fmt.Errorf("%v: %v", res.Status, res.Message)
		return err
	}

	out, err := json.MarshalIndent(res.Data, "", "    ")
	if err != nil {
		return err
	}

	gecfg.eatAllCookies(client, url)
	err = cfg.save()
	if err != nil {
		return err
	}

	fmt.Println(string(out))

	return nil
}

type ConfirmCmd struct {
	Secret string `arg:"" help:"the login secret given to the user"`

	ts *httptest.Server
}

func (cc *ConfirmCmd) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = cc.ts

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
		CookieOK:   true,
		UserSecret: cc.Secret,
	}
	res := api2.NewResult(&api2.SessionRes{})

	message, err := json.Marshal(req)
	if err != nil {
		return err
	}
	pres, err := client.Post(
		url.String(),
		"application/json",
		bytes.NewReader(message),
	)
	if err != nil {
		err = fmt.Errorf("cannot connect to server: %w", err)
		return err
	}

	defer pres.Body.Close()
	dec := json.NewDecoder(pres.Body)

	if pres.StatusCode != http.StatusOK {
		stuff := map[string]any{}
		if dec.Decode(&stuff) == nil {
			out, err := json.MarshalIndent(stuff, "", "    ")
			if err == nil {
				fmt.Println(string(out))
			}
		}

		return fmt.Errorf("confirm failed: %v", pres.Status)
	}

	err = dec.Decode(res)
	if err != nil {
		return err
	}
	if res.Status != api2.StatusSuccess {
		err = fmt.Errorf("%v: %v", res.Status, res.Message)
		return err
	}

	out, err := json.MarshalIndent(res.Data, "", "    ")
	if err != nil {
		return err
	}

	gecfg.eatAllCookies(client, url)
	err = cfg.save()
	if err != nil {
		return err
	}

	fmt.Println(string(out))

	return nil
}
