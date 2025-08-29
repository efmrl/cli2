package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/efmrl/api2"

	"golang.org/x/net/publicsuffix"
)

const (
	// configName is the name of the config file
	configName = "efmrl2.config.js"
	// globalConfigName is where the global config file is kept under the user's
	// home directory.
	globalConfigName = ".config/efmrl2/global_config.js"
	// oldGlobalConfigName is where the old format of the global config file
	// used to be kept
	oldGlobalConfigName = ".config/efmrl2/secrets.js"
	// defaultHost is used if no hostname is given
	defaultHost = "efmrl.net:8443"
	// contentTypeBytes is how many bytes max we read
	// to heuristically determine mime type
	contentTypeBytes = 512
)

// versions of the config file
const (
	// currentVersion is the version of the config file that we write. When we
	// read a config, and the version is less than our current version, we need
	// to migrate the data. When the version is greater than our current
	// version, we error out.
	currentVersion = versionCanonURL

	versionCanonURL = iota
)

// versions of the global config file
const (
	// currentGlobalVersion is the version of the global config file that we
	// write.
	currentGlobalConfigVersion = globalVersionInitial

	globalVersionInitial = iota
)

var (
	wantsRewrite = map[string]bool{
		"index.html": true,
		"index.htm":  true,
		"index.text": true,
		"index.txt":  true,
	}
)

// Config holds data about a given efmrl. It is suitable to check in to source
// control.
type Config struct {
	Version int `json:"version,omitempty"`

	Efmrl     string `json:"efmrl"`
	CanonURL  string `json:"canonURL"`
	APIPrefix string `json:"apiPrefix"`
	BaseHost  string `json:"base_host,omitempty"`
	Insecure  bool   `json:"insecure,omitempty"`

	RootDir string `json:"root_dir"`

	// canonURL is a parsed version of CanonURL
	canonURL *url.URL

	// keep track of which index files to rewrite as their directory names
	IndexRewrite   []string        `json:"index_rewrite,omitempty"`
	indexRewrite   map[string]bool // shadow of IndexRewrite
	IndexNoRewrite []string        `json:"index_no_rewrite,omitempty"`
	indexNoRewrite map[string]bool // shadow of IndexNoRewrite

	// skipLen is used by pathToURL: if nonzero, it skips the first skipLen
	// characters in the path
	skipLen int

	//gcfg is a cached value of the global config
	gcfg *GlobalConfig
	// oldGCFG is a cached value of the global config
	oldGCFG OldGlobalConfig

	// ts is an httptest.Server, to override client behaviors
	ts *httptest.Server
}

type GlobalConfig struct {
	Version int `json:"version"`

	// Efmrls is a map from Canonical URL to GlobalEfmrlConfigs
	Efmrls map[string]*GlobalEfmrlConfig `json:"efmrls,omitempty"`
}

type GlobalEfmrlConfig struct {
	Version int `json:"version"`

	Secrets EfmrlSecrets `json:"secrets,omitempty"`
}

// EfmrlSecrets holds per-efmrl data that we don't want checked in to
// source control as efmrl2.config.js is
type EfmrlSecrets struct {
	Cookie       string `json:"cookie,omitempty"`
	StrictCookie string `json:"strict_cookie,omitempty"`
}

// OldGlobalConfig holds the configs by efmrl name
type OldGlobalConfig map[string]*EfmrlSecrets

func findConfig() (string, string, error) {
	dpath, err := filepath.Abs(".")
	if err != nil {
		return "", "", err
	}

	for {
		fpath := filepath.Join(dpath, configName)
		if _, err := os.Stat(fpath); err == nil {
			return fpath, dpath, nil
		}

		if dpath == "/" {
			return "", "", fmt.Errorf("cannot find %q", configName)
		}
		dpath = filepath.Dir(dpath)
	}
}

func loadConfig() (*Config, error) {
	return loadConfigTS(nil)
}

