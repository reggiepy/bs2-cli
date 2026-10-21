package device

import "fmt"

// Level maps the original BS2 UI's nine levels to fixed target RPM.
type Level struct {
	Number     int    `json:"level"`
	Group      string `json:"group"`
	GroupLevel int    `json:"group_level"`
	RPM        int    `json:"rpm"`
}

// Levels returns the BS2 mapping from the vendor's FanSpeedHelper.
func Levels() []Level {
	speeds := []int{1300, 1700, 1900, 2100, 2400, 2700, 2800, 2900, 3000}
	groups := []string{"silent", "standard", "strong"}
	result := make([]Level, 0, 9)
	for i, rpm := range speeds {
		result = append(result, Level{i + 1, groups[i/3], i%3 + 1, rpm})
	}
	return result
}

// LevelRPM validates a level and returns its fixed RPM.
func LevelRPM(level int) (int, error) {
	if level < 1 || level > 9 {
		return 0, fmt.Errorf("BS2 level must be 1..9")
	}
	return Levels()[level-1].RPM, nil
}
