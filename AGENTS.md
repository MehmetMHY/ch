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
- `internal/ui/youtube.go` - SRT subtitle compaction for YouTube scrapes (strips cue numbers, milliseconds, blank lines; preserves `>>` speaker markers).
- `internal/ui/ocr_cgo.go` - Tesseract OCR image-to-text extraction (CGO builds only).
- `internal/ui/ocr_nocgo.go` - OCR stub for non-CGO builds (e.g., Android).
- `pkg/types/types.go` - shared config/state/platform types.
- `install.sh` - install/build/test/version maintenance script.
- `fresh.sh` - self-contained script that tests the real `curl | bash` installer on a clean Ubuntu image via Docker (embedded Dockerfile, no build context).
- `docs/` - static website files (HTML, CSS, JS, assets).
- `docs/run.py` - local static website server with browser launch and Ctrl+C/Ctrl+D cleanup handling.
- `README.md` - user-facing feature and usage documentation.

## Safety Rules

- Do not touch the user's real `~/.ch` during tests or manual CLI checks.
- For any test or command that can load config, save sessions, clear temp files, or create history, set `HOME` and `USERPROFILE` to a temp directory.
- Never run `ch --clear`, uninstall commands, or installer uninstall paths against the real home directory.
- Do not run commands that require real API keys unless explicitly asked. Prefer tests that unset provider keys and use temp homes.
- The CLI can call paid third-party APIs. Avoid live provider calls in tests.
- The installer may install system packages. Do not run install flows casually against the host; inspect or test targeted helper behavior instead. To exercise the full `curl | bash` installer end-to-end safely, run `./fresh.sh`, which runs it inside a throwaway Docker container rather than touching the host or the real `~/.ch`. It requires Docker and network access and always fetches the installer from `main`.

Safe CLI test pattern:

```bash
tmp_home=$(mktemp -d)
HOME="$tmp_home" USERPROFILE="$tmp_home" env -u OPENAI_API_KEY ./bin/ch -l README.md
```

## Build And Test

Use Go 1.26.5 or newer for local builds and vulnerability checks. The module declares `go 1.26.0`, but `govulncheck` reports reachable standard-library vulnerabilities when using Go 1.26.0.

Common checks:

```bash
go test ./...
go test -count=1 ./...
make security-static
make security-secrets
make security-vuln
make security
make verify
make build
make test
```

`make build` runs `security-static` before compiling and writes `./bin/ch`, which is ignored by git.

`make verify` runs the full portable gate (`fmt-check`, `vet`, `go test -count=1 ./...`, then `make security`). It is provider-agnostic by design: any CI, self-hosted runner, server-side git hook, or manual pre-merge check can run this one command, so the quality gate is never tied to a specific CI vendor.

Security checks:

- `make security-static` runs the locally installed `gosec ./...` scanner.
- `make security-secrets` runs the locally installed `gitleaks git --no-banner --redact .` scanner.
- `make security-secrets-staged` runs `gitleaks git --no-banner --redact --staged .` for pre-commit checks.
- `make security-secrets-working` runs `gitleaks dir --no-banner --redact .` to catch secrets already present in the current checkout, including rename-only staged changes.
- `make security-vuln` runs `go mod verify` and govulncheck. It prefers a locally installed `govulncheck` binary (faster, no network resolution) and falls back to `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` when the binary is not present.
- `make security` runs gosec, committed-history Gitleaks, working-tree Gitleaks, and vulnerability checks.
- `make install-hooks` configures this checkout to use `.githooks/` (both `pre-commit` and `pre-push`), ensures the hooks are executable, and prints a verification command. Git hooks are local config and must be installed once per clone.
- `.githooks/pre-commit` runs formatting checks, unit tests, `gosec`, and staged and working-tree Gitleaks scanning before commits.
- `.githooks/pre-push` runs `make security-vuln` (govulncheck) before pushing. Vulnerability scanning is the slowest local check and its findings do not change between local commits, so it runs on push instead of on every commit to keep commits fast.
- `./install.sh --dev-setup` (or `-d`) is the one-shot maintainer setup: it installs the dev security tools (`gosec`, `gitleaks`, `govulncheck`) and activates the git hooks via `make install-hooks`. Local-repo only.
- `gitleaks git .` scans committed history, not untracked working-tree files. To test a new secret before commit, stage it and run `make security-secrets-staged`. To scan the current checkout, run `make security-secrets-working`.

