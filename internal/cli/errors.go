package cli

import (
	"context"
	"errors"
	"time"

	"synocli/internal/apperr"
	"synocli/internal/output"
	"synocli/internal/synology/downloadstation"
	"synocli/internal/synology/filestation"
)

func toAppError(err error) error {
	var dsErr *downloadstation.APIError
	if errors.As(err, &dsErr) {
		code := "synology_error"
		exit := 1
		if dsErr.Code == 404 {
			exit = 3
		}
		details := map[string]any{
			"synology_code": dsErr.Code,
		}
		if len(dsErr.FailedTasks) > 0 {
			failed := make([]map[string]any, 0, len(dsErr.FailedTasks))
			ids := make([]string, 0, len(dsErr.FailedTasks))
			for _, ft := range dsErr.FailedTasks {
				failed = append(failed, map[string]any{
					"id":   ft.ID,
					"code": ft.Code,
				})
				if ft.ID != "" {
					ids = append(ids, ft.ID)
				}
			}
			details["failed_tasks"] = failed
			if len(ids) > 0 {
				details["failed_task_ids"] = ids
			}
		}
		return &apperr.Error{
			Code:     code,
			Message:  downloadstation.ErrorMessage(dsErr.Code),
			ExitCode: exit,
			Details:  details,
			Err:      err,
		}
	}
	var fsErr *filestation.APIError
	if errors.As(err, &fsErr) {
		code := fsErr.EffectiveCode()
		details := map[string]any{
			"synology_code": code,
		}
		if fsErr.Path != "" {
			details["path"] = fsErr.Path
		}
		if fsErr.Code != 0 && fsErr.Code != code {
			details["synology_parent_code"] = fsErr.Code
		}
		return &apperr.Error{
			Code:     "synology_error",
			Message:  filestation.ErrorMessage(code),
			ExitCode: 1,
			Details:  details,
		}
	}
	var app *apperr.Error
	if errors.As(err, &app) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return apperr.Wrap("timeout", "command timed out", 5, err)
	}
	return apperr.Wrap("internal_error", "command failed", 1, err)
}

func (a *appContext) outputError(commandName, endpoint string, start time.Time, err error) error {
	if !a.opts.JSON {
		return err
	}
	env := output.NewEnvelope(false, commandName, endpoint, start)
	env.Error = &output.ErrInfo{
		Code:    apperr.Code(err),
		Message: err.Error(),
		Details: apperr.Details(err),
	}
	if writeErr := output.WriteJSON(a.out, env); writeErr != nil {
		return err
	}
	return &jsonOutputHandledError{err: err}
}
