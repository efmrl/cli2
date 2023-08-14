package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/textproto"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/efmrl/api2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSync(t *testing.T) {
	goBack, err := cdTmp(t)
	require.NoError(t, err)
	defer goBack()

	ss := &syncServer{
		root: "cloud",
	}
	err = os.MkdirAll(ss.root, 0777)
	require.NoError(t, err)
	ts := newTestServer(ss)
	defer ts.Close()

	cfg := &Config{}
	ctx := &CLIContext{
		Quiet: true,
	}

	t.Run("SyncCmd.Run() syncs ground to cloud", func(t *testing.T) {
		assert := assert.New(t)
		require := require.New(t)

		ground := "ground"
		cfg.RootDir = ground
		cfg.Efmrl = "domino"
		err := cfg.save()
		require.NoError(err)

		testData := []string{
			"index.html",
			"css/main.css",
			"js/bundle.js",
		}
		err = makeFake(ground, testData)
		require.NoError(err)

		sync := SyncCmd{
			SyncCommon: SyncCommon{
				Parallel: 3,
				ts:       ts,
			},
			DeleteOthers: true,
		}

		// require migration
		ss.migrateFor(6 * time.Second)

		err = sync.Run(ctx)
		assert.NoError(err)

		desired, err := fileTreeToMap(ground)
		require.NoError(err)
		actual, err := ss.getFileTreeMap()
		assert.NoError(err)
		require.NotNil(actual)
		assert.Equal(desired, actual)

		fpath := filepath.Join(ground, "stinkum")
		err = os.WriteFile(fpath, []byte("stuffers"), 0666)
		require.NoError(err)
		fpath = filepath.Join(ground, testData[0])
		err = os.Remove(fpath)
		require.NoError(err)

		err = sync.Run(ctx)
		assert.NoError(err)
		desired, err = fileTreeToMap(ground)
		require.NoError(err)
		actual, err = ss.getFileTreeMap()
		require.NoError(err)

		assert.Equal(desired, actual)
	})
}

type syncServer struct {
	sync.Mutex
	root              string
	needsMigrate      bool
	needsMigrateUntil time.Time
	indexFileNames    []string
}

func (ss *syncServer) migrateFor(length time.Duration) {
	ss.Lock()
	defer ss.Unlock()

	ss.needsMigrate = true
	ss.needsMigrateUntil = time.Now().Add(length)
}

func (ss *syncServer) checkMigrate() bool {
	ss.Lock()
	defer ss.Unlock()

	if !ss.needsMigrate {
		return false
	}

	now := time.Now()
	if ss.needsMigrateUntil.Before(now) {
		ss.needsMigrate = false
		return false
	}

	return true
}

func (ss *syncServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "PUT":
		ss.putFile(w, r)
		return
	case "HEAD":
		ss.headFile(w, r)
		return
	case "DELETE":
		ss.deleteFile(w, r)
		return
	case "GET":
		switch {
		case strings.HasSuffix(r.URL.Path, "/l"):
			ss.listFiles(w, r)
		case strings.HasSuffix(r.URL.Path, "/g/efmrl/indexes"):
			ss.listIndexes(w, r)
		}
		return
	}
	w.WriteHeader(http.StatusBadRequest)
}

func (ss *syncServer) putFile(w http.ResponseWriter, r *http.Request) {
	if ss.checkMigrate() {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	fpath := filepath.Join(ss.root, r.URL.Path)
	if fpath == r.URL.Path {
		log.Printf("absolute path: %q", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	dirPath := path.Dir(fpath)
	err := os.MkdirAll(dirPath, 0777)
	if err != nil {
		log.Panicf("error in MkdirAll: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	f, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Panicf("putFile: error opening %q: %v", fpath, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer f.Close()
	_, err = io.Copy(f, r.Body)
	if err != nil {
		log.Panicf("putFile: error writing %q: %v", fpath, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (ss *syncServer) headFile(w http.ResponseWriter, r *http.Request) {
	headerETag := textproto.CanonicalMIMEHeaderKey("ETag")
	fpath := filepath.Join(ss.root, r.URL.Path)
	f, err := os.Open(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		log.Printf("headFile: error opening %q: %v", fpath, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer f.Close()
	e, err := etag(fpath, 0)
	if err != nil {
		log.Panicf("headFile: error computing ETag %q: %v", fpath, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set(headerETag, e)
	w.WriteHeader(http.StatusOK)
}

func (ss *syncServer) deleteFile(w http.ResponseWriter, r *http.Request) {
	if ss.checkMigrate() {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	fpath := filepath.Join(ss.root, r.URL.Path)
	err := os.Remove(fpath)
	if err != nil {
		log.Panicf("deleteFile: error removing %q: %v", fpath, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (ss *syncServer) listFiles(w http.ResponseWriter, r *http.Request) {
	skipLen := len(ss.root) + 1 // +1 for separating '/'
	res := &api2.ListFilesRes{}
	err := filepath.Walk(
		ss.root,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			s3File := &api2.FileInfo{
				//Path: path[skipLen:],
			}
			res.Files[path[skipLen:]] = s3File
			return nil
		})
	if err != nil {
		log.Panicf("listFiles: error walking tree: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	resBytes, err := json.Marshal(res)
	if err != nil {
		log.Panicf("listFiles: error marshalling result: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(resBytes)
}

func (ss *syncServer) listIndexes(w http.ResponseWriter, r *http.Request) {
	res, err := json.Marshal(ss.indexFileNames)
	if err != nil {
		log.Panicf("listIndexes: error marshalling result: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func (ss *syncServer) getFileTreeMap() (fileTreeMap, error) {
	return fileTreeToMap(ss.root)
}
