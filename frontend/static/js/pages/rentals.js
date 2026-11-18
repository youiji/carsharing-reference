// Все аренды (менеджер, администратор): поиск по ФИО и гос. номеру, фильтр
// по статусу, сортировка, отмена бронирований клиентов.
import { api, query } from '../api.js';
import {
  badge, confirmAction, debounce, el, emptyRow, formatDateTime, formatMoney, render,
  RENTAL_STATUS, sortableHeaders,
} from '../ui.js';

const state = { q: '', status: '', user_id: '', car_id: '', sort: 'id', order: 'desc' };
let table;

export async function init() {
  table = document.getElementById('rentals-table');
  const form = document.getElementById('filters');
  const p = new URLSearchParams(location.search);
  for (const k of ['q', 'status', 'user_id', 'car_id']) state[k] = p.get(k) || '';
  form.q.value = state.q;
  form.status.value = state.status;
  if (state.user_id || state.car_id) {
    const note = document.querySelector('.filter-note');
    note.hidden = false;
    render(note, state.user_id ? `Аренды пользователя №${state.user_id}. ` : `Аренды автомобиля №${state.car_id}. `,
      el('a', { href: '/rentals' }, 'Показать все'));
  }

  form.addEventListener('submit', (e) => e.preventDefault());
  form.q.addEventListener('input', debounce(() => { state.q = form.q.value.trim(); load(); }));
  form.status.addEventListener('change', () => { state.status = form.status.value; load(); });
  sortableHeaders(table, state, load);
  await load();
}

async function load() {
  const tbody = table.querySelector('tbody');
  try {
    const { items, total } = await api.get(`/rentals${query(state)}`);
    render(tbody, items.length ? items.map(row) : emptyRow(table, 'Аренды не найдены.'));
    const revenue = items.reduce((s, r) => s + (r.total_cost ? Math.round(r.total_cost * 100) : 0), 0) / 100;
    document.querySelector('.rentals-summary').textContent = `Найдено: ${total}, сумма завершённых: ${formatMoney(revenue)}`;
  } catch (err) {
    render(tbody, emptyRow(table, err.message));
  }
}

function row(r) {
  return el('tr', {},
    el('td', { className: 'num', text: r.id }),
    el('td', {}, el('a', { href: `/rentals?user_id=${r.user.id}`, text: r.user.full_name })),
    el('td', {}, el('b', { text: r.car.brand_model }), el('span', { className: 'cell-sub', text: r.car.plate_number })),
    el('td', { className: 'nowrap', text: formatDateTime(r.booked_at) }),
    el('td', { className: 'nowrap', text: r.started_at ? `${formatDateTime(r.started_at)} – ${r.ended_at ? formatDateTime(r.ended_at) : '…'}` : '—' }),
    el('td', { className: 'num', text: r.total_cost === null ? '—' : formatMoney(r.total_cost) }),
    el('td', {}, badge(r.status, RENTAL_STATUS)),
    el('td', { className: 'actions' }, r.status === 'booked' && el('button', {
      className: 'btn btn-ghost btn-sm', type: 'button', on: { click: () => cancel(r) },
    }, 'Отменить')));
}

async function cancel(r) {
  if (await confirmAction({
    title: 'Отмена бронирования',
    text: `Отменить бронирование клиента ${r.user.full_name}: ${r.car.brand_model} (${r.car.plate_number})? Автомобиль станет свободен.`,
    confirmText: 'Отменить бронирование',
    danger: true,
    action: () => api.post(`/rentals/${r.id}/cancel`),
    success: 'Бронирование отменено',
  })) load();
}