Before committing or handing off, prefer:

```bash
gofmt -w <changed-go-files>
go test -count=1 ./...
make security
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
| `-f [file]`          | `--fetch`          | Fetch a session into interactive mode by bare name, path, or fzf pick (no arg)                                    |
| `-n`                 | `--no-history`     | Disable session saving for this run                                                                               |
| `-d dir`             |                    | Generate a codedump file for the given directory (required non-empty argument)                                    |
| `-p [platform]`      |                    | Switch platform (leave empty for interactive fzf selection)                                                       |
| `-m model`           |                    | Specify model to use                                                                                              |
| `-o platform\|model` |                    | Specify platform and model together (pipe-delimited format)                                                       |
| `-l file/url`        |                    | Load and display file content (supports comma/pipe-delimited multiple values)                                     |
| `-w query`           |                    | Web search and print results (supports comma/pipe-delimited multiple queries)                                     |
| `-s url`             |                    | Scrape a URL and print content (supports comma/pipe-delimited multiple URLs)                                      |
| `-e`                 | `--export`         | Export code blocks from the last response                                                                         |
| `-t [file]`          | `--token [file]`   | Estimate token count for a file, or for piped stdin if no file is given                                           |

Important current behavior:

- Print-only `-l`, `-s`, and `-w` must not initialize an AI provider or require `OPENAI_API_KEY`.
- `-l`, `-s`, and `-w` with an additional prompt should initialize the selected provider and send loaded context plus prompt to the model.
- `-e` and `--export` with a prompt send the prompt first, then export code blocks from the response.
- `-e` and `--export` without a prompt export code blocks from existing chat history.
- `-d` is a string flag and requires a non-empty directory path argument to trigger; do not document it as optional unless the parser is changed.
- `-c` requires `enable_session_save=true`. If the first remaining arg is a valid file path, it loads that file as the session instead of the latest.
- `-a`, `-hs`, and `--history` require `save_all_sessions=true`.
- `-f`/`--fetch` loads a session and falls through to interactive mode (or direct query if a prompt follows). With a bare name (no slashes) it first checks the current directory, then falls back to `~/.ch/tmp/`; with a path containing slashes it treats it as a literal path. The file-load branch requires `enable_session_save=true`; the no-arg fzf branch requires `save_all_sessions=true`. If the file does not exist, it errors with `session file not found: <arg>`. Every `-f` load calls `ForkSessionOnNextSave` so the original session file is preserved when `save_all_sessions=true` and the session changes.
- `-n` and `--no-history` are linked after parsing via `flag.Lookup`.
- `-l`, `-s`, and `-w` all accept comma-separated or pipe-delimited lists to load/scrape/search multiple targets at once.
- Piped stdin (`cat file | ch "query"`) is supported. Piped content is combined with positional arguments before being sent to the model.
- `-t`/`--token` is a string flag, but `cmd/ch/main.go` pre-processes `os.Args` before `flag.Parse()` so a bare trailing `-t`/`--token` (no value) does not trigger Go's "flag needs an argument" error; it is rewritten to an explicit empty value (`-t=`) instead. Whether the flag was passed at all (even empty) is tracked separately via `flag.Visit`, since an empty string is also the flag's zero value.
- `-t`/`--token` with an explicit file path always reads that file, even if stdin is also piped. With no file path, it falls back to piped stdin content (reported as `stdin` in the output); if neither is available, it errors with `no file specified and no piped input available` instead of hanging.

When changing flags, update all of these together:

- `cmd/ch/main.go` flag registration and control flow.
- `internal/ui/ui.go` `ShowHelp` output.
- `README.md` usage examples.
- Regression tests under `cmd/ch/` or relevant package.

## Interactive Mode Commands

These are the default key bindings (configurable in `~/.ch/config.json`):

| Command         | Description                                                                                                         |
| --------------- | ------------------------------------------------------------------------------------------------------------------- |
| `!q`            | Exit                                                                                                                |
| `!h`            | Show interactive help (fzf picker)                                                                                  |
| `>state`        | Help picker option that prints state, including session filename when session saving is active                      |
| `!c`            | Clear chat history                                                                                                  |
| `!m [model]`    | Switch model (or fzf pick if no argument)                                                                           |
| `!p [platform]` | Switch platform (or fzf pick if no argument)                                                                        |
| `!o`            | Pick from all models across all platforms                                                                           |
| `!l [dir]`      | Load files from current or specified directory                                                                      |
| `!d`            | Generate codedump and load into context                                                                             |
| `!x [cmd]`      | Run a shell command and add output to context                                                                       |
| `!!x [cmd]`     | Run a shell command silently (output not saved to history)                                                          |
| `!` (prefix)    | Run a shell command and add output to context                                                                       |
| `!!`            | Record an interactive shell session                                                                                 |
| `!t [buff]`     | Open preferred editor for multi-line input                                                                          |
| `!e [file]`     | Export chat to a file                                                                                               |
| `!b`            | Backtrack (remove last exchange)                                                                                    |
| `!w [query]`    | Web search (or fzf pick from history if no argument)                                                                |
| `!s [url]`      | Scrape URL (or fzf pick from history if no argument)                                                                |
| `!y`            | Copy a response to clipboard (fzf picker)                                                                           |
| `cc`            | Quick-copy the latest response to clipboard                                                                         |
| `!a [filter]`   | Search and restore a previous session; with `save_all_sessions=true`, new messages fork into a new timestamped file |
| `\`             | Enter multi-line mode (trailing `\` on a line continues to next line)                                               |

## Tests

Prefer unit tests that avoid network and real user state.

Patterns already used:

- `internal/config/config_test.go` uses `t.TempDir()` plus `t.Setenv("HOME", tempHome)` and `t.Setenv("USERPROFILE", tempHome)`.
- `cmd/ch/main_test.go` builds the `ch` binary once in `TestMain` (with `CGO_ENABLED=0`, since the exec-based flag tests never touch the OCR path) and shares it via the package-level `testBinPath`. Tests run it with temp `HOME`/`USERPROFILE`, unsetting `OPENAI_API_KEY` where needed. Do not reintroduce per-test `go build` calls; reuse `testBinPath`.
- `cmd/ch/main_test.go` `runWithTempHomeStdin` runs the test binary with a given string piped in as stdin, for flags like `-t` that read from piped input (see `TestTokenCountFlag`).

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
- Local `./install.sh -b` builds should ensure dev security tools are available because `make build` runs `gosec`; the script installs missing `gosec` via `go install` and handles Gitleaks as a local pre-commit/security dependency.
- `./install.sh --dev-setup` (`-d`) installs the full dev security toolchain (`gosec`, `gitleaks`, `govulncheck`) and activates the git hooks. It is local-repo only and guarded against remote/piped installs like `-b`, `-c`, `-r`, and `-v`. `install_dev_security_tools` now also installs `govulncheck` via `ensure_govulncheck`.
- `./fresh.sh` builds a minimal Ubuntu + Go image (Dockerfile embedded in the script) and runs the real `curl | bash` install from README `main` inside a throwaway container, then reports PASS/FAIL. Use it to verify installer changes end-to-end without touching the host. `ensure_govulncheck` is non-fatal on failure since govulncheck is not needed to build and `make security-vuln` falls back to `go run`.
- `--safe-uninstall` prompts first, but it still removes `~/.ch` including config/history/sessions/temp files.
- `--uninstall` removes immediately without confirmation.
- Optional API key status checks should stay aligned with providers documented in README and configured in `internal/config/config.go`.
- Be cautious changing dependency installation logic because it invokes package managers and may require sudo.

## Website Assets

`docs/assets/logo.png` is the source of truth. Every other image in `docs/assets/` is derived from it. Do not hand-edit the derived files; change `logo.png` and regenerate.

Derived files:

| File                         | Size        | Derivation                                      |
| ---------------------------- | ----------- | ----------------------------------------------- |
| `favicon-light.png`          | 32x32       | full-bleed resize, light theme                  |
| `favicon-light-16x16.png`    | 16x16       | full-bleed resize, light theme                  |
| `favicon-light.ico`          | 16/32/48/64 | full-bleed, multi-size ICO, light               |
| `favicon-dark.png`           | 32x32       | pre-inverted full-bleed resize, dark theme      |
| `favicon-dark-16x16.png`     | 16x16       | pre-inverted full-bleed resize, dark theme      |
| `favicon-dark.ico`           | 16/32/48/64 | pre-inverted, multi-size ICO, dark              |
| `favicon-16x16.png`          | 16x16       | full-bleed resize (legacy, unsuffixed)          |
| `favicon-32x32.png`          | 32x32       | full-bleed resize (legacy, unsuffixed)          |
| `favicon.ico`                | 16/32/48/64 | full-bleed, multi-size ICO (legacy, unsuffixed) |
| `apple-touch-icon.png`       | 180x180     | full-bleed resize                               |
| `android-chrome-192x192.png` | 192x192     | full-bleed resize                               |
| `android-chrome-512x512.png` | 512x512     | full-bleed resize                               |
| `thumbnail.png`              | 1220x650    | OG/social card, see below                       |

Properties of the mark that the pipeline depends on:

- Strokes are pure black (`#000000`), the interior is opaque near-white (`#FEFEFE`), and only the outer rounded corners are transparent.
- The interior being opaque (not transparent) is why the mark stays legible on dark grounds without a backing plate.

