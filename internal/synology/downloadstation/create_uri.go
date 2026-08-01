package downloadstation

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) AddURI(ctx context.Context, uri, destination string) ([]string, error) {
	dest, err := c.resolveDestination(ctx, destination)
	if err != nil {
		return nil, err
	}
	vals := c.baseValues()
	vals.Set("method", "create")
	if c.isDS2() {
		vals.Set("type", "url")
		urlJSON, err := json.Marshal([]string{uri})
		if err != nil {
			return nil, fmt.Errorf("encode url: %w", err)
		}
		vals.Set("url", string(urlJSON))
		vals.Set("create_list", "false")
	} else {
		// The legacy v1 create method takes a plain "uri" parameter and
		// returns success without task ids.
		vals.Set("uri", uri)
	}
	vals.Set("destination", dest)
	taskIDs, listIDs, err := c.doGETCreateToPath(ctx, c.path, vals)
	if err != nil {
		return nil, err
	}
	if c.isDS2() {
		if err := validateDirectTaskCreated(taskIDs, listIDs); err != nil {
			return nil, err
		}
	}
	return taskIDs, nil
}
