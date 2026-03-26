package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"synocli/internal/apperr"
	"synocli/internal/output"
	"synocli/internal/synology/filestation"
)

func newFSCmd(ac *appContext) *cobra.Command {
	cmd := &cobra.Command{Use: "fs", Aliases: []string{"filestation"}, Short: "File Station commands"}
	cmd.AddCommand(
		newFSInfoCmd(ac),
		newFSSharesCmd(ac),
		newFSListCmd(ac),
		newFSGetCmd(ac),
		newFSMkdirCmd(ac),
		newFSRenameCmd(ac),
		newFSCopyCmd(ac, false),
		newFSCopyCmd(ac, true),
		newFSDeleteCmd(ac),
		newFSUploadCmd(ac),
		newFSDownloadCmd(ac),
		newFSSearchCmd(ac),
		newFSSearchResultsCmd(ac),
		newFSSearchStopCmd(ac),
		newFSSearchClearCmd(ac),
		newFSDirSizeCmd(ac),
		newFSDirSizeStatusCmd(ac),
		newFSDirSizeStopCmd(ac),
		newFSMD5Cmd(ac),
		newFSMD5StatusCmd(ac),
		newFSMD5StopCmd(ac),
		newFSExtractCmd(ac),
		newFSExtractStatusCmd(ac),
		newFSExtractStopCmd(ac),
		newFSCompressCmd(ac),
		newFSCompressStatusCmd(ac),
		newFSCompressStopCmd(ac),
		newFSTasksCmd(ac),
		newFSTasksClearCmd(ac),
		newFSWatchCmd(ac),
	)
	return cmd
}

func newFSInfoCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Get File Station info",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withFSSession(cmd, joinCommand("fs", "info"), func(ctx context.Context, s *session) (any, error) {
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APIInfo, "get", nil, &out); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				printKVBlock(ac.out, "File Station", []kvField{
					{Label: "Hostname", Value: valueFromMap(out, "hostname")},
					{Label: "Is Manager", Value: valueFromMap(out, "is_manager")},
				})
				return nil, nil
			})
		},
	}
}

func newFSSharesCmd(ac *appContext) *cobra.Command {
	var offset, limit int
	return &cobra.Command{
		Use:   "shares",
		Short: "List shared folders",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withFSSession(cmd, joinCommand("fs", "shares"), func(ctx context.Context, s *session) (any, error) {
				params := makeValues(
					"offset", fmt.Sprintf("%d", offset),
					"limit", fmt.Sprintf("%d", limit),
				)
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APIList, "list_share", params, &out); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				shares := mapSliceAny(out["shares"])
				rows := make([][]string, 0, len(shares))
				for _, sh := range shares {
					rows = append(rows, []string{valueFromMap(sh, "name"), valueFromMap(sh, "path")})
				}
				printKVBlock(ac.out, "Shares", []kvField{{Label: "Count", Value: fmt.Sprintf("%d", len(shares))}})
				printTable(ac.out, []string{"Name", "Path"}, rows)
				return nil, nil
			})
		},
	}
}

func newFSListCmd(ac *appContext) *cobra.Command {
	var offset, limit int
	var sortBy, sortDirection, pattern, filetype string
	var recursive bool
	var additional []string
	cmd := &cobra.Command{
		Use:     "list <folder-path>",
		Aliases: []string{"ls"},
		Short:   "List files in folder",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withFSSession(cmd, joinCommand("fs", "list"), func(ctx context.Context, s *session) (any, error) {
				params := makeValues("folder_path", args[0])
				params.Set("offset", fmt.Sprintf("%d", offset))
				params.Set("limit", fmt.Sprintf("%d", limit))
				if sortBy != "" {
					params.Set("sort_by", sortBy)
				}
				if sortDirection != "" {
					params.Set("sort_direction", sortDirection)
				}
				if pattern != "" {
					params.Set("pattern", pattern)
				}
				if filetype != "" {
					params.Set("filetype", filetype)
				}
				params.Set("recursive", fmt.Sprintf("%t", recursive))
				if len(additional) > 0 {
					j, err := filestation.EncodeJSON(additional)
					if err != nil {
						return nil, err
					}
					params.Set("additional", j)
				}
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APIList, "list", params, &out); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				files := mapSliceAny(out["files"])
				rows := make([][]string, 0, len(files))
				for _, f := range files {
					rows = append(rows, []string{
						valueFromMap(f, "name"),
						valueFromMap(f, "path"),
						fsListSizeDisplay(f),
						fsListMTimeDisplay(f),
					})
				}
				printKVBlock(ac.out, "Folder", []kvField{{Label: "Path", Value: args[0]}, {Label: "Entries", Value: fmt.Sprintf("%d", len(files))}})
				printTable(ac.out, []string{"Name", "Path", "Size", "MTime"}, rows)
				return nil, nil
			})
		},
	}
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Limit")
	cmd.Flags().StringVar(&sortBy, "sort-by", "", "Sort by")
	cmd.Flags().StringVar(&sortDirection, "sort-direction", "", "Sort direction asc/desc")
	cmd.Flags().StringVar(&pattern, "pattern", "", "Name pattern")
	cmd.Flags().StringVar(&filetype, "filetype", "", "file/dir/all")
	cmd.Flags().BoolVar(&recursive, "recursive", false, "Recursive listing")
	cmd.Flags().StringSliceVar(&additional, "additional", []string{"real_path", "size", "time", "type"}, "Additional fields")
	return cmd
}

