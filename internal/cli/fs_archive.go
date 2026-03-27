package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"synocli/internal/apperr"
	"synocli/internal/synology/filestation"
)

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
	cmd.AddCommand(
		newTaskStatusCmd(ac, filestation.APIExtract, "extract", "Check extract status"),
		newTaskStopCmd(ac, filestation.APIExtract, "extract", "Stop extract task"),
	)
	return cmd
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
	cmd.AddCommand(
		newTaskStatusCmd(ac, filestation.APICompress, "compress", "Check compress status"),
		newTaskStopCmd(ac, filestation.APICompress, "compress", "Stop compress task"),
	)
	return cmd
}
