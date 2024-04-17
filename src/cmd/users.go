package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"

	"github.com/efmrl/api2"
)

type UserCmd struct {
	Get    GetUser    `cmd:"" help:"get current user"`
	Create CreateUser `cmd:"" help:"create new user"`
	Update UpdateUser `cmd:"" help:"update a user"`
}

type GetUser struct {
	UserID string `opt:"" help:"user ID; default current user"`

	ts *httptest.Server
}

func (gu *GetUser) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = gu.ts

	userID := gu.UserID
	if userID == "" {
		userID, err = getCurrentUserID(cfg)
		if err != nil {
			err = fmt.Errorf("cannot get current user ID: %w", err)
			return err
		}
	}

	url := getUserPath(cfg, userID)
	if err != nil {
		return err
	}

	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	res := &api2.User{}
	nuRes := api2.NewResult(res)
	err = getJSON(client, url, nuRes)
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

type CreateUser struct {
	Email string `help:"email address"`
	Name  string `help:"name for the user"`

	ts *httptest.Server
}

func (cu *CreateUser) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = cu.ts

	req := &api2.PostUserReq{
		Name: cu.Name,
		Email: &api2.Email{
			Address: cu.Email,
		},
	}
	user := &api2.User{}

	client, err := cfg.getClient()
	if err != nil {
		return err
	}
	url := cfg.pathToAPIurl("/users")

	result := api2.NewResult(user)
	res, err := postJSON(client, url, req, result)
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		err = fmt.Errorf("create failed: %v", result.Message)
		return err
	}

	fmt.Printf("new user ID: %q\n", user.ID)

	return nil
}

type UpdateUser struct {
	UserID string `opt:"" help:"user ID; default current user"`
	Name   string `opt:"" help:"the name for the user"`

	ts *httptest.Server
}

func (uu *UpdateUser) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = uu.ts

	userID := uu.UserID
	if userID == "" {
		userID, err = getCurrentUserID(cfg)
		if err != nil {
			err = fmt.Errorf("cannot get current user: %w", err)
			return err
		}
	}
	url := getUserPath(cfg, userID)
	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	req := &api2.User{
		Name: uu.Name,
	}
	resp, err := patchJSON(ctx.Context, client, url, req, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("error on update: %v", resp.Status)
		return err
	}

	return nil
}

func getUserPath(cfg *Config, userID string) *url.URL {
	path := path.Join("users", userID)
	url := cfg.pathToAPIurl(path)
	return url
}

func getCurrentUserID(cfg *Config) (string, error) {
	url := cfg.pathToAPIurl("session")
	client, err := cfg.getClient()
	if err != nil {
		return "", err
	}

	res := &api2.SessionRes{}
	nsRes := api2.NewResult(res)
	err = getJSON(client, url, nsRes)
	if err != nil {
		err = fmt.Errorf("cannot get session: %w", err)
		return "", err
	}

	if res.UserID == "" {
		return "", fmt.Errorf("not logged in")
	}

	return res.UserID, nil
}
