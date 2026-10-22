const editor = document.querySelector('[data-content-editor]');
if (editor) {
  const language = editor.querySelector('[data-language]');
  const kind = editor.querySelector('[data-kind]');
  const templates = {
    section: editor.querySelector('.content-sections fieldset').cloneNode(true),
    step: editor.querySelector('.content-steps fieldset').cloneNode(true)
  };
  const resourceOptions = Array.from(templates.step.querySelector('[name="step_href"]').options, option => option.cloneNode(true));
  function refresh() {
    editor.querySelectorAll('[data-lang]').forEach(block => block.hidden = language.value !== 'both' && block.dataset.lang !== language.value);
    ['en', 'ar'].forEach(lang => editor.querySelector('[name="title_' + lang + '"]').required = language.value === 'both' || language.value === lang);
    editor.querySelector('[data-path-fields]').hidden = !kind || kind.value !== 'path';
    editor.querySelector('[data-article-fields]').hidden = kind?.value === 'path';
    const resourceRows = Array.from(editor.querySelectorAll('.path-resource-editor'));
    const selectedResources = new Set(resourceRows.map(row => row.querySelector('[name="step_href"]').value).filter(Boolean));
    resourceRows.forEach(row => {
      const picker = row.querySelector('[name="step_href"]');
      const current = picker.value;
      picker.replaceChildren(...resourceOptions
        .filter(option => !option.value || option.value === current || !selectedResources.has(option.value))
        .map(option => option.cloneNode(true)));
      picker.value = current;
      row.querySelector('[data-intro-fields]').hidden = !!row.querySelector('[name="step_href"]').value;
      row.querySelector('[data-move="up"]').disabled = !row.previousElementSibling;
      row.querySelector('[data-move="down"]').disabled = !row.nextElementSibling;
    });
  }
  language.addEventListener('change', refresh);
  kind?.addEventListener('change', refresh);
  editor.addEventListener('change', refresh);
  editor.addEventListener('click', event => {
    const add = event.target.closest('[data-add]');
    const remove = event.target.closest('[data-remove]');
    if (add) {
      const name = add.dataset.add;
      const field = templates[name].cloneNode(true);
      field.querySelectorAll('input,textarea,select').forEach(input => input.value = '');
      editor.querySelector(name === 'step' ? '.content-steps' : '.content-sections').append(field);
      refresh();
      field.querySelector('div:not([hidden]) input')?.focus();
    }
    if (remove) remove.closest('fieldset').remove();
    const move = event.target.closest('[data-move]');
    if (move) { const row = move.closest('fieldset'); if (move.dataset.move === 'up' && row.previousElementSibling) row.previousElementSibling.before(row); else if (move.dataset.move === 'down' && row.nextElementSibling) row.nextElementSibling.after(row); }
    refresh();
  });
  refresh();
}
const search = document.querySelector('[data-content-search]');
if (search) {
  const update = () => {
    let count = 0;
    document.querySelectorAll('[data-content-row]').forEach(row => {
      row.hidden = !row.textContent.toLowerCase().includes(search.value.toLowerCase());
      if (!row.hidden) count++;
    });
    document.querySelector('[data-empty-list]').hidden = count > 0;
  };
  search.addEventListener('input', update);
  update();
}
