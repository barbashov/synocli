package downloadstation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) doGET(ctx context.Context, vals url.Values, out any) error {
	return c.doGETToPath(ctx, c.path, vals, out)
}

func (c *Client) doGETAction(ctx context.Context, vals url.Values) error {
	u := c.endpoint + c.path + "?" + vals.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if sid := vals.Get("_sid"); sid != "" {
		req.AddCookie(&http.Cookie{Name: "id", Value: sid})
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return decodeAction(resp.Body)
}

func (c *Client) doGETCreateToPath(ctx context.Context, path string, vals url.Values) ([]string, []string, error) {
	u := c.endpoint + path + "?" + vals.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build request: %w", err)
	}
	if sid := vals.Get("_sid"); sid != "" {
		req.AddCookie(&http.Cookie{Name: "id", Value: sid})
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return decodeCreate(resp.Body)
}

func (c *Client) doGETToPath(ctx context.Context, path string, vals url.Values, out any) error {
	u := c.endpoint + path + "?" + vals.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if sid := vals.Get("_sid"); sid != "" {
		req.AddCookie(&http.Cookie{Name: "id", Value: sid})
	}
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
	if v, ok := out.(*listResponse); ok {
		if !v.Success {
			return apiErrorFromBase(v.baseResponse)
		}
	}
	return nil
}

func apiErrorFromBase(base baseResponse) *APIError {
	code := 0
	if base.Error != nil {
		code = base.Error.Code
	}
	apiErr := &APIError{Code: code}
	if base.Error != nil && base.Error.Errors != nil {
		apiErr.Name = base.Error.Errors.Name
		apiErr.Reason = base.Error.Errors.Reason
		apiErr.FailedTasks = append(apiErr.FailedTasks, base.Error.Errors.FailedTask...)
	}
	return apiErr
}

func decodeBase(r io.Reader) error {
	var out baseResponse
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return fmt.Errorf("decode base response: %w", err)
	}
	if !out.Success {
		return apiErrorFromBase(out)
	}
	return nil
}

func decodeAction(r io.Reader) error {
	var out struct {
		baseResponse
		Data struct {
			FailedTask []FailedTask `json:"failed_task"`
		} `json:"data"`
	}
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return fmt.Errorf("decode action response: %w", err)
	}
	if !out.Success {
		return apiErrorFromBase(out.baseResponse)
	}
	failed := make([]FailedTask, 0, len(out.Data.FailedTask))
	for _, ft := range out.Data.FailedTask {
		if ft.Code != 0 {
			failed = append(failed, ft)
		}
	}
	if len(failed) > 0 {
		return &APIError{
			Code:        failed[0].Code,
			FailedTasks: failed,
		}
	}
	return nil
}

func decodeCreate(r io.Reader) ([]string, []string, error) {
	var out createResponse
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return nil, nil, fmt.Errorf("decode create response: %w", err)
	}
	if !out.Success {
		return nil, nil, apiErrorFromBase(out.baseResponse)
	}
	return stringSliceFromAny(out.Data.TaskID), stringSliceFromAny(out.Data.ListID), nil
}

func validateDirectTaskCreated(taskIDs, listIDs []string) error {
	if len(taskIDs) > 0 {
		return nil
	}
	if len(listIDs) > 0 {
		return fmt.Errorf("create returned list_id without task_id: %s", strings.Join(listIDs, ","))
	}
	return fmt.Errorf("create returned success without task_id")
}

func stringSliceFromAny(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(t))
		for _, s := range t {
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