func newFSGetCmd(ac *appContext) *cobra.Command {
	var additional []string
	cmd := &cobra.Command{
		Use:   "get <path> [<path>...]",
		Short: "Get file info",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withFSSession(cmd, joinCommand("fs", "get"), func(ctx context.Context, s *session) (any, error) {
				pathsJSON, err := filestation.EncodeJSON(args)
				if err != nil {
					return nil, err
				}
				params := makeValues("path", pathsJSON)
				if len(additional) > 0 {
					j, err := filestation.EncodeJSON(additional)
					if err != nil {
						return nil, err
					}
					params.Set("additional", j)
				}
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APIList, "getinfo", params, &out); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				files := mapSliceAny(out["files"])
				rows := make([][]string, 0, len(files))
				for _, f := range files {
					size := fsListSizeDisplay(f)
					rows = append(rows, []string{valueFromMap(f, "path"), valueFromMap(f, "name"), valueFromMap(f, "isdir"), size})
				}
				printTable(ac.out, []string{"Path", "Name", "Dir", "Size"}, rows)
				return nil, nil
			})
		},
	}
	cmd.Flags().StringSliceVar(&additional, "additional", []string{"real_path", "size", "time", "type", "perm"}, "Additional fields")
	return cmd
}

func newFSMkdirCmd(ac *appContext) *cobra.Command {
	var parents bool
	cmd := &cobra.Command{
		Use:   "mkdir <parent-path> <name> [<name>...]",
		Short: "Create folder(s)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withFSSession(cmd, joinCommand("fs", "mkdir"), func(ctx context.Context, s *session) (any, error) {
				namesJSON, err := filestation.EncodeJSON(args[1:])
				if err != nil {
					return nil, err
				}
				params := makeValues("folder_path", args[0], "name", namesJSON, "force_parent", fmt.Sprintf("%t", parents))
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APICreateFolder, "create", params, &out); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				printKVBlock(ac.out, "Create Folder", []kvField{{Label: "Parent", Value: args[0]}, {Label: "Names", Value: strings.Join(args[1:], ", ")}})
				return nil, nil
			})
		},
	}
	cmd.Flags().BoolVar(&parents, "parents", false, "Create parent folders")
	return cmd
}

func newFSRenameCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <path> <new-name>",
		Short: "Rename file/folder",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withFSSession(cmd, joinCommand("fs", "rename"), func(ctx context.Context, s *session) (any, error) {
				pathsJSON, err := filestation.EncodeJSON([]string{args[0]})
				if err != nil {
					return nil, err
				}
				namesJSON, err := filestation.EncodeJSON([]string{args[1]})
				if err != nil {
					return nil, err
				}
				params := makeValues("path", pathsJSON, "name", namesJSON)
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APIRename, "rename", params, &out); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				printKVBlock(ac.out, "Rename", []kvField{{Label: "Path", Value: args[0]}, {Label: "New Name", Value: args[1]}})
				return nil, nil
			})
		},
	}
}

func newFSCopyCmd(ac *appContext, move bool) *cobra.Command {
	verb := "copy"
	removeSrc := "false"
	if move {
		verb = "move"
		removeSrc = "true"
	}
	var dest string
	var overwrite bool
	var skipExisting bool
	var async bool
	var interval time.Duration
	var maxWait time.Duration
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s <path> [<path>...] --to <destination>", verb),
		Short: capitalizeWord(verb) + " files/folders",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dest == "" {
				return apperr.New("validation_error", "--to is required", 1)
			}
			if overwrite && skipExisting {
				return apperr.New("validation_error", "use only one of --overwrite or --skip-existing", 1)
			}
			if err := validatePositiveDuration("--interval", interval); err != nil {
				return err
			}
			return ac.withFSSession(cmd, joinCommand("fs", verb), func(ctx context.Context, s *session) (any, error) {
				if err := ensureRemoteDir(ctx, s.fsClient, dest); err != nil {
					return nil, err
				}
				pathsJSON, err := filestation.EncodeJSON(args)
				if err != nil {
					return nil, err
				}
				params := makeValues("path", pathsJSON, "dest_folder_path", dest, "remove_src", removeSrc)
				if overwrite {
					params.Set("overwrite", "true")
				}
				if skipExisting {
					params.Set("overwrite", "skip")
				}
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APICopyMove, "start", params, &out); err != nil {
					return nil, err
				}
				taskID := firstTaskID(out)
				if taskID == "" {
					return nil, apperr.New("internal_error", "copy/move task id missing", 1)
				}
				if !async {
					status, err := waitFSTask(ctx, s.fsClient, filestation.APICopyMove, taskID, interval, maxWait)
					if err != nil {
						return nil, err
					}
					out["status"] = status
					out["waited"] = true
				}
				if ac.opts.JSON {
					out["task_id"] = taskID
					return out, nil
				}
				printKVBlock(ac.out, capitalizeWord(verb), []kvField{{Label: "Task ID", Value: taskID}, {Label: "Destination", Value: dest}})
				return nil, nil
			})
		},
	}
	if move {
		cmd.Aliases = []string{"mv"}
	} else {
		cmd.Aliases = []string{"cp"}
	}
	cmd.Flags().StringVar(&dest, "to", "", "Destination folder path")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing files")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "Skip existing files")
	cmd.Flags().BoolVar(&async, "async", false, "Do not wait for completion")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	cmd.Flags().DurationVar(&maxWait, "max-wait", 0, "Maximum wait duration (0 means unlimited)")
	return cmd
}

