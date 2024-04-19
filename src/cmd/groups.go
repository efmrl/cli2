package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"text/tabwriter"

	"github.com/efmrl/api2"
)

type GroupCmd struct {
	Create CreateGroup `cmd:"" help:"create a group"`
	Get    GetGroup    `cmd:"" help:"get a group"`
	List   ListGroups  `cmd:"" help:"list groups"`
	Update UpdateGroup `cmd:"" help:"update a group"`
	Delete DeleteGroup `cmd:"" help:"delete a group"`
}

type CreateGroup struct {
	Name string `opt:"" help:"name of the group to create"`

	ts *httptest.Server
}

func (cg *CreateGroup) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = cg.ts

	req := &api2.PostGroupReq{
		Name: cg.Name,
	}
	group := &api2.Group{}

	client, err := cfg.getClient()
	if err != nil {
		return err
	}
	url := cfg.pathToAPIurl("/groups")

	result := api2.NewResult(group)
	_, err = postJSON(client, url, req, result)
	if err != nil {
		return err
	}

	if result.Status != api2.StatusSuccess {
		err = fmt.Errorf("create failed: %v", result.Message)
		return err
	}

	fmt.Printf("new group ID: %q\n", group.ID)

	return nil
}

type GetGroup struct {
	ID string `opt:"" help:"ID of the group to create"`

	ts *httptest.Server
}

func (gg *GetGroup) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = gg.ts

	url := getGroupPath(cfg, gg.ID)
	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	res := &api2.Group{}
	ngRes := api2.NewResult(res)
	err = getJSON(client, url, ngRes)
	if err != nil {
		err = fmt.Errorf("cannot get group data: %w", err)
		return err
	}

	out, err := json.MarshalIndent(res, "", "    ")
	if err != nil {
		return err
	}

	fmt.Println(string(out))

	return nil
}

type ListGroups struct {
	ts *httptest.Server
}

func (lg *ListGroups) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = lg.ts

	url := cfg.pathToAPIurl("/groups")
	client, err := cfg.getClient()
	if err != nil {
		return err
	}
	groups := &api2.GetGroupsRes{}
	err = getJSON(client, url, api2.NewResult(groups))
	if err != nil {
		return err
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 1, ' ', 0)

	for _, group := range groups.Groups {
		fmt.Fprintf(tw, "%v\t %v\t\n", group.ID, group.Name)
	}

	tw.Flush()
	return nil
}

type UpdateGroup struct {
	ID   string `required:"" help:"ID of the group to update"`
	Name string `opt:"" help:"new name for the group"`

	ts *httptest.Server
}

func (ug *UpdateGroup) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = ug.ts

	url := getGroupPath(cfg, ug.ID)
	client, err := cfg.getClient()
	if err != nil {
		return err
	}
	req := &api2.Group{
		Name: ug.Name,
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

type DeleteGroup struct {
	ID string `opt:"" help:"group ID to be deleted"`

	ts *httptest.Server
}

func (dg *DeleteGroup) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = dg.ts

	url := getGroupPath(cfg, dg.ID)
	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx.Context,
		"DELETE",
		url.String(),
		nil,
	)
	if err != nil {
		return err
	}

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		err = fmt.Errorf("cannot delete group: %v", res.Status)
		return err
	}

	return nil
}

func getGroupPath(cfg *Config, groupID string) *url.URL {
	path := path.Join("groups", groupID)
	url := cfg.pathToAPIurl(path)
	return url
}
