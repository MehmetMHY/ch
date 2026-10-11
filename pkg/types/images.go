package types

import (
	"fmt"
	"math"
	"net/http"
)

const (
	MiB                        = 1024 * 1024
	DefaultImageMaxSizeMB      = 10
	DefaultImageMaxTotalSizeMB = 40
)

// ResolveImageLimits returns byte limits from MiB config values. Zero preserves
// the defaults for omitted options and minimally constructed runtime configs.
func ResolveImageLimits(cfg *Config) (int64, int64, error) {
	perImage, total := cfg.ImageMaxSizeMB, cfg.ImageMaxTotalSizeMB
	if perImage == 0 {
		perImage = DefaultImageMaxSizeMB
	}
	if total == 0 {
		total = DefaultImageMaxTotalSizeMB
	}
	for _, limit := range []struct {
		key   string
		value int
	}{
		{"image_max_size_mb", perImage},
		{"image_max_total_size_mb", total},
	} {
		if limit.value < 0 || int64(limit.value) > math.MaxInt64/MiB {
			return 0, 0, fmt.Errorf("%s must be a positive whole number of MiB (0 uses the default)", limit.key)
		}
	}
	return int64(perImage) * MiB, int64(total) * MiB, nil
}

// ImageAttachment captures the selected file, independent of its original path.
// JSON encodes Data as base64 once in the session's top-level images array.
type ImageAttachment struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MIMEType string `json:"mime_type"`
	Data     []byte `json:"data"`
	Disabled bool   `json:"disabled,omitempty"`
}

func (img ImageAttachment) Validate(maxBytes int64) error {
	if len(img.Data) == 0 || int64(len(img.Data)) > maxBytes {
		return fmt.Errorf("image %s must be non-empty and at most %d MiB (image_max_size_mb)", img.Name, maxBytes/MiB)
	}
	switch img.MIMEType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		return fmt.Errorf("unsupported image format for %s: use PNG, JPEG, GIF, or WebP", img.Name)
	}
	if http.DetectContentType(img.Data) != img.MIMEType {
		return fmt.Errorf("image content does not match MIME type for %s", img.Name)
	}
	return nil
}

// LoadedContent keeps native image bytes separate from human-readable text.
type LoadedContent struct {
	Text     string
	Images   []ImageAttachment
	Warnings []string
}
