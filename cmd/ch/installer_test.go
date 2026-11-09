package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerSecurityNonInteractive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("OPENAI_API_KEY", "")
	// Satisfy the Go prerequisite with the running toolchain's version. The
	// test exercises script invocation and exit handling, not Go compatibility.
	version, err := exec.Command("go", "version").Output()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("INSTALLER_TEST_GO_VERSION", strings.TrimSpace(string(version)))
	t.Setenv("PATH", home+string(os.PathListSeparator)+os.Getenv("PATH"))
	stubs := map[string]string{
		"go":   "#!/bin/sh\nprintf '%s\\n' \"$INSTALLER_TEST_GO_VERSION\"\n",
		"make": "#!/bin/sh\nprintf 'scan:%s\\n' \"$1\"\n[ \"$1\" != \"$STUB_FAIL_TARGET\" ]\n",
		"brew": "#!/bin/sh\nexit 1\n",
	}
	for name, content := range stubs {
		if err := os.WriteFile(filepath.Join(home, name), []byte(content), 0700); err != nil {
			t.Fatal(err)
		}
	}
	script, err := filepath.Abs("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Makefile", "go.mod"} {
		if err := os.WriteFile(filepath.Join(home, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, failTarget := range []string{"", "security-static"} {
		t.Run("fail="+failTarget, func(t *testing.T) {
			t.Setenv("STUB_FAIL_TARGET", failTarget)
			cmd := exec.Command("bash", script, "-k")
			cmd.Dir = home
			cmd.Stdin = strings.NewReader("")
			out, err := cmd.CombinedOutput()
			if (err != nil) != (failTarget != "") {
				t.Fatalf("unexpected exit: %v\n%s", err, out)
			}
			for _, target := range []string{"security-static", "security-secrets", "security-secrets-working", "security-vuln"} {
				if !strings.Contains(string(out), "scan:"+target+"\n") {
					t.Fatalf("missing scan %s:\n%s", target, out)
				}
			}
			if !strings.Contains(string(out), "SECURITY SUMMARY") {
				t.Fatalf("missing summary:\n%s", out)
			}
		})
	}
	// Piped installer content must still reject local-only maintenance flags.
	content, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"-k", "-b", "-r", "--refresh-deps", "-R", "--refresh-all", "-v", "-c", "-d"} {
		t.Run("piped="+flag, func(t *testing.T) {
			cmd := exec.Command("bash", "-s", "--", flag)
			cmd.Dir = home
			cmd.Stdin = strings.NewReader(string(content))
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "only available when running locally") || strings.Contains(string(out), "scan:") {
				t.Fatalf("piped maintenance flag was not rejected: %v\n%s", err, out)
			}
		})
	}
}