Favicon dark-mode behavior:

- The site's dark mode is a JS-driven toggle (`docs/js/main.js` `applyTheme`), not the OS `prefers-color-scheme`. The favicon therefore follows the site theme via JS, not a CSS media query.
- Favicons are PNG/ICO, not SVG. Chrome reliably updates PNG/ICO favicons on a simple `href` swap but is flaky with SVG favicon updates, so PNG/ICO is used for the theme swap. Do not switch back to SVG for the theme swap; Chrome does not reliably re-render SVG favicons on `href` change.
- `docs/assets/favicon-light.png` / `favicon-light-16x16.png` / `favicon-light.ico` are the light tab favicons (black strokes on near-white interior, transparent corners).
- `docs/assets/favicon-dark.png` / `favicon-dark-16x16.png` / `favicon-dark.ico` are the dark tab favicons: same marks but pre-inverted (white strokes on near-black interior, transparent corners). Pixels are inverted at generation time; there is no CSS `filter` dependency.
- `docs/index.html` defines `window.setFaviconTheme(theme)` in the head, which swaps the `href` of the `id="favicon"`, `id="favicon-small"`, and `id="shortcut-icon"` links to `assets/favicon-${theme}.png?v=4` etc. A tiny head IIFE calls it with the saved/initial theme before `DOMContentLoaded` so dark-mode page loads do not show the light favicon for a noticeable delay. Keep this early path in sync with `initializeTheme`.
- `docs/js/main.js` `applyTheme` calls `window.setFaviconTheme(theme)` on every toggle. The toggle button's text is the static label `theme` in both states (not `dark`/`light`), so the centered nav row width does not shift when the label would otherwise change length.
- The hrefs include a static `?v=4` query because Chrome caches favicons in a separate favicon database and may ignore ordinary cache clears. Bump `?v=4` to `?v=5` (and update both the head script and `main.js`) if you ever need to force clients past a stale favicon.
- `docs/site.webmanifest` lists only the maskable `android-chrome` PNG icons (the manifest is for installable/PWA context and does not swap with the theme; the light mark is fine there).

