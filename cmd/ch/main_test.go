package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MehmetMHY/ch/internal/chat"
	chconfig "github.com/MehmetMHY/ch/internal/config"
	"github.com/MehmetMHY/ch/internal/platform"
	"github.com/MehmetMHY/ch/internal/ui"
	"github.com/MehmetMHY/ch/pkg/types"
)

// testBinPath is the path to the ch binary compiled once for the whole test
// package. Building it a single time in TestMain avoids the multi-second cost
// of rebuilding it for every exec-based test.
var testBinPath string

func TestNativeImageCLIAndContinuation(t *testing.T) {
	home := t.TempDir()
	var img bytes.Buffer
	if err := png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "screenshot.png")
	if err := os.WriteFile(path, img.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	requests := make(chan []byte, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests <- body
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"},\"index\":0}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	if err := os.MkdirAll(filepath.Join(home, ".ch"), 0700); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]interface{}{"current_platform": "image-test", "default_model": "vision", "models_dev_enabled": false, "enable_session_save": true, "platforms": map[string]interface{}{"image-test": map[string]interface{}{"name": "image-test", "base_url": server.URL + "/v1", "env_name": "IMAGE_CLI_TEST_KEY", "image_input": true}}}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ch", "config.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(testBinPath, args...)
		cmd.Env = filteredEnv(os.Environ(), map[string]string{"HOME": home, "USERPROFILE": home, "CH_DEFAULT_PLATFORM": "image-test", "CH_DEFAULT_MODEL": "vision", "IMAGE_CLI_TEST_KEY": "test-key"}, "OPENAI_API_KEY")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	if out := run("-l", path, "describe the screenshot"); !strings.Contains(out, "hello") {
		t.Fatalf("native image query failed: %s", out)
	}
	first := <-requests
	want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(img.Bytes())
	if !bytes.Contains(first, []byte(want)) || !bytes.Contains(first, []byte("describe the screenshot")) {
		t.Fatalf("image/prompt missing: %s", first)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if out := run("-c", "another question"); !strings.Contains(out, "hello") {
		t.Fatalf("continuation failed: %s", out)
	}
	if body := <-requests; !bytes.Contains(body, []byte(want)) {
		t.Fatal("continuation depended on original file")
	}
	run("-c", "!i off")
	if err := os.WriteFile(path, img.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	// The shared CLI binary is built without CGO: warn, retain the image, and send text only.
	if out := run("-c", "-l", path, "read the screenshot text"); !strings.Contains(out, "OCR unavailable") {
		t.Fatalf("paused load did not report unavailable OCR: %s", out)
	}
	if body := <-requests; bytes.Contains(body, []byte("image_url")) || bytes.Contains(body, []byte("OCR unavailable")) {
		t.Fatalf("paused load sent pixels or diagnostics to the model: %s", body)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	run("-c", "continue without images")
	if body := <-requests; bytes.Contains(body, []byte("image_url")) || bytes.Contains(body, []byte("base64")) {
		t.Fatal("persisted !i off still sends images")
	}
	sessionPath := filepath.Join(home, ".ch", "tmp", "ch_session_latest.json")
	before, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, img.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	run("-n", "-l", path, "a temporary image")
	if body := <-requests; !bytes.Contains(body, []byte(want)) {
		t.Fatal("no-history query did not send image")
	}
	after, err := os.ReadFile(sessionPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("-n image query changed saved session")
	}
}

func TestImageControlsAndDirectFileLoad(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("OPENAI_API_KEY", "")
	state := chconfig.InitializeAppState()
	state.Config.ModelsDevEnabled = false
	cm := chat.NewManager(state)
	terminal := ui.NewTerminal(state.Config)
	var img bytes.Buffer
	if err := png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "shot.png")
	if err := os.WriteFile(path, img.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	loadOutput := captureStdout(t, func() { handleFileLoad(cm, terminal, state, path) })
	if strings.Contains(loadOutput, "image input:") || strings.Contains(loadOutput, "unverified") {
		t.Fatalf("unexpected capability chatter: %s", loadOutput)
	}
	if len(state.Images) != 1 || len(cm.GetMessages()[1].Images) != 1 {
		t.Fatal("!l file did not attach image")
	}
	handleImageControl("off", terminal, state)
	if len(cm.GetMessages()[1].Images) != 0 {
		t.Fatal("!i off did not exclude image")
	}
	handleImageControl("on", terminal, state)
	if len(cm.GetMessages()[1].Images) != 1 {
		t.Fatal("!i on did not restore image")
	}
	// A fake fzf selects the first row, without requiring an interactive tty.
	if err := os.WriteFile(filepath.Join(home, "fzf"), []byte("#!/bin/sh\nIFS= read -r line\nprintf '%s\\n' \"$line\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", home+string(os.PathListSeparator)+os.Getenv("PATH"))
	handleImageControl("", terminal, state)
	if !state.Images[0].Disabled || len(cm.GetMessages()[1].Images) != 0 {
		t.Fatal("image picker did not disable selected image")
	}
	handleImageControl("", terminal, state)
	if state.Images[0].Disabled || len(cm.GetMessages()[1].Images) != 1 {
		t.Fatal("image picker did not re-enable selected image")
	}
	if err := os.WriteFile(filepath.Join(home, "fzf"), []byte("#!/bin/sh\nexit 130\n"), 0700); err != nil {
		t.Fatal(err)
	}
	handleImageControl("", terminal, state)
	if state.Images[0].Disabled {
		t.Fatal("cancelled picker changed image settings")
	}
	out := captureStdout(t, func() {
		if err := handleShowState(cm, terminal, state, true); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "\033[96mimages:\033[0m \033[92m1/1 active") || strings.Contains(out, "sending on") || strings.Contains(out, "image tokens not estimated") {
		t.Fatalf("state lacks image usage: %s", out)
	}
	// Pausing existing images must not retroactively extract text.
	originalContext := state.ChatHistory[1].Context
	handleImageControl("off", terminal, state)
	if state.ChatHistory[1].Context != originalContext || strings.Contains(originalContext, "OCR") {
		t.Fatal("pausing modified the original image context")
	}
	t.Setenv("PATH", home) // No Tesseract, even in CGO-enabled package tests.
	out = captureStdout(t, func() { handleFileLoad(cm, terminal, state, path) })
	if !strings.Contains(out, "OCR unavailable for shot.png") || len(state.Images) != 2 || len(cm.GetMessages()[2].Images) != 0 {
		t.Fatalf("paused interactive load did not retain image and warn: %s", out)
	}
	handleImageControl("on", terminal, state)
	if len(cm.GetMessages()[2].Images) != 1 {
		t.Fatal("paused image could not be resumed after OCR failure")
	}
}

func TestImageStatusFormatting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("OPENAI_API_KEY", "")
	state := chconfig.InitializeAppState()
	state.Config.ModelsDevEnabled = false
	state.Images = []types.ImageAttachment{{Name: "shot.png", Data: make([]byte, 150215)}}
	cm := chat.NewManager(state)
	terminal := ui.NewTerminal(state.Config)
	for _, piped := range []bool{false, true} {
		state.Config.IsPipedOutput = piped
		for _, off := range []bool{false, true} {
			state.ImagesOff = off
			active, size := 1, "0.15"
			if off {
				active, size = 0, "0"
			}
			want := fmt.Sprintf("\033[96mimages:\033[0m \033[92m%d/1 active (%s MB)\033[0m\n", active, size)
			if piped {
				want = fmt.Sprintf("images: %d/1 active (%s MB)\n", active, size)
			}
			out := captureStdout(t, func() {
				if err := handleShowState(cm, terminal, state, true); err != nil {
					t.Fatal(err)
				}
			})
			if !strings.Contains(out, want) || strings.Contains(out, ", sending ") || strings.Contains(out, "image tokens not estimated") {
				t.Fatalf("piped=%v off=%v, unexpected image status: %q", piped, off, out)
			}
			if piped && strings.Contains(out, "\033[") {
				t.Fatalf("piped output contains colors: %q", out)
			}
		}
	}
	state.Config.IsPipedOutput = true
	state.ImagesOff = false
	for _, tc := range []struct {
		bytes int
		want  string
	}{
		{1, "0.01"},
		{4999, "0.01"},
		{10000, "0.01"},
		{100000, "0.1"},
		{1000000, "1"},
		{1234000, "1.23"},
	} {
		state.Images[0].Data = make([]byte, tc.bytes)
		out := captureStdout(t, func() {
			if err := handleShowState(cm, terminal, state, true); err != nil {
				t.Fatal(err)
			}
		})
		if !strings.Contains(out, "images: 1/1 active ("+tc.want+" MB)\n") {
			t.Fatalf("size %d bytes: unexpected output %q", tc.bytes, out)
		}
	}
	state.Config.IsPipedOutput = false
	state.ImagesOff = true
	loaded := types.LoadedContent{Images: state.Images}
	out := captureStdout(t, func() { printImageLoadStatus(loaded, terminal, state) })
	if !strings.Contains(out, "\033[91mimage sending is off (set !i to resume)\033[0m\n") || strings.Contains(out, "image input:") {
		t.Fatalf("unexpected paused image notice: %q", out)
	}
	state.Config.IsPipedOutput = true
	out = captureStdout(t, func() { printImageLoadStatus(loaded, terminal, state) })
	if out != "" {
		t.Fatalf("piped output contains image notices: %q", out)
	}
}

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "ch-test-bin")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir for test binary: %v\n", err)
		os.Exit(1)
	}

	binPath := filepath.Join(tmpDir, "ch-test")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	// These CLI tests exercise flag flow only (-l, -e, -t) and never touch the
	// OCR path, so build without CGO to keep the one-time compile fast.
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build failed: %v\n%s", err, out)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}
	testBinPath = binPath

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

func runWithTempHome(t *testing.T, binPath string, args ...string) string {
	t.Helper()
	home := t.TempDir()
	cmd := exec.Command(binPath, args...)
	cmd.Env = filteredEnv(os.Environ(), map[string]string{
		"HOME":                home,
		"USERPROFILE":         home,
		"CH_DEFAULT_PLATFORM": "openai",
		"CH_DEFAULT_MODEL":    "gpt-5.4-mini",
	}, "OPENAI_API_KEY")
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func runWithTempHomeStdin(t *testing.T, binPath string, stdin string, args ...string) string {
	t.Helper()
	home := t.TempDir()
	cmd := exec.Command(binPath, args...)
	cmd.Env = filteredEnv(os.Environ(), map[string]string{
		"HOME":                home,
		"USERPROFILE":         home,
		"CH_DEFAULT_PLATFORM": "openai",
		"CH_DEFAULT_MODEL":    "gpt-5.4-mini",
	}, "OPENAI_API_KEY")
	cmd.Stdin = strings.NewReader(stdin)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func filteredEnv(env []string, overrides map[string]string, unset ...string) []string {
	remove := map[string]bool{}
	for _, key := range unset {
		remove[key] = true
	}
	for key := range overrides {
		remove[key] = true
	}

	var result []string
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if ok && remove[key] {
			continue
		}
		result = append(result, entry)
	}
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}
	return result
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	os.Stdout = writer

	fn()

	writer.Close()
	os.Stdout = original
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	return string(out)
}

func TestHandleShowStateSessionFileRow(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	state := &types.AppState{
		Config: &types.Config{
			CurrentPlatform:   "openai",
			CurrentModel:      "gpt-5.4-mini",
			SystemPrompt:      "S",
			EnableSessionSave: true,
			SaveAllSessions:   true,
			IsPipedOutput:     true,
		},
		Messages: []types.ChatMessage{{Role: "system", Content: "S"}},
		ChatHistory: []types.ChatHistory{
			{User: "S"},
		},
		SessionStartTime: 1783568531,
	}
	chatManager := chat.NewManager(state)
	terminal := ui.NewTerminal(state.Config)

	out := captureStdout(t, func() {
		if err := handleShowState(chatManager, terminal, state, false); err != nil {
			t.Fatalf("handleShowState() error: %v", err)
		}
	})
	if !strings.Contains(out, "file: ch_session_1783568531.json") {
		t.Fatalf("expected state output to include session file, got:\n%s", out)
	}

	out = captureStdout(t, func() {
		if err := handleShowState(chatManager, terminal, state, true); err != nil {
			t.Fatalf("handleShowState() with noHistory error: %v", err)
		}
	})
	if strings.Contains(out, "file:") {
		t.Fatalf("expected noHistory state output to hide session file, got:\n%s", out)
	}
}

func TestProcessDirectQueryRemovesPendingMessageOnProviderError(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	type requestMessage struct {
		Role    string      `json:"role"`
		Content interface{} `json:"content"`
	}
	var requestMessages [][]requestMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []requestMessage `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		requestMessages = append(requestMessages, payload.Messages)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request","type":"invalid_request_error"}}`))
	}))
	defer server.Close()

	cfg := chconfig.DefaultConfig()
	cfg.CurrentPlatform = "ollama"
	cfg.CurrentModel = "test-model"
	cfg.SystemPrompt = "system prompt"
	cfg.SlowModelPatterns = []string{".*"}
	cfg.IsPipedOutput = true
	cfg.Platforms["ollama"] = types.Platform{
		Name:    "ollama",
		BaseURL: types.BaseURLValue{Single: server.URL + "/v1"},
	}
	state := &types.AppState{
		Config:      cfg,
		Messages:    []types.ChatMessage{{Role: "system", Content: cfg.SystemPrompt}},
		ChatHistory: []types.ChatHistory{{User: cfg.SystemPrompt}},
	}
	chatManager := chat.NewManager(state)
	platformManager := platform.NewManager(cfg)
	if err := platformManager.Initialize(); err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}
	terminal := ui.NewTerminal(cfg)

	if err := processDirectQuery("first prompt", chatManager, platformManager, terminal, state, false, false); err == nil {
		t.Fatalf("expected first provider error")
	}
	if len(state.Messages) != 1 {
		t.Fatalf("expected failed prompt to be removed, got %v", state.Messages)
	}

	if err := processDirectQuery("second prompt", chatManager, platformManager, terminal, state, false, false); err == nil {
		t.Fatalf("expected second provider error")
	}
	if len(state.Messages) != 1 {
		t.Fatalf("expected second failed prompt to be removed, got %v", state.Messages)
	}
	if len(requestMessages) != 2 {
		t.Fatalf("expected 2 provider requests, got %d", len(requestMessages))
	}

	secondRequest := requestMessages[1]
	if len(secondRequest) != 2 {
		t.Fatalf("expected second request to contain system and current user only, got %v", secondRequest)
	}
	if secondRequest[1].Role != "user" || secondRequest[1].Content != "second prompt" {
		t.Fatalf("expected second request to exclude failed first prompt, got %v", secondRequest)
	}
}

