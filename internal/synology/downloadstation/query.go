package downloadstation

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) List(ctx context.Context, sid string) ([]Task, error) {
	return c.listFrom(ctx, c.apiName(), c.Version, c.Path, sid)
}

func (c *Client) listFrom(ctx context.Context, apiName string, version int, path, sid string) ([]Task, error) {
	vals := c.baseValuesFor(apiName, version, sid)
	vals.Set("method", "list")
	vals.Set("offset", "0")
	vals.Set("limit", "-1")
	additionalJSON, err := json.Marshal([]string{"detail", "transfer", "file"})
	if err != nil {
		return nil, fmt.Errorf("encode additional: %w", err)
	}
	vals.Set("additional", string(additionalJSON))
	var out listResponse
	if err := c.doGETToPath(ctx, path, vals, &out); err != nil {
		return nil, err
	}
	if len(out.Data.Tasks) > 0 {
		return out.Data.Tasks, nil
	}
	if len(out.Data.Task) > 0 {
		return out.Data.Task, nil
	}
	return out.Data.Tasks, nil
}

func (c *Client) Get(ctx context.Context, sid, id string) (*Task, error) {
	vals := c.baseValues(sid)
	vals.Set("method", "get")
	idJSON, err := json.Marshal([]string{id})
	if err != nil {
		return nil, fmt.Errorf("encode id: %w", err)
	}
	additionalJSON, err := json.Marshal([]string{"detail", "transfer", "file", "tracker", "peer"})
	if err != nil {
		return nil, fmt.Errorf("encode additional: %w", err)
	}
	vals.Set("id", string(idJSON))
	vals.Set("additional", string(additionalJSON))
	var out listResponse
	if err := c.doGET(ctx, vals, &out); err != nil {
		return nil, err
	}
	if len(out.Data.Tasks) > 0 {
		return &out.Data.Tasks[0], nil
	}
	if len(out.Data.Task) > 0 {
		return &out.Data.Task[0], nil
	}
	return nil, &APIError{Code: 401}
}