When updating the logo or any favicon source asset:

- Every favicon-derived asset comes in a light/dark pair. If you change `docs/assets/logo.png`, regenerate the full set including both the `favicon-light.*` and `favicon-dark.*` (pre-inverted) variants. The regeneration recipe below handles this; do not regenerate only one theme or the tab favicon will desync between light and dark mode.
- The `favicon-dark.*` variants are derived by inverting RGB and preserving alpha at generation time, not at runtime. If the logo's stroke or interior colors change, verify the inverted output still reads correctly on a dark tab bar (dark interior should be near-black, strokes near-white, corners still transparent).

Icon rules:

- Icons are straight full-bleed resizes. Do not pad, inset, or add a background plate. The transparent corners are intentional and match the previous icon set.
- The `android-chrome` icons are declared `"purpose": "maskable"` in `docs/site.webmanifest` while being full-bleed, so a launcher may crop the outer edge. This matches the prior icons and is deliberate.

`thumbnail.png` (the `og:image` / `twitter:image` card):

- Ground `#FAF8F4` (warm off-white), ink `#121212`, mark 330px centered on 1220x650.
- The mark's white interior is remapped to the ground color so only the ink shows. Do this by lerping the mark's own black-to-white luminance ramp onto ink-to-ground, which keeps the antialiased edges clean. A flat color replacement leaves a halo.
- The ground sits just off pure white on purpose. A pure `#FFFFFF` card has no edge against white chat backgrounds (X, Slack light mode, iMessage) and reads as a floating logo rather than a card.