func TestCLIUtilityAndExportFlags(t *testing.T) {
	binPath := testBinPath

	loadFile := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(loadFile, []byte("hello from load utility"), 0644); err != nil {
		t.Fatalf("failed to write load fixture: %v", err)
	}

	out := runWithTempHome(t, binPath, "-l", loadFile)
	if !strings.Contains(out, "hello from load utility") {
		t.Fatalf("-l should print file content without OpenAI key, got:\n%s", out)
	}
	if strings.Contains(out, "OPENAI_API_KEY") {
		t.Fatalf("-l without prompt should not initialize OpenAI, got:\n%s", out)
	}

	out = runWithTempHome(t, binPath, "--export")
	if strings.Contains(out, "flag provided but not defined") {
		t.Fatalf("--export should be registered, got:\n%s", out)
	}

	// Codedump flag aliases and the build flag must all be registered.
	// These do not reach the interactive fzf flow because the directory is
	// invalid, so they are safe to run without a tty.
	for _, flagArg := range []string{"--dump", "-b", "--build", "-y", "--yes"} {
		out = runWithTempHome(t, binPath, flagArg)
		if strings.Contains(out, "flag provided but not defined") {
			t.Fatalf("%s should be registered, got:\n%s", flagArg, out)
		}
	}

	// The removed --name and --pick flags must no longer be registered.
	for _, flagArg := range []string{"--name", "--pick"} {
		out = runWithTempHome(t, binPath, flagArg)
		if !strings.Contains(out, "flag provided but not defined") {
			t.Fatalf("%s should no longer be registered, got:\n%s", flagArg, out)
		}
	}

	out = runWithTempHome(t, binPath, "-e", "write a code block")
	if strings.Contains(out, "no chat history available") {
		t.Fatalf("-e with a prompt should send the prompt before exporting, got:\n%s", out)
	}
	if !strings.Contains(out, "OPENAI_API_KEY") {
		t.Fatalf("-e with a prompt should reach platform initialization in this test, got:\n%s", out)
	}
}

