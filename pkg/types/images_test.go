package types

import (
	"math"
	"strings"
	"testing"
)

func TestResolveImageLimits(t *testing.T) {
	for _, tc := range []struct {
		name      string
		cfg       Config
		wantImage int64
		wantTotal int64
		wantError string
	}{
		{"defaults", Config{}, 10 * MiB, 40 * MiB, ""},
		{"overrides", Config{ImageMaxSizeMB: 20, ImageMaxTotalSizeMB: 80}, 20 * MiB, 80 * MiB, ""},
		{"partial", Config{ImageMaxSizeMB: 2}, 2 * MiB, 40 * MiB, ""},
		{"smaller-total", Config{ImageMaxSizeMB: 20, ImageMaxTotalSizeMB: 1}, 20 * MiB, MiB, ""},
		{"negative-file", Config{ImageMaxSizeMB: -1}, 0, 0, "image_max_size_mb"},
		{"negative-total", Config{ImageMaxTotalSizeMB: -1}, 0, 0, "image_max_total_size_mb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			perImage, total, err := ResolveImageLimits(&tc.cfg)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("expected %s error, got %v", tc.wantError, err)
				}
				return
			}
			if err != nil || perImage != tc.wantImage || total != tc.wantTotal {
				t.Fatalf("got %d/%d, %v", perImage, total, err)
			}
		})
	}
	maxInt := math.MaxInt
	if int64(maxInt) > math.MaxInt64/MiB {
		if _, _, err := ResolveImageLimits(&Config{ImageMaxSizeMB: maxInt}); err == nil {
			t.Fatal("overflowing MiB value accepted")
		}
	}
}