func TestInstallerRefreshFlags(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	content, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	definitions := strings.TrimSuffix(string(content), "main \"$@\"\n")
	if definitions == string(content) {
		t.Fatal("installer entry point not found")
	}

	for _, path := range []string{".git", "cmd/ch"} {
		if err := os.MkdirAll(filepath.Join(home, path), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"go.mod", "Makefile", "cmd/ch/main.go"} {
		if err := os.WriteFile(filepath.Join(home, path), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}

	script := definitions + `
	check_go() { printf '%s\n' 'check-go'; }
	refresh_go_deps() { printf '%s\n' 'go-refresh'; }
	update_cli_tools() { printf '%s\n' 'cli-update'; }
	build_only() { printf '%s\n' 'build-only'; }
	_install_ch_from_repo() { printf '%s\n' 'install-repo'; }
main "$@"
`
	scriptPath := filepath.Join(home, "installer-test.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		args          []string
		want          []string
		wantGoRefresh bool
		wantCLIUpdate bool
		wantWarning   bool
		wantError     bool
	}{
		{name: "Go-only install shorthand", args: []string{"-r"}, want: []string{"WARNING:", "go-refresh", "install-repo"}, wantGoRefresh: true, wantWarning: true},
		{name: "Go-only install long form", args: []string{"--refresh-deps"}, want: []string{"WARNING:", "go-refresh", "install-repo"}, wantGoRefresh: true, wantWarning: true},
		{name: "full refresh before install", args: []string{"-R"}, want: []string{"go-refresh", "cli-update", "install-repo"}, wantGoRefresh: true, wantCLIUpdate: true},
		{name: "full refresh before build", args: []string{"--refresh-all", "-b"}, want: []string{"go-refresh", "cli-update", "build-only"}, wantGoRefresh: true, wantCLIUpdate: true},
		{name: "Go-only refresh before build", args: []string{"-r", "-b"}, want: []string{"WARNING:", "go-refresh", "build-only"}, wantGoRefresh: true, wantWarning: true},
		{name: "refresh modes conflict", args: []string{"-r", "-R"}, want: []string{"Cannot combine -r/--refresh-deps with -R/--refresh-all"}, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("bash", append([]string{scriptPath}, tt.args...)...)
			cmd.Dir = home
			out, err := cmd.CombinedOutput()
			if (err != nil) != tt.wantError {
				t.Fatalf("unexpected exit: %v\n%s", err, out)
			}
			output := string(out)
			lastIndex := -1
			for _, marker := range tt.want {
				index := strings.Index(output, marker)
				if index < 0 {
					t.Fatalf("missing %q in output:\n%s", marker, output)
				}
				if index < lastIndex {
					t.Fatalf("%q appeared out of order:\n%s", marker, output)
				}
				lastIndex = index
			}
			if got := strings.Contains(output, "\x1b[93mWARNING:"); got != tt.wantWarning {
				t.Fatalf("yellow warning = %v, want %v:\n%s", got, tt.wantWarning, output)
			}

			if got := strings.Contains(output, "go-refresh\n"); got != tt.wantGoRefresh {
				t.Fatalf("Go dependency refresh = %v, want %v:\n%s", got, tt.wantGoRefresh, output)
			}
			if got := strings.Contains(output, "cli-update\n"); got != tt.wantCLIUpdate {
				t.Fatalf("CLI tool update = %v, want %v:\n%s", got, tt.wantCLIUpdate, output)
			}
			if tt.name == "refresh modes conflict" && strings.Contains(output, "install-repo") {
				t.Fatalf("conflicting refresh options continued installation:\n%s", output)
			}
		})
	}
}

func TestMacOSBuildDeploymentTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PATH", home+string(os.PathListSeparator)+os.Getenv("PATH"))
	stubs := map[string]string{
		"sw_vers": "#!/bin/sh\nprintf '15.7.4\\n'\n",
		"uname":   "#!/bin/sh\nif [ \"$1\" = -s ]; then printf 'Darwin\\n'; else printf '%s\\n' \"$TEST_ARCH\"; fi\n",
		"brew":    "#!/bin/sh\nprintf '/test/homebrew\\n'\n",
		"go":      "#!/bin/sh\nprintf 'target:%s\\n' \"${MACOSX_DEPLOYMENT_TARGET-unset}\"\n",
	}
	for name, content := range stubs {
		if err := os.WriteFile(filepath.Join(home, name), []byte(content), 0700); err != nil {
			t.Fatal(err)
		}
	}
	content, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	// Load only the function definitions, bypassing installation and prerequisites.
	definitions := strings.TrimSuffix(string(content), "main \"$@\"\n")
	if definitions == string(content) {
		t.Fatal("installer entry point not found")
	}
	for _, arch := range []string{"x86_64", "arm64"} {
		for _, method := range []string{"direct", "make"} {
			for _, target := range []string{"", "15.0"} {
				t.Run(arch+"/"+method+"/target="+target, func(t *testing.T) {
					t.Setenv("TEST_ARCH", arch)
					t.Setenv("MACOSX_DEPLOYMENT_TARGET", target)
					script := definitions + `
detect_os() { printf 'macos\n'; }
check_tesseract_libs() { return 0; }
compute_ldflags() { printf 'test'; }
make() { go; }
execute_build "$1" unused
`
					cmd := exec.Command("bash", "-s", "--", method)
					cmd.Dir = home
					cmd.Stdin = strings.NewReader(script)
					out, err := cmd.CombinedOutput()
					want := target
					if want == "" {
						want = "15.7.4"
					}
					if err != nil || !strings.Contains(string(out), "target:"+want+"\n") {
						t.Fatalf("expected deployment target %s: %v\n%s", want, err, out)
					}
				})
			}
		}
	}

	makefile, err := filepath.Abs("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"", "15.0"} {
		t.Run("Makefile/target="+target, func(t *testing.T) {
			t.Setenv("MACOSX_DEPLOYMENT_TARGET", target)
			cmd := exec.Command("make", "-f", makefile, "-f", "-", "print-target")
			cmd.Dir = home
			cmd.Stdin = strings.NewReader("print-target:\n\t@go\n")
			want := target
			if want == "" {
				// Make's ?= respects even an explicitly empty environment value.
				cmd.Env = []string{}
				for _, entry := range os.Environ() {
					if !strings.HasPrefix(entry, "MACOSX_DEPLOYMENT_TARGET=") {
						cmd.Env = append(cmd.Env, entry)
					}
				}
				want = "15.7.4"
			}
			out, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(out), "target:"+want+"\n") {
				t.Fatalf("expected deployment target %s: %v\n%s", want, err, out)
			}
		})
	}
}
