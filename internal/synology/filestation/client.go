package filestation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type APISpec struct {
	Name    string
	Path    string
	Version int
}

const (
	APIInfo           = "info"
	APIList           = "list"
	APICreateFolder   = "create_folder"
	APIRename         = "rename"
	APIDelete         = "delete"
	APICopyMove       = "copy_move"
	APIUpload         = "upload"
	APIDownload       = "download"
	APISearch         = "search"
	APIDirSize        = "dir_size"
	APIMD5            = "md5"
	APIExtract        = "extract"
	APICompress       = "compress"
	APIBackgroundTask = "background_task"
)

var defaultAPIs = map[string]APISpec{
	APIInfo:           {Name: "SYNO.FileStation.Info", Path: "/webapi/entry.cgi", Version: 2},
	APIList:           {Name: "SYNO.FileStation.List", Path: "/webapi/entry.cgi", Version: 2},
	APICreateFolder:   {Name: "SYNO.FileStation.CreateFolder", Path: "/webapi/entry.cgi", Version: 2},
	APIRename:         {Name: "SYNO.FileStation.Rename", Path: "/webapi/entry.cgi", Version: 2},
	APIDelete:         {Name: "SYNO.FileStation.Delete", Path: "/webapi/entry.cgi", Version: 2},
	APICopyMove:       {Name: "SYNO.FileStation.CopyMove", Path: "/webapi/entry.cgi", Version: 3},
	APIUpload:         {Name: "SYNO.FileStation.Upload", Path: "/webapi/entry.cgi", Version: 2},
	APIDownload:       {Name: "SYNO.FileStation.Download", Path: "/webapi/entry.cgi", Version: 2},
	APISearch:         {Name: "SYNO.FileStation.Search", Path: "/webapi/entry.cgi", Version: 2},
	APIDirSize:        {Name: "SYNO.FileStation.DirSize", Path: "/webapi/entry.cgi", Version: 2},
	APIMD5:            {Name: "SYNO.FileStation.MD5", Path: "/webapi/entry.cgi", Version: 2},
	APIExtract:        {Name: "SYNO.FileStation.Extract", Path: "/webapi/entry.cgi", Version: 2},
	APICompress:       {Name: "SYNO.FileStation.Compress", Path: "/webapi/entry.cgi", Version: 3},
	APIBackgroundTask: {Name: "SYNO.FileStation.BackgroundTask", Path: "/webapi/entry.cgi", Version: 3},
}

type Client struct {
	endpoint string
	sid      string
	http     *http.Client
	apis     map[string]APISpec
}

type APIError struct {
	Code    int
	SubCode int
	Path    string
	Name    string
	Reason  string
}

func (e *APIError) Error() string {
	code := e.EffectiveCode()
	msg := ErrorMessage(code)
	if e.Path != "" {
		return fmt.Sprintf("file station api error code=%d (%s), path=%s", code, msg, e.Path)
	}
	return fmt.Sprintf("file station api error code=%d (%s)", code, msg)
}

func (e *APIError) EffectiveCode() int {
	if e.SubCode > 0 {
		return e.SubCode
	}
	return e.Code
}

type baseResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code   int             `json:"code"`
		Errors json.RawMessage `json:"errors,omitempty"`
	} `json:"error,omitempty"`
}

