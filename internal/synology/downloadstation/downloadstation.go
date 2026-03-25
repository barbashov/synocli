package downloadstation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Client struct {
	Endpoint    string
	Path        string
	Version     int
	APIName     string
	ListPath    string
	ListVersion int
	ListAPIName string
	HTTP        *http.Client
}

type APIError struct {
	Code   int
	Name   string
	Reason string
}

func (e *APIError) Error() string {
	if e.Name != "" {
		return fmt.Sprintf("download station api error code=%d (%s): %s %s", e.Code, ErrorMessage(e.Code), e.Name, e.Reason)
	}
	return fmt.Sprintf("download station api error code=%d (%s)", e.Code, ErrorMessage(e.Code))
}

type Task struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Type        string          `json:"type"`
	Username    string          `json:"username,omitempty"`
	Size        int64           `json:"size,omitempty"`
	Status      string          `json:"status"`
	StatusExtra string          `json:"status_extra,omitempty"`
	Additional  *TaskAdditional `json:"additional,omitempty"`
}

func (t *Task) UnmarshalJSON(data []byte) error {
	type taskAlias struct {
		ID          string          `json:"id"`
		Title       string          `json:"title"`
		Type        string          `json:"type"`
		Username    string          `json:"username,omitempty"`
		Size        int64           `json:"size,omitempty"`
		Status      any             `json:"status"`
		StatusExtra string          `json:"status_extra,omitempty"`
		Additional  *TaskAdditional `json:"additional,omitempty"`
	}
	var aux taskAlias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	t.ID = aux.ID
	t.Title = aux.Title
	t.Type = aux.Type
	t.Username = aux.Username
	t.Size = aux.Size
	t.StatusExtra = aux.StatusExtra
	t.Additional = aux.Additional
	switch v := aux.Status.(type) {
	case string:
		t.Status = v
	case float64:
		t.Status = strconv.FormatInt(int64(v), 10)
	case nil:
		t.Status = ""
	default:
		t.Status = fmt.Sprintf("%v", v)
	}
	return nil
}

type TaskAdditional struct {
	Detail   *TaskDetail   `json:"detail,omitempty"`
	Transfer *TaskTransfer `json:"transfer,omitempty"`
	Tracker  any           `json:"tracker,omitempty"`
	Peer     any           `json:"peer,omitempty"`
	File     any           `json:"file,omitempty"`
}

type TaskDetail struct {
	Destination   string `json:"destination,omitempty"`
	URI           string `json:"uri,omitempty"`
	CreateTime    int64  `json:"create_time,omitempty"`
	CompletedTime int64  `json:"completed_time,omitempty"`
	ErrorDetail   string `json:"error_detail,omitempty"`
}

type TaskTransfer struct {
	SizeDownloaded int64 `json:"size_downloaded,omitempty"`
	SizeUploaded   int64 `json:"size_uploaded,omitempty"`
	SpeedDownload  int64 `json:"speed_download,omitempty"`
	SpeedUpload    int64 `json:"speed_upload,omitempty"`
}

type baseResponse struct {
	Success bool `json:"success"`
	Error   *struct {
		Code   int `json:"code"`
		Errors *struct {
			Name   string `json:"name"`
			Reason string `json:"reason"`
		} `json:"errors,omitempty"`
	} `json:"error,omitempty"`
}

type listResponse struct {
	baseResponse
	Data struct {
		Offset int    `json:"offset"`
		Total  int    `json:"total"`
		Tasks  []Task `json:"tasks"`
		Task   []Task `json:"task"`
		List   []Task `json:"list"`
	} `json:"data"`
}

type createResponse struct {
	baseResponse
	Data struct {
		TaskID any `json:"task_id"`
		ListID any `json:"list_id"`
	} `json:"data"`
}

var errorMessages = map[int]string{
	100: "unknown error",
	101: "invalid parameter",
	102: "api does not exist",
	103: "method does not exist",
	104: "this API version is not supported",
	105: "insufficient user privilege",
	106: "session timeout",
	107: "session interrupted by duplicate login",
	400: "invalid parameter of task",
	401: "unknown task",
	402: "invalid task id",
	403: "file upload failed",
	404: "max number of tasks reached",
	405: "destination denied",
	406: "destination does not exist",
	407: "invalid task action",
	408: "unsupported protocol type",
	120: "required parameter missing",
}

