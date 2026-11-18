// Мои аренды: текущая аренда (начать, отменить, завершить) с расчётом стоимости
// на лету и история поездок с фильтром и сортировкой.
import { api, query } from '../api.js';
import {
  badge, busy, confirmDialog, el, emptyRow, formatDateTime, formatMinutes, formatMoney,
  openDialog, render, RENTAL_STATUS, sortableHeaders, toast,
} from '../ui.js';
import { carIcon, createMap } from '../map.js';

const filter = { status: '', sort: 'id', order: 'desc' };
let ticker;

export async function init() {
  const table = document.getElementById('history-table');
  const select = document.getElementById('status');
  select.addEventListener('change', () => { filter.status = select.value; loadHistory(table); });
  sortableHeaders(table, filter, () => loadHistory(table));
  await Promise.all([loadCurrent(), loadHistory(table)]);
}

const refresh = () => Promise.all([loadCurrent(), loadHistory(document.getElementById('history-table'))]);

// ---------- Текущая аренда ----------

async function loadCurrent() {
  const box = document.querySelector('.current-rental');
  clearInterval(ticker);
  try {
    const { items } = await api.get('/rentals/my?sort=id&order=desc');
    const rental = items.find((r) => r.status === 'booked' || r.status === 'active');
    if (!rental) {
      render(box, el('div', { className: 'empty-state' },
        el('p', { text: 'Сейчас у вас нет бронирований и поездок.' }),
        el('a', { className: 'btn btn-primary', href: '/catalog' }, 'Выбрать автомобиль')));
      return;
    }
    renderCurrent(box, rental);
  } catch (err) {
    render(box, el('p', { className: 'alert alert-error', text: err.message }));
  }
}

function renderCurrent(box, r) {
  const booked = r.status === 'booked';
  const estimate = el('span', { className: 'tile-value' });
  const waitEl = el('span', { className: 'tile-value' });
  const rideEl = el('span', { className: 'tile-value' });

  const tick = () => {
    const e = estimateCost(r, new Date());
    estimate.textContent = formatMoney(e.cost);
    waitEl.textContent = formatMinutes(e.waitMin);
    rideEl.textContent = booked ? '—' : formatMinutes(e.rideMin);
  };
  tick();
  ticker = setInterval(tick, 1000);

  const actions = booked
    ? [
      el('button', { className: 'btn btn-primary', type: 'button', on: { click: (e) => start(r, e.currentTarget) } }, 'Начать поездку'),
      el('button', { className: 'btn btn-secondary', type: 'button', on: { click: (e) => cancel(r, e.currentTarget) } }, 'Отменить бронирование'),
    ]
    : [el('button', { className: 'btn btn-primary', type: 'button', on: { click: () => finishDialog(r) } }, 'Завершить поездку')];

  render(box, el('div', { className: 'card rental-current' },
    el('div', { className: 'rental-card-head' },
      el('h3', { text: `${r.car.brand_model} · ${r.car.plate_number}` }),
      badge(r.status, RENTAL_STATUS)),
    el('p', { className: 'muted', text: booked
      ? `Забронировано ${formatDateTime(r.booked_at)}. Пока поездка не началась, начисляется ожидание.`
      : `Поездка началась ${formatDateTime(r.started_at)}.` }),
    el('div', { className: 'tiles' },
      el('div', { className: 'tile' }, el('span', { className: 'tile-label', text: 'Ожидание' }), waitEl,
        el('span', { className: 'tile-hint', text: `${formatMoney(r.waiting_per_min)} / мин` })),
      el('div', { className: 'tile' }, el('span', { className: 'tile-label', text: 'Поездка' }), rideEl,
        el('span', { className: 'tile-hint', text: `${formatMoney(r.price_per_min)} / мин` })),
      el('div', { className: 'tile tile-accent' }, el('span', { className: 'tile-label', text: 'Стоимость сейчас' }), estimate,
        el('span', { className: 'tile-hint', text: 'Итог рассчитает сервер при завершении' }))),
    el('div', { className: 'btn-group rental-actions' }, actions)));
}