func newFSDeleteCmd(ac *appContext) *cobra.Command {
	var recursive bool
	var async bool
	var interval time.Duration
	var maxWait time.Duration
	cmd := &cobra.Command{
		Use:   "delete <path> [<path>...]",
		Short: "Delete files/folders",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePositiveDuration("--interval", interval); err != nil {
				return err
			}
			return ac.withFSSession(cmd, joinCommand("fs", "delete"), func(ctx context.Context, s *session) (any, error) {
				if err := ensureDeleteSafety(ctx, s.fsClient, args, recursive); err != nil {
					return nil, err
				}
				pathsJSON, err := filestation.EncodeJSON(args)
				if err != nil {
					return nil, err
				}
				if !async {
					params := makeValues("path", pathsJSON, "recursive", fmt.Sprintf("%t", recursive))
					out := map[string]any{}
					if err := s.fsClient.Call(ctx, filestation.APIDelete, "delete", params, &out); err != nil {
						return nil, err
					}
					out["waited"] = true
					if ac.opts.JSON {
						return out, nil
					}
					printKVBlock(ac.out, "Delete", []kvField{{Label: "Paths", Value: strings.Join(args, ", ")}})
					return nil, nil
				}
				params := makeValues("path", pathsJSON, "recursive", fmt.Sprintf("%t", recursive))
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APIDelete, "start", params, &out); err != nil {
					return nil, err
				}
				if out == nil {
					out = map[string]any{}
				}
				taskID := firstTaskID(out)
				if taskID == "" {
					return nil, apperr.New("internal_error", "delete task id missing", 1)
				}
				status, err := waitFSTask(ctx, s.fsClient, filestation.APIDelete, taskID, interval, maxWait)
				if err != nil {
					return nil, err
				}
				out["task_id"] = taskID
				out["status"] = status
				if ac.opts.JSON {
					return out, nil
				}
				printKVBlock(ac.out, "Delete", []kvField{{Label: "Task ID", Value: taskID}})
				return nil, nil
			})
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Delete directories recursively")
	cmd.Flags().BoolVar(&async, "async", false, "Run async delete task")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	cmd.Flags().DurationVar(&maxWait, "max-wait", 0, "Maximum wait duration (0 means unlimited)")
	return cmd
}

func newFSUploadCmd(ac *appContext) *cobra.Command {
	var parents bool
	var overwrite bool
	var skipExisting bool
	cmd := &cobra.Command{
		Use:   "upload <local-path> <remote-path>",
		Short: "Upload file or directory",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if overwrite && skipExisting {
				return apperr.New("validation_error", "use only one of --overwrite or --skip-existing", 1)
			}
			return ac.withFSSession(cmd, joinCommand("fs", "upload"), func(ctx context.Context, s *session) (any, error) {
				st, err := os.Stat(args[0])
				if err != nil {
					return nil, fmt.Errorf("stat local path: %w", err)
				}
				if st.IsDir() {
					res, err := uploadRecursiveCP(ctx, s.fsClient, args[0], args[1], parents, overwrite, skipExisting)
					if err != nil {
						return nil, err
					}
					if ac.opts.JSON {
						return res, nil
					}
					printKVBlock(ac.out, "Upload Directory", []kvField{{Label: "Local", Value: args[0]}, {Label: "Remote", Value: args[1]}, {Label: "Files", Value: fmt.Sprintf("%v", res["uploaded_files"])}})
					return nil, nil
				}
				res, err := uploadOne(ctx, s.fsClient, args[0], args[1], parents, overwrite, skipExisting)
				if err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return res, nil
				}
				printKVBlock(ac.out, "Upload", []kvField{{Label: "Local", Value: args[0]}, {Label: "Remote", Value: args[1]}})
				return nil, nil
			})
		},
	}
	cmd.Flags().BoolVar(&parents, "parents", true, "Create parent dirs on remote")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing files")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "Skip existing files")
	return cmd
}

func newFSDownloadCmd(ac *appContext) *cobra.Command {
	var outputPath string
	var mode string
	cmd := &cobra.Command{
		Use:   "download <remote-path> [<remote-path>...]",
		Short: "Download file or folder archive",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if outputPath == "" {
				return apperr.New("validation_error", "--output is required", 1)
			}
			return ac.withFSSession(cmd, joinCommand("fs", "download"), func(ctx context.Context, s *session) (any, error) {
				pathsJSON, err := filestation.EncodeJSON(args)
				if err != nil {
					return nil, err
				}
				resp, err := s.fsClient.Download(ctx, makeValues("path", pathsJSON, "mode", mode))
				if err != nil {
					return nil, err
				}
				defer func() { _ = resp.Body.Close() }()
				if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "json") {
					var out map[string]any
					if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
						return nil, fmt.Errorf("decode error response: %w", err)
					}
					return nil, fmt.Errorf("download failed: %v", out)
				}
				f, err := os.Create(outputPath)
				if err != nil {
					return nil, fmt.Errorf("create output file: %w", err)
				}
				defer func() { _ = f.Close() }()
				n, err := io.Copy(f, resp.Body)
				if err != nil {
					return nil, fmt.Errorf("write output file: %w", err)
				}
				data := map[string]any{"output": outputPath, "bytes": n, "paths": args}
				if ac.opts.JSON {
					return data, nil
				}
				printKVBlock(ac.out, "Download", []kvField{{Label: "Output", Value: outputPath}, {Label: "Bytes", Value: fmt.Sprintf("%d", n)}})
				return nil, nil
			})
		},
	}
	cmd.Flags().StringVar(&outputPath, "output", "", "Output file path")
	cmd.Flags().StringVar(&mode, "mode", "download", "download|open")
	return cmd
}

