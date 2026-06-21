# AGENTS.md

Guidance for AI agents working in this repository.

## Project Overview

`ch` is a Go CLI for interacting with AI providers from the terminal. It supports direct prompts, interactive chat, provider/model switching, file loading, URL scraping, web search, shell session capture, chat export, session continuation, piped stdin input, codedump generation, token counting, and OCR-based image text extraction.

Primary entry points:

- `cmd/ch/main.go` - CLI flag parsing, direct mode, interactive command dispatch.
- `internal/config/config.go` - default config, config file loading, environment overrides.
- `internal/config/util.go` - config utility helpers (temp dir, shallow load dir checks).
- `internal/platform/platform.go` - provider client initialization, model listing, streaming/non-streaming requests.
- `internal/chat/chat.go` - chat history, sessions, export logic, backtracking.
- `internal/chat/util.go` - chat utility helpers (hashing, content manipulation).
- `internal/ui/ui.go` - terminal helpers, file loading, scraping, web search, clipboard, fzf flows.
- `internal/ui/util.go` - editor launch helper with fallback.
- `internal/ui/ocr_cgo.go` - Tesseract OCR image-to-text extraction (CGO builds only).
- `internal/ui/ocr_nocgo.go` - OCR stub for non-CGO builds (e.g., Android).
- `pkg/types/types.go` - shared config/state/platform types.
- `install.sh` - install/build/test/version maintenance script.
- `docs/` - static website files (HTML, CSS, JS, assets).
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

Default model: `gpt-5.4-mini`. Default platform: `openai`.

Environment overrides:

- `CH_DEFAULT_PLATFORM` overrides the default/current platform.
- `CH_DEFAULT_MODEL` overrides default/current model.

Provider API key environment variables:

- `OPENAI_API_KEY` for OpenAI (the built-in/default platform).
- `GROQ_API_KEY` for Groq.
- `OPENROUTER_API_KEY` for OpenRouter.
- `DEEP_SEEK_API_KEY` for DeepSeek.
- `ANTHROPIC_API_KEY` for Anthropic.
- `XAI_API_KEY` for xAI (Grok).
- `GEMINI_API_KEY` for Google (Gemini).
- `MISTRAL_API_KEY` for Mistral.
- `TOGETHER_API_KEY` for Together AI.
- `AWS_BEDROCK_API_KEY` for Amazon Bedrock.
- `BRAVE_API_KEY` for web search (Brave Search API).
- Ollama requires no API key (local, uses `http://127.0.0.1:11434/v1`).

Supported platforms (defined in `internal/config/config.go`):

`openai`, `groq`, `openrouter`, `deepseek`, `anthropic`, `xai`, `ollama`, `together`, `google`, `mistral`, `amazon`

Boolean config fields require presence tracking because false is a meaningful value. `types.Config.ExplicitBoolFields` is intentionally non-JSON and is populated by `loadConfigFromFile`. Preserve this behavior when adding new boolean config fields.

Tracked boolean keys (must appear in the explicit list in `config.go`):

`show_search_results`, `mute_notifications`, `enable_session_save`, `save_all_sessions`, `show_thinking`, `ai_name_enable`

If adding a boolean config option:

- Add the field in `pkg/types/types.go`.
- Add its JSON key to the explicit boolean tracking list in `internal/config/config.go` if false must be user-configurable.
- Add/adjust tests in `internal/config/config_test.go`.
- Update README config options.

Notable config fields beyond the basics:

- `shallow_load_dirs` - directories where file loading only includes direct children (depth 1). Has a built-in default list of large/high-level directories.
- `slow_model_patterns` - model name substrings that trigger a loading animation instead of streaming (reasoning models).
- `ai_name_enable`, `ai_name_char_threshold`, `ai_name_count`, `ai_name_timeout_seconds`, `ai_name_prompt` - control AI-generated filename suggestions in the `!e` export flow.

## CLI Flag Flow

Be careful with the order in `cmd/ch/main.go`.

Complete flag reference:

| Flag                 | Alias              | Description                                                                                                       |
| -------------------- | ------------------ | ----------------------------------------------------------------------------------------------------------------- |
| `-h`                 | `--help`           | Show help and exit                                                                                                |
| `-c`                 | `--continue`       | Continue from the latest session (or a specific session file if a valid path is given as the first remaining arg) |
| `--clear`            |                    | Clear all temp files (requires `enable_session_save=true`)                                                        |
| `-a`                 | `-hs`, `--history` | Search and load previous sessions (requires `save_all_sessions=true`)                                             |
| `-n`                 | `--no-history`     | Disable session saving for this run                                                                               |
| `-d dir`             |                    | Generate a codedump file for the given directory (required non-empty argument)                                    |
| `-p [platform]`      |                    | Switch platform (leave empty for interactive fzf selection)                                                       |
| `-m model`           |                    | Specify model to use                                                                                              |
| `-o platform\|model` |                    | Specify platform and model together (pipe-delimited format)                                                       |
| `-l file/url`        |                    | Load and display file content (supports comma/pipe-delimited multiple values)                                     |
| `-w query`           |                    | Web search and print results (supports comma/pipe-delimited multiple queries)                                     |
| `-s url`             |                    | Scrape a URL and print content (supports comma/pipe-delimited multiple URLs)                                      |
| `-e`                 | `--export`         | Export code blocks from the last response                                                                         |
| `-t file`            | `--token file`     | Estimate token count for a file                                                                                   |

