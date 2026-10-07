import './style.css';

const form = document.querySelector('#note-form');
const message = document.querySelector('#message');
const save = document.querySelector('#save');
const health = document.querySelector('#health');
const notes = document.querySelector('#notes');
document.querySelector('#release').textContent = import.meta.env.VITE_RELEASE || 'local';

async function api(path, options) {
  const response = await fetch(path, { ...options, signal: AbortSignal.timeout(10000) });
  const result = await response.json();
  if (!response.ok) throw new Error(result.error || 'Сервис временно недоступен');
  return result;
}

async function checkHealth() {
  try {
    const state = await api('/health');
    health.textContent = state.db ? '● Все системы работают' : '● База данных недоступна';
    health.className = state.db ? 'online' : 'offline';
  } catch {
    health.textContent = '● Нет связи с сервером';
    health.className = 'offline';
  }
}

async function loadNotes() {
  try {
    const result = await api('/api/notes');
    notes.replaceChildren();
    document.querySelector('#count').textContent = result.length;
    if (!result.length) {
      const empty = document.createElement('p');
      empty.className = 'empty';
      empty.textContent = 'Пока здесь чистый лист. Создайте первую заметку.';
      notes.append(empty);
    }
    for (const note of result.slice().reverse()) {
      const card = document.createElement('article');
      const id = document.createElement('span');
      id.className = 'note-id'; id.textContent = `ЗАМЕТКА ${String(note.id).padStart(2, '0')}`;
      const title = document.createElement('h3'); title.textContent = note.title;
      const body = document.createElement('p'); body.textContent = note.body || 'Без текста';
      card.append(id, title, body); notes.append(card);
    }
  } catch {
    message.textContent = 'Не удалось загрузить заметки. Проверьте соединение и повторите.';
  }
}

form.addEventListener('submit', async (event) => {
  event.preventDefault();
  if (!form.reportValidity()) return;
  save.disabled = true; message.textContent = '';
  try {
    await api('/api/notes', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: form.elements.title.value.trim(), body: form.elements.body.value }),
    });
    form.reset(); message.textContent = 'Заметка сохранена.';
    await loadNotes();
  } catch (error) {
    message.textContent = `Не удалось сохранить: ${error.message}`;
  } finally { save.disabled = false; }
});
document.querySelector('#refresh').addEventListener('click', () => { message.textContent = ''; loadNotes(); checkHealth(); });
loadNotes(); checkHealth(); setInterval(checkHealth, 15000);
