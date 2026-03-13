(() => {
  document.querySelectorAll(".partner-logo-img").forEach((img) => {
    img.addEventListener("error", () => {
      const parent = img.closest(".logo-partner");
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

