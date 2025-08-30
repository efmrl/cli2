package main

import (
	"os"
	"testing"

	"github.com/efmrl/api2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	goBack, err := cdTmp(t)
	require.NoError(t, err)
	defer goBack()

	t.Run("later versions error on load", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		goBack, err := cdTmp(t)
		require.NoError(err)
		defer goBack()

		cfg := &Config{
			Version: currentVersion + 1,
			Efmrl:   "who-cares",
		}
		err = cfg.save()
		require.NoError(err)

		cfg, err = loadConfig()
		assert.Error(err)
		assert.Nil(cfg)
	})

	t.Run("older versions get migrated on load", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		goBack, err := cdTmp(t)
		require.NoError(err)
		defer goBack()

		cfg := &Config{
			Version:  0,
			Efmrl:    "who-cares",
			BaseHost: "efmrl.net:8443",
		}
		err = cfg.save()
		require.NoError(err)

		canonURL := "https://google.net/"
		apiPrefix := ".e"
		md := &api2.GetEfmrlMDRes{
			CanonicalURL: canonURL,
			APIPrefix:    apiPrefix,
		}

		cfg, err = loadConfigTS(returnJSONSuccessAny(md))
		assert.NoError(err)
		require.NotNil(cfg)

		assert.Equal(canonURL, cfg.CanonURL)
		assert.Equal(apiPrefix, cfg.APIPrefix)
	})

	t.Run("pathToURL works", func(t *testing.T) {
		assert := assert.New(t)

		prefix := "dist"
		cfg := &Config{
			Version:  currentVersion,
			Efmrl:    "fire-engine",
			BaseHost: "efmrl.net:8443",
			CanonURL: "https://efmrl-abc-123-horsefeathers.horse.feathers/",
			skipLen:  len(prefix) + 1,
		}
		err = cfg.prep()

		urlStr := cfg.pathToURL("", "dist/a/b").String()
		assert.Equal("https://efmrl-abc-123-horsefeathers.horse.feathers/a/b", urlStr)

		cfg.BaseHost = ""
		cfg.Efmrl = "efmrl.net"
		urlStr = cfg.pathToURL("", "dist/index.html").String()
		assert.Equal("https://efmrl-abc-123-horsefeathers.horse.feathers/index.html", urlStr)

		cfg.Efmrl = "dev"
		urlStr = cfg.pathToURL("", "dist/index.html").String()
		assert.Equal("https://efmrl-abc-123-horsefeathers.horse.feathers/index.html", urlStr)
	})

	t.Run("old global config works", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		cleanup, err := fakeHome(t)
		require.NoError(err)
		defer cleanup()

		efmrl1 := "yan-c-bin-funhouse"
		cfg := &Config{
			Version: currentVersion,
			Efmrl:   efmrl1,
		}
		gecfg, err := cfg.getOldGlobalConfig()
		assert.NoError(err)
		require.NotNil(gecfg)

		cookie1 := "cookie1"
		strict1 := "strict1"

		gecfg.Cookie = cookie1
		gecfg.StrictCookie = strict1
		err = cfg.save()
		assert.NoError(err)

		cfg, err = loadConfig()
		assert.NoError(err)
		require.NotNil(cfg)
		gecfg, err = cfg.getOldGlobalConfig()
		assert.NoError(err)
		require.NotNil(gecfg)

		assert.Equal(cookie1, gecfg.Cookie)
		assert.Equal(strict1, gecfg.StrictCookie)

		// add a new efmrl to the mix
		efmrl2 := "playdays"
		cookie2 := "cookie2"
		strict2 := "strict2"
		cfg.Efmrl = efmrl2
		gecfg, err = cfg.getOldGlobalConfig()
		assert.NoError(err)
		require.NotNil(gecfg)
		gecfg.Cookie = cookie2
		gecfg.StrictCookie = strict2

		err = cfg.save()
		assert.NoError(err)

		cfg, err = loadConfig()
		assert.NoError(err)
		require.NotNil(cfg)
		gecfg, err = cfg.getOldGlobalConfig()
		assert.NoError(err)
		require.NotNil(gecfg)
		assert.Equal(cookie2, gecfg.Cookie)
		assert.Equal(strict2, gecfg.StrictCookie)

		cfg.Efmrl = efmrl1
		gecfg, err = cfg.getOldGlobalConfig()
		assert.NoError(err)
		require.NotNil(gecfg)
		assert.Equal(cookie1, gecfg.Cookie)
		assert.Equal(strict1, gecfg.StrictCookie)
	})

	t.Run("old global config creates iff ENOENT", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		cleanup, err := fakeHome(t)
		require.NoError(err)
		defer cleanup()

		// getOldGlobalConfig fails if efmrl not named
		cfg := &Config{
			Version: currentVersion,
		}
		gecfg, err := cfg.getOldGlobalConfig()
		assert.Error(err)
		assert.Nil(gecfg)

		cfg.Efmrl = "snakes-in-a-pit"
		gecfg, err = cfg.getOldGlobalConfig()
		assert.NoError(err)
		require.NotNil(gecfg)

		cookie := "finky"
		gecfg.Cookie = cookie

		err = cfg.save()
		assert.NoError(err)

		cfg, err = loadConfig()
		assert.NoError(err)
		require.NotNil(cfg)
		gecfg, err = cfg.getOldGlobalConfig()
		assert.NoError(err)
		require.NotNil(gecfg)
		assert.Equal(cookie, gecfg.Cookie)

		// set mode to zero, so we will get a permission error
		fpath, err := oldGlobalPath()
		assert.NoError(err)
		err = os.Chmod(fpath, 0)
		assert.NoError(err)

		cfg, err = loadConfig()
		assert.NoError(err)
		gecfg, err = cfg.getOldGlobalConfig()
		assert.Error(err)
		assert.Nil(gecfg)
	})

	t.Run("needsRewrite works as expected", func(t *testing.T) {
		type rewriteCases []struct {
			path    string
			rewrite string
			warn    bool
		}

		assert := assert.New(t)

		cfg := &Config{
			Efmrl:   "undersized-czar",
			RootDir: "_site",
			indexRewrite: map[string]bool{
				"index.html": true,
				"index.txt":  true,
			},
		}

		var cases = rewriteCases{
			{
				path:    "index.html",
				rewrite: ".",
				warn:    false,
			},
			{
				path:    "foo/index.html",
				rewrite: "foo",
				warn:    false,
			},
			{
				path:    "foo/bar/index.txt",
				rewrite: "foo/bar",
				warn:    false,
			},
			{
				path:    "foo/index.htm",
				rewrite: "",
				warn:    true,
			},
		}
		for _, c := range cases {
			rewrite, warn := cfg.needsRewrite(c.path)
			assert.Equalf(c.rewrite, rewrite, "case %#v", c)
			assert.Equal(c.warn, warn != "")
		}
	})

	t.Run("loadConfig finds efmrl2.config.js above", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		goBack, err := cdTmp(t)
		require.NoError(err)
		defer goBack()

		cfg := &Config{
			Version: currentVersion,
			Efmrl:   "monkey-willard",
		}
		err = cfg.save()
		require.NoError(err)
		cfg, err = loadConfig()
		require.NoError(err)
		require.NotNil(cfg)

		underhill := "a/b/c"
		err = os.MkdirAll(underhill, 0777)
		require.NoError(err)
		err = os.Chdir(underhill)
		require.NoError(err)

		cfg2, err := loadConfig()
		assert.NoError(err)
		require.NotNil(cfg2)
		assert.Equal(cfg, cfg2)
	})

	t.Run("global config works", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		cleanup, err := fakeHome(t)
		require.NoError(err)
		defer cleanup()

		canonURL1 := "https://efmrl-test-1.example.com/"
		cfg := &Config{
			Version:  currentVersion,
			Efmrl:    "test-efmrl-1",
			CanonURL: canonURL1,
		}
		err = cfg.prep()
		require.NoError(err)

		gecfg, err := cfg.getGlobalEfmrlConfig()
		assert.NoError(err)
		require.NotNil(gecfg)

		cookie1 := "test-cookie-1"
		strict1 := "test-strict-1"

		gecfg.Secrets.Cookie = cookie1
		gecfg.Secrets.StrictCookie = strict1
		err = cfg.save()
		assert.NoError(err)

		cfg, err = loadConfig()
		assert.NoError(err)
		require.NotNil(cfg)
		gecfg, err = cfg.getGlobalEfmrlConfig()
		assert.NoError(err)
		require.NotNil(gecfg)

		assert.Equal(cookie1, gecfg.Secrets.Cookie)
		assert.Equal(strict1, gecfg.Secrets.StrictCookie)

		// add a new efmrl to the mix
		canonURL2 := "https://efmrl-test-2.example.com/"
		cookie2 := "test-cookie-2"
		strict2 := "test-strict-2"
		cfg.Efmrl = "test-efmrl-2"
		cfg.CanonURL = canonURL2
		err = cfg.prep()
		require.NoError(err)

		gecfg, err = cfg.getGlobalEfmrlConfig()
		assert.NoError(err)
		require.NotNil(gecfg)
		gecfg.Secrets.Cookie = cookie2
		gecfg.Secrets.StrictCookie = strict2

		err = cfg.save()
		assert.NoError(err)

		cfg, err = loadConfig()
		assert.NoError(err)
		require.NotNil(cfg)
		gecfg, err = cfg.getGlobalEfmrlConfig()
		assert.NoError(err)
		require.NotNil(gecfg)
		assert.Equal(cookie2, gecfg.Secrets.Cookie)
		assert.Equal(strict2, gecfg.Secrets.StrictCookie)

		cfg.CanonURL = canonURL1
		err = cfg.prep()
		require.NoError(err)
		gecfg, err = cfg.getGlobalEfmrlConfig()
		assert.NoError(err)
		require.NotNil(gecfg)
		assert.Equal(cookie1, gecfg.Secrets.Cookie)
		assert.Equal(strict1, gecfg.Secrets.StrictCookie)
	})

	t.Run("global config creates iff ENOENT", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		cleanup, err := fakeHome(t)
		require.NoError(err)
		defer cleanup()

		// getGlobalEfmrlConfig fails if canonURL not set
		cfg := &Config{
			Version: currentVersion,
		}
		gecfg, err := cfg.getGlobalEfmrlConfig()
		assert.Error(err)
		assert.Nil(gecfg)

		canonURL := "https://efmrl-test.example.com/"
		cfg.CanonURL = canonURL
		err = cfg.prep()
		require.NoError(err)

		gecfg, err = cfg.getGlobalEfmrlConfig()
		assert.NoError(err)
		require.NotNil(gecfg)

		cookie := "test-cookie"
		gecfg.Secrets.Cookie = cookie

		err = cfg.save()
		assert.NoError(err)

		cfg, err = loadConfig()
		assert.NoError(err)
		require.NotNil(cfg)
		gecfg, err = cfg.getGlobalEfmrlConfig()
		assert.NoError(err)
		require.NotNil(gecfg)
		assert.Equal(cookie, gecfg.Secrets.Cookie)

		// set mode to zero, so we will get a permission error
		fpath, err := globalPath()
		assert.NoError(err)
		err = os.Chmod(fpath, 0)
		assert.NoError(err)

		cfg, err = loadConfig()
		assert.NoError(err)
		gecfg, err = cfg.getGlobalEfmrlConfig()
		assert.Error(err)
		assert.Nil(gecfg)
	})
}
