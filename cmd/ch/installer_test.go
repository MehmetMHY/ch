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

func TestInstallerNodeOptionalDependency(t *testing.T) {
	content, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	definitions := strings.TrimSuffix(string(content), "main \"$@\"\n")
	if definitions == string(content) {
		t.Fatal("installer entry point not found")
	}

	tests := []struct {
		name             string
		initialVersion   string
		installedVersion string
		keepInitial      bool
		wantWarning      bool
	}{
		{name: "installs missing Node 22+", installedVersion: "v22.15.0"},
		{name: "replaces old Node", initialVersion: "v18.20.0", installedVersion: "v22.15.0"},
		{name: "warns when package is still too old", initialVersion: "v18.20.0", installedVersion: "v22.15.0", keepInitial: true, wantWarning: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			binDir := filepath.Join(home, "bin")
			if err := os.MkdirAll(binDir, 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("PATH", binDir)
			t.Setenv("INSTALLER_TEST_BIN", binDir)
			t.Setenv("INSTALLER_TEST_INSTALL_NODE_VERSION", tt.installedVersion)
			logPath := filepath.Join(home, "brew.log")
			t.Setenv("INSTALLER_TEST_BREW_LOG", logPath)
			if tt.keepInitial {
				t.Setenv("INSTALLER_TEST_KEEP_NODE", "true")
			} else {
				t.Setenv("INSTALLER_TEST_KEEP_NODE", "false")
			}

			for _, name := range []string{"fzf", "yt-dlp", "tesseract"} {
				stub := "#!/bin/sh\nexit 0\n"
				if name == "yt-dlp" {
					stub = "#!/bin/sh\nprintf '%s\\n' '--js-runtimes' '--remote-components'\n"
				}
				if err := os.WriteFile(filepath.Join(binDir, name), []byte(stub), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if tt.initialVersion != "" {
				writeNode := "#!/bin/sh\nprintf '%s\\n' '" + tt.initialVersion + "'\n"
				if err := os.WriteFile(filepath.Join(binDir, "node"), []byte(writeNode), 0700); err != nil {
					t.Fatal(err)
				}
			}
			brew := `#!/bin/sh
printf 'brew %s\n' "$*" >> "$INSTALLER_TEST_BREW_LOG"
if [ "$1" = "install" ] && [ "$2" = "node" ] && [ "$INSTALLER_TEST_KEEP_NODE" != "true" ]; then
    /bin/cat > "$INSTALLER_TEST_BIN/node" <<'NODE'
#!/bin/sh
printf '%s\n' "$INSTALLER_TEST_INSTALL_NODE_VERSION"
NODE
    /bin/chmod +x "$INSTALLER_TEST_BIN/node"
fi
`
			if err := os.WriteFile(filepath.Join(binDir, "brew"), []byte(brew), 0700); err != nil {
				t.Fatal(err)
			}

			script := definitions + `
detect_os() { printf 'macos\n'; }
install_dependencies
`
			cmd := exec.Command("/bin/bash", "-s")
			cmd.Stdin = strings.NewReader(script)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("installer failed: %v\n%s", err, out)
			}
			output := string(out)
			brewOutput, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(brewOutput), "brew install node\n") {
				t.Fatalf("installer did not install the Node package:\n%s", brewOutput)
			}
			if got := strings.Contains(output, "Node.js 22+ is required"); got != tt.wantWarning {
				t.Fatalf("Node compatibility warning = %v, want %v:\n%s", got, tt.wantWarning, output)
			}
		})
	}
}

