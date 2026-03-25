package downloadstation

import (
	"context"
	"strconv"
	"strings"
)

func (c *Client) Pause(ctx context.Context, sid string, ids []string) error {
	vals := c.baseValues(sid)
	vals.Set("method", "pause")
	vals.Set("id", strings.Join(ids, ","))
	return c.doGET(ctx, vals, nil)
}

func (c *Client) Resume(ctx context.Context, sid string, ids []string) error {
	vals := c.baseValues(sid)
	vals.Set("method", "resume")
	vals.Set("id", strings.Join(ids, ","))
	return c.doGET(ctx, vals, nil)
}

func (c *Client) Delete(ctx context.Context, sid string, ids []string, withData bool) error {
	vals := c.baseValues(sid)
	vals.Set("method", "delete")
	vals.Set("id", strings.Join(ids, ","))
	vals.Set("force_complete", strconv.FormatBool(withData))
	return c.doGET(ctx, vals, nil)
}
