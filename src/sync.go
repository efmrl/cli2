package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	contentTypeHeader  = "Content-Type"
	cacheControlHeader = "Cache-Control"
	defaultCache       = "no-cache"
	defaultMaxRetries  = 12

	syncDist = iota
	syncVersion
)

// SyncCommon holds common parts between "sync" and "version"
type SyncCommon struct {
	Parallel int  `default:"1" short:"p" help:"how many files to upload at once"`
	DryRun   bool `short:"n" help:"show files that would be pushed without pushing them"`
	Force    bool `short:"f" help:"force sync; don't skip even if file is unchanged"`

	rewriteWarn sync.Once
	quiet       bool             // copied from Context
	debug       bool             // copied from Context
	ts          *httptest.Server // copied to Config
}

// SyncCmd holds the options for the "sync" subcommand
type SyncCmd struct {
	SyncCommon
	DeleteOthers bool `short:"D" help:"delete files on server that are not in local directory"`
}

type seenMap map[string]struct{}

// Run the "sync" subcommand
func (sync *SyncCmd) Run(ctx *Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfg.skipLen = len(cfg.RootDir) + 1 // +1 for '/' separator
	sync.quiet = ctx.Quiet
	sync.debug = ctx.Debug
	cfg.ts = sync.ts
	seen := seenMap{}
	if sync.DeleteOthers {
		err = setSeenMap(cfg, ctx, seen)
	}

	err = sync.syncDir(cfg, "", syncDist, seen)
	if err != nil {
		return err
	}

	if sync.DeleteOthers {
		err = deleteFromSeenMap(cfg, ctx, seen, sync.DryRun)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *SyncCommon) syncDir(
	cfg *Config,
	urlPrefix string,
	which int,
	seen seenMap,
) error {
	type workItem struct {
		path    string
		dirPath string
		info    os.FileInfo
	}

	var path string
	switch which {
	case syncDist:
		path = cfg.RootDir
	case syncVersion:
		path = cfg.VersionDir
	default:
		panic("bad which in syncDir")
	}

	err := cfg.setIndexFileNames()
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(context.Background())
	items := make(chan *workItem)

	g.Go(func() error {
		defer close(items)
		return filepath.Walk(
			path,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.Mode().IsRegular() {
					return nil
				}

				dirPath, warn := cfg.needsRewrite(path, which)
				if warn != "" && !s.quiet {
					s.rewriteWarn.Do(func() {
						fmt.Println(warn)
					})
				}

				item := &workItem{
					path:    path,
					dirPath: dirPath,
					info:    info,
				}

				select {
				case items <- item:
				case <-ctx.Done():
					return ctx.Err()
				}
				return nil
			})
	})

	seenChan := make(chan string)
	for i := 0; i < s.Parallel; i++ {
		g.Go(func() error {
			client, err := cfg.getClient()
			if err != nil {
				return err
			}
			for item := range items {
				path := item.path
				if item.dirPath != "" {
					path = item.dirPath
				}

				seenChan <- path
				url := cfg.pathToURL(urlPrefix, path).String()
				if !s.Force {
					res, err := client.Head(url)
					switch {
					case err != nil:
						return err
					case res.StatusCode == http.StatusNotFound:
					case res.StatusCode != http.StatusOK:
						res.Body.Close()
						return fmt.Errorf(
							"HEAD %v failed: %v",
							url,
							res.Status,
						)
					}
					defer res.Body.Close()

					existingEtag := res.Header.Get("ETag")
					res = nil
					multiPart := etagToMultipart(existingEtag)
					etag, err := etag(item.path, multiPart)
					if err != nil {
						return err
					}

					if existingEtag == etag {
						continue
					}
				}

				contentType, err := cfg.contentType(item.path)
				if err != nil {
					return err
				}

				if !s.quiet {
					fmt.Printf("PUT %v\n", url)
				}
				if s.DryRun {
					continue
				}

				err = s.put(
					client,
					item.path,
					item.info,
					contentType,
					url,
					os.Stdout,
				)
				if err != nil {
					return fmt.Errorf(
						"cannot push file %q to efmrl.com: %w",
						item.path,
						err,
					)
				}
			}
			return nil
		})
	}

	go func() {
		g.Wait()
		close(seenChan)
	}()

	for path := range seenChan {
		if len(path) < cfg.skipLen {
			// This cannot happen. If it does, delete nothing.
			for k := range seen {
				delete(seen, k)
			}
			continue
		}

		if s.debug {
			fmt.Printf("removing from seen: %q\n", path[cfg.skipLen:])
		}
		delete(seen, path[cfg.skipLen:])
	}

	return g.Wait()
}