func ErrorMessage(code int) string {
	if v, ok := errorMessages[code]; ok {
		return v
	}
	return "unmapped"
}

func NormalizeStatus(raw string) string {
	if code, ok := parseNumericStatusCode(raw); ok {
		switch code {
		case 1:
			return "waiting"
		case 2, 6:
			return "downloading"
		case 3:
			return "paused"
		case 4, 8, 9:
			return "finishing"
		case 5:
			return "finished"
		case 7:
			return "seeding"
		case 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35:
			return "error"
		case 36:
			return "unknown"
		default:
			return "unknown"
		}
	}
	s := strings.ToLower(raw)
	switch s {
	case "waiting", "waiting_peer", "waiting_tracker":
		return "waiting"
	case "downloading", "hash_checking":
		return "downloading"
	case "paused":
		return "paused"
	case "finishing", "extracting", "filehosting_waiting":
		return "finishing"
	case "finished":
		return "finished"
	case "seeding":
		return "seeding"
	case "error":
		return "error"
	default:
		return "unknown"
	}
}

var ds2StatusEnum = map[int]string{
	1:  "waiting",
	2:  "downloading",
	3:  "paused",
	4:  "finishing",
	5:  "finished",
	6:  "hashing",
	7:  "seeding",
	8:  "filehosting_waiting",
	9:  "extracting",
	10: "error",
	11: "broken_link",
	12: "destination_not_exist",
	13: "destination_denied",
	14: "disk_full",
	15: "quota_reached",
	16: "timeout",
	17: "exceed_max_file_system_size",
	18: "exceed_max_destination_size",
	19: "exceed_max_temp_size",
	20: "encrypted_name_too_long",
	21: "name_too_long",
	22: "torrent_duplicate",
	23: "file_not_exist",
	24: "required_premium_account",
	25: "not_supported_type",
	26: "try_it_later",
	27: "task_encryption",
	28: "missing_python",
	29: "private_video",
	30: "ftp_encryption_not_supported_type",
	31: "extract_failed",
	32: "extract_failed_wrong_password",
	33: "extract_failed_invalid_archive",
	34: "extract_failed_quota_reached",
	35: "extract_failed_disk_full",
	36: "unknown",
}

func StatusEnum(raw string) string {
	if code, ok := parseNumericStatusCode(raw); ok {
		if name, ok := ds2StatusEnum[code]; ok {
			return name
		}
		return "unknown"
	}
	return strings.ToLower(raw)
}

func StatusCode(raw string) (int, bool) {
	return parseNumericStatusCode(raw)
}

func StatusDisplay(raw string) string {
	if code, ok := parseNumericStatusCode(raw); ok {
		if name, ok := ds2StatusEnum[code]; ok {
			return fmt.Sprintf("%s (%d)", name, code)
		}
		return fmt.Sprintf("unknown (%d)", code)
	}
	return raw
}

func parseNumericStatusCode(raw string) (int, bool) {
	code, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, false
	}
	return code, true
}

func IsTerminalSuccess(normalized string) bool {
	return normalized == "finished" || normalized == "seeding"
}

func IsTerminalFailure(normalized string) bool {
	return normalized == "error"
}

// --- Torrent multipart helpers ---

func openAndStatTorrent(path string) (*os.File, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open torrent file: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, fmt.Errorf("stat torrent file: %w", err)
	}
	return f, st.Size(), nil
}

func buildTorrentMultipart(r io.Reader, textFields [][2]string, fileFieldName, fileName string) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	for _, kv := range textFields {
		if err := mw.WriteField(kv[0], kv[1]); err != nil {
			return nil, "", err
		}
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fileFieldName, fileName))
	h.Set("Content-Type", "application/x-bittorrent")
	part, err := mw.CreatePart(h)
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(part, r); err != nil {
		return nil, "", err
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return body, mw.FormDataContentType(), nil
}