// loadConfigTS loads the config file. ts is a test server, and may be nil.
// pass in a non-nil ts to set the ts value of the config early in the loading
// phase (e.g. to test migration).
func loadConfigTS(ts *httptest.Server) (*Config, error) {
	fpath, dpath, err := findConfig()
	if err != nil {
		return nil, err
	}
	err = os.Chdir(dpath)
	if err != nil {
		return nil, err
	}

	cfgBytes, err := os.ReadFile(fpath)
	if err != nil {
		return nil, fmt.Errorf("cannot load config: %w", err)
	}

	cfg := &Config{
		indexRewrite:   map[string]bool{},
		indexNoRewrite: map[string]bool{},
		ts:             ts,
	}
	err = json.Unmarshal(cfgBytes, cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot parse config: %w", err)
	}

	if cfg.Version > currentVersion {
		return nil, fmt.Errorf("config file not understood; upgrade efmrl")
	}
	err = cfg.migrate()
	if err != nil {
		return nil, err
	}

	err = cfg.prep()
	if err != nil {
		return nil, err
	}

	for _, index := range cfg.IndexRewrite {
		cfg.indexRewrite[index] = true
	}
	for _, index := range cfg.IndexNoRewrite {
		cfg.indexNoRewrite[index] = true
	}

	return cfg, nil
}

func (cfg *Config) prep() error {
	var err error
	cfg.canonURL, err = url.Parse(cfg.CanonURL)
	if err != nil {
		err = fmt.Errorf("cannot parse url %q: %w", cfg.CanonURL, err)
		return err
	}

	return nil
}

func (cfg *Config) save() error {
	cfg.IndexRewrite = make([]string, len(cfg.indexRewrite))
	i := 0
	for fname := range cfg.indexRewrite {
		cfg.IndexRewrite[i] = fname
		i++
	}
	cfg.IndexNoRewrite = make([]string, len(cfg.indexNoRewrite))
	i = 0
	for fname := range cfg.indexNoRewrite {
		cfg.IndexNoRewrite[i] = fname
		i++
	}

	cfgBytes, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	cfgBytes = append(cfgBytes, '\n')

	err = os.WriteFile(configName, cfgBytes, 0666)
	if err != nil {
		return fmt.Errorf("cannot write config file: %w", err)
	}

	if cfg.gcfg != nil {
		err = cfg.gcfg.save()
		if err != nil {
			return fmt.Errorf("cannot save global config: %w", err)
		}
	}
	if cfg.oldGCFG != nil {
		err = cfg.oldGCFG.save()
		if err != nil {
			return fmt.Errorf("cannot save old global config: %w", err)
		}
	}

	return nil
}

func (cfg *Config) migrate() error {
	if cfg.Version < versionCanonURL {
		if err := cfg.getCanonURL(); err != nil {
			return err
		}
		cfg.Version = versionCanonURL
	}

	return nil
}

func (cfg *Config) getCanonURL() error {
	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	url := cfg.pathToMDURL()
	md := &api2.GetEfmrlMDRes{}
	err = getJSON(client, url, api2.NewResult(md))
	if err != nil {
		return err
	}

	cfg.CanonURL = md.CanonicalURL
	cfg.APIPrefix = md.APIPrefix

	return nil
}

func (cfg *Config) getGlobalEfmrlConfig() (*GlobalEfmrlConfig, error) {
	if cfg.CanonURL == "" {
		return nil, fmt.Errorf("efmrl url is not set")
	}

	var err error
	if cfg.gcfg == nil {
		cfg.gcfg, err = loadGlobalConfig()
		if err != nil {
			return nil, err
		}
	}

	gecfg, ok := cfg.gcfg.Efmrls[cfg.CanonURL]
	if !ok {
		gecfg = &GlobalEfmrlConfig{}
		cfg.gcfg.Efmrls[cfg.CanonURL] = gecfg
	}

	return gecfg, nil
}

// getOldGlobalConfig returns the GlobalEfmrlConfig. Efmrl must be set.
func (cfg *Config) getOldGlobalConfig() (*EfmrlSecrets, error) {
	if cfg.Efmrl == "" {
		return nil, fmt.Errorf("efmrl name is not set")
	}

	var err error
	if cfg.oldGCFG == nil {
		cfg.oldGCFG, err = loadOldGlobalConfig()
		if err != nil {
			return nil, err
		}
	}

	gecfg, ok := cfg.oldGCFG[cfg.Efmrl]
	if !ok {
		gecfg = &EfmrlSecrets{}
		cfg.oldGCFG[cfg.Efmrl] = gecfg
	}

	return gecfg, nil
}

func loadGlobalConfig() (*GlobalConfig, error) {
	fpath, err := globalPath()
	if err != nil {
		return nil, err
	}

	gcfgBytes, err := os.ReadFile(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			return &GlobalConfig{}, nil
		}
		err = fmt.Errorf(
			"cannot load global config %q: %w",
			fpath,
			err,
		)
		return nil, err
	}

	gcfg := &GlobalConfig{}
	err = json.Unmarshal(gcfgBytes, gcfg)
	if err != nil {
		err = fmt.Errorf(
			"cannot parse global config %q: %w",
			fpath,
			err,
		)
		return nil, err
	}

	return gcfg, nil
}

