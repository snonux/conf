package internal

import (
	"fmt"
	"time"
)

// humanBytes renders n with decimal (SI) units: 20226448 -> "20.2 MB".
func humanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value, exp := float64(n)/unit, 0
	for value >= unit && exp < 5 {
		value /= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", value, "kMGTPE"[exp])
}

// humanDuration renders d as minutes or hours and minutes: "45m", "2h05m".
func humanDuration(d time.Duration) string {
	minutes := int64(d / time.Minute)
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dh%02dm", minutes/60, minutes%60)
}