// TestVersionFlag verifies -v and --version print the version and exit
// before any provider/config init. The test binary is built without ldflags,
// so it reports the dev fallback (version=dev).
func TestVersionFlag(t *testing.T) {
	binPath := testBinPath
	home := t.TempDir()

	for _, flagArg := range []string{"-v", "--version"} {
		cmd := exec.Command(binPath, flagArg)
		cmd.Env = filteredEnv(os.Environ(), map[string]string{
			"HOME":        home,
			"USERPROFILE": home,
		}, "OPENAI_API_KEY")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s should exit cleanly, got %v:\n%s", flagArg, err, out)
		}
		line := strings.TrimSpace(string(out))
		if line != "ch dev" {
			t.Fatalf("%s should print 'ch dev' when built without ldflags, got:\n%s", flagArg, out)
		}
		if strings.Contains(line, "OPENAI_API_KEY") {
			t.Fatalf("%s must not initialize a provider, got:\n%s", flagArg, out)
		}
	}
}

// TestCodeDumpYesFlag verifies -y skips the interactive exclude-picker so the
// codedump runs non-interactively and writes a file without a tty.
func TestCodeDumpYesFlag(t *testing.T) {
	binPath := testBinPath

	home := t.TempDir()
	// Work directory with a couple of source files; the codedump is written
	// into it so it does not pollute the test process CWD.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("hello\n"), 0600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	// -d -y on a directory with files must complete without a tty and print
	// an auto-named codedump filename (ch_cd*.txt). If the exclude-picker ran,
	// fzf would hang on stdin and this would time out.
	out := runWithPreparedHomeDir(t, binPath, home, dir, "-y", "-d", dir)
	trimmed := strings.TrimSpace(out)
	if !strings.HasPrefix(trimmed, "ch_cd") || !strings.HasSuffix(trimmed, ".txt") {
		t.Fatalf("-y -d should print an auto-named codedump filename, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, trimmed)); err != nil {
		t.Fatalf("codedump file %s should exist: %v", trimmed, err)
	}

	// -b -y with a name should write the named file without any fzf.
	named := "my_dump.txt"
	out = runWithPreparedHomeDir(t, binPath, home, dir, "-y", "-b", dir, named)
	if strings.TrimSpace(out) != named {
		t.Fatalf("-y -b <name> should print %q, got:\n%s", named, out)
	}
	if _, err := os.Stat(filepath.Join(dir, named)); err != nil {
		t.Fatalf("named codedump %s should exist: %v", named, err)
	}

	// -b -y with no name should auto-name without opening the filename fzf.
	out = runWithPreparedHomeDir(t, binPath, home, dir, "-y", "-b", dir)
	trimmed = strings.TrimSpace(out)
	if !strings.HasPrefix(trimmed, "ch_cd") || !strings.HasSuffix(trimmed, ".txt") {
		t.Fatalf("-y -b (no name) should auto-name, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, trimmed)); err != nil {
		t.Fatalf("auto-named codedump %s should exist: %v", trimmed, err)
	}
}

// extractTokenCount parses the "tokens: <n>" line from `-t` output and
// returns the numeric count. It works for both the colored and piped forms.
func extractTokenCount(out string) int {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		idx := strings.Index(line, "tokens:")
		if idx < 0 {
			continue
		}
		rest := strings.TrimSpace(line[idx+len("tokens:"):])
		n, err := strconv.Atoi(rest)
		if err != nil {
			return -1
		}
		return n
	}
	return -1
}

func TestTokenCountFlag(t *testing.T) {
	binPath := testBinPath

	tokenFile := filepath.Join(t.TempDir(), "tokens.txt")
	if err := os.WriteFile(tokenFile, []byte("hello from token fixture"), 0644); err != nil {
		t.Fatalf("failed to write token fixture: %v", err)
	}

	out := runWithTempHome(t, binPath, "-t", tokenFile)
	if !strings.Contains(out, "tokens:") {
		t.Fatalf("-t with a file path should print a token count, got:\n%s", out)
	}
	if !strings.Contains(out, tokenFile) {
		t.Fatalf("-t with a file path should report that file as the source, got:\n%s", out)
	}
	if n := extractTokenCount(out); n != 4 {
		t.Fatalf("-t file token count for known fixture: expected 4, got %d (out=%q)", n, out)
	}

	out = runWithTempHomeStdin(t, binPath, "hello from piped stdin", "-t")
	if !strings.Contains(out, "tokens:") {
		t.Fatalf("-t with piped stdin and no file should print a token count, got:\n%s", out)
	}
	if !strings.Contains(out, "stdin") {
		t.Fatalf("-t with piped stdin and no file should report stdin as the source, got:\n%s", out)
	}
	if n := extractTokenCount(out); n != 5 {
		t.Fatalf("-t stdin token count for known input: expected 5, got %d (out=%q)", n, out)
	}

	out = runWithTempHome(t, binPath, "-t")
	if !strings.Contains(out, "no file specified and no piped input available") {
		t.Fatalf("-t with no file and no piped input should fail with a clear error, got:\n%s", out)
	}

	out = runWithTempHome(t, binPath, "-t", filepath.Join(t.TempDir(), "does-not-exist.txt"))
	if !strings.Contains(out, "file does not exist") {
		t.Fatalf("-t with a missing file should fail with a clear error, got:\n%s", out)
	}
}

// writeChConfig writes a minimal config.json under the given home directory.
func writeChConfig(t *testing.T, home string, config map[string]interface{}) {
	t.Helper()
	chDir := filepath.Join(home, ".ch")
	if err := os.MkdirAll(chDir, 0700); err != nil {
		t.Fatalf("failed to create .ch dir: %v", err)
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chDir, "config.json"), data, 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
}

// writeSessionFile writes a session JSON file under ~/.ch/tmp/ in the given home.
func writeSessionFile(t *testing.T, home string, filename string, session types.SessionFile) {
	t.Helper()
	tmpDir := filepath.Join(home, ".ch", "tmp")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("failed to marshal session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, filename), data, 0600); err != nil {
		t.Fatalf("failed to write session file: %v", err)
	}
}

// runWithPreparedHome runs the ch binary with a pre-configured temp home,
// allowing files (config, sessions) to be written before execution.
func runWithPreparedHome(t *testing.T, binPath string, home string, args ...string) string {
	t.Helper()
	return runWithPreparedHomeDir(t, binPath, home, "", args...)
}

func runWithPreparedHomeDir(t *testing.T, binPath string, home string, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = filteredEnv(os.Environ(), map[string]string{
		"HOME":                home,
		"USERPROFILE":         home,
		"CH_DEFAULT_PLATFORM": "openai",
		"CH_DEFAULT_MODEL":    "gpt-5.4-mini",
	}, "OPENAI_API_KEY")
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func TestPrintSessionHistory(t *testing.T) {
	history := []types.ChatHistory{
		{User: "system prompt"},
		{User: "Loaded: current.json", Context: "full file contents"},
		{User: "Loaded: legacy.json"},
		{User: "Loaded from ./src: main.go"},
		{User: "Codedump loaded", Context: "full codedump"},
		{User: "pending user prompt"},
		{User: "Loaded: is this a status?", Bot: "No, this is a question."},
		{User: "summarize this", Context: "prompt plus file", Bot: "Summary"},
		{Bot: "standalone response"},
	}
	out := captureStdout(t, func() {
		printSessionHistory(history, "system prompt")
	})
	want := "\033[93mLoaded: current.json\033[0m\n" +
		"\033[93mLoaded: legacy.json\033[0m\n" +
		"\033[93mLoaded from ./src: main.go\033[0m\n" +
		"\033[93mCodedump loaded\033[0m\n" +
		"\033[94muser:\033[0m pending user prompt\n" +
		"\033[94muser:\033[0m Loaded: is this a status?\n" +
		"\033[92mNo, this is a question.\033[0m\n" +
		"\033[94muser:\033[0m summarize this\n" +
		"\033[92mSummary\033[0m\n" +
		"\033[92mstandalone response\033[0m\n"
	if out != want {
		t.Fatalf("unexpected session history output:\ngot: %q\nwant: %q", out, want)
	}
}

func TestSessionFlagsLoadSummary(t *testing.T) {
	for _, flag := range []string{"-f", "--fetch", "-c", "--continue"} {
		t.Run(flag, func(t *testing.T) {
			home := t.TempDir()
			writeChConfig(t, home, map[string]interface{}{
				"enable_session_save": true,
			})
			session := types.SessionFile{
				Timestamp: 1783804936,
				Platform:  "openai",
				Model:     "gpt-5.4-mini",
				ChatHistory: []types.ChatHistory{
					{User: "Loaded: ch_session_1753435166.json"},
					{User: "Loaded: notes.txt", Context: "notes content"},
					{User: "hello", Bot: "Hello!"},
				},
			}
			filename := "ch_session_1783804936.json"
			writeSessionFile(t, home, filename, session)
			path := filepath.Join(home, ".ch", "tmp", filename)
			out := runWithPreparedHome(t, testBinPath, home, flag, path)
			for _, summary := range []string{"Loaded: ch_session_1753435166.json", "Loaded: notes.txt"} {
				if !strings.Contains(out, "\033[93m"+summary+"\033[0m\n") {
					t.Fatalf("expected standalone load summary %q, got:\n%s", summary, out)
				}
			}
			if !strings.Contains(out, "\033[94muser:\033[0m hello\n") {
				t.Fatalf("expected user prompt to retain its label, got:\n%s", out)
			}
		})
	}
}

func TestFetchFlagBareName(t *testing.T) {
	binPath := testBinPath
	home := t.TempDir()

	writeChConfig(t, home, map[string]interface{}{
		"enable_session_save": true,
	})

	session := types.SessionFile{
		Timestamp: 1783572299,
		Platform:  "openai",
		Model:     "gpt-5.4-mini",
		ChatHistory: []types.ChatHistory{
			{User: "hello from saved session"},
			{User: "what is 2+2", Bot: "4"},
		},
	}
	writeSessionFile(t, home, "ch_session_1783572299.json", session)

	out := runWithPreparedHome(t, binPath, home, "-f", "ch_session_1783572299.json")

	if !strings.Contains(out, "hello from saved session") {
		t.Fatalf("-f with bare name should print session history, got:\n%s", out)
	}
	if !strings.Contains(out, "what is 2+2") {
		t.Fatalf("-f with bare name should print user message, got:\n%s", out)
	}
	if !strings.Contains(out, "4") {
		t.Fatalf("-f with bare name should print bot response, got:\n%s", out)
	}
	if !strings.Contains(out, "ch_session_1783572299.json") {
		t.Fatalf("-f should print session filename, got:\n%s", out)
	}
	if !strings.Contains(out, "OPENAI_API_KEY") {
		t.Fatalf("-f should fall through to platform init, got:\n%s", out)
	}
}

func TestFetchFlagFullPath(t *testing.T) {
	binPath := testBinPath
	home := t.TempDir()

	writeChConfig(t, home, map[string]interface{}{
		"enable_session_save": true,
	})

	session := types.SessionFile{
		Timestamp: 1783572299,
		Platform:  "openai",
		Model:     "gpt-5.4-mini",
		ChatHistory: []types.ChatHistory{
			{User: "full path session content"},
		},
	}
	fullPath := filepath.Join(home, ".ch", "tmp", "ch_session_1783572299.json")
	writeSessionFile(t, home, "ch_session_1783572299.json", session)

	out := runWithPreparedHome(t, binPath, home, "-f", fullPath)

	if !strings.Contains(out, "full path session content") {
		t.Fatalf("-f with full path should print session history, got:\n%s", out)
	}
	if !strings.Contains(out, "OPENAI_API_KEY") {
		t.Fatalf("-f should fall through to platform init, got:\n%s", out)
	}
}

func TestFetchFlagBareNameFromCurrentDirectory(t *testing.T) {
	binPath := testBinPath
	home := t.TempDir()
	workDir := t.TempDir()

	writeChConfig(t, home, map[string]interface{}{
		"enable_session_save": true,
	})

	session := types.SessionFile{
		Timestamp: 1784674737,
		Platform:  "openai",
		Model:     "gpt-5.4-mini",
		ChatHistory: []types.ChatHistory{
			{User: "current directory session content"},
		},
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "index_ch_dump_1784674737_5.json"), data, 0600); err != nil {
		t.Fatalf("failed to write current directory session file: %v", err)
	}

	out := runWithPreparedHomeDir(t, binPath, home, workDir, "-f", "index_ch_dump_1784674737_5.json")

	if !strings.Contains(out, "current directory session content") {
		t.Fatalf("-f with bare current-directory file should print session history, got:\n%s", out)
	}
	if !strings.Contains(out, "index_ch_dump_1784674737_5.json") {
		t.Fatalf("-f should print current-directory session filename, got:\n%s", out)
	}
	if !strings.Contains(out, "OPENAI_API_KEY") {
		t.Fatalf("-f should fall through to platform init, got:\n%s", out)
	}
}

func TestFetchFlagFileNotFound(t *testing.T) {
	binPath := testBinPath
	home := t.TempDir()

	writeChConfig(t, home, map[string]interface{}{
		"enable_session_save": true,
	})

	out := runWithPreparedHome(t, binPath, home, "-f", "ch_session_nonexistent.json")

	if !strings.Contains(out, "session file not found: ch_session_nonexistent.json") {
		t.Fatalf("-f with missing file should error clearly, got:\n%s", out)
	}
}

func TestFetchFlagLiteralPathNotFound(t *testing.T) {
	binPath := testBinPath
	home := t.TempDir()

	writeChConfig(t, home, map[string]interface{}{
		"enable_session_save": true,
	})

	missingPath := filepath.Join(home, "missing", "session.json")

	out := runWithPreparedHome(t, binPath, home, "-f", missingPath)

	if !strings.Contains(out, "session file not found") {
		t.Fatalf("-f with missing literal path should error clearly, got:\n%s", out)
	}
}

func TestFetchFlagRequiresEnableSessionSave(t *testing.T) {
	binPath := testBinPath
	home := t.TempDir()

	// Config without enable_session_save (defaults to false).
	writeChConfig(t, home, map[string]interface{}{
		"default_model": "gpt-5.4-mini",
	})

	session := types.SessionFile{
		Timestamp: 1783572299,
		Platform:  "openai",
		Model:     "gpt-5.4-mini",
		ChatHistory: []types.ChatHistory{
			{User: "should not load"},
		},
	}
	writeSessionFile(t, home, "ch_session_1783572299.json", session)

	out := runWithPreparedHome(t, binPath, home, "-f", "ch_session_1783572299.json")

	if !strings.Contains(out, "session save feature is disabled in config") {
		t.Fatalf("-f with file arg should require enable_session_save, got:\n%s", out)
	}
	if strings.Contains(out, "should not load") {
		t.Fatalf("-f should not load session when feature is disabled, got:\n%s", out)
	}
}

func TestFetchFlagNoArgRequiresSaveAllSessions(t *testing.T) {
	binPath := testBinPath
	home := t.TempDir()

	// enable_session_save=true but save_all_sessions=false.
	writeChConfig(t, home, map[string]interface{}{
		"enable_session_save": true,
	})

	out := runWithPreparedHome(t, binPath, home, "-f")

	if !strings.Contains(out, "session search requires save_all_sessions to be enabled in config") {
		t.Fatalf("-f with no arg should require save_all_sessions, got:\n%s", out)
	}
}

func TestFetchFlagNoArgNoSessions(t *testing.T) {
	binPath := testBinPath
	home := t.TempDir()

	writeChConfig(t, home, map[string]interface{}{
		"enable_session_save": true,
		"save_all_sessions":   true,
	})

	out := runWithPreparedHome(t, binPath, home, "-f")

	if !strings.Contains(out, "no sessions found") {
		t.Fatalf("-f with no sessions should report no sessions found, got:\n%s", out)
	}
}

func TestFetchFlagFollowUpPrompt(t *testing.T) {
	binPath := testBinPath
	home := t.TempDir()

	writeChConfig(t, home, map[string]interface{}{
		"enable_session_save": true,
	})

	session := types.SessionFile{
		Timestamp: 1783572299,
		Platform:  "openai",
		Model:     "gpt-5.4-mini",
		ChatHistory: []types.ChatHistory{
			{User: "prior exchange"},
		},
	}
	writeSessionFile(t, home, "ch_session_1783572299.json", session)

	out := runWithPreparedHome(t, binPath, home, "-f", "ch_session_1783572299.json", "follow up query")

	if !strings.Contains(out, "prior exchange") {
		t.Fatalf("-f with prompt should still print loaded session history, got:\n%s", out)
	}
	// Should reach platform init (and fail without API key) rather than
	// entering interactive mode.
	if !strings.Contains(out, "OPENAI_API_KEY") {
		t.Fatalf("-f with prompt should fall through to direct query / platform init, got:\n%s", out)
	}
}

// TestReasoningEffortFlagRegistered verifies -r and --reasoning-effort are
// registered as valid flags.
func TestReasoningEffortFlagRegistered(t *testing.T) {
	binPath := testBinPath

	for _, flagArg := range []string{"-r", "--reasoning-effort"} {
		out := runWithTempHome(t, binPath, flagArg)
		if strings.Contains(out, "flag provided but not defined") {
			t.Fatalf("%s should be registered, got:\n%s", flagArg, out)
		}
	}
}

// TestReasoningEffortFlagReachesPlatformInit verifies that -r with a prompt
// reaches platform initialization (and fails without an API key), proving
// the flag value is consumed and the prompt is treated as a direct query.
func TestReasoningEffortFlagReachesPlatformInit(t *testing.T) {
	binPath := testBinPath

	out := runWithTempHome(t, binPath, "-r", "medium", "test prompt")
	if strings.Contains(out, "flag provided but not defined") {
		t.Fatalf("-r should be registered, got:\n%s", out)
	}
	// Should reach platform init (and fail without API key).
	if !strings.Contains(out, "OPENAI_API_KEY") {
		t.Fatalf("-r with prompt should reach platform init, got:\n%s", out)
	}
}

// TestStateOutputIncludesReasoning verifies that >state (handleShowState)
// includes a reasoning line.
func TestStateOutputIncludesReasoning(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	state := &types.AppState{
		Config: &types.Config{
			CurrentPlatform: "openai",
			CurrentModel:    "gpt-5.4-mini",
			SystemPrompt:    "S",
			IsPipedOutput:   true,
		},
		Messages:    []types.ChatMessage{{Role: "system", Content: "S"}},
		ChatHistory: []types.ChatHistory{{User: "S"}},
	}
	chatManager := chat.NewManager(state)
	terminal := ui.NewTerminal(state.Config)

	out := captureStdout(t, func() {
		_ = handleShowState(chatManager, terminal, state, true)
	})

	if !strings.Contains(out, "reasoning:") {
		t.Fatalf("expected state output to include reasoning line, got:\n%s", out)
	}
	if !strings.Contains(out, "default") {
		t.Fatalf("expected 'default' in reasoning state, got:\n%s", out)
	}
}

// TestStateOutputIncludesReasoningEffort verifies that >state shows the
// active reasoning effort when set.
func TestStateOutputIncludesReasoningEffort(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	state := &types.AppState{
		Config: &types.Config{
			CurrentPlatform: "openai",
			CurrentModel:    "gpt-5.4-mini",
			SystemPrompt:    "S",
			IsPipedOutput:   true,
		},
		Messages:        []types.ChatMessage{{Role: "system", Content: "S"}},
		ChatHistory:     []types.ChatHistory{{User: "S"}},
		ReasoningEffort: "high",
	}
	chatManager := chat.NewManager(state)
	terminal := ui.NewTerminal(state.Config)

	out := captureStdout(t, func() {
		_ = handleShowState(chatManager, terminal, state, true)
	})

	if !strings.Contains(out, "reasoning: high") {
		t.Fatalf("expected 'reasoning: high' in state output, got:\n%s", out)
	}
}

// TestCompressHistoryConfigDefault verifies the default config has
// compress_history set to !z and compress_min_tokens set to 4000.
func TestCompressHistoryConfigDefault(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)
	t.Setenv("CH_DEFAULT_PLATFORM", "")
	t.Setenv("CH_DEFAULT_MODEL", "")

	state := chconfig.InitializeAppState()
	if state.Config.CompressHistory != "!z" {
		t.Errorf("expected CompressHistory=!z, got %q", state.Config.CompressHistory)
	}
	if state.Config.CompressMinTokens != 4000 {
		t.Errorf("expected CompressMinTokens=4000, got %d", state.Config.CompressMinTokens)
	}
}

