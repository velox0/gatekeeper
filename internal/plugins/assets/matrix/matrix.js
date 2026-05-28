(function () {
  const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
  if (mq.matches) return;

  const canvas = document.createElement("canvas");
  canvas.className = "matrix-canvas";
  canvas.setAttribute("aria-hidden", "true");
  document.body.insertBefore(canvas, document.body.firstChild);

  const ctx = canvas.getContext("2d");

  function resize() {
    canvas.width  = window.innerWidth;
    canvas.height = window.innerHeight;
  }
  resize();
  window.addEventListener("resize", resize);

  const FONT_SIZE = 14;
  const cols = Math.floor(canvas.width / FONT_SIZE);
  const drops = [];
  for (let i = 0; i < cols; i++) drops[i] = Math.random() * -100;

  const chars = "ゲートキーパー01アクセス許可認証セキュリティABCDEF0123456789";

  function draw() {
    ctx.fillStyle = "rgba(6, 6, 15, 0.06)";
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.font = FONT_SIZE + "px monospace";

    for (let i = 0; i < drops.length; i++) {
      const ch = chars[Math.floor(Math.random() * chars.length)];
      const x = i * FONT_SIZE;
      const y = drops[i] * FONT_SIZE;

      const brightness = Math.random();
      if (brightness > 0.95) {
        ctx.fillStyle = "#e0e0ff";
      } else if (brightness > 0.8) {
        ctx.fillStyle = "rgba(167, 139, 250, 0.9)";
      } else {
        ctx.fillStyle = "rgba(124, 92, 252, 0.5)";
      }

      ctx.fillText(ch, x, y);

      if (y > canvas.height && Math.random() > 0.975) {
        drops[i] = 0;
      }
      drops[i] += 0.6;
    }
    requestAnimationFrame(draw);
  }
  requestAnimationFrame(draw);
})();