func newFSSearchCmd(ac *appContext) *cobra.Command {
	var pattern string
	var recursive bool
	var filetype string
	var async bool
	var interval time.Duration
	var maxWait time.Duration
	cmd := &cobra.Command{
		Use:   "search <folder-path>",
		Short: "Search files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if pattern == "" {
				return apperr.New("validation_error", "--pattern is required", 1)
			}
			if err := validatePositiveDuration("--interval", interval); err != nil {
				return err
			}
			return ac.withFSSession(cmd, joinCommand("fs", "search"), func(ctx context.Context, s *session) (any, error) {
				params := makeValues("folder_path", args[0], "pattern", pattern, "recursive", fmt.Sprintf("%t", recursive))
				if filetype != "" {
					params.Set("filetype", filetype)
				}
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APISearch, "start", params, &out); err != nil {
					return nil, err
				}
				taskID := firstTaskID(out)
				if taskID == "" {
					return nil, apperr.New("internal_error", "search task id missing", 1)
				}
				res := map[string]any{"task_id": taskID}
				if !async {
					snapshot, err := waitSearch(ctx, s.fsClient, taskID, interval, maxWait)
					if err != nil {
						return nil, err
					}
					res["result"] = snapshot
				}
				if ac.opts.JSON {
					return res, nil
				}
				printKVBlock(ac.out, "Search", []kvField{{Label: "Task ID", Value: taskID}, {Label: "Pattern", Value: pattern}})
				return nil, nil
			})
		},
	}
	cmd.Flags().StringVar(&pattern, "pattern", "", "Pattern")
	cmd.Flags().BoolVar(&recursive, "recursive", true, "Recursive search")
	cmd.Flags().StringVar(&filetype, "filetype", "", "file/dir/all")
	cmd.Flags().BoolVar(&async, "async", false, "Do not wait for completion")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	cmd.Flags().DurationVar(&maxWait, "max-wait", 0, "Maximum wait duration (0 means unlimited)")
	return cmd
}

func newFSSearchResultsCmd(ac *appContext) *cobra.Command {
	var offset, limit int
	return &cobra.Command{
		Use:   "search-results <task-id>",
		Short: "Get search results",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withFSSession(cmd, joinCommand("fs", "search-results"), func(ctx context.Context, s *session) (any, error) {
				params := makeValues("taskid", args[0], "offset", fmt.Sprintf("%d", offset), "limit", fmt.Sprintf("%d", limit))
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APISearch, "list", params, &out); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				files := mapSliceAny(out["files"])
				rows := make([][]string, 0, len(files))
				for _, f := range files {
					rows = append(rows, []string{valueFromMap(f, "path"), valueFromMap(f, "name")})
				}
				printTable(ac.out, []string{"Path", "Name"}, rows)
				return nil, nil
			})
		},
	}
}

func newFSSearchStopCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "search-stop <task-id>",
		Short: "Stop search task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withFSSession(cmd, joinCommand("fs", "search-stop"), func(ctx context.Context, s *session) (any, error) {
				if err := s.fsClient.Call(ctx, filestation.APISearch, "stop", makeValues("taskid", args[0]), nil); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return map[string]any{"task_id": args[0], "stopped": true}, nil
				}
				printKVBlock(ac.out, "Search Stop", []kvField{{Label: "Task ID", Value: args[0]}})
				return nil, nil
			})
		},
	}
}

func newFSSearchClearCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "search-clear",
		Short: "Clear search tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withFSSession(cmd, joinCommand("fs", "search-clear"), func(ctx context.Context, s *session) (any, error) {
				if err := s.fsClient.Call(ctx, filestation.APISearch, "clean", nil, nil); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return map[string]any{"cleared": true}, nil
				}
				printKVBlock(ac.out, "Search", []kvField{{Label: "Cleared", Value: "true"}})
				return nil, nil
			})
		},
	}
}

func newTaskStartCmd(ac *appContext, apiKey, cmdName, title, method string, paramsFn func(args []string) (map[string]string, error)) *cobra.Command {
	var async bool
	var interval time.Duration
	var maxWait time.Duration
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s <path>", cmdName),
		Short: title,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePositiveDuration("--interval", interval); err != nil {
				return err
			}
			return ac.withFSSession(cmd, joinCommand("fs", cmdName), func(ctx context.Context, s *session) (any, error) {
				pairs, err := paramsFn(args)
				if err != nil {
					return nil, err
				}
				params := makeValuesFromMap(pairs)
				var out map[string]any
				if err := s.fsClient.Call(ctx, apiKey, method, params, &out); err != nil {
					return nil, err
				}
				taskID := firstTaskID(out)
				if taskID == "" {
					return nil, apperr.New("internal_error", "task id missing", 1)
				}
				if !async {
					status, err := waitFSTask(ctx, s.fsClient, apiKey, taskID, interval, maxWait)
					if err != nil {
						return nil, err
					}
					out["status"] = status
				}
				out["task_id"] = taskID
				if ac.opts.JSON {
					return out, nil
				}
				printKVBlock(ac.out, title, []kvField{{Label: "Task ID", Value: taskID}})
				return nil, nil
			})
		},
	}
	cmd.Flags().BoolVar(&async, "async", false, "Do not wait for completion")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	cmd.Flags().DurationVar(&maxWait, "max-wait", 0, "Maximum wait duration (0 means unlimited)")
	return cmd
}

