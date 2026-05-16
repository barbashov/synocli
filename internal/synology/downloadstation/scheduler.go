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
	schedulerAPIName = "SYNO.DownloadStation2.Settings.Scheduler"
	schedulerPath    = "/webapi/entry.cgi"
	schedulerVersion = 1
)

type SchedulerConfig struct {
	MaxTasks      int `json:"max_tasks"`
	MaxTasksLimit int `json:"max_tasks_limit"`
}

type SchedulerConfigUpdate struct {
	MaxTasks *int
}

func (u SchedulerConfigUpdate) isEmpty() bool {
	return u.MaxTasks == nil
}

func (c *Client) GetSchedulerConfig(ctx context.Context) (*SchedulerConfig, error) {
	vals := c.schedulerBaseValues("get")
	var resp struct {
		baseResponse
		Data SchedulerConfig `json:"data"`
	}
	if err := c.doSchedulerRequest(ctx, vals, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, apiErrorFromBase(resp.baseResponse)
	}
	return &resp.Data, nil
}

func (c *Client) SetSchedulerConfig(ctx context.Context, u SchedulerConfigUpdate) error {
	if u.isEmpty() {
		return fmt.Errorf("SchedulerConfigUpdate has no fields set")
	}
	vals := c.schedulerBaseValues("set")
	if u.MaxTasks != nil {
		vals.Set("max_tasks", strconv.Itoa(*u.MaxTasks))
	}
	return c.doSchedulerRequest(ctx, vals, nil)
}

func (c *Client) schedulerBaseValues(method string) url.Values {
	vals := url.Values{}
	vals.Set("api", schedulerAPIName)
	vals.Set("version", strconv.Itoa(schedulerVersion))
	vals.Set("method", method)
	vals.Set("_sid", c.sid)
	return vals
}

func (c *Client) doSchedulerRequest(ctx context.Context, vals url.Values, out any) error {
	u := c.endpoint + schedulerPath + "?" + vals.Encode()
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