func TestInstallerNodePackageNames(t *testing.T) {
	content, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	definitions := strings.TrimSuffix(string(content), "main \"$@\"\n")
	if definitions == string(content) {
		t.Fatal("installer entry point not found")
	}
	tests := []struct {
		name       string
		os         string
		manager    string
		installLog string
		updateLog  string
	}{
		{name: "Linux apt", os: "linux", manager: "apt-get", installLog: "apt-get install -y nodejs", updateLog: "apt-get install --only-upgrade -y nodejs"},
		{name: "Termux pkg", os: "android", manager: "pkg", installLog: "pkg install -y nodejs", updateLog: "pkg upgrade -y nodejs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			binDir := filepath.Join(home, "bin")
			if err := os.MkdirAll(binDir, 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("PATH", binDir)
			t.Setenv("INSTALLER_TEST_BIN", binDir)
			t.Setenv("INSTALLER_TEST_NODE_VERSION", "v22.15.0")
			logPath := filepath.Join(home, "packages.log")
			t.Setenv("INSTALLER_TEST_PACKAGE_LOG", logPath)

			for _, name := range []string{"fzf", "tesseract"} {
				if err := os.WriteFile(filepath.Join(binDir, name), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(binDir, "yt-dlp"), []byte("#!/bin/sh\nprintf '%s\\n' '--js-runtimes' '--remote-components'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			manager := `#!/bin/sh
printf '%s %s\n' "$INSTALLER_TEST_MANAGER" "$*" >> "$INSTALLER_TEST_PACKAGE_LOG"
case "$*" in
    *"install -y nodejs"*)
        /bin/cat > "$INSTALLER_TEST_BIN/node" <<'NODE'
#!/bin/sh
printf '%s\n' "$INSTALLER_TEST_NODE_VERSION"
NODE
        /bin/chmod +x "$INSTALLER_TEST_BIN/node"
        ;;
esac
`
			t.Setenv("INSTALLER_TEST_MANAGER", tt.manager)
			managerPath := filepath.Join(binDir, tt.manager)
			if err := os.WriteFile(managerPath, []byte(manager), 0700); err != nil {
				t.Fatal(err)
			}
			if tt.os == "linux" {
				if err := os.WriteFile(filepath.Join(binDir, "sudo"), []byte("#!/bin/sh\nexec \"$@\"\n"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(binDir, "id"), []byte("#!/bin/sh\nprintf '1000\\n'\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}

			script := definitions + "\ndetect_os() { printf '%s\\n' '" + tt.os + "'; }\ninstall_dependencies\nupdate_cli_tools\n"
			cmd := exec.Command("/bin/bash", "-s")
			cmd.Stdin = strings.NewReader(script)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("installer failed: %v\n%s", err, out)
			}
			log, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{tt.installLog, tt.updateLog} {
				if !strings.Contains(string(log), want) {
					t.Fatalf("missing package operation %q:\n%s\ninstaller output:\n%s", want, log, out)
				}
			}
		})
	}
}

func TestInstallerWarnsForUnsupportedYtDlp(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	if err := os.WriteFile(filepath.Join(binDir, "yt-dlp"), []byte("#!/bin/sh\nprintf 'old yt-dlp help\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	definitions := strings.TrimSuffix(string(content), "main \"$@\"\n")
	if definitions == string(content) {
		t.Fatal("installer entry point not found")
	}
	script := definitions + "\nwarn_yt_dlp_ejs_support\n"
	cmd := exec.Command("/bin/bash", "-s")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("installer warning check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "does not advertise the JavaScript runtime/EJS options") || !strings.Contains(string(out), "./install.sh -R") {
		t.Fatalf("missing actionable yt-dlp warning:\n%s", out)
	}
}

func TestInstallerRefreshesNode(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PATH", binDir)
	if err := os.WriteFile(filepath.Join(binDir, "node"), []byte("#!/bin/sh\nprintf 'v22.15.0\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	brewLog := filepath.Join(home, "brew.log")
	if err := os.WriteFile(filepath.Join(binDir, "brew"), []byte("#!/bin/sh\nprintf 'brew %s\\n' \"$*\" >> \""+brewLog+"\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	definitions := strings.TrimSuffix(string(content), "main \"$@\"\n")
	if definitions == string(content) {
		t.Fatal("installer entry point not found")
	}
	script := definitions + `
detect_os() { printf 'macos\n'; }
update_cli_tools
`
	cmd := exec.Command("/bin/bash", "-s")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("installer update failed: %v\n%s", err, out)
	}
	log, err := os.ReadFile(brewLog)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "brew upgrade node\n") {
		t.Fatalf("full refresh did not upgrade Node:\n%s", log)
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
