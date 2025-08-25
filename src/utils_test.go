package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/efmrl/api2"
	"github.com/stretchr/testify/assert"
)

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

func returnJSONSuccessAny(res any) *httptest.Server {
	f := func(w http.ResponseWriter, r *http.Request) {
		enc := json.NewEncoder(w)
		err := enc.Encode(api2.NewSuccessAny(res))
		if err != nil {
			panic(err)
		}
	}

	return httptest.NewTLSServer(http.HandlerFunc(f))
}
