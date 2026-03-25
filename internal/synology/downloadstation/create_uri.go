package downloadstation

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) AddURI(ctx context.Context, uri, destination string) ([]string, error) {
	vals := c.baseValues()
	vals.Set("method", "create")
	vals.Set("type", "url")
	urlJSON, err := json.Marshal([]string{uri})
	if err != nil {
		return nil, fmt.Errorf("encode url: %w", err)
	}
	vals.Set("url", string(urlJSON))
	vals.Set("create_list", "false")
	if destination != "" {
		vals.Set("destination", destination)
	}
	taskIDs, listIDs, err := c.doGETCreateToPath(ctx, c.path, vals)
	if err != nil {
		return nil, err
	}
	if err := validateDirectTaskCreated(taskIDs, listIDs); err != nil {
		return nil, err
	}
	return taskIDs, nil
}
