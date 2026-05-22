// Package system wraps DSM system-introspection APIs (model, version,
// utilization, hardware status). Each API surface lives in its own file and
// shares the small transport here.
package system

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// APISpec describes a single DSM API entry that the system client knows how
// to call. Path is the full URL path including the "/webapi/" prefix.
type APISpec struct {
	Name    string
	Path    string
	Version int
}

const (
	APIDSMInfo      = "dsm_info"      // SYNO.DSM.Info
	APISystemInfo   = "system_info"   // SYNO.Core.System
	APIUtilization  = "utilization"   // SYNO.Core.System.Utilization
	APINeedReboot   = "need_reboot"   // SYNO.Core.Hardware.NeedReboot
	APISystemStatus = "system_status" // SYNO.Core.System.Status
)

var defaultAPIs = map[string]APISpec{
	APIDSMInfo:      {Name: "SYNO.DSM.Info", Path: "/webapi/entry.cgi", Version: 2},
	APISystemInfo:   {Name: "SYNO.Core.System", Path: "/webapi/entry.cgi", Version: 3},
	APIUtilization:  {Name: "SYNO.Core.System.Utilization", Path: "/webapi/entry.cgi", Version: 1},
	APINeedReboot:   {Name: "SYNO.Core.Hardware.NeedReboot", Path: "/webapi/entry.cgi", Version: 1},
	APISystemStatus: {Name: "SYNO.Core.System.Status", Path: "/webapi/entry.cgi", Version: 1},
}

type Client struct {
	endpoint string
	sid      string
	http     *http.Client
	apis     map[string]APISpec
}

func NewClient(endpoint, sid string, httpClient *http.Client, apis map[string]APISpec) (*Client, error) {
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
	cfg := make(map[string]APISpec, len(defaultAPIs))
	for k, v := range defaultAPIs {
		cfg[k] = v
	}
	for k, v := range apis {
		if strings.TrimSpace(v.Name) == "" {
			continue
		}
		if strings.TrimSpace(v.Path) == "" {
			v.Path = "/webapi/entry.cgi"
		}
		if !strings.HasPrefix(v.Path, "/") {
			v.Path = "/webapi/" + v.Path
		}
		if v.Version <= 0 {
			if d, ok := cfg[k]; ok {
				v.Version = d.Version
			} else {
				v.Version = 1
			}
		}
		cfg[k] = v
	}
	return &Client{endpoint: endpoint, sid: sid, http: httpClient, apis: cfg}, nil
}

func (c *Client) api(key string) APISpec {
	if a, ok := c.apis[key]; ok {
		return a
	}
	return defaultAPIs[key]
}

// callJSON GETs the given API/method, decodes the success-envelope's `data`
// field into out, and surfaces *APIError when DSM reports success=false.
func (c *Client) callJSON(ctx context.Context, apiKey, method string, out any) error {
	api := c.api(apiKey)
	vals := url.Values{}
	vals.Set("api", api.Name)
	vals.Set("version", strconv.Itoa(api.Version))
	vals.Set("method", method)
	vals.Set("_sid", c.sid)
	u := c.endpoint + api.Path + "?" + vals.Encode()
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
		return &APIError{Code: code, API: api.Name}
	}
	if out == nil || len(base.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(base.Data, out); err != nil {
		return fmt.Errorf("decode data: %w", err)
	}
	return nil
}
