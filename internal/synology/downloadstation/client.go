package downloadstation

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Client struct {
	endpoint string
	path     string
	version  int
	apiName  string
	sid      string
	http     *http.Client
}

const (
	defaultPath    = "/webapi/DownloadStation/task.cgi"
	defaultVersion = 1
	defaultAPIName = "SYNO.DownloadStation.Task"
)

func NewClient(endpoint, sid string, httpClient *http.Client, path string, version int, apiName string) (*Client, error) {
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
	if strings.TrimSpace(apiName) == "" {
		apiName = defaultAPIName
	}
	return &Client{
		endpoint: endpoint,
		path:     path,
		version:  version,
		apiName:  apiName,
		sid:      sid,
		http:     httpClient,
	}, nil
}

func (c *Client) baseValues() url.Values {
	return c.baseValuesFor(c.taskAPIName(), c.version)
}

func (c *Client) baseValuesFor(apiName string, version int) url.Values {
	vals := url.Values{}
	vals.Set("api", apiName)
	vals.Set("version", strconv.Itoa(version))
	vals.Set("_sid", c.sid)
	return vals
}

func (c *Client) taskAPIName() string {
	if c.apiName != "" {
		return c.apiName
	}
	return defaultAPIName
}
