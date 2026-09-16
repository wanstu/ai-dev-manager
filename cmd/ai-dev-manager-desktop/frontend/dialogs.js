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