Regeneration recipe (requires Pillow):

```python
from PIL import Image

src = Image.open("docs/assets/logo.png").convert("RGBA")
A = "docs/assets/"

# icons: full-bleed resizes
for size, name in [
    (16, "favicon-16x16.png"),
    (32, "favicon-32x32.png"),
    (180, "apple-touch-icon.png"),
    (192, "android-chrome-192x192.png"),
    (512, "android-chrome-512x512.png"),
]:
    img = src.resize((size, size), Image.LANCZOS)
    img.quantize(colors=128, method=Image.FASTOCTREE).save(A + name, optimize=True)

src.resize((256, 256), Image.LANCZOS).save(
    A + "favicon.ico", format="ICO", sizes=[(s, s) for s in (16, 32, 48, 64)]
)

# favicon-light.* + favicon-dark.*: PNG/ICO favicons swapped by JS.
# The light variants are full-bleed resizes as-is; the dark variants are
# pre-inverted (RGB inverted, alpha preserved) so dark mode renders correctly
# without relying on a CSS filter. PNG/ICO is used instead of SVG because
# Chrome reliably updates PNG/ICO favicons on href swap but is flaky with SVG.
def _invert(img):
    r, g, b, a = img.split()
    return Image.merge("RGBA", [p.point(lambda v: 255 - v) for p in (r, g, b)] + [a])

for size, suffix in [(32, ""), (16, "-16x16")]:
    light = src.resize((size, size), Image.LANCZOS)
    light.quantize(colors=128, method=Image.FASTOCTREE).save(
        A + f"favicon-light{suffix}.png", optimize=True
    )
    _invert(light).quantize(colors=128, method=Image.FASTOCTREE).save(
        A + f"favicon-dark{suffix}.png", optimize=True
    )

for theme in ("light", "dark"):
    base = src.resize((64, 64), Image.LANCZOS)
    if theme == "dark":
        base = _invert(base)
    base.save(
        A + f"favicon-{theme}.ico",
        format="ICO",
        sizes=[(s, s) for s in (16, 32, 48, 64)],
    )

# thumbnail: mark fill remapped to the ground so only the ink shows
THUMB, MARK = (1220, 650), 330
BG, INK = (0xFA, 0xF8, 0xF4), (0x12, 0x12, 0x12)
logo = src.resize((MARK, MARK), Image.LANCZOS)
lum, alpha = logo.convert("L"), logo.getchannel("A")
mark = Image.merge(
    "RGB",
    [lum.point(lambda v, i=i: round(INK[i] + (BG[i] - INK[i]) * v / 255)) for i in range(3)],
).convert("RGBA")
mark.putalpha(alpha)
card = Image.new("RGBA", THUMB, BG + (255,))
card.alpha_composite(mark, ((THUMB[0] - MARK) // 2, (THUMB[1] - MARK) // 2))
card.convert("RGB").save(A + "thumbnail.png", optimize=True)
```

