package textutil

import "testing"

// TestHumanBytes validates the HumanBytes function with comprehensive edge cases
// and boundary values to ensure proper formatting across all unit ranges.
func TestHumanBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		// Basic cases
		{"zero", 0, "0B"},
		{"one byte", 1, "1B"},
		{"small", 42, "42B"},
		{"large bytes", 999, "999B"},

		// Kilobyte boundary - the function starts at index 0 ("k") after dividing once
		{"1 KB exact", 1000, "1k"},
		{"just under 1 KB", 999, "999B"},
		{"1.5 KB", 1500, "1.5k"},
		{"1.9 KB", 1900, "1.9k"},
		{"2 KB", 2000, "2k"},
		{"10 KB", 10000, "10k"},
		{"99 KB", 99000, "99k"},
		{"999 KB", 999000, "999k"},

		// Megabyte boundary
		{"1 MB exact", 1000000, "1M"},
		{"just under 1 MB", 999999, "999k"}, // Truncates to 999.999, formats as 999k
		{"1.5 MB", 1500000, "1.5M"},
		{"10 MB", 10000000, "10M"},
		{"99 MB", 99000000, "99M"},
		{"999 MB", 999000000, "999M"},

		// Gigabyte boundary
		{"1 GB exact", 1000000000, "1G"},
		{"1.5 GB", 1500000000, "1.5G"},
		{"10 GB", 10000000000, "10G"},
		{"99 GB", 99000000000, "99G"},
		{"999 GB", 999000000000, "999G"},

		// Terabyte boundary
		{"1 TB exact", 1000000000000, "1T"},
		{"1.5 TB", 1500000000000, "1.5T"},
		{"10 TB", 10000000000000, "10T"},
		{"99 TB", 99000000000000, "99T"},
		{"999 TB", 999000000000000, "999T"},
		{"1000 TB (stays in T)", 1000000000000000, "1000T"},
		{"very large (petabytes)", 9999000000000000, "9999T"},

		// Precision tests (values that round)
		{"1536 bytes", 1536, "1.5k"},
		{"1024 bytes", 1024, "1k"},
		{"1234 bytes", 1234, "1.2k"},
		{"1999 bytes", 1999, "1k"}, // 1.999 truncates to 1.0k with %.1f, strips to 1k
		{"123456 bytes", 123456, "123.5k"},
		{"1234567 bytes", 1234567, "1.2M"},

		// Edge cases with decimal precision
		{"100.1 KB", 100100, "100.1k"},
		{"100.9 KB", 100900, "100.9k"},
		{"1.1 MB", 1100000, "1.1M"},
		{"9.9 GB", 9900000000, "9.9G"},

		// Values that should strip .0
		{"exactly 3 KB", 3000, "3k"},
		{"exactly 5 MB", 5000000, "5M"},
		{"exactly 7 GB", 7000000000, "7G"},
		{"exactly 2 TB", 2000000000000, "2T"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HumanBytes(tt.bytes)
			if got != tt.expected {
				t.Errorf("HumanBytes(%d) = %q, want %q", tt.bytes, got, tt.expected)
			}
		})
	}
}

// TestHumanBytesNegative tests behavior with negative values (if applicable)
func TestHumanBytesNegative(t *testing.T) {
	// The function signature uses int64, so negative values are technically possible
	// though they may not be semantically meaningful for byte counts.
	// Test that they don't cause panics and behave reasonably.
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"negative small", -42, "-42B"},
		{"negative KB", -1500, "-1.5k"},
		{"negative MB", -1500000, "-1.5M"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HumanBytes(tt.bytes)
			if got != tt.expected {
				t.Errorf("HumanBytes(%d) = %q, want %q", tt.bytes, got, tt.expected)
			}
		})
	}
}
