package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"synocli/internal/apperr"
	"synocli/internal/cmdutil"
	"synocli/internal/synology/downloadstation"
)

func newDSFilesCmd(ac *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "files",
		Short: "List and choose which files in a multi-file BT task to download",
	}
	cmd.AddCommand(
		newDSFilesListCmd(ac),
		newDSFilesSetCmd(ac),
		newDSFilesPriorityCmd(ac),
	)
	return cmd
}

func newDSFilesListCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:     "list <task-id>",
		Aliases: []string{"ls"},
		Short:   "List files in a BT task with index, size, progress, and selection",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.TrimSpace(args[0])
			return ac.withSession(cmd, joinCommand("ds", "files", "list"), func(ctx context.Context, s *session) (any, error) {
				files, err := s.dsClient.ListBTFiles(ctx, id)
				if err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return map[string]any{"task_id": id, "files": downloadstation.MapBTFiles(files)}, nil
				}
				cmdutil.PrintKVBlock(ac.out, "Task Files", []cmdutil.KVField{
					{Label: "Task ID", Value: id},
					{Label: "Files", Value: strconv.Itoa(len(files))},
				})
				if len(files) == 0 {
					_, _ = fmt.Fprintln(ac.out)
					_, _ = fmt.Fprintln(ac.out, "No files — the task is not a multi-file BT task, or torrent metadata has not been fetched yet.")
					return nil, nil
				}
				printBTFileTable(ac.out, files)
				return nil, nil
			})
		},
	}
}

func newDSFilesSetCmd(ac *appContext) *cobra.Command {
	var include, skip []int
	var all, none bool
	cmd := &cobra.Command{
		Use:   "set <task-id>",
		Short: "Choose which files in a BT task to download or skip",
		Long: "Choose which files in a multi-file BT task to download. Indices come " +
			"from `ds files list <task-id>`.\n\n" +
			"Use --include / --skip with one or more file indices, or --all / --none " +
			"to (de)select every file. Indices are validated against the task's " +
			"current file list before any change is sent, because DSM rejects " +
			"out-of-range indices with an opaque error.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.TrimSpace(args[0])
			if err := validateWantedFlags(all, none, include, skip); err != nil {
				return err
			}
			return ac.withSession(cmd, joinCommand("ds", "files", "set"), func(ctx context.Context, s *session) (any, error) {
				files, err := s.dsClient.ListBTFiles(ctx, id)
				if err != nil {
					return nil, err
				}
				wantIdx, skipIdx, err := resolveWantedSelection(files, all, none, include, skip)
				if err != nil {
					return nil, err
				}
				if err := s.dsClient.SetBTFileWanted(ctx, id, wantIdx, true); err != nil {
					return nil, err
				}
				if err := s.dsClient.SetBTFileWanted(ctx, id, skipIdx, false); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return map[string]any{"task_id": id, "wanted": wantIdx, "skipped": skipIdx}, nil
				}
				cmdutil.PrintKVBlock(ac.out, "Task Files Updated", []cmdutil.KVField{
					{Label: "Task ID", Value: id},
					{Label: "Wanted", Value: joinInts(wantIdx)},
					{Label: "Skipped", Value: joinInts(skipIdx)},
				})
				return nil, nil
			})
		},
	}
	cmd.Flags().IntSliceVar(&include, "include", nil, "File indices to download (mark wanted)")
	cmd.Flags().IntSliceVar(&skip, "skip", nil, "File indices to skip (mark not wanted)")
	cmd.Flags().BoolVar(&all, "all", false, "Download all files")
	cmd.Flags().BoolVar(&none, "none", false, "Skip all files")
	return cmd
}

func newDSFilesPriorityCmd(ac *appContext) *cobra.Command {
	var index []int
	var all bool
	cmd := &cobra.Command{
		Use:   "priority <task-id> <low|normal|high>",
		Short: "Set download priority for files in a BT task",
		Long: "Set the download priority (low, normal, or high) for files in a " +
			"multi-file BT task. Priority is independent of whether a file is " +
			"wanted — use `ds files set` to skip files entirely.\n\n" +
			"Select files with --index (from `ds files list`) or --all.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.TrimSpace(args[0])
			priority := strings.ToLower(strings.TrimSpace(args[1]))
			if !isValidPriority(priority) {
				return apperr.New("validation_error", fmt.Sprintf("priority must be one of low, normal, high; got %q", args[1]), 1)
			}
			if !all && len(index) == 0 {
				return apperr.New("validation_error", "specify files with --index or --all", 1)
			}
			if all && len(index) > 0 {
				return apperr.New("validation_error", "--all cannot be combined with --index", 1)
			}
			return ac.withSession(cmd, joinCommand("ds", "files", "priority"), func(ctx context.Context, s *session) (any, error) {
				files, err := s.dsClient.ListBTFiles(ctx, id)
				if err != nil {
					return nil, err
				}
				indices, err := resolvePriorityIndices(files, all, index)
				if err != nil {
					return nil, err
				}
				if err := s.dsClient.SetBTFilePriority(ctx, id, indices, priority); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return map[string]any{"task_id": id, "priority": priority, "indices": indices}, nil
				}
				cmdutil.PrintKVBlock(ac.out, "Task File Priority Updated", []cmdutil.KVField{
					{Label: "Task ID", Value: id},
					{Label: "Priority", Value: priority},
					{Label: "Indices", Value: joinInts(indices)},
				})
				return nil, nil
			})
		},
	}
	cmd.Flags().IntSliceVar(&index, "index", nil, "File indices to set priority for")
	cmd.Flags().BoolVar(&all, "all", false, "Apply to all files")
	return cmd
}

