'use strict';
const siteMenu = document.querySelector('[data-site-menu]');
if (siteMenu) {
  document.addEventListener('click', event => {
    if (!siteMenu.contains(event.target) || event.target.closest('a')) siteMenu.open = false;
  });
  document.addEventListener('keydown', event => {
    if (event.key === 'Escape' && siteMenu.open) {
      siteMenu.open = false;
      siteMenu.querySelector('summary').focus();
    }
  });
  window.addEventListener('pageshow', () => { siteMenu.open = false; });
}

document.querySelectorAll('[data-dialog]').forEach(button => {
  button.addEventListener('click', () => {
    const dialog = document.getElementById(button.dataset.dialog);
    if (dialog.open) dialog.close();
    dialog.showModal();
  });
});
document.querySelectorAll('[data-close]').forEach(button => {
  button.addEventListener('click', () => document.getElementById(button.dataset.close).close());
});
document.querySelectorAll('form[data-confirm]').forEach(form => {
  form.addEventListener('submit', event => {
    if (!window.confirm(form.dataset.confirm)) event.preventDefault();
  });
});
