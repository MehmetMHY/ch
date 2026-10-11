package chat

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/MehmetMHY/ch/pkg/types"
)

func imageTestState(t *testing.T) (*Manager, *types.AppState, types.ImageAttachment) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	cfg := &types.Config{SystemPrompt: "system", CurrentPlatform: "ollama", CurrentModel: "vision", EnableSessionSave: true}
	state := &types.AppState{Config: cfg, Messages: []types.ChatMessage{{Role: "system", Content: "system"}}, ChatHistory: []types.ChatHistory{{User: "system"}}}
	return NewManager(state), state, types.ImageAttachment{ID: "image-1", Name: "shot.png", MIMEType: "image/png", Data: b.Bytes()}
}

func TestImagesLifecycle(t *testing.T) {
	m, state, img := imageTestState(t)
	if err := m.AddLoadedContent("Loaded: shot.png", types.LoadedContent{Text: "File: shot.png [image]", Images: []types.ImageAttachment{img}}); err != nil {
		t.Fatal(err)
	}
	m.AddUserMessage("describe")
	// Request failure/cancellation rolls back the prompt, not the loaded image.
	if !m.RemovePendingUserMessage("describe") || len(m.GetMessages()[1].Images) != 1 {
		t.Fatal("pending prompt rollback lost image")
	}
	state.ImagesOff = true
	if msg := m.GetMessages()[1]; len(msg.Images) != 0 || !strings.Contains(msg.Content, "not sent") {
		t.Fatalf("off still sends images: %+v", msg)
	}
	state.ImagesOff = false
	state.Images[0].Disabled = true
	if len(m.GetMessages()[1].Images) != 0 {
		t.Fatal("disabled image sent")
	}
	state.Images[0].Disabled = false
	m.AddUserMessage("describe")
	m.AddAssistantMessage("a screenshot")
	m.AddToHistory("describe", "a screenshot")
	state.Compressions = []types.CompressionRecord{{ThroughIndex: 2, Summary: "discussed screenshot"}}
	m.rebuildMessages()
	if len(m.GetMessages()[1].Images) != 1 {
		t.Fatal("compression dropped original image")
	}
	if !strings.Contains(m.buildCompressionTranscript(), "visual contents are not included") {
		t.Fatal("compression transcript misrepresents image contents")
	}
	// Backtrack below the load, using the same reconstruction path as !b.
	state.ChatHistory = state.ChatHistory[:1]
	m.pruneCompressions(1)
	m.rebuildMessages()
	if len(state.Images) != 0 || len(m.GetMessages()) != 1 {
		t.Fatal("backtrack retained discarded image")
	}
	if err := m.AddLoadedContent("Loaded: shot.png", types.LoadedContent{Images: []types.ImageAttachment{img}}); err != nil {
		t.Fatal(err)
	}
	state.ImagesOff = true
	m.ClearHistory()
	if len(state.Images) != 0 || !state.ImagesOff {
		t.Fatal("clear should remove images and preserve paused sending")
	}
}

func TestClearHistoryPreservesImageSendingPreference(t *testing.T) {
	for _, off := range []bool{false, true} {
		name := "on"
		if off {
			name = "off"
		}
		t.Run(name, func(t *testing.T) {
			m, state, img := imageTestState(t)
			loaded := types.LoadedContent{Text: "File: shot.png [image]", Images: []types.ImageAttachment{img}}
			if err := m.AddLoadedContent("Loaded: shot.png", loaded); err != nil {
				t.Fatal(err)
			}
			state.ImagesOff = off
			state.Images[0].Disabled = true
			m.ClearHistory()
			if state.ImagesOff != off || len(state.Images) != 0 || len(state.ChatHistory) != 1 || len(state.Messages) != 1 {
				t.Fatal("clear did not remove content while preserving the image sending setting")
			}
			if err := m.SaveSessionState(); err != nil {
				t.Fatal(err)
			}
			session, err := m.LoadCustomHistoryFile(state.SessionFilePath)
			if err != nil {
				t.Fatal(err)
			}
			state.ImagesOff = !off
			m.RestoreSessionState(session)
			if state.ImagesOff != off {
				t.Fatal("cleared session lost image sending preference on restore")
			}
			if err := m.AddLoadedContent("Loaded again: shot.png", loaded); err != nil {
				t.Fatal(err)
			}
			want := 1
			if off {
				want = 0
			}
			if len(m.GetMessages()[1].Images) != want {
				t.Fatal("newly loaded image does not respect preserved sending preference")
			}
		})
	}
}

