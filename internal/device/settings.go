package device

import (
	"context"
	"fmt"
)

// SettingResult reports device acknowledgement; these commands have no confirmed getter.
type SettingResult struct {
	Setting          string `json:"setting"`
	Value            string `json:"value"`
	Acknowledged     bool   `json:"acknowledged"`
	ReadbackVerified bool   `json:"readback_verified"`
}

// SetOption changes one native BS2 setting using the vendor's command encoding.
func (c Client) SetOption(ctx context.Context, name, value string) (SettingResult, error) {
	result := SettingResult{Setting: name, Value: value}
	var cmd, payload byte
	switch name {
	case "gear-led":
		cmd = 0x48
		switch value {
		case "on":
			payload = 1
		case "off":
			payload = 0
		default:
			return result, fmt.Errorf("gear-led accepts on/off")
		}
	case "power-on":
		cmd = 0x0c
		switch value {
		case "on":
			payload = 1
		case "off":
			payload = 2
		default:
			return result, fmt.Errorf("power-on accepts on/off")
		}
	case "smart-stop":
		cmd = 0x0d
		switch value {
		case "off":
			payload = 0
		case "instant":
			payload = 1
		case "delay":
			payload = 2
		default:
			return result, fmt.Errorf("smart-stop accepts off/instant/delay")
		}
	default:
		return result, fmt.Errorf("unknown setting %q", name)
	}
	if err := c.ack(ctx, cmd, []byte{payload}, false); err != nil {
		return result, err
	}
	result.Acknowledged = true
	return result, nil
}
