// GitHub star counts for the Projects cards.
// Counts are cached in localStorage so repeat visits stay well under the
// unauthenticated GitHub API rate limit. On failure the star slot simply
// stays hidden and the cards remain complete without it.

const CACHE_KEY = "projectStars";
// Bump when the cached shape or project set changes to force a clean refresh.
const CACHE_VERSION = 1;
const CACHE_TTL_MS = 6 * 60 * 60 * 1000;

function formatCount(n) {
  if (n < 1000) return String(n);
  const k = n / 1000;
  return `${k >= 10 ? Math.round(k) : k.toFixed(1).replace(/\.0$/, "")}k`;
}

function readCache() {
  try {
    const cached = JSON.parse(localStorage.getItem(CACHE_KEY));
    if (
      cached &&
      cached.version === CACHE_VERSION &&
      typeof cached.time === "number" &&
      cached.stars
    ) {
      return cached;
    }
  } catch (_) {}
  return null;
}

function writeCache(stars) {
  try {
    localStorage.setItem(
      CACHE_KEY,
      JSON.stringify({ version: CACHE_VERSION, time: Date.now(), stars }),
    );
  } catch (_) {}
}

function render(cards, stars) {
  cards.forEach((card) => {
    const count = stars[card.dataset.repo];
    const slot = card.querySelector(".project-stars");
    const countSlot = slot?.querySelector(".project-star-count");
    if (!slot || !countSlot || typeof count !== "number") return;
    countSlot.textContent = formatCount(count);
    slot.title = `${count} GitHub stars`;
    slot.setAttribute("aria-label", `${count} GitHub stars`);
    slot.hidden = false;
  });
}

async function fetchStars(repo) {
  const res = await fetch(`https://api.github.com/repos/${repo}`, {
    headers: { Accept: "application/vnd.github+json" },
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const data = await res.json();
  if (typeof data.stargazers_count !== "number") throw new Error("no count");
  return data.stargazers_count;
}

export async function initProjectStars() {
  const cards = Array.from(
    document.querySelectorAll(".project-card[data-repo]"),
  );
  if (!cards.length) return;

  const cached = readCache();
  if (cached) render(cards, cached.stars);
  if (cached && Date.now() - cached.time < CACHE_TTL_MS) return;

  const repos = cards.map((card) => card.dataset.repo);
  const results = await Promise.allSettled(repos.map(fetchStars));
  const stars = { ...(cached ? cached.stars : {}) };
  let fetched = false;
  let changed = false;
  results.forEach((result, i) => {
    if (result.status === "fulfilled") {
      if (stars[repos[i]] !== result.value) changed = true;
      stars[repos[i]] = result.value;
      fetched = true;
    }
  });
  if (!fetched) return;
  writeCache(stars);
  if (changed) render(cards, stars);
}
