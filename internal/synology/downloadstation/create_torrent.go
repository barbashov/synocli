package downloadstation

import (
	"bytes"
	"context"
	"encoding/json"
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

func openAndStatTorrent(path string) (*os.File, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open torrent file: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
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

func (c *Client) AddTorrent(ctx context.Context, sid, torrentPath, destination string) ([]string, error) {
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
	defer func() { _ = resp.Body.Close() }()
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

func (c *Client) addTorrentDS2Direct(ctx context.Context, sid, torrentPath, destination string) ([]string, []string, error) {
	f, size, err := openAndStatTorrent(torrentPath)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()

	apiName := c.apiName()
	typeJSON, err := json.Marshal("file")
	if err != nil {
		return nil, nil, fmt.Errorf("encode type: %w", err)
	}
	fileJSON, err := json.Marshal([]string{"torrent"})
	if err != nil {
		return nil, nil, fmt.Errorf("encode file field: %w", err)
	}
	fields := [][2]string{
		{"api", apiName},
		{"method", "create"},
		{"version", strconv.Itoa(c.Version)},
		{"type", string(typeJSON)},
		{"file", string(fileJSON)},
		{"create_list", "false"},
		{"size", strconv.FormatInt(size, 10)},
	}
	if destination != "" {
		destJSON, err := json.Marshal(destination)
		if err != nil {
			return nil, nil, fmt.Errorf("encode destination: %w", err)
		}
		fields = append(fields, [2]string{"destination", string(destJSON)})
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
	defer func() { _ = resp.Body.Close() }()
	return decodeCreate(resp.Body)
}
