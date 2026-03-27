package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
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
	)
	return cmd
}

func newFSInfoCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Get File Station info",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("fs", "info"), func(ctx context.Context, s *session) (any, error) {
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APIInfo, "get", nil, &out); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				printKVBlock(ac.out, "File Station", []kvField{
					{Label: "Hostname", Value: filestation.ValueFromMap(out, "hostname")},
					{Label: "Is Manager", Value: filestation.ValueFromMap(out, "is_manager")},
				})
				return nil, nil
			})
		},
	}
}

func newFSSharesCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "shares",
		Short: "List shared folders",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("fs", "shares"), func(ctx context.Context, s *session) (any, error) {
				params := makeValues("offset", "0", "limit", "0")
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APIList, "list_share", params, &out); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				shares := filestation.MapSliceAny(out["shares"])
				rows := make([][]string, 0, len(shares))
				for _, sh := range shares {
					rows = append(rows, []string{filestation.ValueFromMap(sh, "name"), filestation.ValueFromMap(sh, "path")})
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
	var watch bool
	var interval time.Duration
	cmd := &cobra.Command{
		Use:     "list <folder-path>",
		Aliases: []string{"ls"},
		Short:   "List files in folder",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if watch {
				if err := validatePositiveDuration("--interval", interval); err != nil {
					return err
				}
			}
			return ac.withSession(cmd, joinCommand("fs", "list"), func(ctx context.Context, s *session) (any, error) {
				buildParams := func() (mapValues, error) {
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
					return params, nil
				}
				if watch {
					ui := newHumanUI(ac.out)
					return nil, pollLoop(ctx, interval, func() error {
						params, err := buildParams()
						if err != nil {
							return err
						}
						var out map[string]any
						if err := s.fsClient.Call(ctx, filestation.APIList, "list", params, &out); err != nil {
							return err
						}
						if ac.opts.JSON {
							env := output.NewEnvelope(true, joinCommand("fs", "list"), s.endpoint, s.start)
							env.Meta.APIVersion = s.apiVersions
							env.Data = map[string]any{"event": "snapshot", "mode": "folder", "path": args[0], "snapshot": out}
							return output.WriteJSONLine(ac.out, env)
						}
						if ui.tty {
							_, _ = fmt.Fprint(ac.out, ansiClearScreen)
						}
						files := filestation.MapSliceAny(out["files"])
						rows := make([][]string, 0, len(files))
						for _, f := range files {
							rows = append(rows, []string{
								filestation.ValueFromMap(f, "name"),
								filestation.ValueFromMap(f, "path"),
								fsListSizeDisplay(f),
								fsListMTimeDisplay(f),
							})
						}
						printKVBlock(ac.out, "Folder", []kvField{{Label: "Timestamp", Value: time.Now().Format(time.RFC3339)}, {Label: "Path", Value: args[0]}, {Label: "Entries", Value: fmt.Sprintf("%d", len(files))}})
						printTable(ac.out, []string{"Name", "Path", "Size", "MTime"}, rows)
						return nil
					})
				}
				params, err := buildParams()
				if err != nil {
					return nil, err
				}
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APIList, "list", params, &out); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				files := filestation.MapSliceAny(out["files"])
				rows := make([][]string, 0, len(files))
				for _, f := range files {
					rows = append(rows, []string{
						filestation.ValueFromMap(f, "name"),
						filestation.ValueFromMap(f, "path"),
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
	cmd.Flags().StringVar(&filetype, "file-type", "", "file/dir/all")
	cmd.Flags().BoolVar(&recursive, "recursive", false, "Recursive listing")
	cmd.Flags().StringSliceVar(&additional, "additional", []string{"real_path", "size", "time", "type"}, "Additional fields")
	cmd.Flags().BoolVar(&watch, "watch", false, "Continuous polling mode")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	return cmd
}

func newFSGetCmd(ac *appContext) *cobra.Command {
	var additional []string
	cmd := &cobra.Command{
		Use:   "get <path> [<path>...]",
		Short: "Get file info",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("fs", "get"), func(ctx context.Context, s *session) (any, error) {
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
				files := filestation.MapSliceAny(out["files"])
				rows := make([][]string, 0, len(files))
				for _, f := range files {
					size := fsListSizeDisplay(f)
					rows = append(rows, []string{filestation.ValueFromMap(f, "path"), filestation.ValueFromMap(f, "name"), filestation.ValueFromMap(f, "isdir"), size})
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
			return ac.withSession(cmd, joinCommand("fs", "mkdir"), func(ctx context.Context, s *session) (any, error) {
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
			return ac.withSession(cmd, joinCommand("fs", "rename"), func(ctx context.Context, s *session) (any, error) {
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
			return ac.withSession(cmd, joinCommand("fs", verb), func(ctx context.Context, s *session) (any, error) {
				if err := s.fsClient.EnsureDir(ctx, dest); err != nil {
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
				taskID := filestation.FirstTaskID(out)
				if taskID == "" {
					return nil, apperr.New("internal_error", "copy/move task id missing", 1)
				}
				if !async {
					status, err := s.fsClient.WaitTask(ctx, filestation.APICopyMove, taskID, interval, maxWait)
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
	cmd := &cobra.Command{
		Use:   "delete <path> [<path>...]",
		Short: "Delete files/folders",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("fs", "delete"), func(ctx context.Context, s *session) (any, error) {
				if err := s.fsClient.EnsureDeleteSafety(ctx, args, recursive); err != nil {
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
				taskID := filestation.FirstTaskID(out)
				if taskID == "" {
					return nil, apperr.New("internal_error", "delete task id missing", 1)
				}
				out["task_id"] = taskID
				if ac.opts.JSON {
					return out, nil
				}
				printKVBlock(ac.out, "Delete", []kvField{{Label: "Task ID", Value: taskID}})
				return nil, nil
			})
		},
	}
	cmd.Flags().BoolVar(&recursive, "recursive", false, "Delete directories recursively")
	cmd.Flags().BoolVar(&async, "async", false, "Run async delete task")
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
			return ac.withSession(cmd, joinCommand("fs", "upload"), func(ctx context.Context, s *session) (any, error) {
				st, err := os.Stat(args[0])
				if err != nil {
					return nil, fmt.Errorf("stat local path: %w", err)
				}
				if st.IsDir() {
					res, err := s.fsClient.UploadRecursiveCP(ctx, args[0], args[1], parents, overwrite, skipExisting)
					if err != nil {
						return nil, err
					}
					if ac.opts.JSON {
						return res, nil
					}
					printKVBlock(ac.out, "Upload Directory", []kvField{{Label: "Local", Value: args[0]}, {Label: "Remote", Value: args[1]}, {Label: "Files", Value: fmt.Sprintf("%v", res["uploaded_files"])}})
					return nil, nil
				}
				res, err := s.fsClient.UploadOne(ctx, args[0], args[1], parents, overwrite, skipExisting)
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
			return ac.withSession(cmd, joinCommand("fs", "download"), func(ctx context.Context, s *session) (any, error) {
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
			return ac.withSession(cmd, joinCommand("fs", "search"), func(ctx context.Context, s *session) (any, error) {
				params := makeValues("folder_path", args[0], "pattern", pattern, "recursive", fmt.Sprintf("%t", recursive))
				if filetype != "" {
					params.Set("filetype", filetype)
				}
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APISearch, "start", params, &out); err != nil {
					return nil, err
				}
				taskID := filestation.FirstTaskID(out)
				if taskID == "" {
					return nil, apperr.New("internal_error", "search task id missing", 1)
				}
				res := map[string]any{"task_id": taskID}
				if !async {
					snapshot, err := s.fsClient.WaitSearch(ctx, taskID, interval, maxWait)
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
	cmd.Flags().StringVar(&filetype, "file-type", "", "file/dir/all")
	cmd.Flags().BoolVar(&async, "async", false, "Do not wait for completion")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	cmd.Flags().DurationVar(&maxWait, "max-wait", 0, "Maximum wait duration (0 means unlimited)")
	return cmd
}

func newFSSearchResultsCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "search-results <task-id>",
		Short: "Get search results",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("fs", "search-results"), func(ctx context.Context, s *session) (any, error) {
				params := makeValues("taskid", args[0], "offset", "0", "limit", "0")
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APISearch, "list", params, &out); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				files := filestation.MapSliceAny(out["files"])
				rows := make([][]string, 0, len(files))
				for _, f := range files {
					rows = append(rows, []string{filestation.ValueFromMap(f, "path"), filestation.ValueFromMap(f, "name")})
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
			return ac.withSession(cmd, joinCommand("fs", "search-stop"), func(ctx context.Context, s *session) (any, error) {
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
		Use:   "search-clear <task-id> [<task-id>...]",
		Short: "Clear search tasks",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("fs", "search-clear"), func(ctx context.Context, s *session) (any, error) {
				for _, taskID := range args {
					if err := s.fsClient.Call(ctx, filestation.APISearch, "clean", makeValues("taskid", taskID), nil); err != nil {
						return nil, err
					}
				}
				if ac.opts.JSON {
					return map[string]any{"cleared": true, "task_ids": args}, nil
				}
				printKVBlock(ac.out, "Search", []kvField{
					{Label: "Cleared", Value: "true"},
					{Label: "Task IDs", Value: strings.Join(args, ", ")},
				})
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
			return ac.withSession(cmd, joinCommand("fs", cmdName), func(ctx context.Context, s *session) (any, error) {
				pairs, err := paramsFn(args)
				if err != nil {
					return nil, err
				}
				params := makeValuesFromMap(pairs)
				var out map[string]any
				if err := s.fsClient.Call(ctx, apiKey, method, params, &out); err != nil {
					return nil, err
				}
				taskID := filestation.FirstTaskID(out)
				if taskID == "" {
					return nil, apperr.New("internal_error", "task id missing", 1)
				}
				if !async {
					status, err := s.fsClient.WaitTask(ctx, apiKey, taskID, interval, maxWait)
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
			return ac.withSession(cmd, joinCommand("fs", cmdName), func(ctx context.Context, s *session) (any, error) {
				var out map[string]any
				if err := s.fsClient.Call(ctx, apiKey, "status", makeValues("taskid", args[0]), &out); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				printKVBlock(ac.out, title, []kvField{{Label: "Task ID", Value: args[0]}, {Label: "Finished", Value: filestation.ValueFromMap(out, "finished")}})
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
			return ac.withSession(cmd, joinCommand("fs", cmdName), func(ctx context.Context, s *session) (any, error) {
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
	return newTaskStartCmd(ac, filestation.APIDirSize, "dir-size", "Calculate directory size", "start", func(args []string) (map[string]string, error) {
		j, err := filestation.EncodeJSON(args)
		if err != nil {
			return nil, err
		}
		return map[string]string{"path": j}, nil
	})
}

func newFSDirSizeStatusCmd(ac *appContext) *cobra.Command {
	return newTaskStatusCmd(ac, filestation.APIDirSize, "dir-size-status", "Check dir size status")
}

func newFSDirSizeStopCmd(ac *appContext) *cobra.Command {
	return newTaskStopCmd(ac, filestation.APIDirSize, "dir-size-stop", "Stop dir size calculation")
}

func newFSMD5Cmd(ac *appContext) *cobra.Command {
	return newTaskStartCmd(ac, filestation.APIMD5, "md5", "Calculate MD5 checksum", "start", func(args []string) (map[string]string, error) {
		return map[string]string{"file_path": args[0]}, nil
	})
}

func newFSMD5StatusCmd(ac *appContext) *cobra.Command {
	return newTaskStatusCmd(ac, filestation.APIMD5, "md5-status", "Check MD5 status")
}

func newFSMD5StopCmd(ac *appContext) *cobra.Command {
	return newTaskStopCmd(ac, filestation.APIMD5, "md5-stop", "Stop MD5 calculation")
}

func newFSExtractCmd(ac *appContext) *cobra.Command {
	var dest string
	var overwrite bool
	var keepDir bool
	var createSubfolder bool
	var password string
	cmd := newTaskStartCmd(ac, filestation.APIExtract, "extract", "Extract archive", "start", func(args []string) (map[string]string, error) {
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
	return newTaskStatusCmd(ac, filestation.APIExtract, "extract-status", "Check extract status")
}

func newFSExtractStopCmd(ac *appContext) *cobra.Command {
	return newTaskStopCmd(ac, filestation.APIExtract, "extract-stop", "Stop extract task")
}

func newFSCompressCmd(ac *appContext) *cobra.Command {
	var dest string
	var format string
	var level int
	var mode string
	var password string
	cmd := newTaskStartCmd(ac, filestation.APICompress, "compress", "Compress files", "start", func(args []string) (map[string]string, error) {
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
	return newTaskStatusCmd(ac, filestation.APICompress, "compress-status", "Check compress status")
}

func newFSCompressStopCmd(ac *appContext) *cobra.Command {
	return newTaskStopCmd(ac, filestation.APICompress, "compress-stop", "Stop compress task")
}

func newFSTasksCmd(ac *appContext) *cobra.Command {
	var offset, limit int
	var sortBy, sortDirection string
	var watch bool
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "List File Station background tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if watch {
				if err := validatePositiveDuration("--interval", interval); err != nil {
					return err
				}
			}
			return ac.withSession(cmd, joinCommand("fs", "tasks"), func(ctx context.Context, s *session) (any, error) {
				fetch := func() (map[string]any, error) {
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
					return out, nil
				}
				if watch {
					ui := newHumanUI(ac.out)
					return nil, pollLoop(ctx, interval, func() error {
						out, err := fetch()
						if err != nil {
							return err
						}
						if ac.opts.JSON {
							env := output.NewEnvelope(true, joinCommand("fs", "tasks"), s.endpoint, s.start)
							env.Meta.APIVersion = s.apiVersions
							env.Data = map[string]any{"event": "snapshot", "mode": "tasks", "snapshot": out}
							return output.WriteJSONLine(ac.out, env)
						}
						if ui.tty {
							_, _ = fmt.Fprint(ac.out, ansiClearScreen)
						}
						printKVBlock(ac.out, "Background Tasks", []kvField{{Label: "Timestamp", Value: time.Now().Format(time.RFC3339)}})
						printBackgroundTasks(ac.out, out)
						return nil
					})
				}
				out, err := fetch()
				if err != nil {
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
	cmd.Flags().BoolVar(&watch, "watch", false, "Continuous polling mode")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	return cmd
}

func newFSTasksClearCmd(ac *appContext) *cobra.Command {
	var ids []string
	cmd := &cobra.Command{
		Use:   "tasks-clear",
		Short: "Clear finished background tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("fs", "tasks-clear"), func(ctx context.Context, s *session) (any, error) {
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

func capitalizeWord(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func printBackgroundTasks(w io.Writer, out map[string]any) {
	tasks := filestation.MapSliceAny(out["tasks"])
	rows := make([][]string, 0, len(tasks))
	for _, t := range tasks {
		rows = append(rows, []string{filestation.ValueFromMap(t, "taskid"), filestation.ValueFromMap(t, "api"), filestation.ValueFromMap(t, "status"), filestation.ValueFromMap(t, "progress")})
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
	if n, ok := filestation.Int64FromAny(file["size"]); ok {
		return n
	}
	if additional, ok := file["additional"].(map[string]any); ok {
		if n, ok := filestation.Int64FromAny(additional["size"]); ok {
			return n
		}
	}
	return 0
}

func fsFileMTime(file map[string]any) int64 {
	if n, ok := filestation.Int64FromAny(file["mtime"]); ok {
		return n
	}
	if additional, ok := file["additional"].(map[string]any); ok {
		if tm, ok := additional["time"].(map[string]any); ok {
			if n, ok := filestation.Int64FromAny(tm["mtime"]); ok {
				return n
			}
		}
	}
	if tm, ok := file["time"].(map[string]any); ok {
		if n, ok := filestation.Int64FromAny(tm["mtime"]); ok {
			return n
		}
	}
	return 0
}