/**
 * Оценка стоимости по правилу docs/api.md, раздел 2.3 (окончательную сумму считает сервер):
 * минуты ожидания = ⌈старт − бронирование⌉, минуты поездки = max(1, ⌈сейчас − старт⌉).
 */
function estimateCost(r, now) {
  const ceilMin = (ms) => (ms > 0 ? Math.ceil(ms / 60000) : 0);
  const booked = new Date(r.booked_at);
  const started = r.started_at ? new Date(r.started_at) : null;
  const waitMin = ceilMin((started || now) - booked);
  const rideMin = started ? Math.max(1, ceilMin(now - started)) : 0;
  const kop = (x) => Math.round(x * 100);
  const cost = (kop(r.waiting_per_min) * waitMin + kop(r.price_per_min) * rideMin) / 100;
  return { waitMin, rideMin, cost };
}

async function start(r, button) {
  const ok = await confirmDialog({
    title: 'Начать поездку',
    text: `Начать поездку на ${r.car.brand_model} (${r.car.plate_number})? Ожидание перестанет начисляться, начнётся поминутная оплата.`,
    confirmText: 'Начать',
  });
  if (!ok) return;
  busy(button, true);
  try {
    await api.post(`/rentals/${r.id}/start`);
    toast('Поездка началась. Хорошей дороги!', 'success');
    await refresh();
  } catch (err) {
    toast(err.message, 'error');
    busy(button, false);
  }
}

async function cancel(r, button) {
  const e = estimateCost(r, new Date());
  const ok = await confirmDialog({
    title: 'Отменить бронирование',
    text: `Отменить бронирование ${r.car.brand_model} (${r.car.plate_number})? Автомобиль станет доступен другим клиентам.`,
    confirmText: 'Отменить бронирование',
    danger: true,
  });
  if (!ok) return;
  busy(button, true);
  try {
    await api.post(`/rentals/${r.id}/cancel`);
    toast(`Бронирование отменено (ожидание ${formatMinutes(e.waitMin)})`, 'info');
    await refresh();
  } catch (err) {
    toast(err.message, 'error');
    busy(button, false);
  }
}

// Завершение: место парковки выбирается на карте, уровень топлива — в поле.
async function finishDialog(r) {
  let car = null;
  try { car = await api.get(`/cars/${r.car.id}`); } catch { /* без текущих данных машины */ }

  const fuel = el('input', { id: 'fuel_level', name: 'fuel_level', type: 'number', min: 0, max: 100, step: 1,
    inputmode: 'numeric', value: car ? car.fuel_level : '' });
  const coords = el('p', { className: 'muted small' });
  const error = el('p', { className: 'form-error', role: 'alert' });
  const submit = el('button', { className: 'btn btn-primary', type: 'submit' }, 'Завершить поездку');
  const mapBox = el('div', { className: 'map map-small' });
  const form = el('form', { className: 'form', novalidate: true },
    el('div', { className: 'field' },
      el('label', { text: 'Место парковки' }),
      el('p', { className: 'field-hint', text: 'Нажмите на карту, чтобы указать, где оставлен автомобиль.' }),
      mapBox, coords),
    el('div', { className: 'field' },
      el('label', { for: 'fuel_level', text: 'Уровень топлива, %' }), fuel,
      el('p', { className: 'field-error', dataset: { errorFor: 'fuel_level' } })),
    error,
    el('div', { className: 'form-actions' },
      el('button', { className: 'btn btn-secondary', type: 'button', on: { click: () => d.close() } }, 'Отмена'),
      submit));
  const d = openDialog({ title: 'Завершение поездки', content: form, className: 'modal-wide' });

  let point = car ? [car.latitude, car.longitude] : null;
  const showCoords = () => {
    coords.textContent = point ? `Координаты: ${point[0].toFixed(5)}, ${point[1].toFixed(5)}` : 'Место не выбрано: останутся прежние координаты.';
  };
  showCoords();
  const map = createMap(mapBox, { center: point || undefined, zoom: 15 });
  map.scrollWheelZoom.enable();
  const marker = point ? window.L.marker(point, { icon: carIcon('in_use') }).addTo(map) : null;
  let pin = marker;
  map.on('click', (ev) => {
    point = [ev.latlng.lat, ev.latlng.lng];
    if (pin) pin.setLatLng(ev.latlng);
    else pin = window.L.marker(ev.latlng, { icon: carIcon('in_use') }).addTo(map);
    showCoords();
  });
  setTimeout(() => map.invalidateSize(), 50); // карта в окне: пересчитать размер после показа

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    error.textContent = '';
    form.querySelector('[data-error-for="fuel_level"]').textContent = '';
    const body = {};
    if (fuel.value !== '') {
      const n = Number(fuel.value);
      if (!Number.isInteger(n) || n < 0 || n > 100) {
        form.querySelector('[data-error-for="fuel_level"]').textContent = 'Целое число от 0 до 100';
        fuel.focus();
        return;
      }
      body.fuel_level = n;
    }
    if (point) [body.latitude, body.longitude] = point;

    busy(submit, true);
    try {
      const done = await api.post(`/rentals/${r.id}/finish`, body);
      d.close();
      await refresh();
      resultDialog(done);
    } catch (err) {
      error.textContent = err.message;
      busy(submit, false);
    }
  });
}

