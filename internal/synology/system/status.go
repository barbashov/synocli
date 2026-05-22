package system

import "context"

type NeedReboot struct {
	NeedReboot bool `json:"need_reboot"`
}

type SystemStatus struct {
	IsSystemCrashed bool `json:"is_system_crashed"`
	UpgradeReady    bool `json:"upgrade_ready"`
}

func (c *Client) GetNeedReboot(ctx context.Context) (*NeedReboot, error) {
	var out NeedReboot
	if err := c.callJSON(ctx, APINeedReboot, "get", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetSystemStatus(ctx context.Context) (*SystemStatus, error) {
	var out SystemStatus
	if err := c.callJSON(ctx, APISystemStatus, "get", &out); err != nil {
		return nil, err
	}
	return &out, nil
}
