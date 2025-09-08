package textutil

import "fmt"

// HumanBytes renders n in a short human-friendly form using base-1000 units.
// Examples: 999 -> 999B, 1200 -> 1.2k, 1540000 -> 1.5M
func HumanBytes(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%dB", n)
	}
	const unit = 1000.0
	v := float64(n)
	suffix := []string{"k", "M", "G", "T"}
	i := 0
	for v >= unit && i < len(suffix)-1 {
		v /= unit
		i++
	}
	s := fmt.Sprintf("%.1f%s", v, suffix[i])
	// Strip trailing ".0"
	if len(s) >= 3 && s[len(s)-2:] == ".0" {
		s = fmt.Sprintf("%d%s", int(v), suffix[i])
	}
	return s
}
