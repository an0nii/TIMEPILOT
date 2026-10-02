const api = '/api/v1/tasks';
const form = document.querySelector('#task-form');
const tasks = document.querySelector('#tasks');
const message = document.querySelector('#message');
const cancel = document.querySelector('#cancel');
const planForm = document.querySelector('#plan-form');
const planButton = document.querySelector('#plan-button');
const planOutput = document.querySelector('#ai-plan');
let taskItems = [];

const labels = { todo: 'К выполнению', in_progress: 'В работе', done: 'Готово' };

function escapeHtml(value) {
  const element = document.createElement('div');
  element.textContent = value;
  return element.innerHTML;
}

async function request(url, options = {}) {
  const response = await fetch(url, options);
  if (response.status === 204) return null;
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || `HTTP ${response.status}`);
  return body;
}

async function loadTasks() {
  try {
    taskItems = await request(api);
    tasks.innerHTML = taskItems.length ? taskItems.map(renderTask).join('') : '<p class="empty">Задач пока нет.</p>';
    showMessage('');
  } catch (error) {
    showMessage(error.message);
  }
}

function renderTask(item) {
  return `<article class="task">
    <div>
      <span class="status ${item.status}">${labels[item.status]}</span>
      <h3>${escapeHtml(item.title)}</h3>
      <p>${escapeHtml(item.description || 'Без описания')}</p>
    </div>
    <div class="task-actions">
      <button class="secondary" data-edit="${item.id}">Изменить</button>
      <button class="danger" data-delete="${item.id}">Удалить</button>
    </div>
  </article>`;
}

function showMessage(text) {
  message.textContent = text;
  message.classList.toggle('hidden', !text);
}

function resetForm() {
  form.reset();
  document.querySelector('#task-id').value = '';
  document.querySelector('#form-title').textContent = 'Новая задача';
  cancel.classList.add('hidden');
}

form.addEventListener('submit', async (event) => {
  event.preventDefault();
  const id = document.querySelector('#task-id').value;
  const payload = {
    title: document.querySelector('#title').value,
    description: document.querySelector('#description').value,
    status: document.querySelector('#status').value,
  };
  try {
    await request(id ? `${api}/${id}` : api, {
      method: id ? 'PUT' : 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    resetForm();
    await loadTasks();
  } catch (error) {
    showMessage(error.message);
  }
});

tasks.addEventListener('click', async (event) => {
  const edit = event.target.dataset.edit;
  const remove = event.target.dataset.delete;
  if (edit) {
    const item = taskItems.find((candidate) => candidate.id === Number(edit));
    if (!item) return;
    document.querySelector('#task-id').value = item.id;
    document.querySelector('#title').value = item.title;
    document.querySelector('#description').value = item.description;
    document.querySelector('#status').value = item.status;
    document.querySelector('#form-title').textContent = 'Редактирование';
    cancel.classList.remove('hidden');
    form.scrollIntoView({ behavior: 'smooth' });
  }
  if (remove && confirm('Удалить задачу?')) {
    try {
      await request(`${api}/${remove}`, { method: 'DELETE' });
      await loadTasks();
    } catch (error) {
      showMessage(error.message);
    }
  }
});

cancel.addEventListener('click', resetForm);
document.querySelector('#refresh').addEventListener('click', loadTasks);

async function checkConnection() {
  const badge = document.querySelector('#connection');
  try {
    await request('/readyz');
    badge.textContent = 'сервис готов';
    badge.classList.add('online');
  } catch {
    badge.textContent = 'нет соединения';
  }
}

checkConnection();
loadTasks();

async function checkAI() {
  const status = document.querySelector('#ai-status');
  const hint = document.querySelector('#ai-hint');
  try {
    const info = await request('/api/v1/ai/status');
    if (info.enabled) {
      status.textContent = `${info.provider} · ${info.model}`;
      status.classList.add('online');
      hint.textContent = 'План будет составлен по текущим задачам. API-ключ хранится только на сервере.';
      planButton.disabled = false;
    } else {
      status.textContent = 'не настроен';
      planButton.disabled = true;
    }
  } catch {
    status.textContent = 'недоступен';
    planButton.disabled = true;
  }
}

planForm.addEventListener('submit', async (event) => {
  event.preventDefault();
  planButton.disabled = true;
  planButton.textContent = 'Составляю…';
  planOutput.classList.add('hidden');
  try {
    const result = await request('/api/v1/ai/plan', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        goal: document.querySelector('#goal').value,
        available_minutes: Number(document.querySelector('#available-minutes').value),
      }),
    });
    planOutput.textContent = result.plan;
    planOutput.classList.remove('hidden');
  } catch (error) {
    showMessage(error.message);
  } finally {
    planButton.disabled = false;
    planButton.textContent = 'Создать план';
  }
});

checkAI();