Gotchas:

- Do not palette-quantize `thumbnail.png`. Quantizing pulls the ground off exact `#FAF8F4` (it lands on `#F9F7F3`). Save it as RGB. The icons quantize fine because they are effectively two tones.
- Inverting a card to explore colors also inverts the ink and leaves a color cast (an earlier pass produced a `#FDFAFE` ground with a magenta tint). Always rebuild from `logo.png` rather than transforming a previous card.
- `docs/index.html` declares `og:image:width` 1200 and `og:image:height` 630, but `thumbnail.png` is 1220x650. If you resize the card, update those tags, and vice versa.
- `og:image`, `twitter:image`, `og:url`, and `canonical` are absolute deployed URLs because link-preview crawlers have no page context to resolve relative paths against. `docs/run.py` rewrites them to the local origin when serving HTML, so local previews use local assets. See "Local Preview Server" below.
- Nothing in `docs/index.html` references `logo.png` directly, so its file size does not affect page load. It exists as the master art.

## Local Preview Server

`docs/run.py` serves `docs/` on the first free port in 8000-8099 and opens a browser. Stop it with Ctrl+C or Ctrl+D.

Site UI behavior:

- The main vertical scrollbar is hidden in `docs/css/main.css` via `html { scrollbar-width: none }` and `html::-webkit-scrollbar { display: none }`. Horizontal page scrolling is disabled at the root (`html` and `body` use `overflow-x: hidden`); wide rendered content must scroll inside its own code/table container instead of widening the page. Scrolling still works via keyboard, trackpad, and mouse wheel; only visible page scroll gutters are removed. Do not remove the root `overflow-x: hidden` or the page will horizontally scroll again on mobile when the rendered README contains wide tables, long links, or large images.
- The nav row (`docs/index.html` `.links-container`) is `text-align: center`. On screens `<=768px`, the secondary links `vision`, `install`, and `features` (plus their adjacent `|` separators) are hidden via the `.nav-secondary` class so the row stays on one line and does not wrap. The remaining links `demo`, `docs`, `github`, and the `theme` toggle stay visible. Do not remove the `.nav-secondary` class from these links/`span`s or the mobile nav will wrap again.

Rendered README notes:

- `docs/js/docs.js` wraps markdown tables in `.docs-table-wrap` after Marked renders the README. Keep horizontal table scrolling on the wrapper, not the `table`, so the table layout stays intact and does not leave an empty full-width bordered area.
- Rendered README code-block copy buttons are vertically centered by `docs/css/docs.css` using absolute positioning within `#docs-content pre`.
- Rendered README images, videos, long links, code blocks, and table wrappers are constrained in `docs/css/docs.css` so mobile screens do not get page-level horizontal scrolling. When adding new rendered-content styling, keep wide elements inside their own `overflow-x: auto` container with `max-width: 100%` / `box-sizing: border-box`; do not let them expand the page width.

It rewrites the deployed origin to the local origin in HTML responses only:

- `site_origin` reads the page's own `<link rel="canonical">` href and uses it as the deployed origin. There is no hardcoded production URL in `run.py`, so changing the domain in `docs/index.html` needs no matching change here.
- `localize` no-ops when there is no canonical tag, when the canonical href is relative, or when the origin already matches the local one. Non-HTML responses are never touched.
- `LocalPreviewHandler.send_head` returns the rewritten bytes with a corrected `Content-Length`, so GET and HEAD both stay consistent.
- Every response gets `Cache-Control: no-store`, which stops browsers from serving stale favicons after the icon set is regenerated.

Why this exists: without the rewrite, a link-preview crawler pointed at localhost reads `og:image` and fetches the deployed image instead of the local one, so asset changes appear to have no effect. Crawlers do not run JavaScript, so the swap has to happen server-side.

When editing `run.py`, keep the rewrite HTML-only and keep `Content-Length` in sync with the rewritten body.

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
- Any of the following change: flags, config fields, platforms, env vars, interactive commands, file structure, test patterns, or website assets.

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
