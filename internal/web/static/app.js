(() => {
  const modal = document.getElementById("modal");
  const backdrop = document.getElementById("modalBackdrop");
  const closeBtn = document.getElementById("modalClose");

  const setOpen = (open) => {
    if (!modal || !backdrop) return;
    modal.hidden = !open;
    backdrop.hidden = !open;
    if (open) {
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
    }
  };

  const close = () => setOpen(false);

  closeBtn?.addEventListener("click", close);
  backdrop?.addEventListener("click", close);
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") close();
  });

  document.querySelectorAll(".leaderboard-details").forEach((btn) => {
    btn.addEventListener("click", () => {
      setOpen(true);
    });
  });
})();

(() => {
  document.querySelectorAll(".partner-logo-img").forEach((img) => {
    img.addEventListener("error", () => {
      const parent = img.closest(".logo-partner");
      if (parent) parent.style.display = "none";
    });
  });
})();

