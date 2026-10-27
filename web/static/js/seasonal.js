(() => {
  'use strict';

  // One fixed Madrid/CET window; cached pages must not repeat it next year.
  const startsAt = Date.parse('2026-10-28T00:00:00+01:00');
  const endsAt = Date.parse('2026-11-01T23:59:00+01:00');
  const maxDelay = 2147483647;
  let timer;

  function updateSeason() {
    clearTimeout(timer);
    timer = undefined;
    const now = Date.now();
    document.documentElement.classList.toggle('season-halloween', now >= startsAt && now < endsAt);
    const nextChange = now < startsAt ? startsAt : now < endsAt ? endsAt : undefined;
    if (nextChange !== undefined) timer = setTimeout(updateSeason, Math.min(nextChange - now, maxDelay));
  }

  updateSeason();
  window.addEventListener('pageshow', updateSeason);
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') updateSeason();
  });
})();