func etag(path string, parts int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("cannot read for MD5: %w", err)
	}
	md5bytes := h.Sum(nil)
	if parts < 1 {
		return hex.EncodeToString(md5bytes), nil
	}

	metaHash := &bytes.Buffer{}
	metaHash.Write(md5bytes)
	metaHash = bytes.NewBuffer(metaHash.Bytes())
	md5Final := md5.New()
	if _, err := io.Copy(md5Final, metaHash); err != nil {
		return "", fmt.Errorf("cannot read metahash: %w", err)
	}
	finalBytes := md5Final.Sum(nil)
	return fmt.Sprintf("%x-%v", finalBytes, parts), nil
}

func etagToMultipart(etag string) int {
	etParts := strings.Split(etag, "-")
	if len(etParts) < 2 {
		return 0
	}
	parts, err := strconv.Atoi(etParts[1])
	if err != nil {
		return 0
	}

	return parts
}

func setSeenMap(
	cfg *Config,
	ctx *Context,
	seen seenMap,
) error {
	// get existing files
	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	s3files := &db.S3Files{}
	url := cfg.pathToAPIurl("l")
	res, err := httpGetJSON(client, url, s3files)
	if err != nil {
		return fmt.Errorf("cannot list files on server: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"status %v when listing files from server",
			res.Status,
		)
	}

	// make a list of existing files
	for _, s3file := range s3files.S3Files {
		if ctx.Debug {
			fmt.Printf("adding to seenMap: %q\n", s3file.Path)
		}
		seen[s3file.Path] = struct{}{}
	}

	return nil
}

func deleteFromSeenMap(
	cfg *Config,
	ctx *Context,
	seen seenMap,
	dryRun bool,
) error {
	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	var cfgCopy Config
	cfgCopy = *cfg
	cfgCopy.skipLen = 0

	for fname := range seen {
		url := cfgCopy.pathToURL("", fname)
		if !ctx.Quiet {
			fmt.Printf("DELETE %v\n", url)
		}
		if dryRun {
			continue
		}

		req, err := http.NewRequest("DELETE", url.String(), nil)
		if err != nil {
			return fmt.Errorf("cannot build delete request: %w", err)
		}

		res, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("cannot send delete request: %w", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("status %v when deleting %q", res.Status, fname)
		}
	}

	return nil
}

func (s *SyncCommon) put(
	client *http.Client,
	srcPath string,
	fileinfo os.FileInfo,
	contentType string,
	urlPath string,
	out io.Writer,
) error {
	var (
		src *os.File
		err error
	)

	maxRetries := defaultMaxRetries

	defer func() {
		if src != nil {
			src.Close()
		}
	}()
DoPut:
	src, err = os.Open(srcPath)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", urlPath, src)
	if err != nil {
		return err
	}
	req.Header.Set(contentTypeHeader, contentType)
	req.Header.Set(cacheControlHeader, defaultCache)
	req.ContentLength = fileinfo.Size()
	req.GetBody = func() (io.ReadCloser, error) {
		return ioutil.NopCloser(src), nil
	}

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	maxRetries--
	switch {
	case res.StatusCode == http.StatusOK:
	case maxRetries == 0:
		return fmt.Errorf("too many retries; try again later")
	case res.StatusCode == http.StatusTooManyRequests:
		if !s.quiet {
			fmt.Fprintln(out, "efmrl migrating to dedicated storage; retrying")
		}
		src.Close()
		src = nil
		time.Sleep(5 * time.Second)
		goto DoPut

	default:
		return fmt.Errorf("failed: received status %v", res.StatusCode)
	}

	return nil
}