// TestCompressHistoryLowTokens verifies that CompressHistory refuses
// when the active token count is below the minimum threshold.
func TestCompressHistoryLowTokens(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	cfg := &types.Config{
		SystemPrompt:      "S",
		CurrentPlatform:   "openai",
		CurrentModel:      "gpt-4o",
		CompressMinTokens: 4000,
	}
	state := &types.AppState{
		Config: cfg,
		Messages: []types.ChatMessage{
			{Role: "system", Content: "S"},
			{Role: "user", Content: "Q1"},
			{Role: "assistant", Content: "A1"},
		},
		ChatHistory: []types.ChatHistory{
			{User: "S"},
			{User: "Q1", Bot: "A1"},
		},
	}
	m := chat.NewManager(state)
	terminal := ui.NewTerminal(cfg)

	err := m.CompressHistory(terminal)
	if err == nil {
		t.Fatal("expected error for low-token conversation, got nil")
	}
	if !strings.Contains(err.Error(), "compression skipped") {
		t.Errorf("expected 'compression skipped' in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "minimum is 4000") {
		t.Errorf("expected 'minimum is 4000' in error, got %q", err.Error())
	}
}

// TestCompressHistoryNoPlatformManager verifies that CompressHistory
// returns an error when the platform manager is not set but the
// token count exceeds the threshold.
func TestCompressHistoryNoPlatformManager(t *testing.T) {
	cfg := &types.Config{
		SystemPrompt:      "S",
		CurrentPlatform:   "openai",
		CurrentModel:      "gpt-4o",
		CompressMinTokens: 4000,
	}
	// Build a large enough message body to exceed the 4000 token threshold.
	// ~4 chars/token, so 20000 chars is ~5000 tokens.
	bigContent := strings.Repeat("This is a test message with enough content to exceed the token threshold. ", 300)
	state := &types.AppState{
		Config:   cfg,
		Messages: []types.ChatMessage{{Role: "system", Content: "S"}, {Role: "user", Content: bigContent}},
		ChatHistory: []types.ChatHistory{
			{User: "S"},
			{User: bigContent, Bot: "response"},
		},
	}
	m := chat.NewManager(state)
	terminal := ui.NewTerminal(cfg)

	err := m.CompressHistory(terminal)
	if err == nil {
		t.Fatal("expected error when platform manager is nil, got nil")
	}
	if !strings.Contains(err.Error(), "platform manager not initialized") {
		t.Errorf("expected 'platform manager not initialized', got %q", err.Error())
	}
}
