(() => {
  document.querySelectorAll(".partner-logo-img").forEach((img) => {
    img.addEventListener("error", () => {
      const parent = img.closest(".logo-partner");
      if (parent) parent.style.display = "none";
    });
  });
})();

(() => {
  document.querySelectorAll(".main-logo-img").forEach((img) => {
    img.addEventListener("error", () => {
      const parent = img.closest(".logo-main");
      if (parent) parent.style.display = "none";
    });
  });
})();

(() => {
  const btn = document.getElementById("detailsToggle");
  if (!btn) return;
  btn.addEventListener("click", () => {
    const body = document.body;
    const show = !body.classList.contains("show-details");
    body.classList.toggle("show-details", show);
    btn.textContent = show ? "Скрыть детали" : "Детали";
  });
})();


(() => {
  document.querySelectorAll('.burger').forEach(btn => {
    btn.addEventListener('click', () => {
      const wrapper = btn.closest('.nav-wrapper');
      if (wrapper) {
        wrapper.classList.toggle('nav-open');
      }
    });
  });
})();

// Theme toggle
(() => {
  const STORAGE_KEY = 'resource-battle-theme';
  const LIGHT_CLASS = 'theme-light';

  function setTheme(isLight) {
    const body = document.body;
    if (isLight) {
      body.classList.add(LIGHT_CLASS);
    } else {
      body.classList.remove(LIGHT_CLASS);
    }
    localStorage.setItem(STORAGE_KEY, isLight ? 'light' : 'dark');
    const btn = document.getElementById('themeToggle');
    if (btn) {
      btn.textContent = isLight ? '🌙' : '☀️';
    }
  }

  // Restore saved theme
  const saved = localStorage.getItem(STORAGE_KEY);
  if (saved === 'light') {
    setTheme(true);
  } else if (saved === 'dark') {
    setTheme(false);
  } else {
    // Default: dark
    setTheme(false);
  }

  const btn = document.getElementById('themeToggle');
  if (btn) {
    btn.addEventListener('click', () => {
      const isLight = document.body.classList.contains(LIGHT_CLASS);
      setTheme(!isLight);
    });
  }
})();
