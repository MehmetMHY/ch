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
