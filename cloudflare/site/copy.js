const status = document.querySelector('[role="status"]');

for (const button of document.querySelectorAll('button[data-copy]')) {
  const source = document.getElementById(button.dataset.copy);
  if (!source) continue;
  button.hidden = false;
  let reset;
  button.addEventListener('click', async () => {
    clearTimeout(reset);
    button.textContent = 'Copy';
    try {
      await navigator.clipboard.writeText(source.textContent);
      button.textContent = 'Copied';
      status.textContent = `${button.getAttribute('aria-label').replace(/^Copy /, '')} copied.`;
      reset = setTimeout(() => {
        button.textContent = 'Copy';
      }, 2000);
    } catch {
      source.focus();
      const range = document.createRange();
      range.selectNodeContents(source);
      const selection = window.getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      status.textContent = 'Clipboard unavailable. Press Ctrl+C or ⌘C to copy the selected text.';
    }
  });
}
