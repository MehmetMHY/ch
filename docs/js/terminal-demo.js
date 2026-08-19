/**
 * Animated terminal demo for the Ch landing page.
 * Types a compact tour of common `ch` workflows.
 * Mirrors the index_ch terminal preview behavior:
 * - typing with jitter, trailing blinking cursor
 * - drag-to-scroll on desktop (touch swipes natively)
 * - scroll-in replay with a 3-min cooldown
 * - reduced-motion: runs once with no delays; the static fallback
 *   already lives in the <pre id="demo"> markup so no-JS viewers
 *   still see the content.
 */

export function initTerminalDemo() {
  const demo = document.getElementById("demo");
  const demoContent = document.getElementById("demo-content");
  if (!demo || !demoContent) return;

  const reduced = window.matchMedia(
    "(prefers-reduced-motion: reduce)",
  ).matches;

  const cursor = document.createElement("span");
  cursor.className = "cursor";
  let autoFollow = true;
  let userScrolling = false;

  function sleep(ms) {
    return new Promise((r) => setTimeout(r, ms));
  }

  function span(cls) {
    const s = document.createElement("span");
    if (cls) s.className = cls;
    demoContent.appendChild(s);
    return s;
  }

  // keep the blinking cursor at the end of the output
  function trailCursor() {
    demoContent.appendChild(cursor);
    if (autoFollow) {
      demo.scrollTop = demo.scrollHeight;
    }
  }

  function nearBottom() {
    return demo.scrollHeight - demo.scrollTop - demo.clientHeight < 24;
  }

  function markUserScroll() {
    userScrolling = true;
    autoFollow = false;
  }

  function print(text, cls) {
    const s = span(cls);
    s.textContent = text;
    trailCursor();
  }

  async function type(text, cls, cps) {
    const s = span(cls);
    trailCursor();
    const base = 1000 / (cps || 80);
    for (let i = 0; i < text.length; i++) {
      s.textContent += text[i];
      if (reduced) continue;
      await sleep(base + Math.random() * 8);
    }
  }

  // stream a sequence of {text, cls} segments in order
  async function stream(segments, cps) {
    for (const seg of segments) {
      await type(seg.text, seg.cls, cps);
    }
  }

  async function command(parts) {
    await print("$", "dim");
    await print(" ");
    if (Array.isArray(parts)) {
      await stream(parts, 82);
    } else {
      await type(parts, null, 82);
    }
    await print("\n");
    await sleep(reduced ? 0 : 180);
  }

  async function prompt(parts) {
    await print("user:", "prompt");
    await print(" ");
    if (Array.isArray(parts)) {
      await stream(parts, 94);
    } else {
      await type(parts, null, 94);
    }
    await print("\n");
    await sleep(reduced ? 0 : 130);
  }

  async function output(segments) {
    await stream(segments, 720);
    await sleep(reduced ? 0 : 180);
  }

  async function sceneBreak(ms = 650) {
    await sleep(reduced ? 0 : ms);
  }

  async function run() {
    // clear the static fallback so the animation is clean
    demoContent.textContent = "";
    demo.scrollLeft = 0;
    demo.scrollTop = 0;
    autoFollow = true;
    userScrolling = false;
    trailCursor();

    await command([
      { text: "ch", cls: "acc" },
      { text: " ", cls: null },
      { text: '"Explain goroutines in Go in 4 bullets"', cls: "str" },
    ]);
    await output([
      { text: "Goroutines are lightweight functions scheduled by the Go runtime, not one OS thread each.\n" },
      { text: "- start one with ", cls: null },
      { text: "go", cls: "acc" },
      { text: " f() and keep the caller moving\n- communicate with channels when ownership should move between tasks\n- use sync.WaitGroup or context.Context so work has a clean lifecycle\n- prefer small, boring goroutines over hidden global background work\n\n" },
    ]);
    await sceneBreak();

    await command([
      { text: "git", cls: "acc" },
      { text: " diff ", cls: null },
      { text: "|", cls: "dim" },
      { text: " ", cls: null },
      { text: "ch", cls: "acc" },
      { text: " ", cls: null },
      { text: '"summarize the risky changes"', cls: "str" },
    ]);
    await output([
      { text: "Risk summary:\n", cls: "warn" },
      { text: "- config parsing now treats explicit false correctly\n- export paths sanitize slashes but preserve spaces and dots\n- add one regression test for quoted filenames and one for empty custom names\n\n" },
    ]);
    await sceneBreak();

    await command([
      { text: "ch", cls: "acc" },
      { text: " ", cls: null },
      { text: "-l", cls: "flag" },
      { text: " ", cls: null },
      { text: "README.md", cls: "path" },
      { text: " ", cls: null },
      { text: '"turn install steps into a checklist"', cls: "str" },
    ]);
    await output([
      { text: "Checklist:\n", cls: "ok" },
      { text: "1. install with curl | bash\n2. set OPENAI_API_KEY or switch providers\n3. run ", cls: null },
      { text: "ch -v", cls: "acc" },
      { text: " to verify version metadata\n4. try a direct prompt, then interactive mode\n\n" },
    ]);
    await sceneBreak();

    await command([
      { text: "ch", cls: "acc" },
      { text: " ", cls: null },
      { text: "-s", cls: "flag" },
      { text: " ", cls: null },
      { text: "https://youtube.com/watch?v=AH0xG1iStf4", cls: "path" },
    ]);
    await output([
      { text: "YouTube:", cls: "ok" },
      { text: " Ch demo\nmetadata loaded, subtitles compacted, cue numbers stripped\nready to paste into a prompt or pipe into another command\n\n" },
    ]);
    await sceneBreak(550);

    await command([
      { text: "ch", cls: "acc" },
      { text: " ", cls: null },
      { text: "-w", cls: "flag" },
      { text: " ", cls: null },
      { text: '"Go 1.26 release notes"', cls: "str" },
    ]);
    await output([
      { text: "1. Go release notes - ", cls: null },
      { text: "https://go.dev/doc/devel/release", cls: "path" },
      { text: "\n2. Go blog - ", cls: null },
      { text: "https://go.dev/blog/", cls: "path" },
      { text: "\n3. Standard library changes - ", cls: null },
      { text: "https://pkg.go.dev/std", cls: "path" },
      { text: "\n\n", cls: null },
    ]);
    await sceneBreak();

    await command([
      { text: "ch", cls: "acc" },
      { text: " ", cls: null },
      { text: "-y", cls: "flag" },
      { text: " ", cls: null },
      { text: "-d", cls: "flag" },
      { text: " ", cls: null },
      { text: "./internal", cls: "path" },
    ]);
    await output([
      { text: "codedump saved:", cls: "ok" },
      { text: " ch_dump_internal_2026-08-19.txt\nincluded 47 text files, skipped binaries and vendor caches\n\n" },
    ]);
    await sceneBreak(450);

    await command([
      { text: "ch", cls: "acc" },
      { text: " ", cls: null },
      { text: "-y", cls: "flag" },
      { text: " ", cls: null },
      { text: "-b", cls: "flag" },
      { text: " ", cls: null },
      { text: "./cmd", cls: "path" },
      { text: " ", cls: null },
      { text: "cli_context.txt", cls: "path" },
    ]);
    await output([
      { text: "codedump saved:", cls: "ok" },
      { text: " cli_context.txt\ninteractive pickers skipped by --yes\n\n" },
    ]);
    await sceneBreak();

    await command([
      { text: "cat", cls: "acc" },
      { text: " ", cls: null },
      { text: "README.md", cls: "path" },
      { text: " ", cls: null },
      { text: "|", cls: "dim" },
      { text: " ", cls: null },
      { text: "ch", cls: "acc" },
      { text: " ", cls: null },
      { text: "-t", cls: "flag" },
    ]);
    await output([
      { text: "Token estimate for stdin:", cls: "ok" },
      { text: " 14,820 tokens\nmodel: ", cls: null },
      { text: "gpt-5.4-mini", cls: "model" },
      { text: "\n\n", cls: null },
    ]);
    await sceneBreak();

    await command([
      { text: "ch", cls: "acc" },
      { text: " ", cls: null },
      { text: "-p", cls: "flag" },
      { text: " ", cls: null },
      { text: "groq", cls: "platform" },
      { text: " ", cls: null },
      { text: "-m", cls: "flag" },
      { text: " ", cls: null },
      { text: "llama3-8b-8192", cls: "model" },
      { text: " ", cls: null },
      { text: '"give me a fast regex"', cls: "str" },
    ]);
    await output([
      { text: "groq", cls: "platform" },
      { text: " ", cls: null },
      { text: "llama3-8b-8192", cls: "model" },
      { text: "\nUse ", cls: null },
      { text: "(?m)^func\\s+([A-Za-z0-9_]+)\\(", cls: "str" },
      { text: " to list Go function names line-by-line.\n\n" },
    ]);
    await sceneBreak(750);

    await command([{ text: "ch", cls: "acc" }]);
    await output([
      { text: "interactive session started\n", cls: null },
      { text: "openai", cls: "platform" },
      { text: " ", cls: null },
      { text: "gpt-5.4-mini", cls: "model" },
      { text: "\n\n", cls: null },
    ]);
    await prompt([{ text: "!p", cls: "flag" }, { text: " anthropic", cls: "platform" }]);
    await output([{ text: "anthropic", cls: "platform" }, { text: " claude-3-5-sonnet-latest\n", cls: "model" }]);
    await prompt([{ text: "!m", cls: "flag" }, { text: " claude-3-5-sonnet-latest", cls: "model" }]);
    await output([{ text: "anthropic", cls: "platform" }, { text: " claude-3-5-sonnet-latest\n", cls: "model" }]);
    await prompt([{ text: "!l", cls: "flag" }, { text: " ./internal/config", cls: "path" }]);
    await output([{ text: "loaded", cls: "ok" }, { text: " 6 files into context\n", cls: null }]);
    await prompt("What config behavior should tests protect?");
    await output([
      { text: "Protect env overrides, explicit false booleans, shallow_load_dirs defaults, and provider key mapping.\n" },
    ]);
    await prompt([{ text: "!e", cls: "flag" }, { text: " config_notes.txt", cls: "path" }]);
    await output([{ text: "exported chat:", cls: "ok" }, { text: " config_notes.txt\n", cls: null }]);
    await prompt([{ text: "!q", cls: "flag" }]);
    await output([{ text: "bye\n\n" }]);
    await sceneBreak(750);

    await command([
      { text: "ch", cls: "acc" },
      { text: " ", cls: null },
      { text: "-c", cls: "flag" },
      { text: " ", cls: null },
      { text: '"continue where we left off"', cls: "str" },
    ]);
    await output([{ text: "loaded latest session and sent follow-up prompt\n" }]);

    // drop back to the prompt, cursor waits for the next query
    await print("$", "dim");
    await print(" ");
  }

  const REPLAY_COOLDOWN_MS = 3 * 60 * 1000;
  let lastRunAt = 0;
  let running = false;

  function start() {
    if (running) return;
    running = true;
    lastRunAt = Date.now();
    run().then(
      () => {
        running = false;
      },
      () => {
        running = false;
      },
    );
  }

  if (reduced) {
    start();
    return;
  }

  demo.addEventListener("wheel", markUserScroll, { passive: true });
  demo.addEventListener("touchstart", markUserScroll, { passive: true });
  demo.addEventListener(
    "keydown",
    (e) => {
      if (
        e.key === "ArrowUp" ||
        e.key === "ArrowDown" ||
        e.key === "PageUp" ||
        e.key === "PageDown" ||
        e.key === "Home" ||
        e.key === "End"
      ) {
        markUserScroll();
      }
    },
    { passive: true },
  );
  demo.addEventListener(
    "scroll",
    () => {
      if (userScrolling && nearBottom()) {
        autoFollow = true;
        userScrolling = false;
      }
    },
    { passive: true },
  );

  // drag-to-scroll on desktop (touch already swipes natively).
  // only engages after a small horizontal movement so clicks/taps
  // and text selection are not disturbed.
  (function enableDrag() {
    let down = false;
    let dragging = false;
    let startX = 0;
    let startScroll = 0;
    const THRESH = 6;

    demo.addEventListener("pointerdown", (e) => {
      if (e.pointerType === "touch") return;
      down = true;
      dragging = false;
      startX = e.clientX;
      startScroll = demo.scrollLeft;
      demo.setPointerCapture(e.pointerId);
    });

    demo.addEventListener("pointermove", (e) => {
      if (!down) return;
      const dx = e.clientX - startX;
      if (!dragging && Math.abs(dx) > THRESH) {
        dragging = true;
        demo.classList.add("dragging");
      }
      if (dragging) {
        e.preventDefault();
        demo.scrollLeft = startScroll - dx;
      }
    });

    function end(e) {
      if (!down) return;
      down = false;
      if (dragging) {
        dragging = false;
        demo.classList.remove("dragging");
        try {
          demo.releasePointerCapture(e.pointerId);
        } catch (_) {}
      }
    }

    demo.addEventListener("pointerup", end);
    demo.addEventListener("pointercancel", end);
  })();

  // replay on scroll-in, but only past the cooldown so casual
  // scrolling past the block does not keep retriggering it
  const io = new IntersectionObserver(
    (entries) => {
      entries.forEach((e) => {
        if (!e.isIntersecting) return;
        if (Date.now() - lastRunAt >= REPLAY_COOLDOWN_MS) {
          start();
        }
      });
    },
    { threshold: 0.4 },
  );
  io.observe(demo);
}
