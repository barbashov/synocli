package filestation

import (
	"context"
	"net/url"
	"strings"
	"time"

	"synocli/internal/apperr"
)

// WaitTask polls apiKey/status until the task reports finished or the deadline is exceeded.
func (c *Client) WaitTask(ctx context.Context, apiKey, taskID string, interval, maxWait time.Duration) (map[string]any, error) {
	deadline := time.Time{}
	if maxWait > 0 {
		deadline = time.Now().Add(maxWait)
	}
	for {
		var out map[string]any
		if err := c.Call(ctx, apiKey, "status", url.Values{"taskid": {taskID}}, &out); err != nil {
			return nil, err
		}
		if isFinished(out) {
			return out, nil
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			return nil, apperr.New("timeout", "timeout waiting for task", 5)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// WaitSearch polls the search task until it reports finished or the deadline is exceeded.
func (c *Client) WaitSearch(ctx context.Context, taskID string, interval, maxWait time.Duration) (map[string]any, error) {
	deadline := time.Time{}
	if maxWait > 0 {
		deadline = time.Now().Add(maxWait)
	}
	for {
		var out map[string]any
		if err := c.Call(ctx, APISearch, "list", url.Values{"taskid": {taskID}, "offset": {"0"}, "limit": {"1000"}}, &out); err != nil {
			return nil, err
		}
		if isFinished(out) {
			return out, nil
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			return nil, apperr.New("timeout", "timeout waiting for search", 5)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

func isFinished(out map[string]any) bool {
	if b, ok := out["finished"].(bool); ok {
		return b
	}
	if s, ok := out["status"].(string); ok {
		s = strings.ToLower(s)
		return s == "finished" || s == "done" || s == "success"
	}
	if p, ok := out["progress"].(float64); ok {
		return p >= 100
	}
	return false
}
