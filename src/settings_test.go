package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettings(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(t.TempDir())
	require.NoError(t, err)
	defer os.Chdir(wd)

	ctx := &CLIContext{
		Quiet: true,
	}

	t.Run("init creates a config; set changes it", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		wd, err := os.Getwd()
		require.NoError(err)
		err = os.Chdir(t.TempDir())
		require.NoError(err)
		defer os.Chdir(wd)

		ename := "skunky-joe"
		init := &InitCmd{
			CommonSet: CommonSet{},
			Efmrl:     ename,
			Force:     true,
		}
		err = init.Run(ctx)
		assert.NoError(err)
		assert.FileExists(configName)

		cfg, err := loadConfig()
		assert.NoError(err)
		require.NotNil(cfg)

		assert.Equal(ename, cfg.Efmrl)
	})

	t.Run("init won't overwrite another without force", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		// ensure a clean slate
		os.Remove(configName)

		ename := "smooch-a-pooch"
		rootDir := "root-y-toot-toot"
		init := InitCmd{
			CommonSet: CommonSet{
				Insecure: true,
			},
			Efmrl:   ename,
			RootDir: rootDir,
		}

		err := init.Run(ctx)
		assert.NoError(err)

		cfg, err := loadConfig()
		require.NoError(err)
		assert.Equal(ename, cfg.Efmrl)
		assert.Equal(rootDir, cfg.RootDir)

		err = init.Run(ctx)
		assert.Error(err)

		init.Force = true
		err = init.Run(ctx)
		assert.NoError(err)
	})
}
