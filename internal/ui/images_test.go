package ui

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MehmetMHY/ch/pkg/types"
)

func TestLoadChatContentNativeImages(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	var original bytes.Buffer
	if err := png.Encode(&original, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(home, "screenshot.PNG")
	textPath := filepath.Join(home, "notes.txt")
	if err := os.WriteFile(imagePath, original.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(textPath, []byte("compare these pixels"), 0600); err != nil {
		t.Fatal(err)
	}
	terminal := NewTerminal(&types.Config{})
	for _, selections := range [][]string{{textPath, imagePath}, {home}} {
		loaded, err := terminal.LoadChatContent(selections, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(loaded.Images) != 1 || loaded.Images[0].MIMEType != "image/png" || !bytes.Equal(loaded.Images[0].Data, original.Bytes()) {
			t.Fatalf("native image was not captured: %+v", loaded.Images)
		}
		if !strings.Contains(loaded.Text, "compare these pixels") || !strings.Contains(loaded.Text, "[image]") || strings.Contains(loaded.Text, "OCR") {
			t.Fatalf("unexpected text: %s", loaded.Text)
		}
	}
	// Print-only extraction is still independent of native attachment loading.
	text, err := terminal.LoadFileContent([]string{imagePath})
	if err != nil || !strings.Contains(text, "text extraction (OCR)") {
		t.Fatalf("print-only image behavior changed: %s, %v", text, err)
	}
}

func TestLoadChatContentImageErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		want string
	}{
		{"empty.png", nil, "non-empty"},
		{"invalid.png", []byte("not an image"), "unsupported image format"},
		{"large.png", make([]byte, types.DefaultImageMaxSizeMB*types.MiB+1), "10 MiB"},
		{"bitmap.bmp", []byte{'B', 'M', 0, 0, 0, 0, 0, 0}, "unsupported image format"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.name)
			if err := os.WriteFile(path, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			loaded, err := NewTerminal(&types.Config{}).loadChatContent([]string{path}, true, func([]byte) (string, error) {
				t.Fatal("invalid image reached OCR before validation")
				return "", nil
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) || len(loaded.Images) != 0 {
				t.Fatalf("loaded=%+v err=%v", loaded, err)
			}
		})
	}
}

func TestLoadChatContentPausedOCR(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	var original bytes.Buffer
	if err := png.Encode(&original, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "screenshot.png")
	notes := filepath.Join(home, "notes.txt")
	if err := os.WriteFile(path, original.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(notes, []byte("plain text context"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		off  bool
		text string
		err  error
	}{
		{"sending-on", false, "Invoice total: 42", nil},
		{"paused-text", true, "  Invoice total: 42\n", nil},
		{"paused-empty", true, " \n", nil},
		{"paused-unavailable", true, "", errors.New("Tesseract unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, selections := range [][]string{{notes, path}, {home}} {
				calls := 0
				loaded, err := NewTerminal(&types.Config{}).loadChatContent(selections, tc.off, func(data []byte) (string, error) {
					calls++
					if !bytes.Equal(data, original.Bytes()) {
						t.Fatal("OCR did not use captured image bytes")
					}
					return tc.text, tc.err
				})
				if err != nil || len(loaded.Images) != 1 || !bytes.Equal(loaded.Images[0].Data, original.Bytes()) || loaded.Images[0].Disabled {
					t.Fatalf("native attachment lost: %+v, %v", loaded.Images, err)
				}
				wantCalls := 0
				if tc.off {
					wantCalls = 1
				}
				if calls != wantCalls || !strings.Contains(loaded.Text, "plain text context") || !strings.Contains(loaded.Text, "[image]") {
					t.Fatalf("unexpected extraction calls/text: %d, %q", calls, loaded.Text)
				}
				wantText := tc.off && tc.err == nil && strings.TrimSpace(tc.text) != ""
				if strings.Contains(loaded.Text, "text extraction (OCR):") != wantText || strings.Contains(loaded.Text, "text extraction (OCR):\nInvoice total: 42\n") != wantText {
					t.Fatalf("unexpected OCR context: %q", loaded.Text)
				}
				if tc.err != nil {
					if len(loaded.Warnings) != 1 || !strings.Contains(loaded.Warnings[0], "OCR unavailable for screenshot.png") || !strings.Contains(loaded.Warnings[0], tc.err.Error()) || strings.Contains(loaded.Text, tc.err.Error()) {
						t.Fatalf("warning/context mismatch: %+v", loaded)
					}
				} else if len(loaded.Warnings) != 0 {
					t.Fatalf("unexpected warnings: %v", loaded.Warnings)
				}
			}
		})
	}
}

func TestLoadChatContentConfiguredLimits(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 11*types.MiB)
	copy(data, pngData.Bytes())
	path := filepath.Join(home, "large.png")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		cfg       types.Config
		count     int
		wantError string
	}{
		{"default-file", types.Config{}, 1, "10 MiB"},
		{"raised-file", types.Config{ImageMaxSizeMB: 12}, 1, ""},
		{"lowered-file", types.Config{ImageMaxSizeMB: 1}, 1, "1 MiB"},
		{"lowered-total", types.Config{ImageMaxSizeMB: 12, ImageMaxTotalSizeMB: 20}, 2, "20 MiB total limit"},
		{"raised-total", types.Config{ImageMaxSizeMB: 12, ImageMaxTotalSizeMB: 24}, 2, ""},
		{"total-below-file", types.Config{ImageMaxSizeMB: 12, ImageMaxTotalSizeMB: 1}, 1, "1 MiB total limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selections := []string{path}
			if tc.count == 2 {
				selections = append(selections, path)
			}
			loaded, err := NewTerminal(&tc.cfg).LoadChatContent(selections, false)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) || len(loaded.Images) != 0 {
					t.Fatalf("expected %s, got %v", tc.wantError, err)
				}
			} else if err != nil || len(loaded.Images) != tc.count || !bytes.Equal(loaded.Images[0].Data, data) {
				t.Fatalf("configured image load failed: %v", err)
			}
		})
	}
}
