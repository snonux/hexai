package textutil

import "fmt"

// HumanBytes renders n in a short human-friendly form using base-1000 units.
// Examples: 999 -> 999B, 1200 -> 1.2k, 1540000 -> 1.5M
func HumanBytes(n int64) string {
	// Handle negative values by processing absolute value and adding sign back
	negative := n < 0
	if negative {
		n = -n
	}
	if n < 1000 {
		result := fmt.Sprintf("%dB", n)
		if negative {
			return "-" + result
		}
		return result
	}
	const unit = 1000.0
	v := float64(n)
	suffix := []string{"k", "M", "G", "T"}
	// Divide first to get into the k range, then loop for higher units
	v /= unit
	i := 0
	for v >= unit && i < len(suffix)-1 {
		v /= unit
		i++
	}
	s := fmt.Sprintf("%.1f%s", v, suffix[i])
	// Strip trailing ".0" before the suffix (e.g., "1.0k" -> "1k")
	// Find the suffix position and check if ".0" precedes it
	suffixLen := len(suffix[i])
	if len(s) >= suffixLen+3 {
		// Check if the portion before the suffix ends with ".0"
		beforeSuffix := s[:len(s)-suffixLen]
		if len(beforeSuffix) >= 2 && beforeSuffix[len(beforeSuffix)-2:] == ".0" {
			s = fmt.Sprintf("%d%s", int(v), suffix[i])
		}
	}
	if negative {
		return "-" + s
	}
	return s
}
