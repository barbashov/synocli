package downloadstation

import (
	"context"
	"strings"
)

func (c *Client) Pause(ctx context.Context, ids []string) error {
	vals := c.baseValues()
	vals.Set("method", "pause")
	vals.Set("id", strings.Join(ids, ","))
	return c.doGETAction(ctx, vals)
}

func (c *Client) Resume(ctx context.Context, ids []string) error {
	vals := c.baseValues()
	vals.Set("method", "resume")
	vals.Set("id", strings.Join(ids, ","))
	return c.doGETAction(ctx, vals)
}

func (c *Client) Delete(ctx context.Context, ids []string) error {
	vals := c.baseValues()
	vals.Set("method", "delete")
	vals.Set("id", strings.Join(ids, ","))
	return c.doGETAction(ctx, vals)
}
