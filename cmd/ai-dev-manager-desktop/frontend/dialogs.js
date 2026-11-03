const dialogOpeners = new WeakMap();
function openEditorDialog(id) {
 const dialog = document.getElementById(id);
 if (!dialog || dialog.open) return;
 dialogOpeners.set(dialog, document.activeElement);
 dialog.querySelector('.dialog-message').hidden = true;
 dialog.showModal();
 document.body.classList.add('has-editor-dialog');
 const field = dialog.querySelector('input:not([type="hidden"]):not([disabled]), select, textarea');
 field?.focus({preventScroll: true});
}
function closeEditorDialog(id) {
 const dialog = document.getElementById(id);
 if (dialog?.open) dialog.close();
}
function closeFormDialog(form) {
 const dialog = form.closest('dialog');
 if (dialog) closeEditorDialog(dialog.id);
}
function activeEditorDialog() { return [...document.querySelectorAll('dialog[open]')].at(-1); }
function confirmInApp(message, options = {}) {
 const dialog = document.getElementById('confirmationDialog');
 const title = document.getElementById('confirmationDialogTitle');
 const body = document.getElementById('confirmationDialogMessage');
 const confirmButton = document.getElementById('confirmationDialogConfirmButton');
 if (!dialog || !title || !body || !confirmButton || typeof dialog.showModal !== 'function') {
  return Promise.resolve(typeof window.confirm === 'function' ? window.confirm(String(message || '')) : false);
 }
 if (dialog.open) dialog.close('cancel');
 dialog.returnValue = '';
 title.textContent = String(options.title || '确认操作');
 body.textContent = String(message || '');
 confirmButton.textContent = String(options.confirmText || '确认');
 confirmButton.classList.toggle('danger-button', options.danger !== false);
 dialogOpeners.set(dialog, document.activeElement);
 document.body.classList.add('has-editor-dialog');
 return new Promise((resolve) => {
  const onClose = () => resolve(dialog.returnValue === 'confirm');
  dialog.addEventListener('close', onClose, {once:true});
  dialog.showModal();
  confirmButton.focus({preventScroll:true});
 });
}
document.addEventListener('click', (event) => {
 const opener = event.target.closest('[data-dialog-open]');
 if (opener) {
  if (opener.dataset.dialogOpen === 'mcpEditorFlow') resetMCPEditor(false, false);
  if (opener.dataset.dialogOpen === 'skillSourceDialog') resetSkillSourceEditor(false, false);
  openEditorDialog(opener.dataset.dialogOpen);
 }
 const closer = event.target.closest('[data-dialog-close]');
 if (closer) closeEditorDialog(closer.closest('dialog').id);
});
document.addEventListener('DOMContentLoaded', () => {
 for (const dialog of document.querySelectorAll('.editor-dialog')) {
  dialog.addEventListener('close', () => {
   if (dialog.open) return;
   if (!document.querySelector('dialog[open]')) document.body.classList.remove('has-editor-dialog');
   const opener = dialogOpeners.get(dialog);
   if (opener?.isConnected) opener.focus({preventScroll: true});
   else if (dialog.id === 'mcpEditorFlow') document.getElementById('mcpFilter').focus({preventScroll: true});
   if (dialog.id === 'mcpEditorFlow') resetMCPEditor(false, false);
   if (dialog.id === 'skillSourceDialog') resetSkillSourceEditor(false, false);
  });
 }
});
