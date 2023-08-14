package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// newTestServer starts a TLS-based httptest.Server. It is the caller's
// responsibility to Close().
func newTestServer(h http.Handler) *httptest.Server {
	ts := httptest.NewUnstartedServer(h)
	ts.StartTLS()
	return ts
}

type fileTreeMap map[string]string // path to etag

func fileTreeToMap(path string) (fileTreeMap, error) {
	skipLen := len(path) + 1 // +1 for trailing '/'
	m := fileTreeMap{}
	err := filepath.Walk(
		path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}

			e, err := etag(path, 0)
			if err != nil {
				return err
			}
			m[path[skipLen:]] = e
			return nil
		})
	if err != nil {
		return nil, err
	}
	return m, nil
}

func fakeHome(t *testing.T) (func(), error) {
	curHome := os.Getenv("HOME")

	cleanup := func() {
		err := os.Setenv("HOME", curHome)
		assert.NoError(t, err)
	}

	err := os.Setenv("HOME", t.TempDir())

	return cleanup, err
}

func cdTmp(t *testing.T) (func(), error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	err = os.Chdir(t.TempDir())
	if err != nil {
		return nil, err
	}

	return func() {
		err := os.Chdir(wd)
		assert.NoError(t, err)
	}, nil
}

func makeFake(dirname string, testData []string) error {
	for _, fpath := range testData {
		fpath = filepath.Join(dirname, fpath)
		dirPath := path.Dir(fpath)
		err := os.MkdirAll(dirPath, 0777)
		if err != nil {
			return err
		}
		err = os.WriteFile(fpath, []byte(fpath), 0666)
		if err != nil {
			return err
		}
	}

	return nil
}