function resultDialog(r) {
  const waitMin = Math.ceil((new Date(r.started_at) - new Date(r.booked_at)) / 60000);
  const rideMin = Math.max(1, Math.ceil((new Date(r.ended_at) - new Date(r.started_at)) / 60000));
  const d = openDialog({
    title: 'Поездка завершена',
    content: [
      el('p', { className: 'result-cost', text: formatMoney(r.total_cost) }),
      el('dl', { className: 'props' },
        el('dt', { text: 'Автомобиль' }), el('dd', { text: `${r.car.brand_model}, ${r.car.plate_number}` }),
        el('dt', { text: 'Ожидание' }), el('dd', { text: `${formatMinutes(Math.max(0, waitMin))} × ${formatMoney(r.waiting_per_min)}` }),
        el('dt', { text: 'Поездка' }), el('dd', { text: `${formatMinutes(rideMin)} × ${formatMoney(r.price_per_min)}` })),
      el('div', { className: 'form-actions' },
        el('button', { className: 'btn btn-primary', type: 'button', on: { click: () => d.close() } }, 'Готово')),
    ],
  });
}

// ---------- История ----------

async function loadHistory(table) {
  const tbody = table.querySelector('tbody');
  try {
    const { items } = await api.get(`/rentals/my${query(filter)}`);
    render(tbody, items.length ? items.map(historyRow) : emptyRow(table, filter.status ? 'Нет аренд с этим статусом.' : 'Аренд пока не было.'));
  } catch (err) {
    render(tbody, emptyRow(table, err.message));
  }
}

function historyRow(r) {
  const minutes = r.started_at && r.ended_at
    ? Math.max(1, Math.ceil((new Date(r.ended_at) - new Date(r.started_at)) / 60000))
    : null;
  return el('tr', {},
    el('td', { className: 'num', text: r.id }),
    el('td', {}, el('b', { text: r.car.brand_model }), el('div', { className: 'muted small', text: r.car.plate_number })),
    el('td', { className: 'nowrap', text: formatDateTime(r.booked_at) }),
    el('td', { className: 'nowrap', text: r.started_at ? `${formatDateTime(r.started_at)} – ${r.ended_at ? formatDateTime(r.ended_at) : '…'}` : '—' }),
    el('td', { className: 'num', text: minutes ? formatMinutes(minutes) : '—' }),
    el('td', { className: 'num', text: r.total_cost === null ? '—' : formatMoney(r.total_cost) }),
    el('td', {}, badge(r.status, RENTAL_STATUS)));
}
