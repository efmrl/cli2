package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/efmrl/api2"

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

// SyncCmd holds common parts between "sync" and "version"
type SyncCmd struct {
	Parallel     int  `default:"1" short:"p" help:"how many files to upload at once"`
	DryRun       bool `short:"n" help:"show files that would be pushed without pushing them"`
	Force        bool `short:"f" help:"force sync; don't skip even if file is unchanged"`
	DeleteOthers bool `short:"D" help:"delete files on server that are not in local directory"`
	Debug        bool `help:"add debugging output"`
	MaxFiles     int  `hidden:""`

	rewriteWarn sync.Once
	quiet       bool             // copied from Context
	verbose     bool             // copied from Context
	debug       bool             // copied from Context
	ts          *httptest.Server // copied to Config
}

type seenMap map[string]*atomic.Pointer[api2.FileInfo]

// Run the "sync" subcommand
func (sync *SyncCmd) Run(ctx *CLIContext) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	msg, err := loggedIn(cfg)
	if err != nil {
		return err
	}
	if msg != "" {
		fmt.Println(msg)
		return nil
	}

	cfg.skipLen = len(cfg.RootDir) + 1 // +1 for '/' separator
	sync.quiet = ctx.Quiet
	ctx.Debug = sync.Debug
	sync.debug = ctx.Debug || sync.Debug
	cfg.ts = sync.ts
	seen := seenMap{}
	if sync.DeleteOthers || !sync.Force {
		err = setSeenMap(cfg, ctx, seen, sync.MaxFiles)
		if err != nil {
			return err
		}
	}

	err = sync.syncDir(cfg, "", seen)
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

func (s *SyncCmd) syncDir(
	cfg *Config,
	urlPrefix string,
	seen seenMap,
) error {
	type workItem struct {
		path    string
		dirPath string
		info    os.FileInfo
	}

	var path string
	path = cfg.RootDir

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

				dirPath, warn := cfg.needsRewrite(path)
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

				if len(path) <= cfg.skipLen {
					path += "//"
				}
				p := seen[path[cfg.skipLen:]]
				if s.debug {
					fmt.Printf("seen %q? %v\n", path[cfg.skipLen:], p != nil)
				}
				if p != nil {
					if !s.Force {
						fi := p.Load()
						p.Store(nil)
						if fi.ETAG[:1] == "\"" {
							l := len(fi.ETAG)
							fi.ETAG = fi.ETAG[1 : l-1]
						}
						multiPart := etagToMultipart(fi.ETAG)
						etag, err := etag(item.path, multiPart)
						if err != nil {
							return err
						}
						if fi.ETAG == etag {
							continue
						}
						if s.debug {
							fmt.Printf("cloud %q != local %q\n", fi.ETAG, etag)
						}
					}
				}
				url := cfg.pathToURL(urlPrefix, path).String()

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

	return g.Wait()
}

func setSeenMap(
	cfg *Config,
	ctx *CLIContext,
	seen seenMap,
	maxFiles int,
) error {
	// get existing files
	client, err := cfg.getClient()
	if err != nil {
		return err
	}

	continuation := ""
	for {
		s3files := &api2.ListFilesRes{}
		jres := api2.NewResult(s3files)
		url := cfg.pathToAPIurl("files")
		req := &api2.ListFilesReq{
			Continuation: continuation,
			MaxFiles:     maxFiles,
		}
		res, err := postJSON(client, url, req, jres)
		if err != nil {
			return fmt.Errorf("cannot list files on server: %w", err)
		}

		if res.StatusCode != http.StatusOK {
			return fmt.Errorf(
				"status %v when listing files from server",
				res.Status,
			)
		}

		// make a list of existing files
		for pathy, fileInfo := range s3files.Files {
			if pathy != "" && pathy != "/" {
				pathy = pathy[1:]
			}
			if ctx.Debug {
				fmt.Printf("adding to seenMap: %q\n", pathy)
			}

			p := atomic.Pointer[api2.FileInfo]{}
			p.Store(fileInfo)
			seen[pathy] = &p
		}

		if ctx.Debug {
			fmt.Printf("###\ncontinuing with cont %q\n###\n", s3files.Continuation)
		}
		if s3files.Continuation != "" {
			continuation = s3files.Continuation
			continue
		}

		break
	}

	return nil
}

func deleteFromSeenMap(
	cfg *Config,
	ctx *CLIContext,
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

	for fname, p := range seen {
		if p.Load() == nil {
			continue
		}

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

		if res.StatusCode > 299 {
			return fmt.Errorf("status %v when deleting %q", res.Status, fname)
		}
	}

	return nil
}

func (s *SyncCmd) put(
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
		return io.NopCloser(src), nil
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
