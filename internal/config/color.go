package config

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseHexColor parses "#212121" or "212121" to Discord embed color int.
func ParseHexColor(hex string) (int, error) {
	h := strings.TrimSpace(strings.TrimPrefix(hex, "#"))
	if len(h) != 6 {
		return 0, fmt.Errorf("invalid hex color %q", hex)
	}
	v, err := strconv.ParseUint(h, 16, 24)
	if err != nil {
		return 0, fmt.Errorf("invalid hex color %q: %w", hex, err)
	}
	return int(v), nil
}
