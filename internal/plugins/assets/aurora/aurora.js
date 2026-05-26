(function () {
  var mq = window.matchMedia("(prefers-reduced-motion: reduce)");
  if (mq.matches) return;

  var wrap = document.createElement("div");
  wrap.className = "aurora-wrap";
  wrap.setAttribute("aria-hidden", "true");

  for (var i = 1; i <= 3; i++) {
    var band = document.createElement("div");
    band.className = "aurora-band aurora-band-" + i;
    wrap.appendChild(band);
  }

  document.body.insertBefore(wrap, document.body.firstChild);
})();
