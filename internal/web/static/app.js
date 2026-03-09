(() => {
  const modal = document.getElementById("modal");
  const backdrop = document.getElementById("modalBackdrop");
  const closeBtn = document.getElementById("modalClose");

  if (!modal || !backdrop) {
    return;
  }

  const setOpen = (open) => {
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

