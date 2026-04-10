/**
 * Main JS for Ch CLI landing page
 */

// Initialize theme from localStorage or system preference
export function initializeTheme() {
  const savedTheme = localStorage.getItem("theme") || "light";
  const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
  const theme =
    savedTheme !== "light" && (savedTheme === "dark" || prefersDark)
      ? "dark"
      : "light";
  applyTheme(theme);
}

export function applyTheme(theme) {
  const button = document.querySelector(".theme-toggle");
  if (!button) return;

  if (theme === "dark") {
    document.body.classList.add("dark-mode");
    button.textContent = "light";
  } else {
    document.body.classList.remove("dark-mode");
    button.textContent = "dark";
  }
  localStorage.setItem("theme", theme);
}

export function toggleTheme() {
  const isDark = document.body.classList.contains("dark-mode");
  applyTheme(isDark ? "light" : "dark");
}

export function scrollToTop() {
  window.scrollTo({
    top: 0,
    behavior: "smooth",
  });
}

export async function setupCopyables() {
  if (navigator.clipboard) {
    for (const element of document.getElementsByClassName("copyable")) {
      const codeElement = element.querySelector("code");
      let text = codeElement
        ? codeElement.innerText.trim()
        : element.innerText.trim();
      if (text.startsWith("$")) {
        text = text.substr(1).trimStart();
      }

      const button = document.createElement("button");
      button.innerHTML = "Copy";
      button.setAttribute("aria-label", "Copy to clipboard");
      button.onclick = () => {
        navigator.clipboard.writeText(text);
        button.innerHTML = "Copied!";
        setTimeout(() => (button.innerHTML = "Copy"), 1000);
      };
      element.appendChild(button);
    }
  }
}

// Bind to window for global access (legacy onclick handlers)
window.toggleTheme = toggleTheme;
window.scrollToTop = scrollToTop;
