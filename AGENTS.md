# AGENTS.md

Guidance for future agents working in this repository.

## Project Overview

`ch` is a Go CLI for interacting with AI providers from the terminal. It supports direct prompts, interactive chat, provider/model switching, file loading, web scraping/search, shell session capture, chat export, session continuation, and local Ollama usage.

Primary entry points:

- `cmd/ch/main.go` - CLI flag parsing, direct mode, interactive command dispatch.
- `internal/config/config.go` - default config, config file loading, environment overrides.
- `internal/platform/platform.go` - provider client initialization, model listing, streaming/non-streaming requests.
- `internal/chat/chat.go` - chat history, sessions, export logic, backtracking.
- `internal/ui/ui.go` - terminal helpers, file loading, scraping, web search, clipboard, fzf flows.
- `pkg/types/types.go` - shared config/state/platform types.
- `install.sh` - install/build/test/version maintenance script.
- `README.md` - user-facing feature and usage documentation.

## Safety Rules

- Do not touch the user's real `~/.ch` during tests or manual CLI checks.
- For any test or command that can load config, save sessions, clear temp files, or create history, set `HOME` and `USERPROFILE` to a temp directory.
- Never run `ch --clear`, uninstall commands, or installer uninstall paths against the real home directory.
- Do not run commands that require real API keys unless explicitly asked. Prefer tests that unset provider keys and use temp homes.
- The CLI can call paid third-party APIs. Avoid live provider calls in tests.
- The installer may install system packages. Do not run install flows casually; inspect or test targeted helper behavior instead.

Safe CLI test pattern:

```bash
tmp_home=$(mktemp -d)
HOME="$tmp_home" USERPROFILE="$tmp_home" env -u OPENAI_API_KEY ./bin/ch -l README.md
```

## Build And Test

Use Go 1.26.0 or newer. The module declares `go 1.26.0`.

Common checks:

```bash
go test ./...
go test -count=1 ./...
make build
make test
```

`make build` writes `./bin/ch`, which is ignored by git.

Before committing or handing off, prefer:

```bash
gofmt -w <changed-go-files>
go test -count=1 ./...
make build
```

## Config Behavior

Config file path: `~/.ch/config.json`.

Environment overrides:

- `CH_DEFAULT_PLATFORM` overrides the default/current platform.
- `CH_DEFAULT_MODEL` overrides default/current model.

Provider keys:

- `OPENAI_API_KEY` for default OpenAI AI requests.
- `BRAVE_API_KEY` for web search.
- Other provider keys are defined in `internal/config/config.go`.

Boolean config fields require presence tracking because false is a meaningful value. `types.Config.ExplicitBoolFields` is intentionally non-JSON and is populated by `loadConfigFromFile`. Preserve this behavior when adding new boolean config fields.

If adding a boolean config option:

- Add the field in `pkg/types/types.go`.
- Add its JSON key to the explicit boolean tracking list in `internal/config/config.go` if false must be user-configurable.
- Add/adjust tests in `internal/config/config_test.go`.
- Update README config options.

## CLI Flag Flow

Be careful with the order in `cmd/ch/main.go`.

Important current behavior:

- Print-only `-l`, `-s`, and `-w` must not initialize an AI provider or require `OPENAI_API_KEY`.
- `-l/-s/-w` with an additional prompt should initialize the selected provider and send loaded context plus prompt to the model.
- `-e` and `--export` with a prompt should send the prompt first, then export code blocks from the response.
- `-e` and `--export` without a prompt export code blocks from existing chat history.
- `-d` is a string flag and requires an argument; do not document it as optional unless the parser is changed.
- `-c` requires `enable_session_save=true`.
- `-a`, `-hs`, and `--history` require `save_all_sessions=true`.

When changing flags, update all of these together:

- `cmd/ch/main.go` flag registration and control flow.
- `internal/ui/ui.go` `ShowHelp` output.
- `README.md` usage examples.
- Regression tests under `cmd/ch/` or relevant package.

## Tests

Prefer unit tests that avoid network and real user state.

Patterns already used:

- `internal/config/config_test.go` uses `t.TempDir()` plus `t.Setenv("HOME", tempHome)` and `t.Setenv("USERPROFILE", tempHome)`.
- `cmd/ch/main_test.go` builds a temporary test binary and runs it with temp `HOME`/`USERPROFILE`, unsetting `OPENAI_API_KEY` where needed.

If a test needs a config file, write it under the test temp home:

```go
tempHome := t.TempDir()
t.Setenv("HOME", tempHome)
t.Setenv("USERPROFILE", tempHome)
chDir := filepath.Join(tempHome, ".ch")
```

Avoid tests that depend on:

- Real API keys.
- Network availability.
- `fzf` being interactive.
- The user's actual shell config or clipboard.
- The user's real `~/.ch`.

## Installer Notes

`install.sh` handles install/build/test/version tasks.

Important expectations:

- Local repository installs should use the current checkout as-is and should not run `git pull` automatically.
- `--safe-uninstall` prompts first, but it still removes `~/.ch` including config/history/sessions/temp files.
- `--uninstall` removes immediately without confirmation.
- Optional API key status checks should stay aligned with providers documented in README and configured in `internal/config/config.go`.
- Be cautious changing dependency installation logic because it invokes package managers and may require sudo.

## Documentation Expectations

README claims should match code behavior exactly. Check especially:

- Default model/platform.
- Required vs optional API keys.
- Session save/history requirements.
- Supported flags and aliases.
- Installer side effects.
- Uninstall data deletion.
- Provider names and environment variables.
- AWS Bedrock region count/list.

When a behavior is intentionally limited, document the prerequisite rather than implying it works by default.

## Git And Ignore Notes

The `.gitignore` contains a broad `ch` pattern for local binaries. This can unintentionally ignore files under `cmd/ch/` if they are newly added. There are explicit unignore rules for `cmd/ch/` and `cmd/ch/main_test.go`; add additional unignore rules if adding new files under `cmd/ch/`.

Ignored/generated files include:

- `bin/`
- local `ch` binary
- temp/export/history artifacts
- `.DS_Store`
- `*.orig`

## Code Style

- Keep changes small and direct.
- Use `gofmt` on all edited Go files.
- Prefer package-level tests near the changed behavior.
- Avoid adding abstraction unless it removes real duplication or clarifies a tricky path.
- Do not add compatibility shims unless there is persisted data, shipped behavior, or an explicit requirement.
- Keep README examples runnable and consistent with actual flags.

## Quick Triage Checklist

Before final response:

- Did any command use the real `~/.ch`? If yes, disclose it and verify no destructive operation occurred.
- Did tests pass with `go test -count=1 ./...`?
- Did `make build` pass if code changed?
- Did changed CLI behavior update `README.md` and `ShowHelp`?
- Did config changes include temp-home tests?
- Did installer docs match installer behavior?
- Are new files visible to git and not hidden by `.gitignore`?