func (c *Client) postTorrent(ctx context.Context, sid, reqURL string, body *bytes.Buffer, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("build torrent request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(&http.Cookie{Name: "id", Value: sid})
	return c.HTTP.Do(req)
}

// --- Download operations ---

func (c *Client) AddURI(ctx context.Context, sid, uri, destination string) ([]string, error) {
	vals := c.baseValues(sid)
	vals.Set("method", "create")
	if strings.Contains(c.apiName(), "DownloadStation2.") {
		vals.Set("type", "url")
		vals.Set("url", fmt.Sprintf("[\"%s\"]", uri))
		vals.Set("create_list", "false")
	} else {
		vals.Set("uri", uri)
	}
	if destination != "" {
		vals.Set("destination", destination)
	}
	if strings.Contains(c.apiName(), "DownloadStation2.") {
		taskIDs, listIDs, err := c.doGETCreateToPath(ctx, c.Path, vals)
		if err != nil {
			return nil, err
		}
		if err := validateDirectTaskCreated(taskIDs, listIDs); err != nil {
			return nil, err
		}
		return taskIDs, nil
	}
	return nil, c.doGET(ctx, vals, nil)
}

func (c *Client) AddTorrent(ctx context.Context, sid, torrentPath, destination string) ([]string, error) {
	if strings.Contains(c.apiName(), "DownloadStation2.") {
		dest := destination
		if strings.TrimSpace(dest) == "" {
			defDest, err := c.getDefaultDestination(ctx, sid)
			if err != nil {
				return nil, fmt.Errorf("destination is required and default_destination could not be fetched: %w", err)
			}
			if strings.TrimSpace(defDest) == "" {
				return nil, fmt.Errorf("destination is required and default_destination is empty; pass --destination")
			}
			dest = defDest
		}
		taskIDs, listIDs, err := c.addTorrentDS2Direct(ctx, sid, torrentPath, dest)
		if err != nil {
			return nil, err
		}
		if err := validateDirectTaskCreated(taskIDs, listIDs); err != nil {
			return nil, err
		}
		return taskIDs, nil
	}
	return nil, c.addTorrentWithFallback(ctx, sid, torrentPath, destination)
}

func (c *Client) addTorrentWithFallback(ctx context.Context, sid, torrentPath, destination string) error {
	fields := []string{"file", "torrent", "upload"}
	modes := []string{"query_only", "post_only"}
	filenames := []string{filepath.Base(torrentPath), "upload.torrent"}
	var lastErr error
	for _, mode := range modes {
		for _, field := range fields {
			for _, filename := range filenames {
				if err := c.addTorrentWithField(ctx, sid, torrentPath, destination, field, mode, filename); err != nil {
					lastErr = err
					var apiErr *APIError
					if errors.As(err, &apiErr) && (apiErr.Code == 101 || apiErr.Code == 400 || apiErr.Code == 403) {
						continue
					}
					return err
				}
				return nil
			}
		}
	}
	// Compatibility fallback based on known working community implementation.
	if err := c.addTorrentCreateListStyle(ctx, sid, torrentPath, destination); err == nil {
		return nil
	}
	return lastErr
}

func (c *Client) getDefaultDestination(ctx context.Context, sid string) (string, error) {
	type infoResp struct {
		Success bool `json:"success"`
		Data    struct {
			DefaultDestination string `json:"default_destination"`
		} `json:"data"`
		Error *struct {
			Code int `json:"code"`
		} `json:"error,omitempty"`
	}
	vals := url.Values{}
	vals.Set("api", "SYNO.DownloadStation.Info")
	vals.Set("version", "2")
	vals.Set("method", "getconfig")
	vals.Set("_sid", sid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Endpoint+"/webapi/DownloadStation/info.cgi?"+vals.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.AddCookie(&http.Cookie{Name: "id", Value: sid})
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out infoResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if !out.Success {
		code := 0
		if out.Error != nil {
			code = out.Error.Code
		}
		return "", &APIError{Code: code}
	}
	return out.Data.DefaultDestination, nil
}

func (c *Client) addTorrentWithField(ctx context.Context, sid, torrentPath, destination, fieldName, mode, filename string) error {
	f, err := os.Open(torrentPath)
	if err != nil {
		return fmt.Errorf("open torrent file: %w", err)
	}
	defer f.Close()

	vals := c.baseValues(sid)
	vals.Set("method", "create")
	if destination != "" {
		vals.Set("destination", destination)
	}
	// Synology officially documents two valid multipart strategies:
	// 1) file as the only POST data; all other params in query
	// 2) all params in POST data, with file part as the LAST parameter
	var textFields [][2]string
	if mode == "post_only" {
		for _, key := range []string{"api", "version", "method", "_sid", "destination"} {
			if v := vals.Get(key); v != "" {
				textFields = append(textFields, [2]string{key, v})
			}
		}
	}
	body, contentType, err := buildTorrentMultipart(f, textFields, fieldName, filename)
	if err != nil {
		return err
	}
	u := c.Endpoint + c.Path
	if mode == "query_only" {
		u += "?" + vals.Encode()
	}
	resp, err := c.postTorrent(ctx, sid, u, body, contentType)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeBase(resp.Body)
}

func (c *Client) addTorrentCreateListStyle(ctx context.Context, sid, torrentPath, destination string) error {
	f, size, err := openAndStatTorrent(torrentPath)
	if err != nil {
		return err
	}
	defer f.Close()

	apiName := c.apiName()
	// Keep this field set matching the proven reference implementation shape.
	fields := [][2]string{
		{"api", apiName},
		{"method", "create"},
		{"version", strconv.Itoa(c.Version)},
		{"type", "\"file\""},
		{"file", "[\"torrent\"]"},
		{"create_list", "true"},
		{"size", strconv.FormatInt(size, 10)},
	}
	if destination != "" {
		fields = append(fields, [2]string{"destination", "\"" + destination + "\""})
	}
	body, contentType, err := buildTorrentMultipart(f, fields, "torrent", filepath.Base(torrentPath))
	if err != nil {
		return err
	}
	q := url.Values{}
	q.Set("_sid", sid)
	u := c.Endpoint + c.Path
	// task.cgi/SYNO.DownloadStation.Task is used in some community clients.
	if strings.HasSuffix(c.Path, "/task.cgi") {
		u += "/" + apiName
	}
	resp, err := c.postTorrent(ctx, sid, u+"?"+q.Encode(), body, contentType)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeBase(resp.Body)
}

func (c *Client) addTorrentDS2Direct(ctx context.Context, sid, torrentPath, destination string) ([]string, []string, error) {
	f, size, err := openAndStatTorrent(torrentPath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	apiName := c.apiName()
	fields := [][2]string{
		{"api", apiName},
		{"method", "create"},
		{"version", strconv.Itoa(c.Version)},
		{"type", "\"file\""},
		{"file", "[\"torrent\"]"},
		{"create_list", "false"},
		{"size", strconv.FormatInt(size, 10)},
	}
	if destination != "" {
		fields = append(fields, [2]string{"destination", "\"" + destination + "\""})
	}
	body, contentType, err := buildTorrentMultipart(f, fields, "torrent", filepath.Base(torrentPath))
	if err != nil {
		return nil, nil, err
	}
	q := url.Values{}
	q.Set("_sid", sid)
	resp, err := c.postTorrent(ctx, sid, c.Endpoint+c.Path+"/"+apiName+"?"+q.Encode(), body, contentType)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	return decodeCreate(resp.Body)
}

func (c *Client) List(ctx context.Context, sid string) ([]Task, error) {
	if strings.Contains(c.apiName(), "DownloadStation2.") {
		tasks, err := c.listFrom(ctx, c.apiName(), c.Version, c.Path, sid)
		if err == nil {
			return tasks, nil
		}
		return nil, err
	}
	tasks, err := c.listFrom(ctx, c.listAPIName(), c.listVersionOrTask(), c.listPathOrTask(), sid)
	if err == nil && len(tasks) > 0 {
		return tasks, nil
	}
	// Fallback to Task API list for mixed DSM variants.
	fallbackTasks, fbErr := c.listFrom(ctx, c.apiName(), c.Version, c.Path, sid)
	if fbErr == nil {
		return fallbackTasks, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, fbErr
}

func (c *Client) listFrom(ctx context.Context, apiName string, version int, path, sid string) ([]Task, error) {
	vals := c.baseValuesFor(apiName, version, sid)
	vals.Set("method", "list")
	vals.Set("offset", "0")
	vals.Set("limit", "-1")
	vals.Set("additional", "detail,file")
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
	if len(out.Data.List) > 0 {
		return out.Data.List, nil
	}
	return out.Data.Tasks, nil
}

func (c *Client) Get(ctx context.Context, sid, id string) (*Task, error) {
	vals := c.baseValues(sid)
	if strings.Contains(c.apiName(), "DownloadStation2.") {
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
	} else {
		vals.Set("method", "getinfo")
		vals.Set("id", id)
		vals.Set("additional", "detail,transfer,file,tracker,peer")
	}
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
	if len(out.Data.List) > 0 {
		return &out.Data.List[0], nil
	}
	return nil, &APIError{Code: 401}
}

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

func (c *Client) baseValues(sid string) url.Values {
	return c.baseValuesFor(c.apiName(), c.Version, sid)
}

func (c *Client) baseValuesFor(apiName string, version int, sid string) url.Values {
	vals := url.Values{}
	vals.Set("api", apiName)
	vals.Set("version", strconv.Itoa(version))
	vals.Set("_sid", sid)
	return vals
}

func (c *Client) apiName() string {
	if c.APIName != "" {
		return c.APIName
	}
	return "SYNO.DownloadStation.Task"
}

func (c *Client) doGET(ctx context.Context, vals url.Values, out any) error {
	return c.doGETToPath(ctx, c.Path, vals, out)
}

func (c *Client) doGETCreateToPath(ctx context.Context, path string, vals url.Values) ([]string, []string, error) {
	u := c.Endpoint + path + "?" + vals.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build request: %w", err)
	}
	if sid := vals.Get("_sid"); sid != "" {
		req.AddCookie(&http.Cookie{Name: "id", Value: sid})
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	return decodeCreate(resp.Body)
}

func (c *Client) doGETToPath(ctx context.Context, path string, vals url.Values, out any) error {
	u := c.Endpoint + path + "?" + vals.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if sid := vals.Get("_sid"); sid != "" {
		req.AddCookie(&http.Cookie{Name: "id", Value: sid})
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if out == nil {
		return decodeBase(resp.Body)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	switch v := out.(type) {
	case *listResponse:
		if !v.Success {
			code := 0
			if v.Error != nil {
				code = v.Error.Code
			}
			apiErr := &APIError{Code: code}
			if v.Error != nil && v.Error.Errors != nil {
				apiErr.Name = v.Error.Errors.Name
				apiErr.Reason = v.Error.Errors.Reason
			}
			return apiErr
		}
	}
	return nil
}

func (c *Client) listPathOrTask() string {
	if c.ListPath != "" {
		return c.ListPath
	}
	return c.Path
}

func (c *Client) listVersionOrTask() int {
	if c.ListVersion > 0 {
		return c.ListVersion
	}
	return c.Version
}

func (c *Client) listAPIName() string {
	if c.ListAPIName != "" {
		return c.ListAPIName
	}
	return c.apiName()
}

func decodeBase(r io.Reader) error {
	var out baseResponse
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return fmt.Errorf("decode base response: %w", err)
	}
	if !out.Success {
		code := 0
		if out.Error != nil {
			code = out.Error.Code
		}
		apiErr := &APIError{Code: code}
		if out.Error != nil && out.Error.Errors != nil {
			apiErr.Name = out.Error.Errors.Name
			apiErr.Reason = out.Error.Errors.Reason
		}
		return apiErr
	}
	return nil
}

func decodeCreate(r io.Reader) ([]string, []string, error) {
	var out createResponse
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return nil, nil, fmt.Errorf("decode create response: %w", err)
	}
	if !out.Success {
		code := 0
		if out.Error != nil {
			code = out.Error.Code
		}
		apiErr := &APIError{Code: code}
		if out.Error != nil && out.Error.Errors != nil {
			apiErr.Name = out.Error.Errors.Name
			apiErr.Reason = out.Error.Errors.Reason
		}
		return nil, nil, apiErr
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
