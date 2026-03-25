package apiinfo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type Entry struct {
	Path       string `json:"path"`
	MinVersion int    `json:"minVersion"`
	MaxVersion int    `json:"maxVersion"`
}

type Response struct {
	Success bool             `json:"success"`
	Data    map[string]Entry `json:"data"`
	Error   *struct {
		Code int `json:"code"`
	} `json:"error,omitempty"`
}

func Discover(ctx context.Context, endpoint string, client *http.Client) (map[string]Entry, error) {
	q := url.Values{}
	q.Set("api", "SYNO.API.Info")
	q.Set("version", "1")
	q.Set("method", "query")
	q.Set("query", "all")
	u := endpoint + "/webapi/query.cgi?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build api info request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request api info: %w", err)
	}
	defer resp.Body.Close()
	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode api info: %w", err)
	}
	if !out.Success {
		code := 0
		if out.Error != nil {
			code = out.Error.Code
		}
		return nil, fmt.Errorf("api info failed with code %d", code)
	}
	return out.Data, nil
}

func Select(entries map[string]Entry, api string, fallbackPath string, fallbackVersion int) (path string, version int) {
	entry, ok := entries[api]
	if !ok {
		return fallbackPath, fallbackVersion
	}
	version = entry.MaxVersion
	if version <= 0 {
		version = fallbackVersion
	}
	return "/webapi/" + entry.Path, version
}
