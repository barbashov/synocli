package downloadstation

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type APIError struct {
	Code        int
	Name        string
	Reason      string
	FailedTasks []FailedTask
}

func (e *APIError) Error() string {
	if len(e.FailedTasks) > 0 {
		parts := make([]string, 0, len(e.FailedTasks))
		for _, ft := range e.FailedTasks {
			parts = append(parts, fmt.Sprintf("%s:%d", ft.ID, ft.Code))
		}
		return fmt.Sprintf("download station api error code=%d (%s): failed_task=%s", e.Code, ErrorMessage(e.Code), strings.Join(parts, ","))
	}
	if e.Name != "" {
		return fmt.Sprintf("download station api error code=%d (%s): %s %s", e.Code, ErrorMessage(e.Code), e.Name, e.Reason)
	}
	return fmt.Sprintf("download station api error code=%d (%s)", e.Code, ErrorMessage(e.Code))
}

type Task struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Type        string          `json:"type"`
	Username    string          `json:"username,omitempty"`
	Size        int64           `json:"size,omitempty"`
	Status      string          `json:"status"`
	StatusExtra string          `json:"status_extra,omitempty"`
	Additional  *TaskAdditional `json:"additional,omitempty"`
}

func (t *Task) UnmarshalJSON(data []byte) error {
	type taskAlias struct {
		ID          string          `json:"id"`
		Title       string          `json:"title"`
		Type        string          `json:"type"`
		Username    string          `json:"username,omitempty"`
		Size        int64           `json:"size,omitempty"`
		Status      any             `json:"status"`
		StatusExtra string          `json:"status_extra,omitempty"`
		Additional  *TaskAdditional `json:"additional,omitempty"`
	}
	var aux taskAlias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	t.ID = aux.ID
	t.Title = aux.Title
	t.Type = aux.Type
	t.Username = aux.Username
	t.Size = aux.Size
	t.StatusExtra = aux.StatusExtra
	t.Additional = aux.Additional
	switch v := aux.Status.(type) {
	case string:
		t.Status = v
	case float64:
		t.Status = strconv.FormatInt(int64(v), 10)
	case nil:
		t.Status = ""
	default:
		t.Status = fmt.Sprintf("%v", v)
	}
	return nil
}

type TaskAdditional struct {
	Detail   *TaskDetail   `json:"detail,omitempty"`
	Transfer *TaskTransfer `json:"transfer,omitempty"`
	Tracker  any           `json:"tracker,omitempty"`
	Peer     any           `json:"peer,omitempty"`
	File     any           `json:"file,omitempty"`
}

type TaskDetail struct {
	Destination   string `json:"destination,omitempty"`
	URI           string `json:"uri,omitempty"`
	CreateTime    int64  `json:"create_time,omitempty"`
	CompletedTime int64  `json:"completed_time,omitempty"`
	ErrorDetail   string `json:"error_detail,omitempty"`
}

type TaskTransfer struct {
	SizeDownloaded int64 `json:"size_downloaded,omitempty"`
	SizeUploaded   int64 `json:"size_uploaded,omitempty"`
	SpeedDownload  int64 `json:"speed_download,omitempty"`
	SpeedUpload    int64 `json:"speed_upload,omitempty"`
}

type baseResponse struct {
	Success bool `json:"success"`
	Error   *struct {
		Code   int           `json:"code"`
		Errors *ErrorDetails `json:"errors,omitempty"`
	} `json:"error,omitempty"`
}

type ErrorDetails struct {
	Name       string       `json:"name,omitempty"`
	Reason     string       `json:"reason,omitempty"`
	FailedTask []FailedTask `json:"failed_task,omitempty"`
}

type FailedTask struct {
	Code int    `json:"error"`
	ID   string `json:"id"`
}

type listResponse struct {
	baseResponse
	Data struct {
		Offset int    `json:"offset"`
		Total  int    `json:"total"`
		Tasks  []Task `json:"tasks"`
		Task   []Task `json:"task"`
	} `json:"data"`
}

type createResponse struct {
	baseResponse
	Data struct {
		TaskID any `json:"task_id"`
		ListID any `json:"list_id"`
	} `json:"data"`
}

var errorMessages = map[int]string{
	100: "unknown error",
	101: "invalid parameter",
	102: "api does not exist",
	103: "method does not exist",
	104: "this API version is not supported",
	105: "insufficient user privilege",
	106: "session timeout",
	107: "session interrupted by duplicate login",
	400: "file upload failed",
	401: "max number of tasks reached",
	402: "destination denied",
	403: "destination does not exist",
	404: "invalid task id",
	405: "invalid task action",
	406: "no default destination",
	407: "set destination failed",
	408: "file does not exist",
	120: "required parameter missing",
}

func ErrorMessage(code int) string {
	if v, ok := errorMessages[code]; ok {
		return v
	}
	return "unmapped"
}
