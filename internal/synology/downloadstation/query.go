package downloadstation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// encodeStringList renders a list parameter in the dialect the given API
// expects: DS2 wants a JSON array, the legacy v1 API wants comma-separated.
func encodeStringList(apiName string, items []string) (string, error) {
	if isDS2API(apiName) {
		b, err := json.Marshal(items)
		if err != nil {
			return "", fmt.Errorf("encode list: %w", err)
		}
		return string(b), nil
	}
	return strings.Join(items, ","), nil
}

func (c *Client) List(ctx context.Context) ([]Task, error) {
	return c.listFrom(ctx, c.taskAPIName(), c.version, c.path)
}

func (c *Client) listFrom(ctx context.Context, apiName string, version int, path string) ([]Task, error) {
	vals := c.baseValuesFor(apiName, version)
	vals.Set("method", "list")
	vals.Set("offset", "0")
	vals.Set("limit", "-1")
	additional, err := encodeStringList(apiName, []string{"detail", "transfer", "file"})
	if err != nil {
		return nil, fmt.Errorf("encode additional: %w", err)
	}
	vals.Set("additional", additional)
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

func (c *Client) Get(ctx context.Context, id string) (*Task, error) {
	vals := c.baseValues()
	if c.isDS2() {
		vals.Set("method", "get")
	} else {
		// The legacy v1 API has no "get" method; getinfo is its equivalent.
		vals.Set("method", "getinfo")
	}
	idArg, err := encodeStringList(c.taskAPIName(), []string{id})
	if err != nil {
		return nil, fmt.Errorf("encode id: %w", err)
	}
	additional, err := encodeStringList(c.taskAPIName(), []string{"detail", "transfer", "file", "tracker", "peer"})
	if err != nil {
		return nil, fmt.Errorf("encode additional: %w", err)
	}
	vals.Set("id", idArg)
	vals.Set("additional", additional)
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
	return nil, &APIError{Code: 404}
}
