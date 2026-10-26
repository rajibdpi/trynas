document.querySelectorAll('.copy-command').forEach(button => {
  let reset;
  button.addEventListener('click', async () => {
    const command = button.closest('.command').querySelector('code');
    try {
      await navigator.clipboard.writeText(command.textContent);
      button.setAttribute('aria-label', 'Command copied');
      button.title = 'Copied!';
      button.innerHTML = '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="m5 12 4 4L19 6"/></svg>';
    } catch {
      const selection = window.getSelection();
      const range = document.createRange();
      range.selectNodeContents(command);
      selection.removeAllRanges();
      selection.addRange(range);
      button.setAttribute('aria-label', 'Copy failed. Command selected; copy it manually.');
      button.title = 'Copy failed. Copy the selected command manually.';
    }
    clearTimeout(reset);
    reset = setTimeout(() => {
      button.setAttribute('aria-label', 'Copy install command');
      button.title = 'Copy command';
      button.innerHTML = '<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>';
    }, 2000);
  });
});
