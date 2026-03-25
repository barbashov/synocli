package downloadstation

import (
	"net/http"
	"net/url"
	"strconv"
)

type Client struct {
	Endpoint string
	Path     string
	Version  int
	APIName  string
	HTTP     *http.Client
}

func (c *Client) baseValues(sid string) url.Values {
	return c.baseValuesFor(c.apiName(), c.Version, sid)
}

func (c *Client) baseValuesFor(apiName string, version int, sid string) url.Values {
	vals := url.Values{}
	vals.Set("api", apiName)
	vals.Set("version", strconv.Itoa(version))
	vals.Set("_sid", sid)
	return vals
}

func (c *Client) apiName() string {
	if c.APIName != "" {
		return c.APIName
	}
	return "SYNO.DownloadStation.Task"
}
