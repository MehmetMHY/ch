package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildTestBinary(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "ch-test")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return binPath
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

func TestCLIUtilityAndExportFlags(t *testing.T) {
	binPath := buildTestBinary(t)

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
	binPath := buildTestBinary(t)

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
