//go:build darwin || linux

package main

import (
	"bufio"
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
	"time"

	"github.com/MehmetMHY/ch/pkg/types"
	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

// These tests use the actual CLI in a PTY and interpret its ANSI output. Merely
// checking Readline's returned string cannot detect cursor/erase corruption.
func TestEditorHistoryTerminalRendering(t *testing.T) {
	for _, size := range []struct{ width, rows int }{{40, 80}, {100, 80}, {40, 12}} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.rows), func(t *testing.T) {
			width, rows := size.width, size.rows
			home := t.TempDir()
			const prompt = "History of atomic theory\n\tMain article: History of atomic theory\n\nThe word atom is derived from the ancient Greek word Atomos, which means uncuttable. Modern atomic theory is based on scientific evidence.\n\nPhilosophy\n\tMain article: Atomism\n\nPreserve café, 日本語, blank lines, and indentation.\nEnd of saved prompt."
			editor := filepath.Join(home, "editor")
			if err := os.WriteFile(editor, []byte("#!/bin/sh\nprintf '%s' \"$CH_TEST_EDITOR_CONTENT\" > \"$1\"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			requests := make(chan []types.ChatMessage, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload struct {
					Messages []types.ChatMessage `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Errorf("decode request: %v", err)
				}
				first := len(requests) == 0
				requests <- payload.Messages
				if first {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprint(w, `{"error":{"message":"Unrecognized request argument supplied: reasoning_effort","type":"invalid_request_error"}}`)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"RETRY_SUCCEEDED\"},\"index\":0}]}\n\ndata: [DONE]\n\n")
			}))
			defer server.Close()
			if err := os.MkdirAll(filepath.Join(home, ".ch"), 0700); err != nil {
				t.Fatal(err)
			}
			cfg := map[string]any{
				"current_platform": "ollama", "default_model": "test-model", "models_dev_enabled": false,
				"platforms": map[string]any{"ollama": map[string]any{"name": "ollama", "base_url": server.URL + "/v1"}},
			}
			data, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, ".ch", "config.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binary, "-n")
			cmd.Env = filteredEnv(os.Environ(), map[string]string{
				"HOME": home, "USERPROFILE": home, "EDITOR": editor, "TERM": "xterm-256color",
				"CH_DEFAULT_PLATFORM": "ollama", "CH_DEFAULT_MODEL": "test-model", "CH_TEST_EDITOR_CONTENT": prompt,
				"CH_TEST_TERMINAL_HELPER": "1",
			}, "OPENAI_API_KEY")
			ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(width), Rows: uint16(rows)})
			if err != nil {
				t.Fatal(err)
			}
			defer ptmx.Close()
			waited := false
			defer func() {
				_ = cmd.Process.Kill()
				if !waited {
					_ = cmd.Wait()
				}
			}()
			screen := vt10x.New(vt10x.WithSize(width, rows), vt10x.WithWriter(ptmx))
			changed := make(chan struct{}, 1)
			readerDone := make(chan struct{})
			go func() {
				defer close(readerDone)
				reader := bufio.NewReader(ptmx)
				for {
					if err := screen.Parse(reader); err != nil {
						return
					}
					select {
					case changed <- struct{}{}:
					default:
					}
				}
			}()
			// Close the PTY and join the reader before leaving the test.
			defer func() { _ = cmd.Process.Kill(); _ = ptmx.Close(); <-readerDone }()
			view := func() string { return normalizeTerminalScreen(screen.String()) }
			waitFor := func(description string, match func(string) bool) string {
				t.Helper()
				timer := time.NewTimer(5 * time.Second)
				defer timer.Stop()
				for {
					got := view()
					if match(got) {
						return got
					}
					select {
					case <-changed:
					case <-timer.C:
						t.Fatalf("timed out waiting for %s; terminal:\n%s", description, got)
					}
				}
			}
			send := func(keys string) {
				t.Helper()
				if _, err := io.WriteString(ptmx, keys); err != nil {
					t.Fatal(err)
				}
			}
			waitFor("initial prompt", func(s string) bool { return strings.HasSuffix(s, "user:") })
			send("!t\r")
			waitFor("provider error", func(s string) bool {
				return strings.Contains(strings.ReplaceAll(s, "\n", ""), "reasoning_effort") && strings.HasSuffix(s, "user:")
			})
			send("!r default\r")
			waitFor("corrected setting", func(s string) bool {
				return strings.Contains(s, "reasoning: default") && strings.HasSuffix(s, "user:")
			})
			promptView := func(cols int) string {
				ref := vt10x.New(vt10x.WithSize(cols, rows))
				_, _ = io.WriteString(ref, "user: "+strings.ReplaceAll(strings.ReplaceAll(prompt, "\t", "    "), "\n", "\r\n"))
				return normalizeTerminalScreen(ref.String())
			}
			wantPrompt := promptView(width)
			send("\x1b[A\x1b[A")
			before := waitFor("complete recalled prompt", func(s string) bool { return strings.HasSuffix(s, wantPrompt) })
			send("X")
			waitFor("editing without overwriting or duplicating earlier rows", func(s string) bool { return s == before+"X" })
			send("\x7f")
			waitFor("backspace redraw", func(s string) bool { return s == before })
			send("\x1b[D\x7fX")
			wantEdited := strings.TrimSuffix(before, "prompt.") + "prompX."
			waitFor("editing before the last character", func(s string) bool { return s == wantEdited })
			send("\x7ft\x1b[C")
			waitFor("restored text and cursor", func(s string) bool { return s == before })
			send("\x1b[B")
			waitFor("Down to setting", func(s string) bool { return strings.HasSuffix(s, "user: !r default") })
			send("\x1b[A")
			waitFor("Up restores the same screen", func(s string) bool { return s == before })
			for _, cols := range []int{70, width} {
				screen.Resize(cols, rows)
				if err := pty.Setsize(ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)}); err != nil {
					t.Fatal(err)
				}
				want := promptView(cols)
				resized := waitFor("recalled prompt after resize", func(s string) bool { return strings.HasSuffix(s, want) })
				send("X")
				waitFor("editing after resize", func(s string) bool { return s == resized+"X" })
				send("\x7f")
				waitFor("backspace after resize", func(s string) bool { return s == resized })
			}
			send("\x03")
			waitFor("clean prompt after cancellation", func(s string) bool { return strings.HasSuffix(s, "user:") })
			send("\x1b[A\x1b[A\r")
			waitFor("successful retry", func(s string) bool {
				return strings.Contains(s, "RETRY_SUCCEEDED") && strings.HasSuffix(s, "user:")
			})
			if len(requests) != 2 {
				t.Fatalf("wanted two requests, got %d", len(requests))
			}
			for range 2 {
				messages := <-requests
				if len(messages) != 2 || messages[1].Content != prompt {
					t.Fatalf("prompt was changed or duplicated: %+v", messages)
				}
			}
			send("\x04")
			exited := make(chan error, 1)
			go func() { exited <- cmd.Wait() }()
			waited = true
			select {
			case err := <-exited:
				if err != nil {
					t.Fatalf("CLI exited with an error (including any detected race): %v\n%s", err, view())
				}
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
				<-exited
				t.Fatal("CLI did not exit after Ctrl+D")
			}
		})
	}
}

func normalizeTerminalScreen(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \x00")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}
