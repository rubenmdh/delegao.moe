'use strict';
(() => {
  const link = document.querySelector('[data-ipv6-indicator]');
  if (!link) return;
  const label = link.querySelector('[data-ipv6-label]');
  const image = new Image();
  let finished = false;
  const finish = available => {
    if (finished) return;
    finished = true;
    clearTimeout(timeout);
    image.onload = image.onerror = null;
    link.dataset.state = available ? 'available' : 'undetected';
    label.textContent = available ? 'IPv6 available' : 'IPv6 not detected';
    link.title = available
      ? 'IPv6 connection to ipv6.sx succeeded. Open the full connection test.'
      : 'IPv6 was not detected. External images may be blocked or the server unavailable. Open the full test at ipv6.sx.';
  };
  image.referrerPolicy = 'no-referrer';
  image.onload = () => finish(true);
  image.onerror = () => finish(false);
  const timeout = setTimeout(() => finish(false), 4000);
  // The official embed probes this IPv6-only image host. Images do not need
  // cross-origin API permission; keep the UI in our own typography and colours.
  image.src = 'https://v6.ipv6.sx/badge.svg';
})();