// ---------------------------------------------------------------------------
// Pure helpers (unit-testable without a session)
// ---------------------------------------------------------------------------

func isValidPriority(p string) bool {
	switch p {
	case downloadstation.BTFilePriorityLow,
		downloadstation.BTFilePriorityNormal,
		downloadstation.BTFilePriorityHigh:
		return true
	}
	return false
}

// validateWantedFlags enforces a single, unambiguous selection intent for
// `ds files set` before any network call.
func validateWantedFlags(all, none bool, include, skip []int) error {
	if all && none {
		return apperr.New("validation_error", "--all and --none are mutually exclusive", 1)
	}
	if (all || none) && (len(include) > 0 || len(skip) > 0) {
		return apperr.New("validation_error", "--all/--none cannot be combined with --include/--skip", 1)
	}
	if !all && !none && len(include) == 0 && len(skip) == 0 {
		return apperr.New("validation_error", "specify which files to change with --include, --skip, --all, or --none", 1)
	}
	return nil
}

// resolveWantedSelection turns the validated flags into concrete index lists,
// validating each index against the task's current files (DSM rejects unknown
// indices with an opaque code, so we surface a clear error first).
func resolveWantedSelection(files []downloadstation.BTFile, all, none bool, include, skip []int) (want, skipOut []int64, err error) {
	valid := indexSet(files)
	if all {
		return allIndices(files), nil, nil
	}
	if none {
		return nil, allIndices(files), nil
	}
	wantIdx, err := normalizeIndices(include, valid)
	if err != nil {
		return nil, nil, err
	}
	skipIdx, err := normalizeIndices(skip, valid)
	if err != nil {
		return nil, nil, err
	}
	for _, i := range wantIdx {
		if containsInt64(skipIdx, i) {
			return nil, nil, apperr.New("validation_error", fmt.Sprintf("index %d is in both --include and --skip", i), 1)
		}
	}
	return wantIdx, skipIdx, nil
}

func resolvePriorityIndices(files []downloadstation.BTFile, all bool, index []int) ([]int64, error) {
	if all {
		return allIndices(files), nil
	}
	return normalizeIndices(index, indexSet(files))
}

// normalizeIndices dedupes, sorts, and validates the supplied indices against
// the set of indices that exist in the task.
func normalizeIndices(indices []int, valid map[int64]bool) ([]int64, error) {
	seen := make(map[int64]bool, len(indices))
	out := make([]int64, 0, len(indices))
	for _, i := range indices {
		idx := int64(i)
		if !valid[idx] {
			return nil, apperr.New("validation_error", fmt.Sprintf("file index %d does not exist in this task", i), 1)
		}
		if seen[idx] {
			continue
		}
		seen[idx] = true
		out = append(out, idx)
	}
	sort.Slice(out, func(a, b int) bool { return out[a] < out[b] })
	return out, nil
}

func indexSet(files []downloadstation.BTFile) map[int64]bool {
	m := make(map[int64]bool, len(files))
	for _, f := range files {
		m[f.Index] = true
	}
	return m
}

func allIndices(files []downloadstation.BTFile) []int64 {
	out := make([]int64, 0, len(files))
	for _, f := range files {
		out = append(out, f.Index)
	}
	return out
}

func containsInt64(s []int64, v int64) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func joinInts(values []int64) string {
	if len(values) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, strconv.FormatInt(v, 10))
	}
	return strings.Join(parts, ", ")
}

func printBTFileTable(w io.Writer, files []downloadstation.BTFile) {
	rows := make([][]string, 0, len(files))
	for _, f := range files {
		rows = append(rows, []string{
			strconv.FormatInt(f.Index, 10),
			f.Name,
			cmdutil.FormatBytes(f.Size),
			cmdutil.FormatBytes(f.SizeDownloaded),
			cmdutil.FormatPercent(f.SizeDownloaded, f.Size),
			boolYesNo(f.Wanted),
			valueOrDash(f.Priority),
		})
	}
	cmdutil.PrintTable(w, []string{"Index", "Name", "Size", "Downloaded", "Progress", "Wanted", "Priority"}, rows)
}

func boolYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
