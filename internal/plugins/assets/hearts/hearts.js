(function () {
  var mq = window.matchMedia("(prefers-reduced-motion: reduce)");
  if (mq.matches) return;

  var BURST_MS = 2400;
  var HEARTS   = ["\u2764", "\uD83D\uDC96", "\uD83D\uDC97", "\uD83D\uDC98", "\uD83D\uDC95"];

  function rand(a, b) { return a + Math.random() * (b - a); }

  function createLaunchpad() {
    var lp = document.createElement("div");
    lp.className = "heart-launchpad";
    lp.setAttribute("aria-hidden", "true");
    document.body.appendChild(lp);
    return lp;
  }

  function spawnHeart(lp) {
    var h = document.createElement("span");
    h.className = "heart-float";
    h.textContent = HEARTS[Math.floor(Math.random() * HEARTS.length)];
    h.style.left       = rand(2, 98) + "vw";
    h.style.fontSize   = rand(14, 34) + "px";
    h.style.animationDuration = rand(3.2, 6.1) + "s";
    h.style.animationDelay    = rand(0, 0.9) + "s";
    h.style.setProperty("--drift", rand(-24, 24).toFixed(2));
    h.style.setProperty("--lift",  rand(-8, 18).toFixed(2));
    h.style.setProperty("--spin",  rand(-40, 40).toFixed(2));
    h.addEventListener("animationend", function () { h.remove(); });
    lp.appendChild(h);
  }

  window.addEventListener("load", function () {
    var lp = createLaunchpad();
    var t0 = performance.now();
    var iv = setInterval(function () {
      spawnHeart(lp);
      if (performance.now() - t0 >= BURST_MS) {
        clearInterval(iv);
        setTimeout(function () { lp.remove(); }, 7000);
      }
    }, 34);
    for (var i = 0; i < 20; i++) spawnHeart(lp);
  });
})();
