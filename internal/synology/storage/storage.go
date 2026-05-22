package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

type APIError struct {
	Code int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("storage api error code=%d (%s)", e.Code, ErrorMessage(e.Code))
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

// LoadInfo mirrors the (very large) response of SYNO.Storage.CGI.Storage
// `load_info`. We only model the fields synocli actually surfaces; DSM
// returns many more that are ignored on purpose.
//
// Byte sizes come back as JSON strings (e.g. "11983029272576"). The Size
// type below normalizes them to int64.
type LoadInfo struct {
	Volumes      []Volume `json:"volumes"`
	StoragePools []Pool   `json:"storagePools"`
	Disks        []Disk   `json:"disks"`
}

type Volume struct {
	ID       string `json:"id"`
	VolPath  string `json:"vol_path"`
	Status   string `json:"status"`
	FSType   string `json:"fs_type"`
	RAIDType string `json:"raidType"`
	Size     Size   `json:"size"`
}

type Pool struct {
	ID       string   `json:"id"`
	Status   string   `json:"status"`
	RAIDType string   `json:"raidType"`
	Disks    []string `json:"disks"`
	Size     Size     `json:"size"`
}

type Disk struct {
	ID          string  `json:"id"`           // e.g. "sda"
	Name        string  `json:"name"`         // e.g. "Drive 1"
	Model       string  `json:"model"`
	Status      string  `json:"status"`
	SmartStatus string  `json:"smart_status"`
	SizeTotal   intStr  `json:"size_total"`   // bytes; DSM returns string
	Temp        int     `json:"temp"`         // degrees Celsius
	Vendor      string  `json:"vendor"`
}

// Size is the {total, used} pair DSM emits for volumes and pools.
type Size struct {
	Total intStr `json:"total"`
	Used  intStr `json:"used"`
}

// intStr accepts either a JSON string or number containing an integer.
// DSM is inconsistent across endpoints and even across fields in a single
// response.
type intStr int64

func (i *intStr) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*i = 0
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		if s == "" {
			*i = 0
			return nil
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		*i = intStr(n)
		return nil
	}
	var n int64
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*i = intStr(n)
	return nil
}

func (i intStr) Int64() int64 { return int64(i) }

func (c *Client) LoadInfo(ctx context.Context) (*LoadInfo, error) {
	var out LoadInfo
	if err := c.callJSON(ctx, "load_info", &out); err != nil {
		return nil, err
	}
	return &out, nil
}
