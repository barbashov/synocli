package system

import (
	"context"
	"encoding/json"
	"strconv"
)

// DSMInfo is the response shape of SYNO.DSM.Info `getinfo` (v2). Verified
// against a live DS1511+ NAS on DSM 6.2.4.
type DSMInfo struct {
	Model           string `json:"model"`
	Serial          string `json:"serial"`
	RAM             int    `json:"ram"`            // megabytes
	VersionString   string `json:"version_string"` // e.g. "DSM 6.2.4-25556 Update 7"
	Version         string `json:"version"`        // build number, e.g. "25556"
	Uptime          int64  `json:"uptime"`         // seconds
	Temperature     int    `json:"temperature"`    // degrees Celsius
	TemperatureWarn bool   `json:"temperature_warn"`
	Time            string `json:"time"`
	Codepage        string `json:"codepage"`
}

// SystemInfo is the response shape of SYNO.Core.System `info` (v3).
// CPUCores is intentionally a flexInt because DSM returns it as either a
// JSON string or a number depending on the model.
type SystemInfo struct {
	Model         string  `json:"model"`
	Serial        string  `json:"serial"`
	RAMSize       int64   `json:"ram_size"`        // megabytes
	CPUClockSpeed int     `json:"cpu_clock_speed"` // MHz
	CPUCores      flexInt `json:"cpu_cores"`
	CPUFamily     string  `json:"cpu_family"`
	CPUSeries     string  `json:"cpu_series"`
	CPUVendor     string  `json:"cpu_vendor"`
	FirmwareVer   string  `json:"firmware_ver"`
	FirmwareDate  string  `json:"firmware_date"`
	SysTemp       int     `json:"sys_temp"`
	Time          string  `json:"time"` // e.g. "2026-05-21 23:56:52"
	TimeZone      string  `json:"time_zone"`
	TimeZoneDesc  string  `json:"time_zone_desc"`
	UpTime        string  `json:"up_time"` // formatted "HHH:MM:SS"
	EnabledNTP    bool    `json:"enabled_ntp"`
	NTPServer     string  `json:"ntp_server"`
}

// flexInt accepts either a JSON number or a JSON string containing an
// integer. Used for fields like cpu_cores that DSM reports inconsistently.
type flexInt int

func (f *flexInt) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		if s == "" {
			*f = 0
			return nil
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		*f = flexInt(n)
		return nil
	}
	var n int
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*f = flexInt(n)
	return nil
}

func (c *Client) GetDSMInfo(ctx context.Context) (*DSMInfo, error) {
	var out DSMInfo
	if err := c.callJSON(ctx, APIDSMInfo, "getinfo", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	var out SystemInfo
	if err := c.callJSON(ctx, APISystemInfo, "info", &out); err != nil {
		return nil, err
	}
	return &out, nil
}
