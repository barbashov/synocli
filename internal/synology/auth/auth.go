package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type Client struct {
	Endpoint string
	Path     string
	Version  int
	HTTP     *http.Client
}

type ErrorResponse struct {
	Code int `json:"code"`
}

type loginResponse struct {
	Success bool `json:"success"`
	Data    struct {
		SID string `json:"sid"`
	} `json:"data"`
	Error *ErrorResponse `json:"error,omitempty"`
}

// Login authenticates with the Synology DSM API.
// NOTE: The Synology API requires credentials as GET query parameters.
// This means the password will appear in server access logs and proxy logs.
// The debug transport redacts these values, but network intermediaries may not.
func (c *Client) Login(ctx context.Context, user, password, session string) (string, error) {
	vals := url.Values{}
	vals.Set("api", "SYNO.API.Auth")
	vals.Set("version", strconv.Itoa(c.Version))
	vals.Set("method", "login")
	vals.Set("account", user)
	vals.Set("passwd", password)
	if session == "" {
		session = "DownloadStation"
	}
	vals.Set("session", session)
	vals.Set("format", "sid")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Endpoint+c.Path+"?"+vals.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("build login request: %w", err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("login request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode login response: %w", err)
	}
	if !out.Success {
		code := 0
		if out.Error != nil {
			code = out.Error.Code
		}
		return "", fmt.Errorf("auth failed with code %d", code)
	}
	if out.Data.SID == "" {
		return "", fmt.Errorf("auth succeeded but sid missing")
	}
	return out.Data.SID, nil
}

func (c *Client) Logout(ctx context.Context, sid, session string) error {
	vals := url.Values{}
	vals.Set("api", "SYNO.API.Auth")
	vals.Set("version", strconv.Itoa(c.Version))
	vals.Set("method", "logout")
	if session == "" {
		session = "DownloadStation"
	}
	vals.Set("session", session)
	vals.Set("_sid", sid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Endpoint+c.Path+"?"+vals.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}
