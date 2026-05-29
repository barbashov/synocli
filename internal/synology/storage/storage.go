package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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
	ID                 string        `json:"id"`     // e.g. "sda"
	Name               string        `json:"name"`   // e.g. "Drive 1"
	Device             string        `json:"device"` // e.g. "/dev/sda"
	Model              string        `json:"model"`
	Status             string        `json:"status"`
	SmartStatus        string        `json:"smart_status"`
	SizeTotal          intStr        `json:"size_total"` // bytes; DSM returns string
	Temp               int           `json:"temp"`       // degrees Celsius
	Vendor             string        `json:"vendor"`
	DiskType           string        `json:"diskType"` // "SATA" | "USB" | ...
	IsSSD              bool          `json:"isSsd"`
	SlotID             int           `json:"slot_id"` // bay number
	Serial             string        `json:"serial"`
	Firm               string        `json:"firm"`    // firmware revision
	UsedBy             string        `json:"used_by"` // pool id, or ""
	ExceedBadSectorThr bool          `json:"exceed_bad_sector_thr"`
	BelowRemainLifeThr bool          `json:"below_remain_life_thr"`
	RemainLife         int           `json:"remain_life"` // SSD life %, -1 for HDD
	Container          DiskContainer `json:"container"`
}

// DiskContainer describes which enclosure houses the drive: the main NAS unit
// or one of its expansion shelves.
type DiskContainer struct {
	Order int    `json:"order"`
	Str   string `json:"str"`  // e.g. "DS1511+"
	Type  string `json:"type"` // "internal" | "ebox"
}

// HealthInfo is the parsed response of SYNO.Storage.CGI.Smart get_health_info
// for a single disk. The smartInfo array carries the canonical SMART
// attribute table; overview is DSM's curated health verdict.
type HealthInfo struct {
	Count     int              `json:"count"`
	Overview  HealthOverview   `json:"overview"`
	SmartInfo []SmartAttribute `json:"smartInfo"`
}

type HealthOverview struct {
	Smart              string               `json:"smart"`      // "normal" | ...
	SmartTest          string               `json:"smart_test"` // "normal" | ...
	OverviewStatus     string               `json:"overview_status"`
	Poweron            intStr               `json:"poweron"` // hours; DSM returns string
	IDNF               int                  `json:"idnf"`
	Retry              int                  `json:"retry"`
	UNC                int                  `json:"unc"`
	RemainLife         int                  `json:"remain_life"`
	ExceedBadSectorThr bool                 `json:"exceed_bad_sector_thr"`
	BelowRemainLifeThr bool                 `json:"below_remain_life_thr"`
	IsSSD              bool                 `json:"isSsd"`
	IsNVMe             bool                 `json:"isNVMeDisk"`
	ReadOnly           bool                 `json:"read_only"`
	SmartScheduleList  []SmartScheduleEntry `json:"smart_schedule_list"`
}

type SmartScheduleEntry struct {
	NextTriggerTime string `json:"next_trigger_time"`
}

// SmartAttribute is one row of the standard SMART attribute table. DSM
// emits every field as a string.
type SmartAttribute struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Current   string `json:"current"`
	Worst     string `json:"worst"`
	Threshold string `json:"threshold"`
	Raw       string `json:"raw"`
	Status    string `json:"status"` // "OK" | "Bad"
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

// GetHealthInfo fetches the per-disk SMART health summary and attribute table
// from SYNO.Storage.CGI.Smart. `device` is the device path DSM expects
// (e.g. "/dev/sda") and `diskID` is the short disk id (e.g. "sda"); DSM
// requires both query parameters to be set.
func (c *Client) GetHealthInfo(ctx context.Context, device, diskID string) (*HealthInfo, error) {
	if device == "" {
		return nil, fmt.Errorf("device is required")
	}
	if diskID == "" {
		return nil, fmt.Errorf("diskID is required")
	}
	extra := url.Values{
		"device": []string{device},
		"disk":   []string{diskID},
	}
	var raw struct {
		HealthInfo HealthInfo `json:"healthInfo"`
	}
	if err := c.callAPIJSON(ctx, smartAPIName, smartVersion, "get_health_info", extra, &raw); err != nil {
		return nil, err
	}
	hi := raw.HealthInfo
	return &hi, nil
}
