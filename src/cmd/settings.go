package main

import (
	"fmt"
	"os"
)

// CommonSet holds option that are common between "set" and "init"
type CommonSet struct {
	Rewrite     []string `help:"file names to rewrite to their directories"`
	NoRewrite   []string `help:"file names not to be rewritten as directories"`
	ParentEfmrl string   `kong:"optional"`
	NoParent    bool     `help:"remove any --parent-efmrl"`
	BaseHost    string   `kong:"hidden"`
	Insecure    bool     `kong:"hidden"`
}

// SetCmd holds the options to the "set" subcommand
type SetCmd struct {
	CommonSet
	Efmrl   string `kong:"short='e',help='the name of your efmrl'"`
	RootDir string `kong:"short='r',help='the root directory for syncing'"`
}

// InitCmd holds the options to the "init" subcommand
type InitCmd struct {
	CommonSet
	Efmrl   string `kong:"required,short='e',help='the name of your efmrl'"`
	RootDir string `kong:"required,short='r',help='the directory which will be uploaded to your efmrl'"`
	Force   bool   `kong:"short='f',help='reinitialize even if file already exists'"`
}

// updateConfig updates a config struct with all nonzero members of common
func (common *CommonSet) updateConfig(cfg *Config) {
	for _, fname := range common.NoRewrite {
		delete(cfg.indexRewrite, fname)
		cfg.indexNoRewrite[fname] = true
	}
	for _, fname := range common.Rewrite {
		delete(cfg.indexNoRewrite, fname)
		cfg.indexRewrite[fname] = true
	}

	if common.BaseHost != "" {
		cfg.BaseHost = common.BaseHost
	}

	if common.Insecure {
		cfg.Insecure = true
	}
}

// updateConfig updates a config struct with all nonzero members of set
func (set *SetCmd) updateConfig(cfg *Config) {
	if set.Efmrl != "" {
		cfg.Efmrl = set.Efmrl
	}

	if set.RootDir != "" {
		cfg.RootDir = set.RootDir
	}

	set.CommonSet.updateConfig(cfg)
}

// Run the "set" subcommand
func (set *SetCmd) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"you must be in your project's top directory; if you are, use 'init' to create a new config",
		)
		return err
	}

	set.updateConfig(cfg)

	err = cfg.save()
	if err != nil {
		return err
	}

	if !ctx.Quiet {
		fmt.Printf("%q updated\n", configName)
	}

	return nil
}

// Run the "init" subcommand
func (init *InitCmd) Run(ctx *CLIContext) error {
	if !init.Force {
		_, err := os.Stat(configName)
		if err == nil {
			return fmt.Errorf("config file already exists; modify with 'set'")
		}
	}

	cfg := &Config{
		Efmrl:   init.Efmrl,
		RootDir: init.RootDir,
	}
	init.updateConfig(cfg)

	err := cfg.save()
	if err != nil {
		return err
	}

	if !ctx.Quiet {
		fmt.Printf("%q created\n", configName)
	}

	return nil
}
