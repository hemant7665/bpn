package cdcutil

import (
	"fmt"
	"strings"
)

// StringFromData returns a string form of a CDC/data map value (DMS JSON often uses numbers for ids).
func StringFromData(data map[string]any, key string) (string, error) {
	val, ok := data[key]
	if !ok || val == nil {
		return "", fmt.Errorf("missing required field %q", key)
	}
	switch t := val.(type) {
	case string:
		return strings.TrimSpace(t), nil
	case float64:
		return trimFloatString(t), nil
	case bool:
		if t {
			return "true", nil
		}
		return "false", nil
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		if s == "" {
			return "", fmt.Errorf("field %q is empty", key)
		}
		return s, nil
	}
}

func trimFloatString(f float64) string {
	s := fmt.Sprintf("%.0f", f)
	return strings.TrimRight(strings.TrimRight(s, "0"), ".")
}
