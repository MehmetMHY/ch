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
	for _, flag := range []string{"-k", "-b", "-r", "-v", "-c", "-d"} {
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
