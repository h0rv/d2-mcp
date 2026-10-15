const status = document.querySelector('[role="status"]');

for (const group of document.querySelectorAll('[data-tabs]')) {
  const tabs = [...group.querySelectorAll('[role="tab"]')];
  const panels = tabs.map((tab) => document.getElementById(tab.getAttribute('aria-controls')));
  const select = (index, focus = false) => {
    tabs.forEach((tab, i) => {
      tab.setAttribute('aria-selected', String(i === index));
      tab.tabIndex = i === index ? 0 : -1;
      panels[i].hidden = i !== index;
    });
    if (focus) tabs[index].focus();
  };
  tabs.forEach((tab, index) => {
    panels[index].setAttribute('role', 'tabpanel');
    panels[index].setAttribute('aria-labelledby', tab.id);
    tab.addEventListener('click', () => select(index));
    tab.addEventListener('keydown', (event) => {
      const next = {
        ArrowRight: (index + 1) % tabs.length,
        ArrowLeft: (index + tabs.length - 1) % tabs.length,
        Home: 0,
        End: tabs.length - 1,
      }[event.key];
      if (next !== undefined) {
        event.preventDefault();
        select(next, true);
      }
    });
  });
  for (const heading of group.querySelectorAll('[data-tab-fallback]')) heading.hidden = true;
  select(0);
  group.querySelector('[role="tablist"]').hidden = false;
}

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
