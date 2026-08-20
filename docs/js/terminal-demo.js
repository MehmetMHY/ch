/**
 * Animated terminal demo for the Ch landing page.
 * Types a compact tour of common `ch` workflows.
 *
 * Features:
 * - typing with jitter, trailing blinking cursor
 * - word-by-word streaming for model output (natural token feel)
 * - drag-to-scroll on desktop (touch already swipes natively)
 * - scroll-in replay with a 3-min cooldown
 * - replay button to restart the tour on demand (always visible on
 *   touch, fades in on hover on desktop)
 * - skip button to fast-forward straight to the end of the tour
 * - tap/click a command line to copy it to clipboard
 * - reduced-motion: runs once with no delays; the static fallback
 *   already lives in the <pre id="demo"> markup so no-JS viewers
 *   still see the content.
 *
 * Robustness:
 * - A generation counter (genId) cancels in-flight runs on replay so
 *   overlapping animations never corrupt the DOM.
 * - sleep() polls in small increments so cancellation takes effect
 *   immediately rather than after the full delay.
 * - The animation holds while the tab is hidden and resumes on return.
 *   Browsers throttle background timers to ~1s, which would otherwise
 *   leave a viewer returning to a crawling, half-finished tour. This is
 *   keyed to tab visibility only; hovering never pauses playback.
 * - Clipboard copy falls back to execCommand when the async Clipboard
 *   API is unavailable (insecure contexts, old browsers).
 * - Copy, scroll, and replay listeners are registered before the
 *   reduced-motion early return so those users keep every interaction.
 * - All interactions work on both touch and mouse; the replay button
 *   is always visible on touch devices (no hover dependency).
 *
 * Accessibility:
 * - While JS animates the content, the <pre> becomes role="img" with a
 *   descriptive label so screen readers get one summary instead of a
 *   stream of mutations. Reduced-motion and no-JS viewers keep the
 *   fully readable static text.
 */

