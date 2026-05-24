package downloadstation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// SYNO.DownloadStation2.Task.BT.File exposes the per-file view of a multi-file
// BT task: which files exist, their download progress, whether each is wanted
// (downloaded at all), and its priority. It lives on entry.cgi and is DS2-only;
// there is no SYNO.DownloadStation.BTFile equivalent on DSM 7.
const (
	btFileAPIName = "SYNO.DownloadStation2.Task.BT.File"
	btFilePath    = "/webapi/entry.cgi"
	btFileVersion = 2
)

// Valid BT file priorities. "skip" is NOT a priority — to not download a file,
// set Wanted=false (priority and wanted are independent fields).
const (
	BTFilePriorityLow    = "low"
	BTFilePriorityNormal = "normal"
	BTFilePriorityHigh   = "high"
)

// BTFile is one file inside a multi-file BT download task.
type BTFile struct {
	Index          int64  `json:"index"`
	Name           string `json:"name"` // relative path inside the torrent
	Size           int64  `json:"size"`
	SizeDownloaded int64  `json:"size_downloaded"`
	Priority       string `json:"priority"` // "low" | "normal" | "high"
	Wanted         bool   `json:"wanted"`
}

type btFileListResponse struct {
	baseResponse
	Data struct {
		Offset int      `json:"offset"`
		Total  int      `json:"total"`
		Items  []BTFile `json:"items"`
	} `json:"data"`
}

// ListBTFiles returns every file in a BT task. DSM honors limit=-1, so a single
// request returns the full list regardless of file count; the result is never
// paginated for the caller.
func (c *Client) ListBTFiles(ctx context.Context, taskID string) ([]BTFile, error) {
	vals := c.btFileBaseValues("list")
	vals.Set("task_id", taskID)
	vals.Set("offset", "0")
	vals.Set("limit", "-1")
	var resp btFileListResponse
	if err := c.doBTFileRequest(ctx, vals, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, apiErrorFromBase(resp.baseResponse)
	}
	return resp.Data.Items, nil
}

// SetBTFileWanted marks the given file indices as wanted (downloaded) or not.
// Indices not listed are left unchanged.
func (c *Client) SetBTFileWanted(ctx context.Context, taskID string, indices []int64, wanted bool) error {
	if len(indices) == 0 {
		return nil
	}
	vals := c.btFileBaseValues("set")
	vals.Set("task_id", taskID)
	idxJSON, err := json.Marshal(indices)
	if err != nil {
		return fmt.Errorf("encode index: %w", err)
	}
	vals.Set("index", string(idxJSON))
	vals.Set("wanted", strconv.FormatBool(wanted))
	return c.doBTFileRequest(ctx, vals, nil)
}

// SetBTFilePriority sets the download priority ("low"|"normal"|"high") for the
// given file indices. Priority is independent of wanted-ness.
func (c *Client) SetBTFilePriority(ctx context.Context, taskID string, indices []int64, priority string) error {
	if len(indices) == 0 {
		return nil
	}
	vals := c.btFileBaseValues("set")
	vals.Set("task_id", taskID)
	idxJSON, err := json.Marshal(indices)
	if err != nil {
		return fmt.Errorf("encode index: %w", err)
	}
	vals.Set("index", string(idxJSON))
	vals.Set("priority", priority)
	return c.doBTFileRequest(ctx, vals, nil)
}

func (c *Client) btFileBaseValues(method string) url.Values {
	vals := url.Values{}
	vals.Set("api", btFileAPIName)
	vals.Set("version", strconv.Itoa(btFileVersion))
	vals.Set("method", method)
	vals.Set("_sid", c.sid)
	return vals
}

func (c *Client) doBTFileRequest(ctx context.Context, vals url.Values, out any) error {
	u := c.endpoint + btFilePath + "?" + vals.Encode()
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

// MapBTFile converts a BTFile to a map suitable for JSON output.
func MapBTFile(f BTFile) map[string]any {
	return map[string]any{
		"index":           f.Index,
		"name":            f.Name,
		"size":            f.Size,
		"downloaded_size": f.SizeDownloaded,
		"priority":        f.Priority,
		"wanted":          f.Wanted,
	}
}

// MapBTFiles converts a slice of BTFile to maps for JSON output.
func MapBTFiles(files []BTFile) []map[string]any {
	out := make([]map[string]any, 0, len(files))
	for _, f := range files {
		out = append(out, MapBTFile(f))
	}
	return out
}
