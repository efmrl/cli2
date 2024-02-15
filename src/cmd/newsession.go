package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/efmrl/api2"
)

type NewSession struct {
	Get     NewSessionGet `cmd:"" help:"get newsession info"`
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

	url := cfg.pathToAPIurl("newsession")

	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	res, err := client.Get(url.String())
	if err != nil {
		return fmt.Errorf("cannot connect to server: %w", err)
		return err
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("get newstatus failed: %v", res.Status)
	}

	dec := json.NewDecoder(res.Body)
	nsRes := &api2.NewSessionRes{}
	err = dec.Decode(nsRes)
	if err != nil {
		return err
	}

	out, err := json.MarshalIndent(nsRes, "", "    ")
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

	url := cfg.pathToAPIurl("newsession")

	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	req := &api2.NewSessionReq{
		CookieOK: true,
		UserKey:  dc.Who,
	}
	res := &api2.NewSessionRes{}

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
		if dec.Decode(stuff) == nil {
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

	out, err := json.MarshalIndent(res, "", "    ")
	if err != nil {
		return err
	}

	gecfg.eatAllCookies(client, url)
	cfg.save()

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

	url := cfg.pathToAPIurl("newsession")

	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	req := &api2.NewSessionReq{
		CookieOK:   true,
		UserSecret: cc.Secret,
	}
	res := &api2.NewSessionRes{}

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
		if dec.Decode(stuff) == nil {
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

	out, err := json.MarshalIndent(res, "", "    ")
	if err != nil {
		return err
	}

	gecfg.eatAllCookies(client, url)
	cfg.save()

	fmt.Println(string(out))

	return nil
}
