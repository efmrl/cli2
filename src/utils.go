package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/efmrl/api2"
)

// getJSON handles making a GET call and unmarshalling the result into your
// type. The caller is responsible for closing res.Body if there is no err.
func getJSON(
	client *http.Client,
	url *url.URL,
	target *api2.Response,
) error {
	res, err := client.Get(url.String())
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed: %v", res.Status)
	}

	dec := json.NewDecoder(res.Body)
	err = dec.Decode(&target)
	if err != nil {
		return err
	}

	if target.Status != api2.StatusSuccess {
		err = fmt.Errorf("%v: %v", target.Status, target.Message)
		return err
	}

	return nil
}

func postJSON(
	client *http.Client,
	url *url.URL,
	args any,
	target *api2.Response,
) (*http.Response, error) {
	reqBody, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	res, err := client.Post(url.String(), "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	if target == nil {
		return res, nil
	}

	dec := json.NewDecoder(res.Body)
	defer res.Body.Close()
	err = dec.Decode(&target)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func patchJSON(
	ctx context.Context,
	client *http.Client,
	url *url.URL,
	args any,
	target *api2.Response,
) (*http.Response, error) {
	reqBody, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		"PATCH",
		url.String(),
		bytes.NewBuffer(reqBody),
	)
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	bod, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if target != nil {
		err = json.Unmarshal(bod, &target)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

var message = `
You need to log in to proceed. Go here:

%v

click on "get token"
click on "copy"
type "efmrl login confirm " and paste the token here
`

func loggedIn(
	cfg *Config,
) (string, error) {
	url := cfg.pathToAPIurl("session")
	client, err := cfg.getClient()
	if err != nil {
		return "", err
	}

	session := &api2.SessionRes{}
	err = getJSON(client, url, api2.NewResult(session))

	if err != nil || session.Confirmed == "" {
		message := fmt.Sprintf(message, cfg.pathToAdminURL("session"))
		return message, nil
	}

	return "", nil
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