export function initTerminalDemo() {
  const demo = document.getElementById("demo");
  const demoContent = document.getElementById("demo-content");
  const replayBtn = document.getElementById("terminal-replay");
  const skipBtn = document.getElementById("terminal-skip");
  if (!demo || !demoContent) return;

  const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  const cursor = document.createElement("span");
  cursor.className = "cursor";

  // --- state -----------------------------------------------------
  let autoFollow = true;
  let userScrolling = false;
  let genId = 0;
  let currentParent = demoContent;
  let fastForward = false;
  let running = false;

  // --- helpers ---------------------------------------------------
  function sleep(ms) {
    return new Promise((resolve, reject) => {
      const myGen = genId;
      if (fastForward) {
        resolve();
        return;
      }
      let elapsed = 0;
      const step = 30;
      function tick() {
        if (myGen !== genId) {
          reject(new Error("aborted"));
          return;
        }
        if (fastForward) {
          resolve();
          return;
        }
        // hold, do not advance, while the tab is hidden
        if (document.hidden) {
          setTimeout(tick, 120);
          return;
        }
        elapsed += step;
        if (elapsed >= ms) {
          resolve();
          return;
        }
        setTimeout(tick, Math.min(step, ms - elapsed));
      }
      setTimeout(tick, Math.min(step, ms));
    });
  }

  function span(cls) {
    const s = document.createElement("span");
    if (cls) s.className = cls;
    currentParent.appendChild(s);
    return s;
  }

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

  // char-by-char typing (commands and prompts feel like keyboard input)
  async function type(text, cls, cps) {
    const s = span(cls);
    trailCursor();
    if (reduced || fastForward) {
      s.textContent = text;
      return;
    }
    const base = 1000 / (cps || 80);
    for (let i = 0; i < text.length; i++) {
      if (fastForward) {
        s.textContent = text;
        return;
      }
      s.textContent += text[i];
      await sleep(base + Math.random() * 8);
    }
  }

  // word-by-word streaming (model output feels like token streaming)
  async function typeWords(text, cls) {
    const s = span(cls);
    trailCursor();
    if (reduced || fastForward) {
      s.textContent = text;
      return;
    }
    const tokens = text.split(/(\s+)/);
    for (const token of tokens) {
      if (!token) continue;
      if (fastForward) {
        s.textContent = text;
        return;
      }
      s.textContent += token;
      // whitespace tokens are instant; words get a short delay
      if (/\s/.test(token)) continue;
      await sleep(16 + Math.random() * 6);
      // occasional micro-pause for natural rhythm
      if (Math.random() < 0.04) await sleep(50);
    }
  }

  async function stream(segments, cps) {
    for (const seg of segments) {
      await type(seg.text, seg.cls, cps);
    }
  }

  async function streamWords(segments) {
    for (const seg of segments) {
      await typeWords(seg.text, seg.cls);
    }
  }

  // --- clipboard -------------------------------------------------
  function copyToClipboard(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).catch(() => fallbackCopy(text));
    } else {
      fallbackCopy(text);
    }
  }

  function fallbackCopy(text) {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    try {
      document.execCommand("copy");
    } catch (_) {}
    document.body.removeChild(ta);
  }

  // --- scene builders -------------------------------------------
  async function command(parts) {
    await print("$", "dim");
    await print(" ");

    // wrap command text in a copyable span
    const cmdSpan = document.createElement("span");
    cmdSpan.className = "cmd-copy";
    demoContent.appendChild(cmdSpan);
    const cmdText = parts.map((p) => p.text).join("");
    cmdSpan.setAttribute("data-cmd", cmdText);
    cmdSpan.setAttribute("title", "click to copy");

    currentParent = cmdSpan;
    await stream(parts, 82);
    currentParent = demoContent;

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
    await streamWords(segments);
    await sleep(reduced ? 0 : 180);
  }

  async function sceneBreak(ms = 650) {
    await sleep(reduced ? 0 : ms);
  }

  // --- the tour --------------------------------------------------
  async function run() {
    // clear the static fallback so the animation is clean
    demoContent.textContent = "";
    demo.scrollLeft = 0;
    demo.scrollTop = 0;
    autoFollow = true;
    userScrolling = false;
    // while animating, collapse the mutating subtree into a single
    // labelled image for assistive tech instead of streaming changes
    if (!reduced) demo.setAttribute("role", "img");
    trailCursor();

    await command([
      { text: "ch", cls: "acc" },
      { text: " ", cls: null },
      { text: '"Explain goroutines in Go in 4 bullets"', cls: "str" },
    ]);
    await output([
      {
        text: "Goroutines are lightweight functions scheduled by the Go runtime, not one OS thread each.\n",
      },
      { text: "- start one with ", cls: null },
      { text: "go", cls: "acc" },
      {
        text: " f() and keep the caller moving\n- communicate with channels when ownership should move between tasks\n- use sync.WaitGroup or context.Context so work has a clean lifecycle\n- prefer small, boring goroutines over hidden global background work\n\n",
      },
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
      {
        text: "- config parsing now treats explicit false correctly\n- export paths sanitize slashes but preserve spaces and dots\n- add one regression test for quoted filenames and one for empty custom names\n\n",
      },
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
      {
        text: "1. install with curl | bash\n2. set OPENAI_API_KEY or switch providers\n3. run ",
        cls: null,
      },
      { text: "ch -v", cls: "acc" },
      {
        text: " to verify version metadata\n4. try a direct prompt, then interactive mode\n\n",
      },
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
      {
        text: " Ch demo\nmetadata loaded, subtitles compacted, cue numbers stripped\nready to paste into a prompt or pipe into another command\n\n",
      },
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
      {
        text: " ch_dump_internal_2026-08-19.txt\nincluded 47 text files, skipped binaries and vendor caches\n\n",
      },
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
    await prompt([
      { text: "!p", cls: "flag" },
      { text: " xai", cls: "platform" },
    ]);
    await output([
      { text: "xai", cls: "platform" },
      { text: " grok-4-fast-non-reasoning\n", cls: "model" },
    ]);
    await prompt([{ text: "!o", cls: "flag" }]);
    await output([
      { text: "google", cls: "platform" },
      { text: " gemini-2.5-flash\n", cls: "model" },
    ]);
    await prompt([
      { text: "!p", cls: "flag" },
      { text: " openrouter", cls: "platform" },
    ]);
    await output([
      { text: "openrouter", cls: "platform" },
      { text: " anthropic/claude-3.5-sonnet\n", cls: "model" },
    ]);
    await prompt([
      { text: "!l", cls: "flag" },
      { text: " ./internal/config", cls: "path" },
    ]);
    await output([
      { text: "loaded", cls: "ok" },
      { text: " 6 files into context\n", cls: null },
    ]);
    await prompt("What config behavior should tests protect?");
    await output([
      {
        text: "Protect env overrides, explicit false booleans, shallow_load_dirs defaults, and provider key mapping.\n",
      },
    ]);
    await prompt([
      { text: "!e", cls: "flag" },
      { text: " config_notes.txt", cls: "path" },
    ]);
    await output([
      { text: "exported chat:", cls: "ok" },
      { text: " config_notes.txt\n", cls: null },
    ]);
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
    await output([
      { text: "loaded latest session and sent follow-up prompt\n" },
    ]);

    // drop back to the prompt, cursor waits for the next query
    await print("$", "dim");
    await print(" ");
  }

  // --- run management -------------------------------------------
  const REPLAY_COOLDOWN_MS = 3 * 60 * 1000;
  let lastRunAt = 0;

  function setRunning(state) {
    running = state;
    // nothing to skip when there are no delays to skip past
    if (skipBtn) skipBtn.hidden = reduced ? true : !state;
  }

  function start() {
    genId++;
    const myGen = genId;
    fastForward = false;
    lastRunAt = Date.now();
    setRunning(true);
    run()
      .then(() => {
        // only the newest run may clear the running state
        if (myGen === genId) {
          fastForward = false;
          setRunning(false);
        }
      })
      .catch(() => {});
  }

  // --- copy command on click (event delegation) -----------------
  demoContent.addEventListener("click", (e) => {
    const cmdEl = e.target.closest(".cmd-copy");
    if (!cmdEl) return;
    const text = cmdEl.getAttribute("data-cmd") || "";
    if (!text) return;
    copyToClipboard(text);
    cmdEl.classList.add("copied");
    setTimeout(() => cmdEl.classList.remove("copied"), 800);
  });

  // --- manual scroll handling -----------------------------------
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

  // --- replay / skip buttons ------------------------------------
  if (replayBtn) {
    replayBtn.addEventListener("click", () => {
      start();
    });
  }

  if (skipBtn) {
    skipBtn.hidden = true;
    skipBtn.addEventListener("click", () => {
      if (!running) return;
      // let the in-flight run drain instantly rather than rebuilding,
      // so the tour ends in exactly the state it would have reached
      fastForward = true;
      autoFollow = true;
      userScrolling = false;
    });
  }

  // --- drag-to-scroll on desktop --------------------------------
  // touch already swipes natively. Only engages after a small
  // horizontal movement so clicks/taps and text selection are
  // not disturbed.
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

  // Reduced motion: render the tour once with no delays. Everything
  // above (copy, scroll, drag, replay) stays wired up because those
  // are interactions, not animation.
  if (reduced) {
    start();
    return;
  }

  // --- replay on scroll-in --------------------------------------
  // only past the cooldown so casual scrolling past the block
  // does not keep retriggering it
  if (typeof IntersectionObserver !== "function") {
    // no observer support: just play once so the demo is never blank
    start();
    return;
  }

  const io = new IntersectionObserver(
    (entries) => {
      entries.forEach((e) => {
        if (!e.isIntersecting) return;
        // never restart a tour that is still playing
        if (running) return;
        if (Date.now() - lastRunAt >= REPLAY_COOLDOWN_MS) {
          start();
        }
      });
    },
    { threshold: 0.4 },
  );
  io.observe(demo);
}