func newTaskStatusCmd(ac *appContext, apiKey, cmdName, title string) *cobra.Command {
	return &cobra.Command{
		Use:   fmt.Sprintf("%s <task-id>", cmdName),
		Short: title,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withFSSession(cmd, joinCommand("fs", cmdName), func(ctx context.Context, s *session) (any, error) {
				var out map[string]any
				if err := s.fsClient.Call(ctx, apiKey, "status", makeValues("taskid", args[0]), &out); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				printKVBlock(ac.out, title, []kvField{{Label: "Task ID", Value: args[0]}, {Label: "Finished", Value: valueFromMap(out, "finished")}})
				return nil, nil
			})
		},
	}
}

func newTaskStopCmd(ac *appContext, apiKey, cmdName, title string) *cobra.Command {
	return &cobra.Command{
		Use:   fmt.Sprintf("%s <task-id>", cmdName),
		Short: title,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withFSSession(cmd, joinCommand("fs", cmdName), func(ctx context.Context, s *session) (any, error) {
				if err := s.fsClient.Call(ctx, apiKey, "stop", makeValues("taskid", args[0]), nil); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return map[string]any{"task_id": args[0], "stopped": true}, nil
				}
				printKVBlock(ac.out, title, []kvField{{Label: "Task ID", Value: args[0]}})
				return nil, nil
			})
		},
	}
}

func newFSDirSizeCmd(ac *appContext) *cobra.Command {
	return newTaskStartCmd(ac, filestation.APIDirSize, "dir-size", "Dir Size", "start", func(args []string) (map[string]string, error) {
		j, err := filestation.EncodeJSON(args)
		if err != nil {
			return nil, err
		}
		return map[string]string{"path": j}, nil
	})
}

func newFSDirSizeStatusCmd(ac *appContext) *cobra.Command {
	return newTaskStatusCmd(ac, filestation.APIDirSize, "dir-size-status", "Dir Size Status")
}

func newFSDirSizeStopCmd(ac *appContext) *cobra.Command {
	return newTaskStopCmd(ac, filestation.APIDirSize, "dir-size-stop", "Dir Size Stop")
}

func newFSMD5Cmd(ac *appContext) *cobra.Command {
	return newTaskStartCmd(ac, filestation.APIMD5, "md5", "MD5", "start", func(args []string) (map[string]string, error) {
		return map[string]string{"file_path": args[0]}, nil
	})
}

func newFSMD5StatusCmd(ac *appContext) *cobra.Command {
	return newTaskStatusCmd(ac, filestation.APIMD5, "md5-status", "MD5 Status")
}

func newFSMD5StopCmd(ac *appContext) *cobra.Command {
	return newTaskStopCmd(ac, filestation.APIMD5, "md5-stop", "MD5 Stop")
}

func newFSExtractCmd(ac *appContext) *cobra.Command {
	var dest string
	var overwrite bool
	var keepDir bool
	var createSubfolder bool
	var password string
	cmd := newTaskStartCmd(ac, filestation.APIExtract, "extract", "Extract Archive", "start", func(args []string) (map[string]string, error) {
		if dest == "" {
			return nil, apperr.New("validation_error", "--to is required", 1)
		}
		return map[string]string{
			"file_path":        args[0],
			"dest_folder_path": dest,
			"overwrite":        fmt.Sprintf("%t", overwrite),
			"keep_dir":         fmt.Sprintf("%t", keepDir),
			"create_subfolder": fmt.Sprintf("%t", createSubfolder),
			"extract_password": password,
		}, nil
	})
	cmd.Flags().StringVar(&dest, "to", "", "Destination folder")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing files")
	cmd.Flags().BoolVar(&keepDir, "keep-dir", false, "Keep directory structure")
	cmd.Flags().BoolVar(&createSubfolder, "create-subfolder", false, "Create subfolder")
	cmd.Flags().StringVar(&password, "password", "", "Archive password")
	return cmd
}

func newFSExtractStatusCmd(ac *appContext) *cobra.Command {
	return newTaskStatusCmd(ac, filestation.APIExtract, "extract-status", "Extract Status")
}

func newFSExtractStopCmd(ac *appContext) *cobra.Command {
	return newTaskStopCmd(ac, filestation.APIExtract, "extract-stop", "Extract Stop")
}

func newFSCompressCmd(ac *appContext) *cobra.Command {
	var dest string
	var format string
	var level int
	var mode string
	var password string
	cmd := newTaskStartCmd(ac, filestation.APICompress, "compress", "Compress", "start", func(args []string) (map[string]string, error) {
		if dest == "" {
			return nil, apperr.New("validation_error", "--to is required", 1)
		}
		j, err := filestation.EncodeJSON(args)
		if err != nil {
			return nil, err
		}
		return map[string]string{
			"path":           j,
			"dest_file_path": dest,
			"format":         format,
			"level":          fmt.Sprintf("%d", level),
			"mode":           mode,
			"password":       password,
		}, nil
	})
	cmd.Flags().StringVar(&dest, "to", "", "Destination archive path")
	cmd.Flags().StringVar(&format, "format", "zip", "zip|7z")
	cmd.Flags().IntVar(&level, "level", 5, "Compression level")
	cmd.Flags().StringVar(&mode, "mode", "add", "add|store")
	cmd.Flags().StringVar(&password, "password", "", "Archive password")
	return cmd
}

