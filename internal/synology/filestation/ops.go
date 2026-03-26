package filestation

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"path/filepath"

	"synocli/internal/apperr"
)

// Exists reports whether remotePath exists and whether it is a directory.
func (c *Client) Exists(ctx context.Context, remotePath string) (exists, isDir bool, err error) {
	j, encErr := EncodeJSON([]string{remotePath})
	if encErr != nil {
		return false, false, encErr
	}
	var out map[string]any
	callErr := c.Call(ctx, APIList, "getinfo", url.Values{"path": {j}}, &out)
	if callErr != nil {
		var apiErr *APIError
		if errors.As(callErr, &apiErr) && (apiErr.Code == 408 || apiErr.SubCode == 408) {
			return false, false, nil
		}
		return false, false, callErr
	}
	files := MapSliceAny(out["files"])
	if len(files) == 0 {
		return false, false, nil
	}
	if code, ok := Int64FromAny(files[0]["code"]); ok && code == 408 {
		return false, false, nil
	}
	dir := false
	if v, ok := files[0]["isdir"].(bool); ok {
		dir = v
	} else if v, ok := files[0]["isdir"].(string); ok {
		dir = v == "true" || v == "1"
	}
	return true, dir, nil
}

// EnsureDir creates dir (and all parents) on the remote if it does not already exist.
func (c *Client) EnsureDir(ctx context.Context, dir string) error {
	if dir == "" || dir == "/" {
		return nil
	}
	exists, isDir, err := c.Exists(ctx, dir)
	if err != nil {
		return err
	}
	if exists {
		if !isDir {
			return apperr.New("validation_error", fmt.Sprintf("remote path exists and is not dir: %s", dir), 1)
		}
		return nil
	}
	parent := path.Dir(dir)
	name := path.Base(dir)
	nameJSON, err := EncodeJSON([]string{name})
	if err != nil {
		return err
	}
	return c.Call(ctx, APICreateFolder, "create", url.Values{
		"folder_path":  {parent},
		"name":         {nameJSON},
		"force_parent": {"true"},
	}, nil)
}

// EnsureDeleteSafety returns an error if any of the given paths is a directory
// and recursive is false.
func (c *Client) EnsureDeleteSafety(ctx context.Context, paths []string, recursive bool) error {
	if recursive {
		return nil
	}
	j, err := EncodeJSON(paths)
	if err != nil {
		return err
	}
	var out map[string]any
	if err := c.Call(ctx, APIList, "getinfo", url.Values{"path": {j}, "additional": {`["type"]`}}, &out); err != nil {
		return err
	}
	for _, file := range MapSliceAny(out["files"]) {
		if isDir, ok := file["isdir"].(bool); ok && isDir {
			return apperr.New("validation_error", "directory deletion requires --recursive/-r", 1)
		}
	}
	return nil
}

// JoinRemote joins a remote base path with an element, normalising slashes.
func JoinRemote(base, elem string) string {
	base = strings.TrimSuffix(base, "/")
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	return path.Clean(base + "/" + strings.TrimPrefix(filepath.ToSlash(elem), "/"))
}
