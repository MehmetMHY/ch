package platform

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/MehmetMHY/ch/pkg/types"
)

func requestTestImage(t *testing.T) types.ImageAttachment {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return types.ImageAttachment{ID: "image", Name: "shot.png", MIMEType: "image/png", Data: b.Bytes()}
}

func TestNativeImageRequestPayloads(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "non-streaming", true: "streaming"}[stream], func(t *testing.T) {
			resetModelsDevClient(t)
			t.Setenv("IMAGE_TEST_KEY", "test-key")
			server, captured := captureRequestPayload(t, stream)
			defer server.Close()
			enabled := true
			cfg := &types.Config{CurrentPlatform: "test", ImageControl: "!i", ModelsDevEnabled: false, Platforms: map[string]types.Platform{"test": {Name: "test", BaseURL: types.BaseURLValue{Single: server.URL + "/v1"}, EnvName: "IMAGE_TEST_KEY", ImageInput: &enabled}}}
			if !stream {
				cfg.SlowModelPatterns = []string{".*"}
			}
			m := NewManager(cfg)
			if err := m.Initialize(); err != nil {
				t.Fatal(err)
			}
			img := requestTestImage(t)
			messages := []types.ChatMessage{{Role: "user", Content: "File: shot.png", Images: []types.ImageAttachment{img}}, {Role: "user", Content: "describe it"}}
			cancel := func() {}
			if _, err := m.SendChatRequest(messages, "vision", &cancel, new(bool)); err != nil {
				t.Fatal(err)
			}
			var payload struct {
				Messages []struct {
					Role    string          `json:"role"`
					Content json.RawMessage `json:"content"`
				} `json:"messages"`
			}
			if err := json.Unmarshal([]byte(*captured), &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Messages) != 1 {
				t.Fatalf("user merging failed: %s", *captured)
			}
			var parts []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				ImageURL struct {
					URL string `json:"url"`
				} `json:"image_url"`
			}
			if err := json.Unmarshal(payload.Messages[0].Content, &parts); err != nil {
				t.Fatal(err)
			}
			if len(parts) != 2 || parts[0].Type != "text" || !strings.Contains(parts[0].Text, "describe it") || parts[1].Type != "image_url" || parts[1].ImageURL.URL != "data:image/png;base64,"+base64.StdEncoding.EncodeToString(img.Data) {
				t.Fatalf("bad multipart payload: %s", *captured)
			}
			silentServer, silentCaptured := captureRequestPayload(t, false)
			defer silentServer.Close()
			cfg.CurrentBaseURL = silentServer.URL + "/v1"
			if err := m.Initialize(); err != nil {
				t.Fatal(err)
			}
			if _, err := m.SendSilentChatRequest(messages, "vision", &cancel, new(bool)); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(*silentCaptured, "image_url") || strings.Contains(*silentCaptured, "base64") {
				t.Fatal("auxiliary request sent image bytes")
			}
		})
	}
}

func TestImageCapabilitiesAndBlockedRequests(t *testing.T) {
	resetModelsDevClient(t)
	writeCacheFile(t, ModelsDevCatalog{"google": {Models: map[string]ModelsDevModel{
		"vision": {Modalities: &ModelsDevModalities{Input: []string{"text", "image"}}},
		"text":   {Modalities: &ModelsDevModalities{Input: []string{"text"}}},
		"old":    {},
	}}})
	cfg := &types.Config{CurrentPlatform: "google", ModelsDevEnabled: false, ImageControl: "!i"}
	for model, want := range map[string]string{"models/vision": CapStatusSupported, "text": CapStatusUnsupported, "old": CapStatusUnknown, "missing": CapStatusUnknown} {
		if got := ResolveImageCapability("google", model, cfg); got != want {
			t.Fatalf("%s: got %s want %s", model, got, want)
		}
	}
	m := NewManager(cfg)
	messages := []types.ChatMessage{{Role: "user", Content: "describe", Images: []types.ImageAttachment{requestTestImage(t)}}}
	// A nil provider client proves known unsupported requests stop before I/O.
	if _, err := m.SendChatRequest(messages, "text", new(func()), new(bool)); err == nil || !strings.Contains(err.Error(), "!i off") {
		t.Fatalf("expected actionable error: %v", err)
	}
	if _, err := m.buildChatMessages(messages, "missing"); err != nil {
		t.Fatalf("unknown model must allow explicit image attempt: %v", err)
	}
	cfg.CurrentPlatform = "deepseek"
	if _, err := m.SendChatRequest(messages, "vision", new(func()), new(bool)); err == nil {
		t.Fatal("unsupported transport allowed images")
	}
	if out, err := m.buildChatMessages([]types.ChatMessage{{Role: "user", Content: "text only"}}, "vision"); err != nil || out[0].Content != "text only" {
		t.Fatal("text-only request was blocked")
	}
}

func TestImageTransportOverridesAndAliases(t *testing.T) {
	resetModelsDevClient(t)
	writeCacheFile(t, ModelsDevCatalog{"togetherai": {Models: map[string]ModelsDevModel{"vision": {Modalities: &ModelsDevModalities{Input: []string{"image"}}}}}})
	cfg := &types.Config{CurrentPlatform: "together", ModelsDevEnabled: false, Platforms: map[string]types.Platform{"together": {ModelsDevProvider: "togetherai"}}}
	if !SupportsImageTransport(cfg) || ResolveImageCapability("together", "vision", cfg) != CapStatusSupported {
		t.Fatal("provider alias did not resolve vision model")
	}
	for _, platform := range []string{"amazon", "deepseek", "custom"} {
		cfg.CurrentPlatform = platform
		if SupportsImageTransport(cfg) {
			t.Fatalf("%s should require transport override", platform)
		}
	}
	enabled := true
	cfg.Platforms["custom"] = types.Platform{ImageInput: &enabled}
	if !SupportsImageTransport(cfg) {
		t.Fatal("custom endpoint override ignored")
	}
	enabled = false
	cfg.Platforms["together"] = types.Platform{ImageInput: &enabled}
	cfg.CurrentPlatform = "together"
	if SupportsImageTransport(cfg) {
		t.Fatal("built-in endpoint false override ignored")
	}
}

func TestConfiguredImageRequestLimits(t *testing.T) {
	resetModelsDevClient(t)
	cfg := &types.Config{CurrentPlatform: "google", ModelsDevEnabled: false, ImageControl: "!i", ImageMaxSizeMB: 1, ImageMaxTotalSizeMB: 2}
	m := NewManager(cfg)
	img := requestTestImage(t)
	img.Data = append(img.Data, make([]byte, types.MiB)...)
	messages := []types.ChatMessage{{Role: "user", Content: "describe", Images: []types.ImageAttachment{img}}}
	if _, err := m.buildChatMessages(messages, "vision"); err == nil || !strings.Contains(err.Error(), "1 MiB") {
		t.Fatalf("request per-image limit ignored: %v", err)
	}
	cfg.ImageMaxSizeMB = 2
	if _, err := m.buildChatMessages(messages, "vision"); err != nil {
		t.Fatalf("raised request per-image limit ignored: %v", err)
	}
	messages[0].Images = append(messages[0].Images, img)
	if _, err := m.buildChatMessages(messages, "vision"); err == nil || !strings.Contains(err.Error(), "2 MiB request limit") {
		t.Fatalf("request total limit ignored: %v", err)
	}
	cfg.ImageMaxTotalSizeMB = 3
	if _, err := m.buildChatMessages(messages, "vision"); err != nil {
		t.Fatalf("raised request total limit ignored: %v", err)
	}
}