func newFSCompressStatusCmd(ac *appContext) *cobra.Command {
	return newTaskStatusCmd(ac, filestation.APICompress, "compress-status", "Compress Status")
}

func newFSCompressStopCmd(ac *appContext) *cobra.Command {
	return newTaskStopCmd(ac, filestation.APICompress, "compress-stop", "Compress Stop")
}

func newFSTasksCmd(ac *appContext) *cobra.Command {
	var offset, limit int
	var sortBy, sortDirection string
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "List File Station background tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withFSSession(cmd, joinCommand("fs", "tasks"), func(ctx context.Context, s *session) (any, error) {
				params := makeValues("offset", fmt.Sprintf("%d", offset), "limit", fmt.Sprintf("%d", limit))
				if sortBy != "" {
					params.Set("sort_by", sortBy)
				}
				if sortDirection != "" {
					params.Set("sort_direction", sortDirection)
				}
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APIBackgroundTask, "list", params, &out); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				printBackgroundTasks(ac.out, out)
				return nil, nil
			})
		},
	}
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset")
	cmd.Flags().IntVar(&limit, "limit", 100, "Limit")
	cmd.Flags().StringVar(&sortBy, "sort-by", "", "Sort by")
	cmd.Flags().StringVar(&sortDirection, "sort-direction", "", "Sort direction")
	return cmd
}

func newFSTasksClearCmd(ac *appContext) *cobra.Command {
	var ids []string
	cmd := &cobra.Command{
		Use:   "tasks-clear",
		Short: "Clear finished background tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withFSSession(cmd, joinCommand("fs", "tasks-clear"), func(ctx context.Context, s *session) (any, error) {
				params := makeValues()
				if len(ids) > 0 {
					j, err := filestation.EncodeJSON(ids)
					if err != nil {
						return nil, err
					}
					params.Set("taskid", j)
				}
				if err := s.fsClient.Call(ctx, filestation.APIBackgroundTask, "clear_finished", params, nil); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return map[string]any{"cleared": true, "task_ids": ids}, nil
				}
				printKVBlock(ac.out, "Tasks Clear", []kvField{{Label: "Cleared", Value: "true"}})
				return nil, nil
			})
		},
	}
	cmd.Flags().StringSliceVar(&ids, "task-id", nil, "Task IDs")
	return cmd
}

func newFSWatchCmd(ac *appContext) *cobra.Command {
	cmd := &cobra.Command{Use: "watch", Short: "Watch File Station tasks or folders"}
	cmd.AddCommand(newFSWatchTasksCmd(ac), newFSWatchFolderCmd(ac))
	return cmd
}

func newFSWatchTasksCmd(ac *appContext) *cobra.Command {
	var interval time.Duration
	var limit int
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "Watch background tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePositiveDuration("--interval", interval); err != nil {
				return err
			}
			return ac.withStreamingFSSession(cmd, joinCommand("fs", "watch", "tasks"), func(ctx context.Context, s *session) error {
				ui := newHumanUI(ac.out)
				for {
					var out map[string]any
					if err := s.fsClient.Call(ctx, filestation.APIBackgroundTask, "list", makeValues("offset", "0", "limit", fmt.Sprintf("%d", limit)), &out); err != nil {
						return err
					}
					if ac.opts.JSON {
						env := output.NewEnvelope(true, joinCommand("fs", "watch", "tasks"), s.endpoint, s.start)
						env.Meta.APIVersion = s.apiVersions
						env.Data = map[string]any{"event": "snapshot", "mode": "tasks", "snapshot": out}
						if err := output.WriteJSONLine(ac.out, env); err != nil {
							return err
						}
					} else {
						if ui.tty {
							_, _ = fmt.Fprint(ac.out, ansiClearScreen)
						}
						printKVBlock(ac.out, "File Station Task Watch", []kvField{{Label: "Timestamp", Value: time.Now().Format(time.RFC3339)}})
						printBackgroundTasks(ac.out, out)
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(interval):
					}
				}
			})
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max tasks per snapshot")
	return cmd
}

