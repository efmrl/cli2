package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/efmrl/api2"
)

type PermsCmd struct {
	List   PermsListCmd   `cmd:"" help:"list all perms for efmrl"`
	Define PermsDefineCmd `cmd:"" help:"define permissions"`
	Efmrl  PermsEfmrlSubs `cmd:"" help:"commands for efmrl's permissions"`

	GrantAll PermsGrantAll `cmd:"" help:"grant all permissions to the user"`
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

	url := cfg.pathToAPIurl("perms")

	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	allPerms := &api2.AllPerms{}
	apiRes := api2.NewResult(allPerms)
	err = getJSON(client, url, apiRes)
	if err != nil {
		return err
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 4, ' ', tabwriter.AlignRight)

	if allPerms.Efmrl != nil {
		showSpecialPerms(tw, "efmrl", allPerms.Efmrl)
	}

	if allPerms.Mounts != nil {
		for path, mnt := range allPerms.Mounts {
			showSpecialPerms(tw, path, mnt.Specials)
		}
	}

	fmt.Fprintln(tw)

	for _, user := range allPerms.Users {
		name := user.Name
		if name == "" {
			name = user.ID
		}
		fmt.Fprint(
			tw,
			name,
			": \t",
			showPerms(&user.Perms),
			"\n",
		)
	}

	tw.Flush()

	return nil
}

func showSpecialPerms(out io.Writer, name string, spec *api2.SpecialPerms) {
	fmt.Fprintln(out, name)
	fmt.Fprintf(out, "     everyone: \t%v\n",
		showPerms(spec.Everyone),
	)
	fmt.Fprintf(out, "    sessioned: \t%v\n",
		showPerms(spec.Sessioned),
	)
	fmt.Fprintf(out, "authenticated: \t%v\n",
		showPerms(spec.Authenticated),
	)

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

	allPerms := &api2.AllPerms{
		Efmrl: &api2.SpecialPerms{
			Everyone: &perms,
		},
	}
	res, err := patchJSON(ctx.Context, client, url, allPerms, nil)
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("failed: %v", res.Status)
	}

	fmt.Println("done")

	return nil
}

type PermsGrantAll struct {
	User string `opt:"" help:"user ID to grant; default is self"`

	ts *httptest.Server
}

func (pgoa *PermsGrantAll) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.ts = pgoa.ts

	client, err := cfg.getClient()
	if err != nil {
		return err
	}
	var url *url.URL
	userID := pgoa.User
	if userID == "" {
		url = cfg.pathToAPIurl("session")
		ses := &api2.SessionRes{}
		err = getJSON(client, url, api2.NewResult(ses))
		if err != nil {
			err = fmt.Errorf("cannot get login session: %w", err)
			return err
		}
		userID = ses.UserID
	}

	url = cfg.pathToAPIurl("perms")
	ap := &api2.AllPerms{
		Users: map[string]*api2.User{
			userID: &api2.User{
				Perms: api2.PermAllDefined,
			},
		},
	}
	resp, err := patchJSON(ctx.Context, client, url, ap, nil)
	if err != nil {
		err = fmt.Errorf("cannot grant permissions: %w", err)
		return err
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("cannot grant permissions: %q", resp.Status)
		return err
	}

	return nil
}

func showPerms(perms *api2.Perm) string {
	if perms == nil {
		return ""
	}

	return strings.Join(perms.SimpleNames(), " ")
}