func loadOldGlobalConfig() (OldGlobalConfig, error) {
	fpath, err := oldGlobalPath()
	if err != nil {
		return nil, err
	}

	gcfgBytes, err := os.ReadFile(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			return OldGlobalConfig{}, nil
		}
		return nil, fmt.Errorf(
			"cannot load global config %q: %w",
			fpath,
			err,
		)
	}

	gcfg := &OldGlobalConfig{}
	err = json.Unmarshal(gcfgBytes, gcfg)
	if err != nil {
		return nil, fmt.Errorf("cannot parse global config %q: %w", fpath, err)
	}

	return *gcfg, nil
}

func (gcfg GlobalConfig) save() error {
	fpath, err := globalPath()
	if err != nil {
		return err
	}
	gcfgBytes, err := json.MarshalIndent(gcfg, "", "    ")
	if err != nil {
		return err
	}
	gcfgBytes = append(gcfgBytes, '\n')

	_, err = os.Stat(fpath)
	if os.IsNotExist(err) {
		dirName := filepath.Dir(fpath)
		parentName := filepath.Dir(dirName)
		err = os.MkdirAll(parentName, 0777)
		if err != nil {
			return err
		}
		err = os.MkdirAll(dirName, 0700)
		if err != nil {
			return err
		}
	}

	err = os.WriteFile(fpath, gcfgBytes, 0600)
	if err != nil {
		return fmt.Errorf(
			"cannot write global config %q: %w",
			fpath,
			err,
		)
	}

	return nil
}

func (gcfg OldGlobalConfig) save() error {
	fpath, err := oldGlobalPath()
	if err != nil {
		return err
	}

	gcfgBytes, err := json.MarshalIndent(gcfg, "", "    ")
	if err != nil {
		return err
	}
	gcfgBytes = append(gcfgBytes, '\n')

	_, err = os.Stat(fpath)
	if os.IsNotExist(err) {
		dirName := filepath.Dir(fpath)
		parentName := filepath.Dir(dirName)
		err = os.MkdirAll(parentName, 0777)
		if err != nil {
			return err
		}
		err = os.MkdirAll(dirName, 0700)
		if err != nil {
			return err
		}
	}

	err = os.WriteFile(fpath, gcfgBytes, 0600)
	if err != nil {
		return fmt.Errorf(
			"cannot write old global config %q: %w",
			fpath,
			err,
		)
	}

	return nil
}

func (cfg *Config) hostPart() string {
	if cfg.ts != nil {
		purl, err := url.Parse(cfg.ts.URL)
		if err != nil {
			panic(err)
		}
		return purl.Host
	}

	baseHost := cfg.BaseHost
	if baseHost == "" {
		baseHost = defaultHost
	}

	hostName := baseHost
	if c := strings.IndexRune(hostName, ':'); c > 0 {
		hostName = hostName[:c]
	}
	if cfg.Efmrl == hostName {
		return baseHost
	}

	return fmt.Sprintf("%v.%v", cfg.Efmrl, baseHost)
}

// pathToMDURL is special, because it doesn't depend on the CanonURL to be
// there. It is used when setting up a new project, when changing efmrls, or
// when migrating; or, any other time you need to set CanonURL.
func (cfg *Config) pathToMDURL() *url.URL {
	url := &url.URL{
		Scheme: "https",
		Host:   cfg.hostPart(),
		Path:   filepath.Join(api2.DefaultAPIPrefix, "rest", "md"),
	}

	return url
}

func (cfg *Config) pathToAPIurl(path string) *url.URL {
	url := &url.URL{}
	*url = *cfg.canonURL

	url.Path = filepath.Join(api2.DefaultAPIPrefix, "rest", path)

	return url
}

func (cfg *Config) pathToAdminURL(path string) *url.URL {
	url := &url.URL{}
	*url = *cfg.canonURL

	url.Path = filepath.Join(api2.DefaultAPIPrefix, path)

	return url
}

