// Package storage wraps DSM storage-overview APIs (volumes, pools, disks).
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	apiName        = "SYNO.Storage.CGI.Storage"
	smartAPIName   = "SYNO.Storage.CGI.Smart"
	defaultPath    = "/webapi/entry.cgi"
	defaultVersion = 1
	// smartVersion is the SYNO.Storage.CGI.Smart API version. It is tracked
	// separately from c.version (which is discovered/clamped for the Storage
	// API only) so the two cannot drift onto the wrong wire version.
	smartVersion = 1
)

type Client struct {
	endpoint string
	sid      string
	http     *http.Client
	path     string
	version  int
}

func NewClient(endpoint, sid string, httpClient *http.Client, path string, version int) (*Client, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	sid = strings.TrimSpace(sid)
	if sid == "" {
		return nil, fmt.Errorf("sid is required")
	}
	if httpClient == nil {
		return nil, fmt.Errorf("http client is required")
	}
	if strings.TrimSpace(path) == "" {
		path = defaultPath
	}
	if version <= 0 {
		version = defaultVersion
	}
	return &Client{endpoint: endpoint, sid: sid, http: httpClient, path: path, version: version}, nil
}

func (c *Client) callJSON(ctx context.Context, method string, out any) error {
	return c.callAPIJSON(ctx, apiName, c.version, method, nil, out)
}

// callAPIJSON issues a GET to entry.cgi for the given DSM API name, version, and
// method, merging any caller-supplied extra query parameters and decoding the
// standard envelope into out. Smart and Storage APIs share the same auth/cookie
// surface so they live behind one helper, but each passes its own version.
func (c *Client) callAPIJSON(ctx context.Context, api string, version int, method string, extra url.Values, out any) error {
	vals := url.Values{}
	vals.Set("api", api)
	vals.Set("version", strconv.Itoa(version))
	vals.Set("method", method)
	vals.Set("_sid", c.sid)
	for k, vs := range extra {
		for _, v := range vs {
			vals.Add(k, v)
		}
	}
	u := c.endpoint + c.path + "?" + vals.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.AddCookie(&http.Cookie{Name: "id", Value: c.sid})
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var base baseResponse
	if err := json.NewDecoder(resp.Body).Decode(&base); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if !base.Success {
		code := 0
		if base.Error != nil {
			code = base.Error.Code
		}
		return &APIError{Code: code}
	}
	if out == nil || len(base.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(base.Data, out); err != nil {
		return fmt.Errorf("decode data: %w", err)
	}
	return nil
}