func newFSWatchFolderCmd(ac *appContext) *cobra.Command {
	var interval time.Duration
	var recursive bool
	var additional []string
	cmd := &cobra.Command{
		Use:   "folder <folder-path>",
		Short: "Watch folder listing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePositiveDuration("--interval", interval); err != nil {
				return err
			}
			return ac.withStreamingFSSession(cmd, joinCommand("fs", "watch", "folder"), func(ctx context.Context, s *session) error {
				ui := newHumanUI(ac.out)
				for {
					params := makeValues("folder_path", args[0], "offset", "0", "limit", "1000", "recursive", fmt.Sprintf("%t", recursive))
					if len(additional) > 0 {
						j, err := filestation.EncodeJSON(additional)
						if err != nil {
							return err
						}
						params.Set("additional", j)
					}
					var out map[string]any
					if err := s.fsClient.Call(ctx, filestation.APIList, "list", params, &out); err != nil {
						return err
					}
					if ac.opts.JSON {
						env := output.NewEnvelope(true, joinCommand("fs", "watch", "folder"), s.endpoint, s.start)
						env.Meta.APIVersion = s.apiVersions
						env.Data = map[string]any{"event": "snapshot", "mode": "folder", "path": args[0], "snapshot": out}
						if err := output.WriteJSONLine(ac.out, env); err != nil {
							return err
						}
					} else {
						if ui.tty {
							_, _ = fmt.Fprint(ac.out, ansiClearScreen)
						}
						printKVBlock(ac.out, "File Station Folder Watch", []kvField{{Label: "Timestamp", Value: time.Now().Format(time.RFC3339)}, {Label: "Path", Value: args[0]}})
						files := mapSliceAny(out["files"])
						rows := make([][]string, 0, len(files))
						for _, f := range files {
							rows = append(rows, []string{valueFromMap(f, "name"), valueFromMap(f, "path"), valueFromMap(f, "isdir")})
						}
						printTable(ac.out, []string{"Name", "Path", "Dir"}, rows)
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(interval):
					}
				}
			})
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	cmd.Flags().BoolVar(&recursive, "recursive", false, "Recursive listing")
	cmd.Flags().StringSliceVar(&additional, "additional", []string{"real_path", "size", "time", "type"}, "Additional fields")
	return cmd
}

func makeValues(kv ...string) mapValues {
	vals := mapValues{}
	for i := 0; i+1 < len(kv); i += 2 {
		vals.Set(kv[i], kv[i+1])
	}
	return vals
}

type mapValues = url.Values

func makeValuesFromMap(m map[string]string) mapValues {
	v := mapValues{}
	for k, val := range m {
		if strings.TrimSpace(val) == "" {
			continue
		}
		v.Set(k, val)
	}
	return v
}

