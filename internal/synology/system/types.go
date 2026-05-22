package system

import (
	"encoding/json"
	"fmt"
)

type APIError struct {
	Code int
	API  string
}

func (e *APIError) Error() string {
	if e.API != "" {
		return fmt.Sprintf("system api error code=%d (%s) api=%s", e.Code, ErrorMessage(e.Code), e.API)
	}
	return fmt.Sprintf("system api error code=%d (%s)", e.Code, ErrorMessage(e.Code))
}

type baseResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code int `json:"code"`
	} `json:"error,omitempty"`
}

var errorMessages = map[int]string{
	100: "unknown error",
	101: "invalid parameter",
	102: "api does not exist",
	103: "method does not exist",
	104: "api version is not supported",
	105: "insufficient user privilege",
	106: "session timeout",
	107: "session interrupted by duplicate login",
	119: "SID not found",
}

func ErrorMessage(code int) string {
	if v, ok := errorMessages[code]; ok {
		return v
	}
	return "unmapped"
}
