package downloadstation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const (
	infoAPIName = "SYNO.DownloadStation.Info"
	infoPath    = "/webapi/DownloadStation/info.cgi"
	infoVersion = 2
)

type ServerConfig struct {
	BTMaxDownload           int    `json:"bt_max_download"`
	BTMaxUpload             int    `json:"bt_max_upload"`
	EmuleMaxDownload        int    `json:"emule_max_download"`
	EmuleMaxUpload          int    `json:"emule_max_upload"`
	NZBMaxDownload          int    `json:"nzb_max_download"`
	HTTPMaxDownload         int    `json:"http_max_download"`
	FTPMaxDownload          int    `json:"ftp_max_download"`
	EmuleEnabled            bool   `json:"emule_enabled"`
	UnzipServiceEnabled     bool   `json:"unzip_service_enabled"`
	DefaultDestination      string `json:"default_destination"`
	EmuleDefaultDestination string `json:"emule_default_destination"`
}

type ServerConfigUpdate struct {
	BTMaxDownload *int
	BTMaxUpload   *int
}

func (u ServerConfigUpdate) isEmpty() bool {
	return u.BTMaxDownload == nil && u.BTMaxUpload == nil
}

func (c *Client) GetServerConfig(ctx context.Context) (*ServerConfig, error) {
	vals := c.infoBaseValues("getconfig")
	var resp struct {
		baseResponse
		Data ServerConfig `json:"data"`
	}
	if err := c.doInfoRequest(ctx, vals, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, apiErrorFromBase(resp.baseResponse)
	}
	return &resp.Data, nil
}

func (c *Client) SetServerConfig(ctx context.Context, u ServerConfigUpdate) error {
	if u.isEmpty() {
		return fmt.Errorf("ServerConfigUpdate has no fields set")
	}
	vals := c.infoBaseValues("setserverconfig")
	if u.BTMaxDownload != nil {
		vals.Set("bt_max_download", strconv.Itoa(*u.BTMaxDownload))
	}
	if u.BTMaxUpload != nil {
		vals.Set("bt_max_upload", strconv.Itoa(*u.BTMaxUpload))
	}
	return c.doInfoRequest(ctx, vals, nil)
}

func (c *Client) infoBaseValues(method string) url.Values {
	vals := url.Values{}
	vals.Set("api", infoAPIName)
	vals.Set("version", strconv.Itoa(infoVersion))
	vals.Set("method", method)
	vals.Set("_sid", c.sid)
	return vals
}

func (c *Client) doInfoRequest(ctx context.Context, vals url.Values, out any) error {
	u := c.endpoint + infoPath + "?" + vals.Encode()
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
	if out == nil {
		return decodeBase(resp.Body)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
