// internal/registry/number_test.go
package registry_test

import (
	"fmt"
	"testing"
)

func TestNumberFormatting(t *testing.T) {
	tests := []struct {
		prefix   string
		num      int
		expected string
	}{
		{"INT/", 1, "INT/000001"},
		{"IEST/", 42, "IEST/000042"},
		{"CTR/", 1000, "CTR/001000"},
		{"", 999999, "999999"},
	}
	for _, tt := range tests {
		got := fmt.Sprintf("%s%06d", tt.prefix, tt.num)
		if got != tt.expected {
			t.Errorf("prefix=%q num=%d: want %q got %q", tt.prefix, tt.num, tt.expected, got)
		}
	}
}