func TestImagesSessionRoundTrip(t *testing.T) {
	m, state, img := imageTestState(t)
	if err := m.AddLoadedContent("Loaded: shot.png", types.LoadedContent{Text: "File: shot.png [image]", Images: []types.ImageAttachment{img}}); err != nil {
		t.Fatal(err)
	}
	state.ImagesOff = true
	state.Images[0].Disabled = true
	if err := m.SaveSessionState(); err != nil {
		t.Fatal(err)
	}
	session, err := m.LoadCustomHistoryFile(state.SessionFilePath)
	if err != nil {
		t.Fatal(err)
	}
	state.Images = nil
	state.ImagesOff = false
	m.RestoreSessionState(session)
	if !state.ImagesOff || len(state.Images) != 1 || !state.Images[0].Disabled || !bytes.Equal(state.Images[0].Data, img.Data) {
		t.Fatal("image state did not round-trip")
	}
	state.ImagesOff = false
	state.Images[0].Disabled = false
	if len(m.GetMessages()[1].Images) != 1 {
		t.Fatal("restored image cannot be re-enabled")
	}
	state.Config.SaveAllSessions = true
	m.ForkSessionOnNextSave()
	baseline := m.sessionSaveFingerprint()
	state.ImagesOff = true
	if baseline == m.sessionSaveFingerprint() {
		t.Fatal("image controls do not trigger session fork")
	}
	// Legacy session loading resets all new state.
	m.RestoreSessionState(&types.SessionFile{ChatHistory: []types.ChatHistory{{User: "system"}, {User: "hello"}}})
	if len(state.Images) != 0 || state.ImagesOff {
		t.Fatal("legacy restore retained image state")
	}
}

func TestPausedImageOCRContextPersistence(t *testing.T) {
	m, state, img := imageTestState(t)
	state.ImagesOff = true
	text := "File: shot.png [image]\n\ntext extraction (OCR):\nInvoice total: 42\n"
	if err := m.AddLoadedContent("Loaded: shot.png", types.LoadedContent{Text: text, Images: []types.ImageAttachment{img}}); err != nil {
		t.Fatal(err)
	}
	if msg := m.GetMessages()[1]; len(msg.Images) != 0 || !strings.Contains(msg.Content, "Invoice total: 42") {
		t.Fatalf("paused request lost OCR text or sent pixels: %+v", msg)
	}
	if err := m.SaveSessionState(); err != nil {
		t.Fatal(err)
	}
	session, err := m.LoadCustomHistoryFile(state.SessionFilePath)
	if err != nil {
		t.Fatal(err)
	}
	m.ClearHistory()
	m.RestoreSessionState(session)
	if !state.ImagesOff || state.ChatHistory[1].Context != text || !strings.Contains(m.GetMessages()[1].Content, "Invoice total: 42") {
		t.Fatal("restored session lost OCR context or paused setting")
	}
	state.ImagesOff = false
	if msg := m.GetMessages()[1]; len(msg.Images) != 1 || !bytes.Equal(msg.Images[0].Data, img.Data) || msg.Content != text {
		t.Fatal("resuming image did not preserve original pixels and OCR context")
	}
}

func TestImageSessionLimitIsAtomic(t *testing.T) {
	m, state, img := imageTestState(t)
	state.Images = []types.ImageAttachment{{Data: make([]byte, types.DefaultImageMaxTotalSizeMB*types.MiB)}}
	if err := m.AddLoadedContent("image", types.LoadedContent{Images: []types.ImageAttachment{img}}); err == nil {
		t.Fatal("expected session limit error")
	}
	if len(state.ChatHistory) != 1 || len(state.Messages) != 1 || len(state.Images) != 1 {
		t.Fatal("failed load mutated state")
	}
}

func TestConfiguredImageSessionLimits(t *testing.T) {
	m, state, img := imageTestState(t)
	img.Data = append(img.Data, make([]byte, types.MiB)...)
	state.Config.ImageMaxSizeMB = 1
	loaded := types.LoadedContent{Text: "image", Images: []types.ImageAttachment{img}}
	if err := m.AddLoadedContent("load", loaded); err == nil || !strings.Contains(err.Error(), "1 MiB") {
		t.Fatalf("per-image limit ignored: %v", err)
	}
	state.Config.ImageMaxSizeMB = 2
	state.Config.ImageMaxTotalSizeMB = 2
	if err := m.AddLoadedContent("load", loaded); err != nil {
		t.Fatal(err)
	}
	if err := m.AddLoadedContent("second load", loaded); err == nil || !strings.Contains(err.Error(), "2 MiB session limit") {
		t.Fatalf("cumulative limit ignored: %v", err)
	}
	if len(state.Images) != 1 || len(state.ChatHistory) != 2 {
		t.Fatal("rejected load mutated session")
	}
	state.Config.ImageMaxTotalSizeMB = 3
	img.ID = "image-2"
	loaded.Images = []types.ImageAttachment{img}
	if err := m.AddLoadedContent("second load", loaded); err != nil {
		t.Fatalf("raised cumulative limit ignored: %v", err)
	}
}
