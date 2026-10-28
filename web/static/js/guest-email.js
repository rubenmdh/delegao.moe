'use strict';
(() => {
  const dialog = document.getElementById('email-unlock');
  const form = dialog?.querySelector('[data-email-unlock-form]');
  if (!form) return;
  let pending;
  const status = form.querySelector('[data-email-unlock-status]');

  async function reveal(button) {
    button.parentElement.querySelector('[role="status"]')?.remove();
    const previous = button.textContent;
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 8000);
    button.disabled = true;
    button.textContent = 'Loading…';
    try {
      const response = await fetch(button.dataset.emailUrl, {
        method: 'POST', credentials: 'same-origin', cache: 'no-store',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: new URLSearchParams({csrf: button.dataset.emailCsrf}), signal: controller.signal,
      });
      if (response.status === 403 && response.headers.get('Content-Type')?.includes('application/json')) {
        const challenge = await response.json();
        if (challenge.error === 'dish_required') {
          pending = button;
          status.textContent = '';
          dialog.showModal();
          form.querySelector('[name="dishoftheday"]').focus();
          return;
        }
      }
      if (!response.ok) throw new Error(response.status === 429 ? 'Please try again later.' : 'Reload the page and try again.');
      const result = await response.json();
      if (typeof result.email !== 'string' || result.email.length > 128) throw new Error('Reload the page and try again.');
      const link = document.createElement('a');
      link.className = 'guest-email-contact';
      link.href = 'mailto:' + encodeURIComponent(result.email);
      link.textContent = result.email.replace('@', ' [at] ').replaceAll('.', ' [dot] ');
      link.title = 'Send an email';
      button.replaceWith(link);
      link.focus();
    } catch (error) {
      button.textContent = previous;
      let message = button.parentElement.querySelector('[role="status"]');
      if (!message) { message = document.createElement('small'); message.setAttribute('role', 'status'); button.after(message); }
      message.textContent = error.name === 'AbortError' ? 'Connection timed out. Try again.' : error.message;
    } finally {
      clearTimeout(timeout);
      button.disabled = false;
      if (button.isConnected && button.textContent === 'Loading…') button.textContent = previous;
    }
  }

  document.querySelectorAll('[data-email-url]').forEach(button => {
    button.addEventListener('click', () => reveal(button));
  });

  dialog.addEventListener('close', () => { pending = undefined; });
  form.addEventListener('submit', async event => {
    event.preventDefault();
    const submit = form.querySelector('[type="submit"]');
    submit.disabled = true;
    status.textContent = 'Checking…';
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 8000);
    try {
      const response = await fetch(form.action, {
        method: 'POST', credentials: 'same-origin', cache: 'no-store',
        body: new URLSearchParams(new FormData(form)), signal: controller.signal,
      });
      if (!response.ok) throw new Error(response.status === 422
        ? 'That is not the dish of the day. Check the About me page.'
        : response.status === 429 ? 'Too many answers. Please try again later.' : 'Reload the page and try again.');
      const target = pending;
      form.querySelector('[name="dishoftheday"]').value = '';
      dialog.close();
      if (target?.isConnected) await reveal(target);
    } catch (error) {
      status.textContent = error.name === 'AbortError' ? 'Connection timed out. Try again.' : error.message;
    } finally { clearTimeout(timeout); submit.disabled = false; }
  });
})();
