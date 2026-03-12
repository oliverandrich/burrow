package templates

import (
	"fmt"
	"time"
)

// isTruthy returns true for bool true, non-zero numbers, and "true"/"on"/"1" strings.
func isTruthy(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", val) != "0"
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val) != "0"
	case string:
		return val == "true" || val == "on" || val == "1"
	default:
		return false
	}
}

// formatDateValue formats a value as YYYY-MM-DD for date inputs.
func formatDateValue(v any) string {
	if t, ok := v.(time.Time); ok {
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02")
	}
	return fmt.Sprintf("%v", v)
}
