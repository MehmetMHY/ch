/**
 * Documentation rendering logic for Ch CLI landing page
 */

const DOCS_ROOT_URL = "https://github.com/MehmetMHY/ch/blob/main/";
let docsLoaded = false;
export let docsVisible = false;
let docsRetryCount = 0;
const DOCS_TIMEOUT_MS = 10000;
const DOCS_MAX_RETRIES = 1;

function addDocsCopyButtons() {
  document.querySelectorAll("#docs-content pre").forEach((pre) => {
    const button = document.createElement("button");
    button.className = "docs-copy-btn";
    button.textContent = "Copy";
    button.onclick = () => {
      const code = pre.querySelector("code") || pre;
      navigator.clipboard.writeText(code.textContent);
      button.textContent = "Copied!";
      setTimeout(() => (button.textContent = "Copy"), 1000);
    };
    pre.appendChild(button);
  });
}

function slugify(text) {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, "")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-");
}

function addHeadingIds() {
  document
    .querySelectorAll(
      "#docs-content h1, #docs-content h2, #docs-content h3, #docs-content h4, #docs-content h5, #docs-content h6",
    )
    .forEach((heading) => {
      if (!heading.id) {
        heading.id = slugify(heading.textContent);
      }
    });
}

function fixDocsRelativeLinks() {
  document.querySelectorAll('#docs-content a[href^="./"]').forEach((link) => {
    if (!link.closest("pre")) {
      link.href = DOCS_ROOT_URL + link.getAttribute("href").substring(2);
    }
  });
}

function fetchWithTimeout(url, timeout = DOCS_TIMEOUT_MS) {
  return Promise.race([
    fetch(url),
    new Promise((_, reject) =>
      setTimeout(() => reject(new Error("Request timed out")), timeout),
    ),
  ]);
}

function showDocsLoading() {
  document.getElementById("docs-content").innerHTML =
    '<p style="text-align: center; color: var(--secondary-text);">Loading documentation...</p>';
}

function showDocsError(isTimeout = false, isRetrying = false) {
  const docsContent = document.getElementById("docs-content");
  if (isRetrying) {
    docsContent.innerHTML =
      '<p style="text-align: center; color: var(--secondary-text);">Connection issue detected. Retrying...</p>';
  } else {
    const errorMsg = isTimeout
      ? "Request timed out after 10 seconds."
      : "Failed to load documentation.";
    docsContent.innerHTML = `
      <p style="text-align: center; color: var(--secondary-text);">
        ${errorMsg} GitHub may be temporarily unavailable.<br>
        Please visit the <a href="https://github.com/MehmetMHY/ch/blob/main/README.md" target="_blank">GitHub README</a> directly
        or <a href="#docs" onclick="event.preventDefault(); window.retryLoadDocs();" style="text-decoration: underline; cursor: pointer;">click here to retry</a>.
      </p>
    `;
  }
}

let docsAssetsPromise = null;
function loadDocsAssets() {
  if (docsAssetsPromise) return docsAssetsPromise;
  function addScript(src) {
    return new Promise((resolve, reject) => {
      const s = document.createElement("script");
      s.src = src;
      s.onload = resolve;
      s.onerror = reject;
      document.head.appendChild(s);
    });
  }
  function addStyle(href) {
    return new Promise((resolve, reject) => {
      const l = document.createElement("link");
      l.rel = "stylesheet";
      l.href = href;
      l.onload = resolve;
      l.onerror = reject;
      document.head.appendChild(l);
    });
  }
  docsAssetsPromise = Promise.all([
    addScript("https://cdn.jsdelivr.net/npm/marked@13.0.1/marked.min.js"),
    addScript(
      "https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/highlight.min.js",
    ),
    addStyle(
      "https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/styles/github.min.css",
    ),
  ]);
  return docsAssetsPromise;
}

export function loadDocs(isRetry = false) {
  if (docsLoaded) return;

  if (isRetry) {
    docsRetryCount++;
    showDocsError(false, true);
  } else {
    showDocsLoading();
  }

  loadDocsAssets()
    .then(() =>
      fetchWithTimeout(
        "https://raw.githubusercontent.com/MehmetMHY/ch/refs/heads/main/README.md",
      ),
    )
    .then((response) => {
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      return response.text();
    })
    .then((markdown) => {
      docsLoaded = true;
      marked.setOptions({
        breaks: true,
        gfm: true,
      });
      document.getElementById("docs-content").innerHTML =
        marked.parse(markdown);
      addHeadingIds();
      addDocsCopyButtons();
      fixDocsRelativeLinks();
      document
        .querySelectorAll("#docs-content pre code")
        .forEach((codeBlock) => {
          codeBlock.removeAttribute("data-highlighted");
          hljs.highlightElement(codeBlock);
        });
    })
    .catch((error) => {
      console.error("Error loading docs:", error);
      const isTimeout = error.message === "Request timed out";

      if (docsRetryCount < DOCS_MAX_RETRIES) {
        setTimeout(() => {
          loadDocs(true);
        }, 2000);
      } else {
        showDocsError(isTimeout, false);
        docsLoaded = false;
      }
    });
}

export function retryLoadDocs() {
  docsRetryCount = 0;
  docsLoaded = false;
  loadDocs();
}

window.retryLoadDocs = retryLoadDocs;

export function applyDocsVisibility(visible) {
  const docsContent = document.getElementById("docs-content");
  const docsToggle = document.getElementById("docs-toggle");
  const docsGithubLink = document.getElementById("docs-github-link");

  if (visible) {
    docsContent.style.display = "block";
    docsToggle.textContent = "Collapse the project's rendered README below";
    if (docsGithubLink) docsGithubLink.style.display = "none";
    docsVisible = true;
    loadDocs();
  } else {
    docsContent.style.display = "none";
    docsToggle.textContent = "render the project's README below";
    if (docsGithubLink) docsGithubLink.style.display = "inline";
    docsVisible = false;
  }
  localStorage.setItem("docsVisible", visible);
}

export function initializeDocsVisibility() {
  const savedState = localStorage.getItem("docsVisible");
  const visible = savedState === "true";
  applyDocsVisibility(visible);
}

export function toggleDocs() {
  applyDocsVisibility(!docsVisible);
}
