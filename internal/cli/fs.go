package cli

import (
	"net/url"
	"path"
	"strings"

	"github.com/spf13/cobra"
)

func newFSCmd(ac *appContext) *cobra.Command {
	cmd := &cobra.Command{Use: "fs", Aliases: []string{"filestation"}, Short: "File Station commands"}
	cmd.AddGroup(
		&cobra.Group{ID: "files", Title: "File Operations:"},
		&cobra.Group{ID: "archive", Title: "Archive:"},
		&cobra.Group{ID: "util", Title: "Utilities:"},
		&cobra.Group{ID: "tasks", Title: "Background Tasks:"},
	)
	withGroup := func(g string, cmds ...*cobra.Command) []*cobra.Command {
		for _, c := range cmds {
			c.GroupID = g
		}
		return cmds
	}
	cmd.AddCommand(withGroup("files",
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
	)...)
	cmd.AddCommand(withGroup("archive",
		newFSCompressCmd(ac),
		newFSExtractCmd(ac),
	)...)
	cmd.AddCommand(withGroup("util",
		newFSDirSizeCmd(ac),
		newFSMD5Cmd(ac),
		newFSSearchCmd(ac),
	)...)
	cmd.AddCommand(withGroup("tasks",
		newFSTasksCmd(ac),
		newFSTasksClearCmd(ac),
	)...)
	return cmd
}

type mapValues = url.Values

func makeValues(kv ...string) mapValues {
	vals := mapValues{}
	for i := 0; i+1 < len(kv); i += 2 {
		vals.Set(kv[i], kv[i+1])
	}
	return vals
}

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

// cleanFolderPath normalizes a remote folder path by stripping trailing slashes
// and collapsing redundant separators. The Synology API rejects paths like
// "/foo/bar/" (error 418), so we normalize before sending.
func cleanFolderPath(p string) string {
	return path.Clean(p)
}

func capitalizeWord(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
