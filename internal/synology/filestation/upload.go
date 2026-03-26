package filestation

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"synocli/internal/apperr"
)

// UploadOne uploads a single local file to remotePath.
func (c *Client) UploadOne(ctx context.Context, localPath, remotePath string, parents, overwrite, skipExisting bool) (map[string]any, error) {
	params, err := c.buildUploadParams(ctx, remotePath, parents, overwrite, skipExisting)
	if err != nil {
		return nil, err
	}
	out, err := c.Upload(ctx, params, localPath)
	if err != nil {
		return nil, err
	}
	out["local_path"] = localPath
	out["remote_path"] = remotePath
	return out, nil
}

// UploadRecursiveCP uploads localDir into remotePath, mirroring the directory tree.
// If remotePath already exists as a directory the local dir is placed inside it.
func (c *Client) UploadRecursiveCP(ctx context.Context, localDir, remotePath string, parents, overwrite, skipExisting bool) (map[string]any, error) {
	exists, isDir, err := c.Exists(ctx, remotePath)
	if err != nil {
		return nil, err
	}
	if exists && !isDir {
		return nil, apperr.New("validation_error", "remote destination exists and is not a directory", 1)
	}
	targetRoot := remotePath
	if exists && isDir {
		targetRoot = JoinRemote(remotePath, filepath.Base(localDir))
	}
	if err := c.EnsureDir(ctx, targetRoot); err != nil {
		return nil, err
	}
	uploaded := 0
	skipped := 0
	err = filepath.WalkDir(localDir, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(localDir, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		remoteCurrent := JoinRemote(targetRoot, filepath.ToSlash(rel))
		if d.IsDir() {
			return c.EnsureDir(ctx, remoteCurrent)
		}
		parent := path.Dir(remoteCurrent)
		if skipExisting {
			ex, _, err := c.Exists(ctx, remoteCurrent)
			if err != nil {
				return err
			}
			if ex {
				skipped++
				return nil
			}
		}
		params, err := c.buildUploadParams(ctx, parent, parents, overwrite, skipExisting)
		if err != nil {
			return err
		}
		// Upload may succeed server-side but return an error (e.g. malformed
		// response). If the file landed under its local name we can still
		// rename it to the intended remote name, so only propagate the
		// upload error when the rename fallback also fails.
		if _, err := c.Upload(ctx, params, p); err != nil {
			if renameErr := c.RenameUploaded(ctx, parent, filepath.Base(p), path.Base(remoteCurrent)); renameErr != nil {
				return err
			}
		}
		if filepath.Base(p) != path.Base(remoteCurrent) {
			if err := c.RenameUploaded(ctx, parent, filepath.Base(p), path.Base(remoteCurrent)); err != nil {
				return err
			}
		}
		uploaded++
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"local_path": localDir, "remote_path": targetRoot, "uploaded_files": uploaded, "skipped_files": skipped}, nil
}

// RenameUploaded renames a file at parent/oldName to newName on the remote.
func (c *Client) RenameUploaded(ctx context.Context, parent, oldName, newName string) error {
	if oldName == newName {
		return nil
	}
	p := JoinRemote(parent, oldName)
	pj, err := EncodeJSON([]string{p})
	if err != nil {
		return err
	}
	nj, err := EncodeJSON([]string{newName})
	if err != nil {
		return err
	}
	return c.Call(ctx, APIRename, "rename", url.Values{"path": {pj}, "name": {nj}}, nil)
}

func (c *Client) buildUploadParams(ctx context.Context, remoteDir string, parents, overwrite, skipExisting bool) (map[string]string, error) {
	api := c.API(APIUpload)
	params := map[string]string{"path": remoteDir, "create_parents": fmt.Sprintf("%t", parents)}
	if api.Version >= 3 {
		switch {
		case overwrite:
			params["overwrite"] = "overwrite"
		case skipExisting:
			params["overwrite"] = "skip"
		default:
			params["overwrite"] = "error"
		}
		return params, nil
	}
	if overwrite {
		params["overwrite"] = "true"
	} else {
		params["overwrite"] = "false"
	}
	// Version 2 API has only bool overwrite; skip is handled by caller for recursive mode.
	return params, nil
}