Important current behavior:

- Print-only `-l`, `-s`, and `-w` must not initialize an AI provider or require `OPENAI_API_KEY`.
- `-l`, `-s`, and `-w` with an additional prompt should initialize the selected provider and send loaded context plus prompt to the model.
- `-e` and `--export` with a prompt send the prompt first, then export code blocks from the response.
- `-e` and `--export` without a prompt export code blocks from existing chat history.
- `-d` is a string flag and requires a non-empty directory path argument to trigger; do not document it as optional unless the parser is changed.
- `-c` requires `enable_session_save=true`. If the first remaining arg is a valid file path, it loads that file as the session instead of the latest.
- `-a`, `-hs`, and `--history` require `save_all_sessions=true`.
- `-n` and `--no-history` are linked after parsing via `flag.Lookup`.
- `-l`, `-s`, and `-w` all accept comma-separated or pipe-delimited lists to load/scrape/search multiple targets at once.
- Piped stdin (`cat file | ch "query"`) is supported. Piped content is combined with positional arguments before being sent to the model.

When changing flags, update all of these together:

- `cmd/ch/main.go` flag registration and control flow.
- `internal/ui/ui.go` `ShowHelp` output.
- `README.md` usage examples.
- Regression tests under `cmd/ch/` or relevant package.

## Interactive Mode Commands

These are the default key bindings (configurable in `~/.ch/config.json`):

| Command         | Description                                                           |
| --------------- | --------------------------------------------------------------------- |
| `!q`            | Exit                                                                  |
| `!h`            | Show interactive help (fzf picker)                                    |
| `!c`            | Clear chat history                                                    |
| `!m [model]`    | Switch model (or fzf pick if no argument)                             |
| `!p [platform]` | Switch platform (or fzf pick if no argument)                          |
| `!o`            | Pick from all models across all platforms                             |
| `!l [dir]`      | Load files from current or specified directory                        |
| `!d`            | Generate codedump and load into context                               |
| `!x [cmd]`      | Run a shell command and add output to context                         |
| `!!x [cmd]`     | Run a shell command silently (output not saved to history)            |
| `!` (prefix)    | Run a shell command and add output to context                         |
| `!!`            | Record an interactive shell session                                   |
| `!t [buff]`     | Open preferred editor for multi-line input                            |
| `!e [file]`     | Export chat to a file                                                 |
| `!b`            | Backtrack (remove last exchange)                                      |
| `!w [query]`    | Web search (or fzf pick from history if no argument)                  |
| `!s [url]`      | Scrape URL (or fzf pick from history if no argument)                  |
| `!y`            | Copy a response to clipboard (fzf picker)                             |
| `cc`            | Quick-copy the latest response to clipboard                           |
| `!a [filter]`   | Search and restore a previous session                                 |
| `\`             | Enter multi-line mode (trailing `\` on a line continues to next line) |

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
- Do not use em-dashes (—) in any code, comments, strings, or documentation.

## Keeping AGENTS.md Up To Date

After completing any non-trivial task, check whether AGENTS.md still reflects reality. You must update it when:

- The user says something like "good job", "well done", "looks good", "ship it", or any similar sign-off that signals the work is done.
- A git commit is made.
- Any of the following change: flags, config fields, platforms, env vars, interactive commands, file structure, or test patterns.

Do not wait to be asked. Treat AGENTS.md as a living document and keep it current as part of finishing a task.

## Unit Tests

Always run the unit tests after any code change to confirm nothing is broken:

```bash
go test -count=1 ./...
```

If you add a new feature, fix a bug, or change behavior, add or update tests in the relevant `_test.go` file alongside the change. Follow the existing patterns:

- Use `t.TempDir()` and `t.Setenv("HOME", ...)` / `t.Setenv("USERPROFILE", ...)` to isolate from the real `~/.ch`.
- Unset provider API keys (`t.Setenv("OPENAI_API_KEY", "")`) in tests that exercise the CLI binary.
- Do not write tests that require network, real API keys, or interactive fzf.

## Quick Triage Checklist

Before final response:

- Did any command use the real `~/.ch`? If yes, disclose it and verify no destructive operation occurred.
- Did tests pass with `go test -count=1 ./...`?
- Did `make build` pass if code changed?
- Did changed CLI behavior update `README.md` and `ShowHelp`?
- Did config changes include temp-home tests?
- Did installer docs match installer behavior?
- Are new files visible to git and not hidden by `.gitignore`?
- Does `-n / --no-history` skip session saving as expected?
- Are new platforms listed in both `internal/config/config.go` and `README.md`?
