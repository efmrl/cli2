package main

import (
	"bytes"
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
)

// httpGetJSON handles making a GET call and unmarshalling the result into your
// type. The caller is responsible for closing res.Body if there is no err.
func httpGetJSON(
	client *http.Client,
	url *url.URL,
	target any,
) (*http.Response, error) {
	res, err := client.Get(url.String())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bytes, &target)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func postJSON(
	client *http.Client,
	url *url.URL,
	args any,
	target any,
) (*http.Response, error) {
	reqBody, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	res, err := client.Post(url.String(), "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	bod, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bod, &target)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func unmarshalMost(r *http.Response) ([]byte, map[string]interface{}, error) {
	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, nil, err
	}

	stuff := map[string]interface{}{}
	err = json.Unmarshal(bytes, &stuff)
	if err != nil {
		return bytes, nil, err
	}

	return bytes, stuff, nil
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

func unmarshalError(r *http.Response) string {
	unknown := "unknown error"
	bytes, stuff, err := unmarshalMost(r)
	if err != nil {
		return unknown
	}

	unknown = fmt.Sprintf("unknown error: %v", string(bytes))

	errorI, ok := stuff["message"]
	if !ok {
		return unknown
	}

	errorStr, ok := errorI.(string)
	if !ok {
		return unknown
	}

	return errorStr
}
