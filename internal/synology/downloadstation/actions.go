package downloadstation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (c *Client) Pause(ctx context.Context, ids []string) error {
	return c.doAction(ctx, "pause", ids)
}

func (c *Client) Resume(ctx context.Context, ids []string) error {
	return c.doAction(ctx, "resume", ids)
}

func (c *Client) Delete(ctx context.Context, ids []string) error {
	return c.doAction(ctx, "delete", ids)
}

func (c *Client) doAction(ctx context.Context, method string, ids []string) error {
	idArg, err := c.actionIDsArg(ids)
	if err != nil {
		return err
	}
	vals := c.baseValues()
	vals.Set("method", method)
	vals.Set("id", idArg)
	return c.doGETAction(ctx, vals)
}

func (c *Client) actionIDsArg(ids []string) (string, error) {
	if strings.HasPrefix(c.taskAPIName(), "SYNO.DownloadStation2.") {
		b, err := json.Marshal(ids)
		if err != nil {
			return "", fmt.Errorf("encode ids: %w", err)
		}
		return string(b), nil
	}
	return strings.Join(ids, ","), nil
}