type apiSubError struct {
	Code   int    `json:"code"`
	Path   string `json:"path"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

var errorMessages = map[int]string{
	100:  "unknown error",
	101:  "invalid parameter",
	102:  "api does not exist",
	103:  "method does not exist",
	104:  "api version is not supported",
	105:  "insufficient user privilege",
	106:  "session timeout",
	107:  "session interrupted by duplicate login",
	119:  "SID not found",
	400:  "invalid parameter of file operation",
	401:  "unknown error of file operation",
	402:  "system is too busy",
	403:  "invalid user does this file operation",
	404:  "invalid group does this file operation",
	405:  "invalid user and group does this file operation",
	406:  "can't get user/group information from the account server",
	407:  "operation is not permitted",
	408:  "file or folder does not exist",
	409:  "non-supported file system",
	599:  "internal service error",
	1000: "failed to perform operation",
	1001: "failed to upload file",
	1002: "destination path contains invalid entries",
}

func ErrorMessage(code int) string {
	if msg, ok := errorMessages[code]; ok {
		return msg
	}
	return "unmapped"
}

func NewClient(endpoint, sid string, httpClient *http.Client, apis map[string]APISpec) (*Client, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	sid = strings.TrimSpace(sid)
	if sid == "" {
		return nil, fmt.Errorf("sid is required")
	}
	if httpClient == nil {
		return nil, fmt.Errorf("http client is required")
	}
	cfg := make(map[string]APISpec, len(defaultAPIs))
	for k, v := range defaultAPIs {
		cfg[k] = v
	}
	for k, v := range apis {
		if strings.TrimSpace(v.Name) == "" {
			continue
		}
		if strings.TrimSpace(v.Path) == "" {
			v.Path = "/webapi/entry.cgi"
		}
		if !strings.HasPrefix(v.Path, "/") {
			v.Path = "/webapi/" + v.Path
		}
		if v.Version <= 0 {
			if d, ok := cfg[k]; ok {
				v.Version = d.Version
			} else {
				v.Version = 1
			}
		}
		cfg[k] = v
	}
	return &Client{endpoint: endpoint, sid: sid, http: httpClient, apis: cfg}, nil
}

func (c *Client) API(key string) APISpec {
	if api, ok := c.apis[key]; ok {
		return api
	}
	if d, ok := defaultAPIs[key]; ok {
		return d
	}
	return APISpec{Name: key, Path: "/webapi/entry.cgi", Version: 1}
}

func (c *Client) Call(ctx context.Context, apiKey, method string, params url.Values, out any) error {
	api := c.API(apiKey)
	vals := url.Values{}
	vals.Set("api", api.Name)
	vals.Set("version", strconv.Itoa(api.Version))
	vals.Set("method", method)
	vals.Set("_sid", c.sid)
	for k, v := range params {
		for _, vv := range v {
			vals.Add(k, vv)
		}
	}
	u := c.endpoint + api.Path + "?" + vals.Encode()
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
	return decodeJSON(resp.Body, out)
}

func (c *Client) Upload(ctx context.Context, params map[string]string, localPath string) (map[string]any, error) {
	api := c.API(APIUpload)
	f, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("open upload file: %w", err)
	}
	defer func() { _ = f.Close() }()

	q := url.Values{}
	q.Set("_sid", c.sid)
	u := c.endpoint + api.Path + "?" + q.Encode()

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	errCh := make(chan error, 1)
	go func() {
		defer func() { _ = pw.Close() }()
		defer close(errCh)
		if err := mw.WriteField("api", api.Name); err != nil {
			errCh <- err
			return
		}
		if err := mw.WriteField("version", strconv.Itoa(api.Version)); err != nil {
			errCh <- err
			return
		}
		if err := mw.WriteField("method", "upload"); err != nil {
			errCh <- err
			return
		}
		for k, v := range params {
			if err := mw.WriteField(k, v); err != nil {
				errCh <- err
				return
			}
		}
		part, err := mw.CreateFormFile("file", filepath.Base(localPath))
		if err != nil {
			errCh <- err
			return
		}
		if _, err := io.Copy(part, f); err != nil {
			errCh <- err
			return
		}
		errCh <- mw.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, pr)
	if err != nil {
		return nil, fmt.Errorf("build upload request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "id", Value: c.sid})
	resp, err := c.http.Do(req)
	if err != nil {
		_ = pr.CloseWithError(err)
		<-errCh
		return nil, fmt.Errorf("upload request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := <-errCh; err != nil {
		return nil, err
	}
	var out map[string]any
	if err := decodeJSON(resp.Body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Download(ctx context.Context, params url.Values) (*http.Response, error) {
	api := c.API(APIDownload)
	vals := url.Values{}
	vals.Set("api", api.Name)
	vals.Set("version", strconv.Itoa(api.Version))
	vals.Set("method", "download")
	vals.Set("_sid", c.sid)
	for k, v := range params {
		for _, vv := range v {
			vals.Add(k, vv)
		}
	}
	u := c.endpoint + api.Path + "?" + vals.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build download request: %w", err)
	}
	req.AddCookie(&http.Cookie{Name: "id", Value: c.sid})
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}
	return resp, nil
}

func decodeJSON(r io.Reader, out any) error {
	var base baseResponse
	if err := json.NewDecoder(r).Decode(&base); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if !base.Success {
		code := 0
		if base.Error != nil {
			code = base.Error.Code
		}
		apiErr := &APIError{Code: code}
		if base.Error != nil && len(base.Error.Errors) > 0 {
			parsed := parseSubErrors(base.Error.Errors)
			if len(parsed) > 0 {
				apiErr.SubCode = parsed[0].Code
				apiErr.Path = parsed[0].Path
				apiErr.Name = parsed[0].Name
				apiErr.Reason = parsed[0].Reason
			}
		}
		return apiErr
	}
	if out == nil || len(base.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(base.Data, out); err != nil {
		return fmt.Errorf("decode data: %w", err)
	}
	return nil
}

func parseSubErrors(raw json.RawMessage) []apiSubError {
	var arr []apiSubError
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr
	}
	var one apiSubError
	if err := json.Unmarshal(raw, &one); err == nil {
		if one.Code != 0 || one.Path != "" || one.Name != "" || one.Reason != "" {
			return []apiSubError{one}
		}
	}
	var nameOnly struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &nameOnly); err == nil && nameOnly.Name != "" {
		return []apiSubError{{Name: nameOnly.Name}}
	}
	return nil
}

func EncodeJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
