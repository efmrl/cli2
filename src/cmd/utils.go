package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// httpGetJSON handles making a GET call and unmarshalling the result into your
// type. The caller is responsible for closing res.Body if there is no err.
func httpGetJSON(
	client *http.Client,
	url *url.URL,
	target interface{},
) (*http.Response, error) {
	res, err := client.Get(url.String())
	if err != nil {
		return nil, err
	}

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

func unmarshalError(r *http.Response) string {
	unknown := "unknown error"
	bytes, stuff, err := unmarshalMost(r)
	if err != nil {
		return unknown
	}

	unknown = fmt.Sprintf("unknown error: %v", string(bytes))

	errorI, ok := stuff["error"]
	if !ok {
		return unknown
	}

	errorStr, ok := errorI.(string)
	if !ok {
		return unknown
	}

	return errorStr
}
