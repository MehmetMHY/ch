package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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

	out = runWithTempHome(t, binPath, "-e", "write a code block")
	if strings.Contains(out, "no chat history available") {
		t.Fatalf("-e with a prompt should send the prompt before exporting, got:\n%s", out)
	}
	if !strings.Contains(out, "OPENAI_API_KEY") {
		t.Fatalf("-e with a prompt should reach platform initialization in this test, got:\n%s", out)
	}
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

	out = runWithTempHomeStdin(t, binPath, "hello from piped stdin", "-t")
	if !strings.Contains(out, "tokens:") {
		t.Fatalf("-t with piped stdin and no file should print a token count, got:\n%s", out)
	}
	if !strings.Contains(out, "stdin") {
		t.Fatalf("-t with piped stdin and no file should report stdin as the source, got:\n%s", out)
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
