'use strict';

const editor = document.querySelector('[data-editor]');
if (editor) {
  const body = editor.querySelector('[name="body"]');
  const count = editor.querySelector('[data-character-count]');
  const writePane = editor.querySelector('[data-write-pane]');
  const previewPane = editor.querySelector('[data-preview-pane]');
  const previewStatus = editor.querySelector('[data-preview-status]');
  const modeButtons = [...editor.querySelectorAll('[data-editor-mode]')];
  let dirty = false;
  let previewRequest;
  const updateCount = () => { count.textContent = `${Array.from(body.value).length.toLocaleString('en-US')} / 500,000 characters`; };
  const setMode = mode => {
    writePane.hidden = mode !== 'write';
    previewPane.hidden = mode !== 'preview';
    modeButtons.forEach(button => {
      const active = button.dataset.editorMode === mode;
      button.classList.toggle('active', active);
      button.setAttribute('aria-pressed', String(active));
    });
  };
  updateCount();
  editor.addEventListener('input', () => { dirty = true; updateCount(); });
  body.addEventListener('invalid', () => setMode('write'));
  editor.addEventListener('submit', () => {
    dirty = false;
    const button = editor.querySelector('[data-save-post]');
    button.disabled = true;
    button.textContent = 'Saving changes…';
  });
  window.addEventListener('beforeunload', event => {
    if (dirty) { event.preventDefault(); event.returnValue = ''; }
  });
  modeButtons.forEach(button => button.addEventListener('click', async () => {
    const mode = button.dataset.editorMode;
    setMode(mode);
    if (mode !== 'preview') {
      if (previewRequest) previewRequest.abort();
      previewStatus.textContent = '';
      body.focus();
      return;
    }
    if (previewRequest) previewRequest.abort();
    previewRequest = new AbortController();
    previewStatus.textContent = 'Rendering preview…';
    const values = new URLSearchParams({ body: body.value, csrf: editor.querySelector('[name="csrf"]').value });
    try {
      const response = await fetch(editor.dataset.previewUrl, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded', Accept: 'text/html' },
        body: values,
        signal: previewRequest.signal
      });
      if (!response.ok) throw new Error(response.status === 403 ? 'Your form has expired. Save a copy of your text and reload the page.' : 'The preview could not be generated. Your text is still in the editor.');
      previewPane.innerHTML = await response.text();
      if (!body.value.trim()) previewPane.textContent = 'Your preview will appear here when you start writing.';
      previewStatus.textContent = '';
    } catch (error) {
      if (error.name !== 'AbortError') previewStatus.textContent = error.message;
    }
  }));
}

document.querySelectorAll('[data-publishing-form]').forEach(form => {
  form.addEventListener('submit', () => {
    const button = form.querySelector('button[type="submit"], button:not([type])');
    if (button) { button.disabled = true; button.textContent = 'Generating pages…'; }
  });
});