func (cfg *Config) pathToURL(prefix, path string) *url.URL {
	if len(path) < cfg.skipLen {
		panic("stopping")
	}
	if cfg.skipLen > 0 {
		path = path[cfg.skipLen:]
	}

	if prefix != "" {
		path = filepath.Join(prefix, path)
	}

	u := &url.URL{}
	switch {
	case cfg.canonURL != nil:
		*u = *cfg.canonURL
	case cfg.CanonURL != "":
		err := cfg.prep()
		if err != nil {
			return nil
		}
		*u = *cfg.canonURL
	default:
		u = &url.URL{
			Scheme: "https",
			Host:   cfg.hostPart(),
		}
	}
	u.Path = path

	return u
}

func (gecfg *EfmrlSecrets) eatCookie(cookie *http.Cookie) bool {
	switch cookie.Name {
	case api2.SessionCookieName:
		gecfg.Cookie = cookie.Value
		return true
	case api2.StrictCookieName:
		gecfg.StrictCookie = cookie.Value
		return true
	}

	return false
}

func (gecfg *EfmrlSecrets) eatAllCookies(
	client *http.Client,
	url *url.URL,
) bool {
	var success bool
	url.Path = ""

	for _, cookie := range client.Jar.Cookies(url) {
		if gecfg.eatCookie(cookie) {
			success = true
		}
	}

	return success
}

func (cfg *Config) getClient() (*http.Client, error) {
	jar, err := getJar(cfg)
	if err != nil {
		return nil, err
	}

	if cfg.ts != nil {
		return cfg.getTestClient(jar)
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.Insecure,
			},
		},
		Jar: jar,
	}, nil
}

func getJar(cfg *Config) (*cookiejar.Jar, error) {
	options := &cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	}
	if cfg.ts != nil {
		options.PublicSuffixList = nil
	}
	jar, err := cookiejar.New(options)
	if err != nil {
		return nil, err
	}

	gecfg, err := cfg.getOldGlobalConfig()
	if err != nil {
		return nil, err
	}

	if gecfg.Cookie == "" {
		return jar, nil
	}

	var ccfg = *cfg
	ccfg.skipLen = 0
	u := ccfg.pathToURL("", "")
	u.Path = ""

	jar.SetCookies(u, []*http.Cookie{
		{
			Name:  api2.SessionCookieName,
			Value: gecfg.Cookie,
		},
		{
			Name:  api2.StrictCookieName,
			Value: gecfg.StrictCookie,
		},
	})

	return jar, nil
}

// needsRewrite returns two strings: a path-to-rewrite-to, and a warning
// the rewrite path will be nonempty if the path should be rewritten
// the warning will be nonempty if the path is a candidate for rewrtiting, but
// is unspecified whether or not to rewrite
func (cfg *Config) needsRewrite(path string) (rewrite, warn string) {
	fname := filepath.Base(path)
	if cfg.indexNoRewrite[fname] {
		return
	}

	dpath := filepath.Dir(path)
	if cfg.indexRewrite[fname] {
		rewrite = dpath
		return
	}

	if wantsRewrite[fname] {
		warn = fmt.Sprintf(
			`warning: %q is a candidate for a directory index file
If you want to rewrite %q as %q, do this:
    efmrl set --rewrite %v
If you do not want to rewrite %q, do this:
    efmrl set --no-rewrite %v
`, path, path, dpath, fname, path, fname)
	}
	return
}

// contentType tries to determine the mime type for the given path
// It uses the file extension if there is one. Otherwise, it reads the first
// contentTypeBytes bytes to determine the type.
func (cfg *Config) contentType(path string) (string, error) {
	ext := filepath.Ext(path)
	if ext != "" {
		return mime.TypeByExtension(ext), nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf(
			"can't open %q to determine mime type: %w",
			path,
			err,
		)
	}
	defer f.Close()

	mdata := make([]byte, contentTypeBytes)
	numBytes, err := f.Read(mdata)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf(
			"can't read %q to determine mime type: %w",
			path,
			err,
		)
	}

	return http.DetectContentType(mdata[:numBytes]), nil
}

func (cfg *Config) getTestClient(jar *cookiejar.Jar) (*http.Client, error) {
	client := cfg.ts.Client()
	client.Jar = jar

	return client, nil
}

// globalPath returns the path to the global config file, with the user's
// home directory prepended.
func globalPath() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, globalConfigName), nil
}

// oldGlobalPath returns the path to the global config file, with the user's
// home directory prepended.
func oldGlobalPath() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, oldGlobalConfigName), nil
}

func homeDir() (string, error) {
	home := os.Getenv("HOME")
	if home != "" {
		return home, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	return usr.HomeDir, nil
}