func firstTaskID(m map[string]any) string {
	for _, key := range []string{"taskid", "task_id", "taskId"} {
		if v, ok := m[key]; ok {
			s := firstString(v)
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func firstString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				return s
			}
		}
	case []string:
		for _, s := range t {
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func waitFSTask(ctx context.Context, c *filestation.Client, apiKey, taskID string, interval, maxWait time.Duration) (map[string]any, error) {
	deadline := time.Time{}
	if maxWait > 0 {
		deadline = time.Now().Add(maxWait)
	}
	for {
		var out map[string]any
		if err := c.Call(ctx, apiKey, "status", makeValues("taskid", taskID), &out); err != nil {
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

func waitSearch(ctx context.Context, c *filestation.Client, taskID string, interval, maxWait time.Duration) (map[string]any, error) {
	deadline := time.Time{}
	if maxWait > 0 {
		deadline = time.Now().Add(maxWait)
	}
	for {
		var out map[string]any
		if err := c.Call(ctx, filestation.APISearch, "list", makeValues("taskid", taskID, "offset", "0", "limit", "1000"), &out); err != nil {
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

func valueFromMap(m map[string]any, k string) string {
	if v, ok := m[k]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func mapSliceAny(v any) []map[string]any {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func ensureDeleteSafety(ctx context.Context, c *filestation.Client, paths []string, recursive bool) error {
	if recursive {
		return nil
	}
	j, err := filestation.EncodeJSON(paths)
	if err != nil {
		return err
	}
	var out map[string]any
	if err := c.Call(ctx, filestation.APIList, "getinfo", makeValues("path", j, "additional", `["type"]`), &out); err != nil {
		return err
	}
	for _, file := range mapSliceAny(out["files"]) {
		if isDir, ok := file["isdir"].(bool); ok && isDir {
			return apperr.New("validation_error", "directory deletion requires --recursive/-r", 1)
		}
	}
	return nil
}

func uploadOne(ctx context.Context, c *filestation.Client, localPath, remotePath string, parents, overwrite, skipExisting bool) (map[string]any, error) {
	params, err := uploadParams(ctx, c, remotePath, parents, overwrite, skipExisting)
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

func uploadRecursiveCP(ctx context.Context, c *filestation.Client, localDir, remotePath string, parents, overwrite, skipExisting bool) (map[string]any, error) {
	exists, isDir, err := remoteExists(ctx, c, remotePath)
	if err != nil {
		return nil, err
	}
	if exists && !isDir {
		return nil, apperr.New("validation_error", "remote destination exists and is not a directory", 1)
	}
	targetRoot := remotePath
	if exists && isDir {
		targetRoot = joinRemote(remotePath, filepath.Base(localDir))
	}
	if err := ensureRemoteDir(ctx, c, targetRoot); err != nil {
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
		remoteCurrent := joinRemote(targetRoot, filepath.ToSlash(rel))
		if d.IsDir() {
			return ensureRemoteDir(ctx, c, remoteCurrent)
		}
		parent := path.Dir(remoteCurrent)
		if skipExisting {
			ex, _, err := remoteExists(ctx, c, remoteCurrent)
			if err != nil {
				return err
			}
			if ex {
				skipped++
				return nil
			}
		}
		params, err := uploadParams(ctx, c, parent, parents, overwrite, skipExisting)
		if err != nil {
			return err
		}
		if _, err := c.Upload(ctx, params, p); err != nil {
			if renameErr := renameUploadedFile(ctx, c, parent, filepath.Base(p), path.Base(remoteCurrent)); renameErr != nil {
				return err
			}
		}
		if filepath.Base(p) != path.Base(remoteCurrent) {
			if err := renameUploadedFile(ctx, c, parent, filepath.Base(p), path.Base(remoteCurrent)); err != nil {
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

func renameUploadedFile(ctx context.Context, c *filestation.Client, parent, oldName, newName string) error {
	if oldName == newName {
		return nil
	}
	p := joinRemote(parent, oldName)
	pj, err := filestation.EncodeJSON([]string{p})
	if err != nil {
		return err
	}
	nj, err := filestation.EncodeJSON([]string{newName})
	if err != nil {
		return err
	}
	return c.Call(ctx, filestation.APIRename, "rename", makeValues("path", pj, "name", nj), nil)
}

func uploadParams(ctx context.Context, c *filestation.Client, remoteDir string, parents, overwrite, skipExisting bool) (map[string]string, error) {
	api := c.API(filestation.APIUpload)
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
	if skipExisting {
		// Version 2 API has only bool overwrite; best effort skip is handled by caller for recursive mode.
		_ = ctx
	}
	return params, nil
}

func remoteExists(ctx context.Context, c *filestation.Client, remotePath string) (bool, bool, error) {
	j, err := filestation.EncodeJSON([]string{remotePath})
	if err != nil {
		return false, false, err
	}
	var out map[string]any
	err = c.Call(ctx, filestation.APIList, "getinfo", makeValues("path", j), &out)
	if err != nil {
		var apiErr *filestation.APIError
		if errors.As(err, &apiErr) && (apiErr.Code == 408 || apiErr.SubCode == 408) {
			return false, false, nil
		}
		return false, false, err
	}
	files := mapSliceAny(out["files"])
	if len(files) == 0 {
		return false, false, nil
	}
	if code, ok := int64FromAny(files[0]["code"]); ok && code == 408 {
		return false, false, nil
	}
	isDir := false
	if v, ok := files[0]["isdir"].(bool); ok {
		isDir = v
	} else if v, ok := files[0]["isdir"].(string); ok {
		isDir = v == "true" || v == "1"
	}
	return true, isDir, nil
}

func ensureRemoteDir(ctx context.Context, c *filestation.Client, dir string) error {
	if dir == "" || dir == "/" {
		return nil
	}
	exists, isDir, err := remoteExists(ctx, c, dir)
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
	nameJSON, err := filestation.EncodeJSON([]string{name})
	if err != nil {
		return err
	}
	return c.Call(ctx, filestation.APICreateFolder, "create", makeValues("folder_path", parent, "name", nameJSON, "force_parent", "true"), nil)
}

func joinRemote(base, elem string) string {
	base = strings.TrimSuffix(base, "/")
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	return path.Clean(base + "/" + strings.TrimPrefix(filepath.ToSlash(elem), "/"))
}

func capitalizeWord(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func printBackgroundTasks(w io.Writer, out map[string]any) {
	tasks := mapSliceAny(out["tasks"])
	rows := make([][]string, 0, len(tasks))
	for _, t := range tasks {
		rows = append(rows, []string{valueFromMap(t, "taskid"), valueFromMap(t, "api"), valueFromMap(t, "status"), valueFromMap(t, "progress")})
	}
	printKVBlock(w, "Background Tasks", []kvField{{Label: "Count", Value: fmt.Sprintf("%d", len(tasks))}})
	printTable(w, []string{"Task ID", "API", "Status", "Progress"}, rows)
}

func fsListSizeDisplay(file map[string]any) string {
	if fsFileIsDir(file) {
		return "<DIR>"
	}
	size := fsFileSize(file)
	if size <= 0 {
		return "0 B"
	}
	return formatBytes(size)
}

func fsListMTimeDisplay(file map[string]any) string {
	ts := fsFileMTime(file)
	if ts <= 0 {
		return "-"
	}
	t := time.Unix(ts, 0).Local()
	now := time.Now()
	if t.After(now.AddDate(0, -6, 0)) && t.Before(now.AddDate(0, 6, 0)) {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

func fsFileIsDir(file map[string]any) bool {
	if v, ok := file["isdir"].(bool); ok {
		return v
	}
	if v, ok := file["isdir"].(string); ok {
		return strings.EqualFold(v, "true")
	}
	return false
}

func fsFileSize(file map[string]any) int64 {
	if n, ok := int64FromAny(file["size"]); ok {
		return n
	}
	if additional, ok := file["additional"].(map[string]any); ok {
		if n, ok := int64FromAny(additional["size"]); ok {
			return n
		}
	}
	return 0
}

func fsFileMTime(file map[string]any) int64 {
	if n, ok := int64FromAny(file["mtime"]); ok {
		return n
	}
	if additional, ok := file["additional"].(map[string]any); ok {
		if tm, ok := additional["time"].(map[string]any); ok {
			if n, ok := int64FromAny(tm["mtime"]); ok {
				return n
			}
		}
	}
	if tm, ok := file["time"].(map[string]any); ok {
		if n, ok := int64FromAny(tm["mtime"]); ok {
			return n
		}
	}
	return 0
}

func int64FromAny(v any) (int64, bool) {
	switch t := v.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	case float64:
		return int64(t), true
	case json.Number:
		n, err := t.Int64()
		if err == nil {
			return n, true
		}
	case string:
		var n int64
		_, err := fmt.Sscanf(strings.TrimSpace(t), "%d", &n)
		if err == nil {
			return n, true
		}
	}
	return 0, false
}
