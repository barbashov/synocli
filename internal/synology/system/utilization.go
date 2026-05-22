package system

import "context"

// Utilization is the response shape of SYNO.Core.System.Utilization `get`.
// Values are an instantaneous snapshot at request time.
type Utilization struct {
	CPU     CPUUtilization     `json:"cpu"`
	Memory  MemoryUtilization  `json:"memory"`
	Disk    DiskUtilizationSet `json:"disk"`
	Network []NetworkUsage     `json:"network"`
	Time    int64              `json:"time"`
}

type CPUUtilization struct {
	UserLoad   int `json:"user_load"`
	SystemLoad int `json:"system_load"`
	OtherLoad  int `json:"other_load"`
	Load1Min   int `json:"1min_load"`
	Load5Min   int `json:"5min_load"`
	Load15Min  int `json:"15min_load"`
}

type MemoryUtilization struct {
	RealUsage int   `json:"real_usage"`
	SwapUsage int   `json:"swap_usage"`
	TotalReal int64 `json:"total_real"` // kilobytes
	AvailReal int64 `json:"avail_real"` // kilobytes
	TotalSwap int64 `json:"total_swap"` // kilobytes
	AvailSwap int64 `json:"avail_swap"` // kilobytes
	Cached    int64 `json:"cached"`     // kilobytes
	Buffer    int64 `json:"buffer"`     // kilobytes
}

type DiskUtilizationSet struct {
	Disks []DiskUtilization `json:"disk"`
	Total DiskUtilization   `json:"total"`
}

type DiskUtilization struct {
	Device      string `json:"device"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
	Utilization int    `json:"utilization"`
	ReadAccess  int    `json:"read_access"`
	WriteAccess int    `json:"write_access"`
	ReadByte    int64  `json:"read_byte"`
	WriteByte   int64  `json:"write_byte"`
}

type NetworkUsage struct {
	Device string `json:"device"`
	RX     int64  `json:"rx"`
	TX     int64  `json:"tx"`
}

func (c *Client) GetUtilization(ctx context.Context) (*Utilization, error) {
	var out Utilization
	if err := c.callJSON(ctx, APIUtilization, "get", &out); err != nil {
		return nil, err
	}
	return &out, nil
}
